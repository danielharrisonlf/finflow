package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"finflow/internal/cli"
	"finflow/internal/client"
	"finflow/internal/db"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

// testAnthropicKey é uma chave falsa: as requisições vão para o fakeClaudeAporte, nunca para a API real.
const testAnthropicKey = "sk-ant-api03-teste-abcdefghijklmnopqrstuvwxyz"

func sseEvent(name string, data any) string {
	b, _ := json.Marshal(data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", name, b)
}

// claudeTurn monta um turno em streaming da Messages API com os blocos informados.
func claudeTurn(stopReason string, blocks ...map[string]any) string {
	var b strings.Builder
	b.WriteString(sseEvent("message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": "msg_test", "type": "message", "role": "assistant", "model": "claude-opus-5-5", "content": []any{},
		"stop_reason": nil, "usage": map[string]any{"input_tokens": 10, "output_tokens": 1}}}))
	for i, block := range blocks {
		start := map[string]any{"type": block["type"], "text": ""}
		var delta map[string]any
		if block["type"] == "tool_use" {
			start = map[string]any{"type": "tool_use", "id": "toolu_1", "name": block["name"], "input": map[string]any{}}
			delta = map[string]any{"type": "input_json_delta", "partial_json": block["input"]}
		} else {
			delta = map[string]any{"type": "text_delta", "text": block["text"]}
		}
		b.WriteString(sseEvent("content_block_start", map[string]any{"type": "content_block_start", "index": i, "content_block": start}))
		b.WriteString(sseEvent("content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": delta}))
		b.WriteString(sseEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": i}))
	}
	b.WriteString(sseEvent("message_delta", map[string]any{"type": "message_delta",
		"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil}, "usage": map[string]any{"output_tokens": 20}}))
	b.WriteString(sseEvent("message_stop", map[string]any{"type": "message_stop"}))
	return b.String()
}

// fakeClaudeAporte emula a API do Claude (nenhum modelo de verdade é chamado): a cada pergunta, o
// primeiro turno pede propose_goal_contribution e o segundo responde em texto. Aponta o SDK para si
// via ANTHROPIC_BASE_URL.
func fakeClaudeAporte(t *testing.T, goalID func() string) *httptest.Server {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.Header.Get("X-Api-Key") != testAnthropicKey && r.Header.Get("Authorization") != "Bearer "+testAnthropicKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1)%2 == 1 {
			args, _ := json.Marshal(map[string]any{"goalId": goalID(), "amountCents": 10000})
			_, _ = io.WriteString(w, claudeTurn("tool_use", map[string]any{"type": "tool_use", "name": "propose_goal_contribution", "input": string(args)}))
			return
		}
		_, _ = io.WriteString(w, claudeTurn("end_turn", map[string]any{"type": "text", "text": "Preparei um aporte de **R$ 100,00**; confirme para gravar."}))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
	return srv
}

func runIn(t *testing.T, stdin io.Reader, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	code := cli.RunWithInput(args, stdin, &out, &errOut)
	return result{code: code, stdout: out.String(), stderr: errOut.String()}
}

// nonInteractive é uma entrada que não é terminal (pipe), como em scripts e CI.
func nonInteractive(t *testing.T) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func goalID(t *testing.T, b client.Backend, name string) string {
	t.Helper()
	goals, err := b.Goals(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range goals {
		if g.Name == name {
			return g.ID
		}
	}
	t.Fatalf("meta %q não encontrada", name)
	return ""
}

func TestPerguntarLocal(t *testing.T) {
	path := setupLocal(t)
	mustRun(t, "seed")
	b := localBackend(t, path)
	id := goalID(t, b, "MacBook Pro")
	fakeClaudeAporte(t, func() string { return id })
	t.Setenv("ANTHROPIC_API_KEY", testAnthropicKey)
	before := goalSaved(t, b, "MacBook Pro")

	// Entrada não interativa: mostra a proposta e não grava.
	r := runIn(t, nonInteractive(t), "perguntar", "Guarda", "100", "no", "MacBook")
	if r.code != 0 {
		t.Fatalf("perguntar: %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	assertContains(t, "perguntar", r.stdout,
		"› Preparando um aporte para você confirmar…",
		"Preparei um aporte de **R$ 100,00**; confirme para gravar.",
		"1 proposta do Assistente (nada foi gravado ainda):",
		"Aporte de R$ 100,00 na meta MacBook Pro",
		"Rode de novo com --confirmar")
	if got := goalSaved(t, b, "MacBook Pro"); got != before {
		t.Fatalf("sem confirmação nada é gravado: %d → %d", before, got)
	}

	// Pergunta s/N: "n" não grava, "s" grava.
	r = runIn(t, strings.NewReader("n\n"), "perguntar", "Guarda 100 no MacBook")
	assertContains(t, "resposta n", r.stdout, "Aplicar? [s/N]", "Nada foi gravado.")
	if got := goalSaved(t, b, "MacBook Pro"); got != before {
		t.Fatalf("resposta n não grava: %d → %d", before, got)
	}
	r = runIn(t, strings.NewReader("s\n"), "perguntar", "Guarda 100 no MacBook")
	assertContains(t, "resposta s", r.stdout, "✔ gravado: Aporte de R$ 100,00 na meta MacBook Pro")
	if got := goalSaved(t, b, "MacBook Pro"); got != before+10000 {
		t.Fatalf("resposta s grava: %d → %d", before, got)
	}

	// --confirmar grava sem perguntar.
	r = runIn(t, nonInteractive(t), "perguntar", "--confirmar", "Guarda 100 no MacBook")
	if r.code != 0 || strings.Contains(r.stdout, "[s/N]") {
		t.Fatalf("--confirmar: %d\n%s", r.code, r.stdout)
	}
	if got := goalSaved(t, b, "MacBook Pro"); got != before+20000 {
		t.Fatalf("--confirmar grava: %d → %d", before, got)
	}
}

func TestPerguntarDisabledWithoutKey(t *testing.T) {
	setupLocal(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)

	// Sem chave: mensagem em pt-BR dizendo como ativar, código diferente de 0 e nenhuma requisição.
	mustFail(t, "o Assistente está desativado", "perguntar", "oi")
	mustFail(t, "ANTHROPIC_API_KEY ou salve a sua chave em Configurações → Assistente (IA)", "perguntar", "oi")
	if calls.Load() != 0 {
		t.Fatalf("sem chave não chama a API: %d chamadas", calls.Load())
	}

	// Chave recusada pela API: erro legível (nunca o texto cru do SDK).
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-api03-chave-recusada-abcdefghijkl")
	mustFail(t, "A chave da API do Claude foi recusada", "perguntar", "oi")
}

func TestPerguntarRemote(t *testing.T) {
	isolateEnv(t)
	database := testutil.Open(t, db.DialectSQLite)
	st := store.New(database)
	if _, err := store.Seed(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	svcs := service.New(st, nil)
	b := &client.LocalBackend{Svcs: svcs}
	id := goalID(t, b, "MacBook Pro")
	fakeClaudeAporte(t, func() string { return id })
	// O servidor remoto lê a chave do Claude do próprio ambiente.
	t.Setenv("ANTHROPIC_API_KEY", testAnthropicKey)
	srv := httptest.NewServer(httpapi.NewServer(svcs, httpapi.ServerOptions{}))
	t.Cleanup(srv.Close)
	before := goalSaved(t, b, "MacBook Pro")

	r := runIn(t, nonInteractive(t), "--api", srv.URL, "perguntar", "Guarda 100 no MacBook")
	if r.code != 0 {
		t.Fatalf("perguntar remoto: %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	assertContains(t, "remoto", r.stdout, "› Preparando um aporte", "Preparei um aporte de", "Aporte de R$ 100,00 na meta MacBook Pro", "--confirmar")
	if got := goalSaved(t, b, "MacBook Pro"); got != before {
		t.Fatalf("sem confirmação nada é gravado: %d → %d", before, got)
	}

	r = runIn(t, nonInteractive(t), "--api", srv.URL, "perguntar", "--confirmar", "Guarda 100 no MacBook")
	assertContains(t, "remoto --confirmar", r.stdout, "✔ gravado: Aporte de R$ 100,00 na meta MacBook Pro")
	if got := goalSaved(t, b, "MacBook Pro"); got != before+10000 {
		t.Fatalf("--confirmar grava pela API: %d → %d", before, got)
	}

	// Sem chave no servidor remoto (412 antes do stream): a mesma mensagem de recurso desativado.
	t.Setenv("ANTHROPIC_API_KEY", "")
	mustFail(t, "o Assistente está desativado", "--api", srv.URL, "perguntar", "oi")
}
