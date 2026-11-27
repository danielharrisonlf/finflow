package domain

import "time"

// ───────────────────────── Contratos ─────────────────────────

type ProjectionEvent struct {
	Kind        string `json:"kind"` // "RECURRING" | "INVOICE" | "PENDING" | "DAILY_SPEND"
	Description string `json:"description"`
	AmountCents int64  `json:"amountCents"` // com sinal: + entra, − sai
}

type ProjectionDay struct {
	Date         string            `json:"date"`
	InCents      int64             `json:"inCents"`
	OutCents     int64             `json:"outCents"`
	BalanceCents int64             `json:"balanceCents"`
	Band         string            `json:"band"` // "DARK_GREEN" | "LIGHT_GREEN" | "YELLOW" | "LIGHT_RED" | "RED"
	Events       []ProjectionEvent `json:"events"`
}

type ProjectionLowest struct {
	Date         string `json:"date"`
	BalanceCents int64  `json:"balanceCents"`
}

type ProjectionMonth struct {
	Month             int   `json:"month"`
	Year              int   `json:"year"`
	IncomeCents       int64 `json:"incomeCents"`
	RecurringOutCents int64 `json:"recurringOutCents"`
	InstallmentsCents int64 `json:"installmentsCents"`
	InvoicesCents     int64 `json:"invoicesCents"`
	DailySpendCents   int64 `json:"dailySpendCents"`
	PendingOutCents   int64 `json:"pendingOutCents"`
	EndBalanceCents   int64 `json:"endBalanceCents"`
}

type Projection struct {
	StartDate         string            `json:"startDate"`
	StartBalanceCents int64             `json:"startBalanceCents"`
	LimitConfigured   bool              `json:"limitConfigured"`
	Days              []ProjectionDay   `json:"days"`
	Lowest            ProjectionLowest  `json:"lowest"`
	NextIncomeDate    *string           `json:"nextIncomeDate"`
	CanSaveNowCents   int64             `json:"canSaveNowCents"`
	Months            []ProjectionMonth `json:"months"`
}

const (
	EventRecurring  = "RECURRING"
	EventInvoice    = "INVOICE"
	EventPending    = "PENDING"
	EventDailySpend = "DAILY_SPEND"

	// ProjectionHorizonDays é o horizonte interno da simulação (e o máximo de dias devolvidos).
	ProjectionHorizonDays = 365
	// ProjectionMonths é a quantidade de meses resumidos na projeção.
	ProjectionMonths = 12
)

// DefaultBalanceBands são as faixas padrão do saldo projetado (limites inferiores).
func DefaultBalanceBands() BalanceBands {
	return BalanceBands{DarkGreen: 200000, LightGreen: 100000, Yellow: 0, LightRed: -50000}
}

// BalanceBand classifica o saldo pelas faixas (cada valor é o limite inferior da faixa).
func BalanceBand(balance int64, b BalanceBands) string {
	switch {
	case balance >= b.DarkGreen:
		return "DARK_GREEN"
	case balance >= b.LightGreen:
		return "LIGHT_GREEN"
	case balance >= b.Yellow:
		return "YELLOW"
	case balance >= b.LightRed:
		return "LIGHT_RED"
	default:
		return "RED"
	}
}

// ───────────────────────── Simulação ─────────────────────────

// ProjectionRecurring é uma recorrência ativa com os meses já realizados.
type ProjectionRecurring struct {
	Description string
	Kind        CategoryKind
	AmountCents int64
	Schedule    RecurringSchedule
	Realized    map[string]bool
}

// ProjectionPending é uma transação pendente de conta (valor com sinal).
type ProjectionPending struct {
	Description string
	Date        string
	AmountCents int64
}

// ProjectionInvoice é uma fatura não paga com o total e a parte vinda de parcelas.
type ProjectionInvoice struct {
	Description       string
	DueDate           string
	TotalCents        int64
	InstallmentsCents int64
}

type ProjectionInput struct {
	Today             string
	StartBalanceCents int64
	MonthlyLimitCents *int64
	Bands             BalanceBands
	Recurring         []ProjectionRecurring
	Pending           []ProjectionPending
	Invoices          []ProjectionInvoice
	// InstallmentsByMonth soma as parcelas de cartão (pagas ou não) pelo mês de vencimento da fatura.
	InstallmentsByMonth map[string]int64
	// CurrentMonthVariableCents é o Diário já gasto no mês corrente.
	CurrentMonthVariableCents int64
}

// SimulateProjection roda a simulação diária a partir de hoje. Cobre 365 dias e, se preciso,
// estende até o fim do 12º mês resumido; quem chama corta a lista de dias devolvida.
func SimulateProjection(in ProjectionInput) Projection {
	start := FromDateString(in.Today)
	ty, tm := start.Year(), int(start.Month())
	lastY, lastM := AddMonths(ty, tm, ProjectionMonths-1)
	endOfMonths := time.Date(lastY, time.Month(lastM), DaysInMonth(lastY, lastM), 0, 0, 0, 0, time.UTC)
	n := ProjectionHorizonDays
	if extra := int(endOfMonths.Sub(start).Hours()/24) + 1; extra > n {
		n = extra
	}

	days := make([]ProjectionDay, n)
	for i := range days {
		days[i] = ProjectionDay{Date: ToDateString(start.AddDate(0, 0, i)), Events: []ProjectionEvent{}}
	}
	simEnd := days[n-1].Date
	instByDay := make([]int64, n)

	indexOf := func(date string) int {
		if date <= in.Today {
			return 0
		}
		return int(FromDateString(date).Sub(start).Hours() / 24)
	}
	add := func(date string, ev ProjectionEvent) int {
		if ev.AmountCents == 0 {
			return -1
		}
		idx := indexOf(date)
		if idx >= n {
			return -1
		}
		days[idx].Events = append(days[idx].Events, ev)
		return idx
	}

	for _, p := range in.Pending {
		add(p.Date, ProjectionEvent{Kind: EventPending, Description: p.Description, AmountCents: p.AmountCents})
	}

	endT := FromDateString(simEnd)
	monthSpan := (endT.Year()-ty)*12 + int(endT.Month()) - tm + 1
	for _, r := range in.Recurring {
		sign := int64(-1)
		if r.Kind == CategoryIncome {
			sign = 1
		}
		for i := 0; i < monthSpan; i++ {
			y, m := AddMonths(ty, tm, i)
			date, ok := r.Schedule.OccurrenceIn(y, m)
			if !ok || r.Realized[MonthKeyOf(y, m)] {
				continue
			}
			add(date, ProjectionEvent{Kind: EventRecurring, Description: r.Description, AmountCents: sign * r.AmountCents})
		}
	}

	for _, inv := range in.Invoices {
		if idx := add(inv.DueDate, ProjectionEvent{Kind: EventInvoice, Description: inv.Description, AmountCents: -inv.TotalCents}); idx >= 0 {
			instByDay[idx] += inv.InstallmentsCents
		}
	}

	if in.MonthlyLimitCents != nil {
		for i := 0; i < monthSpan; i++ {
			y, m := AddMonths(ty, tm, i)
			var recurringOut int64
			for _, r := range in.Recurring {
				if r.Kind != CategoryExpense {
					continue
				}
				if _, ok := r.Schedule.OccurrenceIn(y, m); ok {
					recurringOut += r.AmountCents
				}
			}
			budget := *in.MonthlyLimitCents - recurringOut - in.InstallmentsByMonth[MonthKeyOf(y, m)]
			if budget < 0 {
				budget = 0
			}
			firstDay, dayCount := 1, DaysInMonth(y, m)
			if i == 0 {
				firstDay = start.Day()
				dayCount = DaysInMonth(y, m) - start.Day() + 1
				budget -= in.CurrentMonthVariableCents
				if budget < 0 {
					budget = 0
				}
			}
			parts, _ := SplitInstallments(budget, dayCount)
			for k, amt := range parts {
				add(RecurringOccurrenceDate(firstDay+k, y, m), ProjectionEvent{Kind: EventDailySpend, Description: "Gasto diário estimado", AmountCents: -amt})
			}
		}
	}

	balance := in.StartBalanceCents
	for i := range days {
		for _, ev := range days[i].Events {
			if ev.AmountCents > 0 {
				days[i].InCents += ev.AmountCents
			} else {
				days[i].OutCents -= ev.AmountCents
			}
		}
		balance += days[i].InCents - days[i].OutCents
		days[i].BalanceCents = balance
		days[i].Band = BalanceBand(balance, in.Bands)
	}

	out := Projection{
		StartDate:         in.Today,
		StartBalanceCents: in.StartBalanceCents,
		LimitConfigured:   in.MonthlyLimitCents != nil,
		Days:              days,
	}

	lowWindow := min(30, n)
	out.Lowest = ProjectionLowest{Date: days[0].Date, BalanceCents: days[0].BalanceCents}
	for i := 1; i < lowWindow; i++ {
		if days[i].BalanceCents < out.Lowest.BalanceCents {
			out.Lowest = ProjectionLowest{Date: days[i].Date, BalanceCents: days[i].BalanceCents}
		}
	}

	nextIncome := -1
	for i := 0; i < min(ProjectionHorizonDays, n) && nextIncome < 0; i++ {
		for _, ev := range days[i].Events {
			if ev.AmountCents > 0 && (ev.Kind == EventRecurring || ev.Kind == EventPending) {
				nextIncome = i
				d := days[i].Date
				out.NextIncomeDate = &d
				break
			}
		}
	}
	// Menor saldo de hoje até a véspera da próxima entrada (ou 30 dias sem entrada prevista).
	saveWindow := lowWindow
	if nextIncome >= 0 {
		saveWindow = max(1, nextIncome)
	}
	minBal := days[0].BalanceCents
	for i := 1; i < saveWindow; i++ {
		minBal = min(minBal, days[i].BalanceCents)
	}
	out.CanSaveNowCents = max(0, minBal)

	out.Months = make([]ProjectionMonth, ProjectionMonths)
	monthIdx := map[string]int{}
	for i := 0; i < ProjectionMonths; i++ {
		y, m := AddMonths(ty, tm, i)
		out.Months[i] = ProjectionMonth{Month: m, Year: y}
		monthIdx[MonthKeyOf(y, m)] = i
	}
	for i, d := range days {
		mi, ok := monthIdx[MonthKey(d.Date)]
		if !ok {
			continue
		}
		pm := &out.Months[mi]
		pm.EndBalanceCents = d.BalanceCents
		pm.InstallmentsCents += instByDay[i]
		for _, ev := range d.Events {
			switch {
			case ev.AmountCents > 0:
				pm.IncomeCents += ev.AmountCents
			case ev.Kind == EventRecurring:
				pm.RecurringOutCents -= ev.AmountCents
			case ev.Kind == EventInvoice:
				pm.InvoicesCents -= ev.AmountCents
			case ev.Kind == EventDailySpend:
				pm.DailySpendCents -= ev.AmountCents
			case ev.Kind == EventPending:
				pm.PendingOutCents -= ev.AmountCents
			}
		}
	}
	return out
}

// AffordableDates calcula, para cada custo acumulado, a primeira data a partir da qual o
// saldo menos o custo fica >= 0 até o fim da série; nil se não couber.
func AffordableDates(days []ProjectionDay, cumulativeCosts []int64) []*string {
	n := len(days)
	suffixMin := make([]int64, n)
	for i := n - 1; i >= 0; i-- {
		suffixMin[i] = days[i].BalanceCents
		if i+1 < n && suffixMin[i+1] < suffixMin[i] {
			suffixMin[i] = suffixMin[i+1]
		}
	}
	out := make([]*string, len(cumulativeCosts))
	for k, cost := range cumulativeCosts {
		for i := 0; i < n; i++ {
			if suffixMin[i] >= cost {
				d := days[i].Date
				out[k] = &d
				break
			}
		}
	}
	return out
}
