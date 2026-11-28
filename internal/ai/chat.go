package ai

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"finflow/internal/domain"
	"finflow/internal/service"
)

const (
	// chatMaxTokens: streaming, então não há risco de timeout HTTP; respostas são curtas pelo prompt.
	chatMaxTokens = 32000
	// maxToolRounds limita as idas e voltas de ferramentas numa mesma pergunta.
	maxToolRounds = 10
	// Limites da conversa enviada pelo cliente (histórico vive no localStorage).
	maxChatMessages     = 60
	maxChatMessageRunes = 20000
)

// Mensagens de parada exibidas ao usuário.
const (
	refusalMessage   = "Não posso ajudar com esse pedido. Posso responder perguntas sobre seus gastos, saldos, metas e compromissos no FinFlow."
	maxTokensMessage = "A resposta ficou longa demais e foi interrompida. Tente uma pergunta mais específica."
	maxRoundsMessage = "Precisei de etapas demais para responder. Tente dividir a pergunta em partes menores."
)

// Emitter recebe os eventos SSE do chat (text, tool, proposal, done, error). Deve ser seguro para uso
// concorrente (propostas podem ser emitidas por ferramentas rodando em paralelo).
type Emitter func(event string, data any)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatInput struct {
	Messages []ChatMessage `json:"messages"`
}

// ValidateChatInput confere papéis, tamanhos e que a conversa termina numa mensagem do usuário.
func ValidateChatInput(in ChatInput) []domain.ValidationIssue {
	if len(in.Messages) == 0 || len(in.Messages) > maxChatMessages {
		return []domain.ValidationIssue{{Path: "messages", Message: "envie entre 1 e 60 mensagens"}}
	}
	var issues []domain.ValidationIssue
	for i, m := range in.Messages {
		path := "messages." + itoa(i)
		switch m.Role {
		case "user":
			if strings.TrimSpace(m.Content) == "" {
				issues = append(issues, domain.ValidationIssue{Path: path + ".content", Message: "mensagem vazia"})
			}
		case "assistant":
		default:
			issues = append(issues, domain.ValidationIssue{Path: path + ".role", Message: "role deve ser user ou assistant"})
		}
		if utf8.RuneCountInString(m.Content) > maxChatMessageRunes {
			issues = append(issues, domain.ValidationIssue{Path: path + ".content", Message: "mensagem longa demais"})
		}
	}
	if last := in.Messages[len(in.Messages)-1]; last.Role != "user" {
		issues = append(issues, domain.ValidationIssue{Path: "messages", Message: "a última mensagem deve ser do usuário"})
	}
	return issues
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

// buildMessages converte o histórico do cliente. A data de hoje vai no primeiro turno do usuário (depois do
// ponto de cache do system); respostas vazias do assistente (ex.: após um erro) são descartadas e a
// conversa sempre começa por um turno do usuário.
func buildMessages(in ChatInput, now time.Time) []anthropic.BetaMessageParam {
	out := make([]anthropic.BetaMessageParam, 0, len(in.Messages))
	first := true
	for _, m := range in.Messages {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if m.Role == "assistant" {
			if first {
				continue
			}
			out = append(out, anthropic.BetaMessageParam{OfAssistant: &anthropic.BetaAssistantMessageParam{
				Content: []anthropic.BetaContentBlockParamUnion{anthropic.NewBetaTextBlock(content)},
			}})
			continue
		}
		blocks := []anthropic.BetaContentBlockParamUnion{}
		if first {
			blocks = append(blocks, anthropic.NewBetaTextBlock(todayContext(now)))
			first = false
		}
		blocks = append(blocks, anthropic.NewBetaTextBlock(content))
		out = append(out, anthropic.NewBetaUserMessage(blocks...))
	}
	return out
}

// chatParams monta a requisição. Ordem de cache: tools (determinísticas) → system (congelado, com
// breakpoint explícito) → mensagens (breakpoint automático no fim, que acompanha a conversa crescendo).
func chatParams(messages []anthropic.BetaMessageParam) anthropic.BetaMessageNewParams {
	return anthropic.BetaMessageNewParams{
		Model:     Model,
		MaxTokens: chatMaxTokens,
		System: []anthropic.BetaTextBlockParam{{
			Text:         chatSystemPrompt,
			CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		}},
		CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		Tools:        toolParams(),
		// Pensamento adaptativo: `thinking` omitido de propósito (Opus 5.5 roda adaptativo por padrão).
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortMedium},
		Betas:        []anthropic.AnthropicBeta{fallbackBeta},
		Fallbacks:    anthropic.BetaFallbacksParamOfDefault(),
		Messages:     messages,
	}
}

// Chat roda o loop de ferramentas emitindo eventos. Erros antes do primeiro evento (ex.: sem chave) são
// devolvidos para o handler responder em JSON; depois disso o handler emite `error`.
func (s *Service) Chat(ctx context.Context, in ChatInput, emit Emitter) error {
	if issues := ValidateChatInput(in); len(issues) > 0 {
		return service.ValidationFailed(issues)
	}
	client, err := s.client(ctx)
	if err != nil {
		return err
	}
	messages := buildMessages(in, domain.Now())
	if len(messages) == 0 {
		return service.ValidationFailed([]domain.ValidationIssue{{Path: "messages", Message: "conversa vazia"}})
	}

	for round := 0; round < maxToolRounds; round++ {
		msg, err := s.streamTurn(ctx, client, messages, emit)
		if err != nil {
			return upstreamError(err)
		}
		switch msg.StopReason {
		case anthropic.BetaStopReasonRefusal:
			// Toda a cadeia (modelo + fallback) recusou: a resposta parcial deve ser descartada pela UI.
			emit("error", map[string]string{"message": refusalMessage})
			emit("done", map[string]string{"stopReason": string(msg.StopReason)})
			return nil
		case anthropic.BetaStopReasonMaxTokens:
			// Nunca executa ferramentas de um turno truncado (a entrada pode estar incompleta).
			emit("error", map[string]string{"message": maxTokensMessage})
			emit("done", map[string]string{"stopReason": string(msg.StopReason)})
			return nil
		case anthropic.BetaStopReasonToolUse:
			echo, calls := echoContent(msg)
			if len(calls) == 0 {
				emit("done", map[string]string{"stopReason": string(msg.StopReason)})
				return nil
			}
			messages = append(messages, echo)
			results := s.runToolCalls(ctx, calls, emit)
			messages = append(messages, anthropic.NewBetaUserMessage(results...))
		case anthropic.BetaStopReasonPauseTurn:
			echo, _ := echoContent(msg)
			messages = append(messages, echo)
		default:
			emit("done", map[string]string{"stopReason": string(msg.StopReason)})
			return nil
		}
	}
	emit("error", map[string]string{"message": maxRoundsMessage})
	emit("done", map[string]string{"stopReason": "max_rounds"})
	return nil
}

// streamTurn faz uma requisição em streaming, repassando texto e início de ferramentas, e devolve a
// mensagem acumulada.
func (s *Service) streamTurn(ctx context.Context, client anthropic.Client, messages []anthropic.BetaMessageParam, emit Emitter) (anthropic.BetaMessage, error) {
	stream := client.Beta.Messages.NewStreaming(ctx, chatParams(messages))
	defer stream.Close()
	msg := anthropic.BetaMessage{}
	for stream.Next() {
		event := stream.Current()
		if err := msg.Accumulate(event); err != nil {
			return msg, err
		}
		switch ev := event.AsAny().(type) {
		case anthropic.BetaRawContentBlockStartEvent:
			if ev.ContentBlock.Type == "tool_use" {
				emit("tool", map[string]string{"name": ev.ContentBlock.Name, "label": toolLabel(ev.ContentBlock.Name)})
			}
		case anthropic.BetaRawContentBlockDeltaEvent:
			if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
				emit("text", map[string]string{"delta": ev.Delta.Text})
			}
		}
	}
	return msg, stream.Err()
}

type toolCall struct {
	id    string
	name  string
	input json.RawMessage
}

// echoContent prepara o turno do assistente para reenviar e lista as chamadas a executar. Depois de um
// fallback no meio da saída, blocos thinking/tool_use anteriores ao último bloco `fallback` são omitidos
// (o bloco fallback fica na posição original) e só as chamadas posteriores a ele são executadas.
func echoContent(msg anthropic.BetaMessage) (anthropic.BetaMessageParam, []toolCall) {
	lastFallback := -1
	for i, block := range msg.Content {
		if block.Type == "fallback" {
			lastFallback = i
		}
	}
	params := make([]anthropic.BetaContentBlockParamUnion, 0, len(msg.Content))
	var calls []toolCall
	for i, block := range msg.Content {
		if i < lastFallback {
			switch block.Type {
			case "thinking", "redacted_thinking", "tool_use", "server_tool_use":
				continue
			}
		}
		params = append(params, block.ToParam())
		if block.Type == "tool_use" {
			calls = append(calls, toolCall{id: block.ID, name: block.Name, input: append(json.RawMessage(nil), block.Input...)})
		}
	}
	return anthropic.BetaMessageParam{OfAssistant: &anthropic.BetaAssistantMessageParam{Content: params}}, calls
}

// runToolCalls executa as chamadas em paralelo e devolve todos os tool_result na ordem original (um único
// turno do usuário).
func (s *Service) runToolCalls(ctx context.Context, calls []toolCall, emit Emitter) []anthropic.BetaContentBlockParamUnion {
	results := make([]anthropic.BetaContentBlockParamUnion, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			content, isError := s.runTool(ctx, call.name, call.input, emit)
			results[i] = anthropic.NewBetaToolResultBlock(call.id, content, isError)
		}()
	}
	wg.Wait()
	return results
}
