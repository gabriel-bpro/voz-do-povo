package transparencia

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

const (
	defaultDownloadURL = "https://portaldatransparencia.gov.br/download-de-dados/emendas/%d"
	defaultAPIBase     = "https://api.portaldatransparencia.gov.br/api-de-dados"
	// apiPagePause paces the paginated API walk under the documented rate
	// limit (400 req/min daytime): a full year is ~420 pages.
	apiPagePause = 150 * time.Millisecond
)

// Client fetches the Portal da Transparência emendas dataset. Two paths:
// the REST API (needs a registered e-mail key; works from datacenters) and
// the bulk spreadsheet (no key, but Akamai-blocked from datacenters —
// verified 2026-09-22, same wall as TSE).
type Client struct {
	http *http.Client
	// downloadURL is a fmt.Sprintf pattern taking the year.
	downloadURL string
	apiBase     string
}

// New returns a Client against the production endpoints.
func New() *Client {
	return &Client{
		http:        &http.Client{Timeout: 10 * time.Minute},
		downloadURL: defaultDownloadURL,
		apiBase:     defaultAPIBase,
	}
}

// emendaAPIRow is one row of the /emendas API response.
type emendaAPIRow struct {
	CodigoEmenda string `json:"codigoEmenda"`
	Ano          int    `json:"ano"`
	TipoEmenda   string `json:"tipoEmenda"`
	NomeAutor    string `json:"nomeAutor"`
	NumeroEmenda string `json:"numeroEmenda"`
	Localidade   string `json:"localidadeDoGasto"`
	Funcao       string `json:"funcao"`
	Subfuncao    string `json:"subfuncao"`
	Empenhado    string `json:"valorEmpenhado"`
	Liquidado    string `json:"valorLiquidado"`
	Pago         string `json:"valorPago"`
}

// FetchEmendasAPI walks the /emendas API (keyed, 15 rows/page) until an empty
// page and returns every emenda of the year, normalized.
func (c *Client) FetchEmendasAPI(ctx context.Context, year int, apiKey string) ([]Emenda, error) {
	var out []Emenda
	for page := 1; ; page++ {
		url := fmt.Sprintf("%s/emendas?ano=%d&pagina=%d", c.apiBase, year, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("chave-api-dados", apiKey)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("get %s: %w", url, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("get %s: status %d", url, resp.StatusCode)
		}
		var raws []json.RawMessage
		if err := json.Unmarshal(body, &raws); err != nil {
			return nil, fmt.Errorf("decode %s: %w", url, err)
		}
		if len(raws) == 0 {
			return out, nil
		}
		for _, raw := range raws {
			var row emendaAPIRow
			if err := json.Unmarshal(raw, &row); err != nil {
				return nil, fmt.Errorf("parse api row (page %d): %w", page, err)
			}
			e, err := row.normalize()
			if err != nil {
				return nil, fmt.Errorf("normalize api row %q: %w", row.CodigoEmenda, err)
			}
			e.Raw = raw
			out = append(out, e)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(apiPagePause):
		}
	}
}

func (r emendaAPIRow) normalize() (Emenda, error) {
	e := Emenda{
		Code:       r.CodigoEmenda,
		Year:       r.Ano,
		Tipo:       r.TipoEmenda,
		AuthorName: r.NomeAutor,
		Number:     r.NumeroEmenda,
		Localidade: r.Localidade,
		Funcao:     r.Funcao,
		Subfuncao:  r.Subfuncao,
	}
	if e.Code == "" || e.Year == 0 || e.Tipo == "" {
		return Emenda{}, &ParseError{Field: "code/year/tipo", Value: r.CodigoEmenda, Err: fmt.Errorf("missing natural key fields")}
	}
	var err error
	if e.Empenhado, err = ParseBRMoney(r.Empenhado); err != nil {
		return Emenda{}, err
	}
	if e.Liquidado, err = ParseBRMoney(r.Liquidado); err != nil {
		return Emenda{}, err
	}
	if e.Pago, err = ParseBRMoney(r.Pago); err != nil {
		return Emenda{}, err
	}
	return e, nil
}

// FetchEmendas downloads the emendas parlamentares bulk spreadsheet for a
// year (a zip holding one CSV) and returns every row, normalized.
func (c *Client) FetchEmendas(ctx context.Context, year int) ([]Emenda, error) {
	url := fmt.Sprintf(c.downloadURL, year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read emendas zip: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("open emendas zip: %w", err)
	}
	if len(zr.File) != 1 {
		return nil, fmt.Errorf("emendas zip has %d files, want 1", len(zr.File))
	}
	f, err := zr.File[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open emendas entry: %w", err)
	}
	defer f.Close()
	return ParseEmendasCSV(f)
}

// Column lookup is header-driven: the portal's dictionary names are matched
// after normalization (accent-folded, case-insensitive), so header casing and
// accents may drift without breaking the parse.
var emendaColumns = map[string]string{
	"CODIGO DA EMENDA":        "code",
	"ANO DA EMENDA":           "year",
	"TIPO DE EMENDA":          "tipo",
	"AUTOR DA EMENDA":         "authorCode",
	"NOME DO AUTOR DA EMENDA": "authorName",
	"NUMERO DA EMENDA":        "number",
	"LOCALIDADE DO GASTO":     "localidade",
	"FUNCAO":                  "funcao",
	"SUBFUNCAO":               "subfuncao",
	"EMPENHADO (R$)":          "empenhado",
	"LIQUIDADO (R$)":          "liquidado",
	"PAGO (R$)":               "pago",
}

// ParseEmendasCSV parses the emendas spreadsheet: semicolon-delimited,
// ISO-8859-1 encoded, pt-BR decimal commas. Header row required.
func ParseEmendasCSV(r io.Reader) ([]Emenda, error) {
	// The portal publishes Latin-1; decode to UTF-8 before csv parsing.
	decoded := charmap.ISO8859_1.NewDecoder().Reader(r)
	cr := csv.NewReader(decoded)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1 // tolerate ragged trailing columns

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	cols := make([]string, len(header))
	for i, h := range header {
		h = strings.TrimPrefix(h, "\ufeff") // BOM, if present
		// Unknown columns are ignored: the portal adds columns over time,
		// and a new column must not kill the weekly sync.
		cols[i] = emendaColumns[NormalizeHeader(h)]
	}
	// …but the columns we depend on must all be there: a renamed required
	// column is a real format change and must fail loudly.
	for _, required := range []string{"code", "year", "tipo", "authorName", "localidade", "funcao", "empenhado", "pago"} {
		found := false
		for _, c := range cols {
			if c == required {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("required column %q missing from header %v", required, header)
		}
	}

	var out []Emenda
	line := 1
	for {
		rec, err := cr.Read()
		line++
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", line, err)
		}
		e, err := rowToEmenda(cols, rec)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", line, err)
		}
		// Raw layer: the row's verbatim fields as a JSON object.
		raw := map[string]string{}
		for i, h := range header {
			if i < len(rec) {
				raw[h] = rec[i]
			}
		}
		e.Raw, _ = json.Marshal(raw)
		out = append(out, e)
	}
	return out, nil
}

func rowToEmenda(cols, rec []string) (Emenda, error) {
	get := func(name string) string {
		for i, c := range cols {
			if c == name && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
		}
		return ""
	}

	var e Emenda
	e.Code = get("code")
	if e.Code == "" {
		return Emenda{}, &ParseError{Field: "code", Value: "", Err: fmt.Errorf("empty código da emenda")}
	}
	year, err := strconv.Atoi(get("year"))
	if err != nil {
		return Emenda{}, &ParseError{Field: "year", Value: get("year"), Err: err}
	}
	e.Year = year
	e.Tipo = get("tipo")
	if e.Tipo == "" {
		return Emenda{}, &ParseError{Field: "tipo", Value: "", Err: fmt.Errorf("empty tipo de emenda")}
	}
	e.AuthorCode = get("authorCode")
	e.AuthorName = get("authorName")
	e.Number = get("number")
	e.Localidade = get("localidade")
	e.Funcao = get("funcao")
	e.Subfuncao = get("subfuncao")
	if e.Empenhado, err = ParseBRMoney(get("empenhado")); err != nil {
		return Emenda{}, err
	}
	if e.Liquidado, err = ParseBRMoney(get("liquidado")); err != nil {
		return Emenda{}, err
	}
	if e.Pago, err = ParseBRMoney(get("pago")); err != nil {
		return Emenda{}, err
	}
	return e, nil
}
