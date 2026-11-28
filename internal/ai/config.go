package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"finflow/internal/domain"
	"finflow/internal/service"
)

// Provedores do Assistente (docs/local-agent-contract.md §1).
const (
	ProviderOllama           = "ollama"
	ProviderOpenAICompatible = "openai_compatible"
	ProviderClaude           = "claude"
)

// Variáveis de ambiente (§2): têm precedência sobre o que está salvo em settings.
const (
	EnvProvider    = "FINFLOW_AI_PROVIDER"
	EnvBaseURL     = "FINFLOW_AI_BASE_URL"
	EnvModel       = "FINFLOW_AI_MODEL"
	EnvVisionModel = "FINFLOW_AI_VISION_MODEL"
	EnvAPIKey      = "FINFLOW_AI_API_KEY"
	// EnvOllamaURL é onde o Ollama é procurado para a configuração automática e a URL padrão do
	// provedor ollama (padrão: http://localhost:11434; no Docker, http://ollama:11434).
	EnvOllamaURL = "FINFLOW_OLLAMA_URL"
)

// Chaves da tabela settings (valores JSON string). A chave do Claude continua em SettingAPIKey; a do
// servidor compatível fica em SettingLocalAPIKey. Nenhuma das duas é devolvida pela API.
const (
	SettingProvider    = "aiProvider"
	SettingBaseURL     = "aiBaseUrl"
	SettingModel       = "aiModel"
	SettingVisionModel = "aiVisionModel"
	SettingLocalAPIKey = "aiApiKey"
)

// Padrões dos provedores locais.
const (
	DefaultOllamaURL           = "http://localhost:11434"
	DefaultOpenAICompatibleURL = "http://localhost:1234/v1"
	// DefaultOllamaModel é a sugestão de modelo de chat com ferramentas (alternativa: llama3.1:8b).
	DefaultOllamaModel = "qwen2.5:7b"
	// SuggestedVisionModel é a sugestão de modelo de visão para comprovantes (alternativa: llava).
	SuggestedVisionModel = "qwen2.5vl:7b"
)

// ErrNotConfigured é devolvido quando não há provedor utilizável (HTTP 412).
var ErrNotConfigured = &service.AppError{
	Status:  http.StatusPreconditionFailed,
	Message: "Assistente não configurado: escolha um modelo local (Ollama) em Configurações → Assistente, ou defina ANTHROPIC_API_KEY / salve uma chave da API do Claude.",
}

// errModelMissing: servidor compatível sem modelo escolhido (HTTP 412).
var errModelMissing = &service.AppError{
	Status:  http.StatusPreconditionFailed,
	Message: "Escolha o modelo do servidor compatível em Configurações → Assistente (ou defina FINFLOW_AI_MODEL).",
}

// ErrNoVisionModel: leitura de comprovante com provedor local sem modelo de visão (HTTP 412).
var ErrNoVisionModel = &service.AppError{
	Status:  http.StatusPreconditionFailed,
	Message: "Para ler comprovantes com o modelo local, configure um modelo de visão (ex.: " + SuggestedVisionModel + ") em Configurações → Assistente.",
}

func validProvider(p string) bool {
	return p == ProviderOllama || p == ProviderOpenAICompatible || p == ProviderClaude
}

func isLocalProvider(p string) bool {
	return p == ProviderOllama || p == ProviderOpenAICompatible
}

// resolved é a configuração efetiva (ambiente > settings > padrões).
type resolved struct {
	provider    string // "" = nenhum
	baseURL     string
	model       string
	visionModel string
	apiKey      string
	// source: ENV/SETTINGS. No Claude é a origem da credencial ("" sem credencial).
	source string
	// claudeEnv: a credencial do Claude vem da cadeia padrão do SDK (ANTHROPIC_API_KEY etc.).
	claudeEnv bool
}

func (r resolved) configured() bool {
	switch r.provider {
	case ProviderClaude:
		return r.source != ""
	case ProviderOllama:
		return r.model != ""
	case ProviderOpenAICompatible:
		return r.model != ""
	}
	return false
}

func (r resolved) receiptSupported() bool {
	if !r.configured() {
		return false
	}
	return r.provider == ProviderClaude || r.visionModel != ""
}

func (s *Service) getenv(key string) string {
	return strings.TrimSpace(s.cfg.Getenv(key))
}

// savedStrings lê as chaves de settings que guardam strings JSON.
func (s *Service) savedStrings(ctx context.Context) (map[string]string, error) {
	values, err := s.svcs.Store.SettingValues(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, k := range []string{SettingAPIKey, SettingProvider, SettingBaseURL, SettingModel, SettingVisionModel, SettingLocalAPIKey} {
		raw, ok := values[k]
		if !ok {
			continue
		}
		var v string
		if json.Unmarshal([]byte(raw), &v) == nil {
			out[k] = strings.TrimSpace(v)
		}
	}
	return out, nil
}

// ollamaURL: Config.DefaultOllamaURL (testes) > FINFLOW_OLLAMA_URL > http://localhost:11434.
func (s *Service) ollamaURL() string {
	u := s.cfg.DefaultOllamaURL
	if u == "" {
		u = s.getenv(EnvOllamaURL)
	}
	if u == "" {
		u = DefaultOllamaURL
	}
	return strings.TrimRight(u, "/")
}

func (s *Service) defaultBaseURL(provider string) string {
	switch provider {
	case ProviderOllama:
		return s.ollamaURL()
	case ProviderOpenAICompatible:
		return DefaultOpenAICompatibleURL
	}
	return ""
}

// resolve decide o provedor e os campos. Ordem: FINFLOW_AI_PROVIDER → provedor salvo → legado do Claude
// (credencial no ambiente ou chave salva). Campos salvos só valem para o provedor salvo; variáveis de
// ambiente sempre ganham.
func (s *Service) resolve(ctx context.Context) (resolved, error) {
	saved, err := s.savedStrings(ctx)
	if err != nil {
		return resolved{}, err
	}
	var r resolved
	savedProvider := saved[SettingProvider]
	if !validProvider(savedProvider) {
		savedProvider = ""
	}
	if p := strings.ToLower(s.getenv(EnvProvider)); validProvider(p) {
		r.provider, r.source = p, SourceEnv
	} else if savedProvider != "" {
		r.provider, r.source = savedProvider, SourceSettings
	} else if s.cfg.EnvCredentials() || saved[SettingAPIKey] != "" {
		r.provider = ProviderClaude
	}
	pick := func(envKey, settingKey string) string {
		if v := s.getenv(envKey); v != "" {
			return v
		}
		if savedProvider == r.provider {
			return saved[settingKey]
		}
		return ""
	}
	switch r.provider {
	case ProviderClaude:
		r.model, r.visionModel = Model, Model
		switch {
		case s.getenv(EnvAPIKey) != "":
			r.apiKey, r.source = s.getenv(EnvAPIKey), SourceEnv
		case s.cfg.EnvCredentials():
			r.claudeEnv, r.source = true, SourceEnv
		case saved[SettingAPIKey] != "":
			r.apiKey, r.source = saved[SettingAPIKey], SourceSettings
		default:
			r.source = ""
		}
	case ProviderOllama, ProviderOpenAICompatible:
		r.baseURL = pick(EnvBaseURL, SettingBaseURL)
		if r.baseURL == "" {
			r.baseURL = s.defaultBaseURL(r.provider)
		}
		r.baseURL = strings.TrimRight(r.baseURL, "/")
		r.model = pick(EnvModel, SettingModel)
		if r.model == "" && r.provider == ProviderOllama {
			r.model = DefaultOllamaModel
		}
		r.visionModel = pick(EnvVisionModel, SettingVisionModel)
		if r.provider == ProviderOpenAICompatible {
			r.apiKey = pick(EnvAPIKey, SettingLocalAPIKey)
		}
	}
	return r, nil
}

// ConfigInput é o corpo de PUT /api/ai/config. apiKey omitido mantém a chave salva; null ou "" apaga.
type ConfigInput struct {
	Provider    string                  `json:"provider"`
	BaseURL     *string                 `json:"baseUrl"`
	Model       *string                 `json:"model"`
	VisionModel *string                 `json:"visionModel"`
	APIKey      domain.Optional[string] `json:"apiKey"`
}

func trimPtr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// ValidateBaseURL aceita só URLs http(s) com host.
func ValidateBaseURL(raw string) string {
	if len(raw) > 512 {
		return "URL longa demais"
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "use uma URL http:// ou https://, ex.: " + DefaultOllamaURL
	}
	return ""
}

func validateModelName(name string) string {
	if len(name) > 200 {
		return "nome longo demais"
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "nome não pode conter espaços"
		}
	}
	return ""
}

// ValidateConfigInput confere provedor, URL, modelos e chave.
func ValidateConfigInput(in ConfigInput) []domain.ValidationIssue {
	var issues []domain.ValidationIssue
	if !validProvider(in.Provider) {
		return []domain.ValidationIssue{{Path: "provider", Message: "provider deve ser ollama, openai_compatible ou claude"}}
	}
	if isLocalProvider(in.Provider) {
		if u := trimPtr(in.BaseURL); u != "" {
			if msg := ValidateBaseURL(u); msg != "" {
				issues = append(issues, domain.ValidationIssue{Path: "baseUrl", Message: msg})
			}
		}
		for path, v := range map[string]string{"model": trimPtr(in.Model), "visionModel": trimPtr(in.VisionModel)} {
			if msg := validateModelName(v); msg != "" {
				issues = append(issues, domain.ValidationIssue{Path: path, Message: msg})
			}
		}
		if in.Provider == ProviderOpenAICompatible && trimPtr(in.Model) == "" {
			issues = append(issues, domain.ValidationIssue{Path: "model", Message: "informe o modelo carregado no servidor"})
		}
	}
	if in.APIKey.Valid {
		key := strings.TrimSpace(in.APIKey.Value)
		switch {
		case key == "":
		case in.Provider == ProviderClaude:
			issues = append(issues, ValidateAPIKey(key)...)
		case in.Provider == ProviderOpenAICompatible:
			if len(key) > 512 || strings.IndexFunc(key, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
				issues = append(issues, domain.ValidationIssue{Path: "apiKey", Message: "chave deve ter até 512 caracteres, sem espaços"})
			}
		}
	}
	return issues
}

// SaveConfig grava a configuração do provedor e devolve o status. Para provedores locais a URL e os
// modelos são substituídos (vazio = padrão); a chave só muda quando apiKey vem no corpo.
func (s *Service) SaveConfig(ctx context.Context, in ConfigInput) (Status, error) {
	in.Provider = strings.TrimSpace(in.Provider)
	if issues := ValidateConfigInput(in); len(issues) > 0 {
		return Status{}, service.ValidationFailed(issues)
	}
	enc := func(v string) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	values := map[string]string{SettingProvider: enc(in.Provider)}
	if isLocalProvider(in.Provider) {
		values[SettingBaseURL] = enc(strings.TrimRight(trimPtr(in.BaseURL), "/"))
		values[SettingModel] = enc(trimPtr(in.Model))
		values[SettingVisionModel] = enc(trimPtr(in.VisionModel))
	}
	if in.APIKey.Set {
		key := ""
		if in.APIKey.Valid {
			key = strings.TrimSpace(in.APIKey.Value)
		}
		switch in.Provider {
		case ProviderClaude:
			values[SettingAPIKey] = enc(key)
		case ProviderOpenAICompatible:
			values[SettingLocalAPIKey] = enc(key)
		}
	}
	if err := s.svcs.Store.SaveSettingValues(ctx, values); err != nil {
		return Status{}, err
	}
	return s.Status(ctx)
}

// SaveKey (legado: PUT /api/ai/key) equivale a SaveConfig{provider: claude, apiKey}; nil/vazio apaga.
func (s *Service) SaveKey(ctx context.Context, apiKey *string) (Status, error) {
	in := ConfigInput{Provider: ProviderClaude, APIKey: domain.Optional[string]{Set: true}}
	if apiKey != nil {
		in.APIKey.Valid, in.APIKey.Value = true, *apiKey
	}
	return s.SaveConfig(ctx, in)
}
