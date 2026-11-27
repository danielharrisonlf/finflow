package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"finflow/internal/client"
	"finflow/internal/config"
	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/mcp"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/webui"

	"github.com/spf13/cobra"
)

type rootOpts struct {
	apiURL string
	token  string
}

var txTypeLabel = map[domain.TransactionType]string{
	domain.TxIncome:         "receita",
	domain.TxExpense:        "despesa",
	domain.TxTransfer:       "transferência",
	domain.TxCardExpense:    "cartão",
	domain.TxInvoicePayment: "pgto fatura",
}

func Execute() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}

// Run executa a CLI com os argumentos e saídas informados e devolve o código de saída
// (0 = sucesso, 1 = erro). Toda falha é impressa em stderr com uma mensagem legível.
func Run(args []string, stdout, stderr io.Writer) int {
	root := NewRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "%s\n", cRed("✖ "+err.Error()))
		return 1
	}
	return 0
}

// negativeArgRe reconhece o erro do cobra quando um valor negativo (ex.: -10) é lido como flag.
var negativeArgRe = regexp.MustCompile(`unknown shorthand flag: '.' in (-[\d.,]+)$`)

// flagError troca o erro genérico do cobra para valores negativos pela mesma mensagem de valor inválido.
func flagError(_ *cobra.Command, err error) error {
	if m := negativeArgRe.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Errorf(`valor inválido: %q — use formatos como 45,90 ou 1.234,56`, m[1])
	}
	return err
}

func NewRootCmd() *cobra.Command {
	opts := &rootOpts{}

	root := &cobra.Command{
		Use:           "finflow",
		Short:         "FinFlow — controle financeiro pessoal (servidor, CLI e MCP em binário único)",
		Version:       "0.1.0",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetFlagErrorFunc(flagError)
	root.PersistentFlags().StringVar(&opts.apiURL, "api", "", "URL da API remota (se omitido e sem FINFLOW_API_URL, acessa o banco local direto)")
	root.PersistentFlags().StringVar(&opts.token, "token", "", "Token ou senha da API remota (padrão: FINFLOW_TOKEN ou FINFLOW_PASSWORD)")

	root.AddCommand(
		cmdServe(),
		cmdSeed(),
		cmdMCP(opts),
		cmdSaldo(opts),
		cmdResumo(opts),
		cmdExtrato(opts),
		cmdSimpleTx(opts, "despesa", domain.TxExpense, domain.CategoryExpense),
		cmdSimpleTx(opts, "receita", domain.TxIncome, domain.CategoryIncome),
		cmdTransferir(opts),
		cmdCompra(opts),
		cmdCartoes(opts),
		cmdFaturas(opts),
		cmdOrcamentos(opts),
		cmdMetas(opts),
		cmdGuardar(opts),
		cmdExportar(opts),
		cmdImportar(opts),
		cmdProjecao(opts),
		cmdFixas(opts),
		cmdInsights(opts),
		cmdStatus(opts),
	)
	return root
}

// shutdownTimeout limita o encerramento gracioso do `serve` (requisições em andamento).
const shutdownTimeout = 10 * time.Second

// logDatabase informa em stderr qual banco foi aberto (caminho absoluto no caso do SQLite).
func logDatabase(w io.Writer, cmdName string, database *db.DB) {
	fmt.Fprintf(w, "finflow %s: banco %s -> %s\n", cmdName, database.Dialect, database.Target)
}

// openBackend escolhe HTTP quando --api ou FINFLOW_API_URL está definido; senão abre o banco local direto.
func openBackend(ctx context.Context, opts *rootOpts) (backend client.Backend, modeLabel string, cleanup func(), err error) {
	return openBackendLogged(ctx, opts, nil, "")
}

// openBackendLogged é o openBackend que, em modo local e com logTo definido, informa o banco aberto.
func openBackendLogged(ctx context.Context, opts *rootOpts, logTo io.Writer, cmdName string) (backend client.Backend, modeLabel string, cleanup func(), err error) {
	remote := strings.TrimSpace(opts.apiURL)
	if remote == "" {
		remote = strings.TrimSpace(os.Getenv("FINFLOW_API_URL"))
	}
	if remote != "" {
		return client.NewHTTPClient(remote, opts.token), "API: " + remote, func() {}, nil
	}
	cfg := config.Load()
	database, err := db.Open(ctx, cfg.DatabaseURL, cfg.DBPath)
	if err != nil {
		return nil, "", nil, err
	}
	if logTo != nil {
		logDatabase(logTo, cmdName, database)
	}
	st := store.New(database)
	if err := store.Bootstrap(ctx, st); err != nil {
		_ = database.Close()
		return nil, "", nil, err
	}
	svcs := service.New(st, nil)
	label := fmt.Sprintf("local %s (%s)", database.Dialect, database.Target)
	return &client.LocalBackend{Svcs: svcs}, label, func() { _ = database.Close() }, nil
}

func money(cents int64, signed bool) string {
	text := domain.FormatBRL(cents)
	if signed && cents > 0 {
		text = "+" + text
	}
	if cents > 0 {
		return cGreen(text)
	}
	if cents < 0 {
		return cRed(text)
	}
	return cDim(text)
}

func requireCents(raw string) (int64, error) {
	cents, ok := domain.ParseBRLToCents(raw)
	if !ok {
		return 0, fmt.Errorf(`valor inválido: %q — use formatos como 45,90 ou 1.234,56`, raw)
	}
	return cents, nil
}

func resolveAccount(ctx context.Context, b client.Backend, ref, label string) (domain.Account, error) {
	accounts, err := b.Accounts(ctx, false)
	if err != nil {
		return domain.Account{}, err
	}
	if len(accounts) == 0 {
		return domain.Account{}, errors.New("nenhuma conta cadastrada — crie uma no app ou rode `finflow seed`")
	}
	if strings.TrimSpace(ref) == "" {
		return accounts[0], nil
	}
	found, ok := client.ResolveByName(accounts, ref, func(a domain.Account) string { return a.ID }, func(a domain.Account) string { return a.Name })
	if !ok {
		names := make([]string, len(accounts))
		for i, a := range accounts {
			names[i] = a.Name
		}
		return domain.Account{}, fmt.Errorf(`%s %q não encontrada (ou ambígua). Contas: %s`, label, ref, strings.Join(names, ", "))
	}
	return found, nil
}

func resolveCategory(ctx context.Context, b client.Backend, ref string, kind domain.CategoryKind) (*string, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, nil
	}
	cats, err := b.Categories(ctx, &kind)
	if err != nil {
		return nil, err
	}
	found, ok := client.ResolveByName(cats, ref, func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name })
	if !ok {
		names := make([]string, len(cats))
		for i, c := range cats {
			names[i] = c.Name
		}
		return nil, fmt.Errorf(`categoria %q não encontrada. Disponíveis: %s`, ref, strings.Join(names, ", "))
	}
	id := found.ID
	return &id, nil
}

// ───────────────────────── serve / seed / mcp ─────────────────────────

func cmdServe() *cobra.Command {
	var port int
	var host string
	var autoSeed bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Inicia a API HTTP e serve a interface web embutida",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			cfg := config.Load()
			if port > 0 {
				cfg.Port = port
			}
			if host != "" {
				cfg.Host = host
			}

			ctx := cmd.Context()
			database, err := db.Open(ctx, cfg.DatabaseURL, cfg.DBPath)
			if err != nil {
				return err
			}
			defer database.Close()
			logDatabase(cmd.ErrOrStderr(), "serve", database)

			st := store.New(database)
			if err := store.Bootstrap(ctx, st); err != nil {
				return err
			}
			if autoSeed {
				seeded, err := store.Seed(ctx, st)
				if err != nil {
					return err
				}
				if seeded {
					fmt.Fprintln(out, cGreen("✔ Seed aplicado (dados de exemplo criados)"))
				}
			}

			svcs := service.New(st, nil)
			webFS, hasWeb := webui.DistFS(cfg.WebDir)
			apiServer := httpapi.NewServer(svcs, httpapi.ServerOptions{
				WebOrigin: cfg.WebOrigin,
				WebFS:     webFS,
			})

			addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
			httpSrv := httpapi.NewHTTPServer(addr, apiServer)

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigCh)
			errCh := make(chan error, 1)
			go func() {
				errCh <- httpSrv.ListenAndServe()
			}()

			displayHost := cfg.Host
			if displayHost == "" {
				displayHost = "localhost"
			}
			fmt.Fprintf(out, "FinFlow rodando em http://%s:%d (API: /api, banco: %s -> %s)\n", displayHost, cfg.Port, database.Dialect, database.Target)
			if hasWeb {
				fmt.Fprintf(out, "Interface web disponível em http://%s:%d\n", displayHost, cfg.Port)
			} else {
				fmt.Fprintln(out, cDim("Interface web não embutida ainda (rode `pnpm build` ou use `pnpm dev:web` em :5273)"))
			}

			select {
			case sig := <-sigCh:
				fmt.Fprintf(out, "\nEncerrando FinFlow (%v)...\n", sig)
				shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
				defer cancel()
				if err := httpSrv.Shutdown(shutCtx); err != nil {
					_ = httpSrv.Close()
					return fmt.Errorf("encerramento do servidor HTTP excedeu %s: %w", shutdownTimeout, err)
				}
				return nil
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			}
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "porta HTTP (padrão: API_PORT ou 3333)")
	cmd.Flags().StringVar(&host, "host", "", "endereço de bind (padrão: vazio / todas as interfaces)")
	cmd.Flags().BoolVar(&autoSeed, "seed", false, "popula o banco com dados de exemplo se estiver vazio")
	return cmd
}

func cmdSeed() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Popula o banco com dados de exemplo (idempotente)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			cfg := config.Load()
			database, err := db.Open(cmd.Context(), cfg.DatabaseURL, cfg.DBPath)
			if err != nil {
				return err
			}
			defer database.Close()

			st := store.New(database)
			if err := store.Bootstrap(cmd.Context(), st); err != nil {
				return err
			}
			seeded, err := store.Seed(cmd.Context(), st)
			if err != nil {
				return err
			}
			if !seeded {
				fmt.Fprintln(out, "Seed já aplicado (existem contas) — nada a fazer.")
				return nil
			}
			fmt.Fprintln(out, cGreen("✔ Seed concluído: 5 contas, 2 cartões, categorias padrão com tipo de gasto, 6 recorrências, transações de 3 meses, 5 orçamentos, 3 metas (com reserva de emergência), limite mensal, regras e 2 desejos."))
			return nil
		},
	}
}

func cmdMCP(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Inicia o servidor MCP em stdio (24 tools para agentes de IA)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// stdout é exclusivo do protocolo MCP: diagnósticos vão sempre para stderr.
			b, mode, cleanup, err := openBackendLogged(cmd.Context(), opts, cmd.ErrOrStderr(), "mcp")
			if err != nil {
				return err
			}
			defer cleanup()
			return mcp.NewServer(b).Run(cmd.Context(), mode)
		},
	}
}

// ───────────────────────── consultas ─────────────────────────

func cmdSaldo(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "saldo",
		Short: "Saldo de todas as contas e total consolidado",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			accounts, err := b.Accounts(cmd.Context(), false)
			if err != nil {
				return err
			}
			rows := make([][]string, len(accounts))
			var total int64
			for i, a := range accounts {
				total += a.BalanceCents
				rows[i] = []string{a.Name, cDim(strings.ToLower(string(a.Type))), money(a.BalanceCents, false)}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{{Header: "Conta"}, {Header: "Tipo"}, {Header: "Saldo", Align: AlignRight}},
				rows,
			))
			fmt.Fprintf(out, "\n%s %s\n", cBold("Total:"), cBold(money(total, false)))
			return nil
		},
	}
}

func cmdResumo(opts *rootOpts) *cobra.Command {
	var mes, ano int
	cmd := &cobra.Command{
		Use:   "resumo",
		Short: "Resumo do mês: receitas, despesas, balanço e maiores gastos",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			var q domain.PeriodQuery
			if mes > 0 {
				q.Month = &mes
			}
			if ano > 0 {
				q.Year = &ano
			}
			summary, err := b.DashboardSummary(cmd.Context(), q)
			if err != nil {
				return err
			}
			slices, err := b.DashboardCategories(cmd.Context(), q)
			if err != nil {
				return err
			}

			fmt.Fprintln(out, cBold(fmt.Sprintf("Resumo %02d/%d", summary.Month, summary.Year)))
			fmt.Fprintf(out, "  Saldo geral        %s\n", money(summary.TotalBalanceCents, false))
			fmt.Fprintf(out, "  Receitas           %s\n", money(summary.MonthIncomeCents, true))
			fmt.Fprintf(out, "  Despesas           %s\n", money(-summary.MonthExpenseCents, false))
			fmt.Fprintf(out, "  Balanço            %s\n", money(summary.MonthNetCents, true))
			fmt.Fprintf(out, "  Faturas em aberto  %s\n", cYellow(domain.FormatBRL(summary.OpenInvoicesCents)))
			fmt.Fprintf(out, "  Pendências         %d\n", summary.PendingCount)
			if len(slices) > 0 {
				fmt.Fprintf(out, "\n%s\n", cBold("Maiores gastos"))
				limit := len(slices)
				if limit > 5 {
					limit = 5
				}
				for _, sl := range slices[:limit] {
					fmt.Fprintf(out, "  %s %s %s %s\n",
						ProgressBar(sl.Percent, 12),
						padRightRunes(sl.Name, 16),
						domain.FormatBRL(sl.TotalCents),
						cDim(fmt.Sprintf("(%v%%)", sl.Percent)),
					)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&mes, "mes", 0, "mês (1-12)")
	cmd.Flags().IntVar(&ano, "ano", 0, "ano")
	return cmd
}

func cmdExtrato(opts *rootOpts) *cobra.Command {
	var mes, ano, pagina int
	var busca, tipo, status string

	cmd := &cobra.Command{
		Use:   "extrato",
		Short: "Extrato com filtros",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			q := domain.ListTransactionsQuery{Page: pagina, PageSize: 25}
			if mes > 0 {
				q.Month = &mes
			}
			if ano > 0 {
				q.Year = &ano
			}
			if busca != "" {
				q.Search = &busca
			}
			if tipo != "" {
				t := domain.TransactionType(tipo)
				q.Type = &t
			}
			if status != "" {
				st := domain.TransactionStatus(status)
				q.Status = &st
			}

			res, err := b.Transactions(cmd.Context(), q)
			if err != nil {
				return err
			}
			if len(res.Items) == 0 {
				fmt.Fprintln(out, cDim("nenhum lançamento para esses filtros"))
				return nil
			}
			rows := make([][]string, len(res.Items))
			for i, t := range res.Items {
				desc := t.Description
				if t.Status == domain.TxPending {
					desc += cYellow(" (pendente)")
				}
				cat := txTypeLabel[t.Type]
				if t.CategoryName != nil {
					cat = *t.CategoryName
				}
				origin := ""
				if t.AccountName != nil {
					origin = *t.AccountName
				} else if t.CardName != nil {
					origin = *t.CardName
				}
				rows[i] = []string{
					cDim(t.Date),
					desc,
					cDim(cat),
					cDim(origin),
					money(t.AmountCents, true),
				}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Data"},
					{Header: "Descrição"},
					{Header: "Categoria"},
					{Header: "Conta/Cartão"},
					{Header: "Valor", Align: AlignRight},
				},
				rows,
			))
			pages := int(math.Max(1, math.Ceil(float64(res.Total)/float64(res.PageSize))))
			fmt.Fprintln(out, cDim(fmt.Sprintf("\npágina %d/%d — %d lançamentos", res.Page, pages, res.Total)))
			return nil
		},
	}
	cmd.Flags().IntVar(&mes, "mes", 0, "mês (1-12)")
	cmd.Flags().IntVar(&ano, "ano", 0, "ano")
	cmd.Flags().StringVar(&busca, "busca", "", "busca na descrição")
	cmd.Flags().StringVar(&tipo, "tipo", "", "INCOME|EXPENSE|TRANSFER|CARD_EXPENSE|INVOICE_PAYMENT")
	cmd.Flags().StringVar(&status, "status", "", "PENDING|CONFIRMED")
	cmd.Flags().IntVar(&pagina, "pagina", 1, "página")
	return cmd
}

// ───────────────────────── lançamentos ─────────────────────────

func cmdSimpleTx(opts *rootOpts, name string, txType domain.TransactionType, catKind domain.CategoryKind) *cobra.Command {
	var conta, categoria, data string
	var pendente bool

	pendingHelp := "marca como a pagar"
	if txType == domain.TxIncome {
		pendingHelp = "marca como a receber"
	}

	cmd := &cobra.Command{
		Use:   name + " <valor> <descricao...>",
		Short: fmt.Sprintf("Lança uma %s (ex.: finflow %s 45,90 almoço --categoria alimentacao)", name, name),
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cents, err := requireCents(args[0])
			if err != nil {
				return err
			}
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			acc, err := resolveAccount(cmd.Context(), b, conta, "conta")
			if err != nil {
				return err
			}
			catID, err := resolveCategory(cmd.Context(), b, categoria, catKind)
			if err != nil {
				return err
			}
			dateStr := data
			if dateStr == "" {
				dateStr = domain.TodayString()
			}
			st := domain.TxConfirmed
			if pendente {
				st = domain.TxPending
			}
			notes := "lançado via CLI"
			created, err := b.CreateTransaction(cmd.Context(), domain.CreateTransactionInput{
				Type:        txType,
				Description: strings.Join(args[1:], " "),
				AmountCents: cents,
				Date:        dateStr,
				Status:      st,
				AccountID:   acc.ID,
				CategoryID:  catID,
				Notes:       &notes,
			})
			if err != nil {
				return err
			}
			absAmt := created.AmountCents
			if absAmt < 0 {
				absAmt = -absAmt
			}
			fmt.Fprintln(out, cGreen(fmt.Sprintf("✔ %s de %s lançada em %s", name, domain.FormatBRL(absAmt), acc.Name)))
			return nil
		},
	}
	cmd.Flags().StringVar(&conta, "conta", "", "conta (padrão: primeira conta)")
	cmd.Flags().StringVar(&categoria, "categoria", "", "categoria")
	cmd.Flags().StringVar(&data, "data", "", "data YYYY-MM-DD (padrão: hoje)")
	cmd.Flags().BoolVar(&pendente, "pendente", false, pendingHelp)
	return cmd
}

func cmdTransferir(opts *rootOpts) *cobra.Command {
	var de, para, data string

	cmd := &cobra.Command{
		Use:   "transferir <valor>",
		Short: "Transfere entre contas (ex.: finflow transferir 500 --de Nubank --para XP)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cents, err := requireCents(args[0])
			if err != nil {
				return err
			}
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			from, err := resolveAccount(cmd.Context(), b, de, "conta de origem")
			if err != nil {
				return err
			}
			to, err := resolveAccount(cmd.Context(), b, para, "conta de destino")
			if err != nil {
				return err
			}
			dateStr := data
			if dateStr == "" {
				dateStr = domain.TodayString()
			}
			if _, err := b.Transfer(cmd.Context(), domain.TransferInput{
				FromAccountID: from.ID,
				ToAccountID:   to.ID,
				AmountCents:   cents,
				Date:          dateStr,
			}); err != nil {
				return err
			}
			fmt.Fprintln(out, cGreen(fmt.Sprintf("✔ %s transferidos de %s para %s", domain.FormatBRL(cents), from.Name, to.Name)))
			return nil
		},
	}
	cmd.Flags().StringVar(&de, "de", "", "conta de origem")
	cmd.Flags().StringVar(&para, "para", "", "conta de destino")
	cmd.Flags().StringVar(&data, "data", "", "data YYYY-MM-DD (padrão: hoje)")
	_ = cmd.MarkFlagRequired("de")
	_ = cmd.MarkFlagRequired("para")
	return cmd
}

func cmdCompra(opts *rootOpts) *cobra.Command {
	var cartao, categoria, data string
	var parcelas int

	cmd := &cobra.Command{
		Use:   "compra <valor> <descricao...>",
		Short: "Compra no cartão, à vista ou parcelada (--parcelas N)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			totalCents, err := requireCents(args[0])
			if err != nil {
				return err
			}
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			cards, err := b.Cards(cmd.Context(), false)
			if err != nil {
				return err
			}
			card, ok := client.ResolveByName(cards, cartao, func(c domain.Card) string { return c.ID }, func(c domain.Card) string { return c.Name })
			if !ok {
				names := make([]string, len(cards))
				for i, c := range cards {
					names[i] = c.Name
				}
				return fmt.Errorf(`cartão %q não encontrado. Cartões: %s`, cartao, strings.Join(names, ", "))
			}
			catID, err := resolveCategory(cmd.Context(), b, categoria, domain.CategoryExpense)
			if err != nil {
				return err
			}
			dateStr := data
			if dateStr == "" {
				dateStr = domain.TodayString()
			}
			notes := "lançado via CLI"
			if _, err := b.AddCardExpense(cmd.Context(), card.ID, domain.CreateCardExpenseInput{
				CardID:       card.ID,
				Description:  strings.Join(args[1:], " "),
				TotalCents:   totalCents,
				Date:         dateStr,
				CategoryID:   catID,
				Installments: parcelas,
				Notes:        &notes,
			}); err != nil {
				return err
			}
			suffix := ""
			if parcelas > 1 {
				suffix = fmt.Sprintf(" em %dx", parcelas)
			}
			fmt.Fprintln(out, cGreen(fmt.Sprintf("✔ compra de %s no %s%s", domain.FormatBRL(totalCents), card.Name, suffix)))
			return nil
		},
	}
	cmd.Flags().StringVar(&cartao, "cartao", "", "cartão")
	cmd.Flags().IntVar(&parcelas, "parcelas", 1, "número de parcelas")
	cmd.Flags().StringVar(&categoria, "categoria", "", "categoria")
	cmd.Flags().StringVar(&data, "data", "", "data da compra YYYY-MM-DD (padrão: hoje)")
	_ = cmd.MarkFlagRequired("cartao")
	return cmd
}

// ───────────────────────── cartões e faturas ─────────────────────────

func cmdCartoes(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "cartoes",
		Short: "Cartões com fatura atual e limite disponível",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			cards, err := b.Cards(cmd.Context(), false)
			if err != nil {
				return err
			}
			if len(cards) == 0 {
				fmt.Fprintln(out, cDim("nenhum cartão cadastrado"))
				return nil
			}
			rows := make([][]string, len(cards))
			for i, c := range cards {
				total, st, due := cDim("—"), cDim("—"), ""
				if c.CurrentInvoice != nil {
					total = domain.FormatBRL(c.CurrentInvoice.TotalCents)
					st = string(c.CurrentInvoice.Status)
					due = cDim(c.CurrentInvoice.DueDate)
				}
				rows[i] = []string{
					c.Name,
					cDim(string(c.Brand)),
					total,
					st,
					due,
					money(c.AvailableLimitCents, false),
				}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Cartão"},
					{Header: "Bandeira"},
					{Header: "Fatura atual", Align: AlignRight},
					{Header: "Status"},
					{Header: "Vencimento"},
					{Header: "Limite disp.", Align: AlignRight},
				},
				rows,
			))
			return nil
		},
	}
}

var payPeriodRe = regexp.MustCompile(`^(\d{1,2})/(\d{4})$`)

func cmdFaturas(opts *rootOpts) *cobra.Command {
	var pagar, conta string

	cmd := &cobra.Command{
		Use:   "faturas <cartao>",
		Short: "Faturas de um cartão; pague com --pagar MM/AAAA --conta <nome>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			cards, err := b.Cards(cmd.Context(), false)
			if err != nil {
				return err
			}
			card, ok := client.ResolveByName(cards, args[0], func(c domain.Card) string { return c.ID }, func(c domain.Card) string { return c.Name })
			if !ok {
				names := make([]string, len(cards))
				for i, c := range cards {
					names[i] = c.Name
				}
				return fmt.Errorf(`cartão %q não encontrado. Cartões: %s`, args[0], strings.Join(names, ", "))
			}
			invoices, err := b.Invoices(cmd.Context(), card.ID)
			if err != nil {
				return err
			}

			if pagar != "" {
				m := payPeriodRe.FindStringSubmatch(pagar)
				if len(m) != 3 {
					return errors.New("período inválido — use o formato MM/AAAA, ex.: --pagar 11/2026")
				}
				mm, _ := strconv.Atoi(m[1])
				yy, _ := strconv.Atoi(m[2])
				var target *domain.Invoice
				for i := range invoices {
					if invoices[i].Month == mm && invoices[i].Year == yy {
						target = &invoices[i]
						break
					}
				}
				if target == nil {
					return fmt.Errorf("fatura %s não encontrada para %s", pagar, card.Name)
				}
				acc, err := resolveAccount(cmd.Context(), b, conta, "conta de pagamento")
				if err != nil {
					return err
				}
				paid, err := b.PayInvoice(cmd.Context(), card.ID, target.ID, domain.PayInvoiceInput{AccountID: acc.ID})
				if err != nil {
					return err
				}
				fmt.Fprintln(out, cGreen(fmt.Sprintf("✔ fatura %s do %s paga (%s) via %s", pagar, card.Name, domain.FormatBRL(paid.TotalCents), acc.Name)))
				return nil
			}

			rows := make([][]string, len(invoices))
			for i, inv := range invoices {
				st := cGreen("aberta")
				if inv.Status == domain.InvoicePaid {
					st = cDim("paga")
				} else if inv.Status == domain.InvoiceClosed {
					st = cYellow("fechada")
				}
				rows[i] = []string{
					fmt.Sprintf("%02d/%d", inv.Month, inv.Year),
					st,
					cDim(inv.DueDate),
					domain.FormatBRL(inv.TotalCents),
				}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Período"},
					{Header: "Status"},
					{Header: "Vencimento"},
					{Header: "Total", Align: AlignRight},
				},
				rows,
			))
			return nil
		},
	}
	cmd.Flags().StringVar(&pagar, "pagar", "", "paga a fatura do período (MM/AAAA)")
	cmd.Flags().StringVar(&conta, "conta", "", "conta para débito do pagamento")
	return cmd
}

// ───────────────────────── orçamentos e metas ─────────────────────────

func cmdOrcamentos(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "orcamentos",
		Short: "Orçamentos do mês com progresso",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			budgets, err := b.Budgets(cmd.Context(), domain.PeriodQuery{})
			if err != nil {
				return err
			}
			if len(budgets) == 0 {
				fmt.Fprintln(out, cDim("nenhum orçamento definido"))
				return nil
			}
			for _, bg := range budgets {
				fmt.Fprintf(out, "%s %s %s / %s %s\n",
					ProgressBar(bg.Percent, 14),
					padRightRunes(bg.CategoryName, 16),
					domain.FormatBRL(bg.SpentCents),
					domain.FormatBRL(bg.AmountCents),
					cDim(fmt.Sprintf("(%v%%)", bg.Percent)),
				)
			}
			return nil
		},
	}
}

func cmdMetas(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "metas",
		Short: "Metas com progresso e aporte mensal necessário",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			goals, err := b.Goals(cmd.Context())
			if err != nil {
				return err
			}
			if len(goals) == 0 {
				fmt.Fprintln(out, cDim("nenhuma meta criada"))
				return nil
			}
			for _, g := range goals {
				monthly := ""
				if g.MonthlyNeededCents != nil {
					monthly = cDim(fmt.Sprintf(" — guarde %s/mês", domain.FormatBRL(*g.MonthlyNeededCents)))
				}
				fmt.Fprintf(out, "%s %s %s / %s %s%s\n",
					ProgressBar(g.Percent, 14),
					padRightRunes(g.Name, 24),
					domain.FormatBRL(g.SavedCents),
					domain.FormatBRL(g.TargetCents),
					cDim(fmt.Sprintf("(%v%%)", g.Percent)),
					monthly,
				)
			}
			return nil
		},
	}
}

func cmdGuardar(opts *rootOpts) *cobra.Command {
	var meta string

	cmd := &cobra.Command{
		Use:   "guardar <valor>",
		Short: "Aporta em uma meta (ex.: finflow guardar 200 --meta reserva)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cents, err := requireCents(args[0])
			if err != nil {
				return err
			}
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			goals, err := b.Goals(cmd.Context())
			if err != nil {
				return err
			}
			goal, ok := client.ResolveByName(goals, meta, func(g domain.Goal) string { return g.ID }, func(g domain.Goal) string { return g.Name })
			if !ok {
				names := make([]string, len(goals))
				for i, g := range goals {
					names[i] = g.Name
				}
				return fmt.Errorf(`meta %q não encontrada. Metas: %s`, meta, strings.Join(names, ", "))
			}
			updated, err := b.ContributeGoal(cmd.Context(), goal.ID, domain.ContributeGoalInput{AmountCents: cents})
			if err != nil {
				return err
			}
			fmt.Fprintln(out, cGreen(fmt.Sprintf(`✔ %s guardados em %q (%v%%)`, domain.FormatBRL(cents), goal.Name, updated.Percent)))
			return nil
		},
	}
	cmd.Flags().StringVar(&meta, "meta", "", "meta")
	_ = cmd.MarkFlagRequired("meta")
	return cmd
}

// ───────────────────────── exportação e status ─────────────────────────

func cmdExportar(opts *rootOpts) *cobra.Command {
	var formato, saida string
	var mes, ano int

	cmd := &cobra.Command{
		Use:   "exportar",
		Short: "Exporta o extrato em CSV, OFX ou a planilha anual em xlsx",
		Long: "Exporta o extrato em CSV ou OFX. Com --formato xlsx gera a planilha anual " +
			"(Entrada · Saída · Diário · Saldo, Economia, Lançamentos e Categorias) do --ano; " +
			"com --mes, gera só o extrato do mês em xlsx.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if formato != "csv" && formato != "ofx" && formato != "xlsx" {
				return errors.New("formato deve ser csv, ofx ou xlsx")
			}
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			if formato == "xlsx" && mes == 0 {
				return exportYearWorkbook(cmd, b, ano, saida)
			}

			var q domain.ListTransactionsQuery
			if mes > 0 {
				q.Month = &mes
			}
			if ano > 0 {
				q.Year = &ano
			}
			content, err := b.ExportTransactions(cmd.Context(), formato, q)
			if err != nil {
				return err
			}
			file := saida
			if file == "" {
				file = "finflow-extrato." + formato
			}
			absFile, err := filepath.Abs(file)
			if err != nil {
				absFile = file
			}
			if err := os.WriteFile(absFile, []byte(content), 0o644); err != nil {
				return err
			}
			fmt.Fprintln(out, cGreen("✔ extrato exportado para "+absFile))
			return nil
		},
	}
	cmd.Flags().StringVar(&formato, "formato", "csv", "csv|ofx|xlsx")
	cmd.Flags().IntVar(&mes, "mes", 0, "mês (1-12)")
	cmd.Flags().IntVar(&ano, "ano", 0, "ano")
	cmd.Flags().StringVar(&saida, "saida", "", "arquivo de saída")
	return cmd
}

func cmdStatus(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Verifica o backend/API, a autenticação e a integração com o Telegram",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, mode, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			summary, err := b.DashboardSummary(cmd.Context(), domain.PeriodQuery{})
			if err != nil {
				return err
			}
			tg, err := b.TelegramStatus(cmd.Context())
			if err != nil {
				return err
			}
			auth, err := b.AuthStatus(cmd.Context())
			if err != nil {
				return err
			}

			fmt.Fprintln(out, cGreen("✔ FinFlow pronto ("+mode+")"))
			fmt.Fprintf(out, "  Saldo consolidado: %s\n", money(summary.TotalBalanceCents, false))
			if auth.Required {
				fmt.Fprintf(out, "  Autenticação: %s\n", cGreen("protegida por senha"))
			} else {
				fmt.Fprintf(out, "  Autenticação: %s\n", cDim("modo aberto (defina FINFLOW_PASSWORD para proteger)"))
			}
			tgMsg := cDim("bot não configurado")
			if tg.Configured {
				tgMsg = cGreen("bot configurado")
			}
			if tg.ChatLinked {
				tgMsg += cGreen(" + chat vinculado")
			}
			fmt.Fprintf(out, "  Telegram: %s\n", tgMsg)
			return nil
		},
	}
}
