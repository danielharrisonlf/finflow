package domain

import (
	"fmt"
	"sort"
	"strings"
)

// ───────────────────────── Alertas e dicas ─────────────────────────

const (
	AlertBudgetWarning        = "BUDGET_WARNING"
	AlertBudgetDanger         = "BUDGET_DANGER"
	AlertLimitPace            = "LIMIT_PACE"
	AlertPaceAbove            = "PACE_ABOVE"
	AlertNegativeProjection   = "NEGATIVE_PROJECTION"
	AlertBillDue              = "BILL_DUE"
	AlertInvoiceDue           = "INVOICE_DUE"
	AlertLoanDue              = "LOAN_DUE"
	AlertSubscriptionPriceUp  = "SUBSCRIPTION_PRICE_UP"
	AlertNewSubscription      = "NEW_SUBSCRIPTION"
	AlertCategoryAboveAverage = "CATEGORY_ABOVE_AVERAGE"
	AlertReserveLow           = "RESERVE_LOW"
	AlertGoalBehind           = "GOAL_BEHIND"
	AlertPendingOld           = "PENDING_OLD"
	AlertUncategorized        = "UNCATEGORIZED"
	AlertInvestmentStale      = "INVESTMENT_STALE"

	SeverityInfo    = "INFO"
	SeverityWarning = "WARNING"
	SeverityDanger  = "DANGER"

	// AlertDueWindowDays: contas, faturas e parcelas vencendo em até 3 dias.
	AlertDueWindowDays = 3
	// AlertOverdueWindowDays: vencidas e não pagas continuam no feed por até 7 dias.
	AlertOverdueWindowDays = 7
	// AlertDismissRetentionDays: alertas dispensados são esquecidos depois de 90 dias.
	AlertDismissRetentionDays = 90
	// MaxAlertIDChars limita o id aceito em POST /api/alerts/{id}/dismiss.
	MaxAlertIDChars = 200
)

type Alert struct {
	ID       string `json:"id"` // "<kind>:<ref>:<YYYY-MM>"
	Kind     string `json:"kind"`
	Severity string `json:"severity"` // "INFO" | "WARNING" | "DANGER"
	Title    string `json:"title"`
	Message  string `json:"message"`
	Href     string `json:"href"`
	Date     string `json:"date"`
}

type AlertBudget struct {
	CategoryID  string
	Name        string
	Percent     float64
	SpentCents  int64
	AmountCents int64
}

// AlertDue é uma conta fixa, fatura ou parcela de empréstimo ainda não paga.
type AlertDue struct {
	Ref         string
	Description string
	DueDate     string
	AmountCents int64
}

type AlertGoal struct {
	ID                 string
	Name               string
	MonthlyNeededCents int64
	Last30DaysCents    int64
}

type AlertInvestment struct {
	ID             string
	Name           string
	ValueUpdatedAt string
}

type AlertInput struct {
	Today            string
	Budgets          []AlertBudget
	LimitCents       *int64
	MonthSpentCents  int64 // gasto do mês corrente (competência)
	Pace             InsightsPace
	ProjectionLowest *ProjectionLowest // menor saldo nos próximos 30 dias
	Bills            []AlertDue
	Invoices         []AlertDue
	Loans            []AlertDue
	Subscriptions    []DetectedSubscription
	CutSuggestions   []CutSuggestion
	Reserve          *ReserveView
	Goals            []AlertGoal
	PendingDates     []string // datas das transações pendentes
	Uncategorized    int      // lançamentos do mês sem categoria
	Investments      []AlertInvestment
}

var severityRank = map[string]int{SeverityDanger: 0, SeverityWarning: 1, SeverityInfo: 2}

func dueLabel(due, today string) string {
	switch d := daysBetween(today, due); {
	case d < 0:
		return fmt.Sprintf("venceu em %s", FormatDateBR(due))
	case d == 0:
		return "vence hoje"
	case d == 1:
		return "vence amanhã"
	default:
		return fmt.Sprintf("vence em %d dias (%s)", d, FormatDateBR(due))
	}
}

// BuildAlerts aplica as regras do feed e devolve os alertas ordenados por severidade e data.
func BuildAlerts(in AlertInput) []Alert {
	month := MonthKey(in.Today)
	out := []Alert{}
	add := func(kind, ref, severity, title, message, href, date string) {
		out = append(out, Alert{
			ID: kind + ":" + ref + ":" + month, Kind: kind, Severity: severity,
			Title: title, Message: message, Href: href, Date: date,
		})
	}

	for _, b := range in.Budgets {
		switch {
		case b.Percent >= 90:
			add(AlertBudgetDanger, b.CategoryID, SeverityDanger, "Orçamento quase estourado: "+b.Name,
				fmt.Sprintf("Você já usou %v%% do orçamento de %s (%s de %s).", b.Percent, b.Name, FormatBRL(b.SpentCents), FormatBRL(b.AmountCents)),
				"/orcamentos", in.Today)
		case b.Percent >= 70:
			add(AlertBudgetWarning, b.CategoryID, SeverityWarning, "Orçamento em atenção: "+b.Name,
				fmt.Sprintf("Você já usou %v%% do orçamento de %s (%s de %s).", b.Percent, b.Name, FormatBRL(b.SpentCents), FormatBRL(b.AmountCents)),
				"/orcamentos", in.Today)
		}
	}

	t := FromDateString(in.Today)
	day, dim := t.Day(), DaysInMonth(t.Year(), int(t.Month()))
	if in.LimitCents != nil && in.MonthSpentCents > 0 {
		projected := ProjectMonthSpend(in.MonthSpentCents, day, dim, true)
		if projected > *in.LimitCents {
			add(AlertLimitPace, "limit", SeverityWarning, "Ritmo acima do limite mensal",
				fmt.Sprintf("No ritmo atual você termina o mês gastando %s, acima do limite de %s.", FormatBRL(projected), FormatBRL(*in.LimitCents)),
				"/configuracoes", in.Today)
		}
	}

	if p := in.Pace; p.Status == "ABOVE" && p.PreviousSamePeriodCents > 0 && p.DiffCents*100 > p.PreviousSamePeriodCents*10 {
		add(AlertPaceAbove, "pace", SeverityWarning, "Gastando mais que no mês passado",
			fmt.Sprintf("Até o dia %d você gastou %s, %s a mais que no mesmo período do mês passado.", p.DayOfMonth, FormatBRL(p.CurrentCents), FormatBRL(p.DiffCents)),
			"/", in.Today)
	}

	if low := in.ProjectionLowest; low != nil && low.BalanceCents < 0 {
		add(AlertNegativeProjection, "projection", SeverityDanger, "Saldo projetado negativo",
			fmt.Sprintf("Pela projeção, seu saldo disponível chega a %s em %s.", FormatBRL(low.BalanceCents), FormatDateBR(low.Date)),
			"/projecao", low.Date)
	}

	limit := AddDays(in.Today, AlertDueWindowDays)
	overdueCut := AddDays(in.Today, -AlertOverdueWindowDays)
	dues := []struct {
		kind, title, href string
		items             []AlertDue
	}{
		{AlertBillDue, "Conta a vencer", "/compromissos", in.Bills},
		{AlertInvoiceDue, "Fatura a vencer", "/contas", in.Invoices},
		{AlertLoanDue, "Parcela de empréstimo a vencer", "/dividas", in.Loans},
	}
	for _, group := range dues {
		for _, d := range group.items {
			if d.DueDate > limit || d.DueDate < overdueCut {
				continue
			}
			add(group.kind, d.Ref, SeverityWarning, group.title+": "+d.Description,
				fmt.Sprintf("%s de %s %s.", d.Description, FormatBRL(d.AmountCents), dueLabel(d.DueDate, in.Today)),
				group.href, d.DueDate)
		}
	}

	for _, s := range in.Subscriptions {
		if s.PriceChangePct != nil && *s.PriceChangePct >= 5 {
			// Já acompanhada como compromisso é uma conta fixa (luz, internet); senão, assinatura detectada.
			title := "Assinatura mais cara: "
			if s.RecurringID != nil {
				title = "Conta mais cara: "
			}
			pct := strings.Replace(fmt.Sprintf("%.1f", *s.PriceChangePct), ".", ",", 1)
			add(AlertSubscriptionPriceUp, s.Key, SeverityWarning, title+s.Description,
				fmt.Sprintf("A última cobrança de %s foi %s, %s%% acima da média anterior.", s.Description, FormatBRL(s.LastCents), pct),
				"/compromissos", s.LastDate)
		}
		if s.Occurrences == 3 && s.RecurringID == nil {
			add(AlertNewSubscription, s.Key, SeverityInfo, "Nova assinatura detectada: "+s.Description,
				fmt.Sprintf("%s aparece todo ciclo (%s). Acompanhe como conta fixa ou ignore.", s.Description, FormatBRL(s.LastCents)),
				"/compromissos", s.LastDate)
		}
	}

	for _, c := range in.CutSuggestions {
		if c.Kind != "ABOVE_AVERAGE" {
			continue
		}
		ref := "none"
		if c.CategoryID != nil {
			ref = *c.CategoryID
		}
		add(AlertCategoryAboveAverage, ref, SeverityInfo, "Gasto acima da média: "+c.Name,
			fmt.Sprintf("%s está em %s neste mês; a média dos 3 meses anteriores é %s.", c.Name, FormatBRL(c.CurrentCents), FormatBRL(c.AverageCents)),
			"/", in.Today)
	}

	if r := in.Reserve; r != nil && r.AvgMonthlyCostCents > 0 && r.MonthsCovered < 3 {
		add(AlertReserveLow, "reserve", SeverityInfo, "Reserva de emergência baixa",
			fmt.Sprintf("Sua reserva cobre %v meses de custo de vida (%s/mês); o mínimo recomendado é 3.", r.MonthsCovered, FormatBRL(r.AvgMonthlyCostCents)),
			"/metas", in.Today)
	}

	for _, g := range in.Goals {
		if g.MonthlyNeededCents <= 0 || g.Last30DaysCents >= g.MonthlyNeededCents {
			continue
		}
		add(AlertGoalBehind, g.ID, SeverityInfo, "Meta atrasada: "+g.Name,
			fmt.Sprintf("Nos últimos 30 dias você guardou %s para %s; o necessário por mês é %s.", FormatBRL(g.Last30DaysCents), g.Name, FormatBRL(g.MonthlyNeededCents)),
			"/metas", in.Today)
	}

	oldCut := AddDays(in.Today, -7)
	oldCount, oldest := 0, ""
	for _, d := range in.PendingDates {
		if d < oldCut {
			oldCount++
			if oldest == "" || d < oldest {
				oldest = d
			}
		}
	}
	if oldCount > 0 {
		add(AlertPendingOld, "pending", SeverityWarning, "Lançamentos pendentes antigos",
			fmt.Sprintf("Você tem %d lançamento(s) pendente(s) há mais de 7 dias (o mais antigo de %s). Confirme ou exclua.", oldCount, FormatDateBR(oldest)),
			"/extrato?status=PENDING", in.Today)
	}

	if in.Uncategorized >= 5 {
		add(AlertUncategorized, "uncategorized", SeverityInfo, "Lançamentos sem categoria",
			fmt.Sprintf("%d lançamentos deste mês estão sem categoria. Categorize ou crie uma automação.", in.Uncategorized),
			"/extrato", in.Today)
	}

	staleCut := AddDays(in.Today, -45)
	for _, inv := range in.Investments {
		if inv.ValueUpdatedAt == "" || inv.ValueUpdatedAt >= staleCut {
			continue
		}
		add(AlertInvestmentStale, inv.ID, SeverityInfo, "Investimento desatualizado: "+inv.Name,
			fmt.Sprintf("O valor de %s não é atualizado desde %s.", inv.Name, FormatDateBR(inv.ValueUpdatedAt)),
			"/investimentos", in.Today)
	}

	SortAlerts(out)
	return out
}

// SortAlerts ordena por severidade (DANGER, WARNING, INFO) e depois por data e id.
func SortAlerts(alerts []Alert) {
	sort.SliceStable(alerts, func(i, j int) bool {
		a, b := alerts[i], alerts[j]
		if severityRank[a.Severity] != severityRank[b.Severity] {
			return severityRank[a.Severity] < severityRank[b.Severity]
		}
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		return a.ID < b.ID
	})
}

// PruneDismissed remove dispensas com mais de 90 dias.
func PruneDismissed(dismissed map[string]string, today string) map[string]string {
	cut := AddDays(today, -AlertDismissRetentionDays)
	out := make(map[string]string, len(dismissed))
	for id, date := range dismissed {
		if date >= cut {
			out[id] = date
		}
	}
	return out
}
