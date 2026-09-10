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

// int64OrEmpty renders 0 as "" (absent identifier).
func int64OrEmpty(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

// bulkExpenseRow is one row of the Ano-{year}.json CEAP bulk file. The JSON
// file deviates hard from the API's conventions (verified against the real
// 2026 file): camelCase field names, no ideCadastro (only numeroDeputadoID,
// the portal's own id), amounts as strings ("1467"), dates with time
// suffixes, and the parliamentarian's cpf populated on ~99.5% of rows — the
// primary identity-link key.
type bulkExpenseRow struct {
	NomeParlamentar  string      `json:"nomeParlamentar"`
	CPF              string      `json:"cpf"`
	NumeroDeputadoID int64       `json:"numeroDeputadoID"` // portal id, NOT the API id
	SiglaUF          string      `json:"siglaUF"`
	SiglaPartido     string      `json:"siglaPartido"`
	Descricao        string      `json:"descricao"`
	Fornecedor       string      `json:"fornecedor"`
	CNPJCPF          string      `json:"cnpjCPF"`
	Numero           string      `json:"numero"`
	DataEmissao      string      `json:"dataEmissao"`
	ValorDocumento   json.Number `json:"valorDocumento"`
	ValorGlosa       json.Number `json:"valorGlosa"`
	ValorLiquido     json.Number `json:"valorLiquido"`
	Mes              int         `json:"mes"`
	Ano              int         `json:"ano"`
	Parcela          int         `json:"parcela"`
	IDDocumento      int64       `json:"idDocumento"`
	URLDocumento     string      `json:"urlDocumento"`
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
	if err := ParseBulkZip(body, yield); err != nil {
		return fmt.Errorf("bulk %d: %w", year, err)
	}
	return nil
}

// ParseBulkZip decodes a CEAP bulk year zip (a single JSON entry wrapping the
// rows in "dados"), normalizing and yielding each row. Streaming: rows are
// yielded as decoded, so a year file never materializes as a slice.
func ParseBulkZip(body []byte, yield func(Expense) error) error {
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
		// Two-step: keep the row's verbatim bytes for the raw layer, then
		// normalize.
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return fmt.Errorf("decode bulk row: %w", err)
		}
		var row bulkExpenseRow
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("parse bulk row: %w", err)
		}
		exp, err := row.normalize()
		if err != nil {
			return fmt.Errorf("normalize bulk row (idDocumento %d): %w", row.IDDocumento, err)
		}
		exp.Raw = raw
		if err := yield(exp); err != nil {
			return err
		}
	}
	return nil
}

func (r bulkExpenseRow) normalize() (Expense, error) {
	doc, err := parseAmount(r.ValorDocumento)
	if err != nil {
		return Expense{}, fmt.Errorf("valorDocumento: %w", err)
	}
	glosa, err := parseAmount(r.ValorGlosa)
	if err != nil {
		return Expense{}, fmt.Errorf("valorGlosa: %w", err)
	}
	liq, err := parseAmount(r.ValorLiquido)
	if err != nil {
		return Expense{}, fmt.Errorf("valorLiquido: %w", err)
	}
	return Expense{
		// The JSON bulk file has no ideCadastro/API id; CPF and name+UF are
		// the identity keys.
		DeputyCPF:      DigitsOnly(r.CPF),
		DeputyName:     r.NomeParlamentar,
		DeputyUF:       r.SiglaUF,
		PartySigla:     r.SiglaPartido,
		Category:       r.Descricao,
		SupplierName:   r.Fornecedor,
		SupplierTaxID:  DigitsOnly(r.CNPJCPF),
		DocumentNumber: r.Numero,
		IssuedAt:       dateOnly(r.DataEmissao),
		Year:           r.Ano,
		Month:          r.Mes,
		Installment:    r.Parcela,
		AmountDocument: doc,
		AmountGlosa:    glosa,
		AmountNet:      liq,
		DocumentURL:    r.URLDocumento,
		DocumentID:     int64OrEmpty(r.IDDocumento),
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

// parseAmount parses the bulk file's string amounts ("1467", "100.5").
func parseAmount(n json.Number) (float64, error) {
	s := strings.TrimSpace(n.String())
	if s == "" {
		return 0, nil
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
