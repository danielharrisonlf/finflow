package httpapi_test

import (
	"encoding/json"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/spaces"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestSystemSeed(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "")
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		base := service.New(store.New(database), nil)
		srv := httpapi.NewServer(base, httpapi.ServerOptions{})

		res := doReq(t, srv, "POST", "/api/system/seed", nil, nil)
		expectCode(t, res, 201, "seed pessoal")
		out := decodeBody[map[string]any](t, res)
		if out["seeded"] != true || out["space"] != "pessoal" || out["kind"] != string(domain.SpacePersonal) {
			t.Fatalf("resposta do seed: %+v", out)
		}
		if accounts := decodeBody[[]domain.Account](t, doReq(t, srv, "GET", "/api/accounts", nil, nil)); len(accounts) == 0 {
			t.Fatal("seed pessoal deveria criar contas")
		}
		expectCode(t, doReq(t, srv, "POST", "/api/system/seed", nil, nil), 409, "seed num banco com dados")
	})
}

func TestSystemSeedRequiresEmptyDatabase(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "")
	base := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	srv := httpapi.NewServer(base, httpapi.ServerOptions{})
	// Qualquer dado do usuário (aqui só uma tag, sem contas) bloqueia o seed.
	expectCode(t, doReq(t, srv, "POST", "/api/tags", map[string]any{"name": "viagem"}, nil), 201, "criar tag")
	expectCode(t, doReq(t, srv, "POST", "/api/system/seed", nil, nil), 409, "seed com uma tag")
	if accounts := decodeBody[[]domain.Account](t, doReq(t, srv, "GET", "/api/accounts", nil, nil)); len(accounts) != 0 {
		t.Fatalf("seed recusado não deveria criar contas: %+v", accounts)
	}
}

func TestSystemSeedSpaceKinds(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "")
	base := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	reg := spaces.New(base, spaces.Options{DataDir: t.TempDir()})
	t.Cleanup(func() { _ = reg.Close() })
	srv := httpapi.NewServer(base, httpapi.ServerOptions{Spaces: reg})
	expectCode(t, doReq(t, srv, "POST", "/api/spaces", map[string]any{"name": "Casa", "kind": "HOME"}, nil), 201, "criar casa")
	expectCode(t, doReq(t, srv, "POST", "/api/spaces", map[string]any{"name": "Empresa", "kind": "BUSINESS"}, nil), 201, "criar empresa")
	casaH := map[string]string{httpapi.SpaceHeader: "casa"}
	empresaH := map[string]string{httpapi.SpaceHeader: "empresa"}

	res := doReq(t, srv, "POST", "/api/system/seed", nil, casaH)
	expectCode(t, res, 201, "seed casa")
	if out := decodeBody[map[string]any](t, res); out["kind"] != string(domain.SpaceHome) {
		t.Fatalf("seed casa: %+v", out)
	}
	if members := decodeBody[[]map[string]any](t, doReq(t, srv, "GET", "/api/members", nil, casaH)); len(members) != 2 {
		t.Fatalf("seed casa deveria criar 2 membros: %+v", members)
	}
	expectCode(t, doReq(t, srv, "POST", "/api/system/seed", nil, casaH), 409, "seed casa repetido")

	expectCode(t, doReq(t, srv, "POST", "/api/system/seed", nil, empresaH), 201, "seed empresa")
	metrics := doReq(t, srv, "GET", "/api/business/metrics", nil, empresaH)
	expectCode(t, metrics, 200, "métricas da empresa")
	var m map[string]any
	if err := json.Unmarshal(metrics.Body.Bytes(), &m); err != nil || len(m) == 0 {
		t.Fatalf("métricas: %s %v", metrics.Body.String(), err)
	}

	// O pessoal continua vazio e aceita o próprio seed; espaço desconhecido = 404.
	if accounts := decodeBody[[]domain.Account](t, doReq(t, srv, "GET", "/api/accounts", nil, nil)); len(accounts) != 0 {
		t.Fatalf("pessoal deveria continuar vazio: %+v", accounts)
	}
	expectCode(t, doReq(t, srv, "POST", "/api/system/seed", nil, map[string]string{httpapi.SpaceHeader: "nao-existe"}), 404, "espaço desconhecido")
}

func TestSystemInfoBrowserRuntime(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "")
	base := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	server := decodeBody[map[string]any](t, doReq(t, httpapi.NewServer(base, httpapi.ServerOptions{}), "GET", "/api/system/info", nil, nil))
	if server["runtime"] != httpapi.RuntimeServer {
		t.Fatalf("runtime padrão: %+v", server)
	}
	srv := httpapi.NewServer(base, httpapi.ServerOptions{Runtime: httpapi.RuntimeBrowser, LAN: true, Port: 3333})
	info := decodeBody[map[string]any](t, doReq(t, srv, "GET", "/api/system/info", nil, nil))
	urls, ok := info["lanUrls"].([]any)
	if info["runtime"] != httpapi.RuntimeBrowser || info["dataPath"] != nil || info["lanEnabled"] != false || !ok || len(urls) != 0 {
		t.Fatalf("system info no navegador: %+v", info)
	}
}
