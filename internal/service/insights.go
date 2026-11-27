package service

import (
	"context"
	"fmt"
	"sort"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Insights ─────────────────────────

const uncategorizedName = "Sem categoria"

// spendTypeResolver devolve o tipo de gasto efetivo de cada categoria (sem categoria = VARIABLE).
func (s *Services) spendTypeResolver(ctx context.Context) (func(categoryID *string) domain.SpendType, error) {
	cats, err := s.Store.ListCategories(ctx, nil)
	if err != nil {
		return nil, err
	}
	nodes := categoryNodes(cats)
	cache := make(map[string]domain.SpendType, len(cats))
	return func(categoryID *string) domain.SpendType {
		if categoryID == nil {
			return domain.SpendVariable
		}
		if st, ok := cache[*categoryID]; ok {
			return st
		}
		st := domain.SpendVariable
		if eff := domain.EffectiveSpendType(*categoryID, nodes); eff != nil {
			st = *eff
		}
		cache[*categoryID] = st
		return st
	}, nil
}

// spendingByMonth agrega as linhas de gasto por mês ("YYYY-MM") e categoria.
func spendingByMonth(rows []store.SpendingRow, resolve func(*string) domain.SpendType, maxDay func(month string) int) map[string][]domain.CategorySpend {
	type key struct{ month, cat string }
	acc := make(map[key]*domain.CategorySpend)
	var order []key
	for _, r := range rows {
		month := domain.MonthKey(r.Date)
		if maxDay != nil {
			if day := dayOf(r.Date); day > maxDay(month) {
				continue
			}
		}
		k := key{month: month}
		if r.CategoryID != nil {
			k.cat = *r.CategoryID
		}
		cs, ok := acc[k]
		if !ok {
			name := uncategorizedName
			if r.Name != nil && *r.Name != "" {
				name = *r.Name
			}
			cs = &domain.CategorySpend{
				CategoryID: r.CategoryID,
				Name:       name,
				Icon:       r.Icon,
				Color:      r.Color,
				SpendType:  resolve(r.CategoryID),
			}
			acc[k] = cs
			order = append(order, k)
		}
		cs.TotalCents += r.TotalCents
	}
	out := make(map[string][]domain.CategorySpend)
	for _, k := range order {
		out[k.month] = append(out[k.month], *acc[k])
	}
	return out
}

func dayOf(date string) int {
	if len(date) < 10 {
		return 0
	}
	return int(date[8]-'0')*10 + int(date[9]-'0')
}

func sumByMonth(rows []store.DatedAmount) map[string]int64 {
	out := make(map[string]int64)
	for _, r := range rows {
		out[domain.MonthKey(r.Date)] += r.Cents
	}
	return out
}

func totalSpend(items []domain.CategorySpend) int64 {
	var total int64
	for _, it := range items {
		total += it.TotalCents
	}
	return total
}

// monthlyData reúne, para um intervalo de meses, o gasto por categoria e as entradas/economia.
type monthlyData struct {
	spend  map[string][]domain.CategorySpend
	income map[string]int64
	saved  map[string]int64
}

func (m monthlyData) flows(month string) domain.MonthFlows {
	fixed, variable := domain.SplitFlows(m.spend[month])
	return domain.MonthFlows{
		IncomeCents:   m.income[month],
		FixedCents:    fixed,
		VariableCents: variable,
		SavedCents:    m.saved[month],
	}
}

// loadMonthlyData carrega [start, end) agregado por mês.
func (s *Services) loadMonthlyData(ctx context.Context, start, end string, resolve func(*string) domain.SpendType) (monthlyData, error) {
	rows, err := s.Store.SpendingRows(ctx, start, end)
	if err != nil {
		return monthlyData{}, err
	}
	income, err := s.Store.IncomeByDate(ctx, start, end)
	if err != nil {
		return monthlyData{}, err
	}
	transfers, err := s.Store.SavingsTransfersByDate(ctx, start, end)
	if err != nil {
		return monthlyData{}, err
	}
	// Economia = transferências de contas líquidas para contas de reserva. Quem não tem conta de
	// reserva registra o que guarda pelos aportes em metas; somar os dois contaria o mesmo dinheiro duas vezes.
	saved := sumByMonth(transfers)
	hasReserve, err := s.Store.HasReserveAccounts(ctx)
	if err != nil {
		return monthlyData{}, err
	}
	if !hasReserve {
		contribs, err := s.Store.GoalContributionsByDate(ctx, start, end)
		if err != nil {
			return monthlyData{}, err
		}
		saved = sumByMonth(contribs)
	}
	return monthlyData{
		spend:  spendingByMonth(rows, resolve, nil),
		income: sumByMonth(income),
		saved:  saved,
	}, nil
}

// InsightsOverview monta a visão geral do mês (ritmo, limite, performance, tipos de gasto,
// patrimônio, reserva e onde cortar).
func (s *Services) InsightsOverview(ctx context.Context, q domain.PeriodQuery) (domain.InsightsOverview, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return domain.InsightsOverview{}, err
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return domain.InsightsOverview{}, err
	}
	resolve, err := s.spendTypeResolver(ctx)
	if err != nil {
		return domain.InsightsOverview{}, err
	}

	// 12 meses de histórico bastam para a reserva (3 meses com gasto) e a média de cortes.
	histY, histM := domain.AddMonths(year, month, -12)
	start := domain.MonthStart(histY, histM)
	_, end := domain.MonthRange(year, month)
	data, err := s.loadMonthlyData(ctx, start, end, resolve)
	if err != nil {
		return domain.InsightsOverview{}, err
	}
	key := domain.MonthKeyOf(year, month)
	prevY, prevM := domain.AddMonths(year, month, -1)
	prevKey := domain.MonthKeyOf(prevY, prevM)

	// Ritmo: do dia 1 até hoje (mês corrente), até o fim (mês passado) ou nada (mês futuro).
	today := domain.FromDateString(domain.TodayString())
	curKey := domain.MonthKeyOf(today.Year(), int(today.Month()))
	dim := domain.DaysInMonth(year, month)
	paceDay, daysLeft := dim, 0
	switch {
	case key == curKey:
		paceDay = today.Day()
		daysLeft = dim - today.Day() + 1
	case key > curKey:
		paceDay = 0
	}
	prevLimit := min(paceDay, domain.DaysInMonth(prevY, prevM))
	rows, err := s.Store.SpendingRows(ctx, domain.MonthStart(prevY, prevM), end)
	if err != nil {
		return domain.InsightsOverview{}, err
	}
	paced := spendingByMonth(rows, resolve, func(m string) int {
		if m == key {
			return paceDay
		}
		return prevLimit
	})

	cur := data.flows(key)
	totalOut := totalSpend(data.spend[key])

	totalBalance, err := s.Store.TotalBalanceCents(ctx)
	if err != nil {
		return domain.InsightsOverview{}, err
	}
	openInvoices, err := s.Store.OpenInvoicesCents(ctx)
	if err != nil {
		return domain.InsightsOverview{}, err
	}

	costs := make([]int64, 0, 12)
	for i := 1; i <= 12; i++ {
		y, m := domain.AddMonths(year, month, -i)
		costs = append(costs, data.flows(domain.MonthKeyOf(y, m)).TotalOutCents())
	}
	var reserveGoal *domain.ReserveGoalInfo
	goal, err := s.Store.FindActiveReserveGoal(ctx)
	if err != nil {
		return domain.InsightsOverview{}, err
	}
	if goal != nil {
		reserveGoal = &domain.ReserveGoalInfo{ID: goal.ID, TargetCents: goal.TargetCents, SavedCents: goal.SavedCents}
	}

	history := make([][]domain.CategorySpend, 0, 3)
	for i := 1; i <= 3; i++ {
		y, m := domain.AddMonths(year, month, -i)
		history = append(history, data.spend[domain.MonthKeyOf(y, m)])
	}

	return domain.InsightsOverview{
		Month:          month,
		Year:           year,
		Pace:           domain.ComputePace(totalSpend(paced[key]), totalSpend(paced[prevKey]), paceDay),
		MonthlyLimit:   domain.ComputeMonthlyLimit(settings.MonthlyLimitCents, totalOut, daysLeft),
		Performance:    domain.ComputePerformance(cur, data.flows(prevKey).PerformanceCents()),
		SpendTypes:     domain.GroupBySpendType(data.spend[key]),
		Plan:           settings.SpendPlan,
		NetWorthCents:  totalBalance - openInvoices,
		Reserve:        domain.ComputeReserve(reserveGoal, domain.AverageRecentCost(costs), settings),
		CutSuggestions: domain.ComputeCutSuggestions(data.spend[key], history),
	}, nil
}

// InsightsYear resume os 12 meses do ano pelo método da planilha diária.
func (s *Services) InsightsYear(ctx context.Context, year *int) (domain.YearOverview, error) {
	if issues := domain.ValidatePeriodQuery(domain.PeriodQuery{Year: year}); len(issues) > 0 {
		return domain.YearOverview{}, ValidationFailed(issues)
	}
	y := domain.Now().Year()
	if year != nil {
		y = *year
	}
	resolve, err := s.spendTypeResolver(ctx)
	if err != nil {
		return domain.YearOverview{}, err
	}
	start, end := domain.YearRange(y)
	data, err := s.loadMonthlyData(ctx, start, end, resolve)
	if err != nil {
		return domain.YearOverview{}, err
	}
	balance, err := s.Store.BalanceBefore(ctx, start)
	if err != nil {
		return domain.YearOverview{}, err
	}
	flowRows, err := s.Store.ConfirmedFlowsByDate(ctx, start, end)
	if err != nil {
		return domain.YearOverview{}, err
	}
	flows := sumByMonth(flowRows)

	out := domain.YearOverview{Year: y, Months: make([]domain.YearMonthOverview, 0, 12)}
	for m := 1; m <= 12; m++ {
		key := domain.MonthKeyOf(y, m)
		f := data.flows(key)
		balance += flows[key]
		out.Months = append(out.Months, domain.YearMonthOverview{
			Month:            m,
			IncomeCents:      f.IncomeCents,
			FixedCents:       f.FixedCents,
			VariableCents:    f.VariableCents,
			TotalOutCents:    f.TotalOutCents(),
			PerformanceCents: f.PerformanceCents(),
			SavedCents:       f.SavedCents,
			SavingsRatePct:   domain.SavingsRate(f.SavedCents, f.IncomeCents),
			EndBalanceCents:  balance,
		})
		out.Totals.IncomeCents += f.IncomeCents
		out.Totals.FixedCents += f.FixedCents
		out.Totals.VariableCents += f.VariableCents
		out.Totals.TotalOutCents += f.TotalOutCents()
		out.Totals.PerformanceCents += f.PerformanceCents()
		out.Totals.SavedCents += f.SavedCents
	}
	out.Totals.SavingsRatePct = domain.SavingsRate(out.Totals.SavedCents, out.Totals.IncomeCents)
	return out, nil
}

// installmentLegsAround carrega as parcelas cujas faturas vencem perto do intervalo de meses.
func (s *Services) installmentLegsAround(ctx context.Context, year, month, monthsBefore, monthsAfter int) ([]store.InstallmentLeg, error) {
	from := domain.AddMonthsToPeriod(domain.InvoicePeriod{Year: year, Month: month}, -monthsBefore)
	to := domain.AddMonthsToPeriod(domain.InvoicePeriod{Year: year, Month: month}, monthsAfter)
	return s.Store.InstallmentLegs(ctx, from, to)
}

// InsightsCommitments lista contas fixas, receitas recorrentes e parcelas do mês e resume
// os 6 meses a partir dele.
func (s *Services) InsightsCommitments(ctx context.Context, q domain.PeriodQuery) (domain.Commitments, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return domain.Commitments{}, err
	}
	recurrings, err := s.Store.ListRecurring(ctx)
	if err != nil {
		return domain.Commitments{}, err
	}
	realized, err := s.Store.RealizedRecurringMonths(ctx)
	if err != nil {
		return domain.Commitments{}, err
	}
	// A fatura do mês M-1 pode vencer em M (vencimento <= fechamento).
	legs, err := s.installmentLegsAround(ctx, year, month, 1, 6)
	if err != nil {
		return domain.Commitments{}, err
	}
	balance, err := s.Store.LiquidBalanceCents(ctx)
	if err != nil {
		return domain.Commitments{}, err
	}

	out := domain.Commitments{Months: make([]domain.CommitmentMonth, 0, 6), Items: []domain.CommitmentItem{}}
	monthIdx := map[string]int{}
	for i := 0; i < 6; i++ {
		y, m := domain.AddMonths(year, month, i)
		monthIdx[domain.MonthKeyOf(y, m)] = i
		cm := domain.CommitmentMonth{Month: m, Year: y}
		for _, r := range recurrings {
			if !r.Active {
				continue
			}
			if _, ok := recurringSchedule(r).OccurrenceIn(y, m); !ok {
				continue
			}
			if r.Kind == domain.CategoryIncome {
				cm.RecurringIncomeCents += r.AmountCents
			} else {
				cm.RecurringExpenseCents += r.AmountCents
			}
		}
		out.Months = append(out.Months, cm)
	}
	key := domain.MonthKeyOf(year, month)
	for _, l := range legs {
		due := l.DueDate()
		if i, ok := monthIdx[domain.MonthKey(due)]; ok {
			out.Months[i].InstallmentsCents += l.AmountCents
		}
		if domain.MonthKey(due) != key {
			continue
		}
		num, tot, card := l.InstallmentNumber, l.InstallmentTotal, l.CardName
		out.Items = append(out.Items, domain.CommitmentItem{
			Kind:              "INSTALLMENT",
			Direction:         "PAY",
			Description:       l.Description,
			DueDate:           due,
			AmountCents:       l.AmountCents,
			Realized:          l.InvoicePaid,
			InstallmentNumber: &num,
			InstallmentTotal:  &tot,
			CardName:          &card,
		})
	}
	for _, r := range recurrings {
		if !r.Active {
			continue
		}
		due, ok := recurringSchedule(r).OccurrenceIn(year, month)
		if !ok {
			continue
		}
		direction := "PAY"
		if r.Kind == domain.CategoryIncome {
			direction = "RECEIVE"
		}
		id := r.ID
		out.Items = append(out.Items, domain.CommitmentItem{
			Kind:        "RECURRING",
			Direction:   direction,
			Description: r.Description,
			DueDate:     due,
			AmountCents: r.AmountCents,
			RecurringID: &id,
			Realized:    realized[r.ID][key],
		})
	}
	sort.SliceStable(out.Items, func(i, j int) bool {
		if out.Items[i].DueDate != out.Items[j].DueDate {
			return out.Items[i].DueDate < out.Items[j].DueDate
		}
		return out.Items[i].Description < out.Items[j].Description
	})

	var pendingPay, pendingReceive int64
	for _, it := range out.Items {
		if it.Direction == "PAY" {
			out.Totals.PayCents += it.AmountCents
			if !it.Realized {
				pendingPay += it.AmountCents
			}
		} else {
			out.Totals.ReceiveCents += it.AmountCents
			if !it.Realized {
				pendingReceive += it.AmountCents
			}
		}
	}
	out.Totals.BalanceAfterCents = balance + pendingReceive - pendingPay
	return out, nil
}

// simulateProjection monta as entradas e roda a simulação diária completa.
func (s *Services) simulateProjection(ctx context.Context) (domain.Projection, error) {
	today := domain.TodayString()
	t := domain.FromDateString(today)
	ty, tm := t.Year(), int(t.Month())

	settings, err := s.GetSettings(ctx)
	if err != nil {
		return domain.Projection{}, err
	}
	// A projeção usa só o dinheiro líquido: poupança e investimentos são o cofre, não o caixa.
	startBalance, err := s.Store.LiquidBalanceCents(ctx)
	if err != nil {
		return domain.Projection{}, err
	}
	recurrings, err := s.Store.ListRecurring(ctx)
	if err != nil {
		return domain.Projection{}, err
	}
	realized, err := s.Store.RealizedRecurringMonths(ctx)
	if err != nil {
		return domain.Projection{}, err
	}
	pending, err := s.Store.PendingAccountTransactions(ctx)
	if err != nil {
		return domain.Projection{}, err
	}
	invoices, err := s.Store.UnpaidInvoicesWithTotals(ctx)
	if err != nil {
		return domain.Projection{}, err
	}
	legs, err := s.installmentLegsAround(ctx, ty, tm, 12, domain.ProjectionMonths+1)
	if err != nil {
		return domain.Projection{}, err
	}
	resolve, err := s.spendTypeResolver(ctx)
	if err != nil {
		return domain.Projection{}, err
	}
	monthStart, monthEnd := domain.MonthRange(ty, tm)
	spendRows, err := s.Store.SpendingRows(ctx, monthStart, monthEnd)
	if err != nil {
		return domain.Projection{}, err
	}

	in := domain.ProjectionInput{
		Today:               today,
		StartBalanceCents:   startBalance,
		MonthlyLimitCents:   settings.MonthlyLimitCents,
		Bands:               settings.BalanceBands,
		InstallmentsByMonth: make(map[string]int64),
	}
	for _, r := range recurrings {
		if !r.Active {
			continue
		}
		in.Recurring = append(in.Recurring, domain.ProjectionRecurring{
			Description: r.Description,
			Kind:        r.Kind,
			AmountCents: r.AmountCents,
			Schedule:    recurringSchedule(r),
			Realized:    realized[r.ID],
		})
	}
	for _, p := range pending {
		in.Pending = append(in.Pending, domain.ProjectionPending{Description: p.Description, Date: p.Date, AmountCents: p.AmountCents})
	}
	instByInvoice := make(map[string]int64)
	for _, l := range legs {
		in.InstallmentsByMonth[domain.MonthKey(l.DueDate())] += l.AmountCents
		instByInvoice[l.InvoiceID] += l.AmountCents
	}
	for _, inv := range invoices {
		if inv.TotalCents <= 0 {
			continue
		}
		period := domain.InvoicePeriod{Year: inv.Year, Month: inv.Month}
		in.Invoices = append(in.Invoices, domain.ProjectionInvoice{
			Description:       fmt.Sprintf("Fatura %s %02d/%d", inv.CardName, inv.Month, inv.Year),
			DueDate:           domain.InvoiceDueDate(period, inv.ClosingDay, inv.DueDay),
			TotalCents:        inv.TotalCents,
			InstallmentsCents: instByInvoice[inv.ID],
		})
	}
	sort.SliceStable(in.Invoices, func(i, j int) bool { return in.Invoices[i].DueDate < in.Invoices[j].DueDate })
	for _, items := range spendingByMonth(spendRows, resolve, nil) {
		for _, it := range items {
			if it.SpendType == domain.SpendVariable {
				in.CurrentMonthVariableCents += it.TotalCents
			}
		}
	}
	return domain.SimulateProjection(in), nil
}

// DefaultProjectionDays é a quantidade de dias devolvida quando `days` não é informado.
const DefaultProjectionDays = 90

// InsightsProjection devolve a projeção diária (days limita só a lista de dias: 1–365).
func (s *Services) InsightsProjection(ctx context.Context, days int) (domain.Projection, error) {
	if days == 0 {
		days = DefaultProjectionDays
	}
	if days < 1 || days > domain.ProjectionHorizonDays {
		return domain.Projection{}, ValidationFailed([]domain.ValidationIssue{{Path: "days", Message: "dias deve ser entre 1 e 365"}})
	}
	p, err := s.simulateProjection(ctx)
	if err != nil {
		return domain.Projection{}, err
	}
	if len(p.Days) > days {
		p.Days = p.Days[:days]
	}
	return p, nil
}
