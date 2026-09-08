package sync_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"voz-do-povo/internal/camara"
	"voz-do-povo/internal/sync"
	"voz-do-povo/internal/testharness"
)

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(testharness.Fixture(t, name), &v); err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return v
}

func syncDeputiesFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	deputies := loadFixture[[]camara.Deputy](t, "camara-deputies.json")
	if err := sync.SyncCamaraDeputies(context.Background(), db, deputies); err != nil {
		t.Fatalf("SyncCamaraDeputies: %v", err)
	}
}

func queryInt(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return n
}

// Seam 2: deputy registry fixture → raw landed with provenance, canonical
// politicians created with source ids. Idempotent across runs.
func TestSyncCamaraDeputies(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	deputies := loadFixture[[]camara.Deputy](t, "camara-deputies.json")

	for run := 1; run <= 2; run++ {
		if err := sync.SyncCamaraDeputies(ctx, db, deputies); err != nil {
			t.Fatalf("run %d: SyncCamaraDeputies: %v", run, err)
		}

		if n := queryInt(t, db,
			`SELECT count(*) FROM raw.payload WHERE source = 'camara-deputies'`); n != 2 {
			t.Errorf("run %d: raw rows = %d, want 2 (staging replaced, not appended)", run, n)
		}
		if n := queryInt(t, db,
			`SELECT count(*) FROM raw.payload WHERE source = 'camara-deputies' AND pulled_at IS NOT NULL`); n != 2 {
			t.Errorf("run %d: raw rows missing provenance", run)
		}
		if n := queryInt(t, db, `SELECT count(*) FROM curated.politician`); n != 2 {
			t.Errorf("run %d: politicians = %d, want 2", run, n)
		}
		if n := queryInt(t, db,
			`SELECT count(*) FROM curated.politician_source_id WHERE source = 'camara'`); n != 2 {
			t.Errorf("run %d: source ids = %d, want 2", run, n)
		}
	}

	var nameNorm, cpf string
	if err := db.QueryRowContext(ctx, `
		SELECT p.name_norm, p.cpf
		FROM curated.politician p
		JOIN curated.politician_source_id s ON s.politician_id = p.id
		WHERE s.source = 'camara' AND s.source_id = '204001'`).Scan(&nameNorm, &cpf); err != nil {
		t.Fatalf("lookup maria: %v", err)
	}
	if nameNorm != "MARIA EXEMPLO" {
		t.Errorf("name_norm = %q, want %q", nameNorm, "MARIA EXEMPLO")
	}
	if cpf != "12345678901" {
		t.Errorf("cpf = %q, want %q", cpf, "12345678901")
	}
}

// Seam 2: API-delta expense fixture → curated expenses linked via source id,
// provenance pointer set, upsert (not duplicate) on re-run.
func TestSyncCamaraExpensesDelta(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)
	expenses := loadFixture[[]camara.Expense](t, "camara-expenses-api.json")

	for run := 1; run <= 2; run++ {
		stats, err := sync.SyncCamaraExpenses(ctx, db, sync.SourceCamaraCEAPDelta, 0, expenses)
		if err != nil {
			t.Fatalf("run %d: SyncCamaraExpenses: %v", run, err)
		}
		if stats.Rows != 2 || stats.Linked != 2 || stats.Unlinked != 0 {
			t.Errorf("run %d: stats = %+v, want 2 rows all linked", run, stats)
		}
		if n := queryInt(t, db, `SELECT count(*) FROM curated.expense`); n != 2 {
			t.Errorf("run %d: expenses = %d, want 2 (upsert, not duplicate)", run, n)
		}
	}

	var (
		parliamentaryName string
		partySigla        string
		supplierTaxID     string
		amountNet         float64
		rawID             int64
	)
	if err := db.QueryRowContext(ctx, `
		SELECT p.parliamentary_name, pa.sigla, e.supplier_tax_id, e.amount_net, e.raw_id
		FROM curated.expense e
		JOIN curated.politician p ON p.id = e.politician_id
		JOIN curated.party pa ON pa.id = e.party_id
		WHERE e.house = 'camara' AND e.source_document_id = '7700001'`).
		Scan(&parliamentaryName, &partySigla, &supplierTaxID, &amountNet, &rawID); err != nil {
		t.Fatalf("lookup expense 7700001: %v", err)
	}
	if parliamentaryName != "MARIA EXEMPLO" {
		t.Errorf("politician = %q, want MARIA EXEMPLO", parliamentaryName)
	}
	if partySigla != "PT" {
		t.Errorf("party = %q, want PT", partySigla)
	}
	if supplierTaxID != "12345678000190" {
		t.Errorf("supplier_tax_id = %q, want digits only", supplierTaxID)
	}
	if amountNet != 2140.55 {
		t.Errorf("amount_net = %v, want 2140.55", amountNet)
	}
	if rawID == 0 {
		t.Errorf("raw_id not set — provenance missing")
	}
}

// Seam 2: bulk year fixture → identity linkage outcomes (source-id auto-link,
// CPF auto-link, fuzzy → review queue, no match → unlinked), then the weekly
// re-ingest reconciles: updated amounts land, vanished rows are deleted, and
// the review queue is not spammed.
func TestSyncCamaraExpensesBulkReconcile(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)
	bulk := loadFixture[[]camara.Expense](t, "camara-expenses-bulk.json")

	stats, err := sync.SyncCamaraExpenses(ctx, db, sync.SourceCamaraCEAPBulk+"2026", 2026, bulk)
	if err != nil {
		t.Fatalf("bulk sync: %v", err)
	}
	if stats.Rows != 4 || stats.Linked != 2 || stats.Unlinked != 2 {
		t.Errorf("stats = %+v, want 4 rows: 2 linked, 2 unlinked", stats)
	}

	linkOf := func(docID string) sql.NullInt64 {
		t.Helper()
		var pid sql.NullInt64
		if err := db.QueryRowContext(ctx,
			`SELECT politician_id FROM curated.expense WHERE source_document_id = $1`, docID).
			Scan(&pid); err != nil {
			t.Fatalf("lookup expense %s: %v", docID, err)
		}
		return pid
	}
	mariaID := queryInt(t, db,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = 'camara' AND source_id = '204001'`)
	joaoID := queryInt(t, db,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = 'camara' AND source_id = '204002'`)

	// Known nuDeputadoID → auto-link via source id.
	if got := linkOf("8800001"); !got.Valid || got.Int64 != mariaID {
		t.Errorf("8800001 politician_id = %v, want %d", got, mariaID)
	}
	// nuDeputadoID absent (bulk deviation), name matches fuzzily → queued,
	// NOT linked.
	if got := linkOf("8800002"); got.Valid {
		t.Errorf("8800002 politician_id = %v, want NULL (fuzzy must never force-link)", got)
	}
	if n := queryInt(t, db, `
		SELECT count(*) FROM curated.identity_link_review
		WHERE status = 'pending' AND candidate_politician_id = $1`, mariaID); n != 1 {
		t.Errorf("review queue rows for maria = %d, want 1", n)
	}
	// nuDeputadoID absent but CPF present → CPF-first auto-link, even though
	// the name doesn't match.
	if got := linkOf("8800003"); !got.Valid || got.Int64 != joaoID {
		t.Errorf("8800003 politician_id = %v, want %d (CPF auto-link)", got, joaoID)
	}
	// Unknown id and name → unlinked, no queue row.
	if got := linkOf("8800004"); got.Valid {
		t.Errorf("8800004 politician_id = %v, want NULL", got)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.identity_link_review`); n != 1 {
		t.Errorf("review queue rows = %d, want exactly 1", n)
	}

	// Weekly re-ingest: 8800001's amount changed, the other rows vanished
	// from the file.
	updated := loadFixture[[]camara.Expense](t, "camara-expenses-bulk-updated.json")
	stats, err = sync.SyncCamaraExpenses(ctx, db, sync.SourceCamaraCEAPBulk+"2026", 2026, updated)
	if err != nil {
		t.Fatalf("bulk re-ingest: %v", err)
	}
	if stats.ReconciledOut != 3 {
		t.Errorf("reconciled out = %d, want 3 (vanished rows deleted)", stats.ReconciledOut)
	}

	var amount float64
	if err := db.QueryRowContext(ctx,
		`SELECT amount_net FROM curated.expense WHERE source_document_id = '8800001'`).Scan(&amount); err != nil {
		t.Fatalf("lookup 8800001 after reconcile: %v", err)
	}
	if amount != 150.00 {
		t.Errorf("8800001 amount_net = %v, want 150.00 after reconcile", amount)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.expense`); n != 1 {
		t.Errorf("expenses after reconcile = %d, want 1", n)
	}
	if n := queryInt(t, db,
		`SELECT count(*) FROM raw.payload WHERE source = 'camara-ceap-bulk-2026'`); n != 1 {
		t.Errorf("raw rows after reconcile = %d, want 1 (staging replaced)", n)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.identity_link_review`); n != 1 {
		t.Errorf("review queue rows after re-ingest = %d, want 1 (deduplicated)", n)
	}
}

// Seam 2: a failing sync rolls back — previously curated data is untouched.
func TestFailedSyncLeavesCuratedUntouched(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)

	good := loadFixture[[]camara.Expense](t, "camara-expenses-api.json")
	if _, err := sync.SyncCamaraExpenses(ctx, db, sync.SourceCamaraCEAPDelta, 0, good); err != nil {
		t.Fatalf("good sync: %v", err)
	}

	bad := append([]camara.Expense{}, good...)
	bad[0].DocumentID = "" // invalid: no dedup key
	if _, err := sync.SyncCamaraExpenses(ctx, db, sync.SourceCamaraCEAPDelta, 0, bad); err == nil {
		t.Fatal("sync with invalid row should fail")
	}

	if n := queryInt(t, db, `SELECT count(*) FROM curated.expense`); n != 2 {
		t.Errorf("expenses after failed sync = %d, want 2 (rollback)", n)
	}
	var amount float64
	if err := db.QueryRowContext(ctx,
		`SELECT amount_net FROM curated.expense WHERE source_document_id = '7700001'`).Scan(&amount); err != nil {
		t.Fatalf("lookup 7700001: %v", err)
	}
	if amount != 2140.55 {
		t.Errorf("7700001 amount_net = %v, want 2140.55 (untouched)", amount)
	}
}
