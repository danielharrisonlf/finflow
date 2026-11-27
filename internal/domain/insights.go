package domain

import (
	"fmt"
	"math"
	"sort"
)

// ───────────────────────── Contratos ─────────────────────────

type InsightsPace struct {
	CurrentCents            int64  `json:"currentCents"`
	PreviousSamePeriodCents int64  `json:"previousSamePeriodCents"`
	DiffCents               int64  `json:"diffCents"`
	DayOfMonth              int    `json:"dayOfMonth"`
	Status                  string `json:"status"` // "BELOW" | "SAME" | "ABOVE"
	// v3 §4: série acumulada dia a dia e gasto projetado para o mês.
	Series              []PacePoint `json:"series"`
	ProjectedMonthCents int64       `json:"projectedMonthCents"`
}

type MonthlyLimitView struct {
	LimitCents          *int64   `json:"limitCents"`
	SpentCents          int64    `json:"spentCents"`
	RemainingCents      *int64   `json:"remainingCents"`
	DailyAllowanceCents *int64   `json:"dailyAllowanceCents"`
	Percent             *float64 `json:"percent"`
}

type PerformanceView struct {
	IncomeCents              int64    `json:"incomeCents"`
	FixedCents               int64    `json:"fixedCents"`
	VariableCents            int64    `json:"variableCents"`
	TotalOutCents            int64    `json:"totalOutCents"`
	SavedCents               int64    `json:"savedCents"`
	PerformanceCents         int64    `json:"performanceCents"`
	SavingsRatePct           *float64 `json:"savingsRatePct"`
	PreviousPerformanceCents int64    `json:"previousPerformanceCents"`
	Verdict                  string   `json:"verdict"` // "SAVING" | "POSITIVE" | "TIGHT" | "NEGATIVE_SAVED" | "NEGATIVE"
	Message                  string   `json:"message"`
}

type SpendTypeCategory struct {
	CategoryID *string `json:"categoryId"`
	Name       string  `json:"name"`
	Icon       *string `json:"icon"`
	Color      *string `json:"color"`
	TotalCents int64   `json:"totalCents"`
}

type SpendTypeGroup struct {
	SpendType  SpendType           `json:"spendType"`
	TotalCents int64               `json:"totalCents"`
	Percent    float64             `json:"percent"`
	Categories []SpendTypeCategory `json:"categories"`
}

type ReserveView struct {
	GoalID               *string `json:"goalId"`
	AvgMonthlyCostCents  int64   `json:"avgMonthlyCostCents"`
	SuggestedTargetCents int64   `json:"suggestedTargetCents"`
	TargetCents          int64   `json:"targetCents"`
	SavedCents           int64   `json:"savedCents"`
	MonthsCovered        float64 `json:"monthsCovered"`
	Status               string  `json:"status"` // "NONE" | "BUILDING" | "PROTECTED"
}

type CutSuggestion struct {
	Kind         string  `json:"kind"` // "CANCEL" | "REDUCE" | "ABOVE_AVERAGE"
	CategoryID   *string `json:"categoryId"`
	Name         string  `json:"name"`
	CurrentCents int64   `json:"currentCents"`
	AverageCents int64   `json:"averageCents"`
}

type InsightsOverview struct {
	Month          int              `json:"month"`
	Year           int              `json:"year"`
	Pace           InsightsPace     `json:"pace"`
	MonthlyLimit   MonthlyLimitView `json:"monthlyLimit"`
	Performance    PerformanceView  `json:"performance"`
	SpendTypes     []SpendTypeGroup `json:"spendTypes"`
	Plan           *SpendPlan       `json:"plan"`
	NetWorthCents  int64            `json:"netWorthCents"`
	Reserve        ReserveView      `json:"reserve"`
	CutSuggestions []CutSuggestion  `json:"cutSuggestions"`
	// SpendToday é sempre relativo a hoje (mês corrente), qualquer que seja o mês consultado.
	SpendToday SpendToday `json:"spendToday"`
}

type YearMonthOverview struct {
	Month            int      `json:"month"`
	IncomeCents      int64    `json:"incomeCents"`
	FixedCents       int64    `json:"fixedCents"`
	VariableCents    int64    `json:"variableCents"`
	TotalOutCents    int64    `json:"totalOutCents"`
	PerformanceCents int64    `json:"performanceCents"`
	SavedCents       int64    `json:"savedCents"`
	SavingsRatePct   *float64 `json:"savingsRatePct"`
	EndBalanceCents  int64    `json:"endBalanceCents"`
}

type YearTotals struct {
	IncomeCents      int64    `json:"incomeCents"`
	FixedCents       int64    `json:"fixedCents"`
	VariableCents    int64    `json:"variableCents"`
	TotalOutCents    int64    `json:"totalOutCents"`
	PerformanceCents int64    `json:"performanceCents"`
	SavedCents       int64    `json:"savedCents"`
	SavingsRatePct   *float64 `json:"savingsRatePct"`
}

type YearOverview struct {
	Year   int                 `json:"year"`
	Months []YearMonthOverview `json:"months"`
	Totals YearTotals          `json:"totals"`
}

type CommitmentMonth struct {
	Month                 int   `json:"month"`
	Year                  int   `json:"year"`
	RecurringExpenseCents int64 `json:"recurringExpenseCents"`
	RecurringIncomeCents  int64 `json:"recurringIncomeCents"`
	InstallmentsCents     int64 `json:"installmentsCents"`
	LoansCents            int64 `json:"loansCents"` // parcelas de empréstimos que vencem no mês
}

type CommitmentItem struct {
	Kind              string  `json:"kind"`      // "RECURRING" | "INSTALLMENT" | "LOAN"
	Direction         string  `json:"direction"` // "PAY" | "RECEIVE"
	Description       string  `json:"description"`
	DueDate           string  `json:"dueDate"`
	AmountCents       int64   `json:"amountCents"`
	RecurringID       *string `json:"recurringId"`
	Realized          bool    `json:"realized"`
	InstallmentNumber *int    `json:"installmentNumber"`
	InstallmentTotal  *int    `json:"installmentTotal"`
	CardName          *string `json:"cardName"`
	LoanID            *string `json:"loanId"`
}

type CommitmentTotals struct {
	PayCents          int64 `json:"payCents"`
	ReceiveCents      int64 `json:"receiveCents"`
	BalanceAfterCents int64 `json:"balanceAfterCents"`
}

type Commitments struct {
	Months []CommitmentMonth `json:"months"`
	Items  []CommitmentItem  `json:"items"`
	Totals CommitmentTotals  `json:"totals"`
}

// ───────────────────────── Cálculos ─────────────────────────

// RoundPct arredonda percentuais para 1 casa (como orçamentos e metas).
func RoundPct(v float64) float64 {
	return math.Round(v*10) / 10
}

// PercentOf devolve part/total em % com 1 casa (0 quando total <= 0).
func PercentOf(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return RoundPct(float64(part) / float64(total) * 100)
}

// SavingsRate = economia ÷ entradas (%), nil sem entradas.
func SavingsRate(savedCents, incomeCents int64) *float64 {
	if incomeCents <= 0 {
		return nil
	}
	v := PercentOf(savedCents, incomeCents)
	return &v
}

// MonthFlows reúne os números de um mês pelo método da planilha diária.
type MonthFlows struct {
	IncomeCents   int64 // Entradas: INCOME confirmadas
	FixedCents    int64 // Saídas: FIXED + CUTTABLE
	VariableCents int64 // Diário: VARIABLE + sem categoria
	SavedCents    int64 // Economia: transferências para SAVINGS/INVESTMENT + aportes em metas
}

func (f MonthFlows) TotalOutCents() int64    { return f.FixedCents + f.VariableCents }
func (f MonthFlows) PerformanceCents() int64 { return f.IncomeCents - f.TotalOutCents() }

// ComputePace compara o gasto até o dia com o mesmo intervalo do mês anterior;
// SAME quando |diferença| <= 5% do anterior (ou ambos zero).
func ComputePace(currentCents, previousCents int64, dayOfMonth int) InsightsPace {
	diff := currentCents - previousCents
	absDiff := diff
	if absDiff < 0 {
		absDiff = -absDiff
	}
	status := "SAME"
	switch {
	case currentCents == 0 && previousCents == 0:
	case absDiff*100 <= previousCents*5:
	case diff > 0:
		status = "ABOVE"
	default:
		status = "BELOW"
	}
	return InsightsPace{
		CurrentCents:            currentCents,
		PreviousSamePeriodCents: previousCents,
		DiffCents:               diff,
		DayOfMonth:              dayOfMonth,
		Status:                  status,
	}
}

// ComputeMonthlyLimit monta o limite mensal; daysLeft > 0 só no mês corrente
// (dias restantes incluindo hoje) e define o quanto ainda dá para gastar por dia.
func ComputeMonthlyLimit(limitCents *int64, spentCents int64, daysLeft int) MonthlyLimitView {
	view := MonthlyLimitView{SpentCents: spentCents}
	if limitCents == nil {
		return view
	}
	limit := *limitCents
	remaining := limit - spentCents
	pct := PercentOf(spentCents, limit)
	view.LimitCents = &limit
	view.RemainingCents = &remaining
	view.Percent = &pct
	if daysLeft > 0 {
		daily := int64(0)
		if remaining > 0 {
			daily = remaining / int64(daysLeft)
		}
		view.DailyAllowanceCents = &daily
	}
	return view
}

// PerformanceVerdict classifica o mês pelo método da planilha diária, em que guardar dinheiro
// conta como saída. Performance (sem a economia) negativa é sempre alarme: os gastos superaram as
// entradas. NEGATIVE_SAVED é o caso tranquilo em que só ficou negativo porque se guardou mais do que
// sobrou (usando saldo de meses anteriores).
func PerformanceVerdict(f MonthFlows) string {
	perf := f.PerformanceCents()
	switch {
	case perf < 0:
		return "NEGATIVE"
	case f.SavedCents > perf:
		return "NEGATIVE_SAVED"
	case f.SavedCents > 0:
		return "SAVING"
	case perf*100 < f.IncomeCents*5:
		return "TIGHT"
	default:
		return "POSITIVE"
	}
}

func verdictMessage(verdict string, f MonthFlows) string {
	perf := FormatBRL(f.PerformanceCents())
	saved := FormatBRL(f.SavedCents)
	switch verdict {
	case "SAVING":
		return fmt.Sprintf("Mês saudável: sobraram %s e você guardou %s.", perf, saved)
	case "NEGATIVE_SAVED":
		return fmt.Sprintf("Você guardou %s, mais do que os %s que sobraram, usando saldo dos meses anteriores. Tudo bem enquanto o saldo projetado continuar positivo.", saved, perf)
	case "NEGATIVE":
		if f.SavedCents > 0 {
			return fmt.Sprintf("Atenção: seus gastos superaram as entradas em %s. Revise as saídas antes de guardar mais.", FormatBRL(-f.PerformanceCents()))
		}
		return fmt.Sprintf("Atenção: você gastou %s a mais do que recebeu e não guardou nada neste mês.", FormatBRL(-f.PerformanceCents()))
	case "TIGHT":
		return fmt.Sprintf("Mês apertado: sobraram só %s (menos de 5%% das entradas).", perf)
	default:
		return fmt.Sprintf("Mês positivo: sobraram %s. Que tal guardar parte desse valor?", perf)
	}
}

// ComputePerformance monta o bloco de performance do mês.
func ComputePerformance(f MonthFlows, previousPerformanceCents int64) PerformanceView {
	verdict := PerformanceVerdict(f)
	return PerformanceView{
		IncomeCents:              f.IncomeCents,
		FixedCents:               f.FixedCents,
		VariableCents:            f.VariableCents,
		TotalOutCents:            f.TotalOutCents(),
		SavedCents:               f.SavedCents,
		PerformanceCents:         f.PerformanceCents(),
		SavingsRatePct:           SavingsRate(f.SavedCents, f.IncomeCents),
		PreviousPerformanceCents: previousPerformanceCents,
		Verdict:                  verdict,
		Message:                  verdictMessage(verdict, f),
	}
}

// CategorySpend é o gasto de uma categoria (ou "sem categoria") num período.
type CategorySpend struct {
	CategoryID *string
	Name       string
	Icon       *string
	Color      *string
	SpendType  SpendType
	TotalCents int64
}

func (c CategorySpend) key() string {
	if c.CategoryID == nil {
		return ""
	}
	return *c.CategoryID
}

// SplitFlows separa o gasto em Saídas (FIXED + CUTTABLE) e Diário (VARIABLE).
func SplitFlows(items []CategorySpend) (fixed, variable int64) {
	for _, it := range items {
		if it.SpendType == SpendVariable {
			variable += it.TotalCents
		} else {
			fixed += it.TotalCents
		}
	}
	return fixed, variable
}

var spendTypeOrder = []SpendType{SpendFixed, SpendCuttable, SpendVariable}

// GroupBySpendType agrupa o gasto nos três tipos (sempre na ordem Fixo, Cortável, Variável),
// com as categorias de cada grupo em ordem decrescente de valor.
func GroupBySpendType(items []CategorySpend) []SpendTypeGroup {
	var total int64
	for _, it := range items {
		total += it.TotalCents
	}
	out := make([]SpendTypeGroup, 0, len(spendTypeOrder))
	for _, st := range spendTypeOrder {
		g := SpendTypeGroup{SpendType: st, Categories: []SpendTypeCategory{}}
		for _, it := range items {
			if it.SpendType != st || it.TotalCents == 0 {
				continue
			}
			g.TotalCents += it.TotalCents
			g.Categories = append(g.Categories, SpendTypeCategory{
				CategoryID: it.CategoryID,
				Name:       it.Name,
				Icon:       it.Icon,
				Color:      it.Color,
				TotalCents: it.TotalCents,
			})
		}
		sort.SliceStable(g.Categories, func(i, j int) bool { return g.Categories[i].TotalCents > g.Categories[j].TotalCents })
		g.Percent = PercentOf(g.TotalCents, total)
		out = append(out, g)
	}
	return out
}

// AverageRecentCost é a média (arredondada) dos até 3 primeiros valores positivos da lista,
// que deve vir do mês mais recente para o mais antigo.
func AverageRecentCost(monthlyCosts []int64) int64 {
	var sum, n int64
	for _, c := range monthlyCosts {
		if c <= 0 {
			continue
		}
		sum += c
		n++
		if n == 3 {
			break
		}
	}
	if n == 0 {
		return 0
	}
	return (sum + n/2) / n
}

// ReserveGoalInfo é a meta RESERVE ativa (quando existe).
type ReserveGoalInfo struct {
	ID          string
	TargetCents int64
	SavedCents  int64
}

// ComputeReserve calcula a reserva de emergência: meta sugerida = custo médio × meses
// (12 com dependentes); sem meta RESERVE o status é NONE.
func ComputeReserve(goal *ReserveGoalInfo, avgMonthlyCost int64, settings Settings) ReserveView {
	months := int64(settings.ReserveMonths)
	if settings.HasDependents {
		months = 12
	}
	view := ReserveView{
		AvgMonthlyCostCents:  avgMonthlyCost,
		SuggestedTargetCents: avgMonthlyCost * months,
		Status:               "NONE",
	}
	view.TargetCents = view.SuggestedTargetCents
	if goal != nil {
		id := goal.ID
		view.GoalID = &id
		view.TargetCents = goal.TargetCents
		view.SavedCents = goal.SavedCents
		view.Status = "BUILDING"
		if goal.TargetCents > 0 && goal.SavedCents >= goal.TargetCents {
			view.Status = "PROTECTED"
		}
	}
	if avgMonthlyCost > 0 {
		view.MonthsCovered = RoundPct(float64(view.SavedCents) / float64(avgMonthlyCost))
	}
	return view
}

// ComputeCutSuggestions aponta onde dá para cortar. history traz os 3 meses anteriores
// (a média divide por len(history), contando meses sem gasto como zero).
//   - CANCEL: categorias CUTTABLE com gasto no mês;
//   - ABOVE_AVERAGE: VARIABLE acima de 120% da média dos meses anteriores;
//   - REDUCE: as 3 maiores VARIABLE do mês que ainda não apareceram.
func ComputeCutSuggestions(current []CategorySpend, history [][]CategorySpend) []CutSuggestion {
	sums := map[string]int64{}
	for _, month := range history {
		for _, it := range month {
			sums[it.key()] += it.TotalCents
		}
	}
	avg := func(k string) int64 {
		if len(history) == 0 {
			return 0
		}
		n := int64(len(history))
		return (sums[k] + n/2) / n
	}

	sorted := append([]CategorySpend(nil), current...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].TotalCents > sorted[j].TotalCents })

	out := []CutSuggestion{}
	used := map[string]bool{}
	add := func(kind string, it CategorySpend) {
		out = append(out, CutSuggestion{
			Kind:         kind,
			CategoryID:   it.CategoryID,
			Name:         it.Name,
			CurrentCents: it.TotalCents,
			AverageCents: avg(it.key()),
		})
		used[it.key()] = true
	}
	for _, it := range sorted {
		if it.SpendType == SpendCuttable && it.TotalCents > 0 {
			add("CANCEL", it)
		}
	}
	for _, it := range sorted {
		a := avg(it.key())
		if it.SpendType == SpendVariable && a > 0 && it.TotalCents*100 > a*120 {
			add("ABOVE_AVERAGE", it)
		}
	}
	reduced := 0
	for _, it := range sorted {
		if reduced == 3 {
			break
		}
		if it.SpendType != SpendVariable || it.TotalCents <= 0 || used[it.key()] {
			continue
		}
		add("REDUCE", it)
		reduced++
	}
	return out
}
