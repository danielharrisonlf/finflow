package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"finflow/internal/client"
	"finflow/internal/domain"
	"finflow/internal/store"

	"github.com/spf13/cobra"
)

// ───────────────────────── espaços, divisão (Casa) e empresa ─────────────────────────

var spaceKindLabel = map[domain.SpaceKind]string{
	domain.SpacePersonal: "pessoal",
	domain.SpaceHome:     "casa",
	domain.SpaceBusiness: "empresa",
}

// parseSpaceKind aceita casa/empresa/pessoal ou HOME/BUSINESS/PERSONAL.
func parseSpaceKind(raw string) (domain.SpaceKind, error) {
	norm := domain.NormalizeText(raw)
	for kind, label := range spaceKindLabel {
		if norm == label || strings.EqualFold(raw, string(kind)) {
			return kind, nil
		}
	}
	return "", fmt.Errorf("tipo de espaço inválido: %q — use casa, empresa ou pessoal", raw)
}

func periodQuery(mes, ano int) domain.PeriodQuery {
	var q domain.PeriodQuery
	if mes > 0 {
		q.Month = &mes
	}
	if ano > 0 {
		q.Year = &ano
	}
	return q
}

// openSpacesAdmin administra os espaços pela API (--api/FINFLOW_API_URL) ou pelo registro local.
func openSpacesAdmin(ctx context.Context, opts *rootOpts) (client.SpacesAdmin, func(), error) {
	remote := strings.TrimSpace(opts.apiURL)
	if remote == "" {
		remote = strings.TrimSpace(os.Getenv("FINFLOW_API_URL"))
	}
	if remote != "" {
		return client.NewHTTPClient(remote, opts.token), func() {}, nil
	}
	local, err := openLocal(ctx, nil, "")
	if err != nil {
		return nil, nil, err
	}
	return &client.LocalSpaces{Registry: local.reg}, local.close, nil
}

func cmdEspacos(opts *rootOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "espacos",
		Short: "Lista os espaços (pessoal, casa, empresa); use --espaco <slug> nos outros comandos",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			admin, cleanup, err := openSpacesAdmin(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()
			list, err := admin.Spaces(cmd.Context())
			if err != nil {
				return err
			}
			active := opts.spaceSlug()
			if active == "" {
				active = domain.DefaultSpaceSlug
			}
			rows := make([][]string, len(list))
			for i, sp := range list {
				mark := " "
				if sp.Slug == active {
					mark = cGreen("●")
				}
				rows[i] = []string{mark, sp.Slug, sp.Name, cDim(spaceKindLabel[sp.Kind])}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{{Header: ""}, {Header: "Slug"}, {Header: "Nome"}, {Header: "Tipo"}},
				rows,
			))
			return nil
		},
	}
	cmd.AddCommand(cmdEspacosCriar(opts), cmdEspacosVisao(opts))
	return cmd
}

func cmdEspacosCriar(opts *rootOpts) *cobra.Command {
	var tipo, cor, slug string
	cmd := &cobra.Command{
		Use:   "criar <nome>",
		Short: "Cria um espaço (banco separado com as categorias padrão do tipo)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := parseSpaceKind(tipo)
			if err != nil {
				return err
			}
			in := domain.CreateSpaceInput{Name: strings.Join(args, " "), Kind: kind}
			if cor != "" {
				in.Color = &cor
			}
			if slug != "" {
				in.Slug = &slug
			}
			admin, cleanup, err := openSpacesAdmin(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()
			sp, err := admin.CreateSpace(cmd.Context(), in)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s espaço %q criado (%s) — use --espaco %s\n", cGreen("✔"), sp.Name, spaceKindLabel[sp.Kind], sp.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&tipo, "tipo", "casa", "tipo do espaço: casa, empresa ou pessoal")
	cmd.Flags().StringVar(&cor, "cor", "", "cor #RRGGBB (padrão do tipo)")
	cmd.Flags().StringVar(&slug, "slug", "", "slug (padrão: derivado do nome)")
	return cmd
}

func cmdEspacosVisao(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "visao",
		Short: "Visão unificada: saldo, patrimônio e resultado do mês de cada espaço e o total",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			admin, cleanup, err := openSpacesAdmin(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()
			ov, err := admin.SpacesOverview(cmd.Context())
			if err != nil {
				return err
			}
			row := func(name string, t domain.SpaceTotals) []string {
				return []string{name, money(t.BalanceCents, false), money(t.NetWorthCents, false), money(t.MonthIncomeCents, false), money(-t.MonthOutCents, false), money(t.MonthResultCents, true)}
			}
			rows := make([][]string, 0, len(ov.Spaces)+1)
			for _, sp := range ov.Spaces {
				rows = append(rows, row(sp.Name, sp.SpaceTotals))
			}
			rows = append(rows, row(cBold("Total"), ov.Totals))
			fmt.Fprintln(out, RenderTable(
				[]Column{{Header: "Espaço"}, {Header: "Saldo", Align: AlignRight}, {Header: "Patrimônio", Align: AlignRight}, {Header: "Entradas", Align: AlignRight}, {Header: "Saídas", Align: AlignRight}, {Header: "Resultado", Align: AlignRight}},
				rows,
			))
			return nil
		},
	}
}

// demoSpaces são os espaços que `finflow seed --espaco <slug>` cria sozinho quando faltam.
var demoSpaces = map[string]struct {
	name string
	kind domain.SpaceKind
}{
	"casa":    {"Casa", domain.SpaceHome},
	"empresa": {"Empresa", domain.SpaceBusiness},
}

// seedSpace popula o espaço escolhido com a demo do tipo dele (Casa, Empresa ou a pessoal).
func seedSpace(cmd *cobra.Command, slug string) error {
	ctx := cmd.Context()
	local, err := openLocal(ctx, nil, "")
	if err != nil {
		return err
	}
	defer local.close()
	// Atalho da demo: --espaco casa / --espaco empresa cria o espaço do tipo certo se ainda não existir.
	if demo, ok := demoSpaces[slug]; ok && local.reg.Supported() {
		if _, err := local.reg.Get(slug); err != nil {
			s := slug
			if _, err := local.reg.Create(ctx, domain.CreateSpaceInput{Name: demo.name, Kind: demo.kind, Slug: &s}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s espaço %q criado (%s)\n", cGreen("✔"), demo.name, spaceKindLabel[demo.kind])
		}
	}
	svcs, err := local.services(ctx, slug)
	if err != nil {
		return err
	}
	sp, err := local.reg.Get(slug)
	if err != nil {
		return err
	}
	var seeded bool
	var summary string
	switch sp.Kind {
	case domain.SpaceHome:
		seeded, err = store.SeedHome(ctx, svcs.Store)
		summary = "2 membros, conta e cartão da casa, contas fixas divididas dos últimos 3 meses, sofá parcelado, 1 acerto e 1 meta"
	case domain.SpaceBusiness:
		seeded, err = store.SeedBusiness(ctx, svcs.Store)
		summary = "conta PJ, 4 clientes recorrentes (1 desativado no mês), despesas fixas e variáveis dos últimos 3 meses"
	default:
		seeded, err = store.Seed(ctx, svcs.Store)
		summary = "dados de exemplo pessoais"
	}
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if !seeded {
		fmt.Fprintf(out, "Seed já aplicado no espaço %q (existem contas) — nada a fazer.\n", slug)
		return nil
	}
	fmt.Fprintf(out, "%s Seed concluído no espaço %q: %s.\n", cGreen("✔"), slug, summary)
	return nil
}

// ───────────────────────── divisão (Casa) ─────────────────────────

func memberByName(members []domain.Member, ref string) (domain.Member, error) {
	found, ok := client.ResolveByName(members, ref, func(m domain.Member) string { return m.ID }, func(m domain.Member) string { return m.Name })
	if !ok {
		names := make([]string, len(members))
		for i, m := range members {
			names[i] = m.Name
		}
		return domain.Member{}, fmt.Errorf("membro %q não encontrado (ou ambíguo). Membros: %s", ref, strings.Join(names, ", "))
	}
	return found, nil
}

func cmdDivisao(opts *rootOpts) *cobra.Command {
	var mes, ano int
	cmd := &cobra.Command{
		Use:   "divisao",
		Short: "Divisão de despesas do espaço Casa: quanto cada membro pagou, a parte dele e os acertos sugeridos",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()
			bal, err := b.MemberBalances(cmd.Context(), periodQuery(mes, ano))
			if err != nil {
				return err
			}
			if len(bal.Members) == 0 {
				fmt.Fprintln(out, cDim("nenhum membro cadastrado — use `finflow divisao membro <nome>` (com --espaco <casa>)"))
				return nil
			}
			names := map[string]string{}
			rows := make([][]string, len(bal.Members))
			for i, m := range bal.Members {
				names[m.MemberID] = m.Name
				rows[i] = []string{m.Name, domain.FormatBRL(m.PaidCents), domain.FormatBRL(m.OwedCents), money(m.BalanceCents, true)}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{{Header: "Membro"}, {Header: "Pagou", Align: AlignRight}, {Header: "Parte", Align: AlignRight}, {Header: "Saldo", Align: AlignRight}},
				rows,
			))
			if len(bal.Settlements) == 0 {
				fmt.Fprintf(out, "\n%s\n", cGreen("Tudo acertado."))
				return nil
			}
			fmt.Fprintf(out, "\n%s\n", cBold("Acertos sugeridos"))
			for _, st := range bal.Settlements {
				fmt.Fprintf(out, "  %s → %s  %s\n", names[st.FromMemberID], names[st.ToMemberID], cYellow(domain.FormatBRL(st.AmountCents)))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&mes, "mes", 0, "mês (1-12)")
	cmd.Flags().IntVar(&ano, "ano", 0, "ano")
	cmd.AddCommand(cmdDivisaoMembro(opts), cmdDivisaoAcertar(opts))
	return cmd
}

func cmdDivisaoMembro(opts *rootOpts) *cobra.Command {
	var cor string
	cmd := &cobra.Command{
		Use:   "membro <nome>",
		Short: "Cadastra um membro para dividir despesas",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()
			in := domain.CreateMemberInput{Name: strings.Join(args, " ")}
			if cor != "" {
				in.Color = &cor
			}
			m, err := b.CreateMember(cmd.Context(), in)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s membro %q cadastrado\n", cGreen("✔"), m.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&cor, "cor", "", "cor #RRGGBB")
	return cmd
}

func cmdDivisaoAcertar(opts *rootOpts) *cobra.Command {
	var data, deConta, paraConta string
	cmd := &cobra.Command{
		Use:   "acertar <quem paga> <quem recebe> <valor>",
		Short: "Registra um acerto entre membros (com --de-conta e --para-conta também lança a transferência)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cents, err := requireCents(args[2])
			if err != nil {
				return err
			}
			if (deConta == "") != (paraConta == "") {
				return errors.New("informe as duas contas (--de-conta e --para-conta) ou nenhuma")
			}
			ctx := cmd.Context()
			b, _, cleanup, err := openBackend(ctx, opts)
			if err != nil {
				return err
			}
			defer cleanup()
			members, err := b.Members(ctx)
			if err != nil {
				return err
			}
			from, err := memberByName(members, args[0])
			if err != nil {
				return err
			}
			to, err := memberByName(members, args[1])
			if err != nil {
				return err
			}
			in := domain.SettleInput{FromMemberID: from.ID, ToMemberID: to.ID, AmountCents: cents}
			if data != "" {
				in.Date = &data
			}
			if deConta != "" {
				fromAcc, err := resolveAccount(ctx, b, deConta, "conta de origem")
				if err != nil {
					return err
				}
				toAcc, err := resolveAccount(ctx, b, paraConta, "conta de destino")
				if err != nil {
					return err
				}
				in.FromAccountID, in.ToAccountID = &fromAcc.ID, &toAcc.ID
			}
			st, err := b.Settle(ctx, in)
			if err != nil {
				return err
			}
			extra := ""
			if st.TransferGroupID != nil {
				extra = " (com transferência entre as contas)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s acerto registrado: %s → %s %s%s\n", cGreen("✔"), from.Name, to.Name, domain.FormatBRL(st.AmountCents), extra)
			return nil
		},
	}
	cmd.Flags().StringVar(&data, "data", "", "data YYYY-MM-DD (padrão: hoje)")
	cmd.Flags().StringVar(&deConta, "de-conta", "", "conta de quem paga (com --para-conta, lança a transferência)")
	cmd.Flags().StringVar(&paraConta, "para-conta", "", "conta de quem recebe")
	return cmd
}

// ───────────────────────── empresa ─────────────────────────

func cmdEmpresa(opts *rootOpts) *cobra.Command {
	var mes, ano int
	cmd := &cobra.Command{
		Use:   "empresa",
		Short: "Métricas do espaço Empresa: MRR, ARR, churn, queima, runway e caixa",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()
			m, err := b.BusinessMetrics(cmd.Context(), periodQuery(mes, ano))
			if err != nil {
				return err
			}
			runway := cDim("—")
			if m.RunwayMonths != nil {
				runway = strings.Replace(fmt.Sprintf("%.1f meses", *m.RunwayMonths), ".", ",", 1)
			}
			churn := cDim("—")
			if m.ChurnPct != nil {
				churn = strings.Replace(fmt.Sprintf("%.1f%%", *m.ChurnPct), ".", ",", 1)
			}
			fmt.Fprintln(out, cBold("Empresa "+formatMonthYear(m.Month, m.Year)))
			fmt.Fprintf(out, "  MRR           %s\n", money(m.MRRCents, false))
			fmt.Fprintf(out, "  ARR           %s\n", money(m.ARRCents, false))
			fmt.Fprintf(out, "  Novo MRR      %s\n", money(m.NewMRRCents, true))
			fmt.Fprintf(out, "  MRR perdido   %s\n", money(-m.ChurnedMRRCents, false))
			fmt.Fprintf(out, "  Churn         %s\n", churn)
			fmt.Fprintf(out, "  Queima/mês    %s\n", money(-m.BurnCents, false))
			fmt.Fprintf(out, "  Caixa         %s\n", money(m.CashCents, false))
			fmt.Fprintf(out, "  Runway        %s\n", runway)
			return nil
		},
	}
	cmd.Flags().IntVar(&mes, "mes", 0, "mês (1-12)")
	cmd.Flags().IntVar(&ano, "ano", 0, "ano")
	cmd.AddCommand(cmdEmpresaDRE(opts))
	return cmd
}

func cmdEmpresaDRE(opts *rootOpts) *cobra.Command {
	var ano int
	cmd := &cobra.Command{
		Use:   "dre",
		Short: "DRE anual por mês (receita, deduções, custos, despesas operacionais, financeiro e resultado)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()
			var year *int
			if ano > 0 {
				year = &ano
			}
			dre, err := b.BusinessDRE(cmd.Context(), year)
			if err != nil {
				return err
			}
			line := func(label string, l domain.DRELine) []string {
				margin := cDim("—")
				if l.MarginPct != nil {
					margin = strings.Replace(fmt.Sprintf("%.1f%%", *l.MarginPct), ".", ",", 1)
				}
				return []string{label, domain.FormatBRL(l.RevenueCents), domain.FormatBRL(l.DeductionsCents), domain.FormatBRL(l.CostsCents), domain.FormatBRL(l.OpexCents), money(l.FinancialCents+l.OtherCents, true), money(l.NetResultCents, true), margin}
			}
			rows := make([][]string, 0, 13)
			for _, m := range dre.Months {
				rows = append(rows, line(formatMonthYear(m.Month, dre.Year), m.DRELine))
			}
			rows = append(rows, line(cBold("Total"), dre.Totals))
			fmt.Fprintln(out, RenderTable(
				[]Column{{Header: "Mês"}, {Header: "Receita", Align: AlignRight}, {Header: "Deduções", Align: AlignRight}, {Header: "Custos", Align: AlignRight}, {Header: "Desp. oper.", Align: AlignRight}, {Header: "Financeiro", Align: AlignRight}, {Header: "Resultado", Align: AlignRight}, {Header: "Margem", Align: AlignRight}},
				rows,
			))
			return nil
		},
	}
	cmd.Flags().IntVar(&ano, "ano", 0, "ano")
	return cmd
}
