package httpapi_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Seam 1 (ticket 19): the profile votes & attendance payload over HTTP
// against a real Postgres seeded at the curated layer.

func seedPlenarySession(t *testing.T, db *sql.DB, house, key, date string, year int) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO curated.plenary_session (house, session_key, session_date, year)
		VALUES ($1, $2, $3, $4)`, house, key, date, year)
	if err != nil {
		t.Fatalf("seed session (%s/%s): %v", house, key, err)
	}
}

func seedVote(t *testing.T, db *sql.DB, politicianID int64, house, voteID, memberID, votedAt string, year int, desc, vote, party string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO curated.vote (
			politician_id, house, source_vote_id, member_source_id,
			voted_at, year, description, vote, party_sigla
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		politicianID, house, voteID, memberID, votedAt, year, desc, vote, party)
	if err != nil {
		t.Fatalf("seed vote (%s/%s): %v", voteID, memberID, err)
	}
}

func seedOrientation(t *testing.T, db *sql.DB, house, voteID, party, orientation string, year int) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO curated.vote_orientation (house, source_vote_id, party_sigla, orientation, year)
		VALUES ($1,$2,$3,$4,$5)`, house, voteID, party, orientation, year)
	if err != nil {
		t.Fatalf("seed orientation (%s/%s): %v", voteID, party, err)
	}
}

func seedAttendance(t *testing.T, db *sql.DB, politicianID int64, house, sessionKey, memberID, status string, detail *string, year int) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO curated.attendance (
			politician_id, house, session_key, member_source_id, status, status_detail, year
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		politicianID, house, sessionKey, memberID, status, detail, year)
	if err != nil {
		t.Fatalf("seed attendance (%s/%s): %v", sessionKey, memberID, err)
	}
}

type votesSectionBody struct {
	Linked      bool   `json:"linked"`
	Message     string `json:"message"`
	Legislature string `json:"legislature"`
	Attendance  struct {
		Sessions  int     `json:"sessions"`
		Present   int     `json:"present"`
		Justified *int    `json:"justified"`
		Rate      float64 `json:"rate"`
	} `json:"attendance"`
	Alignment *struct {
		Comparable int     `json:"comparable"`
		Aligned    int     `json:"aligned"`
		Rate       float64 `json:"rate"`
	} `json:"alignment"`
	Items []struct {
		Date             string  `json:"date"`
		Description      string  `json:"description"`
		Vote             string  `json:"vote"`
		PartySigla       *string `json:"partySigla"`
		PartyOrientation *string `json:"partyOrientation"`
		Aligned          *bool   `json:"aligned"`
	} `json:"items"`
	Source struct {
		Key      string  `json:"key"`
		Name     string  `json:"name"`
		SyncedAt *string `json:"syncedAt"`
	} `json:"source"`
}

// Câmara profile: attendance rate over all plenary sessions of the
// legislature (presence-only source), ~20 recent votes with the
// party-orientation comparison, source + sync date present.
func TestProfileVotesSectionCamara(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	thisYear := time.Now().Year()
	// 3 plenary sessions this legislature; the deputy was present at 2.
	seedPlenarySession(t, db, "camara", "74889", fmt.Sprintf("%d-02-24", thisYear), thisYear)
	seedPlenarySession(t, db, "camara", "74900", fmt.Sprintf("%d-03-03", thisYear), thisYear)
	seedPlenarySession(t, db, "camara", "74910", fmt.Sprintf("%d-03-10", thisYear), thisYear)
	seedAttendance(t, db, 7, "camara", "74889", "204402", "present", nil, thisYear)
	seedAttendance(t, db, 7, "camara", "74900", "204402", "present", nil, thisYear)

	// Votes: aligned, divergent, freed (Liberado → not comparable), and a
	// presence-only code that must not list.
	seedVote(t, db, 7, "camara", "V1", "204402", fmt.Sprintf("%d-02-24T14:46:15Z", thisYear), thisYear, "Aprovado o PL nº 1.", "Sim", "PT")
	seedVote(t, db, 7, "camara", "V2", "204402", fmt.Sprintf("%d-03-03T15:00:00Z", thisYear), thisYear, "Rejeitado o REQ nº 2.", "Não", "PT")
	seedVote(t, db, 7, "camara", "V3", "204402", fmt.Sprintf("%d-03-10T16:00:00Z", thisYear), thisYear, "Aprovado o PL nº 3.", "Sim", "PT")
	seedVote(t, db, 7, "camara", "V4", "204402", fmt.Sprintf("%d-03-11T16:00:00Z", thisYear), thisYear, "Votação secreta.", "Votou", "PT")
	seedOrientation(t, db, "camara", "V1", "PT", "Sim", thisYear)
	seedOrientation(t, db, "camara", "V2", "PT", "Sim", thisYear)
	seedOrientation(t, db, "camara", "V3", "PT", "Liberado", thisYear)

	res := getJSON(t, fmt.Sprintf("%s/api/politicians/7", srv.URL))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Votes votesSectionBody `json:"votes"`
	}
	mustDecode(t, res, &body)
	v := body.Votes

	if !v.Linked {
		t.Fatalf("votes section not linked: %+v", v)
	}
	if v.Source.Name == "" || v.Source.SyncedAt == nil {
		t.Errorf("source metadata missing name or sync date: %+v", v.Source)
	}
	if v.Legislature != "2023–2026" {
		t.Errorf("legislature = %q, want 2023–2026", v.Legislature)
	}
	if v.Attendance.Sessions != 3 || v.Attendance.Present != 2 {
		t.Errorf("attendance = %+v, want 2/3", v.Attendance)
	}
	if v.Attendance.Justified != nil {
		t.Errorf("justified = %v, want null (câmara source has no justified absences)", *v.Attendance.Justified)
	}
	if v.Attendance.Rate < 0.66 || v.Attendance.Rate > 0.67 {
		t.Errorf("attendance rate = %v, want ~0.667", v.Attendance.Rate)
	}

	// 3 position votes list (Votou never does); orientation comparison:
	// aligned, divergent, freed.
	if len(v.Items) != 3 {
		t.Fatalf("items = %d, want 3: %+v", len(v.Items), v.Items)
	}
	if v.Items[0].Description != "Aprovado o PL nº 3." || v.Items[0].Aligned != nil {
		t.Errorf("newest item = %+v, want the Liberado vote with aligned null", v.Items[0])
	}
	if v.Items[1].Aligned == nil || *v.Items[1].Aligned {
		t.Errorf("V2 aligned = %v, want false (Não contra orientação Sim)", v.Items[1].Aligned)
	}
	if v.Items[2].Aligned == nil || !*v.Items[2].Aligned {
		t.Errorf("V1 aligned = %v, want true", v.Items[2].Aligned)
	}
	if v.Items[2].PartyOrientation == nil || *v.Items[2].PartyOrientation != "Sim" {
		t.Errorf("V1 partyOrientation = %v, want Sim", v.Items[2].PartyOrientation)
	}

	// Alignment rollup: only the two Sim/Não votes compare; 1 of 2 aligned.
	if v.Alignment == nil || v.Alignment.Comparable != 2 || v.Alignment.Aligned != 1 {
		t.Errorf("alignment = %+v, want 1/2", v.Alignment)
	}
}

// Senado profile: attendance over sessions with records for the senator,
// justified absences counted separately.
func TestProfileVotesSectionSenado(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 8, "Ana Mostra Souza", "Ana Mostra", "MG", "11122233344")
	seedSourceID(t, db, 8, "senado", "9002")

	thisYear := time.Now().Year()
	seedPlenarySession(t, db, "senado", "550469", fmt.Sprintf("%d-02-24", thisYear), thisYear)
	seedPlenarySession(t, db, "senado", "550470", fmt.Sprintf("%d-02-25", thisYear), thisYear)
	ap := "AP"
	seedAttendance(t, db, 8, "senado", "550469", "9002", "present", nil, thisYear)
	seedAttendance(t, db, 8, "senado", "550470", "9002", "justified", &ap, thisYear)
	seedVote(t, db, 8, "senado", "7047", "9002", fmt.Sprintf("%d-02-24T14:00:00Z", thisYear), thisYear, "PEC 22/2025.", "Não", "PSDB")
	seedOrientation(t, db, "senado", "7047", "PSDB", "Não", thisYear)

	res := getJSON(t, fmt.Sprintf("%s/api/politicians/8", srv.URL))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Votes votesSectionBody `json:"votes"`
	}
	mustDecode(t, res, &body)
	v := body.Votes

	if !v.Linked {
		t.Fatalf("votes section not linked: %+v", v)
	}
	if v.Attendance.Sessions != 2 || v.Attendance.Present != 1 {
		t.Errorf("attendance = %+v, want 1/2", v.Attendance)
	}
	if v.Attendance.Justified == nil || *v.Attendance.Justified != 1 {
		t.Errorf("justified = %v, want 1", v.Attendance.Justified)
	}
	if v.Alignment == nil || v.Alignment.Comparable != 1 || v.Alignment.Aligned != 1 {
		t.Errorf("alignment = %+v, want 1/1", v.Alignment)
	}
}

// No registry link and no rows: the honest "not linked" state.
func TestProfileVotesSectionUnlinked(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 9, "Sem Vínculo", "Sem Vínculo", "AC", "99988877766")

	res := getJSON(t, fmt.Sprintf("%s/api/politicians/9", srv.URL))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Votes votesSectionBody `json:"votes"`
	}
	mustDecode(t, res, &body)
	if body.Votes.Linked {
		t.Errorf("votes section linked, want not-linked")
	}
	if body.Votes.Message == "" {
		t.Errorf("not-linked message empty")
	}
	if body.Votes.Source.Name == "" {
		t.Errorf("source name empty on not-linked state")
	}
}
