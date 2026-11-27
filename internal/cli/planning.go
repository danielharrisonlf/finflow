package cli

import (
	"fmt"
	"strings"

	"finflow/internal/domain"

	"github.com/spf13/cobra"
)

// ───────────────────────── planejamento ─────────────────────────

var spendTypeLabel = map[domain.SpendType]string{
	domain.SpendFixed:    "Fixo",
	domain.SpendCuttable: "Cortável",
	domain.SpendVariable: "Variável",
}

var paceLabel = map[string]string{
	"BELOW": "abaixo do mês passado",
	"SAME":  "no mesmo ritmo do mês passado",
	"ABOVE": "acima do mês passado",
}

var cutLabel = map[string]string{
	"CANCEL":        "cancelar",
	"ABOVE_AVERAGE": "acima da média",
	"REDUCE":        "reduzir",
}

// bandMoney colore o saldo pela faixa da projeção.
func bandMoney(cents int64, band string) string {
	text := domain.FormatBRL(cents)
	switch band {
	case "DARK_GREEN", "LIGHT_GREEN":
		return cGreen(text)
	case "YELLOW":
		return cYellow(text)
	default:
		return cRed(text)
	}
}

func formatDay(date string) string {
	if len(date) < 10 {
		return date
	}
	return date[8:10] + "/" + date[5:7]
}

func cmdProjecao(opts *rootOpts) *cobra.Command {
	var dias int
	cmd := &cobra.Command{
		Use:   "projecao",
		Short: "Saldo projetado dia a dia (recorrências, faturas, pendências e gasto diário estimado)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			p, err := b.InsightsProjection(cmd.Context(), dias)
			if err != nil {
				return err
			}
			fmt.Fprintln(out, cBold(fmt.Sprintf("Projeção a partir de %s", formatDay(p.StartDate))))
			fmt.Fprintf(out, "  Saldo disponível         %s\n", money(p.StartBalanceCents, false))
			fmt.Fprintf(out, "  Menor saldo em 30 dias   %s em %s\n", money(p.Lowest.BalanceCents, false), formatDay(p.Lowest.Date))
			if p.NextIncomeDate != nil {
				fmt.Fprintf(out, "  Próxima entrada          %s\n", formatDay(*p.NextIncomeDate))
			} else {
				fmt.Fprintf(out, "  Próxima entrada          %s\n", cDim("nenhuma prevista"))
			}
			fmt.Fprintf(out, "  Dá para guardar agora    %s\n", money(p.CanSaveNowCents, false))
			if !p.LimitConfigured {
				fmt.Fprintln(out, cDim("  (defina um limite mensal de gastos para incluir o gasto diário estimado)"))
			}

			rows := make([][]string, 0, len(p.Days))
			for _, d := range p.Days {
				var events []string
				for _, ev := range d.Events {
					if ev.Kind == domain.EventDailySpend {
						continue
					}
					events = append(events, ev.Description)
				}
				in, outC := cDim("—"), cDim("—")
				if d.InCents > 0 {
					in = cGreen(domain.FormatBRL(d.InCents))
				}
				if d.OutCents > 0 {
					outC = cRed(domain.FormatBRL(d.OutCents))
				}
				rows = append(rows, []string{formatDay(d.Date), in, outC, bandMoney(d.BalanceCents, d.Band), cDim(strings.Join(events, ", "))})
			}
			fmt.Fprintln(out)
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Dia"},
					{Header: "Entradas", Align: AlignRight},
					{Header: "Saídas", Align: AlignRight},
					{Header: "Saldo", Align: AlignRight},
					{Header: "Eventos"},
				},
				rows,
			))

			monthRows := make([][]string, 0, len(p.Months))
			for _, m := range p.Months {
				monthRows = append(monthRows, []string{
					fmt.Sprintf("%02d/%d", m.Month, m.Year),
					domain.FormatBRL(m.IncomeCents),
					domain.FormatBRL(m.RecurringOutCents),
					domain.FormatBRL(m.InvoicesCents),
					domain.FormatBRL(m.DailySpendCents),
					money(m.EndBalanceCents, false),
				})
			}
			fmt.Fprintf(out, "\n%s\n", cBold("Próximos 12 meses"))
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Mês"},
					{Header: "Receitas", Align: AlignRight},
					{Header: "Contas fixas", Align: AlignRight},
					{Header: "Faturas", Align: AlignRight},
					{Header: "Dia a dia", Align: AlignRight},
					{Header: "Saldo final", Align: AlignRight},
				},
				monthRows,
			))
			return nil
		},
	}
	cmd.Flags().IntVar(&dias, "dias", 30, "dias exibidos (1-365)")
	return cmd
}

func cmdFixas(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "fixas",
		Short: "Contas fixas e receitas recorrentes com a próxima ocorrência",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			recs, err := b.Recurring(cmd.Context())
			if err != nil {
				return err
			}
			if len(recs) == 0 {
				fmt.Fprintln(out, cDim("nenhuma recorrência cadastrada"))
				return nil
			}
			rows := make([][]string, len(recs))
			var expense, income int64
			for i, r := range recs {
				kind, amount := "conta fixa", money(-r.AmountCents, false)
				if r.Kind == domain.CategoryIncome {
					kind, amount = "receita", money(r.AmountCents, true)
				}
				next := cDim("—")
				if r.NextDate != nil {
					next = formatDay(*r.NextDate)
				}
				desc := r.Description
				if !r.Active {
					desc += cDim(" (inativa)")
				} else if r.Kind == domain.CategoryIncome {
					income += r.AmountCents
				} else {
					expense += r.AmountCents
				}
				account := ""
				if r.AccountName != nil {
					account = *r.AccountName
				}
				rows[i] = []string{fmt.Sprintf("%02d", r.DayOfMonth), desc, cDim(kind), cDim(account), amount, next}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Dia"},
					{Header: "Descrição"},
					{Header: "Tipo"},
					{Header: "Conta"},
					{Header: "Valor", Align: AlignRight},
					{Header: "Próxima"},
				},
				rows,
			))
			fmt.Fprintf(out, "\n%s %s  %s %s\n", cBold("Contas fixas/mês:"), money(-expense, false), cBold("Receitas/mês:"), money(income, true))
			return nil
		},
	}
}

func cmdInsights(opts *rootOpts) *cobra.Command {
	var mes, ano int
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Visão geral do mês: ritmo, limite, performance, tipos de gasto, reserva e onde cortar",
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
			ov, err := b.InsightsOverview(cmd.Context(), q)
			if err != nil {
				return err
			}

			fmt.Fprintln(out, cBold(fmt.Sprintf("Insights %02d/%d", ov.Month, ov.Year)))
			diff := cGreen(domain.FormatBRL(ov.Pace.DiffCents))
			if ov.Pace.DiffCents > 0 {
				diff = cRed("+" + domain.FormatBRL(ov.Pace.DiffCents))
			}
			fmt.Fprintf(out, "  Ritmo        %s até o dia %d — %s (%s)\n",
				domain.FormatBRL(ov.Pace.CurrentCents), ov.Pace.DayOfMonth, paceLabel[ov.Pace.Status], diff)
			if ml := ov.MonthlyLimit; ml.LimitCents != nil {
				line := fmt.Sprintf("  Limite       %s de %s (%v%%)", domain.FormatBRL(ml.SpentCents), domain.FormatBRL(*ml.LimitCents), *ml.Percent)
				if ml.DailyAllowanceCents != nil {
					line += cDim(fmt.Sprintf(" — pode gastar %s/dia", domain.FormatBRL(*ml.DailyAllowanceCents)))
				}
				fmt.Fprintln(out, line)
			} else {
				fmt.Fprintf(out, "  Limite       %s\n", cDim("não definido"))
			}

			pf := ov.Performance
			fmt.Fprintf(out, "\n%s\n", cBold("Performance"))
			fmt.Fprintf(out, "  Entradas     %s\n", money(pf.IncomeCents, true))
			fmt.Fprintf(out, "  Saídas       %s\n", money(-pf.FixedCents, false))
			fmt.Fprintf(out, "  Diário       %s\n", money(-pf.VariableCents, false))
			fmt.Fprintf(out, "  Performance  %s\n", money(pf.PerformanceCents, true))
			rate := cDim("—")
			if pf.SavingsRatePct != nil {
				rate = fmt.Sprintf("%v%%", *pf.SavingsRatePct)
			}
			fmt.Fprintf(out, "  Economia     %s (%s das entradas)\n", domain.FormatBRL(pf.SavedCents), rate)
			fmt.Fprintf(out, "  %s\n", pf.Message)

			fmt.Fprintf(out, "\n%s\n", cBold("Tipos de gasto"))
			for _, g := range ov.SpendTypes {
				plan := ""
				if ov.Plan != nil {
					target := map[domain.SpendType]float64{
						domain.SpendFixed: ov.Plan.FixedPct, domain.SpendCuttable: ov.Plan.CuttablePct, domain.SpendVariable: ov.Plan.VariablePct,
					}[g.SpendType]
					plan = cDim(fmt.Sprintf(" plano %v%%", target))
				}
				fmt.Fprintf(out, "  %s %s %s %s%s\n", ProgressBar(g.Percent, 12), padRightRunes(spendTypeLabel[g.SpendType], 10),
					domain.FormatBRL(g.TotalCents), cDim(fmt.Sprintf("(%v%%)", g.Percent)), plan)
			}

			fmt.Fprintf(out, "\n  Patrimônio   %s\n", money(ov.NetWorthCents, false))
			rs := ov.Reserve
			switch rs.Status {
			case "NONE":
				fmt.Fprintf(out, "  Reserva      %s — sugerido %s (%s/mês de custo médio)\n", cDim("sem meta de reserva"), domain.FormatBRL(rs.SuggestedTargetCents), domain.FormatBRL(rs.AvgMonthlyCostCents))
			default:
				status := cYellow("em construção")
				if rs.Status == "PROTECTED" {
					status = cGreen("protegida")
				}
				fmt.Fprintf(out, "  Reserva      %s de %s — cobre %v meses (%s)\n", domain.FormatBRL(rs.SavedCents), domain.FormatBRL(rs.TargetCents), rs.MonthsCovered, status)
			}

			if len(ov.CutSuggestions) > 0 {
				fmt.Fprintf(out, "\n%s\n", cBold("Onde dá para cortar"))
				for _, c := range ov.CutSuggestions {
					fmt.Fprintf(out, "  %s %s %s %s\n", padRightRunes(cutLabel[c.Kind], 15), padRightRunes(c.Name, 24),
						domain.FormatBRL(c.CurrentCents), cDim("(média "+domain.FormatBRL(c.AverageCents)+")"))
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&mes, "mes", 0, "mês (1-12)")
	cmd.Flags().IntVar(&ano, "ano", 0, "ano")
	return cmd
}
