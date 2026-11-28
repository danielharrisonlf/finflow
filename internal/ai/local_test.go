package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"finflow/internal/domain"
	"finflow/internal/service"
)

func TestOllamaStreamingToolLoop(t *testing.T) {
	fake := newFakeLocal(t).on("/api/chat",
		ndjson(
			ollamaText("Vou "), ollamaText("consultar."),
			ollamaToolCalls([2]string{"get_summary", `{"month":11,"year":2026}`}, [2]string{"list_goals", `{}`}),
			ollamaDone("stop"),
		),
		ndjson(ollamaText("Seu saldo é "), ollamaText("**R$ 5.000,00**."), ollamaDone("stop")),
	)
	svc, svcs := newTestService(t, nil, false)
	ctx := context.Background()
	useLocal(t, svc, ProviderOllama, fake.server.URL, "", "")
	if _, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta", Type: domain.AccountChecking, InitialBalanceCents: 500000}); err != nil {
		t.Fatal(err)
	}

	rec := &recorder{}
	if err := svc.Chat(ctx, ChatInput{Messages: []ChatMessage{
		{Role: "assistant", Content: "Olá! Como posso ajudar?"},
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
	if done := rec.named("done"); len(done) != 1 || done[0].data["stopReason"] != "end_turn" {
		t.Fatalf("done: %+v", done)
	}

	calls := fake.calls("/api/chat")
	if len(calls) != 2 {
		t.Fatalf("esperava 2 chamadas ao Ollama, veio %d", len(calls))
	}
	first := calls[0].body
	if first["model"] != DefaultOllamaModel || first["stream"] != true {
		t.Fatalf("model/stream: %v %v", first["model"], first["stream"])
	}
	if opts := first["options"].(map[string]any); opts["num_ctx"] != float64(ollamaNumCtx) {
		t.Fatalf("options: %v", opts)
	}
	tools0 := first["tools"].([]any)
	if len(tools0) != len(toolSpecs()) || tools0[0].(map[string]any)["type"] != "function" {
		t.Fatalf("tools: %d", len(tools0))
	}
	msgs := first["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("saudação deveria ser descartada: %v", msgs)
	}
	system := msgs[0].(map[string]any)["content"].(string)
	if !strings.Contains(system, "Responda sempre em português do Brasil") || !strings.Contains(system, "Observações para este modelo") {
		t.Fatalf("system prompt: %q", system)
	}
	if user := msgs[1].(map[string]any)["content"].(string); !strings.Contains(user, "Hoje é") || !strings.HasSuffix(user, "Como estou este mês?") {
		t.Fatalf("primeiro turno do usuário: %q", user)
	}

	assistant := messagesWithRole(calls[1], "assistant")
	if len(assistant) != 1 {
		t.Fatalf("assistente ecoado: %v", assistant)
	}
	echoed := assistant[0]["tool_calls"].([]any)
	if len(echoed) != 2 || echoed[0].(map[string]any)["function"].(map[string]any)["arguments"].(map[string]any)["month"] != float64(11) {
		t.Fatalf("tool_calls ecoados: %v", echoed)
	}
	results := messagesWithRole(calls[1], "tool")
	if len(results) != 2 || results[0]["tool_name"] != "get_summary" || !strings.Contains(results[0]["content"].(string), "totalBalanceCents") {
		t.Fatalf("resultados das ferramentas: %v", results)
	}
}

func TestOllamaProposalAndInvalidArgumentsAsToolResults(t *testing.T) {
	ctx := context.Background()
	fake := newFakeLocal(t)
	svc, svcs := newTestService(t, nil, false)
	useLocal(t, svc, ProviderOllama, fake.server.URL, "llama3.1:8b", "")
	acc, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Nubank", Type: domain.AccountChecking})
	if err != nil {
		t.Fatal(err)
	}
	fake.on("/api/chat",
		ndjson(ollamaToolCalls(
			[2]string{"propose_transaction", `{"type":"EXPENSE","amountCents":4590,"description":"Mercado","date":"2026-11-20","accountId":"` + acc.ID + `"}`},
			[2]string{"can_afford", `{"amountCents":"muito"}`},
			[2]string{"inventada", `{}`},
		), ollamaDone("stop")),
		ndjson(ollamaText("Preparei o lançamento; confirme na tela."), ollamaDone("stop")),
	)
	rec := &recorder{}
	if err := svc.Chat(ctx, userAsks("lança 45,90 de mercado dia 20"), rec.emit); err != nil {
		t.Fatal(err)
	}
	props := rec.named("proposal")
	if len(props) != 1 || props[0].data["kind"] != ProposalCreateTransaction {
		t.Fatalf("proposal: %+v", rec.events)
	}
	results := messagesWithRole(fake.calls("/api/chat")[1], "tool")
	if len(results) != 3 || results[0]["content"] != proposalSentMessage ||
		!strings.Contains(results[1]["content"].(string), "INVALID_INPUT") ||
		!strings.Contains(results[2]["content"].(string), "ferramenta desconhecida") {
		t.Fatalf("resultados: %v", results)
	}
	txs, err := svcs.ListTransactions(ctx, domain.ListTransactionsQuery{Page: 1, PageSize: 10})
	if err != nil || txs.Total != 0 {
		t.Fatalf("o modelo nunca grava: total=%d err=%v", txs.Total, err)
	}
}

func TestOpenAICompatibleStreamingToolDeltasSplitAcrossChunks(t *testing.T) {
	fake := newFakeLocal(t)
	svc, _ := newTestService(t, nil, false)
	base := fake.server.URL + "/v1"
	key := "lm-studio-key"
	if _, err := svc.SaveConfig(context.Background(), ConfigInput{
		Provider: ProviderOpenAICompatible, BaseURL: &base, Model: ptr("qwen2.5-7b-instruct"),
		APIKey: domain.Optional[string]{Set: true, Valid: true, Value: key},
	}); err != nil {
		t.Fatal(err)
	}
	fake.on("/v1/chat/completions",
		oaSSE(
			oaDelta(map[string]any{"role": "assistant", "content": "Vou ver"}, nil),
			oaDelta(map[string]any{"content": "."}, nil),
			oaToolDelta(0, "call_a", "get_summary", ""),
			oaToolDelta(0, "", "", `{"month":`),
			oaToolDelta(0, "", "", `11}`),
			oaToolDelta(1, "call_b", "list_goals", `{}`),
			oaDelta(map[string]any{}, "tool_calls"),
		),
		oaSSE(oaDelta(map[string]any{"content": "Tudo certo."}, nil), oaDelta(map[string]any{}, "stop")),
	)
	rec := &recorder{}
	if err := svc.Chat(context.Background(), userAsks("como estou?"), rec.emit); err != nil {
		t.Fatal(err)
	}
	if got := rec.text(); got != "Vou ver.Tudo certo." {
		t.Fatalf("texto: %q", got)
	}
	if tools := rec.named("tool"); len(tools) != 2 || tools[1].data["name"] != "list_goals" {
		t.Fatalf("tools: %+v", tools)
	}
	calls := fake.calls("/v1/chat/completions")
	if len(calls) != 2 {
		t.Fatalf("chamadas: %d", len(calls))
	}
	if calls[0].header.Get("Authorization") != "Bearer "+key || calls[0].body["stream"] != true || calls[0].body["model"] != "qwen2.5-7b-instruct" {
		t.Fatalf("requisição: %v %v", calls[0].header, calls[0].body)
	}
	echoed := messagesWithRole(calls[1], "assistant")[0]["tool_calls"].([]any)
	first := echoed[0].(map[string]any)
	if len(echoed) != 2 || first["id"] != "call_a" || first["function"].(map[string]any)["arguments"] != `{"month":11}` {
		t.Fatalf("tool_calls acumulados: %v", echoed)
	}
	results := messagesWithRole(calls[1], "tool")
	if len(results) != 2 || results[0]["tool_call_id"] != "call_a" || results[1]["tool_call_id"] != "call_b" {
		t.Fatalf("resultados: %v", results)
	}
	if done := rec.named("done"); len(done) != 1 || done[0].data["stopReason"] != "end_turn" {
		t.Fatalf("done: %+v", done)
	}
}

func TestLocalToolCallAsJSONInContent(t *testing.T) {
	fake := newFakeLocal(t).on("/api/chat",
		ndjson(ollamaText("```json\n{\"name\": \"list_goals\", "), ollamaText("\"arguments\": {}}\n```"), ollamaDone("stop")),
		ndjson(ollamaText("Você não tem metas."), ollamaDone("stop")),
	)
	svc, _ := newTestService(t, nil, false)
	useLocal(t, svc, ProviderOllama, fake.server.URL, "", "")
	rec := &recorder{}
	if err := svc.Chat(context.Background(), userAsks("quais minhas metas?"), rec.emit); err != nil {
		t.Fatal(err)
	}
	if got := rec.text(); got != "Você não tem metas." {
		t.Fatalf("o JSON da chamada não deve aparecer: %q", got)
	}
	if tools := rec.named("tool"); len(tools) != 1 || tools[0].data["name"] != "list_goals" {
		t.Fatalf("tools: %+v", tools)
	}
	results := messagesWithRole(fake.calls("/api/chat")[1], "tool")
	if len(results) != 1 || results[0]["tool_name"] != "list_goals" {
		t.Fatalf("resultado: %v", results)
	}

	// JSON que não é chamada de ferramenta conhecida volta como texto normal.
	fake.on("/api/chat", ndjson(ollamaText(`{"nome": "teste"}`), ollamaDone("stop")))
	rec = &recorder{}
	if err := svc.Chat(context.Background(), userAsks("me mostre um json"), rec.emit); err != nil {
		t.Fatal(err)
	}
	if got := rec.text(); got != `{"nome": "teste"}` {
		t.Fatalf("texto: %q", got)
	}
}

func TestParseContentToolCall(t *testing.T) {
	cases := map[string]bool{
		`{"name":"get_summary","arguments":{"month":3}}`:                true,
		`<tool_call>{"name":"list_goals","arguments":"{}"}</tool_call>`: true,
		`{"name":"get_summary","parameters":{}}`:                        true,
		`{"name":"get_summary","arguments":{}} {"name":"list_goals"}`:   false,
		`{"name":"rm_rf","arguments":{}}`:                               false,
		`Claro! {"name":"get_summary","arguments":{}}`:                  false,
		"```\n{\"name\":\"get_insights\",\"arguments\":null}\n```":      true,
		`{"name":"get_summary","arguments":{"month":3}`:                 false,
	}
	for in, want := range cases {
		call, ok := parseContentToolCall(in)
		if ok != want {
			t.Errorf("%q: ok=%v, esperava %v", in, ok, want)
		}
		if ok && !json.Valid(call.input) {
			t.Errorf("%q: argumentos inválidos %q", in, call.input)
		}
	}
	if call, _ := parseContentToolCall(`{"name":"get_summary","arguments":"{\"month\":3}"}`); string(call.input) != `{"month":3}` {
		t.Fatalf("argumentos em string: %q", call.input)
	}
}

func TestLocalToolRoundCap(t *testing.T) {
	fake := newFakeLocal(t).onAlways("/api/chat", ndjson(ollamaToolCalls([2]string{"list_goals", `{}`}), ollamaDone("stop")))
	svc, _ := newTestService(t, nil, false)
	useLocal(t, svc, ProviderOllama, fake.server.URL, "", "")
	rec := &recorder{}
	if err := svc.Chat(context.Background(), userAsks("loop"), rec.emit); err != nil {
		t.Fatal(err)
	}
	if n := len(fake.calls("/api/chat")); n != localMaxToolRounds+1 {
		t.Fatalf("esperava %d chamadas (8 rodadas + a que estourou), veio %d", localMaxToolRounds+1, n)
	}
	if n := len(rec.named("tool")); n != localMaxToolRounds+1 {
		t.Fatalf("eventos tool: %d", n)
	}
	errs := rec.named("error")
	if len(errs) != 1 || errs[0].data["message"] != maxRoundsMessage {
		t.Fatalf("error: %+v", errs)
	}
	if done := rec.named("done"); len(done) != 1 || done[0].data["stopReason"] != "max_rounds" {
		t.Fatalf("done: %+v", done)
	}
}

func TestLocalTruncatedTurnSkipsTools(t *testing.T) {
	fake := newFakeLocal(t).on("/api/chat", ndjson(ollamaToolCalls([2]string{"list_goals", `{}`}), ollamaDone("length")))
	svc, _ := newTestService(t, nil, false)
	useLocal(t, svc, ProviderOllama, fake.server.URL, "", "")
	rec := &recorder{}
	if err := svc.Chat(context.Background(), userAsks("x"), rec.emit); err != nil {
		t.Fatal(err)
	}
	if done := rec.named("done"); len(done) != 1 || done[0].data["stopReason"] != "max_tokens" || len(fake.calls("/api/chat")) != 1 {
		t.Fatalf("turno truncado não executa ferramentas: %+v", rec.events)
	}
}

func appStatus(t *testing.T, err error) *service.AppError {
	t.Helper()
	var appErr *service.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("esperava AppError, veio %v", err)
	}
	return appErr
}

func TestLocalProviderErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("servidor fora do ar", func(t *testing.T) {
		svc, _ := newTestService(t, nil, false)
		useLocal(t, svc, ProviderOllama, closedURL(t), "", "")
		rec := &recorder{}
		appErr := appStatus(t, svc.Chat(ctx, userAsks("oi"), rec.emit))
		if appErr.Status != http.StatusBadGateway || !strings.Contains(appErr.Message, "Não foi possível conectar ao Ollama") || len(rec.events) != 0 {
			t.Fatalf("erro: %+v eventos=%v", appErr, rec.events)
		}
	})

	t.Run("modelo não instalado", func(t *testing.T) {
		fake := newFakeLocal(t).on("/api/chat", localResp{status: 404, body: `{"error":"model \"qwen2.5:7b\" not found, try pulling it first"}`})
		svc, _ := newTestService(t, nil, false)
		useLocal(t, svc, ProviderOllama, fake.server.URL, "", "")
		appErr := appStatus(t, svc.Chat(ctx, userAsks("oi"), (&recorder{}).emit))
		if appErr.Status != http.StatusBadGateway || !strings.Contains(appErr.Message, "ollama pull qwen2.5:7b") {
			t.Fatalf("erro: %+v", appErr)
		}
	})

	t.Run("modelo sem ferramentas", func(t *testing.T) {
		fake := newFakeLocal(t).on("/api/chat", localResp{status: 400, body: `{"error":"registry.ollama.ai/library/gemma:2b does not support tools"}`})
		svc, _ := newTestService(t, nil, false)
		useLocal(t, svc, ProviderOllama, fake.server.URL, "gemma:2b", "")
		appErr := appStatus(t, svc.Chat(ctx, userAsks("oi"), (&recorder{}).emit))
		if !strings.Contains(appErr.Message, "gemma:2b não aceita ferramentas") || !strings.Contains(appErr.Message, DefaultOllamaModel) {
			t.Fatalf("erro: %+v", appErr)
		}
	})

	t.Run("erro no meio do stream", func(t *testing.T) {
		fake := newFakeLocal(t).on("/api/chat", ndjson(ollamaText("Começando"), map[string]any{"error": "sem memória"}))
		svc, _ := newTestService(t, nil, false)
		useLocal(t, svc, ProviderOllama, fake.server.URL, "", "")
		rec := &recorder{}
		appErr := appStatus(t, svc.Chat(ctx, userAsks("oi"), rec.emit))
		if !strings.Contains(appErr.Message, "sem memória") || rec.text() != "Começando" {
			t.Fatalf("erro: %+v texto=%q", appErr, rec.text())
		}
	})

	t.Run("chave recusada no servidor compatível", func(t *testing.T) {
		fake := newFakeLocal(t).on("/v1/chat/completions", localResp{status: 401, body: `{"error":{"message":"invalid api key"}}`})
		svc, _ := newTestService(t, nil, false)
		useLocal(t, svc, ProviderOpenAICompatible, fake.server.URL+"/v1", "local-model", "")
		appErr := appStatus(t, svc.Chat(ctx, userAsks("oi"), (&recorder{}).emit))
		if appErr.Status != http.StatusBadGateway || !strings.Contains(appErr.Message, "recusou a chave") {
			t.Fatalf("erro: %+v", appErr)
		}
	})

	t.Run("resposta vazia", func(t *testing.T) {
		fake := newFakeLocal(t).on("/api/chat", ndjson(ollamaDone("stop")))
		svc, _ := newTestService(t, nil, false)
		useLocal(t, svc, ProviderOllama, fake.server.URL, "", "")
		rec := &recorder{}
		if err := svc.Chat(ctx, userAsks("oi"), rec.emit); err != nil {
			t.Fatal(err)
		}
		if errs := rec.named("error"); len(errs) != 1 || errs[0].data["message"] != emptyAnswerMessage {
			t.Fatalf("eventos: %+v", rec.events)
		}
	})
}

func TestLocalReceiptWithOllamaVision(t *testing.T) {
	ctx := context.Background()
	out, _ := json.Marshal(map[string]any{
		"description": "Supermercado Bom Preço", "amountCents": 12345, "date": "2026-11-25", "kind": "EXPENSE",
		"categoryName": "alimentacao", "merchant": "BOM PRECO LTDA", "installments": nil, "confidence": "HIGH",
	})
	fake := newFakeLocal(t).on("/api/chat", jsonResp(map[string]any{
		"model": "qwen2.5vl:7b", "message": map[string]any{"role": "assistant", "content": string(out)}, "done": true, "done_reason": "stop",
	}))
	svc, svcs := newTestService(t, nil, false)

	// Sem modelo de visão: 412 e receiptSupported false.
	useLocal(t, svc, ProviderOllama, fake.server.URL, "", "")
	if st, _ := svc.Status(ctx); st.ReceiptSupported {
		t.Fatal("sem modelo de visão não lê comprovantes")
	}
	if appErr := appStatus(t, func() error { _, err := svc.ExtractReceipt(ctx, pngHeader, "image/png"); return err }()); appErr.Status != http.StatusPreconditionFailed {
		t.Fatalf("sem visão: %+v", appErr)
	}

	useLocal(t, svc, ProviderOllama, fake.server.URL, "", SuggestedVisionModel)
	if st, _ := svc.Status(ctx); !st.ReceiptSupported || st.VisionModel == nil || *st.VisionModel != SuggestedVisionModel {
		t.Fatalf("status com visão: %+v", st)
	}
	cat, err := svcs.CreateCategory(ctx, domain.CreateCategoryInput{Name: "Alimentação", Kind: domain.CategoryExpense, Icon: "utensils", Color: "#22c55e"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.ExtractReceipt(ctx, pngHeader, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if res.AmountCents != 12345 || res.SuggestedCategoryID == nil || *res.SuggestedCategoryID != cat.ID || res.Confidence != "HIGH" {
		t.Fatalf("resultado: %+v", res)
	}
	req := fake.calls("/api/chat")[0].body
	if req["model"] != SuggestedVisionModel || req["stream"] != false || req["format"] == nil {
		t.Fatalf("requisição: %v", req)
	}
	user := req["messages"].([]any)[1].(map[string]any)
	if imgs := user["images"].([]any); len(imgs) != 1 || imgs[0] != base64.StdEncoding.EncodeToString(pngHeader) {
		t.Fatalf("imagem: %v", user["images"])
	}
	if !strings.Contains(user["content"].(string), "Alimentação") {
		t.Fatal("categorias deveriam ir no pedido")
	}

	if appErr := appStatus(t, func() error { _, err := svc.ExtractReceipt(ctx, []byte("%PDF-1.4\n"), "application/pdf"); return err }()); appErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("PDF no modelo local: %+v", appErr)
	}
}

func TestLocalReceiptWithOpenAICompatibleFallsBackWithoutSchema(t *testing.T) {
	ctx := context.Background()
	fake := newFakeLocal(t).on("/v1/chat/completions",
		localResp{status: 400, body: `{"error":{"message":"response_format not supported"}}`},
		jsonResp(map[string]any{"choices": []any{map[string]any{
			"message":       map[string]any{"role": "assistant", "content": "Aqui está:\n```json\n{\"description\":\"Farmácia\",\"amountCents\":2990,\"date\":\"2026-11-26\",\"kind\":\"EXPENSE\",\"categoryName\":null,\"merchant\":\"Drogaria\",\"installments\":null,\"confidence\":\"MEDIUM\"}\n```"},
			"finish_reason": "stop",
		}}}),
	)
	svc, _ := newTestService(t, nil, false)
	useLocal(t, svc, ProviderOpenAICompatible, fake.server.URL+"/v1", "texto", "llava")
	res, err := svc.ExtractReceipt(ctx, pngHeader, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if res.AmountCents != 2990 || res.Description != "Farmácia" || res.Confidence != "MEDIUM" {
		t.Fatalf("resultado: %+v", res)
	}
	calls := fake.calls("/v1/chat/completions")
	if len(calls) != 2 || calls[0].body["response_format"] == nil || calls[1].body["response_format"] != nil {
		t.Fatalf("esperava tentativa com schema e outra sem: %d", len(calls))
	}
	if calls[0].body["model"] != "llava" {
		t.Fatalf("modelo de visão: %v", calls[0].body["model"])
	}
	parts := calls[0].body["messages"].([]any)[1].(map[string]any)["content"].([]any)
	img := parts[1].(map[string]any)["image_url"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(img, "data:image/png;base64,") {
		t.Fatalf("data URL: %q", img)
	}
}

func ptr(s string) *string { return &s }
