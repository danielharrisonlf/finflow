package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"finflow/internal/domain"
	"finflow/internal/service"
)

// Backend unifica acesso local (in-process no banco) e remoto (HTTP REST) para CLI e MCP.
type Backend interface {
	DashboardSummary(ctx context.Context, q domain.PeriodQuery) (domain.DashboardSummary, error)
	DashboardCashflow(ctx context.Context, q domain.PeriodQuery) ([]domain.CashflowPoint, error)
	DashboardCategories(ctx context.Context, q domain.PeriodQuery) ([]domain.CategorySlice, error)
	DashboardUpcoming(ctx context.Context) ([]domain.UpcomingItem, error)
	Accounts(ctx context.Context, includeArchived bool) ([]domain.Account, error)
	Categories(ctx context.Context, kind *domain.CategoryKind) ([]domain.Category, error)
	Cards(ctx context.Context, includeArchived bool) ([]domain.Card, error)
	Invoices(ctx context.Context, cardID string) ([]domain.Invoice, error)
	Transactions(ctx context.Context, q domain.ListTransactionsQuery) (domain.Paginated[domain.Transaction], error)
	CreateTransaction(ctx context.Context, in domain.CreateTransactionInput) (domain.Transaction, error)
	Transfer(ctx context.Context, in domain.TransferInput) (domain.TransferResult, error)
	AddCardExpense(ctx context.Context, cardID string, in domain.CreateCardExpenseInput) (domain.CardExpenseResult, error)
	PayInvoice(ctx context.Context, cardID, invoiceID string, in domain.PayInvoiceInput) (domain.Invoice, error)
	Budgets(ctx context.Context, q domain.PeriodQuery) ([]domain.Budget, error)
	UpsertBudget(ctx context.Context, in domain.UpsertBudgetInput) (domain.Budget, error)
	Goals(ctx context.Context) ([]domain.Goal, error)
	ContributeGoal(ctx context.Context, id string, in domain.ContributeGoalInput) (domain.Goal, error)
	ExportTransactions(ctx context.Context, format string, q domain.ListTransactionsQuery) (string, error)
	SpreadsheetBackend
	PlanningBackend
	TelegramStatus(ctx context.Context) (domain.TelegramStatus, error)
	AuthStatus(ctx context.Context) (domain.AuthStatus, error)
}

// ───────────────────────── Local Backend ─────────────────────────

type LocalBackend struct {
	Svcs *service.Services
}

func (l *LocalBackend) DashboardSummary(ctx context.Context, q domain.PeriodQuery) (domain.DashboardSummary, error) {
	return l.Svcs.DashboardSummary(ctx, q)
}
func (l *LocalBackend) DashboardCashflow(ctx context.Context, q domain.PeriodQuery) ([]domain.CashflowPoint, error) {
	return l.Svcs.DashboardCashflow(ctx, q)
}
func (l *LocalBackend) DashboardCategories(ctx context.Context, q domain.PeriodQuery) ([]domain.CategorySlice, error) {
	return l.Svcs.DashboardCategories(ctx, q)
}
func (l *LocalBackend) DashboardUpcoming(ctx context.Context) ([]domain.UpcomingItem, error) {
	return l.Svcs.DashboardUpcoming(ctx)
}
func (l *LocalBackend) Accounts(ctx context.Context, includeArchived bool) ([]domain.Account, error) {
	return l.Svcs.ListAccounts(ctx, includeArchived)
}
func (l *LocalBackend) Categories(ctx context.Context, kind *domain.CategoryKind) ([]domain.Category, error) {
	return l.Svcs.ListCategories(ctx, kind)
}
func (l *LocalBackend) Cards(ctx context.Context, includeArchived bool) ([]domain.Card, error) {
	return l.Svcs.ListCards(ctx, includeArchived)
}
func (l *LocalBackend) Invoices(ctx context.Context, cardID string) ([]domain.Invoice, error) {
	return l.Svcs.ListInvoices(ctx, cardID)
}
func (l *LocalBackend) Transactions(ctx context.Context, q domain.ListTransactionsQuery) (domain.Paginated[domain.Transaction], error) {
	return l.Svcs.ListTransactions(ctx, q)
}
func (l *LocalBackend) CreateTransaction(ctx context.Context, in domain.CreateTransactionInput) (domain.Transaction, error) {
	return l.Svcs.CreateTransaction(ctx, in)
}
func (l *LocalBackend) Transfer(ctx context.Context, in domain.TransferInput) (domain.TransferResult, error) {
	return l.Svcs.Transfer(ctx, in)
}
func (l *LocalBackend) AddCardExpense(ctx context.Context, cardID string, in domain.CreateCardExpenseInput) (domain.CardExpenseResult, error) {
	return l.Svcs.AddCardExpense(ctx, cardID, in)
}
func (l *LocalBackend) PayInvoice(ctx context.Context, cardID, invoiceID string, in domain.PayInvoiceInput) (domain.Invoice, error) {
	return l.Svcs.PayInvoice(ctx, cardID, invoiceID, in)
}
func (l *LocalBackend) Budgets(ctx context.Context, q domain.PeriodQuery) ([]domain.Budget, error) {
	return l.Svcs.ListBudgets(ctx, q)
}
func (l *LocalBackend) UpsertBudget(ctx context.Context, in domain.UpsertBudgetInput) (domain.Budget, error) {
	return l.Svcs.UpsertBudget(ctx, in)
}
func (l *LocalBackend) Goals(ctx context.Context) ([]domain.Goal, error) {
	return l.Svcs.ListGoals(ctx)
}
func (l *LocalBackend) ContributeGoal(ctx context.Context, id string, in domain.ContributeGoalInput) (domain.Goal, error) {
	return l.Svcs.ContributeGoal(ctx, id, in)
}
func (l *LocalBackend) ExportTransactions(ctx context.Context, format string, q domain.ListTransactionsQuery) (string, error) {
	content, _, _, err := l.Svcs.ExportTransactions(ctx, format, q)
	return content, err
}
func (l *LocalBackend) TelegramStatus(_ context.Context) (domain.TelegramStatus, error) {
	return l.Svcs.Telegram.Status(), nil
}
func (l *LocalBackend) AuthStatus(_ context.Context) (domain.AuthStatus, error) {
	return l.Svcs.Auth.Status(""), nil
}

// ───────────────────────── HTTP Backend ─────────────────────────

type APIError struct {
	Status  int
	Message string
	Issues  []domain.ValidationIssue
}

func (e *APIError) Error() string {
	if len(e.Issues) > 0 {
		parts := make([]string, len(e.Issues))
		for i, iss := range e.Issues {
			parts[i] = fmt.Sprintf("%s: %s", iss.Path, iss.Message)
		}
		return fmt.Sprintf("%s (%s)", e.Message, strings.Join(parts, "; "))
	}
	return e.Message
}

type HTTPClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewHTTPClient(baseURL, token string) *HTTPClient {
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("FINFLOW_API_URL"))
	}
	if baseURL == "" {
		baseURL = "http://localhost:3333"
	}
	if token == "" {
		for _, k := range []string{"FINFLOW_TOKEN", "FINFLOW_API_TOKEN", "FINFLOW_PASSWORD"} {
			if v := strings.TrimSpace(os.Getenv(k)); v != "" {
				token = v
				break
			}
		}
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/") + "/api",
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	return c.doRaw(ctx, method, path, "application/json", reader, out)
}

// doRaw envia um corpo já serializado (JSON ou multipart) e decodifica a resposta como do.
func (c *HTTPClient) doRaw(ctx context.Context, method, path, contentType string, reader io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return &APIError{
			Status:  0,
			Message: fmt.Sprintf("não consegui falar com a API em %s — ela está rodando? (%v)", c.baseURL, err),
		}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{Status: resp.StatusCode, Message: fmt.Sprintf("erro HTTP %d", resp.StatusCode)}
		var payload struct {
			Message string                   `json:"message"`
			Issues  []domain.ValidationIssue `json:"issues"`
		}
		if json.Unmarshal(raw, &payload) == nil {
			if payload.Message != "" {
				apiErr.Message = payload.Message
			}
			apiErr.Issues = payload.Issues
		}
		return apiErr
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if strPtr, ok := out.(*string); ok {
		*strPtr = string(raw)
		return nil
	}
	return json.Unmarshal(raw, out)
}

func periodQS(q domain.PeriodQuery) string {
	v := url.Values{}
	if q.Month != nil {
		v.Set("month", strconv.Itoa(*q.Month))
	}
	if q.Year != nil {
		v.Set("year", strconv.Itoa(*q.Year))
	}
	if enc := v.Encode(); enc != "" {
		return "?" + enc
	}
	return ""
}

func txQS(q domain.ListTransactionsQuery, format string) string {
	v := url.Values{}
	if q.Month != nil {
		v.Set("month", strconv.Itoa(*q.Month))
	}
	if q.Year != nil {
		v.Set("year", strconv.Itoa(*q.Year))
	}
	if q.AccountID != nil {
		v.Set("accountId", *q.AccountID)
	}
	if q.CardID != nil {
		v.Set("cardId", *q.CardID)
	}
	if q.CategoryID != nil {
		v.Set("categoryId", *q.CategoryID)
	}
	if q.Type != nil {
		v.Set("type", string(*q.Type))
	}
	if q.Status != nil {
		v.Set("status", string(*q.Status))
	}
	if q.Search != nil && *q.Search != "" {
		v.Set("search", *q.Search)
	}
	if format == "" {
		if q.Page > 0 {
			v.Set("page", strconv.Itoa(q.Page))
		}
		if q.PageSize > 0 {
			v.Set("pageSize", strconv.Itoa(q.PageSize))
		}
	} else {
		v.Set("format", format)
	}
	if enc := v.Encode(); enc != "" {
		return "?" + enc
	}
	return ""
}

func (c *HTTPClient) DashboardSummary(ctx context.Context, q domain.PeriodQuery) (domain.DashboardSummary, error) {
	var out domain.DashboardSummary
	return out, c.do(ctx, http.MethodGet, "/dashboard/summary"+periodQS(q), nil, &out)
}
func (c *HTTPClient) DashboardCashflow(ctx context.Context, q domain.PeriodQuery) ([]domain.CashflowPoint, error) {
	var out []domain.CashflowPoint
	return out, c.do(ctx, http.MethodGet, "/dashboard/cashflow"+periodQS(q), nil, &out)
}
func (c *HTTPClient) DashboardCategories(ctx context.Context, q domain.PeriodQuery) ([]domain.CategorySlice, error) {
	var out []domain.CategorySlice
	return out, c.do(ctx, http.MethodGet, "/dashboard/categories"+periodQS(q), nil, &out)
}
func (c *HTTPClient) DashboardUpcoming(ctx context.Context) ([]domain.UpcomingItem, error) {
	var out []domain.UpcomingItem
	return out, c.do(ctx, http.MethodGet, "/dashboard/upcoming", nil, &out)
}
func (c *HTTPClient) Accounts(ctx context.Context, includeArchived bool) ([]domain.Account, error) {
	p := "/accounts"
	if includeArchived {
		p += "?includeArchived=true"
	}
	var out []domain.Account
	return out, c.do(ctx, http.MethodGet, p, nil, &out)
}
func (c *HTTPClient) Categories(ctx context.Context, kind *domain.CategoryKind) ([]domain.Category, error) {
	p := "/categories"
	if kind != nil {
		p += "?kind=" + string(*kind)
	}
	var out []domain.Category
	return out, c.do(ctx, http.MethodGet, p, nil, &out)
}
func (c *HTTPClient) Cards(ctx context.Context, includeArchived bool) ([]domain.Card, error) {
	p := "/cards"
	if includeArchived {
		p += "?includeArchived=true"
	}
	var out []domain.Card
	return out, c.do(ctx, http.MethodGet, p, nil, &out)
}
func (c *HTTPClient) Invoices(ctx context.Context, cardID string) ([]domain.Invoice, error) {
	var out []domain.Invoice
	return out, c.do(ctx, http.MethodGet, "/cards/"+cardID+"/invoices", nil, &out)
}
func (c *HTTPClient) Transactions(ctx context.Context, q domain.ListTransactionsQuery) (domain.Paginated[domain.Transaction], error) {
	var out domain.Paginated[domain.Transaction]
	return out, c.do(ctx, http.MethodGet, "/transactions"+txQS(q, ""), nil, &out)
}
func (c *HTTPClient) CreateTransaction(ctx context.Context, in domain.CreateTransactionInput) (domain.Transaction, error) {
	var out domain.Transaction
	return out, c.do(ctx, http.MethodPost, "/transactions", in, &out)
}
func (c *HTTPClient) Transfer(ctx context.Context, in domain.TransferInput) (domain.TransferResult, error) {
	var out domain.TransferResult
	return out, c.do(ctx, http.MethodPost, "/transactions/transfer", in, &out)
}
func (c *HTTPClient) AddCardExpense(ctx context.Context, cardID string, in domain.CreateCardExpenseInput) (domain.CardExpenseResult, error) {
	var out domain.CardExpenseResult
	return out, c.do(ctx, http.MethodPost, "/cards/"+cardID+"/expenses", in, &out)
}
func (c *HTTPClient) PayInvoice(ctx context.Context, cardID, invoiceID string, in domain.PayInvoiceInput) (domain.Invoice, error) {
	var out domain.Invoice
	return out, c.do(ctx, http.MethodPost, "/cards/"+cardID+"/invoices/"+invoiceID+"/pay", in, &out)
}
func (c *HTTPClient) Budgets(ctx context.Context, q domain.PeriodQuery) ([]domain.Budget, error) {
	var out []domain.Budget
	return out, c.do(ctx, http.MethodGet, "/budgets"+periodQS(q), nil, &out)
}
func (c *HTTPClient) UpsertBudget(ctx context.Context, in domain.UpsertBudgetInput) (domain.Budget, error) {
	var out domain.Budget
	return out, c.do(ctx, http.MethodPost, "/budgets", in, &out)
}
func (c *HTTPClient) Goals(ctx context.Context) ([]domain.Goal, error) {
	var out []domain.Goal
	return out, c.do(ctx, http.MethodGet, "/goals", nil, &out)
}
func (c *HTTPClient) ContributeGoal(ctx context.Context, id string, in domain.ContributeGoalInput) (domain.Goal, error) {
	var out domain.Goal
	return out, c.do(ctx, http.MethodPost, "/goals/"+id+"/contributions", in, &out)
}
func (c *HTTPClient) ExportTransactions(ctx context.Context, format string, q domain.ListTransactionsQuery) (string, error) {
	var out string
	return out, c.do(ctx, http.MethodGet, "/transactions/export"+txQS(q, format), nil, &out)
}
func (c *HTTPClient) TelegramStatus(ctx context.Context) (domain.TelegramStatus, error) {
	var out domain.TelegramStatus
	return out, c.do(ctx, http.MethodGet, "/telegram/status", nil, &out)
}
func (c *HTTPClient) AuthStatus(ctx context.Context) (domain.AuthStatus, error) {
	var out domain.AuthStatus
	return out, c.do(ctx, http.MethodGet, "/auth/status", nil, &out)
}

// ───────────────────────── Name Resolution ─────────────────────────

type Named interface {
	GetID() string
	GetName() string
}

// ResolveByName resolve por id exato, nome exato (case-insensitive) ou substring única.
// Referência vazia (ou só espaços) nunca resolve.
func ResolveByName[T any](items []T, ref string, getID, getName func(T) string) (T, bool) {
	var zero T
	refLower := strings.ToLower(strings.TrimSpace(ref))
	if refLower == "" {
		return zero, false
	}
	for _, it := range items {
		if getID(it) == ref || strings.ToLower(getName(it)) == refLower {
			return it, true
		}
	}
	var matches []T
	for _, it := range items {
		if strings.Contains(strings.ToLower(getName(it)), refLower) {
			matches = append(matches, it)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return zero, false
}
