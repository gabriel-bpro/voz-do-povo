package httpapi_test

import (
	"fmt"
	"net/http"
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
			Linked         bool   `json:"linked"`
			Year           int    `json:"year"`
			AvailableYears []int  `json:"availableYears"`
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

func TestProfileRejectsYearBeforeTerm(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")

	res := getJSON(t, srv.URL+"/api/politicians/7?year=2019")
	defer res.Body.Close()
	mustStatus(t, res, http.StatusBadRequest)
}
