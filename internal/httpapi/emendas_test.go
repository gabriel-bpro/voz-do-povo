package httpapi_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Seam 1: the profile emendas payload — yearly empenhado/pago totals,
// breakdowns by policy area and localidade, top 5 with the PIX type marker,
// and the source + sync date metadata.

func seedEmenda(t *testing.T, db *sql.DB, politicianID *int64, code string, year int, tipo string, isPix bool, localidade, funcao string, empenhado, pago float64) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO curated.emenda (
			politician_id, author_name, code, year, tipo, is_pix,
			localidade, funcao, amount_empenhado, amount_pago
		) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),$9,$10)`,
		politicianID, "AUTOR FIXTURE", code, year, tipo, isPix,
		localidade, funcao, empenhado, pago)
	if err != nil {
		t.Fatalf("seed emenda %s: %v", code, err)
	}
}

type emendaSectionView struct {
	Linked bool `json:"linked"`
	Year   int  `json:"year"`
	Years  []struct {
		Year      int     `json:"year"`
		Empenhado float64 `json:"empenhado"`
		Pago      float64 `json:"pago"`
	} `json:"years"`
	ByArea []struct {
		Label string  `json:"label"`
		Total float64 `json:"total"`
	} `json:"byArea"`
	ByLocalidade []struct {
		Label string  `json:"label"`
		Total float64 `json:"total"`
	} `json:"byLocalidade"`
	Top []struct {
		Code       string  `json:"code"`
		IsPIX      bool    `json:"isPix"`
		Localidade *string `json:"localidade"`
		Empenhado  float64 `json:"empenhado"`
		Pago       float64 `json:"pago"`
	} `json:"top"`
	Badge *struct {
		PixPercent int `json:"pixPercent"`
		Year       int `json:"year"`
	} `json:"badge"`
	Source struct {
		Key      string  `json:"key"`
		Name     string  `json:"name"`
		SyncedAt *string `json:"syncedAt"`
	} `json:"source"`
	Message string `json:"message"`
}

func getEmendas(t *testing.T, srvURL string, id int64, year int) emendaSectionView {
	t.Helper()
	res := getJSON(t, fmt.Sprintf("%s/api/politicians/%d?year=%d", srvURL, id, year))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)
	var body struct {
		Emendas emendaSectionView `json:"emendas"`
	}
	mustDecode(t, res, &body)
	return body.Emendas
}

func TestProfileEmendasPayload(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 7, "Carlos Pereira Souza", "Carlos Souza", "MG", "55566677788")
	seedSourceID(t, db, 7, "camara", "204402")
	pid := int64(7)

	thisYear := time.Now().Year()
	seedEmenda(t, db, &pid, "E001", thisYear, "Emenda Individual", false, "Taubaté - SP", "Saúde", 1500000, 600000)
	seedEmenda(t, db, &pid, "E002", thisYear, "Transferência Especial", true, "Fundo municipal de Taubaté - SP", "", 2500000, 2500000)
	seedEmenda(t, db, &pid, "E003", thisYear, "Emenda Individual", false, "Rio de Janeiro - RJ", "Educação", 750000, 0)
	seedEmenda(t, db, &pid, "E004", 2025, "Emenda Individual", false, "Manaus - AM", "Saúde", 900000, 900000)

	em := getEmendas(t, srv.URL, 7, thisYear)

	if !em.Linked {
		t.Fatalf("emendas.linked = false, want true")
	}
	if len(em.Years) != 2 {
		t.Fatalf("years = %+v, want 2 (%d and 2025)", em.Years, thisYear)
	}
	if em.Years[1].Year != thisYear || em.Years[1].Empenhado != 4750000 || em.Years[1].Pago != 3100000 {
		t.Errorf("years[1] = %+v, want %d: 4750000 empenhado, 3100000 pago", em.Years[1], thisYear)
	}
	if em.Years[0].Year != 2025 || em.Years[0].Empenhado != 900000 {
		t.Errorf("years[0] = %+v, want 2025: 900000 empenhado", em.Years[0])
	}

	// Breakdowns are scoped to the selected year.
	if len(em.ByArea) != 3 { // "Não informado" (E002), Saúde, Educação
		t.Errorf("byArea = %+v, want 3 rows (incl. the not-informed bucket)", em.ByArea)
	} else if em.ByArea[0].Label != "Não informado" || em.ByArea[0].Total != 2500000 {
		t.Errorf("byArea[0] = %+v, want Não informado 2500000 (desc)", em.ByArea[0])
	}
	if len(em.ByLocalidade) != 3 || em.ByLocalidade[0].Label != "Fundo municipal de Taubaté - SP" {
		t.Errorf("byLocalidade = %+v, want the 2500000 fund first", em.ByLocalidade)
	}

	// Top 5: ordered by empenhado desc, PIX flagged as a type marker.
	if len(em.Top) != 3 {
		t.Fatalf("top = %+v, want the 3 emendas of %d", em.Top, thisYear)
	}
	if em.Top[0].Code != "E002" || !em.Top[0].IsPIX {
		t.Errorf("top[0] = %+v, want E002 with isPix", em.Top[0])
	}
	if em.Top[1].IsPIX || em.Top[2].IsPIX {
		t.Errorf("non-PIX emendas carry isPix: %+v", em.Top)
	}
	if em.Top[0].Localidade == nil || *em.Top[0].Localidade != "Fundo municipal de Taubaté - SP" {
		t.Errorf("top[0].localidade = %v, want the fund (beneficiary stand-in)", em.Top[0].Localidade)
	}

	// Provenance: source name + sync date.
	if em.Source.Key != "transparencia-emendas" || em.Source.Name == "" {
		t.Errorf("emendas source = %+v, want the Portal da Transparência block", em.Source)
	}
	if em.Source.SyncedAt == nil {
		t.Errorf("emendas source missing sync date")
	}
}

// Rule 3: the emendas section badges when >= 50% of the politician's
// current-calendar-year empenhado is emenda PIX. The badge reports the actual
// (rounded) percentage and is computed on the current year regardless of the
// ?year= the section is displaying.
func seedEmendaPixShare(t *testing.T, db *sql.DB, pid int64, pixEmpenhado, otherEmpenhado float64) {
	t.Helper()
	thisYear := time.Now().Year()
	seedEmenda(t, db, &pid, "PIX1", thisYear, "Transferência Especial", true, "Fundo X", "", pixEmpenhado, 0)
	seedEmenda(t, db, &pid, "OTH1", thisYear, "Emenda Individual", false, "Cidade Y", "Saúde", otherEmpenhado, 0)
}

func TestEmendasPixBadgeTripsAboveHalf(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 21, "Pix Pesado", "Pix Pesado", "SP", "11122233344")
	seedEmendaPixShare(t, db, 21, 6000, 4000) // 60%

	em := getEmendas(t, srv.URL, 21, time.Now().Year())
	if em.Badge == nil {
		t.Fatalf("badge = nil, want trip at 60%% PIX")
	}
	if em.Badge.PixPercent != 60 || em.Badge.Year != time.Now().Year() {
		t.Errorf("badge = %+v, want 60%% of %d", em.Badge, time.Now().Year())
	}
}

func TestEmendasPixBadgeTripsAtExactlyHalf(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 22, "Pix Metade", "Pix Metade", "SP", "22233344455")
	seedEmendaPixShare(t, db, 22, 5000, 5000) // 50%

	em := getEmendas(t, srv.URL, 22, time.Now().Year())
	if em.Badge == nil {
		t.Fatalf("badge = nil, want trip at exactly 50%% PIX")
	}
	if em.Badge.PixPercent != 50 {
		t.Errorf("badge.pixPercent = %d, want 50", em.Badge.PixPercent)
	}
}

func TestEmendasPixBadgeStaysQuietBelowHalf(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 23, "Pix Leve", "Pix Leve", "SP", "33344455566")
	seedEmendaPixShare(t, db, 23, 4999, 5001) // 49.97%

	em := getEmendas(t, srv.URL, 23, time.Now().Year())
	if em.Badge != nil {
		t.Errorf("badge = %+v, want nil below 50%% PIX", em.Badge)
	}
}

func TestEmendasPixBadgeUsesCurrentYearNotSelectedYear(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 24, "Pix Ano Errado", "Pix Ano", "SP", "44455566677")
	pid := int64(24)
	thisYear := time.Now().Year()
	// Current year is 100% PIX (trips); the selected year below is 0% PIX.
	seedEmenda(t, db, &pid, "CUR1", thisYear, "Transferência Especial", true, "Fundo X", "", 1000, 0)
	seedEmenda(t, db, &pid, "OLD1", thisYear-1, "Emenda Individual", false, "Cidade Y", "Saúde", 9000, 0)

	em := getEmendas(t, srv.URL, 24, thisYear-1)
	if em.Badge == nil || em.Badge.PixPercent != 100 || em.Badge.Year != thisYear {
		t.Errorf("badge = %+v, want 100%% of %d even with ?year=%d", em.Badge, thisYear, thisYear-1)
	}
}

// The mirror: heavy PIX last year must not badge a clean current year.
func TestEmendasPixBadgeIgnoresPastYears(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 25, "Pix Passado", "Pix Passado", "SP", "55566677788")
	pid := int64(25)
	thisYear := time.Now().Year()
	seedEmenda(t, db, &pid, "OLDP", thisYear-1, "Transferência Especial", true, "Fundo X", "", 9000, 0)
	seedEmenda(t, db, &pid, "CURC", thisYear, "Emenda Individual", false, "Cidade Y", "Saúde", 1000, 0)

	em := getEmendas(t, srv.URL, 25, thisYear-1)
	if em.Badge != nil {
		t.Errorf("badge = %+v, want nil — last year's PIX must not trip the badge", em.Badge)
	}
}

func TestEmendasPixBadgeNeedsEmpenhado(t *testing.T) {
	db, srv := newSeededAPI(t)
	seedPolitician(t, db, 26, "Sem Empenho", "Sem Empenho", "SP", "66677788899")
	pid := int64(26)
	// Linked (row exists) but zero empenhado in the current year.
	seedEmenda(t, db, &pid, "ZERO", time.Now().Year(), "Transferência Especial", true, "Fundo X", "", 0, 0)

	em := getEmendas(t, srv.URL, 26, time.Now().Year())
	if em.Badge != nil {
		t.Errorf("badge = %+v, want nil with zero current-year empenhado", em.Badge)
	}
}

func TestProfileEmendasNotLinked(t *testing.T) {
	db, srv := newSeededAPI(t)
	// Politician exists but has no linked emendas.
	seedPolitician(t, db, 9, "Sem Emenda Nenhuma", "Sem Emenda", "BA", "99988877766")

	res := getJSON(t, fmt.Sprintf("%s/api/politicians/9", srv.URL))
	defer res.Body.Close()
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Emendas emendaSectionView `json:"emendas"`
	}
	mustDecode(t, res, &body)
	if body.Emendas.Linked {
		t.Fatalf("emendas.linked = true, want false for a politician with no linked emendas")
	}
	if body.Emendas.Message != "Ainda não conseguimos ligar as emendas desta pessoa à fonte oficial com segurança. Não mostramos números sem essa confirmação." {
		t.Errorf("unlinked message = %q, want exact pt-BR template", body.Emendas.Message)
	}
	if body.Emendas.Source.Key != "transparencia-emendas" {
		t.Errorf("unlinked emendas source = %+v, want the Portal da Transparência block", body.Emendas.Source)
	}
}
