// Package ai implementa o Assistente (§15 do contrato v3 e docs/local-agent-contract.md): chat com
// ferramentas de leitura sobre os dados do FinFlow e leitura de comprovantes. O modelo pode ser local
// (Ollama ou servidor compatível com a API da OpenAI, sem serviço pago) ou a API do Claude com a chave
// do próprio usuário: só o cliente do modelo muda (interface Provider). O loop de ferramentas roda no
// binário; escritas nunca acontecem pelo modelo — as ferramentas de escrita só geram propostas que o
// usuário confirma na interface.
package ai

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/anthropics/anthropic-sdk-go/v2"
	anthropicconfig "github.com/anthropics/anthropic-sdk-go/v2/config"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"finflow/internal/domain"
	"finflow/internal/service"
)

// Model é o modelo usado pelo provedor claude (decisão de produto do contrato v3 §15).
const Model = "claude-opus-5-5"

// SettingAPIKey é a chave da tabela settings onde a chave da API do Claude salva pela interface fica
// (JSON string).
const SettingAPIKey = "anthropicApiKey"

// Fontes de configuração devolvidas em Status.Source.
const (
	SourceEnv      = "ENV"
	SourceSettings = "SETTINGS"
	// SourceAuto: nada no ambiente nem em settings, e um Ollama com modelos respondeu em
	// FINFLOW_OLLAMA_URL (configuração automática).
	SourceAuto = "AUTO"
)

// fallbackBeta habilita `fallbacks: "default"`: se o modelo recusar por política, a API refaz a mesma
// requisição no modelo de fallback dentro da mesma chamada.
const fallbackBeta = anthropic.AnthropicBetaServerSideFallback2026_07_01

// Config permite trocar dependências externas (usado nos testes, que nunca chamam modelos reais).
type Config struct {
	// BaseURL substitui a URL da API do Claude (ex.: servidor httptest nos testes).
	BaseURL string
	// HTTPClient opcional para as chamadas aos modelos (Claude e provedores locais).
	HTTPClient *http.Client
	// EnvCredentials informa se há credenciais do Claude no ambiente (ANTHROPIC_API_KEY,
	// ANTHROPIC_AUTH_TOKEN, perfil do `ant auth login`, federação). Padrão: DetectEnvCredentials.
	EnvCredentials func() bool
	// MaxRetries substitui as retentativas padrão do SDK do Claude (nil = padrão).
	MaxRetries *int
	// Getenv lê as variáveis FINFLOW_AI_* (padrão: os.Getenv).
	Getenv func(string) string
	// DefaultOllamaURL é onde o Ollama é procurado quando nada está configurado e a URL padrão do
	// provedor ollama (padrão: FINFLOW_OLLAMA_URL ou http://localhost:11434). Os testes fixam aqui.
	DefaultOllamaURL string
}

// Service concentra o Assistente.
type Service struct {
	// svcs guarda a configuração (settings do espaço pessoal); data é o espaço consultado pelas
	// ferramentas e pela leitura de comprovantes (nil = o próprio svcs).
	svcs *service.Services
	data *service.Services
	cfg  Config
	// probes guarda o resultado dos pings aos servidores locais (compartilhado entre espaços);
	// auto, o da detecção automática do Ollama.
	probes *probeCache
	auto   *autoCache
}

// ForSpace devolve o Assistente consultando os dados de outro espaço (v3 fase 2); a configuração do
// provedor continua a do espaço pessoal.
func (s *Service) ForSpace(data *service.Services) *Service {
	c := *s
	c.data = data
	return &c
}

func (s *Service) dataSvcs() *service.Services {
	if s.data != nil {
		return s.data
	}
	return s.svcs
}

func NewService(svcs *service.Services, cfg Config) *Service {
	if cfg.EnvCredentials == nil {
		cfg.EnvCredentials = DetectEnvCredentials
	}
	if cfg.Getenv == nil {
		cfg.Getenv = os.Getenv
	}
	return &Service{svcs: svcs, cfg: cfg, probes: newProbeCache(), auto: &autoCache{}}
}

// Status é a resposta de GET /api/ai/status (chaves nunca são devolvidas).
type Status struct {
	Configured  bool    `json:"configured"`
	Provider    *string `json:"provider"`
	Model       *string `json:"model"`
	VisionModel *string `json:"visionModel"`
	BaseURL     *string `json:"baseUrl"`
	Source      *string `json:"source"`
	// Reachable é o ping do servidor local (com nada configurado, do Ollama em localhost:11434);
	// null para o Claude.
	Reachable        *bool `json:"reachable"`
	ReceiptSupported bool  `json:"receiptSupported"`
}

// DetectEnvCredentials replica, sem fazer requisições, as fontes da cadeia padrão do SDK: variáveis
// ANTHROPIC_API_KEY / ANTHROPIC_AUTH_TOKEN / ANTHROPIC_PROFILE, federação por variáveis de ambiente e o
// perfil salvo pelo `ant auth login`.
func DetectEnvCredentials() bool {
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_PROFILE", "ANTHROPIC_FEDERATION_RULE_ID"} {
		if strings.TrimSpace(os.Getenv(k)) != "" {
			return true
		}
	}
	_, err := anthropicconfig.LoadConfig()
	return err == nil
}

// savedKey devolve a chave do Claude salva em Configurações.
func (s *Service) savedKey(ctx context.Context) (string, error) {
	saved, err := s.savedStrings(ctx)
	if err != nil {
		return "", err
	}
	return saved[SettingAPIKey], nil
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// Configured informa, sem acessar a rede, se há um provedor utilizável (usado em /api/system/info):
// a configuração resolvida ou, sem ela, o último resultado da detecção automática.
func (s *Service) Configured(ctx context.Context) (bool, error) {
	r, err := s.resolve(ctx)
	if err != nil {
		return false, err
	}
	if r.provider == "" {
		auto, ok := s.auto.cached(s.ollamaURL(), s.autoPreferred())
		return ok && auto.configured(), nil
	}
	return r.configured(), nil
}

// effective é a configuração usada pelo chat, comprovantes e status: a resolvida ou, sem nada no
// ambiente nem em settings, a detecção automática do Ollama.
func (s *Service) effective(ctx context.Context) (resolved, error) {
	r, err := s.resolve(ctx)
	if err != nil || r.provider != "" {
		return r, err
	}
	if auto, ok := s.autoDetect(ctx); ok {
		return auto, nil
	}
	return r, nil
}

// Status resolve a configuração e, para provedores locais, verifica se o servidor responde (com cache
// curto). Sem nada configurado, procura um Ollama local para a interface oferecer.
func (s *Service) Status(ctx context.Context) (Status, error) {
	r, err := s.effective(ctx)
	if err != nil {
		return Status{}, err
	}
	st := Status{
		Configured:       r.configured(),
		Provider:         strPtr(r.provider),
		Model:            strPtr(r.model),
		VisionModel:      strPtr(r.visionModel),
		BaseURL:          strPtr(r.baseURL),
		Source:           strPtr(r.source),
		ReceiptSupported: r.receiptSupported(),
	}
	switch {
	case isLocalProvider(r.provider):
		ok := s.reachable(ctx, r.provider, r.baseURL, r.apiKey)
		st.Reachable = &ok
	case r.provider == "":
		ok := s.reachable(ctx, ProviderOllama, s.ollamaURL(), "")
		st.Reachable = &ok
	}
	return st, nil
}

// ValidateAPIKey confere o formato mínimo de uma chave do Claude (sem espaços nem caracteres de controle).
func ValidateAPIKey(key string) []domain.ValidationIssue {
	if len(key) < 20 || len(key) > 512 {
		return []domain.ValidationIssue{{Path: "apiKey", Message: "chave deve ter entre 20 e 512 caracteres"}}
	}
	for _, r := range key {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return []domain.ValidationIssue{{Path: "apiKey", Message: "chave não pode conter espaços"}}
		}
	}
	return nil
}

// claudeClient monta o cliente do SDK com a credencial resolvida; sem credencial → ErrNotConfigured.
func (s *Service) claudeClient(r resolved) (anthropic.Client, error) {
	opts := platformOptions()
	switch {
	case r.claudeEnv:
		// Cadeia padrão do SDK (variáveis de ambiente ou perfil do `ant auth login`).
	case r.apiKey != "":
		opts = append(opts, option.WithAPIKey(r.apiKey))
	default:
		return anthropic.Client{}, ErrNotConfigured
	}
	if s.cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(s.cfg.BaseURL))
	}
	if s.cfg.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(s.cfg.HTTPClient))
	}
	if s.cfg.MaxRetries != nil {
		opts = append(opts, option.WithMaxRetries(*s.cfg.MaxRetries))
	}
	return anthropic.NewClient(opts...), nil
}

// provider monta o cliente do modelo configurado; sem configuração utilizável → 412.
func (s *Service) provider(ctx context.Context) (Provider, resolved, error) {
	r, err := s.effective(ctx)
	if err != nil {
		return nil, r, err
	}
	if !r.configured() {
		if r.provider == ProviderOpenAICompatible {
			return nil, r, errModelMissing
		}
		return nil, r, ErrNotConfigured
	}
	switch r.provider {
	case ProviderClaude:
		client, err := s.claudeClient(r)
		if err != nil {
			return nil, r, err
		}
		return &claudeProvider{client: client}, r, nil
	case ProviderOllama:
		return &ollamaProvider{local: s.localClient(r)}, r, nil
	default:
		return &openAIProvider{local: s.localClient(r)}, r, nil
	}
}

// upstreamError traduz erros da API do Claude para mensagens em pt-BR. Nunca devolve 401 (o frontend
// trataria como sessão expirada do próprio FinFlow).
func upstreamError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *service.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden:
			return &service.AppError{Status: http.StatusBadGateway, Message: "A chave da API do Claude foi recusada. Confira a chave em Configurações."}
		case apiErr.StatusCode == http.StatusTooManyRequests:
			return &service.AppError{Status: http.StatusTooManyRequests, Message: "Limite de uso da API do Claude atingido. Tente de novo em instantes."}
		case apiErr.StatusCode == http.StatusBadRequest:
			return &service.AppError{Status: http.StatusBadGateway, Message: "A API do Claude recusou a requisição. Tente reformular a pergunta."}
		case apiErr.StatusCode >= 500:
			return &service.AppError{Status: http.StatusBadGateway, Message: "A API do Claude está indisponível no momento. Tente de novo em instantes."}
		}
		return &service.AppError{Status: http.StatusBadGateway, Message: "Falha ao falar com a API do Claude."}
	}
	return &service.AppError{Status: http.StatusBadGateway, Message: "Não foi possível conectar à API do Claude. Verifique sua internet."}
}
