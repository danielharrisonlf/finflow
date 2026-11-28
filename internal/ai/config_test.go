package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"finflow/internal/domain"
)

func TestConfigPrecedenceEnvOverSettings(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t, nil, false)
	env := map[string]string{}
	svc.cfg.Getenv = func(k string) string { return env[k] }

	saved := closedURL(t)
	useLocal(t, svc, ProviderOllama, saved, "llama3.1:8b", "llava")
	r, err := svc.resolve(ctx)
	if err != nil || r.provider != ProviderOllama || r.source != SourceSettings || r.baseURL != saved ||
		r.model != "llama3.1:8b" || r.visionModel != "llava" || !r.configured() {
		t.Fatalf("settings: %+v %v", r, err)
	}

	// Variáveis de ambiente ganham campo a campo.
	env[EnvModel] = "qwen2.5:14b"
	env[EnvBaseURL] = "http://ollama:11434/"
	if r, _ = svc.resolve(ctx); r.model != "qwen2.5:14b" || r.baseURL != "http://ollama:11434" || r.visionModel != "llava" || r.source != SourceSettings {
		t.Fatalf("env por campo: %+v", r)
	}

	// Outro provedor no ambiente: os campos salvos (de outro provedor) não valem.
	env = map[string]string{EnvProvider: "openai_compatible"}
	if r, _ = svc.resolve(ctx); r.provider != ProviderOpenAICompatible || r.source != SourceEnv ||
		r.baseURL != DefaultOpenAICompatibleURL || r.model != "" || r.configured() {
		t.Fatalf("env com outro provedor: %+v", r)
	}
	env[EnvModel] = "local-model"
	env[EnvAPIKey] = "segredo"
	if r, _ = svc.resolve(ctx); !r.configured() || r.apiKey != "segredo" {
		t.Fatalf("env completo: %+v", r)
	}

	// Ollama só no ambiente: modelo padrão sugerido.
	env = map[string]string{EnvProvider: "OLLAMA"}
	if r, _ = svc.resolve(ctx); r.provider != ProviderOllama || r.model != "llama3.1:8b" || r.source != SourceEnv {
		// mesmo provedor do salvo: campos salvos continuam valendo
		t.Fatalf("env ollama com settings ollama: %+v", r)
	}
	if _, err := svc.SaveConfig(ctx, ConfigInput{Provider: ProviderClaude}); err != nil {
		t.Fatal(err)
	}
	if r, _ = svc.resolve(ctx); r.model != DefaultOllamaModel || r.baseURL != svc.cfg.DefaultOllamaURL {
		t.Fatalf("env ollama sem settings ollama: %+v", r)
	}

	// Valor desconhecido no ambiente é ignorado.
	env = map[string]string{EnvProvider: "gpt"}
	if r, _ = svc.resolve(ctx); r.provider != ProviderClaude || r.configured() {
		t.Fatalf("provedor inválido no ambiente: %+v", r)
	}
}

func TestClaudeLegacyAndEnvKey(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t, nil, false)
	env := map[string]string{}
	svc.cfg.Getenv = func(k string) string { return env[k] }

	if r, _ := svc.resolve(ctx); r.provider != "" {
		t.Fatalf("nada configurado: %+v", r)
	}
	svc.cfg.EnvCredentials = func() bool { return true }
	if r, _ := svc.resolve(ctx); r.provider != ProviderClaude || r.source != SourceEnv || !r.claudeEnv || !r.configured() {
		t.Fatalf("legado ANTHROPIC_API_KEY: %+v", r)
	}
	svc.cfg.EnvCredentials = func() bool { return false }
	env[EnvProvider], env[EnvAPIKey] = "claude", testKey
	if r, _ := svc.resolve(ctx); r.apiKey != testKey || r.source != SourceEnv || r.model != Model {
		t.Fatalf("FINFLOW_AI_API_KEY para o Claude: %+v", r)
	}
}

func TestSaveConfigValidationAndSecrets(t *testing.T) {
	ctx := context.Background()
	svc, svcs := newTestService(t, nil, false)
	bad := []ConfigInput{
		{Provider: "gpt"},
		{Provider: ProviderOllama, BaseURL: ptr("ftp://x")},
		{Provider: ProviderOllama, Model: ptr("qwen 2.5")},
		{Provider: ProviderOpenAICompatible, BaseURL: ptr("http://localhost:1234/v1")},
		{Provider: ProviderClaude, APIKey: domain.Optional[string]{Set: true, Valid: true, Value: "curta"}},
	}
	for i, in := range bad {
		if _, err := svc.SaveConfig(ctx, in); err == nil {
			t.Errorf("caso %d deveria falhar: %+v", i, in)
		}
	}

	key := "sk-local-0001"
	st, err := svc.SaveConfig(ctx, ConfigInput{
		Provider: ProviderOpenAICompatible, BaseURL: ptr(closedURL(t) + "/v1/"), Model: ptr("modelo"),
		APIKey: domain.Optional[string]{Set: true, Valid: true, Value: key},
	})
	if err != nil || !st.Configured || *st.Provider != ProviderOpenAICompatible || *st.Source != SourceSettings ||
		strings.HasSuffix(*st.BaseURL, "/") || st.Reachable == nil || *st.Reachable || st.ReceiptSupported {
		t.Fatalf("status: %+v %v", st, err)
	}
	b, _ := json.Marshal(st)
	if strings.Contains(string(b), key) {
		t.Fatalf("a chave nunca volta no status: %s", b)
	}
	for _, field := range []string{`"configured"`, `"provider"`, `"model"`, `"visionModel"`, `"baseUrl"`, `"source"`, `"reachable"`, `"receiptSupported"`} {
		if !strings.Contains(string(b), field) {
			t.Fatalf("status sem %s: %s", field, b)
		}
	}
	if settings, _ := svcs.GetSettings(ctx); strings.Contains(func() string { b, _ := json.Marshal(settings); return string(b) }(), key) {
		t.Fatal("a chave não aparece nas configurações gerais")
	}

	// apiKey omitido mantém a chave; null apaga. (URL explícita: o teste nunca pinga um servidor real.)
	other := closedURL(t)
	if _, err := svc.SaveConfig(ctx, ConfigInput{Provider: ProviderOpenAICompatible, BaseURL: &other, Model: ptr("outro")}); err != nil {
		t.Fatal(err)
	}
	if r, _ := svc.resolve(ctx); r.apiKey != key || r.model != "outro" || r.baseURL != other {
		t.Fatalf("chave mantida: %+v", r)
	}
	if _, err := svc.SaveConfig(ctx, ConfigInput{Provider: ProviderOpenAICompatible, BaseURL: &other, Model: ptr("outro"), APIKey: domain.Optional[string]{Set: true}}); err != nil {
		t.Fatal(err)
	}
	if r, _ := svc.resolve(ctx); r.apiKey != "" {
		t.Fatalf("chave deveria ser apagada: %+v", r)
	}

	// Legado: PUT /api/ai/key vira provedor claude.
	k := testKey
	if st, err = svc.SaveKey(ctx, &k); err != nil || *st.Provider != ProviderClaude || !st.Configured || !st.ReceiptSupported || st.Reachable != nil {
		t.Fatalf("legado: %+v %v", st, err)
	}
}

func TestStatusDetectsLocalOllama(t *testing.T) {
	ctx := context.Background()
	fake := newFakeLocal(t).onAlways("/api/version", jsonResp(map[string]string{"version": "0.12.0"}))
	svc, _ := newTestService(t, nil, false)
	svc.cfg.DefaultOllamaURL = fake.server.URL

	st, err := svc.Status(ctx)
	if err != nil || st.Configured || st.Provider != nil || st.Reachable == nil || !*st.Reachable || st.ReceiptSupported {
		t.Fatalf("status com Ollama local: %+v %v", st, err)
	}
	if _, err := svc.Status(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(fake.calls("/api/version")); n != 1 {
		t.Fatalf("o ping deve ficar em cache por alguns segundos: %d chamadas", n)
	}

	// "Usar o Ollama deste computador": só o provedor; URL e modelo ficam nos padrões.
	st, err = svc.SaveConfig(ctx, ConfigInput{Provider: ProviderOllama})
	if err != nil || !st.Configured || *st.Model != DefaultOllamaModel || *st.BaseURL != fake.server.URL || !*st.Reachable || *st.Source != SourceSettings {
		t.Fatalf("depois de escolher o Ollama: %+v %v", st, err)
	}

	other, _ := newTestService(t, nil, false)
	if st, _ := other.Status(ctx); st.Reachable == nil || *st.Reachable {
		t.Fatalf("sem Ollama local: %+v", st)
	}
}

func TestModelsListing(t *testing.T) {
	ctx := context.Background()
	fake := newFakeLocal(t).onAlways("/api/tags", jsonResp(map[string]any{"models": []any{
		map[string]any{"name": "qwen2.5:7b", "details": map[string]any{"parameter_size": "7.6B"}},
		map[string]any{"name": "gemma:2b"},
		map[string]any{"name": "antigo:latest"},
	}}))
	// /api/show por modelo (a ordem de chegada varia): responde pelo nome.
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" {
			fake.handle(w, r)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch body["model"] {
		case "qwen2.5:7b":
			_, _ = w.Write([]byte(`{"capabilities":["completion","tools"]}`))
		case "gemma:2b":
			_, _ = w.Write([]byte(`{"capabilities":["completion"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ollama.Close)
	svc, _ := newTestService(t, nil, false)
	svc.cfg.DefaultOllamaURL = ollama.URL

	res, err := svc.Models(ctx, "", "")
	if err != nil || len(res.Models) != 3 {
		t.Fatalf("modelos: %+v %v", res, err)
	}
	q, g, a := res.Models[0], res.Models[1], res.Models[2]
	if q.ID != "qwen2.5:7b" || q.Label != "qwen2.5:7b · 7.6B" || q.Tools == nil || !*q.Tools {
		t.Fatalf("qwen: %+v", q)
	}
	if g.Tools == nil || *g.Tools || a.Tools != nil {
		t.Fatalf("capacidades: %+v %+v", g, a)
	}

	oa := newFakeLocal(t).on("/v1/models", jsonResp(map[string]any{"object": "list", "data": []any{map[string]any{"id": "qwen2.5-7b-instruct"}}}))
	res, err = svc.Models(ctx, ProviderOpenAICompatible, oa.server.URL+"/v1")
	if err != nil || len(res.Models) != 1 || res.Models[0].ID != "qwen2.5-7b-instruct" || res.Models[0].Tools != nil {
		t.Fatalf("openai_compatible: %+v %v", res, err)
	}

	if res, err = svc.Models(ctx, ProviderClaude, ""); err != nil || len(res.Models) != 1 || res.Models[0].ID != Model {
		t.Fatalf("claude: %+v %v", res, err)
	}
	if _, err = svc.Models(ctx, ProviderOllama, "file:///etc"); appStatus(t, err).Status != http.StatusBadRequest {
		t.Fatalf("URL inválida: %v", err)
	}
	if _, err = svc.Models(ctx, ProviderOllama, closedURL(t)); appStatus(t, err).Status != http.StatusBadGateway {
		t.Fatalf("fora do ar: %v", err)
	}
	empty := newFakeLocal(t).on("/api/tags", jsonResp(map[string]any{"models": []any{}}))
	if res, err = svc.Models(ctx, ProviderOllama, empty.server.URL); err != nil || res.Models == nil || len(res.Models) != 0 {
		t.Fatalf("lista vazia deve ser []: %+v %v", res, err)
	}
}
