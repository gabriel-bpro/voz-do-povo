// Package camara is the source adapter for the Câmara dos Deputados open data
// (REST API at dadosabertos.camara.leg.br + CEAP bulk year files). It fetches
// and parses only — landing and promotion live in internal/sync, so tests at
// the sync seam never touch the network.
//
// Note: the REST API's /deputados/{id}/despesas endpoint returns empty for
// every deputy (verified 2026-09-08); CEAP expenses come exclusively from the
// bulk year files, which are updated daily.
package camara

// Deputy is the normalized registry entry: the /deputados list fields plus the
// per-deputy detail fields (CPF, civil name) when fetched. CPF is normalized
// to digits only.
type Deputy struct {
	ID         int64  // Câmara's own id (from the deputados URI; == CEAP ideCadastro)
	Name       string // parliamentary name (nome)
	CivilName  string // nomeCivil, from the detail endpoint; empty if not fetched
	CPF        string // digits only, from the detail endpoint; empty if not fetched
	PartySigla string // siglaPartido
	UF         string // siglaUf
}

// Expense is one normalized CEAP register from the bulk Ano-{year} file.
// The file deviates from the API's conventions: the politician is identified
// by ideCadastro (the API id) and cpf, and registers are NOT uniquely keyed —
// ideDocumento groups installments of one document, so dedup uses a hash of
// the whole row (see sync.expenseRowKey).
type Expense struct {
	// Deputy identification.
	DeputyID   int64  // ideCadastro (0 when absent)
	DeputyCPF  string // digits only
	DeputyName string // txNomeParlamentar
	DeputyUF   string // sgUF

	PartySigla     string
	Category       string // txtDescricao
	SupplierName   string
	SupplierTaxID  string // CNPJ/CPF, digits only
	DocumentNumber string
	IssuedAt       string // ISO date (YYYY-MM-DD); empty when unparsable
	Year           int
	Month          int
	Installment    int    // numParcela: != 0 when reimbursed in installments
	AmountDocument float64
	AmountGlosa    float64
	AmountNet      float64 // vlrLiquido; may be negative (glosa/ressarcimento)
	DocumentURL    string
	DocumentID     string // ideDocumento (0 → ""); grouping key, NOT unique
}
