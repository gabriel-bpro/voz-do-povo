// Package sync holds the sync entry points that land source data into the raw
// layer and promote it into curated. Real per-source syncs arrive with the
// sync tickets (12+); IngestRaw is the shared raw-landing path they build on.
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// IngestRaw lands source payloads verbatim into raw.payload with provenance
// (source name, pulled-at timestamp). Returns the number of rows landed.
func IngestRaw(ctx context.Context, db *sql.DB, source string, payloads []json.RawMessage) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	for _, p := range payloads {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO raw.payload (source, payload) VALUES ($1, $2)`, source, p); err != nil {
			return 0, fmt.Errorf("insert raw payload: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(payloads), nil
}
