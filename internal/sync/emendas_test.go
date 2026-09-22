package sync_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"voz-do-povo/internal/sync"
	"voz-do-povo/internal/testharness"
	"voz-do-povo/internal/transparencia"
)

// loadEmendasFixture parses a recorded spreadsheet fixture through the real
// adapter parser — the seam-2 entry is the sync, but the parse is part of the
// recorded-payload path (source adapters never touch the network in tests).
func loadEmendasFixture(t *testing.T, name string) []transparencia.Emenda {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer f.Close()
	emendas, err := transparencia.ParseEmendasCSV(f)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return emendas
}

// Seam 2: emendas spreadsheet fixture → raw landed with provenance, curated
// rows upserted on the emenda code, identity linked by name (exact, cargo-
// prefix-stripped, and the high-threshold similarity fallback), PIX
// classified as a permanent type marker. Idempotent across runs.
func TestSyncEmendas(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)
	emendas := loadEmendasFixture(t, "transparencia-emendas.csv")

	for run := 1; run <= 2; run++ {
		stats, err := sync.SyncEmendas(ctx, db, sync.SourceTransparenciaEmendasYear+"2026", 2026, emendas)
		if err != nil {
			t.Fatalf("run %d: SyncEmendas: %v", run, err)
		}
		if stats.Rows != 5 || stats.Linked != 3 || stats.Unlinked != 2 {
			t.Errorf("run %d: stats = %+v, want 5 rows: 3 linked, 2 unlinked", run, stats)
		}
		if n := queryInt(t, db, `SELECT count(*) FROM curated.emenda`); n != 5 {
			t.Errorf("run %d: emendas = %d, want 5 (upsert, not duplicate)", run, n)
		}
		if n := queryInt(t, db,
			`SELECT count(*) FROM raw.payload WHERE source = 'transparencia-emendas-2026'`); n != 5 {
			t.Errorf("run %d: raw rows = %d, want 5 (staging replaced)", run, n)
		}
	}

	mariaID := queryInt(t, db,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = 'camara' AND source_id = '204001'`)
	joaoID := queryInt(t, db,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = 'camara' AND source_id = '204002'`)

	linkOf := func(code string) sql.NullInt64 {
		t.Helper()
		var pid sql.NullInt64
		if err := db.QueryRowContext(ctx,
			`SELECT politician_id FROM curated.emenda WHERE code = $1`, code).Scan(&pid); err != nil {
			t.Fatalf("lookup emenda %s: %v", code, err)
		}
		return pid
	}

	if got := linkOf("2026E0001"); !got.Valid || got.Int64 != mariaID {
		t.Errorf("2026E0001 politician_id = %v, want %d (exact name)", got, mariaID)
	}
	if got := linkOf("2026E0003"); !got.Valid || got.Int64 != joaoID {
		t.Errorf("2026E0003 politician_id = %v, want %d (cargo prefix stripped)", got, joaoID)
	}
	// One char off a short name ≈ 0.81 similarity: below the 0.90 floor,
	// unlinked. The fallback's linking side is covered by
	// TestSyncEmendasSimilarityFallback (long-name pair ≈ 0.97).
	if got := linkOf("2026E0005"); got.Valid {
		t.Errorf("2026E0005 politician_id = %v, want NULL (below the similarity floor)", got)
	}
	// No exact match anywhere near the floor: unlinked, nothing queued.
	if got := linkOf("2026E0004"); got.Valid {
		t.Errorf("2026E0004 politician_id = %v, want NULL", got)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.identity_link_review`); n != 0 {
		t.Errorf("review queue = %d, want 0 (no ties, no collisions)", n)
	}

	// PIX classification: transferência especial is the permanent type marker.
	var pix map[string]bool = map[string]bool{}
	rows, err := db.QueryContext(ctx, `SELECT code, is_pix FROM curated.emenda`)
	if err != nil {
		t.Fatalf("query is_pix: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var p bool
		if err := rows.Scan(&code, &p); err != nil {
			t.Fatalf("scan is_pix: %v", err)
		}
		pix[code] = p
	}
	if !pix["2026E0002"] {
		t.Errorf("2026E0002 (Transferência Especial) is_pix = false, want true")
	}
	for _, code := range []string{"2026E0001", "2026E0003", "2026E0004", "2026E0005"} {
		if pix[code] {
			t.Errorf("%s is_pix = true, want false", code)
		}
	}

	// Promoted fields: pt-BR amounts parsed, latin-1 decoded, provenance set.
	var (
		localidade, funcao string
		empenhado, pago    float64
		rawID              int64
	)
	if err := db.QueryRowContext(ctx, `
		SELECT localidade, funcao, amount_empenhado, amount_pago, raw_id
		FROM curated.emenda WHERE code = '2026E0001'`).
		Scan(&localidade, &funcao, &empenhado, &pago, &rawID); err != nil {
		t.Fatalf("lookup 2026E0001: %v", err)
	}
	if localidade != "Taubaté - SP" {
		t.Errorf("localidade = %q, want %q (latin-1 decoded)", localidade, "Taubaté - SP")
	}
	if funcao != "Saúde" {
		t.Errorf("funcao = %q, want Saúde", funcao)
	}
	if empenhado != 1500000.00 || pago != 600000.00 {
		t.Errorf("amounts = %v/%v, want 1500000/600000 (pt-BR parsed)", empenhado, pago)
	}
	if rawID == 0 {
		t.Errorf("raw_id not set — provenance missing")
	}

	// Negative restatement with a floating sign ("- 100.000,00"), seen on
	// live data: parsed, stored negative.
	var negEmpenhado float64
	if err := db.QueryRowContext(ctx,
		`SELECT amount_empenhado FROM curated.emenda WHERE code = '2026E0004'`).Scan(&negEmpenhado); err != nil {
		t.Fatalf("lookup 2026E0004: %v", err)
	}
	if negEmpenhado != -100000.00 {
		t.Errorf("2026E0004 empenhado = %v, want -100000 (floating-sign negative)", negEmpenhado)
	}
}

// Seam 2: the weekly re-ingest reconciles against ground truth — restated
// execution values update in place, emendas absent from the pull are deleted.
func TestSyncEmendasReconcile(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)

	emendas := loadEmendasFixture(t, "transparencia-emendas.csv")
	if _, err := sync.SyncEmendas(ctx, db, sync.SourceTransparenciaEmendasYear+"2026", 2026, emendas); err != nil {
		t.Fatalf("sync: %v", err)
	}

	updated := loadEmendasFixture(t, "transparencia-emendas-updated.csv")
	stats, err := sync.SyncEmendas(ctx, db, sync.SourceTransparenciaEmendasYear+"2026", 2026, updated)
	if err != nil {
		t.Fatalf("re-ingest: %v", err)
	}
	if stats.ReconciledOut != 1 {
		t.Errorf("reconciled out = %d, want 1 (2026E0002 absent from the pull)", stats.ReconciledOut)
	}

	var liquidado, pago float64
	if err := db.QueryRowContext(ctx,
		`SELECT amount_liquidado, amount_pago FROM curated.emenda WHERE code = '2026E0001'`).
		Scan(&liquidado, &pago); err != nil {
		t.Fatalf("lookup 2026E0001 after reconcile: %v", err)
	}
	if liquidado != 900000.00 || pago != 700000.00 {
		t.Errorf("2026E0001 execution = %v/%v, want 900000/700000 after restatement", liquidado, pago)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.emenda`); n != 4 {
		t.Errorf("emendas after reconcile = %d, want 4", n)
	}
}

// Seam 2: a failing emendas sync rolls back — previously curated data is
// untouched.
func TestFailedEmendaSyncLeavesCuratedUntouched(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)

	good := loadEmendasFixture(t, "transparencia-emendas.csv")
	if _, err := sync.SyncEmendas(ctx, db, sync.SourceTransparenciaEmendasYear+"2026", 2026, good); err != nil {
		t.Fatalf("good sync: %v", err)
	}

	bad := append([]transparencia.Emenda{}, good...)
	bad[0].Code = "" // invalid: no natural key
	if _, err := sync.SyncEmendas(ctx, db, sync.SourceTransparenciaEmendasYear+"2026", 2026, bad); err == nil {
		t.Fatal("sync with invalid row should fail")
	}

	if n := queryInt(t, db, `SELECT count(*) FROM curated.emenda`); n != 5 {
		t.Errorf("emendas after failed sync = %d, want 5 (rollback)", n)
	}
}

// Seam 2, identity: two canonical politicians with the same normalized name
// never get the author auto-linked — the collision goes to the review queue.
func TestSyncEmendasNameCollisionQueues(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()

	for _, cpf := range []string{"11111111111", "22222222222"} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO curated.politician (civil_name, parliamentary_name, name_norm, uf, cpf)
			VALUES ('Maria Exemplo', 'MARIA EXEMPLO', 'MARIA EXEMPLO', 'SP', $1)`, cpf); err != nil {
			t.Fatalf("seed colliding politician: %v", err)
		}
	}

	emendas := loadEmendasFixture(t, "transparencia-emendas.csv")
	stats, err := sync.SyncEmendas(ctx, db, sync.SourceTransparenciaEmendasYear+"2026", 2026, emendas)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	// Nothing links: Maria's rows collide (queued), João doesn't exist here,
	// the unknown author and the below-floor near-match stay unlinked.
	if stats.Linked != 0 || stats.Unlinked != 5 {
		t.Errorf("stats = %+v, want 0 linked, 5 unlinked", stats)
	}
	if n := queryInt(t, db,
		`SELECT count(*) FROM curated.identity_link_review WHERE reason = 'emenda-author-name-collision'`); n != 2 {
		t.Errorf("collision review rows = %d, want 2 (both candidates queued)", n)
	}
}

// Seam 2, identity: a long author name with a one-char typo clears the 0.90
// similarity floor and auto-links to the single closest match.
func TestSyncEmendasSimilarityFallback(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()

	var pid int64
	if err := db.QueryRowContext(ctx, `
		INSERT INTO curated.politician (civil_name, parliamentary_name, name_norm, uf, cpf)
		VALUES ('Francisco Ferreira da Silva Junior', 'FRANCISCO FERREIRA DA SILVA JUNIOR',
		        'FRANCISCO FERREIRA DA SILVA JUNIOR', 'BA', '33333333333')
		RETURNING id`).Scan(&pid); err != nil {
		t.Fatalf("seed politician: %v", err)
	}

	emendas := []transparencia.Emenda{{
		Code: "2026E0099", Year: 2026, Tipo: "Emenda Individual",
		// One char off: pg_trgm similarity ≈ 0.917, above the 0.90 floor.
		AuthorName: "FRANCISCO FERREIRA DA SILVAA JUNIOR",
		Localidade: "Salvador - BA", Funcao: "Saúde",
		Empenhado: 100000, Pago: 0,
	}}
	stats, err := sync.SyncEmendas(ctx, db, sync.SourceTransparenciaEmendasYear+"2026", 2026, emendas)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if stats.Linked != 1 {
		t.Errorf("stats = %+v, want 1 linked (similarity fallback)", stats)
	}
	var got int64
	if err := db.QueryRowContext(ctx,
		`SELECT politician_id FROM curated.emenda WHERE code = '2026E0099'`).Scan(&got); err != nil {
		t.Fatalf("lookup 2026E0099: %v", err)
	}
	if got != pid {
		t.Errorf("2026E0099 politician_id = %d, want %d", got, pid)
	}
}
