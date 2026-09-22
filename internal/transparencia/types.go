// Package transparencia is the Portal da Transparência source adapter:
// fetch + parse only, no DB. The emendas parlamentares dataset syncs via the
// weekly bulk spreadsheet (download-de-dados), no API key.
package transparencia

import (
	"encoding/json"
	"strings"
)

// Emenda is one normalized emenda parlamentar register from the bulk
// spreadsheet. Amounts are already parsed from the pt-BR source format
// ("1.234.567,89"). Raw carries the row's verbatim fields (JSON) for the raw
// layer.
type Emenda struct {
	Code       string  `json:"code"` // código da emenda: unique id
	Year       int     `json:"year"`
	Tipo       string  `json:"tipo"`       // tipo de emenda as published
	AuthorCode string  `json:"authorCode"` // código do autor (not a cross-source id)
	AuthorName string  `json:"authorName"` // the only identity key the source gives
	Number     string  `json:"number"`
	Localidade string  `json:"localidade"`
	Funcao     string  `json:"funcao"`
	Subfuncao  string  `json:"subfuncao"`
	Empenhado  float64 `json:"empenhado"`
	Liquidado  float64 `json:"liquidado"`
	Pago       float64 `json:"pago"`

	Raw json.RawMessage `json:"-"`
}

// PIXTipoFragment identifies a transferência especial ("emenda PIX") by its
// normalized tipo de emenda. The live values are plural ("Emenda Individual -
// Transferências Especiais"); the singular appears in older dictionaries.
// "ESPECIAL" is a substring of neither, so the check matches both fragments —
// and no other tipo carries either. The PIX check itself lives in the sync
// package (which owns name normalization); kept as constants here, next to
// the type.
const (
	PIXTipoFragmentPlural   = "ESPECIAIS"
	PIXTipoFragmentSingular = "ESPECIAL"
)

// ParseBRMoney parses the source's pt-BR amounts ("1.234.567,89", "0,00",
// "1500000"), including the negative form with a floating sign ("- 17.088,41",
// seen live on restated empenhados). Empty yields 0.
func ParseBRMoney(s string) (float64, error) {
	s = strings.ReplaceAll(s, " ", "")
	if s == "" || s == "-" {
		return 0, nil
	}
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	var v float64
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return 0, &ParseError{Field: "amount", Value: s, Err: err}
	}
	return v, nil
}

// ParseError marks a row-level parse failure with the offending field.
type ParseError struct {
	Field string
	Value string
	Err   error
}

func (e *ParseError) Error() string {
	return "transparencia: parse " + e.Field + " " + e.Value + ": " + e.Err.Error()
}
func (e *ParseError) Unwrap() error { return e.Err }
