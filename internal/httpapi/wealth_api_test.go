package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestWealthAPI(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		t.Setenv("FINFLOW_PASSWORD", "")
		srv := httpapi.NewServer(service.New(store.New(database), nil), httpapi.ServerOptions{})
		missing := "/11111111-1111-4111-8111-111111111111"
		today := domain.TodayString()

		checking := decodeBody[domain.Account](t, doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "Corrente", "type": "CHECKING", "initialBalanceCents": 1000000}, nil))
		broker := decodeBody[domain.Account](t, doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "Corretora", "type": "INVESTMENT"}, nil))

		// ───── Investimentos
		expectIssue(t, doReq(t, srv, "POST", "/api/investments", map[string]any{"name": "", "kind": "ACOES", "investedCents": 0}, nil), "kind", "investimento inválido")
		expectCode(t, doReq(t, srv, "POST", "/api/investments", map[string]any{"name": "X", "kind": "STOCKS", "investedCents": 0, "accountId": checking.ID}, nil), 400, "conta não-INVESTMENT")
		inv := decodeBody[domain.Investment](t, doReq(t, srv, "POST", "/api/investments", map[string]any{
			"name": "Tesouro", "kind": "TREASURY", "institution": "Corretora", "accountId": broker.ID, "investedCents": 100000, "currentCents": 100000,
		}, nil))
		if inv.ID == "" || inv.AccountName == nil || inv.ReturnPct == nil || *inv.ValueUpdatedAt != today {
			t.Fatalf("investimento criado: %+v", inv)
		}
		res := doReq(t, srv, "POST", "/api/investments/"+inv.ID+"/contribution", map[string]any{"amountCents": 50000, "fromAccountId": checking.ID}, nil)
		expectCode(t, res, 200, "aporte")
		if got := decodeBody[domain.Investment](t, res); got.InvestedCents != 150000 || got.CurrentCents != 150000 {
			t.Fatalf("aporte: %+v", got)
		}
		expectIssue(t, doReq(t, srv, "POST", "/api/investments/"+inv.ID+"/contribution", map[string]any{"amountCents": 0}, nil), "amountCents", "aporte zero")
		res = doReq(t, srv, "POST", "/api/investments/"+inv.ID+"/value", map[string]any{"currentCents": 165000}, nil)
		expectCode(t, res, 200, "valor")
		if got := decodeBody[domain.Investment](t, res); got.ReturnCents != 15000 || *got.ReturnPct != 10 {
			t.Fatalf("valor: %+v", got)
		}
		expectCode(t, doReq(t, srv, "POST", "/api/investments/"+inv.ID+"/redeem", map[string]any{"amountCents": 999999}, nil), 400, "resgate maior")
		expectCode(t, doReq(t, srv, "POST", "/api/investments/"+inv.ID+"/redeem", map[string]any{"amountCents": 15000, "toAccountId": checking.ID}, nil), 200, "resgate")
		res = doReq(t, srv, "PATCH", "/api/investments/"+inv.ID, map[string]any{"name": "Tesouro Selic", "notes": "longo prazo"}, nil)
		expectCode(t, res, 200, "patch investimento")
		if got := decodeBody[domain.Investment](t, res); got.Name != "Tesouro Selic" || *got.Notes != "longo prazo" {
			t.Fatalf("patch: %+v", got)
		}
		list := decodeBody[[]domain.Investment](t, doReq(t, srv, "GET", "/api/investments", nil, nil))
		if len(list) != 1 {
			t.Fatalf("lista: %+v", list)
		}
		sum := decodeBody[domain.InvestmentSummary](t, doReq(t, srv, "GET", "/api/investments/summary", nil, nil))
		if sum.TotalCurrentCents != 150000 || len(sum.History) != 12 || len(sum.ByInstitution) != 1 {
			t.Fatalf("resumo: %+v", sum)
		}
		expectCode(t, doReq(t, srv, "PATCH", "/api/investments"+missing, map[string]any{"name": "x"}, nil), 404, "patch inexistente")
		expectCode(t, doReq(t, srv, "POST", "/api/investments/abc/value", map[string]any{"currentCents": 1}, nil), 400, "id inválido")

		// ───── Empréstimos
		expectIssue(t, doReq(t, srv, "POST", "/api/loans", map[string]any{"name": "X", "kind": "VEHICLE", "principalCents": 1000, "monthlyRatePct": 101, "totalInstallments": 1, "firstDueDate": today}, nil), "monthlyRatePct", "taxa > 100")
		loan := decodeBody[domain.Loan](t, doReq(t, srv, "POST", "/api/loans", map[string]any{
			"name": "Carro", "kind": "VEHICLE", "principalCents": 1000000, "monthlyRatePct": 1, "totalInstallments": 12,
			"firstDueDate": domain.AddMonthsToDateString(today, 1), "accountId": checking.ID,
		}, nil))
		if loan.InstallmentCents != 88849 || loan.MonthlyRatePct != 1 || loan.RemainingInstallments != 12 || loan.OutstandingCents != 1000002 {
			t.Fatalf("empréstimo: %+v", loan)
		}
		res = doReq(t, srv, "POST", "/api/loans/"+loan.ID+"/pay", nil, nil)
		expectCode(t, res, 201, "pagar parcela")
		if tx := decodeBody[domain.Transaction](t, res); tx.Type != domain.TxExpense || tx.AmountCents != -88849 || tx.Date != today {
			t.Fatalf("pagamento: %+v", tx)
		}
		loans := decodeBody[[]domain.Loan](t, doReq(t, srv, "GET", "/api/loans", nil, nil))
		if len(loans) != 1 || loans[0].PaidInstallments != 1 || loans[0].NextDueDate == nil {
			t.Fatalf("lista de empréstimos: %+v", loans)
		}
		res = doReq(t, srv, "PATCH", "/api/loans/"+loan.ID, map[string]any{"monthlyRatePct": 1.99, "lender": "Banco"}, nil)
		expectCode(t, res, 200, "patch empréstimo")
		if got := decodeBody[domain.Loan](t, res); got.MonthlyRatePct != 1.99 || *got.Lender != "Banco" {
			t.Fatalf("patch empréstimo: %+v", got)
		}
		strat := decodeBody[domain.DebtStrategy](t, doReq(t, srv, "GET", "/api/loans/strategy?extraCents=50000", nil, nil))
		if strat.MinimumOnly.Months != 11 || strat.Avalanche.Months >= 11 || len(strat.Snowball.PerLoan) != 1 {
			t.Fatalf("estratégia: %+v", strat)
		}
		expectIssue(t, doReq(t, srv, "GET", "/api/loans/strategy?extraCents=abc", nil, nil), "extraCents", "extra inválido")
		expectCode(t, doReq(t, srv, "POST", "/api/loans"+missing+"/pay", nil, nil), 404, "pagar inexistente")

		// ───── Patrimônio
		points := decodeBody[[]domain.NetWorthPoint](t, doReq(t, srv, "GET", "/api/insights/net-worth?months=3", nil, nil))
		if len(points) != 3 {
			t.Fatalf("patrimônio: %+v", points)
		}
		m, y := domain.CurrentMonthYear()
		if last := points[2]; last.Month != m || last.Year != y || last.InvestmentsCents != 150000 || last.DebtsCents <= 0 {
			t.Fatalf("último ponto: %+v", last)
		}
		expectIssue(t, doReq(t, srv, "GET", "/api/insights/net-worth?months=0", nil, nil), "months", "meses 0")
		ov := decodeBody[domain.InsightsOverview](t, doReq(t, srv, "GET", "/api/insights/overview", nil, nil))
		if ov.NetWorthCents != last(points).NetWorthCents {
			t.Fatalf("patrimônio do overview (%d) difere do último ponto (%+v)", ov.NetWorthCents, last(points))
		}

		expectCode(t, doReq(t, srv, "DELETE", "/api/loans/"+loan.ID, nil, nil), 204, "excluir empréstimo")
		expectCode(t, doReq(t, srv, "DELETE", "/api/investments/"+inv.ID, nil, nil), 204, "excluir investimento")
		expectCode(t, doReq(t, srv, "DELETE", "/api/investments/"+inv.ID, nil, nil), 404, "excluir investimento 2x")
	})
}

func last[T any](s []T) T { return s[len(s)-1] }

func TestMCPOverHTTP(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		st := store.New(database)
		if _, err := store.Seed(context.Background(), st); err != nil {
			t.Fatal(err)
		}
		t.Setenv("FINFLOW_PASSWORD", "segredo")
		srv := httpapi.NewServer(service.New(st, nil), httpapi.ServerOptions{})

		call := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_loans","arguments":{}}}`)
		expectCode(t, doReq(t, srv, "POST", "/api/mcp", call, nil), 401, "MCP sem token")
		login := decodeBody[domain.LoginResponse](t, doReq(t, srv, "POST", "/api/auth/login", map[string]any{"password": "segredo"}, nil))
		auth := map[string]string{"Authorization": "Bearer " + login.Token}

		res := doReq(t, srv, "GET", "/api/mcp", nil, auth)
		expectCode(t, res, 405, "GET /api/mcp")
		if res.Header().Get("Allow") != "POST" {
			t.Fatalf("Allow = %q", res.Header().Get("Allow"))
		}

		res = doReq(t, srv, "POST", "/api/mcp", call, auth)
		expectCode(t, res, 200, "tools/call list_loans")
		if !strings.HasPrefix(res.Header().Get("Content-Type"), "application/json") || !strings.Contains(res.Body.String(), "Financiamento do carro") {
			t.Fatalf("resposta MCP: %s %s", res.Header().Get("Content-Type"), res.Body.String())
		}

		// Lote: initialize + notificação (sem resposta) + tools/list.
		batch := json.RawMessage(`[
			{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}},
			{"jsonrpc":"2.0","method":"notifications/initialized"},
			{"jsonrpc":"2.0","id":"b","method":"tools/list"}
		]`)
		res = doReq(t, srv, "POST", "/api/mcp", batch, auth)
		expectCode(t, res, 200, "lote MCP")
		var resps []struct {
			ID     json.RawMessage `json:"id"`
			Result struct {
				ProtocolVersion string `json:"protocolVersion"`
				Tools           []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &resps); err != nil || len(resps) != 2 {
			t.Fatalf("lote: %s %v", res.Body.String(), err)
		}
		names := map[string]bool{}
		for _, tool := range resps[1].Result.Tools {
			names[tool.Name] = true
		}
		for _, want := range []string{"list_investments", "get_investment_summary", "list_loans", "get_debt_strategy", "get_net_worth_history", "get_summary"} {
			if !names[want] {
				t.Errorf("tools/list sem %s", want)
			}
		}
		if resps[0].Result.ProtocolVersion != "2025-06-18" {
			t.Errorf("protocolVersion = %q", resps[0].Result.ProtocolVersion)
		}

		expectCode(t, doReq(t, srv, "POST", "/api/mcp", json.RawMessage(`{"jsonrpc":"2.0","method":"notifications/initialized"}`), auth), 202, "só notificação")
		req := httptest.NewRequest("POST", "/api/mcp", strings.NewReader(`{"jsonrpc":`))
		req.Header.Set("Authorization", auth["Authorization"])
		res = httptest.NewRecorder()
		srv.ServeHTTP(res, req)
		expectCode(t, res, 200, "JSON inválido")
		if !strings.Contains(res.Body.String(), "-32700") {
			t.Fatalf("parse error: %s", res.Body.String())
		}
		withOrigin := map[string]string{"Authorization": auth["Authorization"], "Origin": "http://evil.example"}
		expectCode(t, doReq(t, srv, "POST", "/api/mcp", call, withOrigin), 403, "origem estranha")
	})
}

func TestSystemInfo(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "")
	database := testutil.Open(t, db.DialectSQLite)
	srv := httpapi.NewServer(service.New(store.New(database), nil), httpapi.ServerOptions{Version: "v9.9.9", LAN: true, Port: 4321})
	res := doReq(t, srv, "GET", "/api/system/info", nil, nil)
	expectCode(t, res, 200, "system info")
	var info map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "dbDialect", "dataPath", "authRequired", "lanEnabled", "lanUrls", "aiConfigured", "mcpHttpPath"} {
		if _, ok := info[key]; !ok {
			t.Errorf("system info sem %q: %s", key, res.Body.String())
		}
	}
	if info["version"] != "v9.9.9" || info["dbDialect"] != "sqlite" || info["lanEnabled"] != true || info["mcpHttpPath"] != "/api/mcp" || info["authRequired"] != false {
		t.Fatalf("system info: %s", res.Body.String())
	}
	if _, ok := info["lanUrls"].([]any); !ok {
		t.Fatalf("lanUrls deve ser array: %s", res.Body.String())
	}
	for _, u := range info["lanUrls"].([]any) {
		if !strings.HasSuffix(u.(string), ":4321") {
			t.Errorf("URL da LAN sem a porta: %v", u)
		}
	}
}
