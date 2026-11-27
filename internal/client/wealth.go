package client

import (
	"context"
	"net/http"
	"strconv"

	"finflow/internal/domain"
)

// WealthBackend reúne investimentos, empréstimos e a evolução do patrimônio (v3 §10–§12).
type WealthBackend interface {
	Investments(ctx context.Context, includeArchived bool) ([]domain.Investment, error)
	InvestmentSummary(ctx context.Context) (domain.InvestmentSummary, error)
	Loans(ctx context.Context) ([]domain.Loan, error)
	DebtStrategy(ctx context.Context, extraCents int64) (domain.DebtStrategy, error)
	NetWorthHistory(ctx context.Context, months int) ([]domain.NetWorthPoint, error)
}

// ───────────────────────── Local Backend ─────────────────────────

func (l *LocalBackend) Investments(ctx context.Context, includeArchived bool) ([]domain.Investment, error) {
	return l.Svcs.ListInvestments(ctx, includeArchived)
}
func (l *LocalBackend) InvestmentSummary(ctx context.Context) (domain.InvestmentSummary, error) {
	return l.Svcs.InvestmentSummary(ctx)
}
func (l *LocalBackend) Loans(ctx context.Context) ([]domain.Loan, error) {
	return l.Svcs.ListLoans(ctx)
}
func (l *LocalBackend) DebtStrategy(ctx context.Context, extraCents int64) (domain.DebtStrategy, error) {
	return l.Svcs.DebtStrategy(ctx, extraCents)
}
func (l *LocalBackend) NetWorthHistory(ctx context.Context, months int) ([]domain.NetWorthPoint, error) {
	return l.Svcs.NetWorthHistory(ctx, months)
}

// ───────────────────────── HTTP Backend ─────────────────────────

func (c *HTTPClient) Investments(ctx context.Context, includeArchived bool) ([]domain.Investment, error) {
	p := "/investments"
	if includeArchived {
		p += "?includeArchived=true"
	}
	var out []domain.Investment
	return out, c.do(ctx, http.MethodGet, p, nil, &out)
}
func (c *HTTPClient) InvestmentSummary(ctx context.Context) (domain.InvestmentSummary, error) {
	var out domain.InvestmentSummary
	return out, c.do(ctx, http.MethodGet, "/investments/summary", nil, &out)
}
func (c *HTTPClient) Loans(ctx context.Context) ([]domain.Loan, error) {
	var out []domain.Loan
	return out, c.do(ctx, http.MethodGet, "/loans", nil, &out)
}
func (c *HTTPClient) DebtStrategy(ctx context.Context, extraCents int64) (domain.DebtStrategy, error) {
	var out domain.DebtStrategy
	return out, c.do(ctx, http.MethodGet, "/loans/strategy?extraCents="+strconv.FormatInt(extraCents, 10), nil, &out)
}
func (c *HTTPClient) NetWorthHistory(ctx context.Context, months int) ([]domain.NetWorthPoint, error) {
	p := "/insights/net-worth"
	if months > 0 {
		p += "?months=" + strconv.Itoa(months)
	}
	var out []domain.NetWorthPoint
	return out, c.do(ctx, http.MethodGet, p, nil, &out)
}
