package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"finflow/internal/domain"
)

// InsightsV3Backend reúne tags, assinaturas detectadas, alertas, "posso comprar?" e fluxo de caixa (v3 §1–§8).
type InsightsV3Backend interface {
	Tags(ctx context.Context) ([]domain.Tag, error)
	Subscriptions(ctx context.Context) (domain.SubscriptionsView, error)
	Alerts(ctx context.Context) ([]domain.Alert, error)
	CanAfford(ctx context.Context, in domain.CanAffordInput) (domain.CanAffordResult, error)
	InsightsCashflow(ctx context.Context, q domain.CashflowQuery) (domain.CashflowView, error)
}

// ───────────────────────── Local Backend ─────────────────────────

func (l *LocalBackend) Tags(ctx context.Context) ([]domain.Tag, error) {
	return l.Svcs.ListTags(ctx)
}
func (l *LocalBackend) Subscriptions(ctx context.Context) (domain.SubscriptionsView, error) {
	return l.Svcs.Subscriptions(ctx)
}
func (l *LocalBackend) Alerts(ctx context.Context) ([]domain.Alert, error) {
	return l.Svcs.Alerts(ctx)
}
func (l *LocalBackend) CanAfford(ctx context.Context, in domain.CanAffordInput) (domain.CanAffordResult, error) {
	return l.Svcs.CanAfford(ctx, in)
}
func (l *LocalBackend) InsightsCashflow(ctx context.Context, q domain.CashflowQuery) (domain.CashflowView, error) {
	return l.Svcs.InsightsCashflow(ctx, q)
}

// ───────────────────────── HTTP Backend ─────────────────────────

func (c *HTTPClient) Tags(ctx context.Context) ([]domain.Tag, error) {
	var out []domain.Tag
	return out, c.do(ctx, http.MethodGet, "/tags", nil, &out)
}
func (c *HTTPClient) Subscriptions(ctx context.Context) (domain.SubscriptionsView, error) {
	var out domain.SubscriptionsView
	return out, c.do(ctx, http.MethodGet, "/insights/subscriptions", nil, &out)
}
func (c *HTTPClient) Alerts(ctx context.Context) ([]domain.Alert, error) {
	var out []domain.Alert
	return out, c.do(ctx, http.MethodGet, "/alerts", nil, &out)
}
func (c *HTTPClient) CanAfford(ctx context.Context, in domain.CanAffordInput) (domain.CanAffordResult, error) {
	var out domain.CanAffordResult
	return out, c.do(ctx, http.MethodPost, "/insights/can-afford", in, &out)
}
func (c *HTTPClient) InsightsCashflow(ctx context.Context, q domain.CashflowQuery) (domain.CashflowView, error) {
	v := url.Values{}
	if q.Months > 0 {
		v.Set("months", strconv.Itoa(q.Months))
	}
	if q.Basis != "" {
		v.Set("basis", q.Basis)
	}
	if q.Month != nil {
		v.Set("month", strconv.Itoa(*q.Month))
	}
	if q.Year != nil {
		v.Set("year", strconv.Itoa(*q.Year))
	}
	p := "/insights/cashflow"
	if enc := v.Encode(); enc != "" {
		p += "?" + enc
	}
	var out domain.CashflowView
	return out, c.do(ctx, http.MethodGet, p, nil, &out)
}
