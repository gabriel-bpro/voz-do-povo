// Package sync holds the sync entry points that land source data into the raw
// layer and promote it into curated. This file is the source-agnostic
// raw-layer plumbing; per-source syncs (camara.go, …) build on it.
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

// replaceRaw deletes the source key's previous raw rows and inserts the new
// payloads, in the same transaction. Raw is staging: each sync run REPLACES
// its key's rows, so the layer holds the latest pull, not an ever-growing log
// (the Postgres volume is the Hobby-plan constraint).
func replaceRaw(ctx context.Context, tx *sql.Tx, source string, payloads []json.RawMessage) error {
	_, err := replaceRawReturningIDs(ctx, tx, source, payloads)
	return err
}

// replaceRawReturningIDs is replaceRaw, returning the new raw row ids in
// payload order for provenance pointers.
func replaceRawReturningIDs(ctx context.Context, tx *sql.Tx, source string, payloads []json.RawMessage) ([]int64, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM raw.payload WHERE source = $1`, source); err != nil {
		return nil, fmt.Errorf("replace raw %s: %w", source, err)
	}

	const batch = 500
	ids := make([]int64, 0, len(payloads))
	for start := 0; start < len(payloads); start += batch {
		end := min(start+batch, len(payloads))
		chunk := payloads[start:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO raw.payload (source, payload) VALUES `)
		args := make([]any, 0, len(chunk)+1)
		args = append(args, source)
		for i, p := range chunk {
			if i > 0 {
				sb.WriteString(", ")
			}
			args = append(args, p)
			fmt.Fprintf(&sb, "($1, $%d)", i+2)
		}
		sb.WriteString(` RETURNING id`)

		rows, err := tx.QueryContext(ctx, sb.String(), args...)
		if err != nil {
			return nil, fmt.Errorf("insert raw %s batch: %w", source, err)
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan raw id: %w", err)
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate raw ids: %w", err)
		}
		rows.Close()
	}
	if len(ids) != len(payloads) {
		return nil, fmt.Errorf("insert raw %s: got %d ids for %d payloads", source, len(ids), len(payloads))
	}
	return ids, nil
}
