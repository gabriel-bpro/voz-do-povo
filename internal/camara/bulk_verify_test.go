package camara_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"voz-do-povo/internal/camara"
)

// The CEAP bulk JSON shape, as served (verified against Ano-2026.json.zip,
// 2026-09-08): camelCase fields, string amounts, datetime suffixed dates, no
// ideCadastro.
func TestParseBulkZipRealShape(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("Ano-2026.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte(`{"dados":[
		{
			"nomeParlamentar": "MARIA EXEMPLO",
			"cpf": "123.456.789-01",
			"numeroDeputadoID": 2812,
			"siglaUF": "SP",
			"siglaPartido": "PT",
			"descricao": "COMBUSTÍVEIS E LUBRIFICANTES",
			"fornecedor": "POSTO EXEMPLO LTDA",
			"cnpjCPF": "98.765.432/0001-10 ",
			"numero": "NF-300",
			"dataEmissao": "2026-07-05T00:00:00",
			"valorDocumento": "100.5",
			"valorGlosa": "0",
			"valorLiquido": "100.5",
			"mes": 7,
			"ano": 2026,
			"parcela": 0,
			"idDocumento": 8800001,
			"urlDocumento": "https://example.test/doc.pdf"
		},
		{
			"nomeParlamentar": "LID.EXEMPLO-CD",
			"cpf": "",
			"numeroDeputadoID": 9999,
			"siglaUF": "NA",
			"siglaPartido": "",
			"descricao": "PASSAGEM AÉREA - RPA",
			"fornecedor": "CIA AEREA EXEMPLO",
			"cnpjCPF": "",
			"numero": "",
			"dataEmissao": "",
			"valorDocumento": "1234.56",
			"valorGlosa": "10.00",
			"valorLiquido": "1224.56",
			"mes": 7,
			"ano": 2026,
			"parcela": 1,
			"idDocumento": 8800002,
			"urlDocumento": ""
		}
	]}`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var got []camara.Expense
	if err := camara.ParseBulkZip(buf.Bytes(), func(e camara.Expense) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatalf("ParseBulkZip: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("parsed %d rows, want 2", len(got))
	}

	first := got[0]
	if first.DeputyCPF != "12345678901" {
		t.Errorf("DeputyCPF = %q, want digits only", first.DeputyCPF)
	}
	if first.DeputyUF != "SP" || first.PartySigla != "PT" {
		t.Errorf("UF/partido = %q/%q", first.DeputyUF, first.PartySigla)
	}
	if first.SupplierTaxID != "98765432000110" {
		t.Errorf("SupplierTaxID = %q, want digits only (trailing space stripped)", first.SupplierTaxID)
	}
	if first.IssuedAt != "2026-07-05" {
		t.Errorf("IssuedAt = %q, want date only", first.IssuedAt)
	}
	if first.AmountNet != 100.5 || first.Year != 2026 || first.Month != 7 {
		t.Errorf("amounts/period = %v %d-%d", first.AmountNet, first.Year, first.Month)
	}
	if first.DocumentID != "8800001" {
		t.Errorf("DocumentID = %q", first.DocumentID)
	}
	if len(first.Raw) == 0 || !bytes.Contains(first.Raw, []byte("nomeParlamentar")) {
		t.Errorf("Raw = %q, want verbatim source record", first.Raw)
	}

	second := got[1]
	if second.AmountDocument != 1234.56 || second.AmountGlosa != 10.00 || second.AmountNet != 1224.56 {
		t.Errorf("string amounts parsed as %v/%v/%v", second.AmountDocument, second.AmountGlosa, second.AmountNet)
	}
	if second.Installment != 1 {
		t.Errorf("Installment = %d, want 1", second.Installment)
	}
}
