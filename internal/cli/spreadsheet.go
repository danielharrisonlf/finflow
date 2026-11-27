package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"finflow/internal/client"
	"finflow/internal/domain"
	"finflow/internal/service"
)

// ───────────────────────── importar ─────────────────────────

func cmdImportar(opts *rootOpts) *cobra.Command {
	var conta, aba string
	var simular bool

	cmd := &cobra.Command{
		Use:   "importar <arquivo>",
		Short: "Importa lançamentos de planilha (xlsx), CSV ou OFX para uma conta",
		Long: "Importa lançamentos de planilha (xlsx), CSV ou OFX. Reconhece a planilha diária " +
			"(Entrada · Saída · Diário · Saldo), tabelas com cabeçalho e extratos de banco. " +
			"Duplicados (no arquivo ou já existentes na conta) são pulados.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("não consegui ler %s: %w", args[0], err)
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
			accID := acc.ID
			preview, err := b.ImportPreview(cmd.Context(), service.ImportPreviewInput{
				Data: data, Filename: filepath.Base(args[0]), Sheet: aba, AccountID: &accID,
			})
			if err != nil {
				return err
			}

			var rows []service.ImportCommitRow
			var errorsCount, dupCount int
			table := make([][]string, 0, len(preview.Rows))
			for _, r := range preview.Rows {
				state := ""
				switch {
				case r.Error != nil:
					errorsCount++
					state = cRed("erro")
				case r.Duplicate:
					dupCount++
					state = cYellow("duplicado")
				default:
					row := service.ImportCommitRow{Date: r.Date, Description: r.Description, AmountCents: r.AmountCents, Kind: r.Kind}
					if r.SuggestedCategoryID != nil {
						row.CategoryID = domain.Optional[string]{Set: true, Valid: true, Value: *r.SuggestedCategoryID}
					}
					rows = append(rows, row)
				}
				if simular && len(table) < 30 {
					signed := r.AmountCents
					if r.Kind == domain.CategoryExpense {
						signed = -signed
					}
					desc := r.Description
					if r.Error != nil {
						desc = *r.Error
					}
					table = append(table, []string{cDim(r.Date), desc, money(signed, true), state})
				}
			}

			fmt.Fprintf(out, "%s %s — %d linhas (%d válidas, %d duplicadas, %d com erro) → conta %s\n",
				cBold("formato:"), preview.Format, len(preview.Rows), len(rows), dupCount, errorsCount, cBold(acc.Name))
			for _, w := range preview.Warnings {
				fmt.Fprintln(out, cYellow("⚠ "+w))
			}
			if simular {
				if len(table) > 0 {
					fmt.Fprintln(out, RenderTable(
						[]Column{{Header: "Data"}, {Header: "Descrição"}, {Header: "Valor", Align: AlignRight}, {Header: ""}},
						table,
					))
				}
				fmt.Fprintln(out, cDim("simulação: nada foi gravado"))
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintln(out, cDim("nada para importar"))
				return nil
			}
			res, err := b.ImportCommit(cmd.Context(), service.ImportCommitInput{AccountID: acc.ID, Rows: rows})
			if err != nil {
				return err
			}
			fmt.Fprintln(out, cGreen(fmt.Sprintf("✔ %d lançamentos importados (%d pulados)", res.Created, res.Skipped+dupCount)))
			return nil
		},
	}
	cmd.Flags().StringVar(&conta, "conta", "", "conta de destino (nome ou id; padrão: a primeira)")
	cmd.Flags().StringVar(&aba, "aba", "", "aba da planilha (padrão: detecção automática)")
	cmd.Flags().BoolVar(&simular, "simular", false, "só mostra a prévia, sem gravar")
	return cmd
}

// ───────────────────────── exportar --formato xlsx ─────────────────────────

// exportYearWorkbook grava a planilha anual (ano 0 = ano corrente).
func exportYearWorkbook(cmd *cobra.Command, b client.Backend, ano int, saida string) error {
	var year *int
	if ano > 0 {
		year = &ano
	}
	content, err := b.ExportYearWorkbook(cmd.Context(), year)
	if err != nil {
		return err
	}
	file := saida
	if file == "" {
		y := domain.Now().Year()
		if year != nil {
			y = *year
		}
		file = fmt.Sprintf("finflow-planilha-%d.xlsx", y)
	}
	absFile, err := filepath.Abs(file)
	if err != nil {
		absFile = file
	}
	if err := os.WriteFile(absFile, content, 0o644); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), cGreen("✔ planilha exportada para "+absFile))
	return nil
}
