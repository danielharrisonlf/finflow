package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"finflow/internal/domain"
	"finflow/internal/service"
)

const (
	// probeTimeout e probeTTL: o status pinga o servidor local rápido e guarda o resultado por ~10 s.
	probeTimeout = 1500 * time.Millisecond
	probeTTL     = 10 * time.Second
	// modelsTimeout limita a listagem de modelos (inclui /api/show do Ollama).
	modelsTimeout = 15 * time.Second
	// maxListedModels e showConcurrency limitam as consultas de capacidades no Ollama.
	maxListedModels = 60
	showConcurrency = 4
	// autoTimeout e autoTTL: a detecção automática (tags + capacidades) é rápida e fica em cache.
	autoTimeout = 3 * time.Second
	autoTTL     = 15 * time.Second
)

// autoCache guarda a última detecção automática (por URL e modelo preferido).
type autoCache struct {
	mu    sync.Mutex
	key   string
	at    time.Time
	found resolved
	ok    bool
}

func (a *autoCache) cached(url, preferred string) (resolved, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.key != url+" "+preferred || a.at.IsZero() || time.Since(a.at) >= autoTTL {
		return resolved{}, false
	}
	return a.found, a.ok
}

func (a *autoCache) store(url, preferred string, found resolved, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.key, a.at, a.found, a.ok = url+" "+preferred, time.Now(), found, ok
}

// autoPreferred é o modelo preferido na detecção automática: FINFLOW_AI_MODEL ou o padrão sugerido.
func (s *Service) autoPreferred() string {
	if m := s.getenv(EnvModel); m != "" {
		return m
	}
	return DefaultOllamaModel
}

// autoDetect configura o Ollama automaticamente quando ele responde em FINFLOW_OLLAMA_URL com pelo
// menos um modelo: o preferido se instalado, senão o primeiro com ferramentas, senão o primeiro.
func (s *Service) autoDetect(ctx context.Context) (resolved, bool) {
	url, preferred := s.ollamaURL(), s.autoPreferred()
	s.auto.mu.Lock()
	fresh := s.auto.key == url+" "+preferred && !s.auto.at.IsZero() && time.Since(s.auto.at) < autoTTL
	found, ok := s.auto.found, s.auto.ok
	s.auto.mu.Unlock()
	if fresh {
		return found, ok
	}
	dctx, cancel := context.WithTimeout(ctx, autoTimeout)
	defer cancel()
	c := s.localClient(resolved{provider: ProviderOllama, baseURL: url})
	models, err := c.ollamaModels(dctx)
	if err != nil || len(models) == 0 {
		if ctx.Err() == nil {
			s.auto.store(url, preferred, resolved{}, false)
		}
		return resolved{}, false
	}
	found = resolved{
		provider: ProviderOllama, baseURL: url, model: pickAutoModel(models, preferred),
		visionModel: s.getenv(EnvVisionModel), source: SourceAuto,
	}
	s.auto.store(url, preferred, found, true)
	return found, true
}

// pickAutoModel escolhe o modelo da configuração automática.
func pickAutoModel(models []ModelInfo, preferred string) string {
	for _, m := range models {
		if m.ID == preferred || m.ID == preferred+":latest" {
			return m.ID
		}
	}
	for _, m := range models {
		if m.Tools != nil && *m.Tools {
			return m.ID
		}
	}
	for _, m := range models {
		if m.Tools == nil {
			return m.ID
		}
	}
	return models[0].ID
}

// probeCache guarda o último ping de cada servidor (provedor + URL).
type probeCache struct {
	mu      sync.Mutex
	entries map[string]probeEntry
}

type probeEntry struct {
	ok bool
	at time.Time
}

func newProbeCache() *probeCache {
	return &probeCache{entries: map[string]probeEntry{}}
}

// reachable informa se o servidor local responde (qualquer resposta HTTP conta).
func (s *Service) reachable(ctx context.Context, provider, baseURL, apiKey string) bool {
	key := provider + " " + baseURL
	s.probes.mu.Lock()
	if e, ok := s.probes.entries[key]; ok && time.Since(e.at) < probeTTL {
		s.probes.mu.Unlock()
		return e.ok
	}
	s.probes.mu.Unlock()

	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	path := "/api/version"
	if provider == ProviderOpenAICompatible {
		path = "/models"
	}
	c := s.localClient(resolved{provider: provider, baseURL: baseURL, apiKey: apiKey})
	ok := false
	if resp, err := c.send(pctx, http.MethodGet, path, nil); err == nil {
		resp.Body.Close()
		ok = true
	}
	if ctx.Err() == nil {
		s.probes.mu.Lock()
		s.probes.entries[key] = probeEntry{ok: ok, at: time.Now()}
		s.probes.mu.Unlock()
	}
	return ok
}

// ModelInfo é um item de GET /api/ai/models. Tools é null quando o servidor não informa capacidades.
type ModelInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Tools *bool  `json:"tools"`
}

// ModelsResult é a resposta de GET /api/ai/models.
type ModelsResult struct {
	Models []ModelInfo `json:"models"`
}

// Models lista os modelos do provedor (padrão: o configurado; sem nada configurado, o Ollama) na URL
// informada (padrão: a configurada para o mesmo provedor, senão a padrão do provedor). Também serve
// como "Testar conexão" na interface.
func (s *Service) Models(ctx context.Context, provider, baseURL string) (ModelsResult, error) {
	r, err := s.resolve(ctx)
	if err != nil {
		return ModelsResult{}, err
	}
	provider, baseURL = strings.TrimSpace(provider), strings.TrimSpace(baseURL)
	if provider == "" {
		provider = r.provider
	}
	if provider == "" {
		provider = ProviderOllama
	}
	if !validProvider(provider) {
		return ModelsResult{}, service.ValidationFailed([]domain.ValidationIssue{{Path: "provider", Message: "provider deve ser ollama, openai_compatible ou claude"}})
	}
	if provider == ProviderClaude {
		yes := true
		return ModelsResult{Models: []ModelInfo{{ID: Model, Label: "Claude Opus 5.5 (API do Claude)", Tools: &yes}}}, nil
	}
	apiKey := ""
	if r.provider == provider {
		apiKey = r.apiKey
		if baseURL == "" {
			baseURL = r.baseURL
		}
	}
	if baseURL == "" {
		baseURL = s.defaultBaseURL(provider)
	}
	if msg := ValidateBaseURL(baseURL); msg != "" {
		return ModelsResult{}, service.ValidationFailed([]domain.ValidationIssue{{Path: "baseUrl", Message: msg}})
	}
	c := s.localClient(resolved{provider: provider, baseURL: baseURL, apiKey: apiKey})
	ctx, cancel := context.WithTimeout(ctx, modelsTimeout)
	defer cancel()
	var models []ModelInfo
	if provider == ProviderOllama {
		models, err = c.ollamaModels(ctx)
	} else {
		models, err = c.openAIModels(ctx)
	}
	if err != nil {
		return ModelsResult{}, err
	}
	if models == nil {
		models = []ModelInfo{}
	}
	return ModelsResult{Models: models}, nil
}

// ollamaModels lê /api/tags e, quando disponível, as capacidades de cada modelo em /api/show.
func (c localClient) ollamaModels(ctx context.Context) ([]ModelInfo, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/tags", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tags struct {
		Models []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Details struct {
				ParameterSize string `json:"parameter_size"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, c.badResponse()
	}
	out := make([]ModelInfo, 0, len(tags.Models))
	for _, m := range tags.Models {
		id := m.Name
		if id == "" {
			id = m.Model
		}
		if id == "" || len(out) >= maxListedModels {
			continue
		}
		label := id
		if m.Details.ParameterSize != "" {
			label += " · " + m.Details.ParameterSize
		}
		out = append(out, ModelInfo{ID: id, Label: label})
	}
	sem := make(chan struct{}, showConcurrency)
	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i].Tools = c.ollamaSupportsTools(ctx, out[i].ID)
		}()
	}
	wg.Wait()
	return out, nil
}

// ollamaSupportsTools consulta /api/show; null quando a versão do Ollama não informa capacidades.
func (c localClient) ollamaSupportsTools(ctx context.Context, model string) *bool {
	resp, err := c.send(ctx, http.MethodPost, "/api/show", map[string]string{"model": model})
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var show struct {
		Capabilities []string `json:"capabilities"`
	}
	if json.NewDecoder(resp.Body).Decode(&show) != nil || show.Capabilities == nil {
		return nil
	}
	has := false
	for _, capability := range show.Capabilities {
		if capability == "tools" {
			has = true
		}
	}
	return &has
}

// openAIModels lê {baseUrl}/models (capacidades não são informadas por esse endpoint).
func (c localClient) openAIModels(ctx context.Context) ([]ModelInfo, error) {
	resp, err := c.do(ctx, http.MethodGet, "/models", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, c.badResponse()
	}
	out := make([]ModelInfo, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID != "" && len(out) < maxListedModels {
			out = append(out, ModelInfo{ID: m.ID, Label: m.ID})
		}
	}
	return out, nil
}
