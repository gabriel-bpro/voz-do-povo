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

	"voz-do-povo/internal/senado"
)

// Raw source keys for the Senado syncs. Raw rows are staging: each sync run
// REPLACES the rows under its key, so the raw layer holds the latest pull.
const (
	SourceSenadoSenators = "senado-senators"
	// SourceSenadoCEAPS identifies the promoted CEAPS dataset
	// (curated.expense rows with house='senado'); the API uses it as the
	// provenance key.
	SourceSenadoCEAPS = "senado-ceaps"
	// SourceSenadoCEAPSYear is the prefix for the per-year reconcile keys
	// (e.g. "senado-ceaps-2026"). The dataset is updated daily upstream.
	SourceSenadoCEAPSYear = "senado-ceaps-"

	houseSenado = "senado"
)

// SyncSenadoSenators lands the senator registry into raw and promotes it.
// Identity resolution goes through LinkIdentity: known Senado code → update,
// CPF match (the Senado publishes no CPF today, but the path is uniform) →
// auto-link, name+UF match against another house's politician → queued for
// manual review AND a separate politician is still created (the senator's
// profile must exist while a human decides; a false merge is a
// defamation-grade bug). All-or-nothing: any failure rolls back.
func SyncSenadoSenators(ctx context.Context, db *sql.DB, senators []senado.Senator) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	payloads := make([]json.RawMessage, 0, len(senators))
	for _, s := range senators {
		p, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("marshal senator %d: %w", s.Code, err)
		}
		payloads = append(payloads, p)
	}
	if err := replaceRaw(ctx, tx, SourceSenadoSenators, payloads); err != nil {
		return err
	}

	for _, s := range senators {
		if s.Code == 0 || s.Name == "" {
			return fmt.Errorf("senator registry entry missing code or name: %+v", s)
		}
		code := strconv.FormatInt(s.Code, 10)

		civilName := s.CivilName
		if civilName == "" {
			civilName = s.Name
		}

		politicianID, err := LinkIdentity(ctx, tx, Identity{
			Source:   houseSenado,
			SourceID: code,
			Name:     s.Name,
			UF:       s.UF,
		})
		if err != nil {
			return fmt.Errorf("link senator %d: %w", s.Code, err)
		}

		if politicianID == 0 {
			err = tx.QueryRowContext(ctx, `
				INSERT INTO curated.politician (civil_name, parliamentary_name, name_norm, uf)
				VALUES ($1, $2, $3, $4)
				RETURNING id`,
				civilName, s.Name, NormalizeName(s.Name), nilIfEmpty(s.UF)).Scan(&politicianID)
			if err != nil {
				return fmt.Errorf("create politician for senator %d: %w", s.Code, err)
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE curated.politician
				SET civil_name = $2, parliamentary_name = $3, name_norm = $4,
				    uf = $5, updated_at = now()
				WHERE id = $1`,
				politicianID, civilName, s.Name, NormalizeName(s.Name), nilIfEmpty(s.UF)); err != nil {
				return fmt.Errorf("update politician %d: %w", politicianID, err)
			}
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO curated.politician_source_id (politician_id, source, source_id)
			VALUES ($1, 'senado', $2)
			ON CONFLICT (source, source_id) DO UPDATE SET politician_id = EXCLUDED.politician_id`,
			politicianID, code); err != nil {
			return fmt.Errorf("map senator %d source id: %w", s.Code, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// SyncSenadoExpenses lands CEAPS expenses into raw under the given source
// key — replacing that key's previous rows — and promotes them into
// curated.expense with house='senado'. Linking is via the row's codSenador
// against the registry-built source-id map (auto-link); rows for senators
// absent from the registry stay unlinked. CEAPS carries no party sigla, so
// partyBySenator (code → sigla, from the registry sync's input) stands in as
// the party at sync time. When reconcileYear > 0, curated rows for that year
// absent from this pull are deleted — the year dataset is ground truth.
// All-or-nothing: any failure rolls back, leaving curated data untouched.
func SyncSenadoExpenses(ctx context.Context, db *sql.DB, source string, reconcileYear int, expenses []senado.Expense, partyBySenator map[int64]string) (ExpenseSyncStats, error) {
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
		// struct (fixtures).
		p := e.Raw
		if len(p) == 0 {
			var err error
			p, err = json.Marshal(e)
			if err != nil {
				return stats, fmt.Errorf("marshal expense %d: %w", e.ID, err)
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
		rowKey := ceapsRowKey(e)

		politicianID, err := LinkIdentity(ctx, tx, Identity{
			Source:   houseSenado,
			SourceID: strconv.FormatInt(e.SenatorCode, 10),
			Name:     e.SenatorName,
		})
		if err != nil {
			return stats, fmt.Errorf("link expense %d: %w", e.ID, err)
		}
		var pid *int64
		if politicianID != 0 {
			pid = &politicianID
			stats.Linked++
		} else {
			stats.Unlinked++
		}

		var partyID *int64
		if sigla := partyBySenator[e.SenatorCode]; sigla != "" {
			id, ok := parties[sigla]
			if !ok {
				id, err = upsertParty(ctx, tx, sigla)
				if err != nil {
					return stats, err
				}
				parties[sigla] = id
			}
			partyID = &id
		}

		var issuedAt *time.Time
		if e.IssuedAt != "" {
			t, err := time.Parse("2006-01-02", e.IssuedAt)
			if err != nil {
				return stats, fmt.Errorf("expense %d issued_at %q: %w", e.ID, e.IssuedAt, err)
			}
			issuedAt = &t
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO curated.expense (
				politician_id, party_id, house, category,
				supplier_name, supplier_tax_id, document_number, issued_at,
				year, month, amount_net,
				source_document_id, source_row_key, raw_id, synced_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14, now())
			ON CONFLICT (house, source_row_key) DO UPDATE SET
				politician_id   = EXCLUDED.politician_id,
				party_id        = EXCLUDED.party_id,
				raw_id          = EXCLUDED.raw_id,
				synced_at       = now()`,
			pid, partyID, houseSenado, e.Category,
			nilIfEmpty(e.SupplierName), nilIfEmpty(e.SupplierTaxID), nilIfEmpty(e.DocumentNumber), issuedAt,
			e.Year, e.Month, e.AmountNet,
			nilIfEmpty(strconv.FormatInt(e.ID, 10)), rowKey, rawIDs[i]); err != nil {
			return stats, fmt.Errorf("upsert expense %d: %w", e.ID, err)
		}
	}

	if reconcileYear > 0 {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM curated.expense
			WHERE house = $1 AND year = $2 AND synced_at < $3`,
			houseSenado, reconcileYear, runStart)
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

// ceapsRowKey is the dedup key for a CEAPS register: a hash of the full
// normalized row. CEAPS rows do have a unique id, but hashing the row (like
// CEAP) makes a source-side correction land as a new register and lets the
// reconcile pass remove the stale one — uniform semantics across houses.
func ceapsRowKey(e senado.Expense) string {
	b, _ := json.Marshal(e)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
