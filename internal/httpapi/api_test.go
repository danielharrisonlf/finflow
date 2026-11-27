package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"finflow/internal/client"
	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func doReq(t *testing.T, srv http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func decodeBody[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal %s: %v", rr.Body.String(), err)
	}
	return out
}

func TestFinFlowAPIIntegration(t *testing.T) {
	testutil.ForEachDialect(t, runAPIIntegration)
}

func runAPIIntegration(t *testing.T, database *db.DB) {
	t.Setenv("FINFLOW_PASSWORD", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_CHAT_ID", "4242")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "test-secret")

	var tgCalls []string
	mockHTTP := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			tgCalls = append(tgCalls, r.URL.String())
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	ctx := context.Background()
	st := store.New(database)
	svcs := service.New(st, mockHTTP)
	srv := httpapi.NewServer(svcs, httpapi.ServerOptions{})
	today := domain.TodayString()

	// 1. Cria contas e categoria
	res := doReq(t, srv, "POST", "/api/accounts", map[string]any{
		"name": "Conta Corrente", "type": "CHECKING", "initialBalanceCents": 100000,
	}, nil)
	if res.Code != 201 {
		t.Fatalf("create account A: %d %s", res.Code, res.Body.String())
	}
	accA := decodeBody[domain.Account](t, res)
	if accA.BalanceCents != 100000 {
		t.Fatalf("accA balance = %d", accA.BalanceCents)
	}

	res = doReq(t, srv, "POST", "/api/accounts", map[string]any{
		"name": "Poupança", "type": "SAVINGS", "initialBalanceCents": 50000,
	}, nil)
	if res.Code != 201 {
		t.Fatalf("create account B: %d %s", res.Code, res.Body.String())
	}
	accB := decodeBody[domain.Account](t, res)

	res = doReq(t, srv, "POST", "/api/categories", map[string]any{
		"name": "Alimentação", "kind": "EXPENSE", "icon": "utensils", "color": "#fb923c",
	}, nil)
	if res.Code != 201 {
		t.Fatalf("create category: %d %s", res.Code, res.Body.String())
	}
	foodCat := decodeBody[domain.Category](t, res)

	// 2. Validação 400 com issues
	res = doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "", "type": "NAO_EXISTE"}, nil)
	if res.Code != 400 {
		t.Fatalf("expected 400, got %d", res.Code)
	}
	var errBody struct {
		Issues []domain.ValidationIssue `json:"issues"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &errBody)
	if len(errBody.Issues) == 0 {
		t.Fatalf("expected validation issues, got %s", res.Body.String())
	}

	// 3. Despesa e receita refletindo no saldo
	if r := doReq(t, srv, "POST", "/api/transactions", map[string]any{
		"type": "EXPENSE", "description": "Mercado", "amountCents": 25000, "date": today,
		"accountId": accA.ID, "categoryId": foodCat.ID,
	}, nil); r.Code != 201 {
		t.Fatalf("create expense: %d %s", r.Code, r.Body.String())
	}
	if r := doReq(t, srv, "POST", "/api/transactions", map[string]any{
		"type": "INCOME", "description": "Salário", "amountCents": 300000, "date": today,
		"accountId": accA.ID,
	}, nil); r.Code != 201 {
		t.Fatalf("create income: %d %s", r.Code, r.Body.String())
	}
	accADetail := decodeBody[domain.Account](t, doReq(t, srv, "GET", "/api/accounts/"+accA.ID, nil, nil))
	if accADetail.BalanceCents != 375000 {
		t.Fatalf("expected 375000, got %d", accADetail.BalanceCents)
	}

	// 4. Transferência atômica e exclusão em par
	if r := doReq(t, srv, "POST", "/api/transactions/transfer", map[string]any{
		"fromAccountId": accA.ID, "toAccountId": accB.ID, "amountCents": 40000, "date": today,
	}, nil); r.Code != 201 {
		t.Fatalf("transfer: %d %s", r.Code, r.Body.String())
	}
	if decodeBody[domain.Account](t, doReq(t, srv, "GET", "/api/accounts/"+accA.ID, nil, nil)).BalanceCents != 335000 {
		t.Fatal("transfer debit wrong")
	}
	if decodeBody[domain.Account](t, doReq(t, srv, "GET", "/api/accounts/"+accB.ID, nil, nil)).BalanceCents != 90000 {
		t.Fatal("transfer credit wrong")
	}
	trList := decodeBody[domain.Paginated[domain.Transaction]](t, doReq(t, srv, "GET", "/api/transactions?type=TRANSFER", nil, nil))
	if len(trList.Items) != 2 {
		t.Fatalf("expected 2 transfer legs, got %d", len(trList.Items))
	}
	if r := doReq(t, srv, "DELETE", "/api/transactions/"+trList.Items[0].ID, nil, nil); r.Code != 204 {
		t.Fatalf("delete transfer leg: %d", r.Code)
	}
	trAfter := decodeBody[domain.Paginated[domain.Transaction]](t, doReq(t, srv, "GET", "/api/transactions?type=TRANSFER", nil, nil))
	if len(trAfter.Items) != 0 {
		t.Fatalf("expected 0 transfer legs after delete, got %d", len(trAfter.Items))
	}

	// 5. Cartão de crédito: parcelado 3x + pagamento de fatura + 409 em pagamento duplicado
	card := decodeBody[domain.Card](t, doReq(t, srv, "POST", "/api/cards", map[string]any{
		"name": "Cartão Teste", "brand": "VISA", "limitCents": 500000, "closingDay": 10, "dueDay": 17,
	}, nil))
	if r := doReq(t, srv, "POST", "/api/cards/"+card.ID+"/expenses", map[string]any{
		"cardId": card.ID, "description": "Fone de ouvido", "totalCents": 100000,
		"date": today, "installments": 3, "categoryId": foodCat.ID,
	}, nil); r.Code != 201 {
		t.Fatalf("card expense: %d %s", r.Code, r.Body.String())
	}
	invoices := decodeBody[[]domain.Invoice](t, doReq(t, srv, "GET", "/api/cards/"+card.ID+"/invoices", nil, nil))
	if len(invoices) != 3 {
		t.Fatalf("expected 3 invoices, got %d", len(invoices))
	}
	var invSum int64
	for _, inv := range invoices {
		invSum += inv.TotalCents
	}
	if invSum != 100000 {
		t.Fatalf("invoices sum = %d", invSum)
	}
	cardDetail := decodeBody[domain.Card](t, doReq(t, srv, "GET", "/api/cards/"+card.ID, nil, nil))
	if cardDetail.UsedLimitCents != 100000 || cardDetail.AvailableLimitCents != 400000 {
		t.Fatalf("card limits wrong: %+v", cardDetail)
	}

	oldest := invoices[len(invoices)-1]
	balBefore := decodeBody[domain.Account](t, doReq(t, srv, "GET", "/api/accounts/"+accA.ID, nil, nil)).BalanceCents
	payRes := doReq(t, srv, "POST", "/api/cards/"+card.ID+"/invoices/"+oldest.ID+"/pay", map[string]any{"accountId": accA.ID}, nil)
	if payRes.Code != 201 {
		t.Fatalf("pay invoice: %d %s", payRes.Code, payRes.Body.String())
	}
	paid := decodeBody[domain.Invoice](t, payRes)
	if paid.Status != domain.InvoicePaid {
		t.Fatalf("expected PAID, got %s", paid.Status)
	}
	balAfter := decodeBody[domain.Account](t, doReq(t, srv, "GET", "/api/accounts/"+accA.ID, nil, nil)).BalanceCents
	if balAfter != balBefore-oldest.TotalCents {
		t.Fatalf("account balance after pay = %d, want %d", balAfter, balBefore-oldest.TotalCents)
	}
	if r := doReq(t, srv, "POST", "/api/cards/"+card.ID+"/invoices/"+oldest.ID+"/pay", map[string]any{"accountId": accA.ID}, nil); r.Code != 409 {
		t.Fatalf("double pay expected 409, got %d", r.Code)
	}

	// 6. Orçamento
	bRes := doReq(t, srv, "POST", "/api/budgets", map[string]any{"categoryId": foodCat.ID, "amountCents": 50000}, nil)
	if bRes.Code != 201 {
		t.Fatalf("upsert budget: %d %s", bRes.Code, bRes.Body.String())
	}
	budget := decodeBody[domain.Budget](t, bRes)
	if budget.SpentCents < 25000 {
		t.Fatalf("budget spent = %d", budget.SpentCents)
	}

	// 7. Meta + aporte + bloqueio de resgate negativo
	goal := decodeBody[domain.Goal](t, doReq(t, srv, "POST", "/api/goals", map[string]any{
		"name": "Reserva", "targetCents": 120000, "icon": "shield",
	}, nil))
	afterGoal := decodeBody[domain.Goal](t, doReq(t, srv, "POST", "/api/goals/"+goal.ID+"/contributions", map[string]any{"amountCents": 30000}, nil))
	if afterGoal.SavedCents != 30000 || afterGoal.Percent != 25 {
		t.Fatalf("goal after contribution: %+v", afterGoal)
	}
	if r := doReq(t, srv, "POST", "/api/goals/"+goal.ID+"/contributions", map[string]any{"amountCents": -50000}, nil); r.Code != 400 {
		t.Fatalf("negative goal withdrawal expected 400, got %d", r.Code)
	}

	// 8. Dashboard & Export CSV / OFX
	summary := decodeBody[domain.DashboardSummary](t, doReq(t, srv, "GET", "/api/dashboard/summary", nil, nil))
	if summary.MonthIncomeCents <= 0 || summary.MonthExpenseCents <= 0 {
		t.Fatalf("summary wrong: %+v", summary)
	}
	csvRes := doReq(t, srv, "GET", "/api/transactions/export?format=csv", nil, nil)
	if csvRes.Code != 200 || !strings.Contains(csvRes.Header().Get("Content-Disposition"), ".csv") || !strings.Contains(csvRes.Body.String(), "Data;Descrição") {
		t.Fatalf("csv export failed: %d %s", csvRes.Code, csvRes.Body.String())
	}
	ofxRes := doReq(t, srv, "GET", "/api/transactions/export?format=ofx", nil, nil)
	if ofxRes.Code != 200 || !strings.Contains(ofxRes.Body.String(), "<OFX>") || !strings.Contains(ofxRes.Body.String(), "<STMTTRN>") {
		t.Fatalf("ofx export failed: %d", ofxRes.Code)
	}

	// 9. Telegram webhook (secret errado 403 + despesa rápida 200)
	if r := doReq(t, srv, "POST", "/api/telegram/webhook", map[string]any{
		"update_id": 1, "message": map[string]any{"message_id": 1, "text": "/saldo", "chat": map[string]any{"id": 4242}},
	}, map[string]string{"x-telegram-bot-api-secret-token": "errado"}); r.Code != 403 {
		t.Fatalf("telegram bad secret expected 403, got %d", r.Code)
	}
	if r := doReq(t, srv, "POST", "/api/telegram/webhook", map[string]any{
		"update_id": 2, "message": map[string]any{"message_id": 2, "text": "19,90 pizza", "chat": map[string]any{"id": 4242}},
	}, map[string]string{"x-telegram-bot-api-secret-token": "test-secret"}); r.Code != 200 {
		t.Fatalf("telegram webhook: %d %s", r.Code, r.Body.String())
	}
	if len(tgCalls) == 0 || !strings.Contains(tgCalls[0], "/bottest-token/sendMessage") {
		t.Fatalf("expected telegram sendMessage call, got %v", tgCalls)
	}
	pizzaList := decodeBody[domain.Paginated[domain.Transaction]](t, doReq(t, srv, "GET", "/api/transactions?search=pizza", nil, nil))
	if len(pizzaList.Items) != 1 || pizzaList.Items[0].AmountCents != -1990 {
		t.Fatalf("telegram pizza tx wrong: %+v", pizzaList.Items)
	}

	// 10. Subcategoria + edição de transação
	sub := decodeBody[domain.Category](t, doReq(t, srv, "POST", "/api/categories", map[string]any{
		"name": "Restaurantes", "kind": "EXPENSE", "icon": "utensils", "color": "#f97316", "parentId": foodCat.ID,
	}, nil))
	if sub.ParentID == nil || *sub.ParentID != foodCat.ID {
		t.Fatalf("sub parentId wrong: %+v", sub)
	}
	updatedSub := decodeBody[domain.Category](t, doReq(t, srv, "PATCH", "/api/categories/"+sub.ID, map[string]any{
		"name": "Restaurantes & Delivery",
	}, nil))
	if updatedSub.Name != "Restaurantes & Delivery" {
		t.Fatalf("updated sub wrong: %+v", updatedSub)
	}
	mercadoList := decodeBody[domain.Paginated[domain.Transaction]](t, doReq(t, srv, "GET", "/api/transactions?search=Mercado", nil, nil))
	editedTx := decodeBody[domain.Transaction](t, doReq(t, srv, "PATCH", "/api/transactions/"+mercadoList.Items[0].ID, map[string]any{
		"description": "Supermercado mensal", "amountCents": 28000, "categoryId": sub.ID, "notes": "compra ajustada",
	}, nil))
	if editedTx.Description != "Supermercado mensal" || editedTx.AmountCents != -28000 || editedTx.CategoryName == nil || *editedTx.CategoryName != "Restaurantes & Delivery" {
		t.Fatalf("edited tx wrong: %+v", editedTx)
	}

	// 11. Autenticação (FINFLOW_PASSWORD)
	t.Setenv("FINFLOW_PASSWORD", "senha-integracao-123")
	stAnon := decodeBody[domain.AuthStatus](t, doReq(t, srv, "GET", "/api/auth/status", nil, nil))
	if !stAnon.Required || stAnon.Authenticated {
		t.Fatalf("anon auth status: %+v", stAnon)
	}
	if r := doReq(t, srv, "GET", "/api/accounts", nil, nil); r.Code != 401 {
		t.Fatalf("expected 401 without token, got %d", r.Code)
	}
	if r := doReq(t, srv, "POST", "/api/auth/login", map[string]any{"password": "errada"}, nil); r.Code != 401 {
		t.Fatalf("expected 401 bad password, got %d", r.Code)
	}
	loginRes := doReq(t, srv, "POST", "/api/auth/login", map[string]any{"password": "senha-integracao-123"}, nil)
	if loginRes.Code != 200 {
		t.Fatalf("login failed: %d %s", loginRes.Code, loginRes.Body.String())
	}
	login := decodeBody[domain.LoginResponse](t, loginRes)
	if login.Token == "" {
		t.Fatal("empty token")
	}
	authHeader := map[string]string{"Authorization": "Bearer " + login.Token}
	stAuth := decodeBody[domain.AuthStatus](t, doReq(t, srv, "GET", "/api/auth/status", nil, authHeader))
	if !stAuth.Required || !stAuth.Authenticated {
		t.Fatalf("auth status: %+v", stAuth)
	}
	if r := doReq(t, srv, "GET", "/api/accounts", nil, authHeader); r.Code != 200 {
		t.Fatalf("authenticated accounts: %d", r.Code)
	}
	if r := doReq(t, srv, "GET", "/api/transactions/export?format=csv&token="+url.QueryEscape(login.Token), nil, nil); r.Code != 200 {
		t.Fatalf("query token export: %d", r.Code)
	}

	// 12. HTTPClient + Seed smoke test
	ts := httptest.NewServer(srv)
	defer ts.Close()
	cli := client.NewHTTPClient(ts.URL, login.Token)
	accounts, err := cli.Accounts(ctx, false)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("HTTPClient.Accounts: %v (%d)", err, len(accounts))
	}
}

func TestSeedIdempotent(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		runSeedIdempotent(t, database)
	})
}

func runSeedIdempotent(t *testing.T, database *db.DB) {
	ctx := context.Background()
	st := store.New(database)
	seeded, err := store.Seed(ctx, st)
	if err != nil || !seeded {
		t.Fatalf("first seed failed: seeded=%v err=%v", seeded, err)
	}
	seededAgain, err := store.Seed(ctx, st)
	if err != nil || seededAgain {
		t.Fatalf("second seed should be no-op: seeded=%v err=%v", seededAgain, err)
	}
	accs, err := st.ListAccounts(ctx, false)
	if err != nil || len(accs) != 4 {
		t.Fatalf("expected 4 accounts, got %d (%v)", len(accs), err)
	}
}
