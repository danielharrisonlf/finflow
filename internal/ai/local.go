package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"finflow/internal/service"
)

// Ajustes dos provedores locais (Ollama e servidores compatíveis com a API da OpenAI).
const (
	// localMaxToolRounds: modelos locais erram mais em chamadas de ferramenta; 8 rodadas por pergunta.
	localMaxToolRounds = 8
	// localTurnTimeout limita um turno (modelos em CPU podem ser lentos; o SSE tem 15 min).
	localTurnTimeout = 10 * time.Minute
	// localReceiptTimeout acompanha o prazo de escrita do handler de comprovantes.
	localReceiptTimeout = 3 * time.Minute
	// localTemperature baixa deixa as chamadas de ferramenta mais estáveis.
	localTemperature = 0.2
	// ollamaNumCtx: o system prompt + 22 ferramentas não cabem no contexto padrão do Ollama.
	ollamaNumCtx = 16384
	// maxErrorBody limita o corpo de erro lido do servidor local.
	maxErrorBody = 8 << 10
)

// localPromptAddendum reforça, para modelos locais, o idioma e o uso do mecanismo de ferramentas.
const localPromptAddendum = `

Observações para este modelo:
- Responda sempre em português do Brasil, mesmo que os dados das ferramentas tenham campos em inglês.
- Para consultar dados ou montar propostas, use o mecanismo de chamada de ferramentas; não escreva o JSON da chamada no texto da resposta.`

// receiptJSONInstruction vai no pedido de leitura para modelos locais (que nem sempre aceitam schema).
const receiptJSONInstruction = "\n\nResponda somente com um objeto JSON com os campos description, amountCents, date, kind, categoryName, merchant, installments e confidence, sem texto antes ou depois."

// emptyAnswerMessage: o modelo local terminou sem texto nem ferramentas.
const emptyAnswerMessage = "O modelo não devolveu resposta. Tente de novo ou reformule a pergunta."

// localClient é a conexão HTTP com um servidor local (net/http + encoding/json, sem SDK).
type localClient struct {
	provider    string
	baseURL     string
	model       string
	visionModel string
	apiKey      string
	http        *http.Client
}

func (s *Service) localClient(r resolved) localClient {
	hc := s.cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	return localClient{
		provider: r.provider, baseURL: strings.TrimRight(r.baseURL, "/"), model: r.model,
		visionModel: r.visionModel, apiKey: r.apiKey, http: hc,
	}
}

func (c localClient) serverName() string {
	if c.provider == ProviderOllama {
		return "Ollama"
	}
	return "servidor compatível"
}

// send faz a requisição sem traduzir erros (usado pelo ping de status).
func (c localClient) send(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return c.http.Do(req)
}

// do faz a requisição e devolve erros já em pt-BR: falha de conexão (com a dica de CORS no navegador)
// ou status de erro traduzido por statusError. Em caso de sucesso, o chamador fecha o corpo.
func (c localClient) do(ctx context.Context, method, path string, body any, model string) (*http.Response, error) {
	resp, err := c.send(ctx, method, path, body)
	if err != nil {
		return nil, c.connectError(ctx, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return nil, c.statusError(resp.StatusCode, errorMessage(raw), model)
	}
	return resp, nil
}

// connectError traduz falhas de rede. Nunca devolve 401 (o frontend trataria como sessão expirada).
func (c localClient) connectError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &service.AppError{Status: http.StatusGatewayTimeout,
			Message: fmt.Sprintf("O %s demorou demais para responder. Modelos menores respondem mais rápido.", c.serverName())}
	}
	hint := "Confira se o Ollama está aberto (ou rode `ollama serve`)."
	if c.provider == ProviderOpenAICompatible {
		hint = "Confira se o servidor está rodando (no LM Studio: Developer → Start server)."
	}
	return &service.AppError{Status: http.StatusBadGateway,
		Message: fmt.Sprintf("Não foi possível conectar ao %s em %s. %s%s", c.serverName(), c.baseURL, hint, platformConnectHint())}
}

// streamError traduz falhas no meio da leitura de uma resposta em streaming.
func (c localClient) streamError(ctx context.Context, err error) error {
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return c.connectError(ctx, context.DeadlineExceeded)
	}
	return &service.AppError{Status: http.StatusBadGateway,
		Message: fmt.Sprintf("A conexão com o %s caiu no meio da resposta. Tente de novo.", c.serverName())}
}

func (c localClient) badResponse() error {
	return &service.AppError{Status: http.StatusBadGateway,
		Message: fmt.Sprintf("O %s devolveu uma resposta em formato inesperado. Confira a URL em Configurações → Assistente.", c.serverName())}
}

// statusError traduz status de erro (ou erro dentro do stream, com status 0) para pt-BR.
func (c localClient) statusError(status int, msg, model string) error {
	lower := strings.ToLower(msg)
	if status == http.StatusNotFound && model == "" {
		return &service.AppError{Status: http.StatusBadGateway, Message: fmt.Sprintf(
			"Endereço não encontrado em %s. Confira a URL em Configurações → Assistente (Ollama: %s; LM Studio: %s).", c.baseURL, DefaultOllamaURL, DefaultOpenAICompatibleURL)}
	}
	switch c.provider {
	case ProviderOllama:
		switch {
		case strings.Contains(lower, "does not support tools"):
			return &service.AppError{Status: http.StatusBadGateway, Message: fmt.Sprintf(
				"O modelo %s não aceita ferramentas. Use um modelo com suporte a ferramentas, como %s ou llama3.1:8b.", model, DefaultOllamaModel)}
		case status == http.StatusNotFound || (strings.Contains(lower, "model") && strings.Contains(lower, "not found")):
			return &service.AppError{Status: http.StatusBadGateway, Message: fmt.Sprintf(
				"O modelo %s não está instalado no Ollama. Rode `ollama pull %s` e tente de novo.", model, model)}
		}
	case ProviderOpenAICompatible:
		switch status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return &service.AppError{Status: http.StatusBadGateway,
				Message: "O servidor compatível recusou a chave da API. Confira a chave em Configurações → Assistente."}
		case http.StatusNotFound:
			return &service.AppError{Status: http.StatusBadGateway, Message: fmt.Sprintf(
				"Não encontrei o modelo %s ou o endpoint em %s. Confira a URL (ex.: %s) e se o modelo está carregado.", model, c.baseURL, DefaultOpenAICompatibleURL)}
		}
	}
	if status == http.StatusTooManyRequests {
		return &service.AppError{Status: http.StatusTooManyRequests, Message: fmt.Sprintf("O %s está ocupado. Tente de novo em instantes.", c.serverName())}
	}
	if msg == "" {
		return &service.AppError{Status: http.StatusBadGateway, Message: fmt.Sprintf("O %s respondeu com erro HTTP %d.", c.serverName(), status)}
	}
	return &service.AppError{Status: http.StatusBadGateway, Message: fmt.Sprintf("O %s respondeu com erro: %s", c.serverName(), truncateRunes(msg, 300))}
}

// errorMessage extrai a mensagem de corpos {"error":"..."}, {"error":{"message":"..."}} ou {"message":"..."}.
func errorMessage(raw []byte) string {
	var body struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return strings.TrimSpace(string(raw))
	}
	if len(body.Error) > 0 {
		var s string
		if json.Unmarshal(body.Error, &s) == nil {
			return s
		}
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body.Error, &obj) == nil && obj.Message != "" {
			return obj.Message
		}
	}
	return body.Message
}

func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// ───────────────────────── ferramentas no formato "function" ─────────────────────────

type functionTool struct {
	Type     string      `json:"type"`
	Function functionDef `json:"function"`
}

type functionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// functionTools converte as mesmas specs do Claude para o formato aceito pelo Ollama e pela API da
// OpenAI (a validação dos argumentos continua no binário, em validateToolInput).
func functionTools() []functionTool {
	specs := toolSpecs()
	out := make([]functionTool, 0, len(specs))
	for _, spec := range specs {
		required := spec.required
		if required == nil {
			required = []string{}
		}
		out = append(out, functionTool{Type: "function", Function: functionDef{
			Name:        spec.name,
			Description: spec.description,
			Parameters: map[string]any{
				"type": "object", "properties": spec.props, "required": required, "additionalProperties": false,
			},
		}})
	}
	return out
}

// normalizeArgs aceita argumentos como objeto ou como string com JSON; vazio vira {}.
func normalizeArgs(raw json.RawMessage) json.RawMessage {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || string(t) == "null" {
		return json.RawMessage("{}")
	}
	if t[0] == '"' {
		var s string
		if json.Unmarshal(t, &s) == nil {
			if strings.TrimSpace(s) == "" {
				return json.RawMessage("{}")
			}
			return json.RawMessage(s)
		}
	}
	return append(json.RawMessage(nil), t...)
}

// argsObject devolve os argumentos como objeto JSON válido para reenviar ao modelo ({} se inválidos).
func argsObject(raw json.RawMessage) json.RawMessage {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return json.RawMessage("{}")
	}
	return raw
}

// parseContentToolCall reconhece, em melhor esforço, um único objeto {"name": ..., "arguments": ...}
// escrito no texto da resposta (com ou sem cercas ``` ou <tool_call>), quando o modelo não usa o
// mecanismo nativo de ferramentas. Só vale para ferramentas conhecidas.
func parseContentToolCall(content string) (toolCall, bool) {
	t := strings.TrimSpace(content)
	t = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "<tool_call>"), "</tool_call>"))
	if strings.HasPrefix(t, "```") {
		if nl := strings.IndexByte(t, '\n'); nl >= 0 {
			t = t[nl+1:]
		}
		t = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(t), "```"))
	}
	if !strings.HasPrefix(t, "{") {
		return toolCall{}, false
	}
	dec := json.NewDecoder(strings.NewReader(t))
	var obj struct {
		Name       string          `json:"name"`
		Arguments  json.RawMessage `json:"arguments"`
		Parameters json.RawMessage `json:"parameters"`
	}
	if err := dec.Decode(&obj); err != nil {
		return toolCall{}, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return toolCall{}, false
	}
	if _, ok := findToolSpec(obj.Name); !ok {
		return toolCall{}, false
	}
	args := obj.Arguments
	if len(args) == 0 {
		args = obj.Parameters
	}
	return toolCall{name: obj.Name, input: normalizeArgs(args)}, true
}

// textHolder repassa o texto em streaming, mas segura respostas que começam como um possível JSON de
// chamada de ferramenta ({, ``` ou <tool_call>) até o fim do turno, para não mostrar o JSON ao usuário.
type textHolder struct {
	emit  Emitter
	buf   strings.Builder
	state int
}

const (
	holdUndecided = iota
	holdStreaming
	holdHolding
	holdDiscarded
)

func (h *textHolder) write(delta string) {
	if delta == "" {
		return
	}
	if h.state == holdStreaming {
		h.emit("text", map[string]string{"delta": delta})
		return
	}
	h.buf.WriteString(delta)
	if h.state != holdUndecided {
		return
	}
	t := strings.TrimLeft(h.buf.String(), " \t\r\n")
	switch {
	case t == "":
	case t[0] == '{' || t[0] == '`' || strings.HasPrefix(t, "<tool_call>"):
		h.state = holdHolding
	case strings.HasPrefix("<tool_call>", t):
		// ainda pode ser o começo de <tool_call>
	default:
		h.state = holdStreaming
		h.emit("text", map[string]string{"delta": h.buf.String()})
		h.buf.Reset()
	}
}

// discard joga fora o texto segurado (era uma chamada de ferramenta).
func (h *textHolder) discard() {
	h.state = holdDiscarded
	h.buf.Reset()
}

// flush emite o que ainda estiver segurado.
func (h *textHolder) flush() {
	if h.state != holdDiscarded && h.buf.Len() > 0 {
		h.emit("text", map[string]string{"delta": h.buf.String()})
		h.buf.Reset()
	}
}

// ───────────────────────── sessão compartilhada dos provedores locais ─────────────────────────

// localMsg é a conversa num formato neutro; cada provedor serializa no seu.
type localMsg struct {
	role    string // system | user | assistant | tool
	content string
	calls   []toolCall  // assistant
	result  *toolResult // tool
}

// localTurn é o que um provedor local devolve depois de ler um turno em streaming.
type localTurn struct {
	content   string
	calls     []toolCall
	truncated bool
}

type localStreamer interface {
	streamChat(ctx context.Context, msgs []localMsg, onText func(string)) (localTurn, error)
}

type localSession struct {
	model localStreamer
	msgs  []localMsg
	seq   int
}

// newLocalSession monta a conversa com o mesmo system prompt (mais o reforço para modelos locais) e a
// data de hoje no primeiro turno do usuário.
func newLocalSession(model localStreamer, history []ChatMessage, now time.Time) *localSession {
	msgs := []localMsg{{role: "system", content: chatSystemPrompt + localPromptAddendum}}
	first := true
	for _, m := range history {
		content := m.Content
		if m.Role == "user" && first {
			content = todayContext(now) + "\n\n" + content
			first = false
		}
		msgs = append(msgs, localMsg{role: m.Role, content: content})
	}
	return &localSession{model: model, msgs: msgs}
}

func (l *localSession) turn(ctx context.Context, emit Emitter) (turnResult, error) {
	text := &textHolder{emit: emit}
	out, err := l.model.streamChat(ctx, l.msgs, text.write)
	if err != nil {
		return turnResult{}, err
	}
	calls, content := out.calls, out.content
	if len(calls) == 0 && !out.truncated {
		if call, ok := parseContentToolCall(content); ok {
			calls, content = []toolCall{call}, ""
			text.discard()
		}
	}
	text.flush()
	if out.truncated {
		return turnResult{kind: turnMaxTokens, stopReason: "max_tokens"}, nil
	}
	if len(calls) == 0 {
		if strings.TrimSpace(content) == "" {
			emit("error", map[string]string{"message": emptyAnswerMessage})
		}
		l.msgs = append(l.msgs, localMsg{role: "assistant", content: content})
		return turnResult{kind: turnEnd, stopReason: "end_turn"}, nil
	}
	for i := range calls {
		if calls[i].id == "" {
			l.seq++
			calls[i].id = fmt.Sprintf("call_%d", l.seq)
		}
		emit("tool", map[string]string{"name": calls[i].name, "label": toolLabel(calls[i].name)})
	}
	l.msgs = append(l.msgs, localMsg{role: "assistant", content: content, calls: calls})
	return turnResult{kind: turnTools, stopReason: "tool_use", calls: calls}, nil
}

func (l *localSession) addToolResults(results []toolResult) {
	for i := range results {
		l.msgs = append(l.msgs, localMsg{role: "tool", content: results[i].content, result: &results[i]})
	}
}
