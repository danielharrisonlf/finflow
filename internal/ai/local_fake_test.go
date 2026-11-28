package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeLocal emula um servidor local (Ollama ou compatível com a API da OpenAI) com respostas
// roteirizadas por caminho — nenhum teste fala com um servidor de modelos de verdade.
type fakeLocal struct {
	t        *testing.T
	mu       sync.Mutex
	routes   map[string][]localResp
	always   map[string]localResp
	requests []localReq
	server   *httptest.Server
}

type localResp struct {
	status      int
	contentType string
	body        string
}

type localReq struct {
	method string
	path   string
	header http.Header
	body   map[string]any
	raw    string
}

func newFakeLocal(t *testing.T) *fakeLocal {
	t.Helper()
	f := &fakeLocal{t: t, routes: map[string][]localResp{}, always: map[string]localResp{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

// on enfileira respostas para o caminho (consumidas em ordem).
func (f *fakeLocal) on(path string, resps ...localResp) *fakeLocal {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[path] = append(f.routes[path], resps...)
	return f
}

// onAlways responde sempre o mesmo no caminho (depois da fila).
func (f *fakeLocal) onAlways(path string, resp localResp) *fakeLocal {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.always[path] = resp
	return f
}

func (f *fakeLocal) handle(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.requests = append(f.requests, localReq{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: body, raw: string(raw)})
	var resp localResp
	queue := f.routes[r.URL.Path]
	switch {
	case len(queue) > 0:
		resp, f.routes[r.URL.Path] = queue[0], queue[1:]
	default:
		var ok bool
		if resp, ok = f.always[r.URL.Path]; !ok {
			resp = localResp{status: http.StatusInternalServerError, body: `{"error":"sem resposta roteirizada"}`}
		}
	}
	f.mu.Unlock()
	if resp.contentType == "" {
		resp.contentType = "application/json"
	}
	if resp.status == 0 {
		resp.status = http.StatusOK
	}
	w.Header().Set("Content-Type", resp.contentType)
	w.WriteHeader(resp.status)
	_, _ = io.WriteString(w, resp.body)
}

func (f *fakeLocal) calls(path string) []localReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []localReq
	for _, r := range f.requests {
		if path == "" || r.path == path {
			out = append(out, r)
		}
	}
	return out
}

func jsonResp(v any) localResp {
	b, _ := json.Marshal(v)
	return localResp{body: string(b)}
}

// ───── Ollama: NDJSON

func ndjson(chunks ...map[string]any) localResp {
	var b strings.Builder
	for _, c := range chunks {
		line, _ := json.Marshal(c)
		b.Write(line)
		b.WriteByte('\n')
	}
	return localResp{contentType: "application/x-ndjson", body: b.String()}
}

func ollamaText(s string) map[string]any {
	return map[string]any{"model": "m", "message": map[string]any{"role": "assistant", "content": s}, "done": false}
}

func ollamaToolCalls(calls ...[2]string) map[string]any {
	list := make([]any, 0, len(calls))
	for _, c := range calls {
		list = append(list, map[string]any{"function": map[string]any{"name": c[0], "arguments": json.RawMessage(c[1])}})
	}
	return map[string]any{"model": "m", "message": map[string]any{"role": "assistant", "content": "", "tool_calls": list}, "done": false}
}

func ollamaDone(reason string) map[string]any {
	return map[string]any{"model": "m", "message": map[string]any{"role": "assistant", "content": ""}, "done": true, "done_reason": reason}
}

// ───── OpenAI-compatible: SSE

func oaSSE(chunks ...any) localResp {
	var b strings.Builder
	for _, c := range chunks {
		data, _ := json.Marshal(c)
		fmt.Fprintf(&b, "data: %s\n\n", data)
	}
	b.WriteString("data: [DONE]\n\n")
	return localResp{contentType: "text/event-stream", body: b.String()}
}

func oaDelta(delta map[string]any, finish any) map[string]any {
	return map[string]any{
		"id": "chatcmpl-1", "object": "chat.completion.chunk",
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
	}
}

func oaToolDelta(index int, id, name, args string) map[string]any {
	fn := map[string]any{"arguments": args}
	if name != "" {
		fn["name"] = name
	}
	call := map[string]any{"index": index, "function": fn}
	if id != "" {
		call["id"], call["type"] = id, "function"
	}
	return oaDelta(map[string]any{"tool_calls": []any{call}}, nil)
}

// useLocal configura o serviço de teste para o provedor local apontado para o falso.
func useLocal(t *testing.T, svc *Service, provider, baseURL, model, vision string) {
	t.Helper()
	in := ConfigInput{Provider: provider, BaseURL: &baseURL, Model: &model, VisionModel: &vision}
	if _, err := svc.SaveConfig(context.Background(), in); err != nil {
		t.Fatalf("configurar %s: %v", provider, err)
	}
}

// toolMessages devolve as mensagens de um papel no corpo de uma requisição local.
func messagesWithRole(req localReq, role string) []map[string]any {
	var out []map[string]any
	for _, raw := range req.body["messages"].([]any) {
		m := raw.(map[string]any)
		if m["role"] == role {
			out = append(out, m)
		}
	}
	return out
}
