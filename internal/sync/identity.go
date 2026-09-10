package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"unicode"
)

// NormalizeName uppercases, strips accents, and collapses whitespace — the
// fuzzy-match key for person names across sources ("Maria Exemplo" ≡ "MARIA
// EXEMPLO").
func NormalizeName(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.Join(strings.Fields(strings.ToUpper(out)), " ")
}

// Identity is the set of person identifiers a source record carries. Any
// field may be absent; LinkIdentity uses what's there.
type Identity struct {
	Source   string `json:"source"`              // e.g. "camara"
	SourceID string `json:"source_id,omitempty"` // the source's own person id
	CPF      string `json:"cpf,omitempty"`       // digits only
	Name     string `json:"name,omitempty"`      // name as published
	UF       string `json:"uf,omitempty"`
}

// LinkIdentity resolves a source record's person identifiers to a canonical
// politician, CPF-first, confidence-gated:
//
//  1. exact source-id match → auto-link
//  2. exact CPF match → auto-link (and the source id, if any, is backfilled
//     into the id map)
//  3. normalized name + UF match → manual review queue, record stays unlinked
//  4. nothing → unlinked
//
// A false link is a defamation-grade bug: fuzzy matches are NEVER linked
// without a human resolving the queue. Returns the politician id, or 0 when
// unlinked.
func LinkIdentity(ctx context.Context, tx *sql.Tx, id Identity) (int64, error) {
	if id.SourceID != "" {
		var pid int64
		err := tx.QueryRowContext(ctx,
			`SELECT politician_id FROM curated.politician_source_id WHERE source = $1 AND source_id = $2`,
			id.Source, id.SourceID).Scan(&pid)
		if err == nil {
			return pid, nil
		}
		if err != sql.ErrNoRows {
			return 0, fmt.Errorf("lookup source id: %w", err)
		}
	}

	if id.CPF != "" {
		var pid int64
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM curated.politician WHERE cpf = $1`, id.CPF).Scan(&pid)
		if err == nil {
			if id.SourceID != "" {
				conflict, err := backfillSourceID(ctx, tx, pid, id)
				if err != nil {
					return 0, err
				}
				if conflict {
					// CPF says one person, the source id is already mapped
					// to another: a false link is a defamation-grade bug —
					// queue for a human, link nothing.
					if err := queueIdentityReview(ctx, tx, id, pid, "conflicting-identifiers"); err != nil {
						return 0, err
					}
					return 0, nil
				}
			}
			return pid, nil
		}
		if err != sql.ErrNoRows {
			return 0, fmt.Errorf("lookup cpf: %w", err)
		}
	}

	if id.Name != "" && id.UF != "" {
		rows, err := tx.QueryContext(ctx,
			`SELECT id FROM curated.politician WHERE name_norm = $1 AND uf = $2`,
			NormalizeName(id.Name), id.UF)
		if err != nil {
			return 0, fmt.Errorf("lookup name+uf: %w", err)
		}
		var candidates []int64
		for rows.Next() {
			var pid int64
			if err := rows.Scan(&pid); err != nil {
				rows.Close()
				return 0, fmt.Errorf("scan candidate: %w", err)
			}
			candidates = append(candidates, pid)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("iterate candidates: %w", err)
		}
		for _, pid := range candidates {
			if err := queueIdentityReview(ctx, tx, id, pid, "fuzzy-name-uf-match"); err != nil {
				return 0, err
			}
		}
		return 0, nil
	}

	return 0, nil
}

// backfillSourceID attaches a source id to a CPF-matched politician. Reports
// conflict=true when the source id is already mapped to a DIFFERENT
// politician (nothing is written in that case).
func backfillSourceID(ctx context.Context, tx *sql.Tx, politicianID int64, id Identity) (conflict bool, err error) {
	var mapped int64
	err = tx.QueryRowContext(ctx,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = $1 AND source_id = $2`,
		id.Source, id.SourceID).Scan(&mapped)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("check source id mapping: %w", err)
	}
	if err == nil {
		return mapped != politicianID, nil
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO curated.politician_source_id (politician_id, source, source_id)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		politicianID, id.Source, id.SourceID); err != nil {
		return false, fmt.Errorf("backfill source id: %w", err)
	}
	return false, nil
}

// queueIdentityReview files a fuzzy or conflicting match for a human. Pending
// rows are deduplicated on (source, identity, candidate) so re-ingests don't
// spam the queue.
func queueIdentityReview(ctx context.Context, tx *sql.Tx, id Identity, candidateID int64, reason string) error {
	identityJSON, err := json.Marshal(id)
	if err != nil {
		return fmt.Errorf("marshal identity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO curated.identity_link_review (source, identity, candidate_politician_id, reason)
		SELECT $1, $2, $3, $4
		WHERE NOT EXISTS (
			SELECT 1 FROM curated.identity_link_review
			WHERE source = $1 AND identity = $2 AND candidate_politician_id = $3 AND status = 'pending'
		)`, id.Source, identityJSON, candidateID, reason); err != nil {
		return fmt.Errorf("queue identity review: %w", err)
	}
	return nil
}
