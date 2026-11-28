package ai

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

const testKey = "sk-ant-test-0000000000000000000000"

// newTestService monta o serviço com banco SQLite em memória, sem credenciais do ambiente (os testes
// nunca usam ANTHROPIC_API_KEY real) e apontado para o servidor falso.
func newTestService(t *testing.T, api *fakeAPI, withKey bool) (*Service, *service.Services) {
	t.Helper()
	svcs := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	zero := 0
	cfg := Config{
		EnvCredentials: func() bool { return false }, MaxRetries: &zero,
		// Sem FINFLOW_AI_* herdado e sem procurar um Ollama de verdade em localhost:11434.
		Getenv: func(string) string { return "" }, DefaultOllamaURL: closedURL(t),
	}
	if api != nil {
		cfg.BaseURL = api.server.URL
	}
	svc := NewService(svcs, cfg)
	if withKey {
		key := testKey
		if _, err := svc.SaveKey(context.Background(), &key); err != nil {
			t.Fatalf("salvar chave: %v", err)
		}
	}
	return svc, svcs
}

func userAsks(q string) ChatInput {
	return ChatInput{Messages: []ChatMessage{{Role: "user", Content: q}}}
}

func TestChatToolLoopParallelTools(t *testing.T) {
	api := newFakeAPI(t,
		newTurn().thinking().text("Vou consultar.").
			toolUse("toolu_1", "get_summary", `{"month": 11, "year": 2026}`).
			toolUse("toolu_2", "list_goals", "").
			stop("tool_use"),
		newTurn().text("Seu saldo é ", "**R$ 5.000,00**.").stop("end_turn"),
	)
	svc, svcs := newTestService(t, api, true)
	ctx := context.Background()
	if _, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta", Type: domain.AccountChecking, InitialBalanceCents: 500000}); err != nil {
		t.Fatal(err)
	}

	rec := &recorder{}
	if err := svc.Chat(ctx, ChatInput{Messages: []ChatMessage{
		{Role: "assistant", Content: "Olá! Como posso ajudar?"}, // saudação inicial é descartada
		{Role: "user", Content: "Como estou este mês?"},
	}}, rec.emit); err != nil {
		t.Fatalf("chat: %v", err)
	}

	tools := rec.named("tool")
	if len(tools) != 2 || tools[0].data["name"] != "get_summary" || tools[0].data["label"] != "Consultando seu resumo do mês" {
		t.Fatalf("eventos tool: %+v", tools)
	}
	if got := rec.text(); got != "Vou consultar.Seu saldo é **R$ 5.000,00**." {
		t.Fatalf("texto: %q", got)
	}
	done := rec.named("done")
	if len(done) != 1 || done[0].data["stopReason"] != "end_turn" {
		t.Fatalf("done: %+v", done)
	}

	calls := api.calls()
	if len(calls) != 2 {
		t.Fatalf("esperava 2 chamadas à API, veio %d", len(calls))
	}
	first := calls[0]
	if first.body["model"] != Model || first.body["stream"] != true {
		t.Fatalf("model/stream: %v %v", first.body["model"], first.body["stream"])
	}
	if first.body["fallbacks"] != "default" {
		t.Fatalf("fallbacks: %v", first.body["fallbacks"])
	}
	if !strings.Contains(first.header.Get("anthropic-beta"), "server-side-fallback-2026-07-01") {
		t.Fatalf("beta: %q", first.header.Get("anthropic-beta"))
	}
	if oc, _ := first.body["output_config"].(map[string]any); oc["effort"] != "medium" {
		t.Fatalf("effort: %v", first.body["output_config"])
	}
	for _, absent := range []string{"thinking", "tool_choice", "temperature"} {
		if _, ok := first.body[absent]; ok {
			t.Fatalf("%s não deveria ser enviado", absent)
		}
	}
	system := first.body["system"].([]any)[0].(map[string]any)
	if system["cache_control"] == nil || strings.Contains(system["text"].(string), "Hoje é") {
		t.Fatalf("system deve ser congelado e com cache_control: %v", system)
	}
	if first.body["cache_control"] == nil {
		t.Fatal("cache automático do fim da conversa ausente")
	}
	tools0 := first.body["tools"].([]any)
	prev := ""
	for _, raw := range tools0 {
		tool := raw.(map[string]any)
		name := tool["name"].(string)
		if name < prev {
			t.Fatalf("tools fora de ordem: %s antes de %s", prev, name)
		}
		prev = name
		schema := tool["input_schema"].(map[string]any)
		if tool["strict"] != true || tool["eager_input_streaming"] != true || schema["additionalProperties"] != false {
			t.Fatalf("tool %s sem strict/eager/additionalProperties: %v", name, tool)
		}
	}
	msgs := first.body["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("saudação do assistente deveria ser descartada: %v", msgs)
	}
	firstUser := msgs[0].(map[string]any)["content"].([]any)
	if !strings.Contains(firstUser[0].(map[string]any)["text"].(string), "Hoje é") {
		t.Fatalf("contexto de data deveria estar no primeiro turno do usuário: %v", firstUser)
	}

	// Segunda chamada: assistente ecoado (com thinking) + UM turno do usuário com os dois tool_result.
	msgs2 := calls[1].body["messages"].([]any)
	if len(msgs2) != 3 {
		t.Fatalf("mensagens na 2ª chamada: %d", len(msgs2))
	}
	assistant := msgs2[1].(map[string]any)["content"].([]any)
	if assistant[0].(map[string]any)["type"] != "thinking" {
		t.Fatalf("thinking deve ser reenviado sem alteração: %v", assistant[0])
	}
	results := msgs2[2].(map[string]any)["content"].([]any)
	if len(results) != 2 {
		t.Fatalf("tool_results: %v", results)
	}
	for i, id := range []string{"toolu_1", "toolu_2"} {
		r := results[i].(map[string]any)
		if r["type"] != "tool_result" || r["tool_use_id"] != id || r["is_error"] == true {
			t.Fatalf("tool_result %d: %v", i, r)
		}
	}
	if !strings.Contains(calls[1].raw, "totalBalanceCents") {
		t.Fatal("resultado de get_summary deveria ir como JSON")
	}
}

func TestChatRefusal(t *testing.T) {
	api := newFakeAPI(t, newTurn().text("Parcial").stop("refusal"))
	svc, _ := newTestService(t, api, true)
	rec := &recorder{}
	if err := svc.Chat(context.Background(), userAsks("pergunta recusada"), rec.emit); err != nil {
		t.Fatal(err)
	}
	errs := rec.named("error")
	if len(errs) != 1 || errs[0].data["message"] != refusalMessage {
		t.Fatalf("error: %+v", errs)
	}
	if done := rec.named("done"); len(done) != 1 || done[0].data["stopReason"] != "refusal" {
		t.Fatalf("done: %+v", done)
	}
	if len(api.calls()) != 1 {
		t.Fatal("recusa não deve continuar o loop")
	}
}

func TestChatProposalNeverWrites(t *testing.T) {
	ctx := context.Background()
	svc, svcs := newTestService(t, nil, true)
	acc, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Nubank", Type: domain.AccountChecking})
	if err != nil {
		t.Fatal(err)
	}
	api := newFakeAPI(t,
		newTurn().toolUse("toolu_p", "propose_transaction",
			`{"type":"EXPENSE","amountCents":4590,"description":"Mercado","date":"2026-11-20","accountId":"`+acc.ID+`"}`).
			stop("tool_use"),
		newTurn().text("Preparei o lançamento; confirme na tela.").stop("end_turn"),
	)
	svc.cfg.BaseURL = api.server.URL

	rec := &recorder{}
	if err := svc.Chat(ctx, userAsks("lança 45,90 de mercado dia 20"), rec.emit); err != nil {
		t.Fatal(err)
	}
	props := rec.named("proposal")
	if len(props) != 1 {
		t.Fatalf("proposal: %+v", rec.events)
	}
	p := props[0].data
	payload := p["payload"].(map[string]any)
	if p["kind"] != ProposalCreateTransaction || p["id"] == "" || payload["accountId"] != acc.ID ||
		payload["amountCents"] != float64(4590) || payload["status"] != "CONFIRMED" || payload["type"] != "EXPENSE" {
		t.Fatalf("proposta: %+v", p)
	}
	if !strings.Contains(p["summary"].(string), "R$ 45,90") || !strings.Contains(p["summary"].(string), "Nubank") {
		t.Fatalf("summary: %v", p["summary"])
	}
	result := api.calls()[1].body["messages"].([]any)[2].(map[string]any)["content"].([]any)[0].(map[string]any)
	if result["content"] != proposalSentMessage && !strings.Contains(api.calls()[1].raw, proposalSentMessage) {
		t.Fatalf("tool_result da proposta: %v", result)
	}
	txs, err := svcs.ListTransactions(ctx, domain.ListTransactionsQuery{Page: 1, PageSize: 10})
	if err != nil || txs.Total != 0 {
		t.Fatalf("o modelo nunca grava: total=%d err=%v", txs.Total, err)
	}
}

func TestChatInvalidToolInputReturnsError(t *testing.T) {
	api := newFakeAPI(t,
		newTurn().
			toolUse("toolu_bad", "can_afford", `{"amountCents":"muito"}`).
			toolUse("toolu_unknown", "get_summary", `{"mes":11}`).
			toolUse("toolu_missing", "propose_goal_contribution", `{"goalId":"11111111-1111-4111-8111-111111111111","amountCents":1000}`).
			stop("tool_use"),
		newTurn().text("Desculpe.").stop("end_turn"),
	)
	svc, _ := newTestService(t, api, true)
	rec := &recorder{}
	if err := svc.Chat(context.Background(), userAsks("posso?"), rec.emit); err != nil {
		t.Fatal(err)
	}
	results := api.calls()[1].body["messages"].([]any)[2].(map[string]any)["content"].([]any)
	if len(results) != 3 {
		t.Fatalf("results: %v", results)
	}
	for _, raw := range results {
		r := raw.(map[string]any)
		if r["is_error"] != true {
			t.Fatalf("esperava is_error: %v", r)
		}
	}
	if len(rec.named("proposal")) != 0 {
		t.Fatal("meta inexistente não deve gerar proposta")
	}
}

func TestChatMaxTokensSkipsTools(t *testing.T) {
	api := newFakeAPI(t, newTurn().toolUse("toolu_1", "propose_wish", `{"name":"Celu`).stop("max_tokens"))
	svc, _ := newTestService(t, api, true)
	rec := &recorder{}
	if err := svc.Chat(context.Background(), userAsks("adiciona um celular"), rec.emit); err != nil {
		t.Fatal(err)
	}
	if len(rec.named("proposal")) != 0 || len(api.calls()) != 1 {
		t.Fatal("turno truncado não executa ferramentas")
	}
	if done := rec.named("done"); len(done) != 1 || done[0].data["stopReason"] != "max_tokens" {
		t.Fatalf("done: %+v", done)
	}
}

func TestChatNotConfigured(t *testing.T) {
	api := newFakeAPI(t)
	svc, _ := newTestService(t, api, false)
	rec := &recorder{}
	err := svc.Chat(context.Background(), userAsks("oi"), rec.emit)
	var appErr *service.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusPreconditionFailed {
		t.Fatalf("esperava 412, veio %v", err)
	}
	if len(api.calls()) != 0 || len(rec.events) != 0 {
		t.Fatal("sem chave não chama a API nem emite eventos")
	}
}

func TestChatUpstreamErrorIsTranslated(t *testing.T) {
	api := newFakeAPI(t) // sem respostas: o falso devolve 500
	svc, _ := newTestService(t, api, true)
	err := svc.Chat(context.Background(), userAsks("oi"), (&recorder{}).emit)
	var appErr *service.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusBadGateway {
		t.Fatalf("esperava 502 traduzido, veio %v", err)
	}
}

func TestValidateChatInput(t *testing.T) {
	cases := map[string]ChatInput{
		"vazio":          {},
		"role inválido":  {Messages: []ChatMessage{{Role: "system", Content: "x"}}},
		"termina no bot": {Messages: []ChatMessage{{Role: "user", Content: "a"}, {Role: "assistant", Content: "b"}}},
		"user vazio":     {Messages: []ChatMessage{{Role: "user", Content: "  "}}},
	}
	for name, in := range cases {
		if len(ValidateChatInput(in)) == 0 {
			t.Errorf("%s: esperava issue", name)
		}
	}
	if issues := ValidateChatInput(userAsks("ok")); len(issues) != 0 {
		t.Fatalf("válido: %v", issues)
	}
}

func TestStatusAndSaveKey(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t, nil, false)
	st, err := svc.Status(ctx)
	if err != nil || st.Configured || st.Source != nil || st.Model != nil || st.Provider != nil {
		t.Fatalf("status inicial: %+v %v", st, err)
	}
	bad := "curta"
	if _, err := svc.SaveKey(ctx, &bad); err == nil {
		t.Fatal("chave curta deveria falhar")
	}
	key := "  " + testKey + "  "
	st, err = svc.SaveKey(ctx, &key)
	if err != nil || !st.Configured || *st.Source != SourceSettings {
		t.Fatalf("após salvar: %+v %v", st, err)
	}
	if saved, _ := svc.savedKey(ctx); saved != testKey {
		t.Fatalf("chave salva sem trim: %q", saved)
	}
	st, err = svc.SaveKey(ctx, nil)
	if err != nil || st.Configured {
		t.Fatalf("após limpar: %+v %v", st, err)
	}
	svc.cfg.EnvCredentials = func() bool { return true }
	if st, _ = svc.Status(ctx); !st.Configured || *st.Source != SourceEnv {
		t.Fatalf("env: %+v", st)
	}
}

func TestEchoContentAfterMidStreamFallback(t *testing.T) {
	var msg anthropic.BetaMessage
	raw := `{"id":"m","type":"message","role":"assistant","model":"claude-opus-4-8","stop_reason":"tool_use",
	  "usage":{"input_tokens":1,"output_tokens":1},
	  "content":[
	    {"type":"thinking","thinking":"","signature":"a"},
	    {"type":"tool_use","id":"toolu_old","name":"list_goals","input":{}},
	    {"type":"text","text":"parcial"},
	    {"type":"fallback","from":{"model":"claude-opus-5-5"},"to":{"model":"claude-opus-4-8"},"trigger":{"type":"refusal","category":"cyber"}},
	    {"type":"tool_use","id":"toolu_new","name":"list_goals","input":{}}
	  ]}`
	if err := msg.UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	echo, calls := echoContent(msg)
	if len(calls) != 1 || calls[0].id != "toolu_new" {
		t.Fatalf("só chamadas depois do fallback: %+v", calls)
	}
	content := echo.OfAssistant.Content
	if len(content) != 3 || content[0].OfText == nil || content[1].OfFallback == nil || content[2].OfToolUse == nil {
		t.Fatalf("eco: %+v", content)
	}
}
