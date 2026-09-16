package httpapi_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"voz-do-povo/internal/httpapi"
	"voz-do-povo/internal/sync"
	"voz-do-povo/internal/testharness"
)

// Curated-layer seed helpers for the seam-1 tests: straight SQL into the
// curated tables, exactly the state a completed sync would leave.

func newSeededAPI(t *testing.T) (*sql.DB, *httptest.Server) {
	t.Helper()
	db := testharness.TestDB(t)
	srv := httptest.NewServer(httpapi.New(db))
	t.Cleanup(srv.Close)
	return db, srv
}

func seedPolitician(t *testing.T, db *sql.DB, id int64, civilName, parlName, uf, cpf string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO curated.politician (id, civil_name, parliamentary_name, name_norm, uf, cpf)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, civilName, parlName, sync.NormalizeName(parlName), uf, cpf)
	if err != nil {
		t.Fatalf("seed politician %d: %v", id, err)
	}
}

func seedSourceID(t *testing.T, db *sql.DB, politicianID int64, source, sourceID string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO curated.politician_source_id (politician_id, source, source_id)
		VALUES ($1, $2, $3)`, politicianID, source, sourceID)
	if err != nil {
		t.Fatalf("seed source id: %v", err)
	}
}

func seedPartyRow(t *testing.T, db *sql.DB, sigla string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO curated.party (sigla) VALUES ($1) RETURNING id`, sigla).Scan(&id); err != nil {
		t.Fatalf("seed party %q: %v", sigla, err)
	}
	return id
}

func seedExpense(t *testing.T, db *sql.DB, politicianID int64, partyID *int64, year, month int, category, supplier, issuedAt string, amountNet float64) {
	t.Helper()
	seedExpenseHouse(t, db, "camara", politicianID, partyID, year, month, category, supplier, issuedAt, amountNet)
}

func seedExpenseHouse(t *testing.T, db *sql.DB, house string, politicianID int64, partyID *int64, year, month int, category, supplier, issuedAt string, amountNet float64) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO curated.expense (
			politician_id, party_id, house, category, supplier_name, issued_at,
			year, month, amount_net, source_row_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		politicianID, partyID, house, category, supplier, issuedAt,
		year, month, amountNet,
		// Unique dedup key per seeded row.
		house+url.QueryEscape(supplier)+issuedAt)
	if err != nil {
		t.Fatalf("seed expense (%s/%s/%d/%d): %v", house, category, year, month, err)
	}
}

func getJSON(t *testing.T, url string) *http.Response {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return res
}

func mustStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		t.Fatalf("GET %s: status = %d, want %d", res.Request.URL, res.StatusCode, want)
	}
}

func mustDecode(t *testing.T, res *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("decode %s body: %v", res.Request.URL, err)
	}
}

func urlQuery(s string) string {
	return url.QueryEscape(s)
}
