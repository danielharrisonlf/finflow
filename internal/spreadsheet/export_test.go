package spreadsheet

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"finflow/internal/domain"
)

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func sampleYear() YearInput {
	confirmed := domain.TxConfirmed
	return YearInput{
		Year:                2026,
		OpeningBalanceCents: 150000,
		Bands:               DefaultBands(),
		Transactions: []Transaction{
			{Date: "2026-01-05", Description: "Salário", Type: domain.TxIncome, Status: confirmed, AmountCents: 500000, AccountName: "Nubank", AccountType: domain.AccountChecking, CategoryName: "Salário"},
			{Date: "2026-01-01", Description: "Bônus", Type: domain.TxIncome, Status: confirmed, AmountCents: 10000, AccountName: "Nubank"},
			{Date: "2026-01-10", Description: "Aluguel", Type: domain.TxExpense, Status: confirmed, AmountCents: -180000, CategoryName: "Aluguel", SpendType: SpendFixed, AccountName: "Nubank"},
			{Date: "2026-01-10", Description: "Netflix", Type: domain.TxExpense, Status: confirmed, AmountCents: -5590, CategoryName: "Streaming", SpendType: SpendCuttable, AccountName: "Nubank"},
			{Date: "2026-01-12", Description: "Padaria", Type: domain.TxExpense, Status: confirmed, AmountCents: -2350, CategoryName: "Alimentação", AccountName: "Nubank"},
			{Date: "2026-01-12", Description: "Pendente", Type: domain.TxExpense, Status: domain.TxPending, AmountCents: -9999, AccountName: "Nubank"},
			// Compra no cartão em janeiro com vencimento em fevereiro: entra no dia 15/02.
			{Date: "2026-01-20", CashDate: "2026-02-15", Description: "Tênis (1/2)", Type: domain.TxCardExpense, Status: confirmed, AmountCents: -30000, CategoryName: "Compras", AccountName: "Visa", InstallmentNumber: 1, InstallmentTotal: 2},
			// Pagamento da fatura não entra de novo.
			{Date: "2026-02-15", Description: "Pagamento fatura", Type: domain.TxInvoicePayment, Status: confirmed, AmountCents: -30000, AccountName: "Nubank"},
			// Transferência para poupança: Saída + Economia (só a perna que entra).
			{Date: "2026-02-20", Description: "Para reserva", Type: domain.TxTransfer, Status: confirmed, AmountCents: -50000, AccountType: domain.AccountChecking, CounterpartAccountType: domain.AccountSavings},
			{Date: "2026-02-20", Description: "Da corrente", Type: domain.TxTransfer, Status: confirmed, AmountCents: 50000, AccountType: domain.AccountSavings, CounterpartAccountType: domain.AccountChecking},
			// Transferência entre contas comuns fica de fora.
			{Date: "2026-02-21", Description: "Entre contas", Type: domain.TxTransfer, Status: confirmed, AmountCents: 1000, AccountType: domain.AccountWallet, CounterpartAccountType: domain.AccountChecking},
			// Compra de dezembro anterior com vencimento em janeiro entra em janeiro.
			{Date: "2025-12-20", CashDate: "2026-01-08", Description: "Presente", Type: domain.TxCardExpense, Status: confirmed, AmountCents: -12000, SpendType: SpendVariable},
			{Date: "2026-12-30", CashDate: "2027-01-08", Description: "Fora do ano", Type: domain.TxCardExpense, Status: confirmed, AmountCents: -7000},
		},
		GoalContributionsByMonth: [12]int64{2: 20000},
	}
}

func openBook(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("excelize não abriu o arquivo: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func cellValue(t *testing.T, f *excelize.File, sheet, ref string) string {
	t.Helper()
	v, err := f.GetCellValue(sheet, ref, excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func formula(t *testing.T, f *excelize.File, sheet, ref string) string {
	t.Helper()
	v, err := f.GetCellFormula(sheet, ref)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestClassify(t *testing.T) {
	cases := []struct {
		tx    Transaction
		col   DailyColumn
		cents int64
		saved int64
	}{
		{Transaction{Type: domain.TxIncome, Status: domain.TxConfirmed, AmountCents: 100}, DailyIncome, 100, 0},
		{Transaction{Type: domain.TxIncome, Status: domain.TxPending, AmountCents: 100}, DailyNone, 0, 0},
		{Transaction{Type: domain.TxExpense, Status: domain.TxConfirmed, AmountCents: -100, SpendType: SpendFixed}, DailyFixed, 100, 0},
		{Transaction{Type: domain.TxExpense, Status: domain.TxConfirmed, AmountCents: -100, SpendType: SpendCuttable}, DailyFixed, 100, 0},
		{Transaction{Type: domain.TxExpense, Status: domain.TxConfirmed, AmountCents: -100}, DailyVariable, 100, 0},
		{Transaction{Type: domain.TxCardExpense, Status: domain.TxPending, AmountCents: -100}, DailyVariable, 100, 0},
		{Transaction{Type: domain.TxCardExpense, Status: domain.TxConfirmed, AmountCents: 40}, DailyVariable, -40, 0},
		{Transaction{Type: domain.TxInvoicePayment, Status: domain.TxConfirmed, AmountCents: -100}, DailyNone, 0, 0},
		{Transaction{Type: domain.TxTransfer, Status: domain.TxConfirmed, AmountCents: 100, AccountType: domain.AccountInvestment, CounterpartAccountType: domain.AccountChecking}, DailyFixed, 100, 100},
		{Transaction{Type: domain.TxTransfer, Status: domain.TxConfirmed, AmountCents: 100, AccountType: domain.AccountInvestment, CounterpartAccountType: domain.AccountSavings}, DailyNone, 0, 0},
		{Transaction{Type: domain.TxTransfer, Status: domain.TxConfirmed, AmountCents: -100, AccountType: domain.AccountChecking, CounterpartAccountType: domain.AccountSavings}, DailyNone, 0, 0},
	}
	for i, tc := range cases {
		col, cents, saved := Classify(tc.tx)
		if col != tc.col || cents != tc.cents || saved != tc.saved {
			t.Errorf("case %d: got (%d, %d, %d), want (%d, %d, %d)", i, col, cents, saved, tc.col, tc.cents, tc.saved)
		}
	}
}

func TestBuildYearWorkbookStructure(t *testing.T) {
	data, err := BuildYearWorkbook(sampleYear())
	if err != nil {
		t.Fatal(err)
	}
	f := openBook(t, data)
	if got := f.GetSheetList(); strings.Join(got, ",") != "2026,Economia,Lançamentos,Categorias" {
		t.Fatalf("sheets = %v", got)
	}
	sh := "2026"
	// Títulos, cabeçalhos e blocos com passo de 6 colunas.
	if cellValue(t, f, sh, "A1") != "JANEIRO" || cellValue(t, f, sh, "G1") != "FEVEREIRO" || cellValue(t, f, sh, "BO1") != "DEZEMBRO" {
		t.Error("títulos dos meses fora do lugar")
	}
	if cellValue(t, f, sh, "BO2") != "Data" || cellValue(t, f, sh, "BS2") != "Saldo" {
		t.Error("cabeçalho de dezembro fora do lugar")
	}
	// 1º/jan = saldo de abertura (1.500) + bônus (100), anotado.
	if v := cellValue(t, f, sh, "B3"); v != "1600" {
		t.Errorf("B3 = %q", v)
	}
	comments, err := f.GetComments(sh)
	if err != nil || len(comments) != 1 || comments[0].Cell != "B3" || !strings.Contains(comments[0].Text, "saldo de abertura") {
		t.Errorf("comentário do saldo de abertura: %+v %v", comments, err)
	}
	// Salário em 05/jan (linha 7), aluguel+netflix em 10/jan (C12), padaria em 12/jan (D14), presente 08/jan (D10).
	checks := map[string]string{"B7": "5000", "C12": "1855.9", "D14": "23.5", "D10": "120", "J17": "300", "I22": "500"}
	for ref, want := range checks {
		if got := cellValue(t, f, sh, ref); got != want {
			t.Errorf("%s = %q, want %q", ref, got, want)
		}
	}
	// Pendente não entra; pagamento de fatura não entra; transferência comum não entra.
	if v := cellValue(t, f, sh, "I17"); v != "" {
		t.Errorf("pagamento de fatura entrou em I17: %q", v)
	}
	// Fórmulas de saldo encadeadas.
	fx := map[string]string{
		"E3":  "B3-(C3+D3)",
		"E4":  "E3+B4-(C4+D4)",
		"K3":  "E33+H3-(I3+J3)",
		"BS3": "BM33+BP3-(BQ3+BR3)",
		"B38": "SUM(B3:B33)",
		"E38": "E33",
		"D41": "C38+D38",
		"D44": "B38-D41",
		"J44": "H38-J41",
	}
	for ref, want := range fx {
		if got := formula(t, f, sh, ref); got != want {
			t.Errorf("fórmula %s = %q, want %q", ref, got, want)
		}
	}
	// Dias inexistentes (30 e 31/fev, 31/abr) repetem o saldo anterior e ficam sem entrada.
	if got := formula(t, f, sh, "K32"); got != "K31" {
		t.Errorf("K32 = %q", got)
	}
	if got := formula(t, f, sh, "W33"); got != "W32" {
		t.Errorf("W33 (31/abr) = %q", got)
	}
	dvs, err := f.GetDataValidations(sh)
	if err != nil || len(dvs) != 1 || !strings.Contains(dvs[0].Sqref, "H32:J32") || dvs[0].Type != "custom" {
		t.Errorf("validação dos dias inexistentes: %+v %v", dvs, err)
	}
	styleID, _ := f.GetCellStyle(sh, "H32")
	style, _ := f.GetStyle(styleID)
	if style == nil || len(style.Fill.Color) == 0 || style.Fill.Color[0] != "D9D9D9" {
		t.Errorf("dia inexistente não ficou cinza: %+v", style)
	}
	// Cores condicionais por faixa no saldo.
	cfs, err := f.GetConditionalFormats(sh)
	if err != nil {
		t.Fatal(err)
	}
	rules := cfs["E3:E33"]
	if len(rules) != 5 || rules[0].Criteria != "greater than or equal to" || rules[0].Value != "2000.00" || rules[3].Value != "-500.00" || rules[4].Criteria != "less than" {
		t.Errorf("faixas de saldo: %+v", rules)
	}
	panes, err := f.GetPanes(sh)
	if err != nil || !panes.Freeze || panes.YSplit != 2 {
		t.Errorf("painéis congelados: %+v %v", panes, err)
	}

	// Economia ligada à aba do ano, com IFERROR.
	if got := formula(t, f, "Economia", "C5"); got != "'2026'!B38" {
		t.Errorf("Economia C5 = %q", got)
	}
	if got := formula(t, f, "Economia", "C16"); got != "'2026'!BP38" {
		t.Errorf("Economia C16 = %q", got)
	}
	if got := formula(t, f, "Economia", "E5"); !strings.HasPrefix(got, "IFERROR(") {
		t.Errorf("Economia E5 = %q", got)
	}
	if v := cellValue(t, f, "Economia", "D6"); v != "500" {
		t.Errorf("Economia fev = %q", v)
	}
	if v := cellValue(t, f, "Economia", "D7"); v != "200" {
		t.Errorf("Economia mar (aporte em meta) = %q", v)
	}
	if v := cellValue(t, f, "Economia", "B18"); v != "TOTAL" {
		t.Errorf("Economia B18 = %q", v)
	}

	// Lançamentos: só datas do ano, com filtro.
	rows, err := f.GetRows("Lançamentos")
	if err != nil {
		t.Fatal(err)
	}
	if rows[0][0] != "Data" || rows[0][4] != "Tipo de gasto" || len(rows) != 13 {
		t.Errorf("Lançamentos: %d linhas, cabeçalho %v", len(rows), rows[0])
	}
	// Categorias: matriz com Aluguel em janeiro.
	cat, err := f.GetRows("Categorias", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range cat {
		if len(r) > 1 && r[0] == "Aluguel" && r[1] == "1800" {
			found = true
		}
	}
	if !found {
		t.Errorf("Categorias sem Aluguel/jan: %v", cat)
	}
}

func TestYearWorkbookRoundTripAsDailySheet(t *testing.T) {
	in := sampleYear()
	data, err := BuildYearWorkbook(in)
	if err != nil {
		t.Fatal(err)
	}
	_, totals := aggregateDaily(in)
	res := mustParse(t, data, Options{Filename: "finflow-2026.xlsx"})
	if res.Format != FormatDailySheet {
		t.Fatalf("format = %s", res.Format)
	}
	var income, fixed, variable [12]int64
	for _, r := range res.Rows {
		if r.Error != nil {
			t.Fatalf("erro: %s", *r.Error)
		}
		m := int(r.Date[5]-'0')*10 + int(r.Date[6]-'0')
		switch {
		case r.Kind == domain.CategoryIncome:
			income[m-1] += r.AmountCents
		case r.SpendTypeHint == SpendFixed:
			fixed[m-1] += r.AmountCents
			if r.CategoryName == nil || *r.CategoryName != DailyFixedCategoryHint {
				t.Errorf("saída sem sugestão de categoria: %+v", r)
			}
		default:
			variable[m-1] += r.AmountCents
		}
	}
	totals.Income[0] += in.OpeningBalanceCents
	if income != totals.Income || fixed != totals.Fixed || variable != totals.Variable {
		t.Fatalf("totais divergentes:\nincome %v vs %v\nfixed %v vs %v\nvariable %v vs %v",
			income, totals.Income, fixed, totals.Fixed, variable, totals.Variable)
	}
	for _, w := range res.Warnings {
		t.Errorf("aviso inesperado: %s", w)
	}

	// As fórmulas vivas fecham com os totais: saldo de dezembro = abertura + entradas − saídas − diário.
	f := openBook(t, data)
	var want int64
	for m := 0; m < 12; m++ {
		want += totals.Income[m] - totals.Fixed[m] - totals.Variable[m]
	}
	got, err := f.CalcCellValue("2026", "BS38", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	if cents, ok := parseCellAmount(got); !ok || cents != want {
		t.Errorf("saldo de dezembro = %q, want %d centavos", got, want)
	}
	if perf, _ := f.CalcCellValue("2026", "D44", excelize.Options{RawCellValue: true}); perf != "4600.6" {
		t.Errorf("performance de janeiro = %q", perf)
	}
	if pct, err := f.CalcCellValue("Economia", "E4", excelize.Options{RawCellValue: true}); err != nil || pct != "%" {
		t.Errorf("Economia E4 = %q %v", pct, err)
	}
	if pct, _ := f.CalcCellValue("Economia", "E6", excelize.Options{RawCellValue: true}); pct != "" {
		t.Errorf("IFERROR com Entradas zero deveria ficar vazio, veio %q", pct)
	}
}

func TestDailySheetImportWarnsInvalidDayNegativeAndDivergentBalance(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "2025")
	sh := "2025"
	_ = f.SetCellValue(sh, "G1", "FEVEREIRO")
	_ = f.SetSheetRow(sh, "G2", &[]any{"Data", "Entrada", "Saída", "Diário", "Saldo"})
	for d := 1; d <= 31; d++ {
		_ = f.SetCellValue(sh, cell(7, 2+d), d)
	}
	_ = f.SetCellValue(sh, "H3", 1000)  // 01/02 entrada
	_ = f.SetCellValue(sh, "K3", 1000)  // saldo correto
	_ = f.SetCellValue(sh, "J4", -50)   // 02/02 diário negativo → vira entrada
	_ = f.SetCellValue(sh, "K4", 999)   // saldo errado (deveria ser 1050)
	_ = f.SetCellValue(sh, "I32", 10)   // 30/02 não existe
	_ = f.SetCellValue(sh, "I5", "abc") // ilegível
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	res := mustParse(t, buf.Bytes(), Options{})
	if res.Format != FormatDailySheet || len(res.Rows) != 2 {
		t.Fatalf("res = %+v", res)
	}
	if r := res.Rows[1]; r.Date != "2025-02-02" || r.Kind != domain.CategoryIncome || r.AmountCents != 5000 {
		t.Errorf("negativo deveria inverter: %+v", r)
	}
	joined := strings.Join(res.Warnings, "\n")
	for _, want := range []string{"dia 30 não existe", "diverge", "ilegível"} {
		if !strings.Contains(joined, want) {
			t.Errorf("faltou aviso %q em:\n%s", want, joined)
		}
	}
}

func TestBuildTransactionsWorkbook(t *testing.T) {
	data, err := BuildTransactionsWorkbook(sampleYear().Transactions[:3])
	if err != nil {
		t.Fatal(err)
	}
	f := openBook(t, data)
	if got := f.GetSheetList(); len(got) != 1 || got[0] != "Lançamentos" {
		t.Fatalf("sheets = %v", got)
	}
	// Reimporta como tabela: valores com sinal e tipo pelo rótulo.
	res := mustParse(t, data, Options{})
	rows := validRows(res)
	if res.Format != FormatTable || len(rows) != 3 || rows[0].Date != "2026-01-01" || rows[2].Kind != domain.CategoryExpense || rows[2].AmountCents != 180000 {
		t.Fatalf("rows = %+v", res.Rows)
	}
}

func TestBuildYearWorkbookRejectsBadYear(t *testing.T) {
	if _, err := BuildYearWorkbook(YearInput{Year: 12}); err == nil {
		t.Fatal("ano inválido deveria falhar")
	}
}
