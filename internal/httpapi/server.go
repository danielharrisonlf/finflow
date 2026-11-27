package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"finflow/internal/domain"
	"finflow/internal/service"
	"finflow/internal/spaces"
)

// MaxBodyBytes é o tamanho máximo aceito no corpo de uma requisição (1 MiB).
const MaxBodyBytes int64 = 1 << 20

type ServerOptions struct {
	WebOrigin string
	WebFS     fs.FS
	// Version, LAN e Port alimentam GET /api/system/info (versão do binário, modo rede local e
	// porta usada para montar as URLs da LAN).
	Version string
	LAN     bool
	Port    int
	// Spaces é o registro de espaços (v3 fase 2); nil = só o espaço pessoal (svcs).
	Spaces *spaces.Registry
}

type Server struct {
	// svcs são os serviços do espaço pessoal: auth, telegram e sistema não dependem do espaço.
	svcs    *service.Services
	spaces  *spaces.Registry
	opts    ServerOptions
	handler http.Handler
}

// SpaceHeader escolhe o espaço da requisição (ausente = pessoal); em links (?token=) vale ?space=.
const SpaceHeader = "X-FinFlow-Space"

func NewServer(svcs *service.Services, opts ServerOptions) *Server {
	reg := opts.Spaces
	if reg == nil {
		reg = spaces.Single(svcs)
	}
	s := &Server{svcs: reg.Base(), spaces: reg, opts: opts}
	mux := http.NewServeMux()

	// Auth (public)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)

	// Accounts
	mux.HandleFunc("GET /api/accounts", s.requireAuth(s.handleListAccounts))
	mux.HandleFunc("GET /api/accounts/{id}", s.requireAuth(s.handleGetAccount))
	mux.HandleFunc("POST /api/accounts", s.requireAuth(s.handleCreateAccount))
	mux.HandleFunc("PATCH /api/accounts/{id}", s.requireAuth(s.handleUpdateAccount))
	mux.HandleFunc("DELETE /api/accounts/{id}", s.requireAuth(s.handleDeleteAccount))

	// Categories
	mux.HandleFunc("GET /api/categories", s.requireAuth(s.handleListCategories))
	mux.HandleFunc("POST /api/categories", s.requireAuth(s.handleCreateCategory))
	mux.HandleFunc("PATCH /api/categories/{id}", s.requireAuth(s.handleUpdateCategory))
	mux.HandleFunc("DELETE /api/categories/{id}", s.requireAuth(s.handleDeleteCategory))

	// Cards & Invoices
	mux.HandleFunc("GET /api/cards", s.requireAuth(s.handleListCards))
	mux.HandleFunc("GET /api/cards/{id}", s.requireAuth(s.handleGetCard))
	mux.HandleFunc("POST /api/cards", s.requireAuth(s.handleCreateCard))
	mux.HandleFunc("PATCH /api/cards/{id}", s.requireAuth(s.handleUpdateCard))
	mux.HandleFunc("DELETE /api/cards/{id}", s.requireAuth(s.handleDeleteCard))
	mux.HandleFunc("POST /api/cards/{id}/expenses", s.requireAuth(s.handleAddCardExpense))
	mux.HandleFunc("GET /api/cards/{id}/invoices", s.requireAuth(s.handleListInvoices))
	mux.HandleFunc("GET /api/cards/{id}/invoices/{invoiceId}/items", s.requireAuth(s.handleListInvoiceItems))
	mux.HandleFunc("POST /api/cards/{id}/invoices/{invoiceId}/pay", s.requireAuth(s.handlePayInvoice))

	// Transactions
	mux.HandleFunc("GET /api/transactions", s.requireAuth(s.handleListTransactions))
	mux.HandleFunc("GET /api/transactions/export", s.requireAuth(s.handleExportTransactions))
	mux.HandleFunc("GET /api/transactions/{id}", s.requireAuth(s.handleGetTransaction))
	mux.HandleFunc("POST /api/transactions", s.requireAuth(s.handleCreateTransaction))
	mux.HandleFunc("POST /api/transactions/transfer", s.requireAuth(s.handleTransfer))
	mux.HandleFunc("PATCH /api/transactions/{id}", s.requireAuth(s.handleUpdateTransaction))
	mux.HandleFunc("PATCH /api/transactions/{id}/confirm", s.requireAuth(s.handleConfirmTransaction))
	mux.HandleFunc("DELETE /api/transactions/{id}", s.requireAuth(s.handleDeleteTransaction))

	// Budgets
	mux.HandleFunc("GET /api/budgets", s.requireAuth(s.handleListBudgets))
	mux.HandleFunc("POST /api/budgets", s.requireAuth(s.handleUpsertBudget))
	mux.HandleFunc("DELETE /api/budgets/{id}", s.requireAuth(s.handleDeleteBudget))

	// Goals
	mux.HandleFunc("GET /api/goals", s.requireAuth(s.handleListGoals))
	mux.HandleFunc("GET /api/goals/{id}", s.requireAuth(s.handleGetGoal))
	mux.HandleFunc("POST /api/goals", s.requireAuth(s.handleCreateGoal))
	mux.HandleFunc("PATCH /api/goals/{id}", s.requireAuth(s.handleUpdateGoal))
	mux.HandleFunc("POST /api/goals/{id}/contributions", s.requireAuth(s.handleContributeGoal))
	mux.HandleFunc("DELETE /api/goals/{id}", s.requireAuth(s.handleDeleteGoal))

	// Dashboard
	mux.HandleFunc("GET /api/dashboard/summary", s.requireAuth(s.handleDashboardSummary))
	mux.HandleFunc("GET /api/dashboard/cashflow", s.requireAuth(s.handleDashboardCashflow))
	mux.HandleFunc("GET /api/dashboard/categories", s.requireAuth(s.handleDashboardCategories))
	mux.HandleFunc("GET /api/dashboard/upcoming", s.requireAuth(s.handleDashboardUpcoming))

	// Recurring (compromissos)
	mux.HandleFunc("GET /api/recurring", s.requireAuth(s.handleListRecurring))
	mux.HandleFunc("POST /api/recurring", s.requireAuth(s.handleCreateRecurring))
	mux.HandleFunc("PATCH /api/recurring/{id}", s.requireAuth(s.handleUpdateRecurring))
	mux.HandleFunc("DELETE /api/recurring/{id}", s.requireAuth(s.handleDeleteRecurring))
	mux.HandleFunc("POST /api/recurring/{id}/post", s.requireAuth(s.handlePostRecurring))

	// Settings
	mux.HandleFunc("GET /api/settings", s.requireAuth(s.handleGetSettings))
	mux.HandleFunc("PATCH /api/settings", s.requireAuth(s.handleUpdateSettings))

	// Category rules (automações)
	mux.HandleFunc("GET /api/rules", s.requireAuth(s.handleListRules))
	mux.HandleFunc("POST /api/rules", s.requireAuth(s.handleCreateRule))
	mux.HandleFunc("POST /api/rules/apply", s.requireAuth(s.handleApplyRules))
	mux.HandleFunc("PATCH /api/rules/{id}", s.requireAuth(s.handleUpdateRule))
	mux.HandleFunc("DELETE /api/rules/{id}", s.requireAuth(s.handleDeleteRule))

	// Wishlist
	mux.HandleFunc("GET /api/wishlist", s.requireAuth(s.handleListWishlist))
	mux.HandleFunc("POST /api/wishlist", s.requireAuth(s.handleCreateWishItem))
	mux.HandleFunc("PATCH /api/wishlist/{id}", s.requireAuth(s.handleUpdateWishItem))
	mux.HandleFunc("DELETE /api/wishlist/{id}", s.requireAuth(s.handleDeleteWishItem))

	// Insights
	mux.HandleFunc("GET /api/insights/overview", s.requireAuth(s.handleInsightsOverview))
	mux.HandleFunc("GET /api/insights/projection", s.requireAuth(s.handleInsightsProjection))
	mux.HandleFunc("GET /api/insights/year", s.requireAuth(s.handleInsightsYear))
	mux.HandleFunc("GET /api/insights/commitments", s.requireAuth(s.handleInsightsCommitments))

	// Telegram (sempre o espaço pessoal)
	mux.HandleFunc("POST /api/telegram/webhook", s.handleTelegramWebhook)
	mux.HandleFunc("GET /api/telegram/status", s.requireAuthGlobal(s.handleTelegramStatus))

	s.registerSpacesRoutes(mux)
	s.registerSpreadsheetRoutes(mux)
	s.registerAIRoutes(mux)
	s.registerWealthRoutes(mux)
	s.registerInsightsV3Routes(mux)

	// Healthcheck
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	// Fallback SPA / Web UI
	if opts.WebFS != nil {
		mux.Handle("/", s.spaHandler(opts.WebFS))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, r, service.NotFound("rota não encontrada"))
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("FinFlow API rodando em /api. Rode `pnpm build` para embutir a interface web.\n"))
		})
	}

	s.handler = recoverMiddleware(s.corsMiddleware(limitBodyMiddleware(mux)))
	return s
}

// NewHTTPServer cria o *http.Server com timeouts adequados para expor o handler da API.
func NewHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	var allowed []string
	if s.opts.WebOrigin != "" {
		for _, o := range strings.Split(s.opts.WebOrigin, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				allowed = append(allowed, trimmed)
			}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if len(allowed) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				for _, a := range allowed {
					if a == "*" || a == origin {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						break
					}
				}
			}
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Token, "+SpaceHeader)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverMiddleware converte panics em 500 JSON, registrando a stack no log.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			slog.Error("panic no handler HTTP",
				"method", r.Method,
				"path", r.URL.Path,
				"panic", fmt.Sprint(rec),
				"stack", string(debug.Stack()),
			)
			writeInternalError(w)
		}()
		next.ServeHTTP(w, r)
	})
}

// limitBodyMiddleware limita o corpo das requisições a MaxBodyBytes (rotas de importação: ver bodyLimitFor).
func limitBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, bodyLimitFor(r))
		}
		next.ServeHTTP(w, r)
	})
}

func extractRequestToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if apiTok := strings.TrimSpace(r.Header.Get("X-Api-Token")); apiTok != "" {
		return apiTok
	}
	if qTok := strings.TrimSpace(r.URL.Query().Get("token")); qTok != "" {
		return qTok
	}
	return ""
}

// requireAuth protege as rotas que dependem do espaço: depois da autenticação, resolve o espaço do
// cabeçalho X-FinFlow-Space (ou ?space=) e guarda os serviços dele no contexto (404 se não existir).
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuthGlobal(func(w http.ResponseWriter, r *http.Request) {
		slug := requestSpace(r)
		if slug == "" || slug == domain.DefaultSpaceSlug {
			next(w, r)
			return
		}
		svcs, err := s.spaces.Services(r.Context(), slug)
		if err != nil {
			writeError(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), spaceCtxKey{}, svcs)))
	})
}

// requireAuthGlobal protege rotas que não dependem do espaço (espaços, sistema, telegram, chave
// do assistente): um cabeçalho de espaço inválido não as derruba.
func (s *Server) requireAuthGlobal(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.svcs.Auth.IsRequired() {
			if !s.svcs.Auth.VerifyToken(extractRequestToken(r)) {
				writeError(w, r, service.Unauthorized("autenticação necessária"))
				return
			}
		}
		next(w, r)
	}
}

type spaceCtxKey struct{}

// requestSpace lê o slug do espaço da requisição (cabeçalho; em downloads por link, ?space=).
func requestSpace(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get(SpaceHeader)); h != "" {
		return strings.ToLower(h)
	}
	return strings.ToLower(strings.TrimSpace(r.URL.Query().Get("space")))
}

// svc devolve os serviços do espaço da requisição (resolvido em requireAuth; padrão: pessoal).
func (s *Server) svc(r *http.Request) *service.Services {
	if svcs, ok := r.Context().Value(spaceCtxKey{}).(*service.Services); ok {
		return svcs
	}
	return s.svcs
}

func (s *Server) spaHandler(webFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(webFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			writeError(w, r, service.NotFound("rota não encontrada"))
			return
		}
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" || clean == "." {
			serveIndexHTML(w, webFS)
			return
		}
		if f, err := webFS.Open(clean); err == nil {
			stat, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !stat.IsDir() {
				// PWA: o manifest precisa do MIME certo (o registro do Windows não conhece .webmanifest) e o
				// service worker não pode ficar em cache, senão uma versão nova do app nunca é instalada.
				switch {
				case path.Ext(clean) == ".webmanifest":
					w.Header().Set("Content-Type", "application/manifest+json")
				case clean == "sw.js":
					w.Header().Set("Cache-Control", "no-cache")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Arquivos estáticos inexistentes devolvem 404 (evita servir index.html com MIME errado);
		// só rotas sem extensão caem no index.html do SPA.
		if path.Ext(clean) != "" || clean == "assets" || strings.HasPrefix(clean, "assets/") {
			http.NotFound(w, r)
			return
		}
		serveIndexHTML(w, webFS)
	})
}

func serveIndexHTML(w http.ResponseWriter, webFS fs.FS) {
	data, err := fs.ReadFile(webFS, "index.html")
	if err != nil {
		http.NotFound(w, &http.Request{})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *service.AppError
	if errors.As(err, &appErr) {
		if appErr.Status >= http.StatusInternalServerError {
			slog.Error("erro na requisição HTTP", "method", r.Method, "path", r.URL.Path, "status", appErr.Status, "err", err)
		}
		writeJSON(w, appErr.Status, appErr)
		return
	}
	// Erros inesperados são registrados no log; o cliente recebe só uma mensagem genérica.
	slog.Error("erro interno na requisição HTTP", "method", r.Method, "path", r.URL.Path, "err", err)
	writeInternalError(w)
}

func writeInternalError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"statusCode": http.StatusInternalServerError,
		"message":    "Erro interno do servidor",
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return &service.AppError{
				Status:  http.StatusRequestEntityTooLarge,
				Message: fmt.Sprintf("corpo da requisição excede %d bytes", tooLarge.Limit),
			}
		}
		return service.BadRequest("JSON inválido: " + err.Error())
	}
	return nil
}

// secretMatches compara segredos em tempo constante.
func secretMatches(got, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func requireUUIDParam(r *http.Request, name string) (string, error) {
	val := r.PathValue(name)
	if !domain.IsValidUUID(val) {
		return "", service.BadRequest(name + " deve ser um UUID válido")
	}
	return val, nil
}

func parseOptionalInt(raw, name string, min, max int) (*int, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return nil, service.ValidationFailed([]domain.ValidationIssue{
			{Path: name, Message: "valor numérico inválido"},
		})
	}
	return &v, nil
}

func parsePeriodQuery(r *http.Request) (domain.PeriodQuery, error) {
	q := r.URL.Query()
	m, err := parseOptionalInt(q.Get("month"), "month", 1, 12)
	if err != nil {
		return domain.PeriodQuery{}, err
	}
	y, err := parseOptionalInt(q.Get("year"), "year", 2000, 2100)
	if err != nil {
		return domain.PeriodQuery{}, err
	}
	return domain.PeriodQuery{Month: m, Year: y}, nil
}

func parseOptionalBool(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, service.BadRequest("valor booleano inválido")
	}
	return b, nil
}

func telegramWebhookSecret() string {
	return strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET"))
}
