package cli

import (
	"fmt"
	"strings"

	"finflow/internal/domain"

	"github.com/spf13/cobra"
)

// ───────────────────────── investimentos, dívidas e patrimônio ─────────────────────────

var investmentKindLabel = map[domain.InvestmentKind]string{
	domain.InvestmentFixedIncome: "Renda fixa",
	domain.InvestmentTreasury:    "Tesouro",
	domain.InvestmentStocks:      "Ações",
	domain.InvestmentFII:         "Fundos imobiliários",
	domain.InvestmentFunds:       "Fundos",
	domain.InvestmentCrypto:      "Cripto",
	domain.InvestmentPension:     "Previdência",
	domain.InvestmentOther:       "Outros",
}

var loanKindLabel = map[domain.LoanKind]string{
	domain.LoanPersonal:  "pessoal",
	domain.LoanPayroll:   "consignado",
	domain.LoanVehicle:   "veículo",
	domain.LoanMortgage:  "imobiliário",
	domain.LoanStudent:   "estudantil",
	domain.LoanCardDebt:  "cartão",
	domain.LoanOverdraft: "cheque especial",
	domain.LoanOther:     "outro",
}

// formatPct formata percentuais com vírgula ("12,35%"); nil vira "—".
func formatPct(v *float64) string {
	if v == nil {
		return cDim("—")
	}
	text := strings.Replace(fmt.Sprintf("%.2f%%", *v), ".", ",", 1)
	switch {
	case *v > 0:
		return cGreen("+" + text)
	case *v < 0:
		return cRed(text)
	}
	return cDim(text)
}

// formatFullDate converte YYYY-MM-DD em DD/MM/AAAA.
func formatFullDate(date string) string {
	if len(date) < 10 {
		return date
	}
	return date[8:10] + "/" + date[5:7] + "/" + date[:4]
}

func formatMonthYear(month, year int) string {
	return fmt.Sprintf("%02d/%d", month, year)
}

func cmdInvestimentos(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "investimentos",
		Short: "Posições de investimento, rentabilidade e alocação",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			items, err := b.Investments(cmd.Context(), false)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Fprintln(out, cDim("nenhum investimento cadastrado"))
				return nil
			}
			summary, err := b.InvestmentSummary(cmd.Context())
			if err != nil {
				return err
			}
			rows := make([][]string, len(items))
			for i, it := range items {
				where := ""
				if it.Institution != nil {
					where = *it.Institution
				} else if it.AccountName != nil {
					where = *it.AccountName
				}
				updated := cDim("—")
				if it.ValueUpdatedAt != nil {
					updated = formatDay(*it.ValueUpdatedAt)
				}
				rows[i] = []string{it.Name, cDim(investmentKindLabel[it.Kind]), cDim(where),
					domain.FormatBRL(it.InvestedCents), domain.FormatBRL(it.CurrentCents), money(it.ReturnCents, true), formatPct(it.ReturnPct), updated}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Posição"},
					{Header: "Tipo"},
					{Header: "Instituição"},
					{Header: "Investido", Align: AlignRight},
					{Header: "Atual", Align: AlignRight},
					{Header: "Rendimento", Align: AlignRight},
					{Header: "%", Align: AlignRight},
					{Header: "Atualizado"},
				},
				rows,
			))
			fmt.Fprintf(out, "\n%s %s  %s %s  %s %s (%s)\n",
				cBold("Investido:"), domain.FormatBRL(summary.TotalInvestedCents),
				cBold("Atual:"), domain.FormatBRL(summary.TotalCurrentCents),
				cBold("Rendimento:"), money(summary.ReturnCents, true), formatPct(summary.ReturnPct))
			if len(summary.ByKind) > 0 {
				parts := make([]string, len(summary.ByKind))
				for i, k := range summary.ByKind {
					parts[i] = fmt.Sprintf("%s %s%%", investmentKindLabel[k.Kind], strings.Replace(fmt.Sprintf("%.1f", k.Percent), ".", ",", 1))
				}
				fmt.Fprintf(out, "%s %s\n", cBold("Alocação:"), strings.Join(parts, " · "))
			}
			return nil
		},
	}
}

func cmdDividas(opts *rootOpts) *cobra.Command {
	var extra string
	cmd := &cobra.Command{
		Use:   "dividas",
		Short: "Empréstimos e financiamentos com saldo devedor e estratégias de quitação",
		Long: "Lista empréstimos e financiamentos (parcelas, próximo vencimento, saldo devedor pela tabela Price) " +
			"e compara a quitação só com as parcelas, por avalanche (maior taxa primeiro) e por bola de neve " +
			"(menor saldo primeiro). Use --extra para simular um valor a mais por mês.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			var extraCents int64
			if strings.TrimSpace(extra) != "" && strings.TrimSpace(extra) != "0" {
				cents, err := requireCents(extra)
				if err != nil {
					return err
				}
				extraCents = cents
			}
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			loans, err := b.Loans(cmd.Context())
			if err != nil {
				return err
			}
			if len(loans) == 0 {
				fmt.Fprintln(out, cDim("nenhum empréstimo ou financiamento cadastrado"))
				return nil
			}
			names := map[string]string{}
			rows := make([][]string, len(loans))
			var outstanding, monthly int64
			for i, l := range loans {
				names[l.ID] = l.Name
				next := cDim("—")
				if l.NextDueDate != nil {
					next = formatDay(*l.NextDueDate)
				}
				name := l.Name
				if !l.Active {
					name += cDim(" (inativo)")
				} else if l.RemainingInstallments > 0 {
					outstanding += l.OutstandingCents
					monthly += l.InstallmentCents
				}
				rate := strings.Replace(fmt.Sprintf("%.2f%%", l.MonthlyRatePct), ".", ",", 1)
				rows[i] = []string{name, cDim(loanKindLabel[l.Kind]), rate, domain.FormatBRL(l.InstallmentCents),
					fmt.Sprintf("%d/%d", l.PaidInstallments, l.TotalInstallments), next,
					money(-l.OutstandingCents, false), domain.FormatBRL(l.TotalInterestRemainingCents)}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Dívida"},
					{Header: "Tipo"},
					{Header: "Taxa a.m.", Align: AlignRight},
					{Header: "Parcela", Align: AlignRight},
					{Header: "Pagas", Align: AlignRight},
					{Header: "Próxima"},
					{Header: "Saldo devedor", Align: AlignRight},
					{Header: "Juros restantes", Align: AlignRight},
				},
				rows,
			))
			fmt.Fprintf(out, "\n%s %s  %s %s\n", cBold("Saldo devedor:"), money(-outstanding, false), cBold("Parcelas/mês:"), domain.FormatBRL(monthly))

			strat, err := b.DebtStrategy(cmd.Context(), extraCents)
			if err != nil {
				return err
			}
			if strat.MinimumOnly.Months == 0 {
				return nil
			}
			title := "Estratégias de quitação"
			if extraCents > 0 {
				title += fmt.Sprintf(" (com %s extra por mês)", domain.FormatBRL(extraCents))
			}
			fmt.Fprintln(out, "\n"+cBold(title))
			plans := []struct {
				label string
				plan  domain.DebtPlan
			}{
				{"Só as parcelas", strat.MinimumOnly},
				{"Avalanche (maior taxa)", strat.Avalanche},
				{"Bola de neve (menor saldo)", strat.Snowball},
			}
			planRows := make([][]string, len(plans))
			for i, p := range plans {
				order := make([]string, len(p.plan.Order))
				for k, id := range p.plan.Order {
					order[k] = names[id]
				}
				saved := strat.MinimumOnly.TotalInterestCents - p.plan.TotalInterestCents
				savedText := cDim("—")
				if i > 0 {
					savedText = money(saved, saved > 0)
				}
				planRows[i] = []string{p.label, fmt.Sprintf("%d", p.plan.Months), formatFullDate(p.plan.PayoffDate),
					domain.FormatBRL(p.plan.TotalInterestCents), savedText, cDim(strings.Join(order, " → "))}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Plano"},
					{Header: "Meses", Align: AlignRight},
					{Header: "Quitação"},
					{Header: "Juros", Align: AlignRight},
					{Header: "Economia", Align: AlignRight},
					{Header: "Ordem"},
				},
				planRows,
			))
			if extraCents == 0 {
				fmt.Fprintln(out, cDim("  (use --extra 500 para simular R$ 500,00 a mais por mês)"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&extra, "extra", "", "valor extra por mês para quitar dívidas (ex.: 500 ou 250,50)")
	return cmd
}

func cmdPatrimonio(opts *rootOpts) *cobra.Command {
	var meses int
	cmd := &cobra.Command{
		Use:   "patrimonio",
		Short: "Evolução do patrimônio: caixa, investimentos, dívidas e patrimônio líquido por mês",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			points, err := b.NetWorthHistory(cmd.Context(), meses)
			if err != nil {
				return err
			}
			rows := make([][]string, len(points))
			for i, p := range points {
				change := cDim("—")
				if i > 0 {
					change = money(p.NetWorthCents-points[i-1].NetWorthCents, true)
				}
				rows[i] = []string{formatMonthYear(p.Month, p.Year), domain.FormatBRL(p.CashCents), domain.FormatBRL(p.InvestmentsCents),
					money(-p.DebtsCents, false), money(p.NetWorthCents, false), change}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Mês"},
					{Header: "Caixa", Align: AlignRight},
					{Header: "Investimentos", Align: AlignRight},
					{Header: "Dívidas", Align: AlignRight},
					{Header: "Patrimônio", Align: AlignRight},
					{Header: "Variação", Align: AlignRight},
				},
				rows,
			))
			if n := len(points); n > 1 {
				fmt.Fprintf(out, "\n%s %s em %d meses\n", cBold("Variação no período:"), money(points[n-1].NetWorthCents-points[0].NetWorthCents, true), n-1)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&meses, "meses", 12, "quantidade de meses (1 a 60)")
	return cmd
}
