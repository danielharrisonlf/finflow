package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"finflow/internal/client"
	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

type rawResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// runLines executa o servidor stdio com as linhas dadas e devolve as respostas
// (validando que tudo o que foi para stdout é JSON-RPC).
func runLines(t *testing.T, backend client.Backend, lines ...string) []rawResponse {
	t.Helper()
	var out, errOut bytes.Buffer
	srv := &Server{
		backend: backend,
		in:      strings.NewReader(strings.Join(lines, "\n") + "\n"),
		out:     &out,
		errOut:  &errOut,
	}
	if err := srv.Run(context.Background(), "test"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(errOut.String(), "finflow-mcp conectado") {
		t.Fatalf("banner deveria ir para stderr, veio %q", errOut.String())
	}
	var resps []rawResponse
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if l == "" {
			continue
		}
		var r rawResponse
		if err := json.Unmarshal([]byte(l), &r); err != nil || r.JSONRPC != "2.0" {
			t.Fatalf("stdout com linha não JSON-RPC: %q (%v)", l, err)
		}
		if !strings.Contains(l, `"id":`) {
			t.Fatalf("resposta sem id: %s", l)
		}
		resps = append(resps, r)
	}
	return resps
}

func TestInitializeProtocolNegotiation(t *testing.T) {
	cases := []struct{ params, want string }{
		{`{"protocolVersion":"2025-06-18"}`, "2025-06-18"},
		{`{"protocolVersion":"2025-03-26"}`, "2025-03-26"},
		{`{"protocolVersion":"2024-11-05"}`, "2024-11-05"},
		{`{"protocolVersion":"1999-01-01"}`, "2025-06-18"},
		{`{"protocolVersion":"2099-12-31"}`, "2025-06-18"},
		{`{}`, "2025-06-18"},
	}
	for _, tc := range cases {
		resps := runLines(t, nil, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":`+tc.params+`}`)
		if len(resps) != 1 {
			t.Fatalf("%s: %d respostas", tc.params, len(resps))
		}
		var res struct {
			ProtocolVersion string         `json:"protocolVersion"`
			Capabilities    map[string]any `json:"capabilities"`
			ServerInfo      map[string]any `json:"serverInfo"`
		}
		if err := json.Unmarshal(resps[0].Result, &res); err != nil {
			t.Fatal(err)
		}
		if res.ProtocolVersion != tc.want {
			t.Fatalf("%s: protocolVersion = %q, want %q", tc.params, res.ProtocolVersion, tc.want)
		}
		if res.Capabilities["tools"] == nil || res.ServerInfo["name"] != "finflow" {
			t.Fatalf("initialize incompleto: %s", resps[0].Result)
		}
	}
}

func TestNotificationsGetNoReply(t *testing.T) {
	resps := runLines(t, nil,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`,
		`{"jsonrpc":"2.0","method":"metodo/inexistente"}`,
		``,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
	)
	if len(resps) != 1 || string(resps[0].ID) != "7" {
		t.Fatalf("só o ping deveria ter resposta, veio %+v", resps)
	}
}

func TestParseErrorHasNullID(t *testing.T) {
	var out bytes.Buffer
	srv := &Server{in: strings.NewReader("{isso não é json\n"), out: &out, errOut: &bytes.Buffer{}}
	if err := srv.Run(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(out.String())
	if !strings.Contains(line, `"id":null`) {
		t.Fatalf("erro de parse deve ter \"id\":null: %s", line)
	}
	var r rawResponse
	if err := json.Unmarshal([]byte(line), &r); err != nil || r.Error == nil || r.Error.Code != -32700 {
		t.Fatalf("resposta = %s (%v)", line, err)
	}
}

func TestUnknownMethod(t *testing.T) {
	resps := runLines(t, nil, `{"jsonrpc":"2.0","id":"abc","method":"resources/list"}`)
	if len(resps) != 1 || resps[0].Error == nil || resps[0].Error.Code != -32601 || string(resps[0].ID) != `"abc"` {
		t.Fatalf("esperava -32601 com id \"abc\": %+v", resps)
	}
}

func seededBackend(t *testing.T, database *db.DB) client.Backend {
	t.Helper()
	st := store.New(database)
	if _, err := store.Seed(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	return &client.LocalBackend{Svcs: service.New(st, nil)}
}

func callLine(id int, tool, args string) string {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": json.RawMessage(args)},
	})
	return string(b)
}

func toolResultOf(t *testing.T, r rawResponse) toolResult {
	t.Helper()
	if r.Error != nil {
		t.Fatalf("erro JSON-RPC inesperado: %+v", r.Error)
	}
	var tr toolResult
	if err := json.Unmarshal(r.Result, &tr); err != nil {
		t.Fatal(err)
	}
	if len(tr.Content) == 0 {
		t.Fatalf("resultado sem content: %s", r.Result)
	}
	return tr
}

func TestToolsCallArgumentValidation(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		backend := seededBackend(t, database)
		ctx := context.Background()
		before, err := backend.Transactions(ctx, domain.ListTransactionsQuery{})
		if err != nil {
			t.Fatal(err)
		}

		invalid := []struct {
			name, tool, args, wantMsg string
		}{
			{"card ausente", "list_card_invoices", `{}`, "card: obrigatório"},
			{"card null", "list_card_invoices", `{"card":null}`, "card: obrigatório"},
			{"card não-string", "list_card_invoices", `{"card":123}`, "card: deve ser texto"},
			{"goal ausente", "contribute_goal", `{"amountCents":100}`, "goal: obrigatório"},
			{"goal vazio", "contribute_goal", `{"goal":"","amountCents":100}`, "não encontrada"},
			{"from/to ausentes", "transfer", `{"amountCents":100}`, "from: obrigatório; to: obrigatório"},
			{"from vazio", "transfer", `{"from":"","to":"Carteira","amountCents":100}`, "não encontrada"},
			{"to só espaços", "transfer", `{"from":"Nubank","to":"   ","amountCents":100}`, "não encontrada"},
			{"amount zero", "transfer", `{"from":"Nubank","to":"Carteira","amountCents":0}`, "amountCents: deve ser >= 1"},
			{"amount fracionado", "create_expense", `{"amountCents":10.5,"description":"x"}`, "amountCents: deve ser um número inteiro"},
			{"amount string", "create_expense", `{"amountCents":"100","description":"x"}`, "amountCents: deve ser um número inteiro"},
			{"descrição vazia", "create_expense", `{"amountCents":100,"description":""}`, "description: deve ter ao menos 1"},
			{"data mal formatada", "create_income", `{"amountCents":100,"description":"x","date":"27/11/2026"}`, "date: formato inválido"},
			{"pending não-bool", "create_expense", `{"amountCents":100,"description":"x","pending":"sim"}`, "pending: deve ser booleano"},
			{"parcelas 49", "create_card_expense", `{"card":"Inter","description":"x","totalCents":100,"installments":49}`, "installments: deve ser <= 48"},
			{"parcelas 0", "create_card_expense", `{"card":"Inter","description":"x","totalCents":100,"installments":0}`, "installments: deve ser >= 1"},
			{"mês 13", "pay_invoice", `{"card":"Inter","month":13,"year":2026,"account":"Nubank"}`, "month: deve ser <= 12"},
			{"mês 0 no período", "get_summary", `{"month":0}`, "month: deve ser >= 1"},
			{"pay_invoice faltando", "pay_invoice", `{"card":"Inter"}`, "month: obrigatório; year: obrigatório; account: obrigatório"},
			{"enum type", "list_transactions", `{"type":"OUTRO"}`, "type: deve ser um de"},
			{"enum kind", "list_categories", `{"kind":"expense"}`, "kind: deve ser um de"},
			{"page 0", "list_transactions", `{"page":0}`, "page: deve ser >= 1"},
			{"format ausente", "export_transactions", `{}`, "format: obrigatório"},
			{"format inválido", "export_transactions", `{"format":"pdf"}`, "format: deve ser um de"},
			{"set_budget amount negativo", "set_budget", `{"category":"Alimentação","amountCents":-1}`, "amountCents: deve ser >= 1"},
			{"arguments não-objeto", "list_accounts", `[1,2]`, "arguments deve ser um objeto"},
			{"tool desconhecida", "apagar_tudo", `{}`, "tool desconhecida"},
		}
		lines := make([]string, len(invalid))
		for i, tc := range invalid {
			lines[i] = callLine(i+1, tc.tool, tc.args)
		}
		resps := runLines(t, backend, lines...)
		if len(resps) != len(invalid) {
			t.Fatalf("esperava %d respostas, veio %d", len(invalid), len(resps))
		}
		for i, tc := range invalid {
			tr := toolResultOf(t, resps[i])
			if !tr.IsError || !strings.Contains(tr.Content[0].Text, tc.wantMsg) {
				t.Errorf("%s: esperava isError com %q, veio isError=%v %q", tc.name, tc.wantMsg, tr.IsError, tr.Content[0].Text)
			}
		}

		// Nada pode ter sido gravado pelas chamadas inválidas.
		after, err := backend.Transactions(ctx, domain.ListTransactionsQuery{})
		if err != nil || after.Total != before.Total {
			t.Fatalf("chamadas inválidas gravaram transações: %d -> %d (%v)", before.Total, after.Total, err)
		}
	})
}

func TestToolsCallHappyPath(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		backend := seededBackend(t, database)
		resps := runLines(t, backend,
			callLine(1, "create_card_expense", `{"card":"inter","description":"Fone MCP","totalCents":30000,"installments":3,"category":"Alimentação"}`),
			callLine(2, "transfer", `{"from":"Nubank","to":"carteira","amountCents":5000,"date":"2026-11-01"}`),
			callLine(3, "list_card_invoices", `{"card":"Inter Gold"}`),
			callLine(4, "contribute_goal", `{"goal":"macbook","amountCents":10000,"note":"via teste"}`),
			callLine(5, "list_transactions", `{"type":"TRANSFER","page":1}`),
			callLine(6, "export_transactions", `{"format":"csv"}`),
			callLine(7, "create_expense", `{"amountCents":4590,"description":"Café MCP","pending":true}`),
			// Inteiros escritos como 4590.0 / 1.2e3 são aceitos (como no zod) e normalizados.
			callLine(8, "create_expense", `{"amountCents":4590.0,"description":"Pão MCP"}`),
			callLine(9, "create_income", `{"amountCents":1.2e3,"description":"Pix MCP"}`),
		)
		if len(resps) != 9 {
			t.Fatalf("esperava 9 respostas, veio %d", len(resps))
		}
		wants := []string{`"created": 3`, `"to": "Carteira"`, `"cardId"`, `"MacBook Pro"`, `"TRANSFER"`, "Data;", "Café MCP", "Pão MCP", "Pix MCP"}
		for i, want := range wants {
			tr := toolResultOf(t, resps[i])
			if tr.IsError || !strings.Contains(tr.Content[0].Text, want) {
				t.Errorf("chamada %d: esperava sucesso contendo %q, veio isError=%v %q", i+1, want, tr.IsError, tr.Content[0].Text)
			}
		}
	})
}
