package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"voz-do-povo/internal/camara"
	"voz-do-povo/internal/senado"
)

// Raw source keys for the votes & attendance syncs (ticket 19). The votes
// syncs take a per-year prefix and compose one key per landed dataset
// ("<prefix>eventos-2026", "<prefix>votos-2026", …); the attendance sync
// takes a full key like the expense syncs.
const (
	SourceCamaraVotacoesYear = "camara-votacoes-"
	SourceSenadoVotacoesYear = "senado-votacoes-"
	SourceCamaraPresencaYear = "camara-presenca-"

	// SourceCamaraVotos / SourceSenadoVotos identify the promoted votes
	// datasets (curated.vote / vote_orientation / plenary_session /
	// attendance rows of each house); the API uses them as provenance keys.
	SourceCamaraVotos = "camara-votacoes"
	SourceSenadoVotos = "senado-votacoes"
)

// VoteSyncStats summarizes a votes sync run for the (logs-only) ops trail.
type VoteSyncStats struct {
	Votes         int   // per-politician vote rows upserted
	Linked        int   // votes linked to a canonical politician
	Unlinked      int   // votes left unlinked (source id unknown to the registry)
	Orientations  int   // party orientations upserted
	Sessions      int   // plenary sessions upserted
	Attendance    int   // attendance rows upserted (senado votes sync only)
	ReconciledOut int64 // curated rows deleted by the year reconcile
}

// AttendanceSyncStats summarizes a câmara attendance run.
type AttendanceSyncStats struct {
	Rows          int // presence rows processed
	Promoted      int // rows on known plenary sessions, upserted
	Skipped       int // rows on non-plenary / unknown events
	ReconciledOut int64
}

// orientationBlocs are bancada names that are not party siglas — a bloc's
// orientation can never be compared to a politician's party, so these rows
// are dropped at sync.
var orientationBlocs = map[string]bool{
	"GOVERNO": true, "OPOSIÇÃO": true, "OPOSICAO": true,
	"MINORIA": true, "MAIORIA": true,
}

// lookupSourceID resolves a registry source id to a canonical politician.
// Votes carry official ids (deputy API id / codigoParlamentar), so there is
// no fuzzy path — unknown ids stay unlinked.
func lookupSourceID(ctx context.Context, tx *sql.Tx, source, sourceID string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT politician_id FROM curated.politician_source_id WHERE source = $1 AND source_id = $2`,
		source, sourceID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("lookup %s source id %s: %w", source, sourceID, err)
	}
	return id, nil
}

func marshalAll[T any](rows []T, pickRaw func(T) json.RawMessage) ([]json.RawMessage, error) {
	payloads := make([]json.RawMessage, 0, len(rows))
	for i, r := range rows {
		p := pickRaw(r)
		if len(p) == 0 {
			var err error
			p, err = json.Marshal(r)
			if err != nil {
				return nil, fmt.Errorf("marshal row %d: %w", i, err)
			}
		}
		payloads = append(payloads, p)
	}
	return payloads, nil
}

// reconcileYear deletes the table's rows for house+year that this run did
// not touch (synced_at older than runStart) — the pull is ground truth.
func reconcileYear(ctx context.Context, tx *sql.Tx, table, house string, year int, runStart time.Time) (int64, error) {
	res, err := tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM curated.%s
		WHERE house = $1 AND year = $2 AND synced_at < $3`, table),
		house, year, runStart)
	if err != nil {
		return 0, fmt.Errorf("reconcile %s %s %d: %w", table, house, year, err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// upsertVoteOrientation inserts a party orientation row. Bloc orientations
// and empty values are dropped (returns false).
func upsertVoteOrientation(ctx context.Context, tx *sql.Tx, house, voteID, partySigla, orientation string, year int, rawID int64) (bool, error) {
	sigla := strings.ToUpper(strings.TrimSpace(partySigla))
	if sigla == "" || orientationBlocs[sigla] || orientation == "" {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO curated.vote_orientation (house, source_vote_id, party_sigla, orientation, year, raw_id, synced_at)
		VALUES ($1,$2,$3,$4,$5,$6, now())
		ON CONFLICT (house, source_vote_id, party_sigla) DO UPDATE SET
			orientation = EXCLUDED.orientation,
			raw_id      = EXCLUDED.raw_id,
			synced_at   = now()`,
		house, voteID, sigla, orientation, year, rawID); err != nil {
		return false, fmt.Errorf("upsert orientation (%s/%s/%s): %w", house, voteID, sigla, err)
	}
	return true, nil
}

func upsertPlenarySession(ctx context.Context, tx *sql.Tx, house, sessionKey string, sessionDate time.Time, year int, rawID int64) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO curated.plenary_session (house, session_key, session_date, year, raw_id, synced_at)
		VALUES ($1,$2,$3,$4,$5, now())
		ON CONFLICT (house, session_key) DO UPDATE SET
			session_date = EXCLUDED.session_date,
			year         = EXCLUDED.year,
			raw_id       = EXCLUDED.raw_id,
			synced_at    = now()`,
		house, sessionKey, sessionDate, year, rawID); err != nil {
		return fmt.Errorf("upsert plenary session (%s/%s): %w", house, sessionKey, err)
	}
	return nil
}

func upsertVote(ctx context.Context, tx *sql.Tx, house, voteID, memberSourceID string,
	votedAt time.Time, year int, description string, matterRef *string, approved *bool,
	vote, partySigla string, politicianID *int64, rawID int64) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO curated.vote (
			politician_id, house, source_vote_id, member_source_id,
			voted_at, year, description, matter_ref, approved,
			vote, party_sigla, raw_id, synced_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, now())
		ON CONFLICT (house, source_vote_id, member_source_id) DO UPDATE SET
			politician_id = EXCLUDED.politician_id,
			voted_at      = EXCLUDED.voted_at,
			year          = EXCLUDED.year,
			description   = EXCLUDED.description,
			matter_ref    = EXCLUDED.matter_ref,
			approved      = EXCLUDED.approved,
			vote          = EXCLUDED.vote,
			party_sigla   = EXCLUDED.party_sigla,
			raw_id        = EXCLUDED.raw_id,
			synced_at     = now()`,
		politicianID, house, voteID, memberSourceID,
		votedAt, year, description, matterRef, approved,
		vote, nilIfEmpty(partySigla), rawID); err != nil {
		return fmt.Errorf("upsert vote (%s/%s/%s): %w", house, voteID, memberSourceID, err)
	}
	return nil
}

func upsertAttendance(ctx context.Context, tx *sql.Tx, house, sessionKey, memberSourceID, status string,
	statusDetail *string, year int, politicianID *int64, rawID int64) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO curated.attendance (
			politician_id, house, session_key, member_source_id,
			status, status_detail, year, raw_id, synced_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now())
		ON CONFLICT (house, session_key, member_source_id) DO UPDATE SET
			politician_id = EXCLUDED.politician_id,
			status        = EXCLUDED.status,
			status_detail = EXCLUDED.status_detail,
			year          = EXCLUDED.year,
			raw_id        = EXCLUDED.raw_id,
			synced_at     = now()`,
		politicianID, house, sessionKey, memberSourceID,
		status, statusDetail, year, rawID); err != nil {
		return fmt.Errorf("upsert attendance (%s/%s/%s): %w", house, sessionKey, memberSourceID, err)
	}
	return nil
}

// --- Câmara ---------------------------------------------------------------

// SyncCamaraVotes lands the year's vote events, per-deputy votes and party
// orientations into raw (each dataset under its own key) and promotes them:
// vote rows (linked via the deputy's API id against the registry-built
// source-id map), party orientations, and the plenary sessions behind
// nominal votes (siglaOrgao=PLEN with an event id) — the attendance-rate
// denominator. All-or-nothing; reconciles all four tables against the year.
func SyncCamaraVotes(ctx context.Context, db *sql.DB, sourcePrefix string, year int,
	events []camara.VoteEvent, casts []camara.VoteCast, orientations []camara.VoteOrientation) (VoteSyncStats, error) {
	var stats VoteSyncStats

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var runStart time.Time
	if err := tx.QueryRowContext(ctx, `SELECT now()`).Scan(&runStart); err != nil {
		return stats, fmt.Errorf("read db clock: %w", err)
	}

	eventPayloads, err := marshalAll(events, func(e camara.VoteEvent) json.RawMessage { return e.Raw })
	if err != nil {
		return stats, err
	}
	eventRawIDs, err := replaceRawReturningIDs(ctx, tx, sourcePrefix+"eventos-"+strconv.Itoa(year), eventPayloads)
	if err != nil {
		return stats, err
	}
	castPayloads, err := marshalAll(casts, func(v camara.VoteCast) json.RawMessage { return v.Raw })
	if err != nil {
		return stats, err
	}
	castRawIDs, err := replaceRawReturningIDs(ctx, tx, sourcePrefix+"votos-"+strconv.Itoa(year), castPayloads)
	if err != nil {
		return stats, err
	}
	orientPayloads, err := marshalAll(orientations, func(o camara.VoteOrientation) json.RawMessage { return o.Raw })
	if err != nil {
		return stats, err
	}
	orientRawIDs, err := replaceRawReturningIDs(ctx, tx, sourcePrefix+"orientacoes-"+strconv.Itoa(year), orientPayloads)
	if err != nil {
		return stats, err
	}

	eventByID := make(map[string]camara.VoteEvent, len(events))
	eventRawByID := make(map[string]int64, len(events))
	for i, e := range events {
		eventByID[e.ID] = e
		eventRawByID[e.ID] = eventRawIDs[i]
	}

	// Sessions: only plenary events behind NOMINAL votes count — the
	// attendance rate is "sessões com votação", and symbolic votes have no
	// per-deputy records.
	nominalIDs := map[string]bool{}
	for _, c := range casts {
		nominalIDs[c.VoteID] = true
	}
	sessionsDone := map[int64]bool{}
	for id := range nominalIDs {
		e, ok := eventByID[id]
		if !ok || e.SiglaOrgao != "PLEN" || e.EventID == 0 || sessionsDone[e.EventID] {
			continue
		}
		sessionsDone[e.EventID] = true
		date, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			return stats, fmt.Errorf("parse votacao %s date %q: %w", e.ID, e.Date, err)
		}
		if err := upsertPlenarySession(ctx, tx, houseCamara,
			strconv.FormatInt(e.EventID, 10), date, year, eventRawByID[id]); err != nil {
			return stats, err
		}
		stats.Sessions++
	}

	for i, c := range casts {
		if c.VoteID == "" || c.DeputyID == 0 || c.Vote == "" {
			return stats, fmt.Errorf("vote cast row %d missing vote id, deputy id or vote: %+v", i, c)
		}
		votedAt, err := time.Parse("2006-01-02T15:04:05", c.VotedAt)
		if err != nil {
			return stats, fmt.Errorf("parse vote time %q: %w", c.VotedAt, err)
		}
		memberID := strconv.FormatInt(c.DeputyID, 10)
		politicianID, err := lookupSourceID(ctx, tx, houseCamara, memberID)
		if err != nil {
			return stats, err
		}
		var pid *int64
		if politicianID != 0 {
			pid = &politicianID
			stats.Linked++
		} else {
			stats.Unlinked++
		}

		var desc string
		var matterRef *string
		var approved *bool
		if e, ok := eventByID[c.VoteID]; ok {
			desc = e.Description
			approved = e.Approved
		}
		if desc == "" {
			desc = "Votação nominal sem descrição na fonte"
		}
		if err := upsertVote(ctx, tx, houseCamara, c.VoteID, memberID,
			votedAt, votedAt.Year(), desc, matterRef, approved,
			c.Vote, strings.ToUpper(c.PartySigla), pid, castRawIDs[i]); err != nil {
			return stats, err
		}
		stats.Votes++
	}

	for i, o := range orientations {
		ok, err := upsertVoteOrientation(ctx, tx, houseCamara, o.VoteID, o.Bancada,
			senado.NormalizeOrientation(o.Orientation), year, orientRawIDs[i])
		if err != nil {
			return stats, err
		}
		if ok {
			stats.Orientations++
		}
	}

	for _, table := range []string{"vote", "vote_orientation", "plenary_session"} {
		n, err := reconcileYear(ctx, tx, table, houseCamara, year, runStart)
		if err != nil {
			return stats, err
		}
		stats.ReconciledOut += n
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

// SyncCamaraAttendance lands the year's presence rows and promotes the ones
// on known plenary sessions (curated.plenary_session — the votes sync must
// run first) as attendance rows with status 'present'. The source publishes
// presences only; absences are derived at query time against the session
// list. All-or-nothing; reconciles the year.
func SyncCamaraAttendance(ctx context.Context, db *sql.DB, source string, year int, rows []camara.PresenceRow) (AttendanceSyncStats, error) {
	var stats AttendanceSyncStats

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var runStart time.Time
	if err := tx.QueryRowContext(ctx, `SELECT now()`).Scan(&runStart); err != nil {
		return stats, fmt.Errorf("read db clock: %w", err)
	}

	known, err := plenarySessionKeys(ctx, tx, houseCamara, year)
	if err != nil {
		return stats, err
	}

	payloads, err := marshalAll(rows, func(p camara.PresenceRow) json.RawMessage { return p.Raw })
	if err != nil {
		return stats, err
	}
	rawIDs, err := replaceRawReturningIDs(ctx, tx, source, payloads)
	if err != nil {
		return stats, err
	}

	seen := map[string]bool{}
	for i, p := range rows {
		stats.Rows++
		key := strconv.FormatInt(p.EventID, 10)
		if !known[key] {
			stats.Skipped++
			continue
		}
		memberID := strconv.FormatInt(p.DeputyID, 10)
		dedup := key + "|" + memberID
		if seen[dedup] {
			continue
		}
		seen[dedup] = true

		politicianID, err := lookupSourceID(ctx, tx, houseCamara, memberID)
		if err != nil {
			return stats, err
		}
		var pid *int64
		if politicianID != 0 {
			pid = &politicianID
		}
		if err := upsertAttendance(ctx, tx, houseCamara, key, memberID, "present", nil, year, pid, rawIDs[i]); err != nil {
			return stats, err
		}
		stats.Promoted++
	}

	n, err := reconcileYear(ctx, tx, "attendance", houseCamara, year, runStart)
	if err != nil {
		return stats, err
	}
	stats.ReconciledOut = n

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

// plenarySessionKeys returns the year's known plenary session keys for a
// house.
func plenarySessionKeys(ctx context.Context, tx *sql.Tx, house string, year int) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT session_key FROM curated.plenary_session WHERE house = $1 AND year = $2`, house, year)
	if err != nil {
		return nil, fmt.Errorf("list plenary sessions (%s/%d): %w", house, year, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out[k] = true
	}
	return out, rows.Err()
}

// --- Senado ---------------------------------------------------------------

// senadoAttendanceStatus classifies a per-senator vote code from the nominal
// feed into the attendance vocabulary. Verified live 2026-09-28: real votes
// (Sim/Não/Abstenção), secret-vote marker (Votou), present-not-voted (P-NRV),
// the president's non-vote, justified absences (AP atividade parlamentar,
// MIS missão, LS/LP/LAP licenças) and absences (NCom, NA). Unknown codes
// classify as absent (conservative; the raw code is kept in status_detail).
func senadoAttendanceStatus(code string) string {
	switch code {
	case "Sim", "Não", "Abstenção", "Votou", "P-NRV", "Presidente (art. 51 RISF)":
		return "present"
	case "AP", "MIS", "LS", "LP", "LAP":
		return "justified"
	default:
		return "absent"
	}
}

// SyncSenadoVotes lands the year's nominal votes (with the full senator
// roster) and party orientations into raw and promotes them: vote rows
// (linked via codigoParlamentar), orientations, plenary sessions, and
// attendance — the roster codes double as the presence record, so one
// session's status for a senator is the best code across the session's
// votes (present > justified > absent). All-or-nothing; reconciles all four
// tables against the year.
func SyncSenadoVotes(ctx context.Context, db *sql.DB, sourcePrefix string, year int,
	votes []senado.NominalVote, orientations []senado.VoteOrientation) (VoteSyncStats, error) {
	var stats VoteSyncStats

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var runStart time.Time
	if err := tx.QueryRowContext(ctx, `SELECT now()`).Scan(&runStart); err != nil {
		return stats, fmt.Errorf("read db clock: %w", err)
	}

	votePayloads, err := marshalAll(votes, func(v senado.NominalVote) json.RawMessage { return v.Raw })
	if err != nil {
		return stats, err
	}
	voteRawIDs, err := replaceRawReturningIDs(ctx, tx, sourcePrefix+"nominal-"+strconv.Itoa(year), votePayloads)
	if err != nil {
		return stats, err
	}
	orientPayloads, err := marshalAll(orientations, func(o senado.VoteOrientation) json.RawMessage { return o.Raw })
	if err != nil {
		return stats, err
	}
	orientRawIDs, err := replaceRawReturningIDs(ctx, tx, sourcePrefix+"orientacoes-"+strconv.Itoa(year), orientPayloads)
	if err != nil {
		return stats, err
	}

	type attendanceKey struct {
		session, member string
	}
	type attendanceAcc struct {
		status string
		detail string
		rawID  int64
	}
	attendance := map[attendanceKey]attendanceAcc{}
	statusRank := map[string]int{"absent": 0, "justified": 1, "present": 2}

	for i, v := range votes {
		if v.SessionCode == "" || v.SessionVoteCode == "" {
			return stats, fmt.Errorf("nominal vote %d missing session codes: %+v", i, v)
		}
		date, err := time.Parse("2006-01-02", v.SessionDate)
		if err != nil {
			return stats, fmt.Errorf("parse session date %q: %w", v.SessionDate, err)
		}
		if err := upsertPlenarySession(ctx, tx, houseSenado, v.SessionCode, date, date.Year(), voteRawIDs[i]); err != nil {
			return stats, err
		}
		stats.Sessions++

		var approved *bool
		switch v.Result {
		case "A":
			t := true
			approved = &t
		case "R":
			f := false
			approved = &f
		}
		var matterRef *string
		if v.MatterRef != "" {
			matterRef = &v.MatterRef
		}
		votedAt := date
		if v.SessionStart != "" {
			if t, err := time.Parse("2006-01-02 15:04", v.SessionDate+" "+v.SessionStart); err == nil {
				votedAt = t
			}
		}

		for _, sv := range v.Votes {
			if sv.SenatorCode == 0 || sv.Vote == "" {
				return stats, fmt.Errorf("senator vote in %s missing code or vote: %+v", v.SessionVoteCode, sv)
			}
			memberID := strconv.FormatInt(sv.SenatorCode, 10)
			politicianID, err := lookupSourceID(ctx, tx, houseSenado, memberID)
			if err != nil {
				return stats, err
			}
			var pid *int64
			if politicianID != 0 {
				pid = &politicianID
				stats.Linked++
			} else {
				stats.Unlinked++
			}

			if err := upsertVote(ctx, tx, houseSenado, v.SessionVoteCode, memberID,
				votedAt, date.Year(), v.Description, matterRef, approved,
				sv.Vote, strings.ToUpper(sv.PartySigla), pid, voteRawIDs[i]); err != nil {
				return stats, err
			}
			stats.Votes++

			ak := attendanceKey{v.SessionCode, memberID}
			status := senadoAttendanceStatus(sv.Vote)
			if cur, ok := attendance[ak]; !ok || statusRank[status] > statusRank[cur.status] {
				attendance[ak] = attendanceAcc{status: status, detail: sv.Vote, rawID: voteRawIDs[i]}
			}
		}
	}

	for ak, acc := range attendance {
		politicianID, err := lookupSourceID(ctx, tx, houseSenado, ak.member)
		if err != nil {
			return stats, err
		}
		var pid *int64
		if politicianID != 0 {
			pid = &politicianID
		}
		if err := upsertAttendance(ctx, tx, houseSenado, ak.session, ak.member,
			acc.status, nilIfEmpty(acc.detail), year, pid, acc.rawID); err != nil {
			return stats, err
		}
		stats.Attendance++
	}

	// Orientations join votes via VoteSeq (the nominal feed's
	// CodigoVotacaoSve); the curated key space is the vote's unique id
	// (SessionVoteCode), so translate. Orientations for votes absent from
	// the year's pull are dropped.
	voteIDBySeq := make(map[string]string, len(votes))
	for _, v := range votes {
		voteIDBySeq[v.VoteSeq] = v.SessionVoteCode
	}
	for i, o := range orientations {
		voteID, ok := voteIDBySeq[o.VoteSeq]
		if !ok {
			continue
		}
		ok, err = upsertVoteOrientation(ctx, tx, houseSenado, voteID, o.PartySigla,
			o.Orientation, year, orientRawIDs[i])
		if err != nil {
			return stats, err
		}
		if ok {
			stats.Orientations++
		}
	}

	for _, table := range []string{"vote", "vote_orientation", "plenary_session", "attendance"} {
		n, err := reconcileYear(ctx, tx, table, houseSenado, year, runStart)
		if err != nil {
			return stats, err
		}
		stats.ReconciledOut += n
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}
