package httpapi

import (
	"context"
	"log"
	"time"

	"voz-do-povo/internal/sync"
)

// Source names as shown to citizens (pt-BR), votes & attendance (ticket 19).
const (
	sourceNameVotacoesCamara = "Câmara dos Deputados — votações nominais e presença em plenário"
	sourceNameVotacoesSenado = "Senado Federal — votações nominais de plenário"
)

// legislatureStart is the first day of the current legislature (57ª,
// 2023-02-01 → 2027-01-31): the attendance rate covers it.
const legislatureStart = "2023-02-01"

// profileVotesLimit is the section's vote-list size ("as ~20 votações mais
// recentes").
const profileVotesLimit = 20

// votesSection builds the profile's votes & attendance block: the ~20 most
// recent nominal votes with the own-party orientation comparison, and the
// current-legislature attendance rate. The linked check follows the other
// sections: a politician with a registry link or any vote/attendance row is
// linked; otherwise the honest "not linked" state. house scopes every query
// ("" = no registry link → unscoped, like expenses).
func (s *server) votesSection(ctx context.Context, politicianID int64, house string) map[string]any {
	source := sourceMeta{
		Key:      sync.SourceCamaraVotos,
		Name:     sourceNameVotacoesCamara,
		SyncedAt: s.lastVoteSync(ctx, "camara"),
	}
	if house == "senado" {
		source = sourceMeta{
			Key:      sync.SourceSenadoVotos,
			Name:     sourceNameVotacoesSenado,
			SyncedAt: s.lastVoteSync(ctx, "senado"),
		}
	}

	var linked bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM curated.politician_source_id
			WHERE politician_id = $1 AND source IN ('camara', 'senado')
		) OR EXISTS (
			SELECT 1 FROM curated.vote WHERE politician_id = $1
		)`, politicianID).Scan(&linked); err != nil {
		log.Printf("profile %d votes link check: %v", politicianID, err)
	}
	if !linked {
		return map[string]any{
			"linked":  false,
			"message": "Ainda não conseguimos ligar os votos e a presença desta pessoa à fonte oficial com segurança. Não mostramos números sem essa confirmação.",
			"source":  source,
		}
	}

	return map[string]any{
		"linked":      true,
		"legislature": "2023–2026",
		"attendance":  s.attendanceBlock(ctx, politicianID, house),
		"alignment":   s.alignmentBlock(ctx, politicianID, house),
		"items":       s.voteItems(ctx, politicianID, house),
		"source":      source,
	}
}

// attendanceBlock is the current-legislature attendance rate. The
// denominator differs per source shape: the Câmara publishes presences only,
// so the denominator is every plenary session that held a nominal vote; the
// Senado lists every senator per vote, so the denominator is the sessions
// where the senator has any record. Justified absences (licenças, missões,
// atividade parlamentar) exist only in the Senado source — null for the
// Câmara.
func (s *server) attendanceBlock(ctx context.Context, politicianID int64, house string) map[string]any {
	var present, sessions int
	var justified *int

	if house == "senado" {
		var total, just int
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FILTER (WHERE status = 'present'),
			       COUNT(*) FILTER (WHERE status = 'justified'),
			       COUNT(*)
			FROM curated.attendance a
			JOIN curated.plenary_session ps
			  ON ps.house = a.house AND ps.session_key = a.session_key
			WHERE a.politician_id = $1 AND a.house = 'senado'
			  AND ps.session_date >= $2`, politicianID, legislatureStart).
			Scan(&present, &just, &total); err != nil {
			log.Printf("profile %d attendance (senado): %v", politicianID, err)
		}
		sessions = total
		justified = &just
	} else {
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM curated.attendance a
			JOIN curated.plenary_session ps
			  ON ps.house = a.house AND ps.session_key = a.session_key
			WHERE a.politician_id = $1 AND a.house = 'camara'
			  AND ps.session_date >= $2`, politicianID, legislatureStart).
			Scan(&present); err != nil {
			log.Printf("profile %d attendance present (camara): %v", politicianID, err)
		}
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM curated.plenary_session
			WHERE house = 'camara' AND session_date >= $1`, legislatureStart).
			Scan(&sessions); err != nil {
			log.Printf("profile %d attendance sessions (camara): %v", politicianID, err)
		}
	}

	rate := 0.0
	if sessions > 0 {
		rate = float64(present) / float64(sessions)
	}
	return map[string]any{
		"sessions":  sessions,
		"present":   present,
		"justified": justified,
		"rate":      rate,
	}
}

// alignmentBlock summarizes how often the politician's vote matched the
// party leadership's orientation, over the votes where both exist and both
// are a position (Sim/Não — Liberado and Abstenção never compare). null
// when there is nothing comparable.
func (s *server) alignmentBlock(ctx context.Context, politicianID int64, house string) map[string]any {
	var comparable, aligned int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE v.vote = o.orientation)
		FROM curated.vote v
		JOIN curated.vote_orientation o
		  ON o.house = v.house AND o.source_vote_id = v.source_vote_id
		 AND o.party_sigla = v.party_sigla
		WHERE v.politician_id = $1 AND ($2 = '' OR v.house = $2)
		  AND v.vote IN ('Sim', 'Não')
		  AND o.orientation IN ('Sim', 'Não')`, politicianID, house).
		Scan(&comparable, &aligned); err != nil {
		log.Printf("profile %d alignment: %v", politicianID, err)
	}
	if comparable == 0 {
		return nil
	}
	return map[string]any{
		"comparable": comparable,
		"aligned":    aligned,
		"rate":       float64(aligned) / float64(comparable),
	}
}

// voteItems are the most recent nominal votes where the politician took a
// position (Sim/Não/Abstenção/Obstrução — presence codes and the secret-vote
// marker "Votou" never list). aligned is the three-state comparison against
// the own party's orientation: null when the party freed the vote, none was
// recorded, or the position is not comparable.
func (s *server) voteItems(ctx context.Context, politicianID int64, house string) []map[string]any {
	rows, err := s.db.QueryContext(ctx, `
		SELECT to_char(v.voted_at, 'YYYY-MM-DD'), v.description, v.matter_ref,
		       v.vote, v.party_sigla, o.orientation,
		       CASE WHEN v.vote IN ('Sim', 'Não') AND o.orientation IN ('Sim', 'Não')
		            THEN v.vote = o.orientation END AS aligned
		FROM curated.vote v
		LEFT JOIN curated.vote_orientation o
		  ON o.house = v.house AND o.source_vote_id = v.source_vote_id
		 AND o.party_sigla = v.party_sigla
		WHERE v.politician_id = $1 AND ($2 = '' OR v.house = $2)
		  AND v.vote IN ('Sim', 'Não', 'Abstenção', 'Obstrução')
		ORDER BY v.voted_at DESC
		LIMIT $3`, politicianID, house, profileVotesLimit)
	if err != nil {
		log.Printf("profile %d vote items: %v", politicianID, err)
		return []map[string]any{}
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var date, description, vote string
		var matterRef, partySigla, orientation *string
		var aligned *bool
		if err := rows.Scan(&date, &description, &matterRef, &vote, &partySigla, &orientation, &aligned); err != nil {
			log.Printf("profile %d vote item scan: %v", politicianID, err)
			continue
		}
		items = append(items, map[string]any{
			"date":             date,
			"description":      description,
			"matterRef":        matterRef,
			"vote":             vote,
			"partySigla":       partySigla,
			"partyOrientation": orientation,
			"aligned":          aligned,
		})
	}
	return items
}

// lastVoteSync derives a house's last votes sync from curated rows: every
// sync run bumps synced_at (upsert), so MAX is the last run that saw data.
// nil when never synced.
func (s *server) lastVoteSync(ctx context.Context, house string) *time.Time {
	var at *time.Time
	if err := s.db.QueryRowContext(ctx,
		`SELECT MAX(synced_at) FROM curated.vote WHERE house = $1`, house).Scan(&at); err != nil {
		log.Printf("vote freshness (%s): %v", house, err)
	}
	return at
}
