package cli_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestLocalPlanejamento(t *testing.T) {
	setupLocal(t)
	mustRun(t, "seed")

	assertContains(t, "fixas", mustRun(t, "fixas"),
		"Dia", "Descrição", "Próxima", "Aluguel", "Salário", "conta fixa", "receita", "Contas fixas/mês:")
	assertContains(t, "projecao", mustRun(t, "projecao", "--dias", "10"),
		"Projeção a partir de", "Saldo disponível", "Menor saldo em 30 dias", "Dá para guardar agora", "Saldo", "Próximos 12 meses", "Saldo final")
	assertContains(t, "insights", mustRun(t, "insights"),
		"Insights", "Ritmo", "Limite", "Performance", "Tipos de gasto", "Fixo", "Cortável", "Variável", "Patrimônio", "Reserva", "plano 50%")
	m, y := domain.CurrentMonthYear()
	prevY, prevM := domain.AddMonths(y, m, -1)
	assertContains(t, "insights mês anterior", mustRun(t, "insights", "--mes", strconv.Itoa(prevM), "--ano", strconv.Itoa(prevY)), "Insights")

	mustFail(t, "days", "projecao", "--dias", "400")
	mustFail(t, "✖", "insights", "--mes", "13")
}

func TestBootCreatesDefaultCategories(t *testing.T) {
	path := setupLocal(t)
	assertContains(t, "fixas vazio", mustRun(t, "fixas"), "nenhuma recorrência")
	expense := domain.CategoryExpense
	cats, err := localBackend(t, path).Categories(context.Background(), &expense)
	if err != nil || len(cats) < 40 {
		t.Fatalf("boot deveria criar as categorias padrão: %d %v", len(cats), err)
	}
	for _, c := range cats {
		if c.SpendType == nil {
			t.Fatalf("categoria padrão sem tipo de gasto: %+v", c)
		}
	}
}

func TestRemotePlanejamento(t *testing.T) {
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

	assertContains(t, "fixas remoto", mustRun(t, "fixas"), "Aluguel", "Internet fibra")
	assertContains(t, "projecao remota", mustRun(t, "projecao"), "Projeção a partir de")
	assertContains(t, "insights remoto", mustRun(t, "insights"), "Performance", "Reserva")
	mustFail(t, "days", "projecao", "--dias", "999")
}
