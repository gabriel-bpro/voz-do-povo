package sync_test

import (
	"context"
	"encoding/json"
	"testing"

	"voz-do-povo/internal/sync"
	"voz-do-povo/internal/testharness"
)

// Seam 2: feed a recorded source fixture into the sync entry point and assert
// the resulting raw table state (source rows verbatim, with provenance).
func TestIngestRawLandsPayloadsWithProvenance(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()

	fixture := testharness.Fixture(t, "ceap-expense-sample.json")
	var payloads []json.RawMessage
	if err := json.Unmarshal(fixture, &payloads); err != nil {
		t.Fatalf("fixture is not a JSON array: %v", err)
	}

	n, err := sync.IngestRaw(ctx, db, "camara-ceap", payloads)
	if err != nil {
		t.Fatalf("IngestRaw: %v", err)
	}
	if n != len(payloads) {
		t.Errorf("landed %d rows, want %d", n, len(payloads))
	}

	rows, err := db.QueryContext(ctx,
		`SELECT source, payload, pulled_at IS NOT NULL FROM raw.payload WHERE source = $1 ORDER BY id`,
		"camara-ceap")
	if err != nil {
		t.Fatalf("query raw.payload: %v", err)
	}
	defer rows.Close()

	var got []json.RawMessage
	for rows.Next() {
		var source string
		var payload json.RawMessage
		var hasPulledAt bool
		if err := rows.Scan(&source, &payload, &hasPulledAt); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if source != "camara-ceap" {
			t.Errorf("source = %q, want %q", source, "camara-ceap")
		}
		if !hasPulledAt {
			t.Errorf("pulled_at is null — provenance missing")
		}
		got = append(got, payload)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	if len(got) != len(payloads) {
		t.Fatalf("raw.payload has %d rows for source, want %d", len(got), len(payloads))
	}
	for i := range payloads {
		var wantJSON, gotJSON any
		if err := json.Unmarshal(payloads[i], &wantJSON); err != nil {
			t.Fatalf("fixture row %d: %v", i, err)
		}
		if err := json.Unmarshal(got[i], &gotJSON); err != nil {
			t.Fatalf("stored row %d is not JSON: %v", i, err)
		}
		wantCanon, _ := json.Marshal(wantJSON)
		gotCanon, _ := json.Marshal(gotJSON)
		if string(gotCanon) != string(wantCanon) {
			t.Errorf("row %d payload mutated:\n got: %s\nwant: %s", i, gotCanon, wantCanon)
		}
	}
}
