package spreadsheet

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"finflow/internal/domain"
)

// Transaction é um lançamento já resolvido pelo serviço (nomes, tipos de conta, vencimento da fatura).
type Transaction struct {
	// Date é a data do lançamento (data da compra, no cartão).
	Date string
	// CashDate é quando o dinheiro sai ou entra: vencimento da fatura para compras no cartão;
	// vazio equivale a Date.
	CashDate    string
	Description string
	Type        domain.TransactionType
	Status      domain.TransactionStatus
	// AmountCents com sinal, como gravado (despesas negativas).
	AmountCents  int64
	CategoryName string
	// SpendType vazio é tratado como VARIABLE em despesas.
	SpendType SpendType
	// AccountName é o nome da conta ou do cartão.
	AccountName string
	// AccountType é o tipo da conta da perna (vazio para cartão).
	AccountType domain.AccountType
	// CounterpartAccountType é o tipo da conta da outra perna (só transferências).
	CounterpartAccountType domain.AccountType
	InstallmentNumber      int
	InstallmentTotal       int
}

func (t Transaction) cashDate() string {
	if t.CashDate != "" {
		return t.CashDate
	}
	return t.Date
}

// YearInput alimenta a pasta de trabalho anual.
type YearInput struct {
	Year int
	// OpeningBalanceCents é o saldo total no fim do ano anterior (entra como Entrada anotada em 1º/jan).
	OpeningBalanceCents int64
	// Transactions deve conter tudo com Date ou CashDate no ano.
	Transactions []Transaction
	// GoalContributionsByMonth soma os aportes em metas de cada mês (índice 0 = janeiro).
	GoalContributionsByMonth [12]int64
	Bands                    Bands
}

// DailyColumn indica em qual coluna da planilha diária um valor cai.
type DailyColumn int

const (
	DailyNone DailyColumn = iota
	DailyIncome
	DailyFixed
	DailyVariable
)

func isSavingsType(t domain.AccountType) bool {
	return t == domain.AccountSavings || t == domain.AccountInvestment
}

// Classify devolve a coluna da planilha diária e o valor positivo (centavos) que o lançamento soma nela,
// além de quanto conta como Economia. Regras (contrato v2 §4.1 e §3): receitas confirmadas → Entrada;
// despesas confirmadas e compras no cartão → Saída (FIXED/CUTTABLE) ou Diário (VARIABLE/sem categoria);
// pagamento de fatura não entra de novo; transferência que entra em conta SAVINGS/INVESTMENT vinda de
// outra conta comum → Saída e Economia; resgate (entra em conta comum vindo de reserva) → Entrada e
// Economia negativa; receitas e despesas dentro das contas de reserva (rendimentos, taxas) e as demais
// transferências ficam de fora — a planilha acompanha só o dinheiro que circula nas contas líquidas.
func Classify(t Transaction) (col DailyColumn, cents int64, savedCents int64) {
	switch t.Type {
	case domain.TxIncome:
		if t.Status != domain.TxConfirmed || isSavingsType(t.AccountType) {
			return DailyNone, 0, 0
		}
		return DailyIncome, t.AmountCents, 0
	case domain.TxExpense, domain.TxCardExpense:
		if t.Type == domain.TxExpense && (t.Status != domain.TxConfirmed || isSavingsType(t.AccountType)) {
			return DailyNone, 0, 0
		}
		// Despesas são negativas; um estorno (positivo) reduz a coluna.
		if t.SpendType == SpendFixed || t.SpendType == SpendCuttable {
			return DailyFixed, -t.AmountCents, 0
		}
		return DailyVariable, -t.AmountCents, 0
	case domain.TxTransfer:
		if t.Status != domain.TxConfirmed || t.AmountCents <= 0 {
			return DailyNone, 0, 0
		}
		if isSavingsType(t.AccountType) && !isSavingsType(t.CounterpartAccountType) {
			return DailyFixed, t.AmountCents, t.AmountCents
		}
		if !isSavingsType(t.AccountType) && isSavingsType(t.CounterpartAccountType) {
			return DailyIncome, t.AmountCents, -t.AmountCents
		}
	}
	return DailyNone, 0, 0
}

type dayTotals struct {
	income, fixed, variable int64
}

// YearTotals resume a aba do ano: totais por mês (índice 0 = janeiro).
type YearTotals struct {
	Income   [12]int64
	Fixed    [12]int64
	Variable [12]int64
	Saved    [12]int64
}

func aggregateDaily(in YearInput) (map[string]*dayTotals, YearTotals) {
	days := make(map[string]*dayTotals)
	var totals YearTotals
	prefix := fmt.Sprintf("%04d-", in.Year)
	for _, t := range in.Transactions {
		d := t.cashDate()
		if !strings.HasPrefix(d, prefix) || len(d) != 10 {
			continue
		}
		col, cents, saved := Classify(t)
		if col == DailyNone {
			continue
		}
		month, err := strconv.Atoi(d[5:7])
		if err != nil || month < 1 || month > 12 {
			continue
		}
		dt := days[d]
		if dt == nil {
			dt = &dayTotals{}
			days[d] = dt
		}
		switch col {
		case DailyIncome:
			dt.income += cents
			totals.Income[month-1] += cents
		case DailyFixed:
			dt.fixed += cents
			totals.Fixed[month-1] += cents
		case DailyVariable:
			dt.variable += cents
			totals.Variable[month-1] += cents
		}
		totals.Saved[month-1] += saved
	}
	for i, v := range in.GoalContributionsByMonth {
		totals.Saved[i] += v
	}
	return days, totals
}

// styles reúne os estilos compartilhados de uma pasta de trabalho.
type styles struct {
	title, header, headerRight, currency, currencyBold, label, labelRight, invalid, invalidText,
	date, percent, percentBold, text, bold, note int
	bands [5]int
}

func newStyles(f *excelize.File) (*styles, error) {
	fmtCurrency := currencyFormat
	fmtPercent := percentFormat
	fmtDate := dateFormat
	border := []excelize.Border{
		{Type: "bottom", Color: "D0D5DD", Style: 1},
	}
	type styleDef struct {
		dst *int
		s   *excelize.Style
	}
	var defs []styleDef
	st := &styles{}
	add := func(dst *int, s *excelize.Style) { defs = append(defs, styleDef{dst, s}) }
	add(&st.title, &excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1F3A5F"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	add(&st.header, &excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E8EDF3"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
		Border:    border,
	})
	add(&st.headerRight, &excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E8EDF3"}},
		Alignment: &excelize.Alignment{Horizontal: "right"},
		Border:    border,
	})
	add(&st.currency, &excelize.Style{CustomNumFmt: &fmtCurrency})
	add(&st.currencyBold, &excelize.Style{CustomNumFmt: &fmtCurrency, Font: &excelize.Font{Bold: true}})
	add(&st.label, &excelize.Style{Font: &excelize.Font{Bold: true}})
	add(&st.labelRight, &excelize.Style{Font: &excelize.Font{Bold: true}, Alignment: &excelize.Alignment{Horizontal: "right"}})
	add(&st.invalid, &excelize.Style{
		CustomNumFmt: &fmtCurrency,
		Fill:         excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9D9D9"}},
		Font:         &excelize.Font{Color: "808080"},
	})
	add(&st.invalidText, &excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9D9D9"}},
		Font:      &excelize.Font{Color: "808080"},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	add(&st.date, &excelize.Style{CustomNumFmt: &fmtDate, Alignment: &excelize.Alignment{Horizontal: "center"}})
	add(&st.percent, &excelize.Style{CustomNumFmt: &fmtPercent})
	add(&st.percentBold, &excelize.Style{CustomNumFmt: &fmtPercent, Font: &excelize.Font{Bold: true}})
	add(&st.text, &excelize.Style{Alignment: &excelize.Alignment{Horizontal: "center"}})
	add(&st.bold, &excelize.Style{Font: &excelize.Font{Bold: true, Size: 13}})
	add(&st.note, &excelize.Style{Font: &excelize.Font{Italic: true, Color: "667085"}, Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	for _, d := range defs {
		id, err := f.NewStyle(d.s)
		if err != nil {
			return nil, err
		}
		*d.dst = id
	}
	for i, color := range []string{colorDarkGreen, colorLightGreen, colorYellow, colorLightRed, colorRed} {
		id, err := f.NewConditionalStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{color}},
		})
		if err != nil {
			return nil, err
		}
		st.bands[i] = id
	}
	return st, nil
}

// bandRules monta as regras de cor do saldo, da faixa mais alta para a mais baixa (prioridade em ordem).
func (st *styles) bandRules(b Bands) []excelize.ConditionalFormatOptions {
	reais := func(c int64) string { return strconv.FormatFloat(centsToFloat(c), 'f', 2, 64) }
	rule := func(i int, criteria string, value int64) excelize.ConditionalFormatOptions {
		id := st.bands[i]
		return excelize.ConditionalFormatOptions{Type: "cell", Criteria: criteria, Value: reais(value), Format: &id, StopIfTrue: true}
	}
	return []excelize.ConditionalFormatOptions{
		rule(0, ">=", b.DarkGreen),
		rule(1, ">=", b.LightGreen),
		rule(2, ">=", b.Yellow),
		rule(3, ">=", b.LightRed),
		rule(4, "<", b.LightRed),
	}
}

func newFile() *excelize.File {
	return excelize.NewFile()
}

// setupSheets renomeia a aba padrão para a primeira e cria as demais na ordem pedida.
func setupSheets(f *excelize.File, names ...string) error {
	if err := f.SetSheetName(defaultSheetName, names[0]); err != nil {
		return err
	}
	for _, n := range names[1:] {
		if _, err := f.NewSheet(n); err != nil {
			return err
		}
	}
	f.SetActiveSheet(0)
	return nil
}

func finish(f *excelize.File) ([]byte, error) {
	defer f.Close()
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sheetRef devolve o nome da aba pronto para fórmulas ('2026'!B38).
func sheetRef(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// BuildYearWorkbook gera a pasta de trabalho anual: abas <ano>, Economia, Lançamentos e Categorias.
func BuildYearWorkbook(in YearInput) ([]byte, error) {
	if in.Year < 1900 || in.Year > 9999 {
		return nil, fmt.Errorf("ano inválido: %d", in.Year)
	}
	f := newFile()
	yearSheet := strconv.Itoa(in.Year)
	if err := setupSheets(f, yearSheet, sheetEconomia, sheetLancamentos, sheetCategorias); err != nil {
		f.Close()
		return nil, err
	}
	st, err := newStyles(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	days, totals := aggregateDaily(in)
	if err := writeDailySheet(f, st, yearSheet, in, days); err != nil {
		f.Close()
		return nil, err
	}
	if err := writeEconomiaSheet(f, st, yearSheet, in.Year, totals); err != nil {
		f.Close()
		return nil, err
	}
	var yearTx []Transaction
	prefix := yearSheet + "-"
	for _, t := range in.Transactions {
		if strings.HasPrefix(t.Date, prefix) {
			yearTx = append(yearTx, t)
		}
	}
	if err := writeLancamentosSheet(f, st, sheetLancamentos, yearTx); err != nil {
		f.Close()
		return nil, err
	}
	if err := writeCategoriasSheet(f, st, in.Year, yearTx); err != nil {
		f.Close()
		return nil, err
	}
	return finish(f)
}

func writeDailySheet(f *excelize.File, st *styles, sheet string, in YearInput, days map[string]*dayTotals) error {
	bands := in.Bands
	if bands == (Bands{}) {
		bands = DefaultBands()
	}
	invalidDays := excelize.NewDataValidation(true)
	invalidDays.Type = "custom"
	invalidDays.Formula1 = "FALSE"
	invalidDays.ShowErrorMessage = true
	invalidDays.SetError(excelize.DataValidationErrorStyleStop, "Dia inexistente", "Este mês não tem este dia.")
	var invalidRefs []string

	for m := 1; m <= 12; m++ {
		c0 := blockStartCol(m)
		cData, cIn, cOut, cDaily, cBal := colName(c0), colName(c0+1), colName(c0+2), colName(c0+3), colName(c0+4)
		if err := f.MergeCell(sheet, cell(c0, 1), cell(c0+4, 1)); err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell(c0, 1), monthNames[m-1]); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell(c0, 1), cell(c0+4, 1), st.title); err != nil {
			return err
		}
		for i, h := range []string{"Data", "Entrada", "Saída", "Diário", "Saldo"} {
			if err := f.SetCellValue(sheet, cell(c0+i, dailyHeaderRow), h); err != nil {
				return err
			}
		}
		if err := f.SetCellStyle(sheet, cell(c0, dailyHeaderRow), cell(c0+4, dailyHeaderRow), st.header); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell(c0+1, dailyFirstDayRow), cell(c0+4, dailyLastDayRow), st.currency); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell(c0, dailyFirstDayRow), cell(c0, dailyLastDayRow), st.text); err != nil {
			return err
		}
		valid := daysIn(in.Year, m)
		for day := 1; day <= 31; day++ {
			r := dailyFirstDayRow + day - 1
			if err := f.SetCellValue(sheet, cell(c0, r), day); err != nil {
				return err
			}
			// Saldo encadeado: o 1º dia de cada mês parte da linha 33 do mês anterior.
			prev := ""
			switch {
			case day > 1:
				prev = fmt.Sprintf("%s%d", cBal, r-1)
			case m > 1:
				prev = fmt.Sprintf("%s%d", colName(blockStartCol(m-1)+4), dailyLastDayRow)
			}
			if day > valid {
				// Dia inexistente: cinza, sem entrada, saldo só repete o anterior.
				formula := prev
				if prev == "" {
					formula = "0"
				}
				if err := f.SetCellFormula(sheet, cell(c0+4, r), formula); err != nil {
					return err
				}
				if err := f.SetCellStyle(sheet, cell(c0, r), cell(c0, r), st.invalidText); err != nil {
					return err
				}
				if err := f.SetCellStyle(sheet, cell(c0+1, r), cell(c0+4, r), st.invalid); err != nil {
					return err
				}
				invalidRefs = append(invalidRefs, fmt.Sprintf("%s:%s", cell(c0+1, r), cell(c0+3, r)))
				continue
			}
			date := fmt.Sprintf("%04d-%02d-%02d", in.Year, m, day)
			if dt := days[date]; dt != nil {
				for _, v := range []struct {
					col   int
					cents int64
				}{{c0 + 1, dt.income}, {c0 + 2, dt.fixed}, {c0 + 3, dt.variable}} {
					if v.cents != 0 {
						if err := f.SetCellValue(sheet, cell(v.col, r), centsToFloat(v.cents)); err != nil {
							return err
						}
					}
				}
			}
			move := fmt.Sprintf("%s%d-(%s%d+%s%d)", cIn, r, cOut, r, cDaily, r)
			formula := move
			if prev != "" {
				formula = fmt.Sprintf("%s+%s", prev, move)
			}
			if err := f.SetCellFormula(sheet, cell(c0+4, r), formula); err != nil {
				return err
			}
		}
		// Totais do mês.
		for i, label := range []string{"ENTRADAS", "SAÍDAS", "DIÁRIO", "SALDO"} {
			if err := f.SetCellValue(sheet, cell(c0+1+i, dailyTotalsRow-1), label); err != nil {
				return err
			}
		}
		if err := f.SetCellStyle(sheet, cell(c0+1, dailyTotalsRow-1), cell(c0+4, dailyTotalsRow-1), st.headerRight); err != nil {
			return err
		}
		for _, c := range []string{cIn, cOut, cDaily} {
			if err := f.SetCellFormula(sheet, fmt.Sprintf("%s%d", c, dailyTotalsRow),
				fmt.Sprintf("SUM(%s%d:%s%d)", c, dailyFirstDayRow, c, dailyLastDayRow)); err != nil {
				return err
			}
		}
		// Saldo do fim do mês aponta para a linha 33 (dia 31 ou repetição do último dia válido).
		if err := f.SetCellFormula(sheet, fmt.Sprintf("%s%d", cBal, dailyTotalsRow), fmt.Sprintf("%s%d", cBal, dailyLastDayRow)); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell(c0+1, dailyTotalsRow), cell(c0+4, dailyTotalsRow), st.currencyBold); err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("%s%d", cDaily, dailyOutTotalRow-1), "Saída Total"); err != nil {
			return err
		}
		if err := f.SetCellFormula(sheet, fmt.Sprintf("%s%d", cDaily, dailyOutTotalRow),
			fmt.Sprintf("%s%d+%s%d", cOut, dailyTotalsRow, cDaily, dailyTotalsRow)); err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("%s%d", cDaily, dailyPerfRow-1), "Performance"); err != nil {
			return err
		}
		if err := f.SetCellFormula(sheet, fmt.Sprintf("%s%d", cDaily, dailyPerfRow),
			fmt.Sprintf("%s%d-%s%d", cIn, dailyTotalsRow, cDaily, dailyOutTotalRow)); err != nil {
			return err
		}
		for _, r := range []int{dailyOutTotalRow - 1, dailyPerfRow - 1} {
			if err := f.SetCellStyle(sheet, fmt.Sprintf("%s%d", cDaily, r), fmt.Sprintf("%s%d", cDaily, r), st.labelRight); err != nil {
				return err
			}
		}
		for _, r := range []int{dailyOutTotalRow, dailyPerfRow} {
			if err := f.SetCellStyle(sheet, fmt.Sprintf("%s%d", cDaily, r), fmt.Sprintf("%s%d", cDaily, r), st.currencyBold); err != nil {
				return err
			}
		}
		balRange := fmt.Sprintf("%s%d:%s%d", cBal, dailyFirstDayRow, cBal, dailyLastDayRow)
		if err := f.SetConditionalFormat(sheet, balRange, st.bandRules(bands)); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet, cData, cData, 6); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet, cIn, cBal, 13); err != nil {
			return err
		}
		if m < 12 {
			sep := colName(c0 + 5)
			if err := f.SetColWidth(sheet, sep, sep, 3); err != nil {
				return err
			}
		}
	}
	if len(invalidRefs) > 0 {
		invalidDays.SetSqref(strings.Join(invalidRefs, " "))
		if err := f.AddDataValidation(sheet, invalidDays); err != nil {
			return err
		}
	}
	if in.OpeningBalanceCents != 0 {
		// Saldo de abertura entra como Entrada anotada no 1º de janeiro.
		ref := cell(2, dailyFirstDayRow)
		current := int64(0)
		if dt := days[fmt.Sprintf("%04d-01-01", in.Year)]; dt != nil {
			current = dt.income
		}
		if err := f.SetCellValue(sheet, ref, centsToFloat(current+in.OpeningBalanceCents)); err != nil {
			return err
		}
		text := fmt.Sprintf("Inclui o saldo de abertura (saldo total em 31/12/%d): %s", in.Year-1, domain.FormatBRL(in.OpeningBalanceCents))
		if err := f.AddComment(sheet, excelize.Comment{Cell: ref, Author: "FinFlow", Text: text}); err != nil {
			return err
		}
	}
	return f.SetPanes(sheet, &excelize.Panes{
		Freeze: true, YSplit: dailyHeaderRow, TopLeftCell: cell(1, dailyFirstDayRow), ActivePane: "bottomLeft",
	})
}

func writeEconomiaSheet(f *excelize.File, st *styles, yearSheet string, year int, totals YearTotals) error {
	sh := sheetEconomia
	if err := f.SetCellValue(sh, "B2", fmt.Sprintf("Economia %d", year)); err != nil {
		return err
	}
	if err := f.SetCellStyle(sh, "B2", "B2", st.bold); err != nil {
		return err
	}
	for i, h := range []any{year, "Entradas", "Economia", "%"} {
		if err := f.SetCellValue(sh, cell(2+i, 4), h); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(sh, "B4", "E4", st.header); err != nil {
		return err
	}
	ref := sheetRef(yearSheet)
	for m := 1; m <= 12; m++ {
		r := 4 + m
		if err := f.SetCellValue(sh, cell(2, r), monthShort[m-1]); err != nil {
			return err
		}
		incomeCell := fmt.Sprintf("%s!%s%d", ref, colName(blockStartCol(m)+1), dailyTotalsRow)
		if err := f.SetCellFormula(sh, cell(3, r), incomeCell); err != nil {
			return err
		}
		if err := f.SetCellValue(sh, cell(4, r), centsToFloat(totals.Saved[m-1])); err != nil {
			return err
		}
		if err := f.SetCellFormula(sh, cell(5, r), fmt.Sprintf(`IFERROR(D%d/C%d,"")`, r, r)); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(sh, "C5", "D16", st.currency); err != nil {
		return err
	}
	if err := f.SetCellStyle(sh, "E5", "E16", st.percent); err != nil {
		return err
	}
	if err := f.SetCellValue(sh, "B18", "TOTAL"); err != nil {
		return err
	}
	if err := f.SetCellStyle(sh, "B18", "B18", st.label); err != nil {
		return err
	}
	for _, c := range []string{"C", "D"} {
		if err := f.SetCellFormula(sh, c+"18", fmt.Sprintf("SUM(%s5:%s16)", c, c)); err != nil {
			return err
		}
	}
	if err := f.SetCellFormula(sh, "E18", `IFERROR(D18/C18,"")`); err != nil {
		return err
	}
	if err := f.SetCellStyle(sh, "C18", "D18", st.currencyBold); err != nil {
		return err
	}
	if err := f.SetCellStyle(sh, "E18", "E18", st.percentBold); err != nil {
		return err
	}
	if err := f.SetCellValue(sh, "B20", "Economia = transferências para contas poupança/investimento + aportes em metas. % = Economia ÷ Entradas."); err != nil {
		return err
	}
	if err := f.SetCellStyle(sh, "B20", "B20", st.note); err != nil {
		return err
	}
	if err := f.SetColWidth(sh, "A", "A", 3); err != nil {
		return err
	}
	return f.SetColWidth(sh, "B", "E", 14)
}

var typeLabels = map[domain.TransactionType]string{
	domain.TxIncome:         "Receita",
	domain.TxExpense:        "Despesa",
	domain.TxTransfer:       "Transferência",
	domain.TxCardExpense:    "Cartão de crédito",
	domain.TxInvoicePayment: "Pagamento de fatura",
}

var spendTypeLabels = map[SpendType]string{
	SpendFixed:    "Fixo",
	SpendCuttable: "Cortável",
	SpendVariable: "Variável",
}

func spendLabel(t Transaction) string {
	if t.Type != domain.TxExpense && t.Type != domain.TxCardExpense {
		return ""
	}
	if l, ok := spendTypeLabels[t.SpendType]; ok {
		return l
	}
	return spendTypeLabels[SpendVariable]
}

func parseDateValue(d string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", d)
	return t, err == nil
}

func writeLancamentosSheet(f *excelize.File, st *styles, sh string, txs []Transaction) error {
	sorted := append([]Transaction(nil), txs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date < sorted[j].Date })
	headers := []string{"Data", "Descrição", "Tipo", "Categoria", "Tipo de gasto", "Conta/Cartão", "Valor (R$)", "Status", "Parcela"}
	for i, h := range headers {
		if err := f.SetCellValue(sh, cell(1+i, 1), h); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(sh, "A1", cell(len(headers), 1), st.header); err != nil {
		return err
	}
	for i, t := range sorted {
		r := 2 + i
		var dateVal any = t.Date
		if d, ok := parseDateValue(t.Date); ok {
			dateVal = d
		}
		status := "Pendente"
		if t.Status == domain.TxConfirmed {
			status = "Confirmada"
		}
		installment := ""
		if t.InstallmentTotal > 1 {
			installment = fmt.Sprintf("%d/%d", t.InstallmentNumber, t.InstallmentTotal)
		}
		values := []any{dateVal, t.Description, typeLabels[t.Type], t.CategoryName, spendLabel(t), t.AccountName,
			centsToFloat(t.AmountCents), status, installment}
		if err := f.SetSheetRow(sh, cell(1, r), &values); err != nil {
			return err
		}
	}
	last := len(sorted) + 1
	if last >= 2 {
		if err := f.SetCellStyle(sh, "A2", fmt.Sprintf("A%d", last), st.date); err != nil {
			return err
		}
		if err := f.SetCellStyle(sh, "G2", fmt.Sprintf("G%d", last), st.currency); err != nil {
			return err
		}
	}
	if err := f.AutoFilter(sh, fmt.Sprintf("A1:I%d", max(last, 2)), nil); err != nil {
		return err
	}
	for col, w := range map[string]float64{"A": 12, "B": 40, "C": 18, "D": 22, "E": 14, "F": 20, "G": 14, "H": 12, "I": 9} {
		if err := f.SetColWidth(sh, col, col, w); err != nil {
			return err
		}
	}
	return f.SetPanes(sh, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

type catRow struct {
	name   string
	months [12]int64
	total  int64
}

func writeCategoriasSheet(f *excelize.File, st *styles, year int, txs []Transaction) error {
	sh := sheetCategorias
	expense := map[string]*catRow{}
	income := map[string]*catRow{}
	for _, t := range txs {
		var target map[string]*catRow
		var cents int64
		switch t.Type {
		case domain.TxExpense, domain.TxCardExpense:
			// Competência (data da compra), incluindo pendentes — igual aos orçamentos.
			target, cents = expense, -t.AmountCents
		case domain.TxIncome:
			if t.Status != domain.TxConfirmed {
				continue
			}
			target, cents = income, t.AmountCents
		default:
			continue
		}
		d, ok := parseDateValue(t.Date)
		if !ok || d.Year() != year {
			continue
		}
		name := t.CategoryName
		if name == "" {
			name = "Sem categoria"
		}
		row := target[name]
		if row == nil {
			row = &catRow{name: name}
			target[name] = row
		}
		row.months[d.Month()-1] += cents
		row.total += cents
	}
	headers := []string{"Categoria"}
	for _, m := range monthShort {
		headers = append(headers, m)
	}
	headers = append(headers, "Total")
	for i, h := range headers {
		if err := f.SetCellValue(sh, cell(1+i, 1), h); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(sh, "A1", cell(len(headers), 1), st.header); err != nil {
		return err
	}
	r := 2
	writeSection := func(title string, rows map[string]*catRow) error {
		if err := f.SetCellValue(sh, cell(1, r), title); err != nil {
			return err
		}
		if err := f.SetCellStyle(sh, cell(1, r), cell(1, r), st.label); err != nil {
			return err
		}
		r++
		list := make([]*catRow, 0, len(rows))
		for _, row := range rows {
			list = append(list, row)
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].total != list[j].total {
				return list[i].total > list[j].total
			}
			return list[i].name < list[j].name
		})
		first := r
		for _, row := range list {
			if err := f.SetCellValue(sh, cell(1, r), row.name); err != nil {
				return err
			}
			for m := 0; m < 12; m++ {
				if row.months[m] != 0 {
					if err := f.SetCellValue(sh, cell(2+m, r), centsToFloat(row.months[m])); err != nil {
						return err
					}
				}
			}
			if err := f.SetCellFormula(sh, cell(14, r), fmt.Sprintf("SUM(B%d:M%d)", r, r)); err != nil {
				return err
			}
			r++
		}
		if err := f.SetCellStyle(sh, cell(2, first), cell(14, max(r-1, first)), st.currency); err != nil {
			return err
		}
		if err := f.SetCellValue(sh, cell(1, r), "Total "+strings.ToLower(title)); err != nil {
			return err
		}
		for c := 2; c <= 14; c++ {
			col := colName(c)
			formula := "0"
			if r > first {
				formula = fmt.Sprintf("SUM(%s%d:%s%d)", col, first, col, r-1)
			}
			if err := f.SetCellFormula(sh, cell(c, r), formula); err != nil {
				return err
			}
		}
		if err := f.SetCellStyle(sh, cell(1, r), cell(1, r), st.label); err != nil {
			return err
		}
		if err := f.SetCellStyle(sh, cell(2, r), cell(14, r), st.currencyBold); err != nil {
			return err
		}
		r += 2
		return nil
	}
	if err := writeSection("Despesas", expense); err != nil {
		return err
	}
	if err := writeSection("Receitas", income); err != nil {
		return err
	}
	if err := f.SetColWidth(sh, "A", "A", 28); err != nil {
		return err
	}
	if err := f.SetColWidth(sh, "B", "N", 12); err != nil {
		return err
	}
	return f.SetPanes(sh, &excelize.Panes{Freeze: true, XSplit: 1, YSplit: 1, TopLeftCell: "B2", ActivePane: "bottomRight"})
}

// BuildTransactionsWorkbook gera o extrato em uma aba "Lançamentos" (mesmos filtros do extrato).
func BuildTransactionsWorkbook(txs []Transaction) ([]byte, error) {
	f := newFile()
	if err := setupSheets(f, sheetLancamentos); err != nil {
		f.Close()
		return nil, err
	}
	st, err := newStyles(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	if err := writeLancamentosSheet(f, st, sheetLancamentos, txs); err != nil {
		f.Close()
		return nil, err
	}
	return finish(f)
}

// BuildImportTemplate gera o modelo de importação: aba Lançamentos (com exemplos e lista em Tipo) e Como usar.
func BuildImportTemplate(today time.Time) ([]byte, error) {
	f := newFile()
	if err := setupSheets(f, sheetLancamentos, sheetComoUsar); err != nil {
		f.Close()
		return nil, err
	}
	st, err := newStyles(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	sh := sheetLancamentos
	headers := []string{"Data", "Descrição", "Valor", "Tipo", "Categoria", "Conta"}
	for i, h := range headers {
		if err := f.SetCellValue(sh, cell(1+i, 1), h); err != nil {
			f.Close()
			return nil, err
		}
	}
	if err := f.SetCellStyle(sh, "A1", "F1", st.header); err != nil {
		f.Close()
		return nil, err
	}
	base := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	examples := [][]any{
		{base.AddDate(0, 0, 4), "Salário", 5000.00, "Receita", "Salário", ""},
		{base.AddDate(0, 0, 9), "Aluguel", 1800.00, "Despesa", "Aluguel", ""},
		{base.AddDate(0, 0, 11), "Supermercado", 352.47, "Despesa", "Supermercado", ""},
	}
	for i, row := range examples {
		vals := row
		if err := f.SetSheetRow(sh, cell(1, 2+i), &vals); err != nil {
			f.Close()
			return nil, err
		}
	}
	if err := f.SetCellStyle(sh, "A2", "A1000", st.date); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.SetCellStyle(sh, "C2", "C1000", st.currency); err != nil {
		f.Close()
		return nil, err
	}
	dv := excelize.NewDataValidation(true)
	dv.SetSqref("D2:D1000")
	if err := dv.SetDropList([]string{"Receita", "Despesa"}); err != nil {
		f.Close()
		return nil, err
	}
	dv.SetError(excelize.DataValidationErrorStyleStop, "Tipo inválido", "Use Receita ou Despesa.")
	if err := f.AddDataValidation(sh, dv); err != nil {
		f.Close()
		return nil, err
	}
	for col, w := range map[string]float64{"A": 12, "B": 36, "C": 14, "D": 12, "E": 22, "F": 18} {
		if err := f.SetColWidth(sh, col, col, w); err != nil {
			f.Close()
			return nil, err
		}
	}
	if err := f.SetPanes(sh, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		f.Close()
		return nil, err
	}

	help := []string{
		"Como usar este modelo",
		"",
		"1. Preencha uma linha por lançamento na aba Lançamentos (apague as 3 linhas de exemplo).",
		"2. Data: dd/mm/aaaa. Valor: positivo, com vírgula decimal (ex.: 1.234,56).",
		"3. Tipo: Receita ou Despesa. Sem Tipo, valor negativo vira despesa e positivo vira receita.",
		"4. Categoria: nome igual ao cadastrado no FinFlow (maiúsculas e acentos não importam). Se ficar vazia, as regras de automação sugerem uma.",
		"5. Conta: opcional — a conta de destino é escolhida na hora de importar.",
		"6. No FinFlow, abra Planilhas → Importar, envie o arquivo, revise a prévia e confirme.",
		"",
		"Também é possível importar CSV/OFX do banco e a planilha diária (Entrada · Saída · Diário · Saldo) exportada pelo FinFlow.",
	}
	for i, line := range help {
		if err := f.SetCellValue(sheetComoUsar, cell(1, 1+i), line); err != nil {
			f.Close()
			return nil, err
		}
	}
	if err := f.SetCellStyle(sheetComoUsar, "A1", "A1", st.bold); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.SetColWidth(sheetComoUsar, "A", "A", 110); err != nil {
		f.Close()
		return nil, err
	}
	return finish(f)
}
