package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"finflow/internal/ai"
	"finflow/internal/db"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

// fakeMessagesAPI devolve, a cada chamada, a próxima resposta roteirizada (SSE ou JSON). Nenhum teste
// fala com a API real: o serviço é trocado via newAIService e ignora credenciais do ambiente.
func fakeMessagesAPI(t *testing.T, responses ...string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		i := int(calls.Add(1)) - 1
		if i >= len(responses) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if strings.HasPrefix(responses[i], "event:") {
			w.Header().Set("Content-Type", "text/event-stream")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		_, _ = io.WriteString(w, responses[i])
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// closedServerURL devolve uma URL onde nada escuta (nenhum teste procura um Ollama de verdade).
func closedServerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

func sse(name string, data any) string {
	b, _ := json.Marshal(data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", name, b)
}

func textTurnSSE(text string) string {
	return sse("message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": "msg", "type": "message", "role": "assistant", "model": ai.Model, "content": []any{},
		"stop_reason": nil, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}}}) +
		sse("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}}) +
		sse("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": text}}) +
		sse("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0}) +
		sse("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}, "usage": map[string]any{"output_tokens": 5}}) +
		sse("message_stop", map[string]any{"type": "message_stop"})
}

func newAITestServer(t *testing.T, apiURL string) *Server {
	t.Helper()
	return newAITestServerWith(t, apiURL, closedServerURL(t), nil)
}

// newAITestServerWith permite apontar o Ollama "deste computador" para um falso e simular FINFLOW_AI_*.
func newAITestServerWith(t *testing.T, apiURL, ollamaURL string, env map[string]string) *Server {
	t.Helper()
	t.Setenv("FINFLOW_PASSWORD", "")
	prev := newAIService
	zero := 0
	newAIService = func(svcs *service.Services) *ai.Service {
		return ai.NewService(svcs, ai.Config{
			BaseURL: apiURL, EnvCredentials: func() bool { return false }, MaxRetries: &zero,
			Getenv: func(k string) string { return env[k] }, DefaultOllamaURL: ollamaURL,
		})
	}
	t.Cleanup(func() { newAIService = prev })
	svcs := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	return NewServer(svcs, ServerOptions{})
}

func aiReq(t *testing.T, srv http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func multipartReq(t *testing.T, srv http.Handler, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(content)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/ai/receipt", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestAIStatusAndKey(t *testing.T) {
	srv := newAITestServer(t, "http://127.0.0.1:1")
	rr := aiReq(t, srv, http.MethodGet, "/api/ai/status", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"configured":false`) || !strings.Contains(rr.Body.String(), `"source":null`) {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
	if rr := aiReq(t, srv, http.MethodPut, "/api/ai/key", map[string]any{"apiKey": "curta"}); rr.Code != 400 {
		t.Fatalf("chave inválida: %d %s", rr.Code, rr.Body.String())
	}
	key := "sk-ant-api03-abcdefghijklmnopqrstuvwxyz"
	rr = aiReq(t, srv, http.MethodPut, "/api/ai/key", map[string]any{"apiKey": key})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"source":"SETTINGS"`) || strings.Contains(rr.Body.String(), key) {
		t.Fatalf("salvar chave: %d %s", rr.Code, rr.Body.String())
	}
	// A chave nunca aparece em /api/settings.
	if rr := aiReq(t, srv, http.MethodGet, "/api/settings", nil); strings.Contains(rr.Body.String(), key) {
		t.Fatalf("chave vazou em /api/settings: %s", rr.Body.String())
	}
	rr = aiReq(t, srv, http.MethodPut, "/api/ai/key", map[string]any{"apiKey": nil})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"configured":false`) {
		t.Fatalf("limpar chave: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAIChatHandler(t *testing.T) {
	api, calls := fakeMessagesAPI(t, textTurnSSE("Olá, tudo certo."))
	srv := newAITestServer(t, api.URL)

	body := map[string]any{"messages": []map[string]string{{"role": "user", "content": "oi"}}}
	rr := aiReq(t, srv, http.MethodPost, "/api/ai/chat", body)
	if rr.Code != http.StatusPreconditionFailed || !strings.Contains(rr.Body.String(), "ANTHROPIC_API_KEY") {
		t.Fatalf("sem chave deveria ser 412: %d %s", rr.Code, rr.Body.String())
	}
	if calls.Load() != 0 {
		t.Fatal("sem chave não chama a API")
	}

	if rr := aiReq(t, srv, http.MethodPost, "/api/ai/chat", map[string]any{"messages": []any{}}); rr.Code != 400 {
		t.Fatalf("conversa vazia: %d", rr.Code)
	}

	aiReq(t, srv, http.MethodPut, "/api/ai/key", map[string]any{"apiKey": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz"})
	rr = aiReq(t, srv, http.MethodPost, "/api/ai/chat", body)
	if rr.Code != 200 || !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("chat: %d %s %s", rr.Code, rr.Header().Get("Content-Type"), rr.Body.String())
	}
	out := rr.Body.String()
	if !strings.Contains(out, "event: text\ndata: {\"delta\":\"Olá, tudo certo.\"}\n\n") ||
		!strings.Contains(out, "event: done\ndata: {\"stopReason\":\"end_turn\"}\n\n") {
		t.Fatalf("eventos SSE: %q", out)
	}

	// Falha da API depois de configurado: ainda antes do primeiro evento → JSON 502 (nunca 401).
	rr = aiReq(t, srv, http.MethodPost, "/api/ai/chat", body)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("erro da API: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAIReceiptHandler(t *testing.T) {
	result, _ := json.Marshal(map[string]any{
		"description": "Farmácia", "amountCents": 2990, "date": "2026-11-26", "kind": "EXPENSE",
		"categoryName": nil, "merchant": "Drogaria", "installments": nil, "confidence": "MEDIUM",
	})
	msg, _ := json.Marshal(map[string]any{
		"id": "msg", "type": "message", "role": "assistant", "model": ai.Model,
		"content": []any{map[string]any{"type": "text", "text": string(result)}}, "stop_reason": "end_turn",
		"usage": map[string]any{"input_tokens": 1, "output_tokens": 1},
	})
	api, calls := fakeMessagesAPI(t, string(msg))
	srv := newAITestServer(t, api.URL)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00")

	if rr := multipartReq(t, srv, "nota.png", png); rr.Code != http.StatusPreconditionFailed {
		t.Fatalf("sem chave: %d %s", rr.Code, rr.Body.String())
	}
	aiReq(t, srv, http.MethodPut, "/api/ai/key", map[string]any{"apiKey": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz"})

	heic := append([]byte{0, 0, 0, 0x18}, []byte("ftypheic\x00\x00\x00\x00")...)
	if rr := multipartReq(t, srv, "IMG.HEIC", heic); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("heic: %d %s", rr.Code, rr.Body.String())
	}
	if rr := aiReq(t, srv, http.MethodPost, "/api/ai/receipt", map[string]any{}); rr.Code != 400 {
		t.Fatalf("sem multipart: %d", rr.Code)
	}
	big := make([]byte, ai.MaxReceiptBytes+1)
	copy(big, png)
	if rr := multipartReq(t, srv, "grande.png", big); rr.Code != http.StatusRequestEntityTooLarge && rr.Code != 400 {
		t.Fatalf("arquivo grande: %d %s", rr.Code, rr.Body.String())
	}
	if calls.Load() != 0 {
		t.Fatal("arquivos recusados não chamam a API")
	}

	rr := multipartReq(t, srv, "nota.png", png)
	if rr.Code != 200 {
		t.Fatalf("receipt: %d %s", rr.Code, rr.Body.String())
	}
	var res ai.ReceiptResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil || res.AmountCents != 2990 || res.Confidence != "MEDIUM" || res.SuggestedCategoryID != nil {
		t.Fatalf("resultado: %+v %v", res, err)
	}
}

func TestBodyLimitForReceipt(t *testing.T) {
	if got := bodyLimitFor(httptest.NewRequest(http.MethodPost, "/api/ai/receipt", nil)); got != MaxReceiptBodyBytes {
		t.Fatalf("receipt: %d", got)
	}
	if got := bodyLimitFor(httptest.NewRequest(http.MethodPost, "/api/ai/chat", nil)); got != MaxBodyBytes {
		t.Fatalf("chat: %d", got)
	}
}

// fakeOllama responde /api/version, /api/tags e, em ordem, os chunks NDJSON de /api/chat.
func fakeOllama(t *testing.T, chats ...string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var chatCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		switch r.URL.Path {
		case "/api/version":
			_, _ = io.WriteString(w, `{"version":"0.12.0"}`)
		case "/api/tags":
			_, _ = io.WriteString(w, `{"models":[{"name":"qwen2.5:7b","details":{"parameter_size":"7.6B"}}]}`)
		case "/api/show":
			_, _ = io.WriteString(w, `{"capabilities":["completion","tools"]}`)
		case "/api/chat":
			i := int(chatCalls.Add(1)) - 1
			if i >= len(chats) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"error":"sem resposta roteirizada"}`)
				return
			}
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, chats[i])
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &chatCalls
}

func TestAIConfigModelsAndLocalChat(t *testing.T) {
	ollama, chatCalls := fakeOllama(t,
		`{"message":{"role":"assistant","content":"Olá"},"done":false}`+"\n"+
			`{"message":{"role":"assistant","content":", tudo certo."},"done":false}`+"\n"+
			`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`+"\n",
	)
	srv := newAITestServerWith(t, "http://127.0.0.1:1", ollama.URL, nil)

	// Nada configurado, mas há um Ollama com modelos "neste computador": configuração automática.
	rr := aiReq(t, srv, http.MethodGet, "/api/ai/status", nil)
	var st ai.Status
	if err := json.Unmarshal(rr.Body.Bytes(), &st); err != nil || rr.Code != 200 || !st.Configured || st.Provider == nil ||
		*st.Provider != "ollama" || *st.Source != "AUTO" || *st.Model != "qwen2.5:7b" || *st.BaseURL != ollama.URL ||
		st.Reachable == nil || !*st.Reachable || st.ReceiptSupported {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
	if rr := aiReq(t, srv, http.MethodGet, "/api/system/info", nil); !strings.Contains(rr.Body.String(), `"aiConfigured":true`) {
		t.Fatalf("system info com detecção automática: %s", rr.Body.String())
	}

	rr = aiReq(t, srv, http.MethodGet, "/api/ai/models?provider=ollama", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `{"id":"qwen2.5:7b","label":"qwen2.5:7b · 7.6B","tools":true}`) {
		t.Fatalf("models: %d %s", rr.Code, rr.Body.String())
	}
	if rr := aiReq(t, srv, http.MethodGet, "/api/ai/models?provider=ollama&baseUrl=javascript:alert(1)", nil); rr.Code != 400 {
		t.Fatalf("baseUrl inválida: %d %s", rr.Code, rr.Body.String())
	}
	if rr := aiReq(t, srv, http.MethodPut, "/api/ai/config", map[string]any{"provider": "gpt"}); rr.Code != 400 {
		t.Fatalf("provider inválido: %d", rr.Code)
	}

	rr = aiReq(t, srv, http.MethodPut, "/api/ai/config", map[string]any{"provider": "ollama", "model": "qwen2.5:7b"})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"provider":"ollama"`) || !strings.Contains(rr.Body.String(), `"configured":true`) ||
		!strings.Contains(rr.Body.String(), `"source":"SETTINGS"`) || !strings.Contains(rr.Body.String(), `"receiptSupported":false`) {
		t.Fatalf("config: %d %s", rr.Code, rr.Body.String())
	}

	body := map[string]any{"messages": []map[string]string{{"role": "user", "content": "oi"}}}
	rr = aiReq(t, srv, http.MethodPost, "/api/ai/chat", body)
	out := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(out, "event: text\ndata: {\"delta\":\"Olá\"}\n\n") ||
		!strings.Contains(out, "event: done\ndata: {\"stopReason\":\"end_turn\"}\n\n") || chatCalls.Load() != 1 {
		t.Fatalf("chat local: %d %q", rr.Code, out)
	}

	// Comprovante sem modelo de visão: 412 com instrução em pt-BR.
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00")
	if rr := multipartReq(t, srv, "nota.png", png); rr.Code != http.StatusPreconditionFailed || !strings.Contains(rr.Body.String(), "modelo de visão") {
		t.Fatalf("receipt sem visão: %d %s", rr.Code, rr.Body.String())
	}

	// Servidor local fora do ar antes do primeiro evento: JSON 502 (nunca 401).
	ollama.Close()
	if rr := aiReq(t, srv, http.MethodPost, "/api/ai/chat", body); rr.Code != http.StatusBadGateway || !strings.Contains(rr.Body.String(), "Ollama") {
		t.Fatalf("Ollama fora do ar: %d %s", rr.Code, rr.Body.String())
	}

	// Legado continua funcionando e troca o provedor para o Claude.
	rr = aiReq(t, srv, http.MethodPut, "/api/ai/key", map[string]any{"apiKey": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz"})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"provider":"claude"`) || !strings.Contains(rr.Body.String(), `"reachable":null`) {
		t.Fatalf("legado: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAIConfigEnvPrecedence(t *testing.T) {
	ollama, _ := fakeOllama(t)
	srv := newAITestServerWith(t, "http://127.0.0.1:1", closedServerURL(t), map[string]string{
		"FINFLOW_AI_PROVIDER": "ollama", "FINFLOW_AI_BASE_URL": ollama.URL, "FINFLOW_AI_MODEL": "llama3.1:8b",
		"FINFLOW_AI_VISION_MODEL": "llava",
	})
	aiReq(t, srv, http.MethodPut, "/api/ai/key", map[string]any{"apiKey": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz"})
	rr := aiReq(t, srv, http.MethodGet, "/api/ai/status", nil)
	var st ai.Status
	_ = json.Unmarshal(rr.Body.Bytes(), &st)
	if st.Provider == nil || *st.Provider != "ollama" || *st.Source != "ENV" || *st.Model != "llama3.1:8b" ||
		*st.BaseURL != ollama.URL || !*st.Reachable || !st.ReceiptSupported {
		t.Fatalf("env deve ganhar do que está salvo: %s", rr.Body.String())
	}
	if rr := aiReq(t, srv, http.MethodGet, "/api/system/info", nil); !strings.Contains(rr.Body.String(), `"aiConfigured":true`) {
		t.Fatalf("system info: %s", rr.Body.String())
	}
}
