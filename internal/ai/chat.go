package ai

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"finflow/internal/domain"
	"finflow/internal/service"
)

const (
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

// chatHistory normaliza o histórico do cliente para qualquer provedor: textos sem espaços nas pontas,
// respostas vazias do assistente (ex.: após um erro) descartadas e a conversa sempre começando por um
// turno do usuário.
func chatHistory(in ChatInput) []ChatMessage {
	out := make([]ChatMessage, 0, len(in.Messages))
	for _, m := range in.Messages {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if m.Role == "assistant" && len(out) == 0 {
			continue
		}
		out = append(out, ChatMessage{Role: m.Role, Content: content})
	}
	return out
}

// Provider é o cliente de um modelo. O loop do agente, as ferramentas, as propostas, o contrato SSE e
// o system prompt são compartilhados (Service.Chat e Service.ExtractReceipt); cada provedor só sabe
// conversar com o próprio modelo.
type Provider interface {
	// ID é o identificador do provedor (ollama, openai_compatible, claude).
	ID() string
	// newSession abre uma conversa com o histórico normalizado (começa por um turno do usuário).
	newSession(ctx context.Context, history []ChatMessage, now time.Time) chatSession
	// maxToolRounds limita as rodadas de ferramentas numa mesma pergunta.
	maxToolRounds() int
	// extractReceipt lê um comprovante e devolve os campos crus (antes de normalizeReceipt).
	extractReceipt(ctx context.Context, req receiptRequest) (receiptRaw, error)
}

// chatSession guarda a conversa no formato do provedor entre os turnos do loop.
type chatSession interface {
	// turn faz uma requisição em streaming, emitindo `text` e `tool`, e diz como o turno terminou.
	// Erros já vêm traduzidos para mensagens em pt-BR (service.AppError).
	turn(ctx context.Context, emit Emitter) (turnResult, error)
	// addToolResults acrescenta os resultados das chamadas do último turno, na mesma ordem.
	addToolResults(results []toolResult)
}

type turnKind int

const (
	turnEnd turnKind = iota
	turnTools
	turnPause
	turnRefusal
	turnMaxTokens
)

type turnResult struct {
	kind turnKind
	// stopReason vai no evento `done` (end_turn, tool_use, max_tokens, refusal…).
	stopReason string
	calls      []toolCall
}

type toolCall struct {
	id    string
	name  string
	input json.RawMessage
}

type toolResult struct {
	call    toolCall
	content string
	isError bool
}

// Chat roda o loop de ferramentas emitindo eventos. Erros antes do primeiro evento (ex.: sem
// configuração, servidor local fora do ar) são devolvidos para o handler responder em JSON; depois
// disso o handler emite `error`.
func (s *Service) Chat(ctx context.Context, in ChatInput, emit Emitter) error {
	if issues := ValidateChatInput(in); len(issues) > 0 {
		return service.ValidationFailed(issues)
	}
	p, _, err := s.provider(ctx)
	if err != nil {
		return err
	}
	history := chatHistory(in)
	if len(history) == 0 {
		return service.ValidationFailed([]domain.ValidationIssue{{Path: "messages", Message: "conversa vazia"}})
	}
	session := p.newSession(ctx, history, domain.Now())

	limit := p.maxToolRounds()
	for rounds := 0; ; {
		res, err := session.turn(ctx, emit)
		if err != nil {
			return err
		}
		switch res.kind {
		case turnRefusal:
			// Toda a cadeia (modelo + fallback) recusou: a resposta parcial deve ser descartada pela UI.
			emit("error", map[string]string{"message": refusalMessage})
			emit("done", map[string]string{"stopReason": res.stopReason})
			return nil
		case turnMaxTokens:
			// Nunca executa ferramentas de um turno truncado (a entrada pode estar incompleta).
			emit("error", map[string]string{"message": maxTokensMessage})
			emit("done", map[string]string{"stopReason": res.stopReason})
			return nil
		case turnTools, turnPause:
			if rounds >= limit {
				emit("error", map[string]string{"message": maxRoundsMessage})
				emit("done", map[string]string{"stopReason": "max_rounds"})
				return nil
			}
			rounds++
			if res.kind == turnTools {
				session.addToolResults(s.runToolCalls(ctx, res.calls, emit))
			}
		default:
			emit("done", map[string]string{"stopReason": res.stopReason})
			return nil
		}
	}
}

// runToolCalls executa as chamadas em paralelo e devolve os resultados na ordem original (um único
// turno de resultados). Erros de validação ou execução voltam como resultado de erro para o modelo
// corrigir.
func (s *Service) runToolCalls(ctx context.Context, calls []toolCall, emit Emitter) []toolResult {
	results := make([]toolResult, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			content, isError := s.runTool(ctx, call.name, call.input, emit)
			results[i] = toolResult{call: call, content: content, isError: isError}
		}()
	}
	wg.Wait()
	return results
}
