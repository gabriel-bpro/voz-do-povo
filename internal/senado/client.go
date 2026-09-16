package senado

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"voz-do-povo/internal/camara"
)

const (
	defaultAdminBase = "https://adm.senado.gov.br/adm-dadosabertos"
	defaultLegisBase = "https://legis.senado.leg.br/dadosabertos"
)

// Client fetches from the Senado Legislative API (senator registry) and the
// Administrative API (CEAPS expenses).
type Client struct {
	http      *http.Client
	adminBase string
	legisBase string
}

// New returns a Client against the production endpoints.
func New() *Client {
	return &Client{
		http:      &http.Client{Timeout: 5 * time.Minute},
		adminBase: defaultAdminBase,
		legisBase: defaultLegisBase,
	}
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

// senatorList mirrors the lista/atual envelope (deeply nested, capitalized
// keys — the Legislative API's XML-first heritage).
type senatorList struct {
	Lista struct {
		Parlamentares struct {
			Parlamentar []struct {
				Identificacao struct {
					Codigo     string `json:"CodigoParlamentar"`
					Nome       string `json:"NomeParlamentar"`
					NomeCivil  string `json:"NomeCompletoParlamentar"`
					Partido    string `json:"SiglaPartidoParlamentar"`
					UF         string `json:"UfParlamentar"`
				} `json:"IdentificacaoParlamentar"`
			} `json:"Parlamentar"`
		} `json:"Parlamentares"`
	} `json:"ListaParlamentarEmExercicio"`
}

// ListSenators returns the registry of senators currently in exercise. The
// Senado publishes no CPF for senators.
func (c *Client) ListSenators(ctx context.Context) ([]Senator, error) {
	var list senatorList
	if err := c.get(ctx, c.legisBase+"/senador/lista/atual.json", &list); err != nil {
		return nil, err
	}
	var out []Senator
	for _, p := range list.Lista.Parlamentares.Parlamentar {
		id := p.Identificacao
		var code int64
		if _, err := fmt.Sscan(id.Codigo, &code); err != nil {
			return nil, fmt.Errorf("parse senator code %q: %w", id.Codigo, err)
		}
		out = append(out, Senator{
			Code:       code,
			Name:       id.Nome,
			CivilName:  id.NomeCivil,
			PartySigla: id.Partido,
			UF:         id.UF,
		})
	}
	return out, nil
}

// ceapsRow is one row of the CEAPS year dataset. JSON keys are camelCase;
// amounts are plain numbers; dates are ISO. Verified against the live 2026
// dataset (2026-09-15).
type ceapsRow struct {
	ID            int64   `json:"id"`
	Ano           int     `json:"ano"`
	Mes           int     `json:"mes"`
	CodSenador    int64   `json:"codSenador"`
	NomeSenador   string  `json:"nomeSenador"`
	TipoDespesa   string  `json:"tipoDespesa"`
	CPFCNPJ       string  `json:"cpfCnpj"`
	Fornecedor    string  `json:"fornecedor"`
	Documento     string  `json:"documento"`
	Data          string  `json:"data"`
	Detalhamento  string  `json:"detalhamento"`
	ValorReemb    float64 `json:"valorReembolsado"`
}

func (r ceapsRow) normalize() Expense {
	return Expense{
		ID:             r.ID,
		SenatorCode:    r.CodSenador,
		SenatorName:    r.NomeSenador,
		Category:       r.TipoDespesa,
		SupplierName:   r.Fornecedor,
		SupplierTaxID:  camara.DigitsOnly(r.CPFCNPJ),
		DocumentNumber: r.Documento,
		IssuedAt:       r.Data,
		Year:           r.Ano,
		Month:          r.Mes,
		Detail:         r.Detalhamento,
		AmountNet:      r.ValorReemb,
	}
}

// FetchExpenses downloads the CEAPS year dataset (JSON; a CSV variant exists
// at the same path + /csv, but it is latin-1 with decimal commas — JSON is
// the lossless path). The dataset is updated daily. Rows are returned
// normalized, each keeping its verbatim bytes for the raw layer.
func (c *Client) FetchExpenses(ctx context.Context, year int) ([]Expense, error) {
	url := fmt.Sprintf("%s/api/v1/senadores/despesas_ceaps/%d", c.adminBase, year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}

	// The payload is one JSON array; stream it row by row.
	dec := json.NewDecoder(resp.Body)
	if _, err := dec.Token(); err != nil { // opening '['
		return nil, fmt.Errorf("decode ceaps array start: %w", err)
	}
	var out []Expense
	for dec.More() {
		// Two-step: keep the row's verbatim bytes for the raw layer, then
		// normalize.
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("decode ceaps row: %w", err)
		}
		var row ceapsRow
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, fmt.Errorf("parse ceaps row: %w", err)
		}
		exp := row.normalize()
		exp.Raw = raw
		out = append(out, exp)
	}
	return out, nil
}
