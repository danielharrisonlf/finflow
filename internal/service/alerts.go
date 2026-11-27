package service

import (
	"context"
	"fmt"
	"strings"

	"finflow/internal/domain"
)

// ───────────────────────── Alertas e dicas (§3) ─────────────────────────

// Alerts monta o feed de alertas do mês corrente (sem os dispensados), por severidade e data.
func (s *Services) Alerts(ctx context.Context) ([]domain.Alert, error) {
	in, err := s.alertInput(ctx)
	if err != nil {
		return nil, err
	}
	dismissed, err := s.dismissedAlerts(ctx)
	if err != nil {
		return nil, err
	}
	all := domain.BuildAlerts(in)
	out := make([]domain.Alert, 0, len(all))
	for _, a := range all {
		if _, ok := dismissed[a.ID]; !ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *Services) alertInput(ctx context.Context) (domain.AlertInput, error) {
	today := domain.TodayString()
	t := domain.FromDateString(today)
	y, m := t.Year(), int(t.Month())
	in := domain.AlertInput{Today: today}

	budgets, err := s.ListBudgets(ctx, domain.PeriodQuery{Month: &m, Year: &y})
	if err != nil {
		return in, err
	}
	for _, b := range budgets {
		in.Budgets = append(in.Budgets, domain.AlertBudget{
			CategoryID: b.CategoryID, Name: b.CategoryName, Percent: b.Percent, SpentCents: b.SpentCents, AmountCents: b.AmountCents,
		})
	}

	overview, err := s.InsightsOverview(ctx, domain.PeriodQuery{Month: &m, Year: &y})
	if err != nil {
		return in, err
	}
	in.LimitCents = overview.MonthlyLimit.LimitCents
	in.MonthSpentCents = overview.MonthlyLimit.SpentCents
	in.Pace = overview.Pace
	in.CutSuggestions = overview.CutSuggestions
	reserve := overview.Reserve
	in.Reserve = &reserve

	p, err := s.simulateProjection(ctx)
	if err != nil {
		return in, err
	}
	lowest := p.Lowest
	in.ProjectionLowest = &lowest

	if in.Bills, err = s.billDues(ctx, today); err != nil {
		return in, err
	}
	invoices, err := s.Store.UnpaidInvoicesWithTotals(ctx)
	if err != nil {
		return in, err
	}
	for _, inv := range invoices {
		if inv.TotalCents <= 0 {
			continue
		}
		in.Invoices = append(in.Invoices, domain.AlertDue{
			Ref:         inv.ID,
			Description: fmt.Sprintf("Fatura %s %02d/%d", inv.CardName, inv.Month, inv.Year),
			DueDate:     domain.InvoiceDueDate(domain.InvoicePeriod{Year: inv.Year, Month: inv.Month}, inv.ClosingDay, inv.DueDay),
			AmountCents: inv.TotalCents,
		})
	}
	loans, err := s.ListLoans(ctx)
	if err != nil {
		return in, err
	}
	for _, l := range loans {
		if l.Active && l.NextDueDate != nil {
			in.Loans = append(in.Loans, domain.AlertDue{Ref: l.ID, Description: l.Name, DueDate: *l.NextDueDate, AmountCents: l.InstallmentCents})
		}
	}
	if in.Subscriptions, err = s.detectSubscriptions(ctx, false); err != nil {
		return in, err
	}

	goals, err := s.ListGoals(ctx)
	if err != nil {
		return in, err
	}
	contribs, err := s.Store.GoalContributionsSince(ctx, domain.AddDays(today, -30))
	if err != nil {
		return in, err
	}
	for _, g := range goals {
		if g.MonthlyNeededCents == nil {
			continue
		}
		in.Goals = append(in.Goals, domain.AlertGoal{ID: g.ID, Name: g.Name, MonthlyNeededCents: *g.MonthlyNeededCents, Last30DaysCents: contribs[g.ID]})
	}

	if in.PendingDates, err = s.Store.PendingTransactionDates(ctx); err != nil {
		return in, err
	}
	start, end := domain.MonthRange(y, m)
	if in.Uncategorized, err = s.Store.UncategorizedCount(ctx, start, end); err != nil {
		return in, err
	}
	if in.Investments, err = s.Store.InvestmentValueDates(ctx); err != nil {
		return in, err
	}
	return in, nil
}

// billDues lista as ocorrências não lançadas das contas fixas no mês corrente e no seguinte
// (para vencimentos na virada do mês); o domínio filtra a janela de 3 dias.
func (s *Services) billDues(ctx context.Context, today string) ([]domain.AlertDue, error) {
	recurrings, err := s.Store.ListRecurring(ctx)
	if err != nil {
		return nil, err
	}
	realized, err := s.Store.RealizedRecurringMonths(ctx)
	if err != nil {
		return nil, err
	}
	t := domain.FromDateString(today)
	var out []domain.AlertDue
	for _, r := range recurrings {
		if !r.Active || r.Kind != domain.CategoryExpense {
			continue
		}
		for i := 0; i < 2; i++ {
			y, m := domain.AddMonths(t.Year(), int(t.Month()), i)
			date, ok := recurringSchedule(r).OccurrenceIn(y, m)
			if !ok || realized[r.ID][domain.MonthKeyOf(y, m)] {
				continue
			}
			out = append(out, domain.AlertDue{Ref: r.ID, Description: r.Description, DueDate: date, AmountCents: r.AmountCents})
			break
		}
	}
	return out, nil
}

func (s *Services) dismissedAlerts(ctx context.Context) (map[string]string, error) {
	dismissed := map[string]string{}
	if err := s.readSettingJSON(ctx, settingDismissedAlerts, &dismissed); err != nil {
		return nil, err
	}
	if dismissed == nil {
		dismissed = map[string]string{}
	}
	return dismissed, nil
}

// DismissAlert guarda o id (com a data) em settings; dispensas com mais de 90 dias são limpas.
func (s *Services) DismissAlert(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > domain.MaxAlertIDChars || strings.Count(id, ":") < 2 {
		return BadRequest("id de alerta inválido")
	}
	dismissed, err := s.dismissedAlerts(ctx)
	if err != nil {
		return err
	}
	today := domain.TodayString()
	dismissed = domain.PruneDismissed(dismissed, today)
	dismissed[id] = today
	return s.writeSettingJSON(ctx, settingDismissedAlerts, dismissed)
}
