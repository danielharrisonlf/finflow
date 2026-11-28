package cli_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"finflow/internal/cli"
	"finflow/internal/client"
	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

// isolateEnv deixa o processo de teste sem configuração herdada e sem cores ANSI.
func isolateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	for _, k := range []string{
		"FINFLOW_API_URL", "DATABASE_URL", "FINFLOW_DB_PATH",
		"FINFLOW_TOKEN", "FINFLOW_API_TOKEN", "FINFLOW_PASSWORD", "FINFLOW_AUTH_SECRET",
		"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID", "TELEGRAM_WEBHOOK_SECRET",
		"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_PROFILE", "ANTHROPIC_FEDERATION_RULE_ID", "ANTHROPIC_BASE_URL",
	} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
	// Sem o perfil do `ant auth login` desta máquina: o Assistente só vê a chave que o teste definir.
	t.Setenv("ANTHROPIC_CONFIG_DIR", t.TempDir())
}

// setupLocal aponta a CLI para um SQLite novo em arquivo temporário e devolve o caminho.
func setupLocal(t *testing.T) string {
	t.Helper()
	isolateEnv(t)
	path := filepath.Join(t.TempDir(), "finflow.db")
	t.Setenv("FINFLOW_DB_PATH", path)
	return path
}

type result struct {
	code           int
	stdout, stderr string
}

func run(t *testing.T, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	code := cli.Run(args, &out, &errOut)
	return result{code: code, stdout: out.String(), stderr: errOut.String()}
}

// mustRun executa o comando e falha o teste se o código de saída não for 0.
func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	r := run(t, args...)
	if r.code != 0 {
		t.Fatalf("finflow %s: código %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), r.code, r.stdout, r.stderr)
	}
	return r.stdout
}

// mustFail executa o comando esperando erro e que o stderr contenha wantErr.
func mustFail(t *testing.T, wantErr string, args ...string) {
	t.Helper()
	r := run(t, args...)
	if r.code == 0 {
		t.Fatalf("finflow %s: esperava falha, saiu com 0\nstdout:\n%s", strings.Join(args, " "), r.stdout)
	}
	if !strings.Contains(r.stderr, wantErr) {
		t.Fatalf("finflow %s: stderr não contém %q:\n%s", strings.Join(args, " "), wantErr, r.stderr)
	}
}

func assertContains(t *testing.T, label, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("%s: saída não contém %q:\n%s", label, w, out)
		}
	}
}

// localBackend abre o mesmo banco usado pela CLI para conferir o estado após os comandos.
func localBackend(t *testing.T, path string) client.Backend {
	t.Helper()
	database, err := db.Open(context.Background(), "", path)
	if err != nil {
		t.Fatalf("abrir banco %s: %v", path, err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return &client.LocalBackend{Svcs: service.New(store.New(database), nil)}
}

func balances(t *testing.T, b client.Backend) map[string]int64 {
	t.Helper()
	accounts, err := b.Accounts(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]int64, len(accounts))
	for _, a := range accounts {
		out[a.Name] = a.BalanceCents
	}
	return out
}

func goalSaved(t *testing.T, b client.Backend, name string) int64 {
	t.Helper()
	goals, err := b.Goals(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range goals {
		if g.Name == name {
			return g.SavedCents
		}
	}
	t.Fatalf("meta %q não encontrada", name)
	return 0
}

func TestLocalQueriesAfterSeed(t *testing.T) {
	setupLocal(t)

	assertContains(t, "seed", mustRun(t, "seed"), "Seed concluído")
	assertContains(t, "seed repetido", mustRun(t, "seed"), "nada a fazer")

	assertContains(t, "saldo", mustRun(t, "saldo"),
		"Conta", "Saldo", "Nubank", "Itaú Salário", "Carteira", "XP Investimentos", "Total:", "R$ ")
	assertContains(t, "resumo", mustRun(t, "resumo"),
		"Resumo", "Saldo geral", "Receitas", "Despesas", "Balanço", "Faturas em aberto")
	assertContains(t, "extrato", mustRun(t, "extrato"),
		"Data", "Descrição", "Conta/Cartão", "lançamentos")
	assertContains(t, "cartoes", mustRun(t, "cartoes"),
		"Nubank Ultravioleta", "Inter Gold", "Limite disp.")
	assertContains(t, "faturas", mustRun(t, "faturas", "Inter Gold"),
		"Período", "Vencimento", "Total")
	assertContains(t, "orcamentos", mustRun(t, "orcamentos"),
		"Supermercado", "R$ 900,00", "Restaurantes", "Streaming")
	assertContains(t, "metas", mustRun(t, "metas"),
		"Reserva de emergência", "R$ 30.000,00", "Eurotrip 2027", "MacBook Pro")
	assertContains(t, "status", mustRun(t, "status"),
		"FinFlow pronto (local sqlite", "Saldo consolidado", "modo aberto", "bot não configurado")
}

func TestLocalLancamentos(t *testing.T) {
	path := setupLocal(t)
	mustRun(t, "seed")
	b := localBackend(t, path)

	before := balances(t, b)
	out := mustRun(t, "despesa", "12,50", "almoço", "no", "centro", "--conta", "Carteira", "--categoria", "Alimentação")
	assertContains(t, "despesa", out, "despesa de R$ 12,50 lançada em Carteira")
	mustRun(t, "despesa", "R$ 10", "café", "--conta", "Carteira")
	out = mustRun(t, "receita", "1.234,56", "freela", "--conta", "Nubank")
	assertContains(t, "receita", out, "receita de R$ 1.234,56 lançada em Nubank")

	after := balances(t, b)
	if got := after["Carteira"] - before["Carteira"]; got != -2250 {
		t.Errorf("Carteira variou %d, want -2250", got)
	}
	if got := after["Nubank"] - before["Nubank"]; got != 123456 {
		t.Errorf("Nubank variou %d, want 123456", got)
	}

	before = after
	out = mustRun(t, "transferir", "500", "--de", "Nubank", "--para", "XP")
	assertContains(t, "transferir", out, "R$ 500,00 transferidos de Nubank para XP Investimentos")
	after = balances(t, b)
	if got := after["Nubank"] - before["Nubank"]; got != -50000 {
		t.Errorf("Nubank variou %d na transferência, want -50000", got)
	}
	if got := after["XP Investimentos"] - before["XP Investimentos"]; got != 50000 {
		t.Errorf("XP variou %d na transferência, want 50000", got)
	}

	assertContains(t, "extrato --busca", mustRun(t, "extrato", "--busca", "almoço"), "almoço no centro")
}

func TestLocalCompraParcelada(t *testing.T) {
	path := setupLocal(t)
	mustRun(t, "seed")

	out := mustRun(t, "compra", "300", "tenis", "corrida", "--cartao", "Inter", "--parcelas", "3")
	assertContains(t, "compra", out, "compra de R$ 300,00 no Inter Gold em 3x")

	search := "tenis corrida"
	txType := domain.TxCardExpense
	res, err := localBackend(t, path).Transactions(context.Background(), domain.ListTransactionsQuery{
		Search: &search, Type: &txType, PageSize: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 3 {
		t.Fatalf("esperava 3 parcelas, veio %d", len(res.Items))
	}
	var sum int64
	seen := map[int]bool{}
	for _, tx := range res.Items {
		if tx.InstallmentTotal == nil || *tx.InstallmentTotal != 3 || tx.InstallmentNumber == nil {
			t.Fatalf("parcela sem numeração 1..3: %+v", tx)
		}
		seen[*tx.InstallmentNumber] = true
		amt := tx.AmountCents
		if amt < 0 {
			amt = -amt
		}
		sum += amt
	}
	if sum != 30000 || !seen[1] || !seen[2] || !seen[3] {
		t.Fatalf("parcelas somam %d (want 30000), números %v", sum, seen)
	}
}

func TestLocalGuardarEExportar(t *testing.T) {
	path := setupLocal(t)
	mustRun(t, "seed")
	b := localBackend(t, path)

	before := goalSaved(t, b, "Reserva de emergência")
	out := mustRun(t, "guardar", "200", "--meta", "reserva")
	assertContains(t, "guardar", out, `R$ 200,00 guardados em "Reserva de emergência"`)
	if got := goalSaved(t, b, "Reserva de emergência") - before; got != 20000 {
		t.Errorf("meta variou %d, want 20000", got)
	}

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "extrato.csv")
	assertContains(t, "exportar csv", mustRun(t, "exportar", "--formato", "csv", "--saida", csvPath), "extrato exportado para", csvPath)
	csv, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(strings.TrimSpace(string(csv)), "\n"); lines < 1 {
		t.Errorf("CSV sem linhas de dados:\n%s", csv)
	}

	ofxPath := filepath.Join(dir, "extrato.ofx")
	mustRun(t, "exportar", "--formato", "ofx", "--saida", ofxPath)
	ofx, err := os.ReadFile(ofxPath)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, "ofx", string(ofx), "OFX", "STMTTRN")

	mustFail(t, "formato deve ser csv, ofx ou xlsx", "exportar", "--formato", "pdf", "--saida", filepath.Join(dir, "x.pdf"))
}

func TestLocalErros(t *testing.T) {
	path := setupLocal(t)
	mustRun(t, "seed")
	b := localBackend(t, path)
	before := balances(t, b)

	for _, bad := range []string{"abc", "0", "0,00", "12,5x", "R$", "1,2,3"} {
		mustFail(t, "valor inválido", "despesa", bad, "teste")
	}
	// Sem "--" o cobra lê "-10" como flag; a mensagem precisa continuar sendo a de valor inválido.
	mustFail(t, `valor inválido: "-10"`, "despesa", "-10", "teste")
	mustFail(t, `valor inválido: "-10"`, "despesa", "--", "-10", "teste")
	mustFail(t, "valor inválido", "transferir", "-5", "--de", "Nubank", "--para", "XP")
	mustFail(t, "valor inválido", "guardar", "zero", "--meta", "reserva")
	mustFail(t, "valor inválido", "compra", "0", "x", "--cartao", "Inter")

	mustFail(t, `conta "Inexistente" não encontrada`, "despesa", "10", "teste", "--conta", "Inexistente")
	mustFail(t, `cartão "Visa Black" não encontrado`, "compra", "10", "x", "--cartao", "Visa Black")
	mustFail(t, `meta "Casa na praia" não encontrada`, "guardar", "10", "--meta", "Casa na praia")
	mustFail(t, `categoria "Nada" não encontrada`, "despesa", "10", "x", "--categoria", "Nada")
	mustFail(t, "período inválido", "faturas", "Inter", "--pagar", "2026-11")
	mustFail(t, "✖", "resumo", "--mes", "13")
	mustFail(t, "unknown command", "comando-que-nao-existe")
	mustFail(t, "required flag", "transferir", "10")

	if after := balances(t, b); after["Nubank"] != before["Nubank"] || after["Carteira"] != before["Carteira"] {
		t.Errorf("comandos com erro alteraram saldos: antes %v, depois %v", before, after)
	}
}

func TestLocalSemContas(t *testing.T) {
	setupLocal(t)
	mustFail(t, "nenhuma conta cadastrada", "despesa", "10", "teste")
}

func TestRemoteMode(t *testing.T) {
	isolateEnv(t)
	localPath := filepath.Join(t.TempDir(), "nao-usar.db")
	t.Setenv("FINFLOW_DB_PATH", localPath)

	database := testutil.Open(t, db.DialectSQLite)
	st := store.New(database)
	if _, err := store.Seed(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	svcs := service.New(st, nil)
	srv := httptest.NewServer(httpapi.NewServer(svcs, httpapi.ServerOptions{}))
	t.Cleanup(srv.Close)
	t.Setenv("FINFLOW_API_URL", srv.URL)

	b := &client.LocalBackend{Svcs: svcs}
	before := balances(t, b)

	assertContains(t, "saldo remoto", mustRun(t, "saldo"), "Nubank", "XP Investimentos", "Total:")
	assertContains(t, "despesa remota", mustRun(t, "despesa", "45,90", "mercado", "--conta", "Nubank"),
		"despesa de R$ 45,90 lançada em Nubank")
	if got := balances(t, b)["Nubank"] - before["Nubank"]; got != -4590 {
		t.Errorf("Nubank variou %d via API, want -4590", got)
	}
	assertContains(t, "status remoto", mustRun(t, "status"), "FinFlow pronto (API: "+srv.URL)

	// --api tem precedência e erros da API chegam como mensagem legível no stderr.
	mustFail(t, "valor inválido", "--api", srv.URL, "despesa", "abc", "x")
	mustFail(t, `conta "Inexistente" não encontrada`, "despesa", "1", "x", "--conta", "Inexistente")

	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Errorf("modo remoto não deveria criar o banco local %s (stat err: %v)", localPath, err)
	}
}
