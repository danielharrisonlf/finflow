package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

type fakeModel struct {
	name string
	caps []string // nil = /api/show sem capacidades (versões antigas do Ollama)
}

// fakeOllamaWith emula /api/tags, /api/show e /api/version; /api/chat responde um texto simples.
func fakeOllamaWith(t *testing.T, models ...fakeModel) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var tags atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			_, _ = w.Write([]byte(`{"version":"0.12.0"}`))
		case "/api/tags":
			tags.Add(1)
			list := make([]any, 0, len(models))
			for _, m := range models {
				list = append(list, map[string]any{"name": m.name})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"models": list})
		case "/api/show":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			for _, m := range models {
				if m.name == body["model"] {
					_ = json.NewEncoder(w).Encode(map[string]any{"capabilities": m.caps})
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		case "/api/chat":
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Oi!"},"done":true,"done_reason":"stop"}` + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &tags
}

func autoService(t *testing.T, ollamaURL string, env map[string]string) *Service {
	t.Helper()
	svc, _ := newTestService(t, nil, false)
	svc.cfg.DefaultOllamaURL = ollamaURL
	svc.cfg.Getenv = func(k string) string { return env[k] }
	return svc
}

func TestAutoDetectUsesConfiguredModelWhenInstalled(t *testing.T) {
	ctx := context.Background()
	ollama, tags := fakeOllamaWith(t,
		fakeModel{"gemma:2b", []string{"completion"}},
		fakeModel{"llama3.1:8b", []string{"completion", "tools"}},
		fakeModel{"qwen2.5:14b", []string{"completion", "tools"}},
	)
	svc := autoService(t, ollama.URL, map[string]string{EnvModel: "qwen2.5:14b"})

	st, err := svc.Status(ctx)
	if err != nil || !st.Configured || *st.Provider != ProviderOllama || *st.Source != SourceAuto ||
		*st.Model != "qwen2.5:14b" || *st.BaseURL != ollama.URL || !*st.Reachable || st.ReceiptSupported {
		t.Fatalf("status: %+v %v", st, err)
	}
	if ok, _ := svc.Configured(ctx); !ok {
		t.Fatal("Configured deve refletir a detecção em cache")
	}
	rec := &recorder{}
	if err := svc.Chat(ctx, userAsks("oi"), rec.emit); err != nil || rec.text() != "Oi!" {
		t.Fatalf("chat com detecção automática: %v %q", err, rec.text())
	}
	if _, err := svc.Status(ctx); err != nil {
		t.Fatal(err)
	}
	if n := tags.Load(); n != 1 {
		t.Fatalf("a detecção deve ficar em cache: %d leituras de /api/tags", n)
	}
}

func TestAutoDetectFallsBackToFirstModel(t *testing.T) {
	ctx := context.Background()
	// Preferido (qwen2.5:7b) ausente: o primeiro com ferramentas.
	ollama, _ := fakeOllamaWith(t,
		fakeModel{"gemma:2b", []string{"completion"}},
		fakeModel{"llama3.1:8b", []string{"completion", "tools"}},
	)
	if st, _ := autoService(t, ollama.URL, nil).Status(ctx); !st.Configured || *st.Model != "llama3.1:8b" {
		t.Fatalf("primeiro com ferramentas: %+v", st)
	}
	// Sem capacidades informadas: o primeiro instalado.
	old, _ := fakeOllamaWith(t, fakeModel{"mistral:latest", nil}, fakeModel{"phi3:mini", nil})
	if st, _ := autoService(t, old.URL, map[string]string{EnvModel: "nao-instalado"}).Status(ctx); !st.Configured || *st.Model != "mistral:latest" {
		t.Fatalf("primeiro instalado: %+v", st)
	}
	// FINFLOW_OLLAMA_URL é onde a detecção procura (sem URL fixada na Config).
	svc := autoService(t, "", map[string]string{EnvOllamaURL: ollama.URL + "/"})
	if st, _ := svc.Status(ctx); !st.Configured || *st.BaseURL != ollama.URL || *st.Source != SourceAuto {
		t.Fatalf("FINFLOW_OLLAMA_URL: %+v", st)
	}
}

func TestAutoDetectUnreachableOrEmptyIsNotConfigured(t *testing.T) {
	ctx := context.Background()
	svc := autoService(t, closedURL(t), nil)
	st, err := svc.Status(ctx)
	if err != nil || st.Configured || st.Provider != nil || st.Source != nil || st.Reachable == nil || *st.Reachable {
		t.Fatalf("fora do ar: %+v %v", st, err)
	}
	if appErr := appStatus(t, svc.Chat(ctx, userAsks("oi"), (&recorder{}).emit)); appErr.Status != http.StatusPreconditionFailed {
		t.Fatalf("chat sem nada: %+v", appErr)
	}
	// Ollama no ar mas sem modelos: não configura, mas a interface pode oferecê-lo.
	empty, _ := fakeOllamaWith(t)
	if st, _ := autoService(t, empty.URL, nil).Status(ctx); st.Configured || st.Reachable == nil || !*st.Reachable {
		t.Fatalf("sem modelos: %+v", st)
	}
}

func TestSettingsAndEnvOverrideAutoDetect(t *testing.T) {
	ctx := context.Background()
	ollama, tags := fakeOllamaWith(t, fakeModel{"qwen2.5:7b", []string{"tools"}})
	svc := autoService(t, ollama.URL, map[string]string{})

	k := testKey
	st, err := svc.SaveKey(ctx, &k)
	if err != nil || *st.Provider != ProviderClaude || *st.Source != SourceSettings {
		t.Fatalf("settings ganham da detecção: %+v %v", st, err)
	}
	if tags.Load() != 0 {
		t.Fatal("com algo salvo, a detecção nem roda")
	}

	other := autoService(t, ollama.URL, map[string]string{EnvProvider: "openai_compatible", EnvModel: "m", EnvBaseURL: closedURL(t)})
	if st, _ := other.Status(ctx); *st.Provider != ProviderOpenAICompatible || *st.Source != SourceEnv {
		t.Fatalf("FINFLOW_AI_PROVIDER ganha da detecção: %+v", st)
	}
}
