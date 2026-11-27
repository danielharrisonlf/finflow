package spreadsheet

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"finflow/internal/domain"
)

var yearSheetRe = regexp.MustCompile(`^\d{4}$`)

func dailyYearSheets(sheets []string) []string {
	var out []string
	for _, s := range sheets {
		if name := strings.TrimSpace(s); yearSheetRe.MatchString(name) {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

var monthByName = func() map[string]int {
	m := map[string]int{}
	for i, n := range monthNames {
		m[Normalize(n)] = i + 1
		m[monthShort[i]] = i + 1
	}
	return m
}()

// dailyBlock é um mês localizado na aba: coluna da Data (1-based) e linha do cabeçalho.
type dailyBlock struct {
	month     int
	col       int
	headerRow int
}

// sheetGrid dá acesso por coordenada às células brutas (valores em cache das fórmulas).
type sheetGrid struct {
	f     *excelize.File
	sheet string
	rows  [][]string
}

func (g *sheetGrid) raw(col, row int) string {
	if row < 1 || row > len(g.rows) || col < 1 || col > len(g.rows[row-1]) {
		return ""
	}
	return strings.TrimSpace(g.rows[row-1][col-1])
}

// value devolve o valor da célula; fórmula sem valor em cache (arquivo gerado por biblioteca) é calculada.
func (g *sheetGrid) value(col, row int) string {
	v := g.raw(col, row)
	if v != "" {
		return v
	}
	ref := cell(col, row)
	formula, err := g.f.GetCellFormula(g.sheet, ref)
	if err != nil || formula == "" {
		return ""
	}
	out, err := g.f.CalcCellValue(g.sheet, ref, excelize.Options{RawCellValue: true})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func isDailyHeader(g *sheetGrid, col, row int) bool {
	want := []string{"data", "entrada", "saida", "diario", "saldo"}
	for i, w := range want {
		if Normalize(g.raw(col+i, row)) != w {
			return false
		}
	}
	return true
}

// findDailyBlocks acha os blocos pelo cabeçalho Data|Entrada|Saída|Diário|Saldo nas 10 primeiras linhas.
// O mês vem do título acima do cabeçalho; sem título, da posição (coluna 1 + 6·(mês−1)).
func findDailyBlocks(g *sheetGrid) []dailyBlock {
	var blocks []dailyBlock
	used := map[int]bool{}
	for r := 1; r <= 10 && r <= len(g.rows); r++ {
		for c := 1; c <= len(g.rows[r-1]); c++ {
			if !isDailyHeader(g, c, r) {
				continue
			}
			month := 0
			for up := r - 1; up >= 1 && month == 0; up-- {
				for dc := 0; dc < 5 && month == 0; dc++ {
					month = monthByName[Normalize(g.raw(c+dc, up))]
				}
			}
			if month == 0 && (c-1)%dailyBlockStride == 0 {
				month = (c-1)/dailyBlockStride + 1
			}
			if month < 1 || month > 12 || used[month] {
				continue
			}
			used[month] = true
			blocks = append(blocks, dailyBlock{month: month, col: c, headerRow: r})
		}
	}
	if len(blocks) == 0 && looksLikeDailyLayout(g) {
		for m := 1; m <= 12; m++ {
			blocks = append(blocks, dailyBlock{month: m, col: blockStartCol(m), headerRow: dailyHeaderRow})
		}
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].month < blocks[j].month })
	return blocks
}

// looksLikeDailyLayout aceita o layout posicional quando a linha 1 traz os nomes dos meses nas colunas esperadas.
func looksLikeDailyLayout(g *sheetGrid) bool {
	hits := 0
	for m := 1; m <= 12; m++ {
		if monthByName[Normalize(g.raw(blockStartCol(m), 1))] == m {
			hits++
		}
	}
	return hits >= 6
}

// parseDailySheet lê uma aba de ano; devolve false se ela não tiver o layout diário.
func parseDailySheet(f *excelize.File, sheet string, res *Result) (bool, error) {
	year, err := strconv.Atoi(strings.TrimSpace(sheet))
	if err != nil {
		return false, nil
	}
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return false, fmt.Errorf("não consegui ler a aba %q: %w", sheet, err)
	}
	g := &sheetGrid{f: f, sheet: sheet, rows: rows}
	blocks := findDailyBlocks(g)
	if len(blocks) == 0 {
		return false, nil
	}

	var running int64
	runningKnown := false
	for _, b := range blocks {
		divergent := false
		for i := 0; i < 31; i++ {
			r := b.headerRow + 1 + i
			day := i + 1
			if d, ok := dayFromCell(g.raw(b.col, r)); ok {
				day = d
			}
			type entry struct {
				col  int
				kind domain.CategoryKind
				desc string
				hint SpendType
			}
			var dayNet int64
			anyValue := false
			for _, e := range []entry{
				{b.col + 1, domain.CategoryIncome, dailyDescIncome, ""},
				{b.col + 2, domain.CategoryExpense, dailyDescFixed, SpendFixed},
				{b.col + 3, domain.CategoryExpense, dailyDescVariable, SpendVariable},
			} {
				raw := g.value(e.col, r)
				if raw == "" {
					continue
				}
				cents, ok := parseCellAmount(raw)
				ref := fmt.Sprintf("%s!%s", sheet, cell(e.col, r))
				if !ok {
					res.warn("%s: valor ilegível %q ignorado", ref, raw)
					continue
				}
				if cents == 0 {
					continue
				}
				anyValue = true
				if e.kind == domain.CategoryIncome {
					dayNet += cents
				} else {
					dayNet -= cents
				}
				if day > daysIn(year, b.month) {
					res.warn("%s: dia %d não existe em %s/%d — valor ignorado", ref, day, strings.ToLower(monthNames[b.month-1]), year)
					continue
				}
				kind := e.kind
				if cents < 0 {
					// Valor negativo inverte o tipo (estorno numa saída vira entrada e vice-versa).
					cents = -cents
					if kind == domain.CategoryIncome {
						kind = domain.CategoryExpense
					} else {
						kind = domain.CategoryIncome
					}
				}
				row := Row{
					Date:        fmt.Sprintf("%04d-%02d-%02d", year, b.month, day),
					Description: e.desc,
					AmountCents: cents,
					Kind:        kind,
				}
				if kind == e.kind {
					row.SpendTypeHint = e.hint
					if e.hint == SpendFixed {
						row.CategoryName = strPtr(DailyFixedCategoryHint)
					}
				}
				if !res.addRow(row) {
					return true, nil
				}
			}

			// Recalcula o saldo e compara com a coluna Saldo da planilha (quando há valor em cache).
			balRaw := g.raw(b.col+4, r)
			bal, balOK := int64(0), false
			if balRaw != "" {
				bal, balOK = parseCellAmount(balRaw)
			}
			if !runningKnown {
				if balOK {
					// Primeiro saldo conhecido define o ponto de partida (saldo anterior à aba).
					running = bal - dayNet
					runningKnown = true
				} else if anyValue {
					running = 0
					runningKnown = true
				}
			}
			if runningKnown {
				running += dayNet
				if balOK && !divergent && abs(running-bal) > 1 {
					divergent = true
					res.warn("%s/%d: saldo da planilha (%s) diverge do recalculado (%s) a partir do dia %d",
						strings.ToLower(monthNames[b.month-1]), year, domain.FormatBRL(bal), domain.FormatBRL(running), day)
					// Realinha para não repetir o aviso em cascata nos meses seguintes.
					running = bal
				}
			}
		}
	}
	return true, nil
}

// dayFromCell lê a coluna Data: número do dia (1..31), serial do Excel ou data em texto.
func dayFromCell(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		if v >= 1 && v <= 31 && v == float64(int(v)) {
			return int(v), true
		}
		if d, ok := ExcelSerialToDate(v); ok {
			n, _ := strconv.Atoi(d[8:10])
			return n, true
		}
		return 0, false
	}
	if d, ok := ParseDate(raw, false); ok {
		n, _ := strconv.Atoi(d[8:10])
		return n, true
	}
	return 0, false
}
