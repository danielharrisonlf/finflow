package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"finflow/internal/db"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func newTestServer(t *testing.T, database *db.DB, opts httpapi.ServerOptions) *httpapi.Server {
	t.Helper()
	return httpapi.NewServer(service.New(store.New(database), nil), opts)
}

func rawReq(t *testing.T, srv http.Handler, method, target string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestBodyTooLargeReturns413(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		t.Setenv("FINFLOW_PASSWORD", "")
		srv := newTestServer(t, database, httpapi.ServerOptions{})

		big := []byte(`{"name":"` + strings.Repeat("a", int(httpapi.MaxBodyBytes)+1) + `","type":"CHECKING"}`)
		res := rawReq(t, srv, http.MethodPost, "/api/accounts", big, map[string]string{"Content-Type": "application/json"})
		if res.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("esperava 413, veio %d %s", res.Code, res.Body.String())
		}
		body := decodeBody[map[string]any](t, res)
		if body["statusCode"] != float64(413) {
			t.Fatalf("corpo = %v", body)
		}

		// Dentro do limite continua funcionando.
		ok := rawReq(t, srv, http.MethodPost, "/api/accounts", []byte(`{"name":"Conta","type":"CHECKING"}`), nil)
		if ok.Code != http.StatusCreated {
			t.Fatalf("corpo pequeno: %d %s", ok.Code, ok.Body.String())
		}
	})
}

func TestInternalErrorDoesNotLeak(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		t.Setenv("FINFLOW_PASSWORD", "")
		var logs bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
		t.Cleanup(func() { slog.SetDefault(prev) })

		srv := newTestServer(t, database, httpapi.ServerOptions{})
		// Banco fechado força um erro inesperado (não-AppError) na camada de dados.
		_ = database.Close()

		res := doReq(t, srv, http.MethodGet, "/api/accounts", nil, nil)
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("esperava 500, veio %d %s", res.Code, res.Body.String())
		}
		body := decodeBody[map[string]any](t, res)
		if body["message"] != "Erro interno do servidor" || body["statusCode"] != float64(500) || len(body) != 2 {
			t.Fatalf("corpo vazou detalhes: %s", res.Body.String())
		}
		if !strings.Contains(logs.String(), "path=/api/accounts") {
			t.Fatalf("erro não foi logado: %s", logs.String())
		}
	})
}

func spaFS() fs.FS {
	return fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>FinFlow</title>")},
		"assets/app-abc.js":    {Data: []byte("console.log('ok')")},
		"assets/app-abc.css":   {Data: []byte("body{}")},
		"manifest.webmanifest": {Data: []byte(`{"name":"FinFlow"}`)},
		"sw.js":                {Data: []byte("self.addEventListener('fetch',()=>{})")},
	}
}

func TestSPAFallbackAndMissingAssets(t *testing.T) {
	srv := newTestServer(t, testutil.Open(t, db.DialectSQLite), httpapi.ServerOptions{WebFS: spaFS()})

	cases := []struct {
		path       string
		wantStatus int
		wantCT     string
		wantBody   string
	}{
		{"/", 200, "text/html", "FinFlow"},
		{"/transacoes", 200, "text/html", "FinFlow"},
		{"/cartoes/123/faturas", 200, "text/html", "FinFlow"},
		{"/assets/app-abc.js", 200, "javascript", "console.log"},
		{"/assets/app-abc.css", 200, "text/css", "body{}"},
		{"/assets/app-OLD.js", 404, "", ""},
		{"/assets/chunk", 404, "", ""},
		{"/assets", 404, "", ""},
		{"/favicon.ico", 404, "", ""},
		{"/rota/arquivo.png", 404, "", ""},
		{"/api/nao-existe", 404, "application/json", "rota não encontrada"},
		{"/manifest.webmanifest", 200, "application/manifest+json", "FinFlow"},
		{"/sw.js", 200, "javascript", "addEventListener"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			res := rawReq(t, srv, http.MethodGet, tc.path, nil, nil)
			if res.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", res.Code, tc.wantStatus, res.Body.String())
			}
			if tc.wantStatus == 404 && strings.Contains(res.Body.String(), "<title>FinFlow") {
				t.Fatal("asset inexistente não pode devolver index.html")
			}
			if tc.wantCT != "" && !strings.Contains(res.Header().Get("Content-Type"), tc.wantCT) {
				t.Fatalf("Content-Type = %q, want %q", res.Header().Get("Content-Type"), tc.wantCT)
			}
			if !strings.Contains(res.Body.String(), tc.wantBody) {
				t.Fatalf("corpo = %q", res.Body.String())
			}
			if tc.path == "/sw.js" && res.Header().Get("Cache-Control") != "no-cache" {
				t.Fatalf("sw.js Cache-Control = %q", res.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestSPAPathTraversalNotServed(t *testing.T) {
	root := t.TempDir()
	web := filepath.Join(root, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("<!doctype html>SPA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module segredo-traversal"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t, testutil.Open(t, db.DialectSQLite), httpapi.ServerOptions{WebFS: os.DirFS(web)})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	for _, target := range []string{"/..%2f..%2fgo.mod", "/..%2fgo.mod", "/../go.mod", "/assets/..%2f..%2fgo.mod", "/%2e%2e/go.mod"} {
		t.Run(target, func(t *testing.T) {
			// Recorder (handler direto) e servidor real (com o ServeMux do net/http na frente).
			res := rawReq(t, srv, http.MethodGet, target, nil, nil)
			if strings.Contains(res.Body.String(), "segredo-traversal") {
				t.Fatalf("arquivo fora do webroot servido via handler: %d", res.Code)
			}
			resp, err := http.Get(ts.URL + target)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(resp.Body)
			if strings.Contains(buf.String(), "segredo-traversal") {
				t.Fatalf("arquivo fora do webroot servido via HTTP: %d", resp.StatusCode)
			}
		})
	}
}

func TestTelegramWebhookSecret(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		t.Setenv("TELEGRAM_WEBHOOK_SECRET", "segredo-webhook")
		t.Setenv("TELEGRAM_BOT_TOKEN", "")
		srv := newTestServer(t, database, httpapi.ServerOptions{})
		update := map[string]any{"update_id": 1}

		cases := []struct {
			name   string
			header map[string]string
			want   int
		}{
			{"sem header", nil, http.StatusForbidden},
			{"secret errado", map[string]string{"X-Telegram-Bot-Api-Secret-Token": "segredo-webhoo"}, http.StatusForbidden},
			{"secret com sufixo", map[string]string{"X-Telegram-Bot-Api-Secret-Token": "segredo-webhook!"}, http.StatusForbidden},
			{"secret correto", map[string]string{"X-Telegram-Bot-Api-Secret-Token": "segredo-webhook"}, http.StatusOK},
		}
		for _, tc := range cases {
			res := doReq(t, srv, http.MethodPost, "/api/telegram/webhook", update, tc.header)
			if res.Code != tc.want {
				t.Fatalf("%s: status = %d, want %d (%s)", tc.name, res.Code, tc.want, res.Body.String())
			}
		}
	})
}

func TestCORSPreflight(t *testing.T) {
	database := testutil.Open(t, db.DialectSQLite)
	t.Setenv("FINFLOW_PASSWORD", "senha-cors")

	t.Run("origem permitida", func(t *testing.T) {
		srv := newTestServer(t, database, httpapi.ServerOptions{WebOrigin: "http://app.local, http://outro.local"})
		res := rawReq(t, srv, http.MethodOptions, "/api/accounts", nil, map[string]string{
			"Origin":                        "http://outro.local",
			"Access-Control-Request-Method": "POST",
		})
		if res.Code != http.StatusNoContent {
			t.Fatalf("preflight deveria ser 204 mesmo com auth ligada, veio %d", res.Code)
		}
		h := res.Header()
		if h.Get("Access-Control-Allow-Origin") != "http://outro.local" ||
			h.Get("Access-Control-Allow-Credentials") != "true" ||
			!strings.Contains(h.Get("Access-Control-Allow-Methods"), "PATCH") ||
			!strings.Contains(h.Get("Access-Control-Allow-Headers"), "Authorization") ||
			h.Get("Vary") != "Origin" {
			t.Fatalf("headers CORS = %v", h)
		}
	})

	t.Run("origem não permitida", func(t *testing.T) {
		srv := newTestServer(t, database, httpapi.ServerOptions{WebOrigin: "http://app.local"})
		res := rawReq(t, srv, http.MethodOptions, "/api/accounts", nil, map[string]string{
			"Origin":                        "http://evil.local",
			"Access-Control-Request-Method": "DELETE",
		})
		if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("origem não permitida recebeu Allow-Origin %q", got)
		}
	})

	t.Run("request real ainda exige auth", func(t *testing.T) {
		srv := newTestServer(t, database, httpapi.ServerOptions{WebOrigin: "http://app.local"})
		res := rawReq(t, srv, http.MethodGet, "/api/accounts", nil, map[string]string{"Origin": "http://app.local"})
		if res.Code != http.StatusUnauthorized || res.Header().Get("Access-Control-Allow-Origin") != "http://app.local" {
			t.Fatalf("status=%d headers=%v", res.Code, res.Header())
		}
	})
}

func TestNewHTTPServerTimeouts(t *testing.T) {
	h := http.NewServeMux()
	s := httpapi.NewHTTPServer(":0", h)
	if s.Addr != ":0" || s.Handler != h {
		t.Fatalf("addr/handler = %q/%v", s.Addr, s.Handler)
	}
	if s.ReadHeaderTimeout != 10*time.Second || s.ReadTimeout != 30*time.Second ||
		s.WriteTimeout != 60*time.Second || s.IdleTimeout != 120*time.Second {
		t.Fatalf("timeouts = %v/%v/%v/%v", s.ReadHeaderTimeout, s.ReadTimeout, s.WriteTimeout, s.IdleTimeout)
	}
}

// Garante que o corpo de erro 413 continua no formato padrão da API.
func TestBodyTooLargeShape(t *testing.T) {
	srv := newTestServer(t, testutil.Open(t, db.DialectSQLite), httpapi.ServerOptions{})
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	big := bytes.Repeat([]byte(" "), int(httpapi.MaxBodyBytes)+10)
	res := rawReq(t, srv, http.MethodPost, "/api/telegram/webhook", append(big, []byte(`{"update_id":1}`)...), nil)
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d %s", res.Code, res.Body.String())
	}
	var body struct {
		StatusCode int    `json:"statusCode"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.StatusCode != 413 || body.Message == "" {
		t.Fatalf("corpo = %s (%v)", res.Body.String(), err)
	}
}
