package httpapi

import (
	"context"
	"log"
	"net/http"
	"time"

	"voz-do-povo/internal/sync"
)

// Home expense rankings (issue 16): top spenders of the current calendar
// year and biggest single expenses of the latest synced month, per house.
// CEAP and CEAPS quotas are never mixed: every aggregate is scoped to the
// requested house. Unlinked expenses (politician_id NULL) never rank — a
// ranking row must link to a profile.

// rankingSize keeps the home page scannable (~10 rows per ranking).
const rankingSize = 10

// rankingPerson is the politician a ranking row links to.
type rankingPerson struct {
	ID                int64   `json:"id"`
	ParliamentaryName string  `json:"parliamentaryName"`
	UF                *string `json:"uf"`
	Party             *string `json:"party"`
}

type topSpenderRow struct {
	Politician rankingPerson `json:"politician"`
	Total      float64       `json:"total"`
}

type biggestExpenseRow struct {
	Politician   rankingPerson `json:"politician"`
	Category     string        `json:"category"`
	SupplierName *string       `json:"supplierName"`
	IssuedAt     *string       `json:"issuedAt"`
	AmountNet    float64       `json:"amountNet"`
}

func (s *server) handleRankings(w http.ResponseWriter, r *http.Request) {
	house := r.URL.Query().Get("house")
	if house != "camara" && house != "senado" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "house must be camara or senado"})
		return
	}

	ctx := r.Context()
	year := time.Now().Year()

	// "Of the month" = the latest month the sync has landed for this house,
	// so the ranking degrades gracefully (last good data) when a sync lags.
	var monthYear, month int
	err := s.db.QueryRowContext(ctx, `
		SELECT year, month FROM curated.expense
		WHERE house = $1
		ORDER BY year DESC, month DESC
		LIMIT 1`, house).Scan(&monthYear, &month)
	if err != nil {
		log.Printf("rankings latest month (%s): %v", house, err)
	}

	source := sourceMeta{Key: sync.SourceCamaraCEAP, Name: sourceNameCEAP, SyncedAt: s.lastExpenseSync(ctx, house)}
	if house == "senado" {
		source = sourceMeta{Key: sync.SourceSenadoCEAPS, Name: sourceNameCEAPS, SyncedAt: s.lastExpenseSync(ctx, house)}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"house":           house,
		"year":            year,
		"month":           month,
		"monthYear":       monthYear,
		"topSpenders":     s.topSpenders(ctx, house, year),
		"biggestExpenses": s.biggestExpenses(ctx, house, monthYear, month),
		"source":          source,
	})
}

// topSpenders ranks politicians by net expenses summed over the current
// calendar year. Party = the most recent expense's party in that house
// (the party at expense time is the meaningful one).
func (s *server) topSpenders(ctx context.Context, house string, year int) []topSpenderRow {
	rows := []topSpenderRow{}
	qr, err := s.db.QueryContext(ctx, `
		SELECT x.politician_id, p.parliamentary_name, p.uf, pa.sigla, x.total
		FROM (
			SELECT politician_id, SUM(amount_net)::float8 AS total
			FROM curated.expense
			WHERE house = $1 AND year = $2 AND politician_id IS NOT NULL
			GROUP BY politician_id
			ORDER BY total DESC
			LIMIT $3
		) x
		JOIN curated.politician p ON p.id = x.politician_id
		LEFT JOIN LATERAL (
			SELECT e.party_id
			FROM curated.expense e
			WHERE e.politician_id = x.politician_id AND e.house = $1 AND e.party_id IS NOT NULL
			ORDER BY e.year DESC, e.month DESC
			LIMIT 1
		) le ON true
		LEFT JOIN curated.party pa ON pa.id = le.party_id
		ORDER BY x.total DESC`, house, year, rankingSize)
	if err != nil {
		log.Printf("rankings top spenders (%s/%d): %v", house, year, err)
		return rows
	}
	defer qr.Close()
	for qr.Next() {
		var row topSpenderRow
		if err := qr.Scan(&row.Politician.ID, &row.Politician.ParliamentaryName,
			&row.Politician.UF, &row.Politician.Party, &row.Total); err != nil {
			log.Printf("rankings top spenders scan: %v", err)
			return rows
		}
		rows = append(rows, row)
	}
	return rows
}

// biggestExpenses ranks single expense items of the given month.
func (s *server) biggestExpenses(ctx context.Context, house string, year, month int) []biggestExpenseRow {
	rows := []biggestExpenseRow{}
	if month == 0 {
		return rows
	}
	qr, err := s.db.QueryContext(ctx, `
		SELECT e.politician_id, p.parliamentary_name, p.uf, pa.sigla,
		       e.category, e.supplier_name, to_char(e.issued_at, 'YYYY-MM-DD'),
		       e.amount_net::float8
		FROM curated.expense e
		JOIN curated.politician p ON p.id = e.politician_id
		LEFT JOIN curated.party pa ON pa.id = e.party_id
		WHERE e.house = $1 AND e.year = $2 AND e.month = $3
		  AND e.politician_id IS NOT NULL
		ORDER BY e.amount_net DESC
		LIMIT $4`, house, year, month, rankingSize)
	if err != nil {
		log.Printf("rankings biggest expenses (%s/%d-%02d): %v", house, year, month, err)
		return rows
	}
	defer qr.Close()
	for qr.Next() {
		var row biggestExpenseRow
		if err := qr.Scan(&row.Politician.ID, &row.Politician.ParliamentaryName,
			&row.Politician.UF, &row.Politician.Party,
			&row.Category, &row.SupplierName, &row.IssuedAt, &row.AmountNet); err != nil {
			log.Printf("rankings biggest expenses scan: %v", err)
			return rows
		}
		rows = append(rows, row)
	}
	return rows
}
