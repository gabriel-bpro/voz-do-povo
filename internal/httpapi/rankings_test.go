package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Seam 1: home expense rankings (issue 16) — top spenders of the current
// calendar year and biggest single expenses of the latest synced month, per
// house. Covers both rankings, both house selections, sync-date presence,
// and the no-cross-house-mixing rule (CEAP and CEAPS never compared).

type rankingsBody struct {
	House     string `json:"house"`
	Year      int    `json:"year"`
	Month     int    `json:"month"`
	MonthYear int    `json:"monthYear"`
	Top       []struct {
		Politician struct {
			ID                int64   `json:"id"`
			ParliamentaryName string  `json:"parliamentaryName"`
			UF                string  `json:"uf"`
			Party             *string `json:"party"`
		} `json:"politician"`
		Total float64 `json:"total"`
	} `json:"topSpenders"`
	Biggest []struct {
		Politician struct {
			ID                int64  `json:"id"`
			ParliamentaryName string `json:"parliamentaryName"`
		} `json:"politician"`
		Category  string  `json:"category"`
		AmountNet float64 `json:"amountNet"`
	} `json:"biggestExpenses"`
	Source struct {
		Key      string  `json:"key"`
		Name     string  `json:"name"`
		SyncedAt *string `json:"syncedAt"`
	} `json:"source"`
}

func getRankings(t *testing.T, srvURL, house string) rankingsBody {
	t.Helper()
	res := getJSON(t, srvURL+"/api/rankings?house="+house)
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)
	var body rankingsBody
	mustDecode(t, res, &body)
	return body
}

func TestRankingsRejectsBadHouse(t *testing.T) {
	_, srv := newSeededAPI(t)

	res := getJSON(t, srv.URL+"/api/rankings")
	defer res.Body.Close()
	mustStatus(t, res, http.StatusBadRequest)

	res = getJSON(t, srv.URL+"/api/rankings?house=assembleia")
	defer res.Body.Close()
	mustStatus(t, res, http.StatusBadRequest)
}

func TestRankingsTopSpendersPerHouse(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 1, "Ana Gasta Muito", "Ana Gasta", "SP", "11111111111")
	seedSourceID(t, db, 1, "camara", "300001")
	seedPolitician(t, db, 2, "Beto Gasta Menos", "Beto Menos", "RJ", "22222222222")
	seedSourceID(t, db, 2, "camara", "300002")
	seedPolitician(t, db, 3, "Sena Dora Rica", "Sena Rica", "MG", "33333333333")
	seedSourceID(t, db, 3, "senado", "9003")
	partyID := seedPartyRow(t, db, "PT")

	y := time.Now().Year()
	at := fmt.Sprintf("%d-03-10", y)
	// Câmara: Ana 1000+500 = 1500; Beto 800. One glosa nets Ana down.
	seedExpense(t, db, 1, &partyID, y, 3, "Passagens aéreas", "Voe Bem", at, 1000.00)
	seedExpense(t, db, 1, &partyID, y, 3, "Combustíveis", "Posto", fmt.Sprintf("%d-03-11", y), 500.00)
	seedExpense(t, db, 2, nil, y, 3, "Passagens aéreas", "Voe Bem", fmt.Sprintf("%d-03-12", y), 800.00)
	// Prior year: must not count toward the current-year ranking.
	seedExpense(t, db, 2, nil, y-1, 12, "Passagens aéreas", "Voe Bem", fmt.Sprintf("%d-12-10", y-1), 99999.00)
	// Senado: bigger total, but CEAPS never mixes with CEAP.
	seedExpenseHouse(t, db, "senado", 3, nil, y, 3, "Passagens aéreas", "Voe Bem", at, 9000.00)
	// Unlinked Câmara expense: no profile to link to — never ranks.
	seedBucketExpensesHouse(t, db, "camara", "Consultoria", 77777.00, 1)

	body := getRankings(t, srv.URL, "camara")

	if body.Year != y {
		t.Errorf("year = %d, want %d", body.Year, y)
	}
	if len(body.Top) != 2 {
		t.Fatalf("topSpenders = %+v, want the 2 câmara politicians", body.Top)
	}
	if body.Top[0].Politician.ID != 1 || body.Top[0].Total != 1500.00 {
		t.Errorf("top[0] = %+v, want politician 1 total 1500", body.Top[0])
	}
	if body.Top[1].Politician.ID != 2 || body.Top[1].Total != 800.00 {
		t.Errorf("top[1] = %+v, want politician 2 total 800", body.Top[1])
	}
	if body.Top[0].Politician.Party == nil || *body.Top[0].Politician.Party != "PT" {
		t.Errorf("top[0] party = %v, want PT (latest expense party)", body.Top[0].Politician.Party)
	}
	if body.Source.Key != "camara-ceap" || body.Source.SyncedAt == nil {
		t.Errorf("source = %+v, want the CEAP block with a sync date", body.Source)
	}

	// The senado selection ranks the senator — cross-house values never mix.
	sen := getRankings(t, srv.URL, "senado")
	if len(sen.Top) != 1 || sen.Top[0].Politician.ID != 3 || sen.Top[0].Total != 9000.00 {
		t.Errorf("senado topSpenders = %+v, want only politician 3 total 9000", sen.Top)
	}
	if sen.Source.Key != "senado-ceaps" || sen.Source.SyncedAt == nil {
		t.Errorf("senado source = %+v, want the CEAPS block with a sync date", sen.Source)
	}
}

func TestRankingsBiggestExpensesOfLatestMonth(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 1, "Ana Gasta Muito", "Ana Gasta", "SP", "11111111111")
	seedSourceID(t, db, 1, "camara", "300001")
	seedPolitician(t, db, 2, "Sena Dora Rica", "Sena Rica", "MG", "33333333333")
	seedSourceID(t, db, 2, "senado", "9004")

	now := time.Now()
	at := now.Format("2006-01-02")
	y, m := now.Year(), int(now.Month())
	seedExpense(t, db, 1, nil, y, m, "Divulgação da atividade parlamentar", "Gráfica", at, 28400.00)
	seedExpense(t, db, 1, nil, y, m, "Passagens aéreas", "Voe Bem", at, 4200.00)
	// Older month: must not appear in the month ranking.
	old := now.AddDate(0, -1, 0)
	seedExpense(t, db, 1, nil, old.Year(), int(old.Month()), "Passagens aéreas", "Voe Bem", old.Format("2006-01-02"), 99999.00)
	// Senado item in the same month: excluded from the câmara ranking.
	seedExpenseHouse(t, db, "senado", 2, nil, y, m, "Locação de aeronaves", "Aerotur", at, 50000.00)

	body := getRankings(t, srv.URL, "camara")

	if body.Month != m || body.MonthYear != y {
		t.Errorf("month = %d/%d, want %d/%d", body.Month, body.MonthYear, m, y)
	}
	if len(body.Biggest) != 2 {
		t.Fatalf("biggestExpenses = %+v, want the 2 câmara items of the month", body.Biggest)
	}
	if body.Biggest[0].AmountNet != 28400.00 || body.Biggest[0].Politician.ID != 1 {
		t.Errorf("biggest[0] = %+v, want the 28400 item of politician 1", body.Biggest[0])
	}
	if body.Biggest[0].Category != "Divulgação da atividade parlamentar" {
		t.Errorf("biggest[0] category = %q", body.Biggest[0].Category)
	}

	// The senado selection shows the senado item.
	sen := getRankings(t, srv.URL, "senado")
	if len(sen.Biggest) != 1 || sen.Biggest[0].AmountNet != 50000.00 {
		t.Errorf("senado biggestExpenses = %+v, want only the 50000 CEAPS item", sen.Biggest)
	}
}

// An empty house (no expenses synced yet) renders empty rankings, not an
// error — the app degrades gracefully.
func TestRankingsEmptyHouse(t *testing.T) {
	_, srv := newSeededAPI(t)

	body := getRankings(t, srv.URL, "senado")
	if len(body.Top) != 0 || len(body.Biggest) != 0 {
		t.Errorf("empty house rankings = %+v/%+v, want both empty", body.Top, body.Biggest)
	}
	if body.Source.SyncedAt != nil {
		t.Errorf("source syncedAt = %v, want null (never synced)", *body.Source.SyncedAt)
	}
}
