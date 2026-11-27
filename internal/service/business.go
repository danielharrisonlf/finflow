package service

import (
	"context"

	"finflow/internal/domain"
)

// ───────────────────────── Empresa: métricas e DRE (v3 fase 2 §5) ─────────────────────────

// BusinessMetrics calcula MRR, ARR, churn, queima e runway do mês (padrão: mês corrente).
func (s *Services) BusinessMetrics(ctx context.Context, q domain.PeriodQuery) (domain.BusinessMetrics, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return domain.BusinessMetrics{}, err
	}
	rows, err := s.Store.ListRecurring(ctx)
	if err != nil {
		return domain.BusinessMetrics{}, err
	}
	recs := make([]domain.RecurringIncome, 0, len(rows))
	for _, r := range rows {
		if r.Kind != domain.CategoryIncome {
			continue
		}
		recs = append(recs, domain.RecurringIncome{
			AmountCents:   r.AmountCents,
			StartDate:     r.StartDate,
			EndDate:       r.EndDate,
			Active:        r.Active,
			DeactivatedAt: r.DeactivatedAt,
		})
	}
	prev := make([]domain.MonthCashFlow, 0, 3)
	for i := 3; i >= 1; i-- {
		p := domain.AddMonthsToPeriod(domain.InvoicePeriod{Year: year, Month: month}, -i)
		in, err := s.Store.MonthIncomeCents(ctx, p.Year, p.Month)
		if err != nil {
			return domain.BusinessMetrics{}, err
		}
		out, err := s.Store.MonthExpenseCents(ctx, p.Year, p.Month)
		if err != nil {
			return domain.BusinessMetrics{}, err
		}
		prev = append(prev, domain.MonthCashFlow{InCents: in, OutCents: out})
	}
	cash, err := s.Store.TotalBalanceCents(ctx)
	if err != nil {
		return domain.BusinessMetrics{}, err
	}
	return domain.ComputeBusinessMetrics(year, month, recs, prev, cash), nil
}

// BusinessDRE monta o DRE anual (padrão: ano corrente) pelo grupo do DRE das categorias:
// lançamentos confirmados pela data; sem categoria, receitas contam como REVENUE e despesas como OPEX.
func (s *Services) BusinessDRE(ctx context.Context, year *int) (domain.DRE, error) {
	if issues := domain.ValidatePeriodQuery(domain.PeriodQuery{Year: year}); len(issues) > 0 {
		return domain.DRE{}, ValidationFailed(issues)
	}
	y := domain.Now().Year()
	if year != nil {
		y = *year
	}
	cats, err := s.Store.ListCategories(ctx, nil)
	if err != nil {
		return domain.DRE{}, err
	}
	nodes := dreNodes(cats)
	start, end := domain.YearRange(y)
	txs, err := s.Store.DRETransactionsBetween(ctx, start, end)
	if err != nil {
		return domain.DRE{}, err
	}
	entries := make([]domain.DREEntry, 0, len(txs))
	for _, t := range txs {
		kind, _ := domain.CategoryKindForTransaction(t.Type)
		group := domain.DefaultDREGroup(kind)
		if t.CategoryID != nil {
			if _, ok := nodes[*t.CategoryID]; ok {
				group = domain.EffectiveDREGroup(*t.CategoryID, nodes)
			}
		}
		entries = append(entries, domain.DREEntry{
			Month:       int(domain.FromDateString(t.Date).Month()),
			Group:       group,
			AmountCents: t.AmountCents,
		})
	}
	return domain.BuildDRE(y, entries), nil
}

// SpaceTotals resume o espaço para a visão unificada: saldo das contas, patrimônio e o
// resultado do mês corrente (entradas e saídas confirmadas).
func (s *Services) SpaceTotals(ctx context.Context) (domain.SpaceTotals, error) {
	balance, err := s.Store.TotalBalanceCents(ctx)
	if err != nil {
		return domain.SpaceTotals{}, err
	}
	nw, err := s.netWorthBreakdown(ctx)
	if err != nil {
		return domain.SpaceTotals{}, err
	}
	month, year := domain.CurrentMonthYear()
	income, err := s.Store.MonthIncomeCents(ctx, year, month)
	if err != nil {
		return domain.SpaceTotals{}, err
	}
	out, err := s.Store.MonthExpenseCents(ctx, year, month)
	if err != nil {
		return domain.SpaceTotals{}, err
	}
	return domain.SpaceTotals{
		BalanceCents:     balance,
		NetWorthCents:    nw.NetWorthCents(),
		MonthIncomeCents: income,
		MonthOutCents:    out,
		MonthResultCents: income - out,
	}, nil
}
