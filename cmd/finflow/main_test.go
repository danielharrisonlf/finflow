package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func portOf(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	_, p, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(p)
	return n
}

func TestChoosePortSkipsOtherPrograms(t *testing.T) {
	// Outro servidor local (como um app Node) com /api/health respondendo 200: não é o FinFlow.
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer other.Close()
	base := portOf(t, other)

	port, running, err := choosePort(base)
	if err != nil {
		t.Fatal(err)
	}
	if running || port == base {
		t.Fatalf("choosePort(%d) = %d, running=%v; esperava outra porta livre", base, port, running)
	}
	if port < base || port > base+portFallbacks {
		t.Fatalf("porta fora da sequência de fallback: %d", port)
	}
}

func TestChoosePortReusesRunningFinFlow(t *testing.T) {
	ff := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/system/info" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"v0.3.0","dbDialect":"sqlite"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ff.Close()
	base := portOf(t, ff)

	port, running, err := choosePort(base)
	if err != nil || !running || port != base {
		t.Fatalf("choosePort(%d) = %d, running=%v, err=%v; esperava reaproveitar o FinFlow", base, port, running, err)
	}
}

func TestIsFinFlowRejectsUnrelatedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"outro app"}`))
	}))
	defer srv.Close()
	if isFinFlow(srv.URL) {
		t.Fatal("um JSON qualquer não identifica o FinFlow")
	}
}
