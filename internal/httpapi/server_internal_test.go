package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"finflow/internal/service"
)

// captureLogs redireciona o slog padrão para um buffer durante o teste.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func decodeErrBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo não é JSON: %q (%v)", rr.Body.String(), err)
	}
	return body
}

func TestWriteErrorInternalDoesNotLeak(t *testing.T) {
	logs := captureLogs(t)
	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	rr := httptest.NewRecorder()

	writeError(rr, req, errors.New("pq: senha do banco xyz recusada"))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	body := decodeErrBody(t, rr)
	if body["message"] != "Erro interno do servidor" || body["statusCode"] != float64(500) {
		t.Fatalf("corpo = %v", body)
	}
	if strings.Contains(rr.Body.String(), "xyz") {
		t.Fatalf("detalhe interno vazou: %s", rr.Body.String())
	}
	out := logs.String()
	for _, want := range []string{"method=GET", "path=/api/accounts", "senha do banco xyz"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log sem %q: %s", want, out)
		}
	}
}

func TestWriteErrorAppErrorKeepsShape(t *testing.T) {
	logs := captureLogs(t)
	req := httptest.NewRequest(http.MethodPost, "/api/accounts", nil)
	rr := httptest.NewRecorder()

	writeError(rr, req, service.Conflict("fatura já paga"))

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d", rr.Code)
	}
	body := decodeErrBody(t, rr)
	if body["message"] != "fatura já paga" || body["statusCode"] != float64(409) {
		t.Fatalf("corpo = %v", body)
	}
	if logs.Len() != 0 {
		t.Fatalf("erro 4xx não deveria ser logado: %s", logs.String())
	}
}

func TestRecoverMiddlewarePanicReturns500JSON(t *testing.T) {
	logs := captureLogs(t)
	h := recoverMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("estourou: segredo-interno")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/goals/1", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
	body := decodeErrBody(t, rr)
	if body["message"] != "Erro interno do servidor" || strings.Contains(rr.Body.String(), "segredo-interno") {
		t.Fatalf("corpo = %s", rr.Body.String())
	}
	out := logs.String()
	for _, want := range []string{"segredo-interno", "path=/api/goals/1", "method=DELETE", "goroutine"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log sem %q: %s", want, out)
		}
	}
}

func TestRecoverMiddlewareRepanicsAbortHandler(t *testing.T) {
	h := recoverMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler {
			t.Fatalf("esperava re-panic com ErrAbortHandler, veio %v", rec)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestSecretMatches(t *testing.T) {
	if !secretMatches("abc", "abc") {
		t.Fatal("iguais deveriam casar")
	}
	for _, got := range []string{"", "ab", "abcd", "abd"} {
		if secretMatches(got, "abc") {
			t.Fatalf("%q não deveria casar", got)
		}
	}
}
