package cli_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"finflow/internal/db"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestLocalInsightsV3(t *testing.T) {
	setupLocal(t)
	assertContains(t, "assinaturas vazio", mustRun(t, "assinaturas"), "nenhuma assinatura")
	mustRun(t, "seed")

	assertContains(t, "alertas", mustRun(t, "alertas"))
	assertContains(t, "posso", mustRun(t, "posso", "50", "café"), "Posso comprar?", "Menor saldo depois")
	assertContains(t, "posso cartão", mustRun(t, "posso", "1.200,00", "TV", "--cartao", "nubank", "--parcelas", "3"), "Posso comprar?", "Limite do mês")
	assertContains(t, "posso enorme", mustRun(t, "posso", "99.999.999,00"), "NÃO")

	mustFail(t, "valor inválido", "posso", "abc")
	mustFail(t, "não encontrado", "posso", "10", "--cartao", "Inexistente")
	mustFail(t, "installments", "posso", "10", "--cartao", "nubank", "--parcelas", "60")
}

func TestRemoteInsightsV3(t *testing.T) {
	isolateEnv(t)
	t.Setenv("FINFLOW_DB_PATH", filepath.Join(t.TempDir(), "nao-usar.db"))
	database := testutil.Open(t, db.DialectSQLite)
	st := store.New(database)
	if _, err := store.Seed(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.NewServer(service.New(st, nil), httpapi.ServerOptions{}))
	t.Cleanup(srv.Close)
	t.Setenv("FINFLOW_API_URL", srv.URL)

	mustRun(t, "assinaturas")
	mustRun(t, "alertas")
	assertContains(t, "posso remoto", mustRun(t, "posso", "10"), "Posso comprar?")
}
