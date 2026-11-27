package service

import (
	"context"

	"finflow/internal/domain"
)

// ───────────────────────── Patrimônio ─────────────────────────

// netWorthBreakdown é o patrimônio de hoje: contas ativas (contas INVESTMENT com posições valem
// a soma das posições), posições avulsas, menos faturas não pagas e saldo devedor dos empréstimos.
func (s *Services) netWorthBreakdown(ctx context.Context) (domain.NetWorthBreakdown, error) {
	accounts, err := s.ListAccounts(ctx, false)
	if err != nil {
		return domain.NetWorthBreakdown{}, err
	}
	positions, err := s.Store.ListInvestments(ctx, false)
	if err != nil {
		return domain.NetWorthBreakdown{}, err
	}
	openInvoices, err := s.Store.OpenInvoicesCents(ctx)
	if err != nil {
		return domain.NetWorthBreakdown{}, err
	}
	loans, err := s.loanStates(ctx)
	if err != nil {
		return domain.NetWorthBreakdown{}, err
	}
	nwAccounts := make([]domain.NetWorthAccount, 0, len(accounts))
	for _, a := range accounts {
		nwAccounts = append(nwAccounts, domain.NetWorthAccount{ID: a.ID, Type: a.Type, BalanceCents: a.BalanceCents})
	}
	nwPositions := make([]domain.NetWorthPosition, 0, len(positions))
	for _, p := range positions {
		nwPositions = append(nwPositions, domain.NetWorthPosition{AccountID: p.AccountID, ValueCents: p.CurrentCents})
	}
	debts := openInvoices
	for _, l := range loans {
		debts += l.schedule().OutstandingCents()
	}
	return domain.ComposeNetWorth(nwAccounts, nwPositions, debts), nil
}

// DefaultNetWorthMonths é quantos meses a evolução do patrimônio devolve sem `months`.
const DefaultNetWorthMonths = 12

// MaxNetWorthMonths limita a série da evolução do patrimônio.
const MaxNetWorthMonths = 60

// NetWorthHistory devolve o patrimônio no fim de cada um dos últimos `months` meses (o mais
// antigo primeiro, terminando no mês corrente). Caixa = contas não-INVESTMENT; investimentos =
// snapshot mais recente de cada posição até a data (ou saldo contábil de contas INVESTMENT sem
// posição com snapshot); dívidas = faturas não pagas com vencimento até a data + saldo devedor
// dos empréstimos naquela data.
func (s *Services) NetWorthHistory(ctx context.Context, months int) ([]domain.NetWorthPoint, error) {
	if months == 0 {
		months = DefaultNetWorthMonths
	}
	if months < 1 || months > MaxNetWorthMonths {
		return nil, ValidationFailed([]domain.ValidationIssue{{Path: "months", Message: "meses deve ser entre 1 e 60"}})
	}
	accounts, err := s.Store.ListAccounts(ctx, false)
	if err != nil {
		return nil, err
	}
	positions, err := s.Store.ListInvestments(ctx, false)
	if err != nil {
		return nil, err
	}
	snaps, err := s.Store.InvestmentSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	invoices, err := s.Store.InvoiceDebtRows(ctx)
	if err != nil {
		return nil, err
	}
	loans, err := s.loanStates(ctx)
	if err != nil {
		return nil, err
	}

	month, year := domain.CurrentMonthYear()
	out := make([]domain.NetWorthPoint, 0, months)
	for _, p := range domain.LastMonths(year, month, months) {
		_, next := domain.MonthRange(p.Year, p.Month)
		end := domain.AddDays(next, -1)
		sums, err := s.Store.ConfirmedSumsByAccountUntil(ctx, end)
		if err != nil {
			return nil, err
		}
		nwAccounts := make([]domain.NetWorthAccount, 0, len(accounts))
		for _, a := range accounts {
			nwAccounts = append(nwAccounts, domain.NetWorthAccount{ID: a.ID, Type: a.Type, BalanceCents: a.InitialBalanceCents + sums[a.ID]})
		}
		var nwPositions []domain.NetWorthPosition
		for _, pos := range positions {
			if snap, ok := domain.LatestSnapshotAt(snaps[pos.ID], end); ok {
				nwPositions = append(nwPositions, domain.NetWorthPosition{AccountID: pos.AccountID, ValueCents: snap.ValueCents})
			}
		}
		var debts int64
		for _, inv := range invoices {
			if inv.DueDate() <= end && inv.UnpaidAt(end) {
				debts += inv.TotalCents
			}
		}
		for _, l := range loans {
			debts += l.schedule().OutstandingAt(end, l.paidAt(end))
		}
		b := domain.ComposeNetWorth(nwAccounts, nwPositions, debts)
		out = append(out, domain.NetWorthPoint{
			Month:            p.Month,
			Year:             p.Year,
			CashCents:        b.CashCents,
			InvestmentsCents: b.InvestmentsCents,
			DebtsCents:       b.DebtsCents,
			NetWorthCents:    b.NetWorthCents(),
		})
	}
	return out, nil
}
