package client

import (
	"context"
	"net/http"
	"strconv"

	"finflow/internal/domain"
)

// PlanningBackend reúne insights (visão geral, projeção, ano, compromissos) e recorrências.
type PlanningBackend interface {
	InsightsOverview(ctx context.Context, q domain.PeriodQuery) (domain.InsightsOverview, error)
	InsightsProjection(ctx context.Context, days int) (domain.Projection, error)
	InsightsYear(ctx context.Context, year *int) (domain.YearOverview, error)
	InsightsCommitments(ctx context.Context, q domain.PeriodQuery) (domain.Commitments, error)
	Recurring(ctx context.Context) ([]domain.Recurring, error)
	CreateRecurring(ctx context.Context, in domain.CreateRecurringInput) (domain.Recurring, error)
}

// ───────────────────────── Local Backend ─────────────────────────

func (l *LocalBackend) InsightsOverview(ctx context.Context, q domain.PeriodQuery) (domain.InsightsOverview, error) {
	return l.Svcs.InsightsOverview(ctx, q)
}
func (l *LocalBackend) InsightsProjection(ctx context.Context, days int) (domain.Projection, error) {
	return l.Svcs.InsightsProjection(ctx, days)
}
func (l *LocalBackend) InsightsYear(ctx context.Context, year *int) (domain.YearOverview, error) {
	return l.Svcs.InsightsYear(ctx, year)
}
func (l *LocalBackend) InsightsCommitments(ctx context.Context, q domain.PeriodQuery) (domain.Commitments, error) {
	return l.Svcs.InsightsCommitments(ctx, q)
}
func (l *LocalBackend) Recurring(ctx context.Context) ([]domain.Recurring, error) {
	return l.Svcs.ListRecurring(ctx)
}
func (l *LocalBackend) CreateRecurring(ctx context.Context, in domain.CreateRecurringInput) (domain.Recurring, error) {
	return l.Svcs.CreateRecurring(ctx, in)
}

// ───────────────────────── HTTP Backend ─────────────────────────

func (c *HTTPClient) InsightsOverview(ctx context.Context, q domain.PeriodQuery) (domain.InsightsOverview, error) {
	var out domain.InsightsOverview
	return out, c.do(ctx, http.MethodGet, "/insights/overview"+periodQS(q), nil, &out)
}
func (c *HTTPClient) InsightsProjection(ctx context.Context, days int) (domain.Projection, error) {
	p := "/insights/projection"
	if days > 0 {
		p += "?days=" + strconv.Itoa(days)
	}
	var out domain.Projection
	return out, c.do(ctx, http.MethodGet, p, nil, &out)
}
func (c *HTTPClient) InsightsYear(ctx context.Context, year *int) (domain.YearOverview, error) {
	var out domain.YearOverview
	return out, c.do(ctx, http.MethodGet, "/insights/year"+periodQS(domain.PeriodQuery{Year: year}), nil, &out)
}
func (c *HTTPClient) InsightsCommitments(ctx context.Context, q domain.PeriodQuery) (domain.Commitments, error) {
	var out domain.Commitments
	return out, c.do(ctx, http.MethodGet, "/insights/commitments"+periodQS(q), nil, &out)
}
func (c *HTTPClient) Recurring(ctx context.Context) ([]domain.Recurring, error) {
	var out []domain.Recurring
	return out, c.do(ctx, http.MethodGet, "/recurring", nil, &out)
}
func (c *HTTPClient) CreateRecurring(ctx context.Context, in domain.CreateRecurringInput) (domain.Recurring, error) {
	var out domain.Recurring
	return out, c.do(ctx, http.MethodPost, "/recurring", in, &out)
}
