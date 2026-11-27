package cli_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

const importCSV = "Data;Descrição;Valor\n03/02/2026;Importado padaria;-12,50\n04/02/2026;Importado freela;900,00\n04/02/2026;Importado freela;900,00\nxx;Quebrado;1\n"

func countDescriptions(t *testing.T, list []domain.Transaction, prefix string) int {
	t.Helper()
	n := 0
	for _, tx := range list {
		if strings.HasPrefix(tx.Description, prefix) {
			n++
		}
	}
	return n
}

func TestLocalImportarEExportarXLSX(t *testing.T) {
	path := setupLocal(t)
	mustRun(t, "seed")
	dir := t.TempDir()
	file := filepath.Join(dir, "banco.csv")
	if err := os.WriteFile(file, []byte(importCSV), 0o644); err != nil {
		t.Fatal(err)
	}

	out := mustRun(t, "importar", file, "--conta", "Nubank", "--simular")
	assertContains(t, "simular", out, "formato: CSV", "4 linhas (2 válidas, 1 duplicadas, 1 com erro)", "Nubank", "simulação: nada foi gravado")

	b := localBackend(t, path)
	year := 2026
	list, err := b.Transactions(context.Background(), domain.ListTransactionsQuery{Year: &year, PageSize: 100, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if n := countDescriptions(t, list.Items, "Importado"); n != 0 {
		t.Fatalf("--simular gravou %d lançamentos", n)
	}

	assertContains(t, "importar", mustRun(t, "importar", file, "--conta", "Nubank"), "2 lançamentos importados (1 pulados)")
	list, err = b.Transactions(context.Background(), domain.ListTransactionsQuery{Year: &year, PageSize: 100, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if n := countDescriptions(t, list.Items, "Importado"); n != 2 {
		t.Fatalf("esperava 2 importados, veio %d", n)
	}
	// Reimportar o mesmo arquivo não duplica.
	assertContains(t, "reimportar", mustRun(t, "importar", file, "--conta", "Nubank"), "nada para importar")
	mustFail(t, "não consegui ler", "importar", filepath.Join(dir, "nao-existe.csv"))

	xlsxPath := filepath.Join(dir, "planilha.xlsx")
	assertContains(t, "exportar xlsx", mustRun(t, "exportar", "--formato", "xlsx", "--ano", "2026", "--saida", xlsxPath), "planilha exportada")
	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got := strings.Join(f.GetSheetList(), ","); got != "2026,Economia,Lançamentos,Categorias" {
		t.Fatalf("abas = %s", got)
	}

	monthPath := filepath.Join(dir, "mes.xlsx")
	mustRun(t, "exportar", "--formato", "xlsx", "--mes", "2", "--ano", "2026", "--saida", monthPath)
	mf, err := excelize.OpenFile(monthPath)
	if err != nil {
		t.Fatal(err)
	}
	defer mf.Close()
	if got := strings.Join(mf.GetSheetList(), ","); got != "Lançamentos" {
		t.Fatalf("abas do extrato mensal = %s", got)
	}
}

func TestRemoteImportarEExportarXLSX(t *testing.T) {
	isolateEnv(t)
	t.Setenv("FINFLOW_DB_PATH", filepath.Join(t.TempDir(), "nao-usar.db"))
	database := testutil.Open(t, db.DialectSQLite)
	st := store.New(database)
	if _, err := store.Seed(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	svcs := service.New(st, nil)
	srv := httptest.NewServer(httpapi.NewServer(svcs, httpapi.ServerOptions{}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	file := filepath.Join(dir, "banco.csv")
	if err := os.WriteFile(file, []byte(importCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	assertContains(t, "importar remoto", mustRun(t, "--api", srv.URL, "importar", file, "--conta", "Nubank"), "2 lançamentos importados")
	xlsxPath := filepath.Join(dir, "remoto.xlsx")
	mustRun(t, "--api", srv.URL, "exportar", "--formato", "xlsx", "--ano", "2026", "--saida", xlsxPath)
	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		t.Fatalf("planilha remota inválida: %v", err)
	}
	defer f.Close()
	if got := f.GetSheetList(); len(got) != 4 {
		t.Fatalf("abas = %v", got)
	}
}
