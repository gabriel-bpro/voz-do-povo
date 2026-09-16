// Package senado is the source adapter for the Senado Federal open data
// (Legislative API at legis.senado.leg.br/dadosabertos + Administrative API
// at adm.senado.gov.br/adm-dadosabertos). It fetches and parses only —
// landing and promotion live in internal/sync, so tests at the sync seam
// never touch the network.
//
// Note: the Senado publishes no CPF for senators (verified against the
// registry and detail endpoints 2026-09-15). Cross-house identity linkage
// therefore resolves via the normalized name+UF review queue; the CEAPS
// rows themselves carry codSenador, which auto-links once the registry is
// synced.
package senado

import "encoding/json"

// Senator is the normalized registry entry from
// /dadosabertos/senador/lista/atual.json (senators in exercise).
type Senator struct {
	Code       int64  // CodigoParlamentar — the Senado source id
	Name       string // NomeParlamentar
	CivilName  string // NomeCompletoParlamentar
	PartySigla string // SiglaPartidoParlamentar
	UF         string // UfParlamentar
}

// Expense is one normalized CEAPS register from the admin API's
// /api/v1/senadores/despesas_ceaps/{year} endpoint (updated daily). Unlike
// CEAP, CEAPS rows ARE uniquely keyed (id) and carry the senator's code —
// the identity-link key. CEAPS publishes a single amount (the reimbursed
// value); there is no document/glosa split.
type Expense struct {
	ID          int64 // CEAPS register id (unique per row)
	SenatorCode int64 // codSenador — matches Senator.Code
	SenatorName string

	Category       string // tipoDespesa
	SupplierName   string // fornecedor
	SupplierTaxID  string // cpfCnpj of the supplier, digits only
	DocumentNumber string // documento
	IssuedAt       string // data, verbatim (YYYY-MM-DD); sync rejects unparsable values
	Year           int
	Month          int
	Detail         string  // detalhamento
	AmountNet      float64 // valorReembolsado

	// Raw is the verbatim source record, for the raw layer. Excluded from
	// serialization (and therefore from the row-hash dedup key).
	Raw json.RawMessage `json:"-"`
}
