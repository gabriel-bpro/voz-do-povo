package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"voz-do-povo/internal/transparencia"
)

// Raw source keys for the Portal da Transparência syncs. Same staging rule as
// the other sources: each run REPLACES the rows under its key.
const (
	// SourceTransparenciaEmendas identifies the promoted emendas dataset; the
	// API uses it as the provenance key.
	SourceTransparenciaEmendas = "transparencia-emendas"
	// SourceTransparenciaEmendasYear is the prefix for the per-year bulk
	// keys (e.g. "transparencia-emendas-2026").
	SourceTransparenciaEmendasYear = "transparencia-emendas-"

	sourceTransparencia = "transparencia"
)

// emendaAuthorSimilarityFloor is the "really high threshold" for the fuzzy
// author-name fallback (user decision, issue 17): below it, no link.
const emendaAuthorSimilarityFloor = 0.90

// isPIXEmenda reports the permanent PIX type marker: a transferência
// especial, identified by its normalized tipo (see transparencia constants).
func isPIXEmenda(tipo string) bool {
	norm := NormalizeName(tipo)
	return strings.Contains(norm, transparencia.PIXTipoFragmentPlural) ||
		strings.Contains(norm, transparencia.PIXTipoFragmentSingular)
}

// EmendaSyncStats summarizes an emendas sync run for the (logs-only) ops trail.
type EmendaSyncStats struct {
	Rows          int
	Linked        int
	Unlinked      int
	ReconciledOut int64
}

// SyncEmendas lands the Portal da Transparência bulk spreadsheet into raw —
// replacing that key's previous rows — and promotes it into curated.emenda,
// upserting on the emenda's natural key (code). When reconcileYear > 0,
// curated rows for that year absent from this pull are deleted: the bulk file
// is ground truth. All-or-nothing: any failure rolls back, leaving curated
// data untouched.
func SyncEmendas(ctx context.Context, db *sql.DB, source string, reconcileYear int, emendas []transparencia.Emenda) (EmendaSyncStats, error) {
	var stats EmendaSyncStats

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

	payloads := make([]json.RawMessage, 0, len(emendas))
	for _, e := range emendas {
		p := e.Raw
		if len(p) == 0 {
			var err error
			p, err = json.Marshal(e)
			if err != nil {
				return stats, fmt.Errorf("marshal emenda %q: %w", e.Code, err)
			}
		}
		payloads = append(payloads, p)
	}
	rawIDs, err := replaceRawReturningIDs(ctx, tx, source, payloads)
	if err != nil {
		return stats, err
	}

	// Author names repeat heavily across rows; resolve each distinct name once.
	linkCache := map[string]int64{}
	for i, e := range emendas {
		stats.Rows++
		if e.Code == "" || e.Year == 0 || e.Tipo == "" {
			return stats, fmt.Errorf("emenda row %d missing code, year or tipo: %+v", i, e)
		}

		normName := normalizeEmendaAuthor(e.AuthorName)
		politicianID, ok := linkCache[normName]
		if !ok {
			politicianID, err = linkEmendaAuthor(ctx, tx, e.AuthorName, normName)
			if err != nil {
				return stats, fmt.Errorf("link emenda %q author %q: %w", e.Code, e.AuthorName, err)
			}
			linkCache[normName] = politicianID
		}
		var pid *int64
		if politicianID != 0 {
			pid = &politicianID
			stats.Linked++
		} else {
			stats.Unlinked++
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO curated.emenda (
				politician_id, author_name, code, year, number, tipo, is_pix,
				localidade, funcao, subfuncao,
				amount_empenhado, amount_liquidado, amount_pago,
				raw_id, synced_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14, now())
			ON CONFLICT (code) DO UPDATE SET
				politician_id    = EXCLUDED.politician_id,
				author_name      = EXCLUDED.author_name,
				number           = EXCLUDED.number,
				tipo             = EXCLUDED.tipo,
				is_pix           = EXCLUDED.is_pix,
				localidade       = EXCLUDED.localidade,
				funcao           = EXCLUDED.funcao,
				subfuncao        = EXCLUDED.subfuncao,
				amount_empenhado = EXCLUDED.amount_empenhado,
				amount_liquidado = EXCLUDED.amount_liquidado,
				amount_pago      = EXCLUDED.amount_pago,
				raw_id           = EXCLUDED.raw_id,
				synced_at        = now()`,
			pid, e.AuthorName, e.Code, e.Year, nilIfEmpty(e.Number), e.Tipo,
			isPIXEmenda(e.Tipo),
			nilIfEmpty(e.Localidade), nilIfEmpty(e.Funcao), nilIfEmpty(e.Subfuncao),
			e.Empenhado, e.Liquidado, e.Pago,
			rawIDs[i]); err != nil {
			return stats, fmt.Errorf("upsert emenda %q: %w", e.Code, err)
		}
	}

	if reconcileYear > 0 {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM curated.emenda
			WHERE year = $1 AND synced_at < $2`,
			reconcileYear, runStart)
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

// normalizeEmendaAuthor prepares the author name for matching: the portal
// publishes it bare or with a cargo prefix ("Dep. Fulana"), which the
// normalized registry names never carry.
func normalizeEmendaAuthor(name string) string {
	n := NormalizeName(name)
	for _, prefix := range []string{"DEPUTADO ", "DEPUTADA ", "SENADOR ", "SENADORA ", "DEP. ", "SEN. "} {
		if strings.HasPrefix(n, prefix) {
			n = strings.TrimSpace(strings.TrimPrefix(n, prefix))
			break
		}
	}
	return n
}

// linkEmendaAuthor resolves an emenda's author to a canonical politician. The
// source carries no CPF, no UF, no usable source id — the name is all there
// is. Per the issue-17 decision:
//
//  1. normalized name matches exactly ONE politician → auto-link
//  2. no exact match → pg_trgm similarity; the single closest match at or
//     above the floor (0.90) auto-links
//  3. a tie at the top → manual review queue, nothing links
//  4. otherwise → unlinked
//
// Multiple exact matches are queued pairwise (name collisions do exist).
func linkEmendaAuthor(ctx context.Context, tx *sql.Tx, publishedName, normName string) (int64, error) {
	if normName == "" {
		return 0, nil
	}
	id := Identity{Source: sourceTransparencia, Name: publishedName}

	exact, err := tx.QueryContext(ctx,
		`SELECT id FROM curated.politician WHERE name_norm = $1`, normName)
	if err != nil {
		return 0, fmt.Errorf("lookup exact name: %w", err)
	}
	var exactIDs []int64
	for exact.Next() {
		var pid int64
		if err := exact.Scan(&pid); err != nil {
			exact.Close()
			return 0, fmt.Errorf("scan exact match: %w", err)
		}
		exactIDs = append(exactIDs, pid)
	}
	exact.Close()
	if err := exact.Err(); err != nil {
		return 0, fmt.Errorf("iterate exact matches: %w", err)
	}
	switch len(exactIDs) {
	case 1:
		return exactIDs[0], nil
	case 0:
		// fall through to the similarity fallback
	default:
		for _, pid := range exactIDs {
			if err := queueIdentityReview(ctx, tx, id, pid, "emenda-author-name-collision"); err != nil {
				return 0, err
			}
		}
		return 0, nil
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, similarity(name_norm, $1) AS sim
		FROM curated.politician
		WHERE similarity(name_norm, $1) >= $2
		ORDER BY sim DESC
		LIMIT 2`, normName, emendaAuthorSimilarityFloor)
	if err != nil {
		return 0, fmt.Errorf("lookup similar names: %w", err)
	}
	type hit struct {
		id  int64
		sim float64
	}
	var hits []hit
	for rows.Next() {
		var h hit
		if err := rows.Scan(&h.id, &h.sim); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan similar name: %w", err)
		}
		hits = append(hits, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate similar names: %w", err)
	}
	if len(hits) == 0 {
		return 0, nil
	}
	if len(hits) > 1 && hits[1].sim == hits[0].sim {
		for _, h := range hits {
			if err := queueIdentityReview(ctx, tx, id, h.id, "emenda-author-similarity-tie"); err != nil {
				return 0, err
			}
		}
		return 0, nil
	}
	return hits[0].id, nil
}
