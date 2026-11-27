// Package spreadsheet gera e lê planilhas (xlsx, CSV e OFX) sem acessar o banco.
//
// Exportação: pasta de trabalho anual no formato "planilha diária (Entrada · Saída · Diário · Saldo)",
// extrato em uma aba e modelo de importação. Importação: detecção de formato (DAILY_SHEET, TABLE,
// CSV, OFX), leitura de datas e valores no padrão brasileiro e marcação de duplicados no arquivo.
// Dinheiro sempre em centavos inteiros; datas YYYY-MM-DD.
package spreadsheet

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/xuri/excelize/v2"
)

// SpendType espelha o tipo de gasto da categoria (contrato v2 §1.1).
type SpendType string

const (
	SpendFixed    SpendType = "FIXED"
	SpendCuttable SpendType = "CUTTABLE"
	SpendVariable SpendType = "VARIABLE"
)

// Format é o formato detectado de um arquivo importado.
type Format string

const (
	FormatDailySheet Format = "DAILY_SHEET"
	FormatTable      Format = "TABLE"
	FormatCSV        Format = "CSV"
	FormatOFX        Format = "OFX"
)

// Bands são os limites inferiores (em centavos) das faixas de cor do saldo diário.
type Bands struct {
	DarkGreen  int64 `json:"darkGreen"`
	LightGreen int64 `json:"lightGreen"`
	Yellow     int64 `json:"yellow"`
	LightRed   int64 `json:"lightRed"`
}

// DefaultBands devolve as faixas padrão do contrato (R$ 2.000, R$ 1.000, R$ 0, −R$ 500).
func DefaultBands() Bands {
	return Bands{DarkGreen: 200000, LightGreen: 100000, Yellow: 0, LightRed: -50000}
}

// Cores das faixas de saldo (verde escuro → vermelho).
const (
	colorDarkGreen  = "6AA84F"
	colorLightGreen = "B7E1CD"
	colorYellow     = "FCE8B2"
	colorLightRed   = "F4CCCC"
	colorRed        = "FF0000"
)

const (
	currencyFormat = `[$R$ -416]#,##0.00`
	percentFormat  = `0.00%`
	dateFormat     = `dd/mm/yyyy`

	sheetEconomia       = "Economia"
	sheetLancamentos    = "Lançamentos"
	sheetCategorias     = "Categorias"
	sheetComoUsar       = "Como usar"
	defaultSheetName    = "Sheet1"
	dailyBlockStride    = 6
	dailyHeaderRow      = 2
	dailyFirstDayRow    = 3
	dailyLastDayRow     = 33
	dailyTotalsRow      = 38
	dailyOutTotalRow    = 41
	dailyPerfRow        = 44
	maxImportRows       = 20000
	maxDescriptionRunes = 120
	maxUnzipSize        = 128 << 20
	maxUnzipXMLInMem    = 64 << 20
	dailyDescIncome     = "Entrada (planilha)"
	dailyDescFixed      = "Saída (planilha)"
	dailyDescVariable   = "Gasto diário (planilha)"
	// DailyFixedCategoryHint é o nome de categoria sugerido para as Saídas da planilha diária.
	DailyFixedCategoryHint = "Custo de vida"
)

var monthNames = [12]string{
	"JANEIRO", "FEVEREIRO", "MARÇO", "ABRIL", "MAIO", "JUNHO",
	"JULHO", "AGOSTO", "SETEMBRO", "OUTUBRO", "NOVEMBRO", "DEZEMBRO",
}

var monthShort = [12]string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}

var accentStripper = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// Normalize remove acentos, baixa a caixa e colapsa espaços — base para comparar cabeçalhos,
// descrições (duplicados) e nomes de categoria.
func Normalize(s string) string {
	out, _, err := transform.String(accentStripper, s)
	if err != nil {
		out = s
	}
	return strings.Join(strings.Fields(strings.ToLower(out)), " ")
}

// colName converte índice de coluna 1-based em letras ("A", "BS").
func colName(col int) string {
	name, err := excelize.ColumnNumberToName(col)
	if err != nil {
		return "A"
	}
	return name
}

func cell(col, row int) string {
	name, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return "A1"
	}
	return name
}

// blockStartCol devolve a coluna (1-based) da coluna Data do bloco do mês (1..12).
func blockStartCol(month int) int {
	return 1 + dailyBlockStride*(month-1)
}

func centsToFloat(cents int64) float64 {
	return float64(cents) / 100
}

func daysIn(year, month int) int {
	switch month {
	case 4, 6, 9, 11:
		return 30
	case 2:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	}
	return 31
}
