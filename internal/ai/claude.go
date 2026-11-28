package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"finflow/internal/service"
)

const (
	// chatMaxTokens: streaming, então não há risco de timeout HTTP; respostas são curtas pelo prompt.
	chatMaxTokens = 32000
	// claudeMaxToolRounds: até 9 rodadas de ferramentas + o turno da resposta = no máximo 10
	// requisições por pergunta (o mesmo teto de antes dos provedores locais).
	claudeMaxToolRounds = 9
	receiptMaxTokens    = 16000
)

// claudeProvider fala com a API do Claude pelo SDK oficial (Messages API beta, com fallback no
// servidor, cache de prompt e esforço configurado).
type claudeProvider struct {
	client anthropic.Client
}

func (p *claudeProvider) ID() string         { return ProviderClaude }
func (p *claudeProvider) maxToolRounds() int { return claudeMaxToolRounds }

func (p *claudeProvider) newSession(_ context.Context, history []ChatMessage, now time.Time) chatSession {
	return &claudeSession{client: p.client, messages: buildMessages(history, now)}
}

// buildMessages converte o histórico normalizado. A data de hoje vai no primeiro turno do usuário
// (depois do ponto de cache do system).
func buildMessages(history []ChatMessage, now time.Time) []anthropic.BetaMessageParam {
	out := make([]anthropic.BetaMessageParam, 0, len(history))
	first := true
	for _, m := range history {
		if m.Role == "assistant" {
			out = append(out, anthropic.BetaMessageParam{OfAssistant: &anthropic.BetaAssistantMessageParam{
				Content: []anthropic.BetaContentBlockParamUnion{anthropic.NewBetaTextBlock(m.Content)},
			}})
			continue
		}
		blocks := []anthropic.BetaContentBlockParamUnion{}
		if first {
			blocks = append(blocks, anthropic.NewBetaTextBlock(todayContext(now)))
			first = false
		}
		blocks = append(blocks, anthropic.NewBetaTextBlock(m.Content))
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

type claudeSession struct {
	client   anthropic.Client
	messages []anthropic.BetaMessageParam
}

func (c *claudeSession) turn(ctx context.Context, emit Emitter) (turnResult, error) {
	msg, err := streamTurn(ctx, c.client, c.messages, emit)
	if err != nil {
		return turnResult{}, upstreamError(err)
	}
	res := turnResult{stopReason: string(msg.StopReason)}
	switch msg.StopReason {
	case anthropic.BetaStopReasonRefusal:
		res.kind = turnRefusal
	case anthropic.BetaStopReasonMaxTokens:
		res.kind = turnMaxTokens
	case anthropic.BetaStopReasonToolUse:
		echo, calls := echoContent(msg)
		if len(calls) == 0 {
			return res, nil
		}
		c.messages = append(c.messages, echo)
		res.kind, res.calls = turnTools, calls
	case anthropic.BetaStopReasonPauseTurn:
		echo, _ := echoContent(msg)
		c.messages = append(c.messages, echo)
		res.kind = turnPause
	}
	return res, nil
}

// addToolResults manda todos os tool_result num único turno do usuário, na ordem original.
func (c *claudeSession) addToolResults(results []toolResult) {
	blocks := make([]anthropic.BetaContentBlockParamUnion, len(results))
	for i, r := range results {
		blocks[i] = anthropic.NewBetaToolResultBlock(r.call.id, r.content, r.isError)
	}
	c.messages = append(c.messages, anthropic.NewBetaUserMessage(blocks...))
}

// streamTurn faz uma requisição em streaming, repassando texto e início de ferramentas, e devolve a
// mensagem acumulada.
func streamTurn(ctx context.Context, client anthropic.Client, messages []anthropic.BetaMessageParam, emit Emitter) (anthropic.BetaMessage, error) {
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

// extractReceipt lê o comprovante (imagem ou PDF) com structured outputs.
func (p *claudeProvider) extractReceipt(ctx context.Context, req receiptRequest) (receiptRaw, error) {
	b64 := base64.StdEncoding.EncodeToString(req.data)
	var fileBlock anthropic.BetaContentBlockParamUnion
	if req.mediaType == "application/pdf" {
		fileBlock = anthropic.NewBetaDocumentBlock(anthropic.BetaBase64PDFSourceParam{Data: b64})
	} else {
		fileBlock = anthropic.NewBetaImageBlock(anthropic.BetaBase64ImageSourceParam{
			Data: b64, MediaType: anthropic.BetaBase64ImageSourceMediaType(req.mediaType),
		})
	}
	msg, err := p.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     Model,
		MaxTokens: receiptMaxTokens,
		System:    []anthropic.BetaTextBlockParam{{Text: receiptSystemPrompt}},
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortLow,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: receiptSchema()},
		},
		Betas:     []anthropic.AnthropicBeta{fallbackBeta},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
		Messages:  []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(fileBlock, anthropic.NewBetaTextBlock(req.prompt))},
	})
	if err != nil {
		return receiptRaw{}, upstreamError(err)
	}
	switch msg.StopReason {
	case anthropic.BetaStopReasonRefusal:
		return receiptRaw{}, errReceiptUnreadable
	case anthropic.BetaStopReasonMaxTokens:
		return receiptRaw{}, &service.AppError{Status: http.StatusBadGateway, Message: "A leitura do comprovante foi interrompida. Tente uma foto mais nítida."}
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	var raw receiptRaw
	if err := json.Unmarshal([]byte(text.String()), &raw); err != nil {
		return receiptRaw{}, errReceiptFormat
	}
	return raw, nil
}
