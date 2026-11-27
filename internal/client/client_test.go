package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"finflow/internal/client"
	"finflow/internal/domain"
)

func newTestServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func TestHTTPClientSendsAuthHeaderAndPrefix(t *testing.T) {
	var gotAuth, gotPath, gotQuery string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a1","name":"Nubank","balanceCents":1234}]`))
	})

	cli := client.NewHTTPClient(ts.URL+"/", "  meu-token  ")
	accs, err := cli.Accounts(context.Background(), true)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	if gotAuth != "Bearer meu-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotPath != "/api/accounts" || gotQuery != "includeArchived=true" {
		t.Fatalf("path/query = %q ? %q", gotPath, gotQuery)
	}
	if len(accs) != 1 || accs[0].Name != "Nubank" || accs[0].BalanceCents != 1234 {
		t.Fatalf("accounts = %+v", accs)
	}
}

func TestHTTPClientNoTokenOmitsAuthHeader(t *testing.T) {
	for _, k := range []string{"FINFLOW_TOKEN", "FINFLOW_API_TOKEN", "FINFLOW_PASSWORD"} {
		t.Setenv(k, "")
	}
	var hasAuth bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, hasAuth = r.Header["Authorization"]
		_, _ = w.Write([]byte(`[]`))
	})
	if _, err := client.NewHTTPClient(ts.URL, "").Goals(context.Background()); err != nil {
		t.Fatalf("Goals: %v", err)
	}
	if hasAuth {
		t.Fatal("Authorization não deveria ser enviado sem token")
	}
}

func TestHTTPClientEnvDefaults(t *testing.T) {
	var gotAuth string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"required":true,"authenticated":true}`))
	})
	t.Setenv("FINFLOW_API_URL", ts.URL)
	t.Setenv("FINFLOW_TOKEN", "")
	t.Setenv("FINFLOW_API_TOKEN", "token-env")
	t.Setenv("FINFLOW_PASSWORD", "senha")

	st, err := client.NewHTTPClient("", "").AuthStatus(context.Background())
	if err != nil {
		t.Fatalf("AuthStatus: %v", err)
	}
	if gotAuth != "Bearer token-env" {
		t.Fatalf("Authorization = %q (FINFLOW_API_TOKEN deveria vencer FINFLOW_PASSWORD)", gotAuth)
	}
	if !st.Required || !st.Authenticated {
		t.Fatalf("status = %+v", st)
	}
}

func TestHTTPClientAPIErrorWithIssues(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"statusCode":400,"message":"Dados inválidos","issues":[{"path":"amountCents","message":"deve ser positivo"},{"path":"date","message":"formato inválido"}]}`))
	})

	_, err := client.NewHTTPClient(ts.URL, "t").CreateTransaction(context.Background(), domain.CreateTransactionInput{})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("esperava *APIError, veio %T: %v", err, err)
	}
	if apiErr.Status != 400 || apiErr.Message != "Dados inválidos" || len(apiErr.Issues) != 2 {
		t.Fatalf("APIError = %+v", apiErr)
	}
	want := "Dados inválidos (amountCents: deve ser positivo; date: formato inválido)"
	if apiErr.Error() != want {
		t.Fatalf("Error() = %q, want %q", apiErr.Error(), want)
	}
}

func TestHTTPClientAPIErrorNonJSONBody(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`<html><body>Bad Gateway</body></html>`))
	})

	_, err := client.NewHTTPClient(ts.URL, "t").Cards(context.Background(), false)
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("esperava *APIError, veio %T: %v", err, err)
	}
	if apiErr.Status != http.StatusBadGateway || apiErr.Message != "erro HTTP 502" || len(apiErr.Issues) != 0 {
		t.Fatalf("APIError = %+v", apiErr)
	}
}

func TestHTTPClientConnectionError(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	url := ts.URL
	ts.Close()

	_, err := client.NewHTTPClient(url, "t").Accounts(context.Background(), false)
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 0 || !strings.Contains(apiErr.Message, "não consegui falar com a API") {
		t.Fatalf("erro de conexão inesperado: %v", err)
	}
}

func TestHTTPClientExportReturnsRawBody(t *testing.T) {
	var gotQuery string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("Data;Descrição\n2026-01-01;Café\n"))
	})
	month := 3
	out, err := client.NewHTTPClient(ts.URL, "t").ExportTransactions(context.Background(), "csv", domain.ListTransactionsQuery{Month: &month, Page: 2})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !strings.HasPrefix(out, "Data;Descrição") {
		t.Fatalf("conteúdo = %q", out)
	}
	if gotQuery != "format=csv&month=3" {
		t.Fatalf("query = %q (page não deve ir no export)", gotQuery)
	}
}

type item struct{ id, name string }

func resolve(items []item, ref string) (item, bool) {
	return client.ResolveByName(items, ref, func(i item) string { return i.id }, func(i item) string { return i.name })
}

func TestResolveByName(t *testing.T) {
	items := []item{
		{"id-1", "Nubank"},
		{"id-2", "Nubank Ultravioleta"},
		{"id-3", "Inter Gold"},
		{"id-4", "Itaú Salário"},
	}
	cases := []struct {
		name   string
		ref    string
		wantID string
		wantOK bool
	}{
		{"id exato", "id-3", "id-3", true},
		{"nome exato vence substring ambígua", "Nubank", "id-1", true},
		{"nome exato case-insensitive", "  nUBANK ultravioleta ", "id-2", true},
		{"substring única", "gold", "id-3", true},
		{"substring com acento", "itaú", "id-4", true},
		{"substring ambígua", "u", "", false},
		{"inexistente", "Bradesco", "", false},
		{"vazio", "", "", false},
		{"só espaços", "   ", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := resolve(items, tc.ref)
			if ok != tc.wantOK || got.id != tc.wantID {
				t.Fatalf("ResolveByName(%q) = (%+v, %v), want (%s, %v)", tc.ref, got, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}

func TestResolveByNameEmptyRefWithSingleItem(t *testing.T) {
	// Regressão: com um único item, ref "" casava via strings.Contains(name, "").
	if got, ok := resolve([]item{{"id-1", "Única"}}, ""); ok {
		t.Fatalf("ref vazio não deveria resolver, veio %+v", got)
	}
	// Item com id vazio também não pode casar com ref vazio.
	if _, ok := resolve([]item{{"", "Sem id"}}, ""); ok {
		t.Fatal("ref vazio casou com id vazio")
	}
}
