package cli

import (
	"fmt"
	"strings"

	"finflow/internal/client"
	"finflow/internal/domain"

	"github.com/spf13/cobra"
)

// ───────────────────────── assinaturas, alertas e "posso comprar?" (v3) ─────────────────────────

var cadenceLabel = map[string]string{
	domain.CadenceWeekly:  "semanal",
	domain.CadenceMonthly: "mensal",
	domain.CadenceYearly:  "anual",
}

var severityLabel = map[string]func(string) string{
	domain.SeverityDanger:  cRed,
	domain.SeverityWarning: cYellow,
	domain.SeverityInfo:    cDim,
}

var verdictLabel = map[string]func(string) string{
	domain.VerdictYes:     cGreen,
	domain.VerdictCareful: cYellow,
	domain.VerdictNo:      cRed,
}

func cmdAssinaturas(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "assinaturas",
		Short: "Assinaturas detectadas no histórico (cobranças recorrentes de valor estável)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			view, err := b.Subscriptions(cmd.Context())
			if err != nil {
				return err
			}
			if len(view.Items) == 0 {
				fmt.Fprintln(out, cDim("nenhuma assinatura detectada no histórico"))
				return nil
			}
			rows := make([][]string, len(view.Items))
			for i, it := range view.Items {
				change := cDim("—")
				if it.PriceChangePct != nil {
					text := fmt.Sprintf("%+.1f%%", *it.PriceChangePct)
					change = cGreen(text)
					if *it.PriceChangePct > 0 {
						change = cRed(text)
					}
				}
				tracked := cDim("não")
				if it.RecurringID != nil {
					tracked = cGreen("sim")
				}
				rows[i] = []string{
					it.Description, cadenceLabel[it.Cadence], domain.FormatBRL(it.AverageCents),
					fmt.Sprintf("%d", it.Occurrences), formatDay(it.NextExpectedDate), change, tracked,
				}
			}
			fmt.Fprintln(out, RenderTable(
				[]Column{
					{Header: "Assinatura"},
					{Header: "Ciclo"},
					{Header: "Média", Align: AlignRight},
					{Header: "Vezes", Align: AlignRight},
					{Header: "Próxima"},
					{Header: "Preço"},
					{Header: "Acompanhada"},
				},
				rows,
			))
			fmt.Fprintf(out, "\n%s %s\n", cBold("Total por mês:"), money(-view.MonthlyTotalCents, false))
			return nil
		},
	}
}

func cmdAlertas(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "alertas",
		Short: "Alertas e dicas do mês (orçamentos, ritmo, vencimentos, assinaturas, reserva...)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			b, _, cleanup, err := openBackend(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer cleanup()

			alerts, err := b.Alerts(cmd.Context())
			if err != nil {
				return err
			}
			if len(alerts) == 0 {
				fmt.Fprintln(out, cGreen("✔ nenhum alerta no momento"))
				return nil
			}
			for _, a := range alerts {
				color := severityLabel[a.Severity]
				if color == nil {
					color = cDim
				}
				fmt.Fprintf(out, "%s %s\n", color("● "+padRightRunes(a.Severity, 7)), cBold(a.Title))
				fmt.Fprintf(out, "          %s\n", a.Message)
			}
			return nil
		},
	}
}

func cmdPosso(opts *rootOpts) *cobra.Command {
	var cartao, data string
	var parcelas int
	cmd := &cobra.Command{
		Use:   "posso <valor> [descricao...]",
		Short: "Simula uma compra sobre a projeção: cabe agora? (--parcelas N --cartao X)",
		Args:  cobra.MinimumNArgs(1),
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

			in := domain.CanAffordInput{AmountCents: cents}
			if desc := strings.TrimSpace(strings.Join(args[1:], " ")); desc != "" {
				in.Description = &desc
			}
			if data != "" {
				in.Date = &data
			}
			if parcelas > 0 {
				in.Installments = &parcelas
			}
			if strings.TrimSpace(cartao) != "" {
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
				in.CardID = &card.ID
			}
			res, err := b.CanAfford(cmd.Context(), in)
			if err != nil {
				return err
			}
			label := map[string]string{domain.VerdictYes: "SIM", domain.VerdictCareful: "COM CUIDADO", domain.VerdictNo: "NÃO"}[res.Verdict]
			color := verdictLabel[res.Verdict]
			if color == nil {
				color = cDim
			}
			fmt.Fprintf(out, "%s %s\n", cBold("Posso comprar?"), color(label))
			fmt.Fprintf(out, "  %s\n", res.Message)
			fmt.Fprintf(out, "  Menor saldo depois       %s em %s\n", money(res.LowestBalanceAfterCents, false), formatDay(res.LowestDate))
			if res.Limit != nil {
				fmt.Fprintf(out, "  Limite do mês            %s de %s (%v%%)\n", domain.FormatBRL(res.Limit.SpentAfterCents), domain.FormatBRL(res.Limit.LimitCents), res.Limit.PercentAfter)
			}
			if res.FitsOn != nil {
				fmt.Fprintf(out, "  Caberia a partir de      %s\n", domain.FormatDateBR(*res.FitsOn))
			}
			if res.ReserveMonthsAfter != nil {
				fmt.Fprintf(out, "  Reserva depois           %v meses de custo de vida\n", *res.ReserveMonthsAfter)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cartao, "cartao", "", "cartão (nome ou id); omitido = sai do saldo disponível")
	cmd.Flags().IntVar(&parcelas, "parcelas", 0, "parcelas no cartão (1-48)")
	cmd.Flags().StringVar(&data, "data", "", "data da compra YYYY-MM-DD (padrão: hoje)")
	return cmd
}
