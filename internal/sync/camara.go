package sync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"voz-do-povo/internal/camara"
)

// Raw source keys for the Câmara syncs. Raw rows are staging: each sync run
// REPLACES the rows under its key, so the raw layer holds the latest pull, not
// an ever-growing log (the Postgres volume is the Hobby-plan constraint).
const (
	SourceCamaraDeputies = "camara-deputies"
	// SourceCamaraCEAPBulk is the prefix for the per-year bulk reconcile keys
	// (e.g. "camara-ceap-bulk-2026"). The bulk files are updated daily and
	// are the only working CEAP source: the API's despesas endpoint returns
	// empty for every deputy (verified 2026-09-08).
	SourceCamaraCEAPBulk = "camara-ceap-bulk-"

	houseCamara = "camara"
)

// SyncCamaraDeputies lands the deputy registry into raw and promotes it:
// parties are upserted by sigla, canonical politicians are created (or updated
// via their Câmara source id / exact CPF), and the source-id map is filled.
// All-or-nothing: any failure rolls back, leaving curated data untouched.
func SyncCamaraDeputies(ctx context.Context, db *sql.DB, deputies []camara.Deputy) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	payloads := make([]json.RawMessage, 0, len(deputies))
	for _, d := range deputies {
		p, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("marshal deputy %d: %w", d.ID, err)
		}
		payloads = append(payloads, p)
	}
	if err := replaceRaw(ctx, tx, SourceCamaraDeputies, payloads); err != nil {
		return err
	}

	for _, d := range deputies {
		if d.ID == 0 || d.Name == "" {
			return fmt.Errorf("deputy registry entry missing id or name: %+v", d)
		}

		// Party sigla from the registry is not stored on the politician:
		// parties are upserted when expenses (which carry the sigla at
		// expense time) are promoted.
		civilName := d.CivilName
		if civilName == "" {
			civilName = d.Name
		}

		// Known source id → update the canonical record.
		var politicianID int64
		err := tx.QueryRowContext(ctx,
			`SELECT politician_id FROM curated.politician_source_id WHERE source = 'camara' AND source_id = $1`,
			strconv.FormatInt(d.ID, 10)).Scan(&politicianID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("lookup deputy %d source id: %w", d.ID, err)
		}

		// New to this source: exact CPF match attaches the Câmara id to the
		// existing canonical person (cross-source auto-link).
		if err == sql.ErrNoRows && d.CPF != "" {
			err = tx.QueryRowContext(ctx,
				`SELECT id FROM curated.politician WHERE cpf = $1`, d.CPF).Scan(&politicianID)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("lookup deputy %d cpf: %w", d.ID, err)
			}
		}

		if politicianID == 0 {
			err = tx.QueryRowContext(ctx, `
				INSERT INTO curated.politician (civil_name, parliamentary_name, name_norm, uf, cpf)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id`,
				civilName, d.Name, NormalizeName(d.Name), nilIfEmpty(d.UF), nilIfEmpty(d.CPF)).Scan(&politicianID)
			if err != nil {
				return fmt.Errorf("create politician for deputy %d: %w", d.ID, err)
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE curated.politician
				SET civil_name = $2, parliamentary_name = $3, name_norm = $4,
				    uf = $5, cpf = COALESCE($6, cpf), updated_at = now()
				WHERE id = $1`,
				politicianID, civilName, d.Name, NormalizeName(d.Name), nilIfEmpty(d.UF), nilIfEmpty(d.CPF)); err != nil {
				return fmt.Errorf("update politician %d: %w", politicianID, err)
			}
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO curated.politician_source_id (politician_id, source, source_id)
			VALUES ($1, 'camara', $2)
			ON CONFLICT (source, source_id) DO UPDATE SET politician_id = EXCLUDED.politician_id`,
			politicianID, strconv.FormatInt(d.ID, 10)); err != nil {
			return fmt.Errorf("map deputy %d source id: %w", d.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ExpenseSyncStats summarizes a CEAP sync run for the (logs-only) ops trail.
type ExpenseSyncStats struct {
	Rows          int   // source rows processed
	Linked        int   // expenses linked to a canonical politician
	Unlinked      int   // expenses left unlinked (includes queued fuzzy matches)
	ReconciledOut int64 // curated rows deleted by bulk reconcile (absent from the pull)
}

// SyncCamaraExpenses lands CEAP expenses into raw under the given source key —
// replacing that key's previous rows — and promotes them into curated.expense.
// CEAP registers have no unique id (ideDocumento groups installments), so the
// dedup key is a hash of the full normalized row: a corrected row lands as a
// new register and the stale one is reconciled out. When reconcileYear > 0,
// curated rows for that year absent from this pull are deleted — the bulk file
// is ground truth. Identity linking is CPF-first with fuzzy matches queued for
// review, never force-linked. All-or-nothing: any failure rolls back, leaving
// curated data untouched.
func SyncCamaraExpenses(ctx context.Context, db *sql.DB, source string, reconcileYear int, expenses []camara.Expense) (ExpenseSyncStats, error) {
	var stats ExpenseSyncStats

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	// DB clock, so the reconcile cutoff below shares synced_at's clock.
	var runStart time.Time
	if err := tx.QueryRowContext(ctx, `SELECT now()`).Scan(&runStart); err != nil {
		return stats, fmt.Errorf("read db clock: %w", err)
	}

	payloads := make([]json.RawMessage, 0, len(expenses))
	for _, e := range expenses {
		// Prefer the verbatim source record; fall back to the normalized
		// struct (fixtures, future sources without a verbatim path).
		p := e.Raw
		if len(p) == 0 {
			var err error
			p, err = json.Marshal(e)
			if err != nil {
				return stats, fmt.Errorf("marshal expense %q: %w", e.DocumentID, err)
			}
		}
		payloads = append(payloads, p)
	}
	rawIDs, err := replaceRawReturningIDs(ctx, tx, source, payloads)
	if err != nil {
		return stats, err
	}

	parties := map[string]int64{}
	for i, e := range expenses {
		stats.Rows++
		if e.Year == 0 || e.Month == 0 || e.Category == "" {
			return stats, fmt.Errorf("expense row %d missing category or period: %+v", i, e)
		}
		rowKey := expenseRowKey(e)

		politicianID, err := LinkIdentity(ctx, tx, Identity{
			Source: houseCamara,
			CPF:    e.DeputyCPF,
			Name:   e.DeputyName,
			UF:     e.DeputyUF,
		})
		if err != nil {
			return stats, fmt.Errorf("link expense %q: %w", e.DocumentID, err)
		}
		var pid *int64
		if politicianID != 0 {
			pid = &politicianID
			stats.Linked++
		} else {
			stats.Unlinked++
		}

		var partyID *int64
		if e.PartySigla != "" {
			id, ok := parties[e.PartySigla]
			if !ok {
				id, err = upsertParty(ctx, tx, e.PartySigla)
				if err != nil {
					return stats, err
				}
				parties[e.PartySigla] = id
			}
			partyID = &id
		}

		var issuedAt *time.Time
		if e.IssuedAt != "" {
			t, err := time.Parse("2006-01-02", e.IssuedAt)
			if err != nil {
				return stats, fmt.Errorf("expense %q issued_at %q: %w", e.DocumentID, e.IssuedAt, err)
			}
			issuedAt = &t
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO curated.expense (
				politician_id, party_id, house, category,
				supplier_name, supplier_tax_id, document_number, issued_at,
				year, month, amount_document, amount_glosa, amount_net,
				document_url, source_document_id, source_row_key, raw_id, synced_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17, now())
			ON CONFLICT (house, source_row_key) DO UPDATE SET
				politician_id   = EXCLUDED.politician_id,
				party_id        = EXCLUDED.party_id,
				raw_id          = EXCLUDED.raw_id,
				synced_at       = now()`,
			pid, partyID, houseCamara, e.Category,
			nilIfEmpty(e.SupplierName), nilIfEmpty(e.SupplierTaxID), nilIfEmpty(e.DocumentNumber), issuedAt,
			e.Year, e.Month, e.AmountDocument, e.AmountGlosa, e.AmountNet,
			nilIfEmpty(e.DocumentURL), nilIfEmpty(e.DocumentID), rowKey, rawIDs[i]); err != nil {
			return stats, fmt.Errorf("upsert expense %q: %w", e.DocumentID, err)
		}
	}

	if reconcileYear > 0 {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM curated.expense
			WHERE house = $1 AND year = $2 AND synced_at < $3`,
			houseCamara, reconcileYear, runStart)
		if err != nil {
			return stats, fmt.Errorf("reconcile year %d: %w", reconcileYear, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			stats.ReconciledOut = n
		}
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

// CamaraDeputiesWithCPF returns the Câmara source ids whose canonical
// politician already has a CPF — i.e. the deputies the registry sync can skip
// fetching details for.
func CamaraDeputiesWithCPF(ctx context.Context, db *sql.DB) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.source_id
		FROM curated.politician_source_id s
		JOIN curated.politician p ON p.id = s.politician_id
		WHERE s.source = 'camara' AND p.cpf IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("query deputies with cpf: %w", err)
	}
	defer rows.Close()

	out := map[int64]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan source id: %w", err)
		}
		if n, err := strconv.ParseInt(id, 10, 64); err == nil {
			out[n] = true
		}
	}
	return out, rows.Err()
}

// upsertParty returns the id of the party with the given sigla, creating it if
// needed.
func upsertParty(ctx context.Context, tx *sql.Tx, sigla string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO curated.party (sigla) VALUES ($1)
		ON CONFLICT (sigla) DO UPDATE SET sigla = EXCLUDED.sigla
		RETURNING id`, sigla).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert party %q: %w", sigla, err)
	}
	return id, nil
}

// expenseRowKey is the dedup key for a CEAP register: a hash of the full
// normalized row. CEAP has no per-register unique id (ideDocumento groups
// installments of one document), so identity = the whole row; a source-side
// correction produces a new key and the reconcile pass removes the stale row.
func expenseRowKey(e camara.Expense) string {
	b, _ := json.Marshal(e)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
