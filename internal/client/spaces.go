package client

import (
	"context"
	"net/http"
	"strconv"

	"finflow/internal/domain"
	"finflow/internal/spaces"
)

// SpacesBackend reúne a divisão de despesas (Casa) e as métricas da empresa (v3 fase 2 §4–§5),
// sempre no espaço escolhido (--espaco / FINFLOW_SPACE).
type SpacesBackend interface {
	Members(ctx context.Context) ([]domain.Member, error)
	CreateMember(ctx context.Context, in domain.CreateMemberInput) (domain.Member, error)
	MemberBalances(ctx context.Context, q domain.PeriodQuery) (domain.MemberBalances, error)
	Settle(ctx context.Context, in domain.SettleInput) (domain.Settlement, error)
	BusinessMetrics(ctx context.Context, q domain.PeriodQuery) (domain.BusinessMetrics, error)
	BusinessDRE(ctx context.Context, year *int) (domain.DRE, error)
}

// SpacesAdmin lista e cria espaços: no modo local pelo registro, no remoto pela API.
type SpacesAdmin interface {
	Spaces(ctx context.Context) ([]domain.Space, error)
	CreateSpace(ctx context.Context, in domain.CreateSpaceInput) (domain.Space, error)
	SpacesOverview(ctx context.Context) (domain.SpacesOverview, error)
}

// ───────────────────────── Local Backend ─────────────────────────

func (l *LocalBackend) Members(ctx context.Context) ([]domain.Member, error) {
	return l.Svcs.ListMembers(ctx)
}
func (l *LocalBackend) CreateMember(ctx context.Context, in domain.CreateMemberInput) (domain.Member, error) {
	return l.Svcs.CreateMember(ctx, in)
}
func (l *LocalBackend) MemberBalances(ctx context.Context, q domain.PeriodQuery) (domain.MemberBalances, error) {
	return l.Svcs.MemberBalances(ctx, q)
}
func (l *LocalBackend) Settle(ctx context.Context, in domain.SettleInput) (domain.Settlement, error) {
	return l.Svcs.Settle(ctx, in)
}
func (l *LocalBackend) BusinessMetrics(ctx context.Context, q domain.PeriodQuery) (domain.BusinessMetrics, error) {
	return l.Svcs.BusinessMetrics(ctx, q)
}
func (l *LocalBackend) BusinessDRE(ctx context.Context, year *int) (domain.DRE, error) {
	return l.Svcs.BusinessDRE(ctx, year)
}

// LocalSpaces administra os espaços direto no registro (modo local da CLI).
type LocalSpaces struct {
	Registry *spaces.Registry
}

func (l *LocalSpaces) Spaces(_ context.Context) ([]domain.Space, error) {
	return l.Registry.List()
}
func (l *LocalSpaces) CreateSpace(ctx context.Context, in domain.CreateSpaceInput) (domain.Space, error) {
	return l.Registry.Create(ctx, in)
}
func (l *LocalSpaces) SpacesOverview(ctx context.Context) (domain.SpacesOverview, error) {
	return l.Registry.Overview(ctx)
}

// ───────────────────────── HTTP Backend ─────────────────────────

func (c *HTTPClient) Members(ctx context.Context) ([]domain.Member, error) {
	var out []domain.Member
	return out, c.do(ctx, http.MethodGet, "/members", nil, &out)
}
func (c *HTTPClient) CreateMember(ctx context.Context, in domain.CreateMemberInput) (domain.Member, error) {
	var out domain.Member
	return out, c.do(ctx, http.MethodPost, "/members", in, &out)
}
func (c *HTTPClient) MemberBalances(ctx context.Context, q domain.PeriodQuery) (domain.MemberBalances, error) {
	var out domain.MemberBalances
	return out, c.do(ctx, http.MethodGet, "/members/balances"+periodQS(q), nil, &out)
}
func (c *HTTPClient) Settle(ctx context.Context, in domain.SettleInput) (domain.Settlement, error) {
	var out domain.Settlement
	return out, c.do(ctx, http.MethodPost, "/members/settle", in, &out)
}
func (c *HTTPClient) BusinessMetrics(ctx context.Context, q domain.PeriodQuery) (domain.BusinessMetrics, error) {
	var out domain.BusinessMetrics
	return out, c.do(ctx, http.MethodGet, "/business/metrics"+periodQS(q), nil, &out)
}
func (c *HTTPClient) BusinessDRE(ctx context.Context, year *int) (domain.DRE, error) {
	p := "/business/dre"
	if year != nil {
		p += "?year=" + strconv.Itoa(*year)
	}
	var out domain.DRE
	return out, c.do(ctx, http.MethodGet, p, nil, &out)
}

func (c *HTTPClient) Spaces(ctx context.Context) ([]domain.Space, error) {
	var out []domain.Space
	return out, c.do(ctx, http.MethodGet, "/spaces", nil, &out)
}
func (c *HTTPClient) CreateSpace(ctx context.Context, in domain.CreateSpaceInput) (domain.Space, error) {
	var out domain.Space
	return out, c.do(ctx, http.MethodPost, "/spaces", in, &out)
}
func (c *HTTPClient) SpacesOverview(ctx context.Context) (domain.SpacesOverview, error) {
	var out domain.SpacesOverview
	return out, c.do(ctx, http.MethodGet, "/spaces/overview", nil, &out)
}

// WithSpace faz todas as chamadas seguintes irem para o espaço informado (cabeçalho
// X-FinFlow-Space; vazio = pessoal).
func (c *HTTPClient) WithSpace(slug string) *HTTPClient {
	c.space = slug
	return c
}
