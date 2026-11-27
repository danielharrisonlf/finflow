package cli_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"finflow/internal/db"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/spaces"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestLocalSpacesCommands(t *testing.T) {
	path := setupLocal(t)
	t.Setenv("FINFLOW_SPACE", "")
	dir := filepath.Dir(path)

	assertContains(t, "espacos", mustRun(t, "espacos"), "pessoal", "Pessoal")
	assertContains(t, "criar", mustRun(t, "espacos", "criar", "Casa", "da", "Praia", "--tipo", "casa"), `espaço "Casa da Praia" criado (casa)`, "--espaco casa-da-praia")
	mustFail(t, "tipo de espaço inválido", "espacos", "criar", "X", "--tipo", "fazenda")
	mustFail(t, "slug", "espacos", "criar", "X", "--slug", "../fora")
	if _, err := os.Stat(filepath.Join(dir, "spaces", "casa-da-praia.db")); err != nil {
		t.Fatalf("banco do espaço: %v", err)
	}
	mustFail(t, `espaço "nao-existe" não encontrado`, "--espaco", "nao-existe", "saldo")

	// Demo da casa: cria o espaço "casa" sozinho e popula com membros e despesas divididas.
	assertContains(t, "seed casa", mustRun(t, "seed", "--espaco", "casa"), `espaço "Casa" criado`, `Seed concluído no espaço "casa"`)
	assertContains(t, "seed casa repetido", mustRun(t, "seed", "--espaco", "casa"), "nada a fazer")
	assertContains(t, "saldo casa", mustRun(t, "--espaco", "casa", "saldo"), "Conta da casa")
	assertContains(t, "divisao", mustRun(t, "--espaco", "casa", "divisao"), "Ana", "Bruno", "Pagou", "Parte", "Saldo")
	assertContains(t, "membro", mustRun(t, "--espaco", "casa", "divisao", "membro", "Caio"), `membro "Caio" cadastrado`)
	assertContains(t, "acertar", mustRun(t, "--espaco", "casa", "divisao", "acertar", "Caio", "Ana", "10,00"), "acerto registrado: Caio → Ana R$ 10,00")
	mustFail(t, "membro", "--espaco", "casa", "divisao", "acertar", "Zé", "Ana", "10")

	// FINFLOW_SPACE escolhe o espaço sem a flag.
	t.Setenv("FINFLOW_SPACE", "casa")
	assertContains(t, "FINFLOW_SPACE", mustRun(t, "saldo"), "Conta da casa")
	assertContains(t, "espacos com ativo", mustRun(t, "espacos"), "casa", "Casa da Praia")
	t.Setenv("FINFLOW_SPACE", "")

	// Demo da empresa: métricas e DRE.
	assertContains(t, "seed empresa", mustRun(t, "seed", "--espaco", "empresa"), `Seed concluído no espaço "empresa"`)
	assertContains(t, "empresa", mustRun(t, "--espaco", "empresa", "empresa"), "MRR", "ARR", "Churn", "Runway", "Caixa")
	assertContains(t, "dre", mustRun(t, "--espaco", "empresa", "empresa", "dre"), "Receita", "Deduções", "Resultado", "Total")
	assertContains(t, "visao", mustRun(t, "espacos", "visao"), "Pessoal", "Casa", "Empresa", "Total")

	// O seed do pessoal continua igual e no banco atual.
	assertContains(t, "seed pessoal", mustRun(t, "seed"), "Seed concluído: 5 contas")
	assertContains(t, "saldo pessoal", mustRun(t, "saldo"), "Nubank")
}

func TestRemoteSpacesCommands(t *testing.T) {
	isolateEnv(t)
	t.Setenv("FINFLOW_SPACE", "")
	base := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	reg := spaces.New(base, spaces.Options{DataDir: t.TempDir()})
	t.Cleanup(func() { _ = reg.Close() })
	srv := httptest.NewServer(httpapi.NewServer(base, httpapi.ServerOptions{Spaces: reg}))
	t.Cleanup(srv.Close)
	t.Setenv("FINFLOW_API_URL", srv.URL)

	assertContains(t, "criar remoto", mustRun(t, "espacos", "criar", "Empresa", "--tipo", "empresa"), `espaço "Empresa" criado (empresa)`)
	assertContains(t, "espacos remoto", mustRun(t, "espacos"), "pessoal", "empresa")
	mustRun(t, "--espaco", "empresa", "divisao", "membro", "Sócia")
	casaSvcs, err := reg.Services(context.Background(), "empresa")
	if err != nil {
		t.Fatal(err)
	}
	if members, _ := casaSvcs.ListMembers(context.Background()); len(members) != 1 || members[0].Name != "Sócia" {
		t.Fatalf("membro remoto no espaço: %+v", members)
	}
	if members, _ := base.ListMembers(context.Background()); len(members) != 0 {
		t.Fatalf("o pessoal não deveria ter membros: %+v", members)
	}
	assertContains(t, "empresa remota", mustRun(t, "--espaco", "empresa", "empresa"), "MRR")
	mustFail(t, "espaço não encontrado", "--espaco", "nao-existe", "saldo")
}
