package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"finflow/internal/ai"
	"finflow/internal/client"
	"finflow/internal/domain"

	"github.com/spf13/cobra"
)

// ───────────────────────── perguntar (Assistente) ─────────────────────────

// proposalApplier grava as propostas confirmadas (modo local pelo serviço, remoto pela API).
type proposalApplier interface {
	CreateTransaction(ctx context.Context, in domain.CreateTransactionInput) (domain.Transaction, error)
	CreateRecurring(ctx context.Context, in domain.CreateRecurringInput) (domain.Recurring, error)
	ContributeGoal(ctx context.Context, id string, in domain.ContributeGoalInput) (domain.Goal, error)
	CreateWishItem(ctx context.Context, in domain.CreateWishItemInput) (domain.WishItem, error)
}

// askPrinter imprime os eventos do chat: texto em streaming, ferramentas em cinza numa linha própria e
// propostas guardadas para depois da resposta. Seguro para uso concorrente (propostas podem chegar de
// ferramentas rodando em paralelo).
type askPrinter struct {
	mu          sync.Mutex
	out         io.Writer
	atLineStart bool
	proposals   []ai.Proposal
	errMsg      string
}

func (p *askPrinter) newline() {
	if !p.atLineStart {
		fmt.Fprintln(p.out)
		p.atLineStart = true
	}
}

func (p *askPrinter) event(name string, data json.RawMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch name {
	case "text":
		var ev struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(data, &ev) == nil && ev.Delta != "" {
			fmt.Fprint(p.out, ev.Delta)
			p.atLineStart = strings.HasSuffix(ev.Delta, "\n")
		}
	case "tool":
		var ev struct {
			Label string `json:"label"`
		}
		if json.Unmarshal(data, &ev) == nil {
			p.newline()
			fmt.Fprintln(p.out, cDim("  › "+ev.Label+"…"))
		}
	case "proposal":
		var prop ai.Proposal
		if json.Unmarshal(data, &prop) == nil {
			p.proposals = append(p.proposals, prop)
		}
	case "error":
		var ev struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(data, &ev) == nil {
			p.errMsg = ev.Message
		}
	}
}

// emit adapta o Emitter do serviço local para o mesmo formato dos eventos SSE da API.
func (p *askPrinter) emit(event string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	p.event(event, raw)
}

// isInteractive: só um terminal de verdade (ou uma entrada fornecida por quem chamou a CLI, como nos
// testes) pode responder à pergunta s/N; pipes e /dev/null nunca confirmam nada.
func isInteractive(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return in != nil
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func cmdPerguntar(opts *rootOpts) *cobra.Command {
	var confirmar bool
	cmd := &cobra.Command{
		Use:   "perguntar <pergunta...>",
		Short: "Pergunta ao Assistente (modelo local ou Claude); propostas só são gravadas com --confirmar ou s/N",
		Example: `  finflow perguntar "Posso comprar um celular de R$ 2.000 em 10x?"
  finflow perguntar "Lança 45,90 de mercado hoje no Nubank" --confirmar`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			question := strings.TrimSpace(strings.Join(args, " "))
			if question == "" {
				return errors.New("escreva a pergunta, ex.: finflow perguntar \"Onde estou gastando mais?\"")
			}
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			printer := &askPrinter{out: out, atLineStart: true}

			applier, cleanup, err := askAssistant(ctx, opts, question, printer)
			if err != nil {
				return err
			}
			defer cleanup()
			printer.newline()
			if printer.errMsg != "" {
				return errors.New(printer.errMsg)
			}
			if len(printer.proposals) == 0 {
				return nil
			}
			return handleProposals(ctx, cmd, applier, printer.proposals, confirmar)
		},
	}
	cmd.Flags().BoolVar(&confirmar, "confirmar", false, "grava as propostas do Assistente sem perguntar")
	return cmd
}

// askAssistant roda a pergunta no modo local (mesmo agente do servidor, banco local e configuração do
// assistente em settings/FINFLOW_AI_*) ou, com --api/FINFLOW_API_URL, pelo SSE da API remota.
func askAssistant(ctx context.Context, opts *rootOpts, question string, printer *askPrinter) (applier proposalApplier, cleanup func(), err error) {
	remote := strings.TrimSpace(opts.apiURL)
	if remote == "" {
		remote = strings.TrimSpace(os.Getenv("FINFLOW_API_URL"))
	}
	slug := opts.spaceSlug()
	if remote != "" {
		hc := client.NewHTTPClient(remote, opts.token).WithSpace(slug)
		err := hc.AIChat(ctx, []client.AIChatMessage{{Role: "user", Content: question}}, printer.event)
		return hc, func() {}, err
	}
	local, err := openLocal(ctx, nil, "")
	if err != nil {
		return nil, nil, err
	}
	// A configuração do assistente fica no espaço pessoal; as ferramentas e as propostas usam o espaço.
	personal, err := local.services(ctx, "")
	if err != nil {
		local.close()
		return nil, nil, err
	}
	data, err := local.services(ctx, slug)
	if err != nil {
		local.close()
		return nil, nil, err
	}
	svc := ai.NewService(personal, ai.Config{}).ForSpace(data)
	if err := svc.Chat(ctx, ai.ChatInput{Messages: []ai.ChatMessage{{Role: "user", Content: question}}}, printer.emit); err != nil {
		local.close()
		return nil, nil, err
	}
	return &client.LocalBackend{Svcs: data}, local.close, nil
}

// handleProposals lista as propostas e só grava com --confirmar ou "s" no terminal.
func handleProposals(ctx context.Context, cmd *cobra.Command, applier proposalApplier, proposals []ai.Proposal, confirmar bool) error {
	out := cmd.OutOrStdout()
	label := "proposta"
	if len(proposals) > 1 {
		label = "propostas"
	}
	fmt.Fprintf(out, "\n%s\n", cBold(fmt.Sprintf("%d %s do Assistente (nada foi gravado ainda):", len(proposals), label)))
	for i, p := range proposals {
		fmt.Fprintf(out, "  %d. %s\n", i+1, p.Summary)
	}
	if !confirmar {
		in := cmd.InOrStdin()
		if !isInteractive(in) {
			fmt.Fprintln(out, cDim("Nada foi gravado. Rode de novo com --confirmar para aplicar."))
			return nil
		}
		fmt.Fprintf(out, "Aplicar? [s/N] ")
		answer, _ := bufio.NewReader(in).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "s", "sim", "y", "yes":
		default:
			fmt.Fprintln(out, cDim("Nada foi gravado."))
			return nil
		}
	}
	failed := 0
	for _, p := range proposals {
		if err := applyProposal(ctx, applier, p); err != nil {
			failed++
			fmt.Fprintf(out, "%s %s — %v\n", cRed("✖"), p.Summary, err)
			continue
		}
		fmt.Fprintf(out, "%s %s\n", cGreen("✔ gravado:"), p.Summary)
	}
	if failed > 0 {
		return fmt.Errorf("%d de %d propostas não foram gravadas", failed, len(proposals))
	}
	return nil
}

// applyProposal chama o mesmo endpoint/serviço que a interface chama ao confirmar o cartão.
func applyProposal(ctx context.Context, b proposalApplier, p ai.Proposal) error {
	raw, err := json.Marshal(p.Payload)
	if err != nil {
		return err
	}
	switch p.Kind {
	case ai.ProposalCreateTransaction:
		var in domain.CreateTransactionInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return err
		}
		_, err = b.CreateTransaction(ctx, in)
	case ai.ProposalCreateRecurring:
		var in domain.CreateRecurringInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return err
		}
		_, err = b.CreateRecurring(ctx, in)
	case ai.ProposalContributeGoal:
		var in struct {
			GoalID string `json:"goalId"`
			domain.ContributeGoalInput
		}
		if err := json.Unmarshal(raw, &in); err != nil {
			return err
		}
		_, err = b.ContributeGoal(ctx, in.GoalID, in.ContributeGoalInput)
	case ai.ProposalAddWish:
		var in domain.CreateWishItemInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return err
		}
		_, err = b.CreateWishItem(ctx, in)
	default:
		return fmt.Errorf("tipo de proposta desconhecido: %s", p.Kind)
	}
	return err
}
