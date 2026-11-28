package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
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

// fakeOllamaAporte emula o Ollama (nenhum modelo de verdade é chamado): a cada pergunta, o primeiro
// turno pede propose_goal_contribution e o segundo responde em texto.
func fakeOllamaAporte(t *testing.T, goalID func() string) *httptest.Server {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path != "/api/chat" {
			_, _ = io.WriteString(w, `{"version":"0.12.0"}`)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		if calls.Add(1)%2 == 1 {
			args, _ := json.Marshal(map[string]any{"goalId": goalID(), "amountCents": 10000})
			_, _ = io.WriteString(w, `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"propose_goal_contribution","arguments":`+string(args)+`}}]},"done":false}`+"\n"+
				`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`+"\n")
			return
		}
		_, _ = io.WriteString(w, `{"message":{"role":"assistant","content":"Preparei um aporte de "},"done":false}`+"\n"+
			`{"message":{"role":"assistant","content":"**R$ 100,00**; confirme para gravar."},"done":false}`+"\n"+
			`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`+"\n")
	}))
	t.Cleanup(srv.Close)
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
	ollama := fakeOllamaAporte(t, func() string { return id })
	t.Setenv("FINFLOW_AI_PROVIDER", "ollama")
	t.Setenv("FINFLOW_AI_BASE_URL", ollama.URL)
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

func TestPerguntarErrors(t *testing.T) {
	setupLocal(t)
	// Servidor compatível sem modelo: 412 sem chamar rede nenhuma.
	t.Setenv("FINFLOW_AI_PROVIDER", "openai_compatible")
	mustFail(t, "Escolha o modelo do servidor compatível", "perguntar", "oi")

	t.Setenv("FINFLOW_AI_PROVIDER", "ollama")
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	t.Setenv("FINFLOW_AI_BASE_URL", srv.URL)
	mustFail(t, "Não foi possível conectar ao Ollama", "perguntar", "oi")
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
	ollama := fakeOllamaAporte(t, func() string { return id })
	// O servidor remoto lê a configuração do assistente do próprio ambiente.
	t.Setenv("FINFLOW_AI_PROVIDER", "ollama")
	t.Setenv("FINFLOW_AI_BASE_URL", ollama.URL)
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

	// Erro antes do stream (412) chega legível.
	t.Setenv("FINFLOW_AI_PROVIDER", "openai_compatible")
	t.Setenv("FINFLOW_AI_BASE_URL", "")
	mustFail(t, "Escolha o modelo do servidor compatível", "--api", srv.URL, "perguntar", "oi")
}
