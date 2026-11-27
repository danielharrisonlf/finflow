package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Settings auxiliares (chaves do v3) ─────────────────────────

const (
	settingDismissedSubscriptions = "dismissedSubscriptions"
	settingDismissedAlerts        = "dismissedAlerts"
)

func (s *Services) readSettingJSON(ctx context.Context, key string, dst any) error {
	values, err := s.Store.SettingValues(ctx)
	if err != nil {
		return err
	}
	if raw, ok := values[key]; ok {
		_ = json.Unmarshal([]byte(raw), dst) // valor ilegível = vazio
	}
	return nil
}

func (s *Services) writeSettingJSON(ctx context.Context, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.Store.SaveSettingValues(ctx, map[string]string{key: string(b)})
}

// ───────────────────────── Ritmo e "quanto pode gastar hoje" (§4) ─────────────────────────

// addOverviewV3 completa a visão geral com a série acumulada do ritmo, o gasto projetado do mês
// e o "quanto pode gastar hoje". rows traz o gasto do mês anterior e do mês consultado.
func (s *Services) addOverviewV3(ctx context.Context, out *domain.InsightsOverview, rows []store.SpendingRow, settings domain.Settings, paceDay int, totalOut int64) error {
	key := domain.MonthKeyOf(out.Year, out.Month)
	prevY, prevM := domain.AddMonths(out.Year, out.Month, -1)
	prevKey := domain.MonthKeyOf(prevY, prevM)
	cur, prev := map[int]int64{}, map[int]int64{}
	for _, r := range rows {
		switch domain.MonthKey(r.Date) {
		case key:
			cur[dayOf(r.Date)] += r.TotalCents
		case prevKey:
			prev[dayOf(r.Date)] += r.TotalCents
		}
	}
	dim := domain.DaysInMonth(out.Year, out.Month)
	out.Pace.Series = domain.BuildPaceSeries(cur, prev, dim, domain.DaysInMonth(prevY, prevM), paceDay)
	isCurrent := key == domain.MonthKey(domain.TodayString())
	spent := totalOut
	if isCurrent {
		spent = out.Pace.CurrentCents
	}
	out.Pace.ProjectedMonthCents = domain.ProjectMonthSpend(spent, paceDay, dim, isCurrent)

	today, err := s.spendToday(ctx, settings)
	if err != nil {
		return err
	}
	out.SpendToday = today
	return nil
}

// spendToday: com limite mensal, o quanto ainda dá por dia no mês corrente; sem limite, o menor
// saldo projetado até o fim do mês dividido pelos dias restantes (incluindo hoje).
func (s *Services) spendToday(ctx context.Context, settings domain.Settings) (domain.SpendToday, error) {
	today := domain.TodayString()
	t := domain.FromDateString(today)
	y, m := t.Year(), int(t.Month())
	daysLeft := domain.DaysInMonth(y, m) - t.Day() + 1
	if settings.MonthlyLimitCents != nil {
		start, end := domain.MonthRange(y, m)
		rows, err := s.Store.SpendingRows(ctx, start, end)
		if err != nil {
			return domain.SpendToday{}, err
		}
		var spent int64
		for _, r := range rows {
			spent += r.TotalCents
		}
		view := domain.ComputeMonthlyLimit(settings.MonthlyLimitCents, spent, daysLeft)
		cents := int64(0)
		if view.DailyAllowanceCents != nil {
			cents = *view.DailyAllowanceCents
		}
		return domain.SpendToday{Cents: cents, Basis: domain.SpendTodayLimit}, nil
	}
	p, err := s.simulateProjection(ctx)
	if err != nil {
		return domain.SpendToday{}, err
	}
	window := p.Days[:min(daysLeft, len(p.Days))]
	low := domain.LowestBalance(window)
	return domain.SpendToday{Cents: domain.SpendTodayFromBalance(low.BalanceCents, daysLeft), Basis: domain.SpendTodayBalance}, nil
}

// ───────────────────────── Assinaturas (§2) ─────────────────────────

var subscriptionKeyRe = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

func validSubscriptionKey(key string) bool {
	return len(key) <= 200 && subscriptionKeyRe.MatchString(key)
}

func (s *Services) dismissedSubscriptions(ctx context.Context) (map[string]bool, error) {
	var keys []string
	if err := s.readSettingJSON(ctx, settingDismissedSubscriptions, &keys); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}
	return out, nil
}

// detectSubscriptions roda a heurística sobre os últimos 13 meses e liga cada assinatura à
// recorrência que já a acompanha (por recurring_id ou descrição normalizada). includeDismissed
// mantém as ignoradas pelo usuário.
func (s *Services) detectSubscriptions(ctx context.Context, includeDismissed bool) ([]domain.DetectedSubscription, error) {
	today := domain.TodayString()
	txs, err := s.Store.SubscriptionCandidates(ctx, domain.AddMonthsToDateString(today, -domain.SubscriptionLookbackMonths))
	if err != nil {
		return nil, err
	}
	items := domain.DetectSubscriptions(txs, today)
	recurrings, err := s.Store.ListRecurring(ctx)
	if err != nil {
		return nil, err
	}
	byKey := map[string]string{}
	for _, r := range recurrings {
		if r.Kind == domain.CategoryExpense && r.Active {
			if k := domain.SubscriptionKey(r.Description); k != "" {
				if _, ok := byKey[k]; !ok {
					byKey[k] = r.ID
				}
			}
		}
	}
	dismissed := map[string]bool{}
	if !includeDismissed {
		if dismissed, err = s.dismissedSubscriptions(ctx); err != nil {
			return nil, err
		}
	}
	out := make([]domain.DetectedSubscription, 0, len(items))
	for _, it := range items {
		if dismissed[it.Key] {
			continue
		}
		if it.RecurringID == nil {
			if id, ok := byKey[it.Key]; ok {
				it.RecurringID = &id
			}
		}
		out = append(out, it)
	}
	return out, nil
}

// Subscriptions lista as assinaturas detectadas no histórico e o total mensal normalizado.
func (s *Services) Subscriptions(ctx context.Context) (domain.SubscriptionsView, error) {
	items, err := s.detectSubscriptions(ctx, false)
	if err != nil {
		return domain.SubscriptionsView{}, err
	}
	return domain.SubscriptionsView{Items: items, MonthlyTotalCents: domain.SubscriptionsMonthlyTotal(items)}, nil
}

// TrackSubscription cria uma conta fixa (EXPENSE, isSubscription) a partir da assinatura detectada:
// dia do último lançamento, a partir do dia seguinte a ele. Cadências semanal e anual viram o valor
// mensal equivalente (a recorrência do FinFlow é mensal).
func (s *Services) TrackSubscription(ctx context.Context, key string) (domain.Recurring, error) {
	if !validSubscriptionKey(key) {
		return domain.Recurring{}, BadRequest("chave de assinatura inválida")
	}
	items, err := s.detectSubscriptions(ctx, false)
	if err != nil {
		return domain.Recurring{}, err
	}
	var sub *domain.DetectedSubscription
	for i := range items {
		if items[i].Key == key {
			sub = &items[i]
			break
		}
	}
	if sub == nil {
		return domain.Recurring{}, NotFound("assinatura não encontrada")
	}
	if sub.RecurringID != nil {
		return domain.Recurring{}, Conflict("esta assinatura já é acompanhada como conta fixa")
	}
	accountID := sub.AccountID
	if accountID != nil {
		if acc, err := s.Store.FindAccountByID(ctx, *accountID); err != nil {
			return domain.Recurring{}, err
		} else if acc == nil || acc.Archived {
			accountID = nil
		}
	}
	categoryID := sub.CategoryID
	if categoryID != nil {
		if cat, err := s.Store.FindCategoryByID(ctx, *categoryID); err != nil {
			return domain.Recurring{}, err
		} else if cat == nil || cat.Kind != domain.CategoryExpense {
			categoryID = nil
		}
	}
	description := sub.Description
	if r := []rune(description); len(r) > 120 {
		description = string(r[:120])
	}
	amount := sub.LastCents
	if sub.Cadence != domain.CadenceMonthly {
		amount = domain.MonthlyEquivalentCents(sub.Cadence, sub.LastCents)
	}
	start := domain.AddDays(sub.LastDate, 1)
	yes := true
	return s.CreateRecurring(ctx, domain.CreateRecurringInput{
		Description:    description,
		Kind:           domain.CategoryExpense,
		AmountCents:    amount,
		DayOfMonth:     dayOf(sub.LastDate),
		AccountID:      accountID,
		CategoryID:     categoryID,
		StartDate:      &start,
		IsSubscription: &yes,
	})
}

// DismissSubscription esconde a assinatura da lista (a chave fica guardada em settings).
func (s *Services) DismissSubscription(ctx context.Context, key string) error {
	if !validSubscriptionKey(key) {
		return BadRequest("chave de assinatura inválida")
	}
	var keys []string
	if err := s.readSettingJSON(ctx, settingDismissedSubscriptions, &keys); err != nil {
		return err
	}
	for _, k := range keys {
		if k == key {
			return nil
		}
	}
	return s.writeSettingJSON(ctx, settingDismissedSubscriptions, append(keys, key))
}

// ───────────────────────── Fluxo de caixa (§5) ─────────────────────────

// InsightsCashflow resume os últimos `months` meses (terminando no mês corrente ou em month/year)
// por regime de caixa (cartão no vencimento da fatura) ou competência (data da compra).
func (s *Services) InsightsCashflow(ctx context.Context, q domain.CashflowQuery) (domain.CashflowView, error) {
	if issues := domain.ValidateCashflowQuery(q); len(issues) > 0 {
		return domain.CashflowView{}, ValidationFailed(issues)
	}
	month, year, err := resolvePeriod(domain.PeriodQuery{Month: q.Month, Year: q.Year})
	if err != nil {
		return domain.CashflowView{}, err
	}
	n := q.Months
	if n == 0 {
		n = domain.DefaultCashflowMonths
	}
	basis := q.Basis
	if basis == "" {
		basis = domain.CashflowBasisCash
	}
	// n+1 meses: o primeiro só serve de base para a variação do resultado.
	ym := make([]domain.InvoicePeriod, 0, n+1)
	for i := n; i >= 0; i-- {
		y, m := domain.AddMonths(year, month, -i)
		ym = append(ym, domain.InvoicePeriod{Year: y, Month: m})
	}
	// Compras feitas até 2 meses antes podem vencer dentro da janela (regime de caixa).
	fromY, fromM := domain.AddMonths(ym[0].Year, ym[0].Month, -2)
	_, end := domain.MonthRange(year, month)
	txs, err := s.Store.CashflowTxs(ctx, domain.MonthStart(fromY, fromM), end)
	if err != nil {
		return domain.CashflowView{}, err
	}
	return domain.BuildCashflow(txs, ym, basis), nil
}

// ───────────────────────── Composição da fatura (§6) ─────────────────────────

func (s *Services) subscriptionKeys(ctx context.Context) (map[string]bool, error) {
	items, err := s.detectSubscriptions(ctx, false)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, it := range items {
		keys[it.Key] = true
	}
	recurrings, err := s.Store.ListRecurring(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range recurrings {
		if r.IsSubscription {
			if k := domain.SubscriptionKey(r.Description); k != "" {
				keys[k] = true
			}
		}
	}
	return keys, nil
}

func (s *Services) InvoiceBreakdown(ctx context.Context, cardID, invoiceID string) (domain.InvoiceBreakdown, error) {
	inv, err := s.Store.FindInvoiceByID(ctx, invoiceID)
	if err != nil {
		return domain.InvoiceBreakdown{}, err
	}
	if inv == nil || inv.CardID != cardID {
		return domain.InvoiceBreakdown{}, NotFound("fatura não encontrada para este cartão")
	}
	lines, paid, err := s.Store.InvoiceLines(ctx, invoiceID)
	if err != nil {
		return domain.InvoiceBreakdown{}, err
	}
	keys, err := s.subscriptionKeys(ctx)
	if err != nil {
		return domain.InvoiceBreakdown{}, err
	}
	return domain.BuildInvoiceBreakdown(lines, paid, keys), nil
}

// ───────────────────────── Transferências inteligentes (§7) ─────────────────────────

// TransferCandidateLookbackDays limita a busca de pares aos lançamentos recentes.
const TransferCandidateLookbackDays = 180

func (s *Services) TransferCandidates(ctx context.Context) ([]domain.TransferCandidate, error) {
	since := domain.AddDays(domain.TodayString(), -TransferCandidateLookbackDays)
	expenses, incomes, err := s.Store.TransferCandidateTxs(ctx, since)
	if err != nil {
		return nil, err
	}
	return domain.PairTransferCandidates(expenses, incomes), nil
}

// MergeTransfer converte uma despesa e uma receita de contas diferentes, com o mesmo valor, em
// uma transferência (mesmo grupo, categoria nula, descrições mantidas).
func (s *Services) MergeTransfer(ctx context.Context, in domain.MergeTransferInput) (domain.TransferResult, error) {
	if issues := domain.ValidateMergeTransfer(in); len(issues) > 0 {
		return domain.TransferResult{}, ValidationFailed(issues)
	}
	exp, err := s.Store.FindTransactionByID(ctx, in.ExpenseID)
	if err != nil {
		return domain.TransferResult{}, err
	}
	inc, err := s.Store.FindTransactionByID(ctx, in.IncomeID)
	if err != nil {
		return domain.TransferResult{}, err
	}
	if exp == nil || inc == nil {
		return domain.TransferResult{}, NotFound("transação não encontrada")
	}
	if exp.Type != domain.TxExpense || inc.Type != domain.TxIncome {
		return domain.TransferResult{}, BadRequest("informe uma despesa (expenseId) e uma receita (incomeId)")
	}
	if exp.AccountID == nil || inc.AccountID == nil || *exp.AccountID == *inc.AccountID {
		return domain.TransferResult{}, BadRequest("as duas pernas devem ser de contas diferentes")
	}
	if absCents(exp.AmountCents) != absCents(inc.AmountCents) {
		return domain.TransferResult{}, BadRequest("as duas pernas devem ter o mesmo valor")
	}
	if exp.TransferGroupID != nil || inc.TransferGroupID != nil || exp.RecurringID != nil || inc.RecurringID != nil {
		return domain.TransferResult{}, Conflict("lançamentos ligados a recorrência ou transferência não podem ser mesclados")
	}
	group, err := s.Store.MergeTransferLegs(ctx, in.ExpenseID, in.IncomeID)
	if errors.Is(err, store.ErrConflict) {
		return domain.TransferResult{}, Conflict("lançamentos ligados a recorrência, empréstimo ou transferência não podem ser mesclados")
	}
	if err != nil {
		return domain.TransferResult{}, err
	}
	return domain.TransferResult{TransferGroupID: group}, nil
}

func absCents(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ───────────────────────── "Posso comprar?" (§8) ─────────────────────────

// CanAfford roda a projeção com a compra injetada: sem cartão, uma saída única na data (padrão
// hoje; parcelas são ignoradas); com cartão, as parcelas seguem o ciclo da fatura e saem no
// vencimento. A compra entra além do gasto diário estimado (visão conservadora).
func (s *Services) CanAfford(ctx context.Context, in domain.CanAffordInput) (domain.CanAffordResult, error) {
	if issues := domain.ValidateCanAfford(in); len(issues) > 0 {
		return domain.CanAffordResult{}, ValidationFailed(issues)
	}
	today := domain.TodayString()
	date := today
	if in.Date != nil {
		date = *in.Date
	}
	desc := "Compra simulada"
	if in.Description != nil && strings.TrimSpace(*in.Description) != "" {
		desc = strings.TrimSpace(*in.Description)
	}
	var extra []domain.ProjectionPending
	firstLeg := in.AmountCents
	if in.CardID != nil {
		card, err := s.Store.FindCardByID(ctx, *in.CardID)
		if err != nil {
			return domain.CanAffordResult{}, err
		}
		if card == nil {
			return domain.CanAffordResult{}, NotFound("cartão não encontrado")
		}
		installments := 1
		if in.Installments != nil {
			installments = *in.Installments
		}
		extra, firstLeg = domain.CardPurchaseEvents(desc, in.AmountCents, installments, date, card.ClosingDay, card.DueDay)
	} else {
		// Sem cartão, parcelas viram saídas mensais da conta (boleto/carnê) a partir da data da compra,
		// com o resto da divisão nas primeiras parcelas — o mesmo critério das compras no cartão.
		installments := 1
		if in.Installments != nil && *in.Installments > 1 {
			installments = *in.Installments
		}
		amounts, err := domain.SplitInstallments(in.AmountCents, installments)
		if err != nil {
			return domain.CanAffordResult{}, err
		}
		for i, amt := range amounts {
			legDesc := desc
			if installments > 1 {
				legDesc = fmt.Sprintf("%s (%d/%d)", desc, i+1, installments)
			}
			extra = append(extra, domain.ProjectionPending{
				Description: legDesc,
				Date:        domain.AddMonthsToDateString(date, i),
				AmountCents: -amt,
			})
		}
		firstLeg = amounts[0]
	}

	settings, err := s.GetSettings(ctx)
	if err != nil {
		return domain.CanAffordResult{}, err
	}
	p, err := s.simulateProjection(ctx, extra...)
	if err != nil {
		return domain.CanAffordResult{}, err
	}
	low := domain.LowestBalance(p.Days[:min(domain.ProjectionHorizonDays, len(p.Days))])
	out := domain.CanAffordResult{LowestBalanceAfterCents: low.BalanceCents, LowestDate: low.Date}

	limitExceeded := false
	if settings.MonthlyLimitCents != nil {
		purchaseDate := max(date, today)
		pd := domain.FromDateString(purchaseDate)
		start, end := domain.MonthRange(pd.Year(), int(pd.Month()))
		rows, err := s.Store.SpendingRows(ctx, start, end)
		if err != nil {
			return domain.CanAffordResult{}, err
		}
		spent := firstLeg
		for _, r := range rows {
			spent += r.TotalCents
		}
		limit := *settings.MonthlyLimitCents
		out.Limit = &domain.CanAffordLimit{LimitCents: limit, SpentAfterCents: spent, PercentAfter: domain.PercentOf(spent, limit)}
		limitExceeded = spent > limit
	}
	out.Verdict = domain.CanAffordVerdict(low.BalanceCents, settings.BalanceBands, limitExceeded)

	if low.BalanceCents < 0 {
		months, err := s.reserveMonthsAfter(ctx, in.AmountCents)
		if err != nil {
			return domain.CanAffordResult{}, err
		}
		out.ReserveMonthsAfter = months
	}
	if out.Verdict == domain.VerdictNo {
		base, err := s.simulateProjection(ctx)
		if err != nil {
			return domain.CanAffordResult{}, err
		}
		horizon := base.Days[:min(domain.ProjectionHorizonDays, len(base.Days))]
		out.FitsOn = domain.AffordableDates(horizon, []int64{in.AmountCents})[0]
	}
	out.Message = domain.CanAffordMessage(out, in.AmountCents, settings.BalanceBands)
	return out, nil
}

// reserveMonthsAfter estima quantos meses de custo de vida a reserva cobriria se a compra saísse
// dela: contas SAVINGS/INVESTMENT (ou a meta RESERVE, sem contas de reserva) menos o valor,
// dividido pelo custo médio dos últimos meses. nil sem custo médio conhecido.
func (s *Services) reserveMonthsAfter(ctx context.Context, amount int64) (*float64, error) {
	hasReserve, err := s.Store.HasReserveAccounts(ctx)
	if err != nil {
		return nil, err
	}
	var funds int64
	if hasReserve {
		if funds, err = s.Store.ReserveAccountsBalanceCents(ctx); err != nil {
			return nil, err
		}
	} else {
		goal, err := s.Store.FindActiveReserveGoal(ctx)
		if err != nil {
			return nil, err
		}
		if goal != nil {
			funds = goal.SavedCents
		}
	}
	resolve, err := s.spendTypeResolver(ctx)
	if err != nil {
		return nil, err
	}
	t := domain.FromDateString(domain.TodayString())
	y, m := t.Year(), int(t.Month())
	histY, histM := domain.AddMonths(y, m, -12)
	data, err := s.loadMonthlyData(ctx, domain.MonthStart(histY, histM), domain.MonthStart(y, m), resolve)
	if err != nil {
		return nil, err
	}
	costs := make([]int64, 0, 12)
	for i := 1; i <= 12; i++ {
		py, pm := domain.AddMonths(y, m, -i)
		costs = append(costs, data.flows(domain.MonthKeyOf(py, pm)).TotalOutCents())
	}
	avg := domain.AverageRecentCost(costs)
	if avg <= 0 {
		return nil, nil
	}
	v := domain.RoundPct(float64(max(0, funds-amount)) / float64(avg))
	return &v, nil
}
