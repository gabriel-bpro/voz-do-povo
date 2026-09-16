package sync_test

import (
	"context"
	"database/sql"
	"testing"

	"voz-do-povo/internal/camara"
	"voz-do-povo/internal/senado"
	"voz-do-povo/internal/sync"
	"voz-do-povo/internal/testharness"
)

func syncSenatorsFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	senators := loadFixture[[]senado.Senator](t, "senado-senators.json")
	if err := sync.SyncSenadoSenators(context.Background(), db, senators); err != nil {
		t.Fatalf("SyncSenadoSenators: %v", err)
	}
}

var senadoParties = map[int64]string{9001: "PT", 9002: "PSDB"}

// Seam 2: senator registry fixture → raw landed with provenance, canonical
// politicians created with Senado source ids. Idempotent across runs.
func TestSyncSenadoSenators(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	senators := loadFixture[[]senado.Senator](t, "senado-senators.json")

	for run := 1; run <= 2; run++ {
		if err := sync.SyncSenadoSenators(ctx, db, senators); err != nil {
			t.Fatalf("run %d: SyncSenadoSenators: %v", run, err)
		}

		if n := queryInt(t, db,
			`SELECT count(*) FROM raw.payload WHERE source = 'senado-senators'`); n != 2 {
			t.Errorf("run %d: raw rows = %d, want 2 (staging replaced, not appended)", run, n)
		}
		if n := queryInt(t, db, `SELECT count(*) FROM curated.politician`); n != 2 {
			t.Errorf("run %d: politicians = %d, want 2", run, n)
		}
		if n := queryInt(t, db,
			`SELECT count(*) FROM curated.politician_source_id WHERE source = 'senado'`); n != 2 {
			t.Errorf("run %d: source ids = %d, want 2", run, n)
		}
	}

	var nameNorm string
	if err := db.QueryRowContext(ctx, `
		SELECT p.name_norm
		FROM curated.politician p
		JOIN curated.politician_source_id s ON s.politician_id = p.id
		WHERE s.source = 'senado' AND s.source_id = '9001'`).Scan(&nameNorm); err != nil {
		t.Fatalf("lookup maria: %v", err)
	}
	if nameNorm != "MARIA EXEMPLO" {
		t.Errorf("name_norm = %q, want %q", nameNorm, "MARIA EXEMPLO")
	}
}

// Seam 2, cross-house: a sitting senator with the same normalized name + UF
// as a deputada is NOT auto-merged (the Senado publishes no CPF) — the match
// is queued for manual review and the senator gets her own politician record
// with the Senado source id. The queue dedups across runs.
func TestSyncSenadoSenatorsCrossHouseReview(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)
	senators := loadFixture[[]senado.Senator](t, "senado-senators.json")

	for run := 1; run <= 2; run++ {
		if err := sync.SyncSenadoSenators(ctx, db, senators); err != nil {
			t.Fatalf("run %d: SyncSenadoSenators: %v", run, err)
		}

		// Maria exists twice (deputada + senator candidate) pending review;
		// João + Ana are house-only.
		if n := queryInt(t, db, `SELECT count(*) FROM curated.politician`); n != 4 {
			t.Errorf("run %d: politicians = %d, want 4 (no auto-merge)", run, n)
		}
		if n := queryInt(t, db,
			`SELECT count(*) FROM curated.identity_link_review WHERE status = 'pending'`); n != 1 {
			t.Errorf("run %d: pending review rows = %d, want 1 (deduplicated)", run, n)
		}
	}

	mariaCamaraID := queryInt(t, db,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = 'camara' AND source_id = '204001'`)
	mariaSenadoID := queryInt(t, db,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = 'senado' AND source_id = '9001'`)
	if mariaCamaraID == mariaSenadoID {
		t.Errorf("maria deputada and maria senadora share politician %d — fuzzy must never auto-link", mariaCamaraID)
	}

	var reason string
	var candidate int64
	if err := db.QueryRowContext(ctx, `
		SELECT reason, candidate_politician_id
		FROM curated.identity_link_review WHERE status = 'pending'`).Scan(&reason, &candidate); err != nil {
		t.Fatalf("lookup review row: %v", err)
	}
	if reason != "fuzzy-name-uf-match" {
		t.Errorf("reason = %q, want fuzzy-name-uf-match", reason)
	}
	if candidate != mariaCamaraID {
		t.Errorf("candidate = %d, want the deputada %d", candidate, mariaCamaraID)
	}
}

// Seam 2: CEAPS fixture → curated expenses with house='senado', linked via
// codSenador against the registry-built source-id map; unknown codes stay
// unlinked; party comes from the registry sigla; upsert (not duplicate) on
// re-run.
func TestSyncSenadoExpenses(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncSenatorsFixture(t, db)
	expenses := loadFixture[[]senado.Expense](t, "senado-ceaps.json")

	for run := 1; run <= 2; run++ {
		stats, err := sync.SyncSenadoExpenses(ctx, db, sync.SourceSenadoCEAPSYear+"2026", 2026, expenses, senadoParties)
		if err != nil {
			t.Fatalf("run %d: sync: %v", run, err)
		}
		if stats.Rows != 3 || stats.Linked != 2 || stats.Unlinked != 1 {
			t.Errorf("run %d: stats = %+v, want 3 rows: 2 linked, 1 unlinked", run, stats)
		}
		if n := queryInt(t, db,
			`SELECT count(*) FROM curated.expense WHERE house = 'senado'`); n != 3 {
			t.Errorf("run %d: expenses = %d, want 3 (upsert, not duplicate)", run, n)
		}
		if n := queryInt(t, db,
			`SELECT count(*) FROM raw.payload WHERE source = 'senado-ceaps-2026'`); n != 3 {
			t.Errorf("run %d: raw rows = %d, want 3 (staging replaced)", run, n)
		}
	}

	mariaID := queryInt(t, db,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = 'senado' AND source_id = '9001'`)
	anaID := queryInt(t, db,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = 'senado' AND source_id = '9002'`)

	linkOf := func(docID string) sql.NullInt64 {
		t.Helper()
		var pid sql.NullInt64
		if err := db.QueryRowContext(ctx,
			`SELECT politician_id FROM curated.expense WHERE house = 'senado' AND source_document_id = $1`, docID).
			Scan(&pid); err != nil {
			t.Fatalf("lookup expense %s: %v", docID, err)
		}
		return pid
	}

	if got := linkOf("77001"); !got.Valid || got.Int64 != mariaID {
		t.Errorf("77001 politician_id = %v, want %d (codSenador auto-link)", got, mariaID)
	}
	if got := linkOf("77002"); !got.Valid || got.Int64 != anaID {
		t.Errorf("77002 politician_id = %v, want %d", got, anaID)
	}
	// Senator absent from the registry (e.g. left office): unlinked.
	if got := linkOf("77003"); got.Valid {
		t.Errorf("77003 politician_id = %v, want NULL", got)
	}

	// Promoted fields + provenance.
	var (
		partySigla    string
		supplierTaxID string
		amountNet     float64
		rawID         int64
	)
	if err := db.QueryRowContext(ctx, `
		SELECT pa.sigla, e.supplier_tax_id, e.amount_net, e.raw_id
		FROM curated.expense e
		JOIN curated.party pa ON pa.id = e.party_id
		WHERE e.house = 'senado' AND e.source_document_id = '77001'`).
		Scan(&partySigla, &supplierTaxID, &amountNet, &rawID); err != nil {
		t.Fatalf("lookup expense 77001: %v", err)
	}
	if partySigla != "PT" {
		t.Errorf("party = %q, want PT (from the registry sigla)", partySigla)
	}
	if supplierTaxID != "05120923000109" {
		t.Errorf("supplier_tax_id = %q, want digits only", supplierTaxID)
	}
	if amountNet != 1315.35 {
		t.Errorf("amount_net = %v, want 1315.35", amountNet)
	}
	if rawID == 0 {
		t.Errorf("raw_id not set — provenance missing")
	}
}

// Seam 2: the CEAPS re-ingest reconciles against ground truth — a corrected
// amount lands as a new register, rows absent from the pull are deleted, and
// Câmara rows are untouched (reconcile is per house).
func TestSyncSenadoExpensesReconcile(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)
	camaraBulk := loadFixture[[]camara.Expense](t, "camara-expenses-bulk.json")
	if _, err := sync.SyncCamaraExpenses(ctx, db, sync.SourceCamaraCEAPBulk+"2026", 2026, camaraBulk); err != nil {
		t.Fatalf("camara sync: %v", err)
	}
	syncSenatorsFixture(t, db)

	expenses := loadFixture[[]senado.Expense](t, "senado-ceaps.json")
	if _, err := sync.SyncSenadoExpenses(ctx, db, sync.SourceSenadoCEAPSYear+"2026", 2026, expenses, senadoParties); err != nil {
		t.Fatalf("sync: %v", err)
	}

	updated := loadFixture[[]senado.Expense](t, "senado-ceaps-updated.json")
	stats, err := sync.SyncSenadoExpenses(ctx, db, sync.SourceSenadoCEAPSYear+"2026", 2026, updated, senadoParties)
	if err != nil {
		t.Fatalf("re-ingest: %v", err)
	}
	if stats.ReconciledOut != 3 {
		t.Errorf("reconciled out = %d, want 3 (all stale senado registers deleted)", stats.ReconciledOut)
	}

	var amount float64
	if err := db.QueryRowContext(ctx,
		`SELECT amount_net FROM curated.expense WHERE house = 'senado' AND source_document_id = '77001'`).Scan(&amount); err != nil {
		t.Fatalf("lookup 77001 after reconcile: %v", err)
	}
	if amount != 1400.00 {
		t.Errorf("77001 amount_net = %v, want 1400.00 after reconcile", amount)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.expense WHERE house = 'senado'`); n != 1 {
		t.Errorf("senado expenses after reconcile = %d, want 1", n)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.expense WHERE house = 'camara'`); n != 4 {
		t.Errorf("camara expenses after senado reconcile = %d, want 4 (untouched)", n)
	}
}

// Seam 2: a failing senado sync rolls back — previously curated data is
// untouched.
func TestFailedSenadoSyncLeavesCuratedUntouched(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncSenatorsFixture(t, db)

	good := loadFixture[[]senado.Expense](t, "senado-ceaps.json")
	if _, err := sync.SyncSenadoExpenses(ctx, db, sync.SourceSenadoCEAPSYear+"2026", 2026, good, senadoParties); err != nil {
		t.Fatalf("good sync: %v", err)
	}

	bad := append([]senado.Expense{}, good...)
	bad[0].Category = "" // invalid: no category
	if _, err := sync.SyncSenadoExpenses(ctx, db, sync.SourceSenadoCEAPSYear+"2026", 2026, bad, senadoParties); err == nil {
		t.Fatal("sync with invalid row should fail")
	}

	if n := queryInt(t, db,
		`SELECT count(*) FROM curated.expense WHERE house = 'senado'`); n != 3 {
		t.Errorf("expenses after failed sync = %d, want 3 (rollback)", n)
	}
}
