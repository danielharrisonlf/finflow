package domain

import (
	"math"
	"strings"
)

// ───────────────────────── Empresa: grupos do DRE e métricas (v3 fase 2 §5) ─────────────────────────

type DREGroup string

const (
	DRERevenue    DREGroup = "REVENUE"
	DREDeductions DREGroup = "DEDUCTIONS"
	DRECosts      DREGroup = "COSTS"
	DREOpex       DREGroup = "OPEX"
	DREFinancial  DREGroup = "FINANCIAL"
	DREOther      DREGroup = "OTHER"
)

func (g DREGroup) Valid() bool {
	switch g {
	case DRERevenue, DREDeductions, DRECosts, DREOpex, DREFinancial, DREOther:
		return true
	}
	return false
}

var dreGroupIssue = ValidationIssue{Path: "dreGroup", Message: "grupo do DRE deve ser REVENUE, DEDUCTIONS, COSTS, OPEX, FINANCIAL ou OTHER"}

// Palavras (normalizadas) da heurística do grupo do DRE; o plural com "s"/"es" também casa.
var (
	dreFinancialWords = []string{"juro", "juros", "emprestimo", "financiamento", "iof", "rendimento", "aplicacao", "aplicacoes", "financeira", "financeiro"}
	dreDeductionWords = []string{"imposto", "deducao", "deducoes", "simples", "iss", "icms", "pis", "cofins", "tributo", "devolucao", "devolucoes"}
	dreCostWords      = []string{"custo", "fornecedor", "infraestrutura", "cloud", "hospedagem", "servidor", "servidores", "comissao", "comissoes", "mercadoria", "materia", "insumo", "frete", "estoque"}
)

// GuessDREGroup classifica uma categoria pelo nome; nil quando nenhuma palavra conhecida casa.
// Receitas: FINANCIAL (juros, rendimentos) ou nil; despesas: FINANCIAL → DEDUCTIONS → COSTS.
func GuessDREGroup(name string, kind CategoryKind) *DREGroup {
	norm := NormalizeText(name)
	tokens := strings.FieldsFunc(norm, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	var g DREGroup
	switch {
	case hasWord(tokens, dreFinancialWords):
		g = DREFinancial
	case kind == CategoryIncome:
		return nil
	case hasWord(tokens, dreDeductionWords):
		g = DREDeductions
	case hasWord(tokens, dreCostWords):
		g = DRECosts
	default:
		return nil
	}
	return &g
}

// DRENode é o mínimo de uma categoria para resolver o grupo do DRE efetivo.
type DRENode struct {
	Name     string
	Kind     CategoryKind
	ParentID *string
	DREGroup *DREGroup // valor próprio (nil = heurística / herda)
}

// DefaultDREGroup é o grupo de lançamentos sem categoria (ou sem nada que os classifique).
func DefaultDREGroup(kind CategoryKind) DREGroup {
	if kind == CategoryIncome {
		return DRERevenue
	}
	return DREOpex
}

// EffectiveDREGroup resolve o grupo do DRE: o próprio, a heurística pelo nome, o do ancestral
// mais próximo (pelas mesmas regras) ou o padrão do tipo (REVENUE para receitas, OPEX para despesas).
func EffectiveDREGroup(id string, nodes map[string]DRENode) DREGroup {
	node, ok := nodes[id]
	if !ok {
		return DREOpex
	}
	seen := map[string]bool{}
	cur := id
	for !seen[cur] {
		seen[cur] = true
		n, ok := nodes[cur]
		if !ok {
			break
		}
		if n.DREGroup != nil {
			return *n.DREGroup
		}
		if g := GuessDREGroup(n.Name, n.Kind); g != nil {
			return *g
		}
		if n.ParentID == nil {
			break
		}
		cur = *n.ParentID
	}
	return DefaultDREGroup(node.Kind)
}

// ───────────────────────── Métricas da empresa ─────────────────────────

// RecurringIncome é uma recorrência de receita para MRR/churn (as recorrências são mensais).
type RecurringIncome struct {
	AmountCents   int64
	StartDate     string
	EndDate       *string
	Active        bool
	DeactivatedAt *string // data em que active virou false (nil em registros antigos)
}

// ActiveOn indica se a recorrência estava ativa na data: já começou, não terminou e não tinha
// sido desativada (desativadas sem data de desativação contam como inativas sempre).
func (r RecurringIncome) ActiveOn(date string) bool {
	if r.StartDate > date {
		return false
	}
	if r.EndDate != nil && *r.EndDate < date {
		return false
	}
	if r.Active {
		return true
	}
	return r.DeactivatedAt != nil && *r.DeactivatedAt > date
}

// churnedIn indica se a recorrência foi desativada (active → false) ou terminou (endDate) em [start, end).
func (r RecurringIncome) churnedIn(start, end string) bool {
	if !r.Active && r.DeactivatedAt != nil && *r.DeactivatedAt >= start && *r.DeactivatedAt < end {
		return true
	}
	return r.EndDate != nil && *r.EndDate >= start && *r.EndDate < end
}

// MonthCashFlow são as entradas e saídas (positivas) confirmadas de um mês.
type MonthCashFlow struct {
	InCents  int64
	OutCents int64
}

type BusinessMetrics struct {
	Month           int      `json:"month"`
	Year            int      `json:"year"`
	MRRCents        int64    `json:"mrrCents"`
	ARRCents        int64    `json:"arrCents"`
	NewMRRCents     int64    `json:"newMrrCents"`
	ChurnedMRRCents int64    `json:"churnedMrrCents"`
	ChurnPct        *float64 `json:"churnPct"`
	BurnCents       int64    `json:"burnCents"`
	RunwayMonths    *float64 `json:"runwayMonths"`
	CashCents       int64    `json:"cashCents"`
}

// ComputeBusinessMetrics calcula as métricas do mês:
//   - MRR = soma das recorrências de receita ativas no último dia do mês; ARR = 12 × MRR;
//   - novo MRR = recorrências que começaram no mês e estão ativas no fim dele;
//   - MRR perdido = ativas no início do mês (fim do anterior) desativadas ou encerradas no mês;
//     churn = perdido ÷ MRR do início do mês (nil sem MRR inicial);
//   - queima = média de (saídas − entradas) dos 3 meses anteriores, 0 se o caixa cresceu;
//   - runway = caixa ÷ queima (nil sem queima; 0 com caixa negativo).
func ComputeBusinessMetrics(year, month int, recs []RecurringIncome, prev []MonthCashFlow, cashCents int64) BusinessMetrics {
	start, end := MonthRange(year, month)
	lastDay := AddDays(end, -1)
	beforeStart := AddDays(start, -1)
	out := BusinessMetrics{Month: month, Year: year, CashCents: cashCents}
	var startMRR int64
	for _, r := range recs {
		activeEnd := r.ActiveOn(lastDay)
		activeStart := r.ActiveOn(beforeStart)
		if activeEnd {
			out.MRRCents += r.AmountCents
			if r.StartDate >= start {
				out.NewMRRCents += r.AmountCents
			}
		}
		if activeStart {
			startMRR += r.AmountCents
			if r.churnedIn(start, end) {
				out.ChurnedMRRCents += r.AmountCents
			}
		}
	}
	out.ARRCents = 12 * out.MRRCents
	if startMRR > 0 {
		v := PercentOf(out.ChurnedMRRCents, startMRR)
		out.ChurnPct = &v
	}
	if len(prev) > 0 {
		var net int64
		for _, f := range prev {
			net += f.OutCents - f.InCents
		}
		avg := int64(math.Round(float64(net) / float64(len(prev))))
		if avg > 0 {
			out.BurnCents = avg
		}
	}
	if out.BurnCents > 0 {
		v := 0.0
		if cashCents > 0 {
			v = RoundPct(float64(cashCents) / float64(out.BurnCents))
		}
		out.RunwayMonths = &v
	}
	return out
}

// ───────────────────────── DRE ─────────────────────────

// DREEntry é um lançamento classificado: valor com sinal (receitas positivas, despesas negativas).
type DREEntry struct {
	Month       int
	Group       DREGroup
	AmountCents int64
}

// DRELine são as linhas do DRE de um período. Receitas, deduções, custos e despesas operacionais
// vêm positivos; financeiro e outros vêm com sinal (receitas − despesas).
type DRELine struct {
	RevenueCents         int64    `json:"revenueCents"`
	DeductionsCents      int64    `json:"deductionsCents"`
	NetRevenueCents      int64    `json:"netRevenueCents"`
	CostsCents           int64    `json:"costsCents"`
	GrossProfitCents     int64    `json:"grossProfitCents"`
	OpexCents            int64    `json:"opexCents"`
	OperatingResultCents int64    `json:"operatingResultCents"`
	FinancialCents       int64    `json:"financialCents"`
	OtherCents           int64    `json:"otherCents"`
	NetResultCents       int64    `json:"netResultCents"`
	MarginPct            *float64 `json:"marginPct"`
}

func (l *DRELine) add(e DREEntry) {
	switch e.Group {
	case DRERevenue:
		l.RevenueCents += e.AmountCents
	case DREDeductions:
		l.DeductionsCents -= e.AmountCents
	case DRECosts:
		l.CostsCents -= e.AmountCents
	case DREOpex:
		l.OpexCents -= e.AmountCents
	case DREFinancial:
		l.FinancialCents += e.AmountCents
	default:
		l.OtherCents += e.AmountCents
	}
}

// finish calcula os subtotais e a margem líquida (resultado líquido ÷ receita líquida).
func (l *DRELine) finish() {
	l.NetRevenueCents = l.RevenueCents - l.DeductionsCents
	l.GrossProfitCents = l.NetRevenueCents - l.CostsCents
	l.OperatingResultCents = l.GrossProfitCents - l.OpexCents
	l.NetResultCents = l.OperatingResultCents + l.FinancialCents + l.OtherCents
	l.MarginPct = nil
	if l.NetRevenueCents > 0 {
		v := PercentOf(l.NetResultCents, l.NetRevenueCents)
		if l.NetResultCents < 0 {
			v = -PercentOf(-l.NetResultCents, l.NetRevenueCents)
		}
		l.MarginPct = &v
	}
}

type DREMonth struct {
	Month int `json:"month"`
	DRELine
}

type DRE struct {
	Year   int        `json:"year"`
	Months []DREMonth `json:"months"`
	Totals DRELine    `json:"totals"`
}

// BuildDRE monta o DRE anual (12 meses) a partir dos lançamentos classificados.
func BuildDRE(year int, entries []DREEntry) DRE {
	out := DRE{Year: year, Months: make([]DREMonth, 12)}
	for i := range out.Months {
		out.Months[i].Month = i + 1
	}
	for _, e := range entries {
		if e.Month < 1 || e.Month > 12 {
			continue
		}
		out.Months[e.Month-1].add(e)
		out.Totals.add(e)
	}
	for i := range out.Months {
		out.Months[i].finish()
	}
	out.Totals.finish()
	return out
}
