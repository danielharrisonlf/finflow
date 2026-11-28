package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeAPI emula a Messages API (SSE ou JSON) para os testes — nada sai para a rede.
type fakeAPI struct {
	t         *testing.T
	mu        sync.Mutex
	responses []fakeResponse
	requests  []recordedRequest
	server    *httptest.Server
}

type fakeResponse struct {
	sse  []string // eventos SSE já serializados (event + data)
	json string   // corpo JSON para requisições sem stream
}

type recordedRequest struct {
	header http.Header
	body   map[string]any
	raw    string
}

func newFakeAPI(t *testing.T, responses ...fakeResponse) *fakeAPI {
	t.Helper()
	f := &fakeAPI{t: t, responses: responses}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{header: r.Header.Clone(), body: body, raw: string(raw)})
	idx := len(f.requests) - 1
	f.mu.Unlock()
	if r.URL.Path != "/v1/messages" {
		http.Error(w, `{"type":"error","error":{"type":"not_found_error","message":"rota"}}`, http.StatusNotFound)
		return
	}
	if idx >= len(f.responses) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"sem resposta roteirizada"}}`))
		return
	}
	resp := f.responses[idx]
	if resp.json != "" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp.json))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for _, ev := range resp.sse {
		_, _ = io.WriteString(w, ev)
	}
}

// closedURL devolve uma URL onde nada escuta (conexão recusada na hora).
func closedURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

func (f *fakeAPI) calls() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.requests...)
}

// ───── construtores de eventos SSE

func sseEvent(name string, data any) string {
	b, _ := json.Marshal(data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", name, b)
}

type turn struct {
	events []string
	index  int
}

func newTurn() *turn {
	t := &turn{}
	t.events = append(t.events, sseEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": "msg_test", "type": "message", "role": "assistant", "model": Model,
			"content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": 10, "output_tokens": 1},
		},
	}))
	return t
}

func (t *turn) text(parts ...string) *turn {
	t.events = append(t.events, sseEvent("content_block_start", map[string]any{
		"type": "content_block_start", "index": t.index, "content_block": map[string]any{"type": "text", "text": ""},
	}))
	for _, p := range parts {
		t.events = append(t.events, sseEvent("content_block_delta", map[string]any{
			"type": "content_block_delta", "index": t.index, "delta": map[string]any{"type": "text_delta", "text": p},
		}))
	}
	t.events = append(t.events, sseEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": t.index}))
	t.index++
	return t
}

func (t *turn) thinking() *turn {
	t.events = append(t.events, sseEvent("content_block_start", map[string]any{
		"type": "content_block_start", "index": t.index, "content_block": map[string]any{"type": "thinking", "thinking": "", "signature": ""},
	}))
	t.events = append(t.events, sseEvent("content_block_delta", map[string]any{
		"type": "content_block_delta", "index": t.index, "delta": map[string]any{"type": "signature_delta", "signature": "sig"},
	}))
	t.events = append(t.events, sseEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": t.index}))
	t.index++
	return t
}

func (t *turn) toolUse(id, name, inputJSON string) *turn {
	t.events = append(t.events, sseEvent("content_block_start", map[string]any{
		"type": "content_block_start", "index": t.index,
		"content_block": map[string]any{"type": "tool_use", "id": id, "name": name, "input": map[string]any{}},
	}))
	if inputJSON != "" {
		t.events = append(t.events, sseEvent("content_block_delta", map[string]any{
			"type": "content_block_delta", "index": t.index, "delta": map[string]any{"type": "input_json_delta", "partial_json": inputJSON},
		}))
	}
	t.events = append(t.events, sseEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": t.index}))
	t.index++
	return t
}

func (t *turn) stop(reason string) fakeResponse {
	delta := map[string]any{"stop_reason": reason, "stop_sequence": nil}
	if reason == "refusal" {
		delta["stop_details"] = map[string]any{"type": "refusal", "category": "cyber", "explanation": nil}
	}
	t.events = append(t.events,
		sseEvent("message_delta", map[string]any{"type": "message_delta", "delta": delta, "usage": map[string]any{"output_tokens": 20}}),
		sseEvent("message_stop", map[string]any{"type": "message_stop"}),
	)
	return fakeResponse{sse: t.events}
}

// ───── coletor de eventos emitidos pelo chat

type emitted struct {
	name string
	data map[string]any
}

type recorder struct {
	mu     sync.Mutex
	events []emitted
}

func (r *recorder) emit(event string, data any) {
	b, _ := json.Marshal(data)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	r.mu.Lock()
	r.events = append(r.events, emitted{name: event, data: m})
	r.mu.Unlock()
}

func (r *recorder) named(name string) []emitted {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []emitted
	for _, e := range r.events {
		if e.name == name {
			out = append(out, e)
		}
	}
	return out
}

func (r *recorder) text() string {
	var b strings.Builder
	for _, e := range r.named("text") {
		b.WriteString(e.data["delta"].(string))
	}
	return b.String()
}
