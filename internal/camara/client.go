package camara

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBase  = "https://dadosabertos.camara.leg.br/api/v2"
	defaultBulkURL  = "https://www.camara.leg.br/cotas/Ano-%d.json.zip"
	bulkPayloadKey  = "dados"
	maxPageItems    = 100
)

// Client fetches from the Câmara REST API and the CEAP bulk files.
type Client struct {
	http    *http.Client
	apiBase string
	// bulkURL is a fmt.Sprintf pattern taking the year.
	bulkURL string
}

// New returns a Client against the production endpoints.
func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 5 * time.Minute},
		apiBase: defaultAPIBase,
		bulkURL: defaultBulkURL,
	}
}

// paged is the API's envelope for list endpoints.
type paged struct {
	Dados json.RawMessage `json:"dados"`
	Links []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

func (c *Client) get(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// getPaged follows the API's next links, decoding each page's dados array with
// decode and appending the results.
func (c *Client) getPaged(ctx context.Context, url string, decode func(json.RawMessage) error) error {
	for url != "" {
		var page paged
		if err := c.get(ctx, url, &page); err != nil {
			return err
		}
		if err := decode(page.Dados); err != nil {
			return err
		}
		next := ""
		for _, l := range page.Links {
			if l.Rel == "next" {
				next = l.Href
			}
		}
		url = next
	}
	return nil
}

type deputyListItem struct {
	ID          int64  `json:"id"`
	Nome        string `json:"nome"`
	SiglaPartido string `json:"siglaPartido"`
	SiglaUF     string `json:"siglaUf"`
}

// ListDeputies returns the registry entries for the given legislatures
// (e.g. 56 and 57 cover 2023+). Detail fields (CPF, civil name) are not
// populated — use GetDeputyDetail.
func (c *Client) ListDeputies(ctx context.Context, legislatures ...int) ([]Deputy, error) {
	var out []Deputy
	for _, leg := range legislatures {
		url := fmt.Sprintf("%s/deputados?idLegislatura=%d&itens=%d&ordem=ASC&ordenarPor=nome",
			c.apiBase, leg, maxPageItems)
		var items []deputyListItem
		err := c.getPaged(ctx, url, func(raw json.RawMessage) error {
			var page []deputyListItem
			if err := json.Unmarshal(raw, &page); err != nil {
				return fmt.Errorf("decode deputies page: %w", err)
			}
			items = append(items, page...)
			return nil
		})
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			out = append(out, Deputy{
				ID:         it.ID,
				Name:       it.Nome,
				PartySigla: it.SiglaPartido,
				UF:         it.SiglaUF,
			})
		}
	}
	return out, nil
}

type deputyDetail struct {
	Dados struct {
		CPF       string `json:"cpf"`
		NomeCivil string `json:"nomeCivil"`
	} `json:"dados"`
}

// FillDetail fetches /deputados/{id} and populates CPF (digits only) and
// CivilName on the deputy.
func (c *Client) FillDetail(ctx context.Context, d *Deputy) error {
	var det deputyDetail
	if err := c.get(ctx, fmt.Sprintf("%s/deputados/%d", c.apiBase, d.ID), &det); err != nil {
		return err
	}
	d.CPF = DigitsOnly(det.Dados.CPF)
	d.CivilName = det.Dados.NomeCivil
	return nil
}

type apiExpenseItem struct {
	Ano               int     `json:"ano"`
	Mes               int     `json:"mes"`
	TipoDespesa       string  `json:"tipoDespesa"`
	CodDocumento      int64   `json:"codDocumento"`
	DataDocumento     string  `json:"dataDocumento"`
	NumDocumento      string  `json:"numDocumento"`
	ValorDocumento    float64 `json:"valorDocumento"`
	URLDocumento      string  `json:"urlDocumento"`
	NomeFornecedor    string  `json:"nomeFornecedor"`
	CNPJCPFFornecedor string  `json:"cnpjCpfFornecedor"`
	ValorLiquido      float64 `json:"valorLiquido"`
	ValorGlosa        float64 `json:"valorGlosa"`
}

// ListExpenses returns one deputy's API expenses for a year+month (the daily
// delta shape).
func (c *Client) ListExpenses(ctx context.Context, deputyID int64, year, month int) ([]Expense, error) {
	url := fmt.Sprintf("%s/deputados/%d/despesas?ano=%d&mes=%d&itens=%d&ordem=ASC&ordenarPor=ano",
		c.apiBase, deputyID, year, month, maxPageItems)
	var out []Expense
	err := c.getPaged(ctx, url, func(raw json.RawMessage) error {
		var page []apiExpenseItem
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("decode expenses page: %w", err)
		}
		for _, it := range page {
			out = append(out, Expense{
				DeputyID:       deputyID,
				Category:       it.TipoDespesa,
				SupplierName:   it.NomeFornecedor,
				SupplierTaxID:  DigitsOnly(it.CNPJCPFFornecedor),
				DocumentNumber: it.NumDocumento,
				IssuedAt:       dateOnly(it.DataDocumento),
				Year:           it.Ano,
				Month:          it.Mes,
				AmountDocument: it.ValorDocumento,
				AmountGlosa:    it.ValorGlosa,
				AmountNet:      it.ValorLiquido,
				DocumentURL:    it.URLDocumento,
				DocumentID:     strconv.FormatInt(it.CodDocumento, 10),
			})
		}
		return nil
	})
	return out, err
}

// bulkExpenseRow is one row of the Ano-{year}.json CEAP bulk file. Amounts may
// arrive as JSON numbers or pt-BR strings ("1.234,56"), hence json.Number.
type bulkExpenseRow struct {
	NomeParlamentar string      `json:"txNomeParlamentar"`
	CPF             string      `json:"cpf"`
	IDEcadastro     int64       `json:"ideCadastro"` // == the API's deputy id
	NuDeputadoID    int64       `json:"nuDeputadoID"`
	SgUF            string      `json:"sgUF"`
	SgPartido       string      `json:"sgPartido"`
	Descricao       string      `json:"txtDescricao"`
	Fornecedor      string      `json:"txtFornecedor"`
	CNPJCPF         string      `json:"txtCNPJCPF"`
	Numero          string      `json:"txtNumero"`
	DatEmissao      string      `json:"datEmissao"`
	VlrDocumento    json.Number `json:"vlrDocumento"`
	VlrGlosa        json.Number `json:"vlrGlosa"`
	VlrLiquido      json.Number `json:"vlrLiquido"`
	Mes             int         `json:"numMes"`
	Ano             int         `json:"numAno"`
	NumParcela      int         `json:"numParcela"`
	IDEDocumento    int64       `json:"ideDocumento"`
	URLDocumento    string      `json:"urlDocumento"`
}

// FetchBulkExpenses downloads the CEAP bulk year file and invokes yield for
// each row, normalized. Streaming: rows are yielded as decoded, so a year file
// never materializes as a slice.
func (c *Client) FetchBulkExpenses(ctx context.Context, year int, yield func(Expense) error) error {
	url := fmt.Sprintf(c.bulkURL, year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read bulk zip: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("open bulk zip: %w", err)
	}
	if len(zr.File) != 1 {
		return fmt.Errorf("bulk zip has %d files, want 1", len(zr.File))
	}
	f, err := zr.File[0].Open()
	if err != nil {
		return fmt.Errorf("open bulk entry: %w", err)
	}
	defer f.Close()

	// The payload is a single JSON object wrapping the rows in "dados".
	dec := json.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("decode bulk envelope: %w", err)
		}
		if key, ok := tok.(string); ok && key == bulkPayloadKey {
			break
		}
	}
	if _, err := dec.Token(); err != nil { // opening '['
		return fmt.Errorf("decode bulk array start: %w", err)
	}
	for dec.More() {
		var row bulkExpenseRow
		if err := dec.Decode(&row); err != nil {
			return fmt.Errorf("decode bulk row: %w", err)
		}
		exp, err := row.normalize()
		if err != nil {
			return fmt.Errorf("normalize bulk row (ideDocumento %d): %w", row.IDEDocumento, err)
		}
		if err := yield(exp); err != nil {
			return err
		}
	}
	return nil
}

func (r bulkExpenseRow) normalize() (Expense, error) {
	doc, err := parseAmount(r.VlrDocumento)
	if err != nil {
		return Expense{}, fmt.Errorf("vlrDocumento: %w", err)
	}
	glosa, err := parseAmount(r.VlrGlosa)
	if err != nil {
		return Expense{}, fmt.Errorf("vlrGlosa: %w", err)
	}
	liq, err := parseAmount(r.VlrLiquido)
	if err != nil {
		return Expense{}, fmt.Errorf("vlrLiquido: %w", err)
	}
	return Expense{
		DeputyID:       r.IDEcadastro, // the API's deputy id; 0 when absent
		DeputyCPF:      DigitsOnly(r.CPF),
		DeputyName:     r.NomeParlamentar,
		DeputyUF:       r.SgUF,
		PartySigla:     r.SgPartido,
		Category:       r.Descricao,
		SupplierName:   r.Fornecedor,
		SupplierTaxID:  DigitsOnly(r.CNPJCPF),
		DocumentNumber: r.Numero,
		IssuedAt:       dateOnly(r.DatEmissao),
		Year:           r.Ano,
		Month:          r.Mes,
		Installment:    r.NumParcela,
		AmountDocument: doc,
		AmountGlosa:    glosa,
		AmountNet:      liq,
		DocumentURL:    r.URLDocumento,
		DocumentID:     int64OrEmpty(r.IDEDocumento),
	}, nil
}

// DigitsOnly strips a CNPJ/CPF to its digits ("12.345.678/0001-90" →
// "12345678000190"). Non-numeric sentinels yield "".
func DigitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// parseAmount accepts JSON numbers and pt-BR formatted strings
// ("1.234,56").
func parseAmount(n json.Number) (float64, error) {
	s := strings.TrimSpace(n.String())
	if s == "" {
		return 0, nil
	}
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse amount %q: %w", n.String(), err)
	}
	return v, nil
}

// dateOnly normalizes "2026-08-01T00:00:00" and friends to "2026-08-01".
func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return ""
}
