package httpapi_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// Seam 1: drive the API over HTTP against a real Postgres seeded at the
// curated layer. Covers politician search, the profile expenses payload, and
// the honest "not linked" state.

// --- search -----------------------------------------------------------------

func TestSearchFindsPoliticianByFuzzyName(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 1, "Maria José da Silva", "Maria da Silva", "SP", "12345678901")
	seedSourceID(t, db, 1, "camara", "204400")

	res := getJSON(t, srv.URL+"/api/politicians/search?q="+urlQuery("maria jose silva"))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Query   string `json:"query"`
		Results []struct {
			ID                int64  `json:"id"`
			ParliamentaryName string `json:"parliamentaryName"`
			UF                string `json:"uf"`
		} `json:"results"`
		Sources []struct {
			Key      string  `json:"key"`
			Name     string  `json:"name"`
			SyncedAt *string `json:"syncedAt"`
		} `json:"sources"`
	}
	mustDecode(t, res, &body)

	if len(body.Results) == 0 {
		t.Fatalf("no results for fuzzy query; body: %+v", body)
	}
	if body.Results[0].ID != 1 {
		t.Errorf("top result id = %d, want 1", body.Results[0].ID)
	}
	if body.Results[0].ParliamentaryName != "Maria da Silva" {
		t.Errorf("top result name = %q", body.Results[0].ParliamentaryName)
	}
	if len(body.Sources) == 0 || body.Sources[0].Name == "" {
		t.Errorf("search response missing source metadata: %+v", body.Sources)
	}
	if body.Sources[0].SyncedAt == nil {
		t.Errorf("search response missing source sync date")
	}
}

func TestSearchMatchesAccentFoldedQuery(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 1, "João Gonçalves", "João Gonçalves", "RJ", "11122233344")
	seedSourceID(t, db, 1, "camara", "204401")

	res := getJSON(t, srv.URL+"/api/politicians/search?q="+urlQuery("joao goncalves"))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Results []struct {
			ID int64 `json:"id"`
		} `json:"results"`
	}
	mustDecode(t, res, &body)
	if len(body.Results) != 1 || body.Results[0].ID != 1 {
		t.Errorf("accent-folded query results = %+v, want the seeded politician", body.Results)
	}
}

func TestSearchRejectsEmptyQuery(t *testing.T) {
	_, srv := newSeededAPI(t)

	res := getJSON(t, srv.URL+"/api/politicians/search")
	defer res.Body.Close()
	mustStatus(t, res, http.StatusBadRequest)
}

// --- profile ----------------------------------------------------------------

// termStart mirrors the API's term-floor for the year selector.
const termStart = 2023

func TestProfileExpensesPayload(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")
	partyID := seedPartyRow(t, db, "PT")

	thisYear := time.Now().Year()
	// 2025: two months, three expenses (one negative glosa).
	seedExpense(t, db, 7, &partyID, 2025, 1, "Combustíveis e lubrificantes", "Posto Estrela", "2025-01-10", 900.00)
	seedExpense(t, db, 7, &partyID, 2025, 1, "Passagens aéreas", "Voe Bem", "2025-01-20", 2500.00)
	seedExpense(t, db, 7, &partyID, 2025, 2, "Locomoção, hospedagem...", "Hotel Serra", "2025-02-05", 1800.00)
	// Current year: one big item + one glosa (negative).
	seedExpense(t, db, 7, &partyID, thisYear, 3, "Passagens aéreas", "Voe Bem", fmt.Sprintf("%d-03-04", thisYear), 4200.00)
	seedExpense(t, db, 7, &partyID, thisYear, 3, "Combustíveis e lubrificantes", "Posto Estrela", fmt.Sprintf("%d-03-15", thisYear), -150.00)

	res := getJSON(t, fmt.Sprintf("%s/api/politicians/7?year=%d", srv.URL, thisYear))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Politician struct {
			ID                int64   `json:"id"`
			CivilName         string  `json:"civilName"`
			ParliamentaryName string  `json:"parliamentaryName"`
			UF                string  `json:"uf"`
			Party             *string `json:"party"`
		} `json:"politician"`
		Summary string `json:"summary"`
		Sources []struct {
			Key      string  `json:"key"`
			Name     string  `json:"name"`
			SyncedAt *string `json:"syncedAt"`
		} `json:"sources"`
		Expenses struct {
			Linked         bool    `json:"linked"`
			Year           int     `json:"year"`
			AvailableYears []int   `json:"availableYears"`
			Total          float64 `json:"total"`
			Monthly        []struct {
				Month int     `json:"month"`
				Total float64 `json:"total"`
			} `json:"monthly"`
			LargestItems []struct {
				Category     string  `json:"category"`
				SupplierName string  `json:"supplierName"`
				IssuedAt     string  `json:"issuedAt"`
				AmountNet    float64 `json:"amountNet"`
			} `json:"largestItems"`
			Source struct {
				Key      string  `json:"key"`
				Name     string  `json:"name"`
				SyncedAt *string `json:"syncedAt"`
			} `json:"source"`
		} `json:"expenses"`
	}
	mustDecode(t, res, &body)

	if body.Politician.ParliamentaryName != "Carlos Souza" {
		t.Errorf("parliamentaryName = %q", body.Politician.ParliamentaryName)
	}
	if body.Politician.Party == nil || *body.Politician.Party != "PT" {
		t.Errorf("party = %v, want PT (from latest expenses)", body.Politician.Party)
	}
	if body.Summary != "Carlos Souza é deputado(a) federal por MG (PT), no mandato 2023–2026." {
		t.Errorf("summary = %q, want exact pt-BR template", body.Summary)
	}
	if !body.Expenses.Linked {
		t.Fatalf("expenses.linked = false, want true")
	}
	if body.Expenses.Total != 4050.00 {
		t.Errorf("total = %v, want 4050 (4200 - 150)", body.Expenses.Total)
	}
	if len(body.Expenses.Monthly) != 12 {
		t.Fatalf("monthly has %d buckets, want 12", len(body.Expenses.Monthly))
	}
	if body.Expenses.Monthly[2].Month != 3 || body.Expenses.Monthly[2].Total != 4050.00 {
		t.Errorf("monthly[2] = %+v, want month 3 total 4050", body.Expenses.Monthly[2])
	}
	if len(body.Expenses.LargestItems) == 0 {
		t.Fatalf("largestItems empty")
	}
	if body.Expenses.LargestItems[0].AmountNet != 4200.00 {
		t.Errorf("largest item = %v, want 4200 (ordered desc)", body.Expenses.LargestItems[0].AmountNet)
	}
	// The selector covers the whole current term (2023+), even years with
	// no expenses at all.
	for y := termStart; y <= thisYear; y++ {
		found := false
		for _, got := range body.Expenses.AvailableYears {
			if got == y {
				found = true
			}
		}
		if !found {
			t.Errorf("availableYears %v missing term year %d", body.Expenses.AvailableYears, y)
		}
	}
	if body.Expenses.Source.Name == "" || body.Expenses.Source.SyncedAt == nil {
		t.Errorf("expenses source metadata incomplete: %+v", body.Expenses.Source)
	}
	if len(body.Sources) == 0 || body.Sources[0].SyncedAt == nil {
		t.Errorf("profile missing registry source metadata: %+v", body.Sources)
	}
}

func TestProfileDefaultsToCurrentYear(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	res := getJSON(t, srv.URL+"/api/politicians/7")
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Expenses struct {
			Year int `json:"year"`
		} `json:"expenses"`
	}
	mustDecode(t, res, &body)
	if body.Expenses.Year != time.Now().Year() {
		t.Errorf("year = %d, want current calendar year %d", body.Expenses.Year, time.Now().Year())
	}
}

func TestProfileUnlinkedSourceRendersNotLinked(t *testing.T) {
	db, srv := newSeededAPI(t)
	// Politician with no Câmara source id and no linked expenses.
	seedPolitician(t, db, 9, "Sem Vínculo Nenhum", "Sem Vínculo", "BA", "99988877766")

	res := getJSON(t, srv.URL+"/api/politicians/9")
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Expenses struct {
			Linked  bool   `json:"linked"`
			Message string `json:"message"`
		} `json:"expenses"`
	}
	mustDecode(t, res, &body)
	if body.Expenses.Linked {
		t.Fatalf("expenses.linked = true, want false for unlinked politician")
	}
	if body.Expenses.Message != "Ainda não conseguimos ligar os gastos desta pessoa à fonte oficial com segurança. Não mostramos números sem essa confirmação." {
		t.Errorf("unlinked message = %q, want exact pt-BR template", body.Expenses.Message)
	}
}

func TestProfileUnknownPolitician(t *testing.T) {
	_, srv := newSeededAPI(t)

	res := getJSON(t, srv.URL+"/api/politicians/424242")
	defer res.Body.Close()
	mustStatus(t, res, http.StatusNotFound)
}

// Seam 1, Senado: a senator profile serves CEAPS data — senador(a) summary
// template, CEAPS source block, and aggregates scoped to house='senado' (a
// same-politician CEAP row from a past mandate is never mixed in).
func TestProfileSenatorExpensesPayload(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 11, "Ana Mostra Souza", "Ana Mostra", "MG", "77766655544")
	seedSourceID(t, db, 11, "senado", "9002")
	partyID := seedPartyRow(t, db, "PSDB")

	thisYear := time.Now().Year()
	at := fmt.Sprintf("%d-03-04", thisYear)
	seedExpenseHouse(t, db, "senado", 11, &partyID, thisYear, 3, "Passagens aéreas, aquáticas e terrestres nacionais", "Aerotur", at, 1315.35)
	// A CEAP row from the same canonical person's past: excluded from the
	// senator view (quotas never mix).
	seedExpenseHouse(t, db, "camara", 11, &partyID, thisYear, 3, "Combustíveis e lubrificantes", "Posto", at, 900.00)

	res := getJSON(t, fmt.Sprintf("%s/api/politicians/11?year=%d", srv.URL, thisYear))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Summary string `json:"summary"`
		Sources []struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"sources"`
		Expenses struct {
			Linked       bool    `json:"linked"`
			Total        float64 `json:"total"`
			LargestItems []struct {
				Category  string  `json:"category"`
				AmountNet float64 `json:"amountNet"`
			} `json:"largestItems"`
			Source struct {
				Key      string  `json:"key"`
				Name     string  `json:"name"`
				SyncedAt *string `json:"syncedAt"`
			} `json:"source"`
		} `json:"expenses"`
	}
	mustDecode(t, res, &body)

	if body.Summary != "Ana Mostra é senador(a) da República por MG (PSDB)." {
		t.Errorf("summary = %q, want exact pt-BR senador(a) template", body.Summary)
	}
	if !body.Expenses.Linked {
		t.Fatalf("expenses.linked = false, want true (senado source id)")
	}
	if body.Expenses.Total != 1315.35 {
		t.Errorf("total = %v, want 1315.35 (CEAP row excluded)", body.Expenses.Total)
	}
	if len(body.Expenses.LargestItems) != 1 {
		t.Errorf("largestItems = %+v, want only the CEAPS row", body.Expenses.LargestItems)
	}
	if body.Expenses.Source.Key != "senado-ceaps" || body.Expenses.Source.Name == "" {
		t.Errorf("expenses source = %+v, want the CEAPS block", body.Expenses.Source)
	}
	if body.Expenses.Source.SyncedAt == nil {
		t.Errorf("expenses source missing sync date")
	}
	if len(body.Sources) != 1 || body.Sources[0].Key != "senado-senators" {
		t.Errorf("profile sources = %+v, want the senado registry block", body.Sources)
	}
}

// Seam 1, Senado: the median badge compares against the SENADO bucket only —
// a cheap Câmara bucket of the same category must not make a senator's item
// badge.
func TestBadgeBucketsArePerHouse(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 11, "Ana Mostra Souza", "Ana Mostra", "MG", "77766655544")
	seedSourceID(t, db, 11, "senado", "9002")

	// Same category label in both houses: Câmara median 100, Senado median
	// 5000. Cross-house mixing would badge the 3000 item; per-house it must
	// not.
	seedBucketExpensesHouse(t, db, "camara", "Passagens", 100.00, 30)
	seedBucketExpensesHouse(t, db, "senado", "Passagens", 5000.00, 30)

	now := time.Now()
	at := now.Format("2006-01-02")
	seedExpenseHouse(t, db, "senado", 11, nil, now.Year(), int(now.Month()), "Passagens", "Sob a Mediana", at, 3000.00)
	seedExpenseHouse(t, db, "senado", 11, nil, now.Year(), int(now.Month()), "Passagens", "Acima da Mediana", at, 6000.00)

	exp := getExpenses(t, srv.URL, 11)

	if it := findItem(t, exp.LargestItems, 3000.00); it.Badge != nil {
		t.Errorf("item 3000: badge = %+v, want absent (senado bucket median is 5000)", it.Badge)
	}
	it := findItem(t, exp.LargestItems, 6000.00)
	if it.Badge == nil {
		t.Fatalf("item 6000: badge absent, want median 5000 (senado bucket)")
	}
	if it.Badge.Median != 5000.00 {
		t.Errorf("item 6000: badge median = %v, want 5000 (senado bucket)", it.Badge.Median)
	}
	if exp.FlaggedCount != 1 {
		t.Errorf("flaggedCount = %d, want 1", exp.FlaggedCount)
	}
}

func TestProfileRejectsYearBeforeTerm(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	res := getJSON(t, srv.URL+"/api/politicians/7?year=2019")
	defer res.Body.Close()
	mustStatus(t, res, http.StatusBadRequest)
}

// --- attention badge (rule 1: above bucket median) --------------------------

// seedBucketExpenses fills a house×category bucket with n items of the same
// amount, issued last month (safely inside the trailing-12-month window).
// Rows stay unlinked (politician_id NULL): buckets span the whole house,
// linked or not. The supplier name carries the amount so repeated calls for
// one category never collide on the dedup key.
func seedBucketExpenses(t *testing.T, db *sql.DB, category string, amount float64, n int) {
	t.Helper()
	seedBucketExpensesHouse(t, db, "camara", category, amount, n)
}

// seedBucketExpensesHouse is seedBucketExpenses for a given house.
func seedBucketExpensesHouse(t *testing.T, db *sql.DB, house, category string, amount float64, n int) {
	t.Helper()
	base := time.Now().AddDate(0, -1, 0)
	for i := 0; i < n; i++ {
		d := base.AddDate(0, 0, -(i % 25))
		supplier := fmt.Sprintf("Fornecedor %s %v %02d", category, amount, i)
		_, err := db.Exec(`
			INSERT INTO curated.expense (
				politician_id, house, category, supplier_name, issued_at,
				year, month, amount_net, source_row_key
			) VALUES (NULL,$1,$2,$3,$4,$5,$6,$7,$8)`,
			house, category, supplier, d.Format("2006-01-02"),
			d.Year(), int(d.Month()), amount,
			house+url.QueryEscape(supplier)+d.Format("2006-01-02"))
		if err != nil {
			t.Fatalf("seed bucket expense (%s/%s/%v/%d): %v", house, category, amount, i, err)
		}
	}
}

type badgeItem struct {
	Category  string  `json:"category"`
	AmountNet float64 `json:"amountNet"`
	Badge     *struct {
		Median float64 `json:"median"`
	} `json:"badge"`
}

func getExpenses(t *testing.T, srvURL string, id int64) struct {
	Linked       bool        `json:"linked"`
	FlaggedCount int         `json:"flaggedCount"`
	LargestItems []badgeItem `json:"largestItems"`
} {
	t.Helper()
	res := getJSON(t, fmt.Sprintf("%s/api/politicians/%d", srvURL, id))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)
	var body struct {
		Expenses struct {
			Linked       bool        `json:"linked"`
			FlaggedCount int         `json:"flaggedCount"`
			LargestItems []badgeItem `json:"largestItems"`
		} `json:"expenses"`
	}
	mustDecode(t, res, &body)
	return body.Expenses
}

func findItem(t *testing.T, items []badgeItem, amount float64) badgeItem {
	t.Helper()
	for _, it := range items {
		if it.AmountNet == amount {
			return it
		}
	}
	t.Fatalf("no expense item with amount %v in %+v", amount, items)
	return badgeItem{}
}

func TestBadgeTripsAboveMedian(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	// Bucket: 30 items at R$ 100 → median 100.
	seedBucketExpenses(t, db, "Consultoria jurídica", 100.00, 30)

	now := time.Now()
	at := now.Format("2006-01-02")
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Consultoria jurídica", "Alvo Grande", at, 5000.00)
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Consultoria jurídica", "Alvo Médio", at, 3000.00)
	// Below the R$ 1.000 floor: above the median but never badges.
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Consultoria jurídica", "Alvo Pequeno", at, 500.00)

	exp := getExpenses(t, srv.URL, 7)

	for _, amount := range []float64{5000.00, 3000.00} {
		it := findItem(t, exp.LargestItems, amount)
		if it.Badge == nil {
			t.Errorf("item %v: badge absent, want median 100", amount)
		} else if it.Badge.Median != 100.00 {
			t.Errorf("item %v: badge median = %v, want 100", amount, it.Badge.Median)
		}
	}
	if it := findItem(t, exp.LargestItems, 500.00); it.Badge != nil {
		t.Errorf("item 500 (below floor): badge = %+v, want absent", it.Badge)
	}
	if exp.FlaggedCount != 2 {
		t.Errorf("flaggedCount = %d, want 2", exp.FlaggedCount)
	}
}

func TestBadgeRespectsThousandFloor(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	seedBucketExpenses(t, db, "Passagens espaciais", 100.00, 30)

	now := time.Now()
	at := now.Format("2006-01-02")
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Passagens espaciais", "Sob o Piso", at, 999.99)
	// Exactly R$ 1.000 is >= the floor: badges.
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Passagens espaciais", "No Piso", at, 1000.00)

	exp := getExpenses(t, srv.URL, 7)

	if it := findItem(t, exp.LargestItems, 999.99); it.Badge != nil {
		t.Errorf("item 999.99: badge = %+v, want absent (below floor)", it.Badge)
	}
	if it := findItem(t, exp.LargestItems, 1000.00); it.Badge == nil {
		t.Errorf("item 1000.00: badge absent, want present (floor is inclusive)")
	}
	if exp.FlaggedCount != 1 {
		t.Errorf("flaggedCount = %d, want 1", exp.FlaggedCount)
	}
}

func TestBadgeRequiresThirtyItemBucket(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	// One short of the minimum (28 donor + the target item = 29): the
	// bucket never badges.
	seedBucketExpenses(t, db, "Teletransporte", 100.00, 28)

	now := time.Now()
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Teletransporte", "Caro Demais", now.Format("2006-01-02"), 5000.00)

	exp := getExpenses(t, srv.URL, 7)

	if it := findItem(t, exp.LargestItems, 5000.00); it.Badge != nil {
		t.Errorf("item 5000: badge = %+v, want absent (bucket has 29 items)", it.Badge)
	}
	if exp.FlaggedCount != 0 {
		t.Errorf("flaggedCount = %d, want 0", exp.FlaggedCount)
	}
}

func TestBadgeExcludesNegativesFromMedian(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	// Positives: 15×100 + 15×200, plus the target item (1200) → 31 values,
	// median = 200 (16th of 31). Glosas/ressarcimentos must not drag the
	// median down: wrongly including the 20 negatives would give 100.
	seedBucketExpenses(t, db, "Holograma de campanha", 100.00, 15)
	seedBucketExpenses(t, db, "Holograma de campanha", 200.00, 15)
	seedBucketExpenses(t, db, "Holograma de campanha", -9999.00, 20)

	now := time.Now()
	at := now.Format("2006-01-02")
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Holograma de campanha", "Acima", at, 1200.00)
	// Negative items never badge, however large in magnitude.
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Holograma de campanha", "Glosa Grande", at, -5000.00)

	exp := getExpenses(t, srv.URL, 7)

	it := findItem(t, exp.LargestItems, 1200.00)
	if it.Badge == nil {
		t.Fatalf("item 1200: badge absent, want median 200")
	}
	if it.Badge.Median != 200.00 {
		t.Errorf("badge median = %v, want 200 (negatives excluded from the sample)", it.Badge.Median)
	}
	for _, other := range exp.LargestItems {
		if other.AmountNet < 0 && other.Badge != nil {
			t.Errorf("negative item %v: badge = %+v, want absent", other.AmountNet, other.Badge)
		}
	}
	if exp.FlaggedCount != 1 {
		t.Errorf("flaggedCount = %d, want 1", exp.FlaggedCount)
	}
}

func TestBadgeIgnoresItemsOutsideTrailingWindow(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	// 28 recent + 5 ancient (13 months out) + the target item: the window
	// keeps the bucket at 29 — no badge. Without the window filter, n = 34
	// and the item would badge.
	seedBucketExpenses(t, db, "Máquina do tempo", 100.00, 28)
	seedPolitician(t, db, 8, "Doador de Balde", "Doador Balde", "SP", "11100011100")
	old := time.Now().AddDate(0, -13, 0)
	for i := 0; i < 5; i++ {
		d := old.AddDate(0, 0, -i)
		seedExpense(t, db, 8, nil, d.Year(), int(d.Month()),
			"Máquina do tempo", fmt.Sprintf("Antigo %02d", i), d.Format("2006-01-02"), 100.00)
	}

	now := time.Now()
	seedExpense(t, db, 7, nil, now.Year(), int(now.Month()), "Máquina do tempo", "Caro Demais", now.Format("2006-01-02"), 5000.00)

	exp := getExpenses(t, srv.URL, 7)

	if it := findItem(t, exp.LargestItems, 5000.00); it.Badge != nil {
		t.Errorf("item 5000: badge = %+v, want absent (window excludes 13-month-old items)", it.Badge)
	}
}
