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

// Seam 2 (ticket 19): recorded vote/attendance fixtures → sync entry points
// → assert raw + curated table state.

func syncCamaraVotesFixture(t *testing.T, db *sql.DB) sync.VoteSyncStats {
	t.Helper()
	events := loadFixture[[]camara.VoteEvent](t, "camara-votacoes.json")
	casts := loadFixture[[]camara.VoteCast](t, "camara-votos.json")
	orientations := loadFixture[[]camara.VoteOrientation](t, "camara-orientacoes.json")
	stats, err := sync.SyncCamaraVotes(context.Background(), db, sync.SourceCamaraVotacoesYear, 2026, events, casts, orientations)
	if err != nil {
		t.Fatalf("SyncCamaraVotes: %v", err)
	}
	return stats
}

// Câmara votes: casts linked via the deputy's API id, description from the
// vote event, bloc/empty orientations dropped, sessions only for plenary
// events behind nominal votes. Idempotent; the year reconcile removes rows
// absent from a re-pull.
func TestSyncCamaraVotes(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)

	for run := 1; run <= 2; run++ {
		stats := syncCamaraVotesFixture(t, db)

		if stats.Votes != 4 || stats.Linked != 3 || stats.Unlinked != 1 {
			t.Errorf("run %d: stats = %+v, want 4 votes / 3 linked / 1 unlinked", run, stats)
		}
		if stats.Orientations != 2 {
			t.Errorf("run %d: orientations = %d, want 2 (bloc + empty dropped)", run, stats.Orientations)
		}
		if stats.Sessions != 2 {
			t.Errorf("run %d: sessions = %d, want 2 (74889 + 74900; comissão excluded)", run, stats.Sessions)
		}

		if n := queryInt(t, db, `SELECT count(*) FROM curated.vote WHERE house = 'camara'`); n != 4 {
			t.Errorf("run %d: votes = %d, want 4 (upsert, not dup)", run, n)
		}
		if n := queryInt(t, db, `SELECT count(*) FROM raw.payload WHERE source = 'camara-votacoes-votos-2026'`); n != 4 {
			t.Errorf("run %d: raw votos = %d, want 4 (staging replaced)", run, n)
		}
	}

	// Linkage + denormalized fields.
	var vote, party, desc string
	var politicianID *int64
	var approved bool
	if err := db.QueryRowContext(ctx, `
		SELECT politician_id, vote, party_sigla, description, approved
		FROM curated.vote
		WHERE house = 'camara' AND source_vote_id = '2585316-100' AND member_source_id = '204001'`).
		Scan(&politicianID, &vote, &party, &desc, &approved); err != nil {
		t.Fatalf("lookup maria vote: %v", err)
	}
	if politicianID == nil {
		t.Errorf("maria vote unlinked, want linked via deputy id 204001")
	}
	if vote != "Sim" || party != "PT" || !approved {
		t.Errorf("vote = %q party = %q approved = %v, want Sim/PT/true", vote, party, approved)
	}
	if desc != "Aprovado o Projeto de Lei nº 100, de 2026." {
		t.Errorf("description = %q, want the vote event's", desc)
	}
	if n := queryInt(t, db, `
		SELECT count(*) FROM curated.vote
		WHERE house = 'camara' AND member_source_id = '999999' AND politician_id IS NULL`); n != 1 {
		t.Errorf("unknown deputy vote linked or missing, want 1 unlinked row")
	}

	// Orientation join keys match the vote's source id.
	var orientation string
	if err := db.QueryRowContext(ctx, `
		SELECT orientation FROM curated.vote_orientation
		WHERE house = 'camara' AND source_vote_id = '2585316-100' AND party_sigla = 'PL'`).
		Scan(&orientation); err != nil {
		t.Fatalf("lookup PL orientation: %v", err)
	}
	if orientation != "Sim" {
		t.Errorf("PL orientation = %q, want Sim", orientation)
	}

	// Reconcile: re-pull without one cast → the stale vote goes.
	casts := loadFixture[[]camara.VoteCast](t, "camara-votos.json")
	events := loadFixture[[]camara.VoteEvent](t, "camara-votacoes.json")
	stats, err := sync.SyncCamaraVotes(ctx, db, sync.SourceCamaraVotacoesYear, 2026,
		events, casts[:3], nil)
	if err != nil {
		t.Fatalf("reconcile run: %v", err)
	}
	if stats.ReconciledOut == 0 {
		t.Errorf("reconcile removed nothing, want the dropped vote (and its session) out")
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.vote WHERE house = 'camara'`); n != 3 {
		t.Errorf("votes after reconcile = %d, want 3", n)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.vote_orientation WHERE house = 'camara'`); n != 0 {
		t.Errorf("orientations after reconcile = %d, want 0 (absent from the pull)", n)
	}
}

// Câmara attendance: presences promoted only for known plenary sessions
// (votes sync first), deduped per deputy+event; reconcile removes stale rows.
func TestSyncCamaraAttendance(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncDeputiesFixture(t, db)
	syncCamaraVotesFixture(t, db)

	presence := loadFixture[[]camara.PresenceRow](t, "camara-presenca.json")
	for run := 1; run <= 2; run++ {
		stats, err := sync.SyncCamaraAttendance(ctx, db, sync.SourceCamaraPresencaYear+"2026", 2026, presence)
		if err != nil {
			t.Fatalf("run %d: SyncCamaraAttendance: %v", run, err)
		}
		if stats.Rows != 3 || stats.Promoted != 1 || stats.Skipped != 1 {
			t.Errorf("run %d: stats = %+v, want 3 rows / 1 promoted (dedup) / 1 skipped (unknown event)", run, stats)
		}
		if n := queryInt(t, db, `SELECT count(*) FROM curated.attendance WHERE house = 'camara'`); n != 1 {
			t.Errorf("run %d: attendance rows = %d, want 1", run, n)
		}
	}

	var status string
	var politicianID *int64
	if err := db.QueryRowContext(ctx, `
		SELECT politician_id, status FROM curated.attendance
		WHERE house = 'camara' AND session_key = '74889'`).Scan(&politicianID, &status); err != nil {
		t.Fatalf("lookup attendance: %v", err)
	}
	if politicianID == nil || status != "present" {
		t.Errorf("attendance = (%v, %q), want linked present", politicianID, status)
	}

	// Reconcile: a re-pull without the row deletes it.
	stats, err := sync.SyncCamaraAttendance(ctx, db, sync.SourceCamaraPresencaYear+"2026", 2026, nil)
	if err != nil {
		t.Fatalf("reconcile run: %v", err)
	}
	if stats.ReconciledOut != 1 {
		t.Errorf("reconciled out = %d, want 1", stats.ReconciledOut)
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.attendance WHERE house = 'camara'`); n != 0 {
		t.Errorf("attendance after reconcile = %d, want 0", n)
	}
}

// Senado votes: roster codes double as the presence record — per-session
// attendance is the best code across the session's votes. Orientations join
// via VoteSeq translated to the vote's unique id; bloc orientations dropped.
// Secret votes land (vote='Votou') but carry no position.
func TestSyncSenadoVotes(t *testing.T) {
	db := testharness.TestDB(t)
	ctx := context.Background()
	syncSenatorsFixture(t, db)

	votes := loadFixture[[]senado.NominalVote](t, "senado-nominal.json")
	orientations := loadFixture[[]senado.VoteOrientation](t, "senado-orientacoes.json")

	for run := 1; run <= 2; run++ {
		stats, err := sync.SyncSenadoVotes(ctx, db, sync.SourceSenadoVotacoesYear, 2026, votes, orientations)
		if err != nil {
			t.Fatalf("run %d: SyncSenadoVotes: %v", run, err)
		}
		if stats.Votes != 7 || stats.Linked != 6 || stats.Unlinked != 1 {
			t.Errorf("run %d: stats = %+v, want 7 votes / 6 linked / 1 unlinked", run, stats)
		}
		if stats.Sessions != 3 || stats.Attendance != 5 {
			t.Errorf("run %d: sessions/attendance = %d/%d, want 3/5", run, stats.Sessions, stats.Attendance)
		}
		if stats.Orientations != 2 {
			t.Errorf("run %d: orientations = %d, want 2 (GOVERNO dropped)", run, stats.Orientations)
		}
		if n := queryInt(t, db, `SELECT count(*) FROM curated.vote WHERE house = 'senado'`); n != 7 {
			t.Errorf("run %d: votes = %d, want 7 (upsert, not dup)", run, n)
		}
		if n := queryInt(t, db, `SELECT count(*) FROM raw.payload WHERE source = 'senado-votacoes-nominal-2026'`); n != 3 {
			t.Errorf("run %d: raw nominal = %d, want 3 (staging replaced)", run, n)
		}
	}

	// Per-session attendance: best code wins (Sim > P-NRV; AP alone → justified).
	type att struct {
		status, detail string
	}
	got := map[string]att{}
	rows, err := db.QueryContext(ctx, `
		SELECT session_key || '|' || member_source_id, status, status_detail
		FROM curated.attendance WHERE house = 'senado'`)
	if err != nil {
		t.Fatalf("query attendance: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var a att
		var detail *string
		if err := rows.Scan(&k, &a.status, &detail); err != nil {
			t.Fatalf("scan attendance: %v", err)
		}
		if detail != nil {
			a.detail = *detail
		}
		got[k] = a
	}
	want := map[string]att{
		"550469|9001": {"present", "Sim"},  // Sim beats P-NRV across the session
		"550469|9002": {"present", "Não"},  // first present code of the session
		"550469|9003": {"justified", "AP"}, // unlinked senator still gets a row
		"550470|9001": {"justified", "AP"},
		"550470|9002": {"present", "Votou"}, // secret vote = present
	}
	for k, w := range want {
		if g, ok := got[k]; !ok || g != w {
			t.Errorf("attendance %s = %+v (found %v), want %+v", k, g, ok, w)
		}
	}

	// Session rows are the attendance denominator; one per CodigoSessao.
	if n := queryInt(t, db, `SELECT count(*) FROM curated.plenary_session WHERE house = 'senado'`); n != 2 {
		t.Errorf("senado sessions = %d, want 2", n)
	}

	// Orientation stored under the vote's unique id (translated from VoteSeq).
	var orientation string
	if err := db.QueryRowContext(ctx, `
		SELECT orientation FROM curated.vote_orientation
		WHERE house = 'senado' AND source_vote_id = '7047' AND party_sigla = 'PSDB'`).
		Scan(&orientation); err != nil {
		t.Fatalf("lookup PSDB orientation: %v", err)
	}
	if orientation != "Sim" {
		t.Errorf("PSDB orientation = %q, want Sim", orientation)
	}

	// Vote row: matter ref + result from the vote, not the roster.
	var matterRef string
	var approved bool
	if err := db.QueryRowContext(ctx, `
		SELECT matter_ref, approved FROM curated.vote
		WHERE house = 'senado' AND source_vote_id = '7047' AND member_source_id = '9001'`).
		Scan(&matterRef, &approved); err != nil {
		t.Fatalf("lookup maria senado vote: %v", err)
	}
	if matterRef != "PEC 22/2025" || !approved {
		t.Errorf("matter_ref/approved = %q/%v, want PEC 22/2025/true", matterRef, approved)
	}

	// Reconcile: a re-pull without the second session deletes its rows.
	stats, err := sync.SyncSenadoVotes(ctx, db, sync.SourceSenadoVotacoesYear, 2026, votes[:2], orientations)
	if err != nil {
		t.Fatalf("reconcile run: %v", err)
	}
	if stats.ReconciledOut == 0 {
		t.Errorf("reconcile removed nothing, want the dropped session's rows out")
	}
	if n := queryInt(t, db, `SELECT count(*) FROM curated.attendance WHERE house = 'senado'`); n != 3 {
		t.Errorf("attendance after reconcile = %d, want 3 (session 550470 gone)", n)
	}
}
