package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/testutil"
)

func TestWealthTools(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		backend := seededBackend(t, database)
		resps := runLines(t, backend,
			callLine(1, "list_investments", `{}`),
			callLine(2, "get_investment_summary", `{}`),
			callLine(3, "list_loans", `{}`),
			callLine(4, "get_debt_strategy", `{"extraCents":50000}`),
			callLine(5, "get_net_worth_history", `{"months":3}`),
			callLine(6, "get_debt_strategy", `{"extraCents":-1}`),
			callLine(7, "get_net_worth_history", `{"months":61}`),
		)
		if len(resps) != 7 {
			t.Fatalf("esperava 7 respostas, veio %d", len(resps))
		}
		wants := []string{`"Tesouro Selic 2029"`, `"byKind"`, `"Financiamento do carro"`, `"avalanche"`, `"netWorthCents"`}
		for i, want := range wants {
			tr := toolResultOf(t, resps[i])
			if tr.IsError || !strings.Contains(tr.Content[0].Text, want) {
				t.Errorf("chamada %d: esperava sucesso contendo %q, veio isError=%v %.200q", i+1, want, tr.IsError, tr.Content[0].Text)
			}
		}
		var points []domain.NetWorthPoint
		if err := json.Unmarshal([]byte(toolResultOf(t, resps[4]).Content[0].Text), &points); err != nil || len(points) != 3 {
			t.Fatalf("get_net_worth_history months=3: %d %v", len(points), err)
		}
		for i, want := range []string{"extraCents: deve ser >= 0", "months: deve ser <= 60"} {
			tr := toolResultOf(t, resps[5+i])
			if !tr.IsError || !strings.Contains(tr.Content[0].Text, want) {
				t.Errorf("validação %d: esperava isError com %q, veio %v %q", i, want, tr.IsError, tr.Content[0].Text)
			}
		}
	})
}

func TestInProcessAPI(t *testing.T) {
	tools := Tools()
	if len(tools) != len(allTools()) {
		t.Fatalf("Tools() = %d, allTools() = %d", len(tools), len(allTools()))
	}
	readOnly := map[string]bool{}
	for _, tool := range tools {
		readOnly[tool.Name] = tool.ReadOnly
		if tool.InputSchema == nil || tool.Description == "" {
			t.Errorf("tool incompleta: %+v", tool)
		}
	}
	if !readOnly["list_loans"] || !readOnly["get_net_worth_history"] || readOnly["create_expense"] || readOnly["transfer"] {
		t.Fatalf("ReadOnly incorreto: %+v", readOnly)
	}

	database := testutil.Open(t, db.DialectSQLite)
	backend := seededBackend(t, database)
	ctx := context.Background()
	text, err := Call(ctx, backend, "list_loans", nil)
	if err != nil || !strings.Contains(text, "outstandingCents") {
		t.Fatalf("Call list_loans = %.200q %v", text, err)
	}
	if _, err := Call(ctx, backend, "get_net_worth_history", json.RawMessage(`{"months":0}`)); err == nil || !strings.Contains(err.Error(), "months") {
		t.Fatalf("Call com argumento inválido deveria falhar: %v", err)
	}
	if _, err := Call(ctx, backend, "nao_existe", nil); err == nil || !strings.Contains(err.Error(), "tool desconhecida") {
		t.Fatalf("tool desconhecida: %v", err)
	}

	srv := NewServer(backend)
	out, ok := srv.HandleJSONRPC(ctx, []byte(`{"jsonrpc":"2.0","id":9,"method":"ping"}`))
	if !ok || !strings.Contains(string(out), `"id":9`) {
		t.Fatalf("ping = %s %v", out, ok)
	}
	if out, ok := srv.HandleJSONRPC(ctx, []byte(`[{"jsonrpc":"2.0","method":"notifications/initialized"}]`)); ok || out != nil {
		t.Fatalf("lote só de notificações não tem resposta: %s", out)
	}
	if out, ok := srv.HandleJSONRPC(ctx, []byte(`[]`)); !ok || !strings.Contains(string(out), "-32700") {
		t.Fatalf("lote vazio = %s", out)
	}
}
