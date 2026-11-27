// Package ai implementa o Assistente (§15 do contrato v3): chat com ferramentas de leitura sobre os
// dados do FinFlow e leitura de comprovantes, usando a API do Claude com a chave do próprio usuário.
// O loop de ferramentas roda no binário (Messages API); escritas nunca acontecem pelo modelo — as
// ferramentas de escrita só geram propostas que o usuário confirma na interface.
package ai

import (
	"context"
	"encoding/json"
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

// Model é o modelo usado pelo Assistente (decisão de produto do contrato v3 §15).
const Model = "claude-opus-5-5"

// SettingAPIKey é a chave da tabela settings onde a chave da API salva pela interface fica (JSON string).
const SettingAPIKey = "anthropicApiKey"

// Fontes de credencial devolvidas em Status.Source.
const (
	SourceEnv      = "ENV"
	SourceSettings = "SETTINGS"
)

// fallbackBeta habilita `fallbacks: "default"`: se o modelo recusar por política, a API refaz a mesma
// requisição no modelo de fallback dentro da mesma chamada.
const fallbackBeta = anthropic.AnthropicBetaServerSideFallback2026_07_01

// Config permite trocar dependências externas (usado nos testes, que nunca chamam a API real).
type Config struct {
	// BaseURL substitui a URL da API (ex.: servidor httptest nos testes).
	BaseURL string
	// HTTPClient opcional para as chamadas à API.
	HTTPClient *http.Client
	// EnvCredentials informa se há credenciais no ambiente (ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN,
	// perfil do `ant auth login`, federação). Padrão: DetectEnvCredentials.
	EnvCredentials func() bool
	// MaxRetries substitui as retentativas padrão do SDK (nil = padrão).
	MaxRetries *int
}

// Service concentra o Assistente.
type Service struct {
	svcs *service.Services
	cfg  Config
}

func NewService(svcs *service.Services, cfg Config) *Service {
	if cfg.EnvCredentials == nil {
		cfg.EnvCredentials = DetectEnvCredentials
	}
	return &Service{svcs: svcs, cfg: cfg}
}

// Status é a resposta de GET /api/ai/status (a chave nunca é devolvida).
type Status struct {
	Configured bool    `json:"configured"`
	Source     *string `json:"source"`
	Model      string  `json:"model"`
}

// ErrNotConfigured é devolvido quando não há credencial (HTTP 412).
var ErrNotConfigured = &service.AppError{
	Status:  http.StatusPreconditionFailed,
	Message: "Assistente não configurado: defina ANTHROPIC_API_KEY ou salve uma chave da API do Claude em Configurações.",
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

func (s *Service) savedKey(ctx context.Context) (string, error) {
	values, err := s.svcs.Store.SettingValues(ctx)
	if err != nil {
		return "", err
	}
	raw, ok := values[SettingAPIKey]
	if !ok {
		return "", nil
	}
	var key string
	if err := json.Unmarshal([]byte(raw), &key); err != nil {
		return "", nil
	}
	return strings.TrimSpace(key), nil
}

// credential decide a fonte: ambiente primeiro (como as demais configurações do binário), depois a
// chave salva em Configurações. Devolve a fonte e, para SETTINGS, a chave.
func (s *Service) credential(ctx context.Context) (source, key string, err error) {
	if s.cfg.EnvCredentials() {
		return SourceEnv, "", nil
	}
	key, err = s.savedKey(ctx)
	if err != nil {
		return "", "", err
	}
	if key != "" {
		return SourceSettings, key, nil
	}
	return "", "", nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	source, _, err := s.credential(ctx)
	if err != nil {
		return Status{}, err
	}
	st := Status{Model: Model}
	if source != "" {
		st.Configured = true
		st.Source = &source
	}
	return st, nil
}

// ValidateAPIKey confere o formato mínimo de uma chave (sem espaços nem caracteres de controle).
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

// SaveKey grava (ou, com nil/vazio, apaga) a chave salva em Configurações e devolve o status.
func (s *Service) SaveKey(ctx context.Context, apiKey *string) (Status, error) {
	value := ""
	if apiKey != nil {
		value = strings.TrimSpace(*apiKey)
	}
	if value != "" {
		if issues := ValidateAPIKey(value); len(issues) > 0 {
			return Status{}, service.ValidationFailed(issues)
		}
	}
	encoded, _ := json.Marshal(value)
	if err := s.svcs.Store.SaveSettingValues(ctx, map[string]string{SettingAPIKey: string(encoded)}); err != nil {
		return Status{}, err
	}
	return s.Status(ctx)
}

// client monta o cliente do SDK com a credencial resolvida; sem credencial → ErrNotConfigured.
func (s *Service) client(ctx context.Context) (anthropic.Client, error) {
	source, key, err := s.credential(ctx)
	if err != nil {
		return anthropic.Client{}, err
	}
	var opts []option.RequestOption
	switch source {
	case SourceEnv:
		// Cadeia padrão do SDK (variáveis de ambiente ou perfil do `ant auth login`).
	case SourceSettings:
		opts = append(opts, option.WithAPIKey(key))
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
