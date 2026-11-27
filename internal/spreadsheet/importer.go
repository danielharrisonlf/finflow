package spreadsheet

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"

	"finflow/internal/domain"
)

// ErrUnsupported indica arquivo de formato desconhecido ou ilegível.
var ErrUnsupported = errors.New("formato de arquivo não suportado — envie xlsx, csv ou ofx")

// Mapping indica o índice (0-based) de cada coluna; nil = não mapeada.
type Mapping struct {
	Date        *int `json:"date"`
	Description *int `json:"description"`
	Amount      *int `json:"amount"`
	Debit       *int `json:"debit"`
	Credit      *int `json:"credit"`
	Type        *int `json:"type"`
	Category    *int `json:"category"`
	Account     *int `json:"account"`
}

// IsZero indica que nenhuma coluna foi mapeada.
func (m Mapping) IsZero() bool {
	return m == Mapping{}
}

// Row é uma linha lida do arquivo, no formato da prévia de importação (contrato v2 §4.2).
type Row struct {
	Index               int                 `json:"index"`
	Date                string              `json:"date"`
	Description         string              `json:"description"`
	AmountCents         int64               `json:"amountCents"`
	Kind                domain.CategoryKind `json:"kind"`
	CategoryName        *string             `json:"categoryName"`
	SuggestedCategoryID *string             `json:"suggestedCategoryId"`
	AccountName         *string             `json:"accountName"`
	Duplicate           bool                `json:"duplicate"`
	Error               *string             `json:"error"`
	// SpendTypeHint sugere o tipo de gasto (Saídas da planilha diária → FIXED); não vai para o JSON.
	SpendTypeHint SpendType `json:"-"`
}

// Result é o resultado da leitura de um arquivo.
type Result struct {
	Format   Format   `json:"format"`
	Sheets   []string `json:"sheets"`
	Sheet    *string  `json:"sheet"`
	Columns  []string `json:"columns"`
	Mapping  Mapping  `json:"mapping"`
	Rows     []Row    `json:"rows"`
	Warnings []string `json:"warnings"`
}

// Options ajusta a leitura: aba escolhida e mapeamento manual de colunas (TABLE/CSV).
type Options struct {
	Filename string
	Sheet    string
	Mapping  *Mapping
}

func newResult(format Format) *Result {
	return &Result{Format: format, Sheets: []string{}, Columns: []string{}, Rows: []Row{}, Warnings: []string{}}
}

func (r *Result) warn(format string, args ...any) {
	if len(r.Warnings) >= 50 {
		return
	}
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

func strPtr(s string) *string {
	return &s
}

func (r *Result) addRow(row Row) bool {
	if len(r.Rows) >= maxImportRows {
		return false
	}
	row.Index = len(r.Rows)
	row.Description = strings.Join(strings.Fields(row.Description), " ")
	// Mesmo limite da descrição de um lançamento (120 caracteres).
	if rs := []rune(row.Description); len(rs) > maxDescriptionRunes {
		row.Description = strings.TrimSpace(string(rs[:maxDescriptionRunes]))
	}
	r.Rows = append(r.Rows, row)
	return true
}

// Parse detecta o formato pelo conteúdo (e extensão como desempate) e lê as linhas.
func Parse(data []byte, opts Options) (*Result, error) {
	if len(data) == 0 {
		return nil, errors.New("arquivo vazio")
	}
	var (
		res *Result
		err error
	)
	ext := strings.ToLower(filepath.Ext(opts.Filename))
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		res, err = parseXLSX(data, opts)
	case bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0}):
		return nil, errors.New("arquivos .xls (Excel 97-2003) não são suportados — salve como .xlsx ou .csv")
	case looksLikeOFX(data) || ext == ".ofx":
		res, err = parseOFX(data)
	default:
		res, err = parseCSV(data, opts)
	}
	if err != nil {
		return nil, err
	}
	markInFileDuplicates(res.Rows)
	if len(res.Rows) >= maxImportRows {
		res.warn("o arquivo tem mais de %d linhas; só as primeiras foram lidas", maxImportRows)
	}
	return res, nil
}

// DuplicateKey identifica lançamentos iguais: data, valor, tipo e descrição normalizada.
func DuplicateKey(date string, amountCents int64, kind domain.CategoryKind, description string) string {
	if amountCents < 0 {
		amountCents = -amountCents
	}
	return fmt.Sprintf("%s|%d|%s|%s", date, amountCents, kind, Normalize(description))
}

func markInFileDuplicates(rows []Row) {
	seen := make(map[string]bool, len(rows))
	for i := range rows {
		if rows[i].Error != nil {
			continue
		}
		key := DuplicateKey(rows[i].Date, rows[i].AmountCents, rows[i].Kind, rows[i].Description)
		if seen[key] {
			rows[i].Duplicate = true
		}
		seen[key] = true
	}
}

// ───────────────────────── xlsx ─────────────────────────

func openXLSX(data []byte) (*excelize.File, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    maxUnzipSize,
		UnzipXMLSizeLimit: maxUnzipXMLInMem,
	})
	if err != nil {
		return nil, fmt.Errorf("não consegui abrir a planilha: %w", err)
	}
	return f, nil
}

func parseXLSX(data []byte, opts Options) (*Result, error) {
	f, err := openXLSX(data)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("a planilha não tem abas")
	}

	if opts.Sheet != "" && !containsString(sheets, opts.Sheet) {
		return nil, fmt.Errorf("aba %q não encontrada", opts.Sheet)
	}
	// Planilha diária: abas com nome de ano. Se o usuário escolheu uma aba que não é de ano, lê como tabela.
	yearSheets := dailyYearSheets(sheets)
	if len(yearSheets) > 0 && (opts.Sheet == "" || containsString(yearSheets, opts.Sheet)) {
		res := newResult(FormatDailySheet)
		res.Sheets = sheets
		targets := yearSheets
		if opts.Sheet != "" {
			targets = []string{opts.Sheet}
			res.Sheet = strPtr(opts.Sheet)
		}
		found := false
		for _, sh := range targets {
			ok, err := parseDailySheet(f, sh, res)
			if err != nil {
				return nil, err
			}
			found = found || ok
		}
		if found {
			return res, nil
		}
		// Abas com nome de ano mas sem layout diário: cai para tabela.
	}

	res := newResult(FormatTable)
	res.Sheets = sheets
	sheet := opts.Sheet
	if sheet == "" {
		sheet = pickTableSheet(f, sheets)
	}
	res.Sheet = strPtr(sheet)
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, fmt.Errorf("não consegui ler a aba %q: %w", sheet, err)
	}
	parseTable(rows, opts.Mapping, true, res)
	return res, nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// pickTableSheet prefere a aba "Lançamentos"; senão, a primeira com cabeçalho reconhecível.
func pickTableSheet(f *excelize.File, sheets []string) string {
	for _, sh := range sheets {
		if Normalize(sh) == Normalize(sheetLancamentos) {
			return sh
		}
	}
	for _, sh := range sheets {
		rows, err := f.GetRows(sh, excelize.Options{RawCellValue: true})
		if err != nil {
			continue
		}
		if _, _, ok := detectHeader(rows); ok {
			return sh
		}
	}
	return sheets[0]
}

// ───────────────────────── Tabela (xlsx/CSV) ─────────────────────────

// headerSynonyms lista cabeçalhos aceitos (já normalizados) por campo.
var headerSynonyms = map[string][]string{
	"date": {"data", "date", "dt", "dia", "data lancamento", "data do lancamento", "data movimento",
		"data da movimentacao", "data movimentacao", "data da compra", "data compra", "data transacao",
		"data da transacao", "data operacao", "transaction date", "posted date", "posting date", "vencimento"},
	"description": {"descricao", "historico", "lancamento", "lancamentos", "description", "memo", "detalhe",
		"detalhes", "estabelecimento", "titulo", "nome", "item", "payee", "name", "descricao do lancamento",
		"historico do lancamento", "observacao", "identificador"},
	"amount": {"valor", "valor r$", "valor rs", "valor em r$", "amount", "quantia", "montante", "value",
		"valor do lancamento", "valor da transacao", "valor brl", "total"},
	"debit":    {"debito", "debitos", "saida", "saidas", "debit", "valor debito", "debito r$", "saida r$"},
	"credit":   {"credito", "creditos", "entrada", "entradas", "credit", "valor credito", "credito r$", "entrada r$"},
	"type":     {"tipo", "type", "natureza", "d/c", "dc", "c/d", "cd", "operacao", "tipo de lancamento", "tipo de transacao"},
	"category": {"categoria", "category", "classificacao", "categorias"},
	"account":  {"conta", "account", "conta/cartao", "conta cartao", "cartao", "origem", "banco", "conta origem"},
}

func normalizeHeader(h string) string {
	n := Normalize(h)
	n = strings.NewReplacer("(", " ", ")", " ", ":", " ", "_", " ", ".", " ").Replace(n)
	return strings.Join(strings.Fields(n), " ")
}

func matchHeader(h string) string {
	n := normalizeHeader(h)
	if n == "" {
		return ""
	}
	for _, field := range []string{"date", "description", "amount", "debit", "credit", "type", "category", "account"} {
		for _, syn := range headerSynonyms[field] {
			if n == syn {
				return field
			}
		}
	}
	return ""
}

// detectHeader procura nas 20 primeiras linhas a que mais casa com sinônimos; exige data e valor
// (ou débito/crédito).
func detectHeader(rows [][]string) (int, Mapping, bool) {
	best, bestScore := -1, 0
	var bestMap Mapping
	for i := 0; i < len(rows) && i < 20; i++ {
		var m Mapping
		score := 0
		for j, c := range rows[i] {
			field := matchHeader(c)
			idx := j
			var slot **int
			switch field {
			case "date":
				slot = &m.Date
			case "description":
				slot = &m.Description
			case "amount":
				slot = &m.Amount
			case "debit":
				slot = &m.Debit
			case "credit":
				slot = &m.Credit
			case "type":
				slot = &m.Type
			case "category":
				slot = &m.Category
			case "account":
				slot = &m.Account
			}
			if slot != nil && *slot == nil {
				*slot = &idx
				score++
			}
		}
		hasValue := m.Amount != nil || m.Debit != nil || m.Credit != nil
		if m.Date != nil && hasValue && score > bestScore {
			best, bestScore, bestMap = i, score, m
		}
	}
	return best, bestMap, best >= 0
}

func cellAt(row []string, idx *int) string {
	if idx == nil || *idx < 0 || *idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[*idx])
}

// kindFromType interpreta a coluna Tipo (Receita/Despesa, Crédito/Débito, C/D, income/expense...).
func kindFromType(raw string) (domain.CategoryKind, bool) {
	n := Normalize(raw)
	switch n {
	case "receita", "receitas", "entrada", "entradas", "credito", "c", "cr", "income", "credit", "deposito", "+":
		return domain.CategoryIncome, true
	case "despesa", "despesas", "saida", "saidas", "debito", "d", "db", "expense", "debit", "pagamento", "-",
		"cartao de credito", "cartao", "gasto", "gasto diario", "diario":
		return domain.CategoryExpense, true
	}
	return "", false
}

func validateMapping(m Mapping, cols int, res *Result) Mapping {
	fix := func(name string, p *int) *int {
		if p == nil {
			return nil
		}
		if *p < 0 || *p >= cols {
			res.warn("mapeamento de %q aponta para coluna inexistente (%d) e foi ignorado", name, *p)
			return nil
		}
		v := *p
		return &v
	}
	return Mapping{
		Date: fix("date", m.Date), Description: fix("description", m.Description), Amount: fix("amount", m.Amount),
		Debit: fix("debit", m.Debit), Credit: fix("credit", m.Credit), Type: fix("type", m.Type),
		Category: fix("category", m.Category), Account: fix("account", m.Account),
	}
}

// parseTable lê linhas tabulares; fromXLSX liga seriais de data e números brutos do Excel.
func parseTable(rows [][]string, override *Mapping, fromXLSX bool, res *Result) {
	headerIdx, detected, ok := detectHeader(rows)
	width := 0
	for _, r := range rows {
		width = max(width, len(r))
	}
	start := 0
	if ok {
		start = headerIdx + 1
		for _, h := range rows[headerIdx] {
			res.Columns = append(res.Columns, strings.TrimSpace(h))
		}
		for len(res.Columns) < width {
			res.Columns = append(res.Columns, fmt.Sprintf("Coluna %d", len(res.Columns)+1))
		}
	} else {
		for i := 0; i < width; i++ {
			res.Columns = append(res.Columns, fmt.Sprintf("Coluna %d", i+1))
		}
	}
	mapping := detected
	if override != nil && !override.IsZero() {
		mapping = validateMapping(*override, width, res)
	} else if !ok {
		res.warn("não reconheci o cabeçalho (data, descrição, valor) — informe o mapeamento das colunas")
	}
	res.Mapping = mapping
	if mapping.Date == nil || (mapping.Amount == nil && mapping.Debit == nil && mapping.Credit == nil) {
		return
	}

	amount := ParseAmount
	if fromXLSX {
		amount = parseCellAmount
	}
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if isBlankRow(row) {
			continue
		}
		line := i + 1
		out := Row{Description: cellAt(row, mapping.Description)}
		if c := cellAt(row, mapping.Category); c != "" {
			out.CategoryName = strPtr(c)
		}
		if a := cellAt(row, mapping.Account); a != "" {
			out.AccountName = strPtr(a)
		}
		var problems []string
		rawDate := cellAt(row, mapping.Date)
		if d, ok := ParseDate(rawDate, fromXLSX || plainNumberRe.MatchString(rawDate)); ok {
			out.Date = d
		} else if rawDate == "" {
			problems = append(problems, "data vazia")
		} else {
			problems = append(problems, fmt.Sprintf("data inválida %q", rawDate))
		}

		var signed int64
		valueOK := false
		if mapping.Amount != nil {
			if raw := cellAt(row, mapping.Amount); raw != "" {
				if v, ok := amount(raw); ok {
					signed, valueOK = v, true
				} else {
					problems = append(problems, fmt.Sprintf("valor inválido %q", raw))
				}
			}
		}
		if !valueOK && (mapping.Debit != nil || mapping.Credit != nil) {
			debitRaw, creditRaw := cellAt(row, mapping.Debit), cellAt(row, mapping.Credit)
			var debit, credit int64
			bad := false
			if debitRaw != "" {
				v, ok := amount(debitRaw)
				if !ok {
					problems = append(problems, fmt.Sprintf("débito inválido %q", debitRaw))
					bad = true
				}
				debit = abs(v)
			}
			if creditRaw != "" {
				v, ok := amount(creditRaw)
				if !ok {
					problems = append(problems, fmt.Sprintf("crédito inválido %q", creditRaw))
					bad = true
				}
				credit = abs(v)
			}
			if !bad && (debitRaw != "" || creditRaw != "") {
				signed, valueOK = credit-debit, true
			}
		}
		if !valueOK && len(problems) == 0 {
			problems = append(problems, "valor vazio")
		}

		out.Kind = domain.CategoryIncome
		if signed < 0 {
			out.Kind = domain.CategoryExpense
		}
		if mapping.Type != nil {
			if k, ok := kindFromType(cellAt(row, mapping.Type)); ok {
				out.Kind = k
			}
		}
		out.AmountCents = abs(signed)
		if valueOK && out.AmountCents == 0 {
			problems = append(problems, "valor zerado")
		}
		if out.Description == "" {
			out.Description = defaultDescription(out.Kind)
		}
		if len(problems) > 0 {
			out.Error = strPtr(fmt.Sprintf("linha %d: %s", line, strings.Join(problems, "; ")))
		}
		if !res.addRow(out) {
			return
		}
	}
}

func defaultDescription(kind domain.CategoryKind) string {
	if kind == domain.CategoryIncome {
		return "Receita importada"
	}
	return "Despesa importada"
}

func isBlankRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
