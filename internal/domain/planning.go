package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ───────────────────────── Recorrências ─────────────────────────

// DaysInMonth devolve quantos dias tem o mês.
func DaysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// MonthKey identifica o mês de uma data YYYY-MM-DD ("YYYY-MM").
func MonthKey(date string) string {
	if len(date) < 7 {
		return date
	}
	return date[:7]
}

// MonthKeyOf monta a chave "YYYY-MM" de um mês.
func MonthKeyOf(year, month int) string {
	return fmt.Sprintf("%04d-%02d", year, month)
}

// AddMonths soma delta meses a (year, month).
func AddMonths(year, month, delta int) (int, int) {
	p := AddMonthsToPeriod(InvoicePeriod{Year: year, Month: month}, delta)
	return p.Year, p.Month
}

// RecurringOccurrenceDate é a data da ocorrência no mês; meses curtos usam o último dia.
func RecurringOccurrenceDate(dayOfMonth, year, month int) string {
	day := dayOfMonth
	if last := DaysInMonth(year, month); day > last {
		day = last
	}
	if day < 1 {
		day = 1
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

// RecurringSchedule é o mínimo de uma recorrência para calcular suas ocorrências.
type RecurringSchedule struct {
	DayOfMonth int
	StartDate  string
	EndDate    *string
}

// OccurrenceIn devolve a ocorrência do mês e se ela existe (dentro de [início, fim]).
func (r RecurringSchedule) OccurrenceIn(year, month int) (string, bool) {
	date := RecurringOccurrenceDate(r.DayOfMonth, year, month)
	if date < r.StartDate {
		return date, false
	}
	if r.EndDate != nil && *r.EndDate != "" && date > *r.EndDate {
		return date, false
	}
	return date, true
}

// maxRecurringLookahead limita a busca da próxima ocorrência (50 anos).
const maxRecurringLookahead = 600

// NextRecurringDate é a próxima ocorrência ainda não lançada: a do mês corrente (mesmo
// atrasada) se existir e não estiver realizada; senão a primeira dos meses seguintes.
// realized contém as chaves "YYYY-MM" dos meses já lançados.
func NextRecurringDate(r RecurringSchedule, active bool, realized map[string]bool, today string) *string {
	if !active {
		return nil
	}
	t := FromDateString(today)
	year, month := t.Year(), int(t.Month())
	for i := 0; i < maxRecurringLookahead; i++ {
		y, m := AddMonths(year, month, i)
		if r.EndDate != nil && *r.EndDate != "" && MonthStart(y, m) > *r.EndDate {
			return nil
		}
		date, ok := r.OccurrenceIn(y, m)
		if ok && !realized[MonthKeyOf(y, m)] {
			return &date
		}
	}
	return nil
}

// ───────────────────────── Regras de categorização ─────────────────────────

var accentFold = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y', 'ÿ': 'y',
}

// NormalizeText deixa o texto em minúsculas, sem acentos e com espaços simples, para
// comparações tolerantes ("Padaria São João" == "padaria sao joao").
func NormalizeText(s string) string {
	lower := strings.ToLower(s)
	folded := strings.Map(func(r rune) rune {
		if f, ok := accentFold[r]; ok {
			return f
		}
		return r
	}, lower)
	return strings.Join(strings.Fields(folded), " ")
}

// RuleCandidate é uma regra pronta para casar (categoria com o tipo já resolvido).
type RuleCandidate struct {
	Pattern      string
	CategoryID   string
	CategoryKind CategoryKind
	Priority     int
	TagID        *string // tag aplicada junto com a categoria (v3)
}

// MatchCategoryRule escolhe a categoria da regra que casa com a descrição: o padrão é
// substring sem diferenciar maiúsculas nem acentos; vence a maior prioridade e, no empate,
// o padrão mais longo (depois a ordem recebida). Só considera categorias do mesmo tipo.
func MatchCategoryRule(rules []RuleCandidate, description string, kind CategoryKind) *string {
	r := MatchRule(rules, description, kind)
	if r == nil {
		return nil
	}
	id := r.CategoryID
	return &id
}

// MatchRule devolve a regra vencedora (mesmos critérios de MatchCategoryRule) ou nil.
func MatchRule(rules []RuleCandidate, description string, kind CategoryKind) *RuleCandidate {
	desc := NormalizeText(description)
	if desc == "" {
		return nil
	}
	bestIdx, bestLen := -1, 0
	for i, r := range rules {
		if r.CategoryKind != kind {
			continue
		}
		pat := NormalizeText(r.Pattern)
		if pat == "" || !strings.Contains(desc, pat) {
			continue
		}
		plen := utf8.RuneCountInString(pat)
		if bestIdx < 0 || r.Priority > rules[bestIdx].Priority || (r.Priority == rules[bestIdx].Priority && plen > bestLen) {
			bestIdx, bestLen = i, plen
		}
	}
	if bestIdx < 0 {
		return nil
	}
	r := rules[bestIdx]
	return &r
}

// CategoryKindForTransaction mapeia o tipo da transação para o tipo de categoria que as
// regras podem aplicar (INCOME ↔ INCOME; EXPENSE/CARD_EXPENSE ↔ EXPENSE).
func CategoryKindForTransaction(t TransactionType) (CategoryKind, bool) {
	switch t {
	case TxIncome:
		return CategoryIncome, true
	case TxExpense, TxCardExpense:
		return CategoryExpense, true
	}
	return "", false
}

// ───────────────────────── Tipo de gasto ─────────────────────────

// Palavras (já normalizadas) usadas para classificar categorias existentes; o plural com
// "s" também casa. A ordem de checagem é Cortável → Fixo → Variável.
var (
	cuttableWords = []string{"assinatura", "streaming", "netflix", "spotify", "musica", "jogo", "tv", "tarifa", "academia", "clube", "digital", "digitais"}
	fixedWords    = []string{"aluguel", "iptu", "condominio", "energia", "luz", "agua", "gas", "internet", "celular", "telefone", "moradia", "utilidade", "educacao", "escola", "faculdade", "mensalidade", "curso", "seguro", "emprestimo", "financiamento", "consorcio", "imposto", "ipva", "servico", "servicos", "plano"}
	variableWords = []string{"alimentacao", "mercado", "supermercado", "restaurante", "delivery", "ifood", "padaria", "transporte", "uber", "combustivel", "gasolina", "automotivo", "saude", "farmacia", "compra", "roupa", "viagem", "lazer", "pet", "doacao", "presente", "outro", "outros"}
)

func hasWord(tokens []string, words []string) bool {
	for _, tok := range tokens {
		for _, w := range words {
			if tok == w || tok == w+"s" || tok == w+"es" {
				return true
			}
		}
	}
	return false
}

// GuessSpendType classifica uma despesa pelo nome; nil quando nenhuma palavra conhecida casa.
func GuessSpendType(name string) *SpendType {
	norm := NormalizeText(name)
	tokens := strings.FieldsFunc(norm, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	var st SpendType
	switch {
	case hasWord(tokens, cuttableWords):
		st = SpendCuttable
	case hasWord(tokens, fixedWords):
		st = SpendFixed
	case hasWord(tokens, variableWords):
		st = SpendVariable
	default:
		return nil
	}
	return &st
}

// CategoryNode é o mínimo de uma categoria para resolver o tipo de gasto efetivo.
type CategoryNode struct {
	Kind      CategoryKind
	ParentID  *string
	SpendType *SpendType // valor próprio (nil = herda)
}

// EffectiveSpendType resolve o tipo de gasto: o próprio, o do ancestral mais próximo que
// tiver um, ou VARIABLE. Receitas não têm tipo de gasto (nil).
func EffectiveSpendType(id string, nodes map[string]CategoryNode) *SpendType {
	node, ok := nodes[id]
	if !ok || node.Kind != CategoryExpense {
		return nil
	}
	seen := map[string]bool{}
	cur := id
	for !seen[cur] {
		seen[cur] = true
		n, ok := nodes[cur]
		if !ok {
			break
		}
		if n.SpendType != nil {
			st := *n.SpendType
			return &st
		}
		if n.ParentID == nil {
			break
		}
		cur = *n.ParentID
	}
	st := SpendVariable
	return &st
}
