package spreadsheet

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"

	"finflow/internal/domain"
)

func mustParse(t *testing.T, data []byte, opts Options) *Result {
	t.Helper()
	res, err := Parse(data, opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return res
}

func validRows(res *Result) []Row {
	var out []Row
	for _, r := range res.Rows {
		if r.Error == nil {
			out = append(out, r)
		}
	}
	return out
}

func TestParseCSVSemicolonBOMBrazilian(t *testing.T) {
	data := string([]byte{0xEF, 0xBB, 0xBF}) + "Data;Descrição;Valor (R$);Categoria\r\n" +
		"05/03/2026;Padaria;-12,50;Alimentação\r\n" +
		"06/03/2026;Salário;R$ 5.000,00;\r\n" +
		"07/03/2026;\"Mercado; filial 2\";(1.234,56);Supermercado\r\n"
	res := mustParse(t, []byte(data), Options{Filename: "extrato.csv"})
	if res.Format != FormatCSV {
		t.Fatalf("format = %s", res.Format)
	}
	if len(res.Columns) != 4 || res.Columns[1] != "Descrição" {
		t.Fatalf("columns = %v", res.Columns)
	}
	if res.Mapping.Date == nil || *res.Mapping.Date != 0 || res.Mapping.Amount == nil || *res.Mapping.Amount != 2 ||
		res.Mapping.Category == nil || *res.Mapping.Category != 3 {
		t.Fatalf("mapping = %+v", res.Mapping)
	}
	rows := validRows(res)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", res.Rows)
	}
	want := []struct {
		date string
		amt  int64
		kind domain.CategoryKind
		desc string
	}{
		{"2026-03-05", 1250, domain.CategoryExpense, "Padaria"},
		{"2026-03-06", 500000, domain.CategoryIncome, "Salário"},
		{"2026-03-07", 123456, domain.CategoryExpense, "Mercado; filial 2"},
	}
	for i, w := range want {
		r := rows[i]
		if r.Date != w.date || r.AmountCents != w.amt || r.Kind != w.kind || r.Description != w.desc {
			t.Errorf("row %d = %+v, want %+v", i, r, w)
		}
	}
	if rows[0].CategoryName == nil || *rows[0].CategoryName != "Alimentação" || rows[1].CategoryName != nil {
		t.Errorf("categories: %v %v", rows[0].CategoryName, rows[1].CategoryName)
	}
}

func TestParseCSVCommaEnglishAndTypeColumn(t *testing.T) {
	data := "Date,Description,Amount,Type\n2026-01-10,Coffee,\"1,234.50\",debit\n2026-01-11,Refund,20.00,credit\n"
	res := mustParse(t, []byte(data), Options{})
	rows := validRows(res)
	if len(rows) != 2 || rows[0].AmountCents != 123450 || rows[0].Kind != domain.CategoryExpense ||
		rows[1].Kind != domain.CategoryIncome || rows[1].AmountCents != 2000 {
		t.Fatalf("rows = %+v", res.Rows)
	}
}

func TestParseCSVTabWindows1252DebitCredit(t *testing.T) {
	src := "Data\tHistórico\tDébito\tCrédito\n01/02/2026\tTarifa pacote\t35,90\t\n02/02/2026\tPIX recebido\t\t1.000,00\n"
	enc, err := charmap.Windows1252.NewEncoder().String(src)
	if err != nil {
		t.Fatal(err)
	}
	res := mustParse(t, []byte(enc), Options{Filename: "banco.txt"})
	if res.Columns[1] != "Histórico" {
		t.Fatalf("decodificação Windows-1252 falhou: %v", res.Columns)
	}
	rows := validRows(res)
	if len(rows) != 2 || rows[0].Kind != domain.CategoryExpense || rows[0].AmountCents != 3590 ||
		rows[1].Kind != domain.CategoryIncome || rows[1].AmountCents != 100000 {
		t.Fatalf("rows = %+v", res.Rows)
	}
}

func TestParseCSVDCSuffixSepHintAndShortYear(t *testing.T) {
	data := "sep=;\nlançamento;data;valor\nCompra cartão;03/04/26;150,00 D\nEstorno;04/04/26;150,00 C\n"
	res := mustParse(t, []byte(data), Options{})
	rows := validRows(res)
	if len(rows) != 2 || rows[0].Date != "2026-04-03" || rows[0].Kind != domain.CategoryExpense ||
		rows[1].Kind != domain.CategoryIncome || rows[0].Description != "Compra cartão" {
		t.Fatalf("rows = %+v", res.Rows)
	}
}

func TestParseCSVErrorsDuplicatesAndBlankLines(t *testing.T) {
	data := "Data;Descrição;Valor\n" +
		"10/05/2026;Uber;-23,40\n" +
		"\n" +
		"10/05/2026;UBER ;-23,40\n" +
		"32/05/2026;Data ruim;-1,00\n" +
		"11/05/2026;Sem valor;\n" +
		"12/05/2026;Zero;0,00\n" +
		"Total;;-47,80\n"
	res := mustParse(t, []byte(data), Options{})
	if len(res.Rows) != 6 {
		t.Fatalf("esperava 6 linhas (blank ignorada), veio %d: %+v", len(res.Rows), res.Rows)
	}
	if res.Rows[0].Duplicate || !res.Rows[1].Duplicate {
		t.Errorf("duplicado no arquivo não marcado: %+v", res.Rows[:2])
	}
	for i := 2; i < 6; i++ {
		if res.Rows[i].Error == nil {
			t.Errorf("linha %d deveria ter erro: %+v", i, res.Rows[i])
		}
	}
	if !strings.Contains(*res.Rows[2].Error, "data inválida") || !strings.Contains(*res.Rows[3].Error, "valor vazio") ||
		!strings.Contains(*res.Rows[4].Error, "valor zerado") {
		t.Errorf("mensagens: %q / %q / %q", *res.Rows[2].Error, *res.Rows[3].Error, *res.Rows[4].Error)
	}
	for i, r := range res.Rows {
		if r.Index != i {
			t.Errorf("index %d = %d", i, r.Index)
		}
	}
}

func TestParseCSVWithoutHeaderNeedsMapping(t *testing.T) {
	data := "Mercado X;15/06/2026;99,90\nFarmácia;16/06/2026;-10,00\n"
	res := mustParse(t, []byte(data), Options{})
	if len(res.Rows) != 0 || len(res.Warnings) == 0 || res.Columns[0] != "Coluna 1" {
		t.Fatalf("sem cabeçalho deveria pedir mapeamento: %+v", res)
	}
	d, desc, amt, bad := 1, 0, 2, 9
	res = mustParse(t, []byte(data), Options{Mapping: &Mapping{Date: &d, Description: &desc, Amount: &amt, Category: &bad}})
	rows := validRows(res)
	if len(rows) != 2 || rows[0].Description != "Mercado X" || rows[0].Kind != domain.CategoryIncome ||
		rows[1].Kind != domain.CategoryExpense || rows[1].AmountCents != 1000 {
		t.Fatalf("rows = %+v", res.Rows)
	}
	if res.Mapping.Category != nil || len(res.Warnings) == 0 {
		t.Errorf("mapeamento fora do intervalo deveria ser ignorado com aviso: %+v %v", res.Mapping, res.Warnings)
	}
}

func TestParseOFX(t *testing.T) {
	ofx := `OFXHEADER:100
DATA:OFXSGML
VERSION:102
CHARSET:1252

<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS><BANKTRANLIST>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20260310120000[-3:BRT]
<TRNAMT>-45.90
<FITID>1
<NAME>PADARIA
<MEMO>Compra Padaria &amp; Cia
</STMTTRN>
<STMTTRN>
<TRNTYPE>CREDIT
<DTPOSTED>20260311
<TRNAMT>1500,00
<FITID>2
<NAME>PIX RECEBIDO
</STMTTRN>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>xx
<TRNAMT>-1.00
</STMTTRN>
</BANKTRANLIST></STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>`
	res := mustParse(t, []byte(ofx), Options{Filename: "extrato.ofx"})
	if res.Format != FormatOFX || len(res.Rows) != 3 {
		t.Fatalf("res = %+v", res)
	}
	r0, r1 := res.Rows[0], res.Rows[1]
	if r0.Date != "2026-03-10" || r0.AmountCents != 4590 || r0.Kind != domain.CategoryExpense || r0.Description != "Compra Padaria & Cia" {
		t.Errorf("r0 = %+v", r0)
	}
	if r1.Date != "2026-03-11" || r1.AmountCents != 150000 || r1.Kind != domain.CategoryIncome || r1.Description != "PIX RECEBIDO" {
		t.Errorf("r1 = %+v", r1)
	}
	if res.Rows[2].Error == nil {
		t.Errorf("DTPOSTED inválido deveria gerar erro")
	}
}

func TestParseOFXRoundTripWithDomainExport(t *testing.T) {
	acc := "Conta"
	ofx := domain.BuildOFX([]domain.Transaction{
		{ID: "a", Date: "2026-02-01", Description: "Salário", AmountCents: 300000, AccountName: &acc},
		{ID: "b", Date: "2026-02-03", Description: "Aluguel", AmountCents: -150000, AccountName: &acc},
	})
	res := mustParse(t, []byte(ofx), Options{})
	rows := validRows(res)
	if len(rows) != 2 || rows[0].AmountCents != 300000 || rows[1].Kind != domain.CategoryExpense {
		t.Fatalf("rows = %+v", res.Rows)
	}
}

func TestParseCSVRoundTripWithDomainExport(t *testing.T) {
	cat := "Mercado"
	csv := domain.BuildCSV([]domain.Transaction{
		{Date: "2026-02-01", Description: "Compra; grande", AmountCents: -123456, Type: domain.TxExpense, CategoryName: &cat, Status: domain.TxConfirmed},
		{Date: "2026-02-02", Description: "Freela", AmountCents: 50000, Type: domain.TxIncome, Status: domain.TxConfirmed},
	})
	res := mustParse(t, []byte(csv), Options{})
	rows := validRows(res)
	if len(rows) != 2 || rows[0].AmountCents != 123456 || rows[0].Kind != domain.CategoryExpense ||
		rows[0].Description != "Compra; grande" || *rows[0].CategoryName != "Mercado" || rows[1].Kind != domain.CategoryIncome {
		t.Fatalf("rows = %+v", res.Rows)
	}
}

func TestParseXLSXTableWithSerialDatesAndOffsetHeader(t *testing.T) {
	f := excelize.NewFile()
	sh := "Sheet1"
	_ = f.SetCellValue(sh, "A1", "Extrato do banco")
	_ = f.SetSheetRow(sh, "A3", &[]any{"Data", "Lançamento", "Valor", "Conta"})
	_ = f.SetSheetRow(sh, "A4", &[]any{46086, "Mercado", -250.75, "Nubank"})
	_ = f.SetSheetRow(sh, "A5", &[]any{"06/03/2026", "Reembolso", "R$ 80,00", ""})
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	res := mustParse(t, buf.Bytes(), Options{Filename: "x.xlsx"})
	if res.Format != FormatTable || res.Sheet == nil || *res.Sheet != sh {
		t.Fatalf("res = %+v", res)
	}
	rows := validRows(res)
	if len(rows) != 2 || rows[0].Date != "2026-03-05" || rows[0].AmountCents != 25075 || rows[0].Kind != domain.CategoryExpense ||
		rows[0].AccountName == nil || *rows[0].AccountName != "Nubank" || rows[1].AmountCents != 8000 {
		t.Fatalf("rows = %+v", res.Rows)
	}
}

func TestParseRejectsXLSAndUnknownSheet(t *testing.T) {
	if _, err := Parse([]byte{0xD0, 0xCF, 0x11, 0xE0, 0, 0}, Options{}); err == nil || !strings.Contains(err.Error(), ".xls") {
		t.Fatalf("xls: %v", err)
	}
	data, err := BuildImportTemplate(mustDate("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data, Options{Sheet: "Nope"}); err == nil {
		t.Fatal("aba inexistente deveria falhar")
	}
	if _, err := Parse(nil, Options{}); err == nil {
		t.Fatal("arquivo vazio deveria falhar")
	}
	if _, err := Parse([]byte("PK\x03\x04garbage"), Options{}); err == nil {
		t.Fatal("zip inválido deveria falhar")
	}
}

func TestImportTemplateParsesAsTable(t *testing.T) {
	data, err := BuildImportTemplate(mustDate("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.GetSheetList(); len(got) != 2 || got[0] != "Lançamentos" || got[1] != "Como usar" {
		t.Fatalf("sheets = %v", got)
	}
	dvs, err := f.GetDataValidations("Lançamentos")
	if err != nil || len(dvs) != 1 || !strings.Contains(dvs[0].Formula1, "Receita") {
		t.Fatalf("validação de lista: %+v %v", dvs, err)
	}
	f.Close()

	res := mustParse(t, data, Options{})
	rows := validRows(res)
	if res.Format != FormatTable || len(rows) != 3 {
		t.Fatalf("res = %+v", res)
	}
	if rows[0].Kind != domain.CategoryIncome || rows[0].AmountCents != 500000 || rows[0].Date != "2026-03-05" ||
		rows[2].Kind != domain.CategoryExpense || rows[2].AmountCents != 35247 || *rows[2].CategoryName != "Supermercado" {
		t.Fatalf("rows = %+v", rows)
	}
}

// TestParseRealDailyTemplate lê uma cópia em branco da planilha diária, se disponível localmente.
// Defina FINFLOW_DAILY_TEMPLATE com o caminho do .xlsx (o arquivo não é versionado).
func TestParseRealDailyTemplate(t *testing.T) {
	path := os.Getenv("FINFLOW_DAILY_TEMPLATE")
	if path == "" {
		t.Skip("FINFLOW_DAILY_TEMPLATE não definido")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res := mustParse(t, data, Options{Filename: path})
	if res.Format != FormatDailySheet {
		t.Fatalf("format = %s", res.Format)
	}
	if !containsString(res.Sheets, "Economia") {
		t.Errorf("sheets = %v", res.Sheets)
	}
	if len(res.Rows) != 0 {
		t.Errorf("modelo em branco não deveria gerar linhas: %+v", res.Rows[:min(3, len(res.Rows))])
	}
	for _, w := range res.Warnings {
		if strings.Contains(w, "diverge") {
			t.Errorf("aviso inesperado: %s", w)
		}
	}
}
