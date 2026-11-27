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

func TestLocalPatrimonioCommands(t *testing.T) {
	setupLocal(t)
	assertContains(t, "investimentos vazio", mustRun(t, "investimentos"), "nenhum investimento")
	assertContains(t, "dividas vazio", mustRun(t, "dividas"), "nenhum empréstimo")
	mustRun(t, "seed")

	assertContains(t, "investimentos", mustRun(t, "investimentos"),
		"Posição", "Tesouro Selic 2029", "Fundo imobiliário de logística", "Tesouro", "XP Investimentos", "Investido:", "Rendimento:", "Alocação:")
	assertContains(t, "dividas", mustRun(t, "dividas"),
		"Financiamento do carro", "veículo", "1,49%", "Saldo devedor:", "Estratégias de quitação", "Só as parcelas", "Avalanche", "Bola de neve", "--extra 500")
	assertContains(t, "dividas --extra", mustRun(t, "dividas", "--extra", "500"), "com R$ 500,00 extra por mês", "Economia")
	assertContains(t, "patrimonio", mustRun(t, "patrimonio", "--meses", "6"),
		"Mês", "Caixa", "Investimentos", "Dívidas", "Patrimônio", "Variação no período:")

	mustFail(t, "valor inválido", "dividas", "--extra", "abc")
	mustFail(t, "months", "patrimonio", "--meses", "99")
}

func TestServeLANRequiresPassword(t *testing.T) {
	setupLocal(t)
	mustFail(t, "FINFLOW_PASSWORD", "serve", "--lan")
	t.Setenv("FINFLOW_LAN", "1")
	mustFail(t, "FINFLOW_PASSWORD", "serve")
}

func TestRemotePatrimonio(t *testing.T) {
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

	assertContains(t, "investimentos remoto", mustRun(t, "investimentos"), "Tesouro Selic 2029")
	assertContains(t, "dividas remoto", mustRun(t, "dividas", "--extra", "250,50"), "Financiamento do carro", "R$ 250,50 extra")
	assertContains(t, "patrimonio remoto", mustRun(t, "patrimonio"), "Patrimônio")
}
