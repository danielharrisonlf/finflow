package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// ollamaProvider fala com o Ollama pela API nativa: POST /api/chat com `tools` e `stream: true`
// (chunks NDJSON, chamadas em message.tool_calls).
type ollamaProvider struct {
	local localClient
}

func (p *ollamaProvider) ID() string         { return ProviderOllama }
func (p *ollamaProvider) maxToolRounds() int { return localMaxToolRounds }

func (p *ollamaProvider) newSession(_ context.Context, history []ChatMessage, now time.Time) chatSession {
	return newLocalSession(p, history, now)
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []functionTool  `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
	Format   any             `json:"format,omitempty"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	ToolName  string           `json:"tool_name,omitempty"`
}

type ollamaToolCall struct {
	ID       string `json:"id,omitempty"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

type ollamaChunk struct {
	Message struct {
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls"`
	} `json:"message"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason"`
	Error      string `json:"error"`
}

func ollamaMessages(msgs []localMsg) []ollamaMessage {
	out := make([]ollamaMessage, 0, len(msgs))
	for _, m := range msgs {
		om := ollamaMessage{Role: m.role, Content: m.content}
		for _, c := range m.calls {
			var tc ollamaToolCall
			tc.Function.Name = c.name
			tc.Function.Arguments = argsObject(c.input)
			om.ToolCalls = append(om.ToolCalls, tc)
		}
		if m.result != nil {
			om.ToolName = m.result.call.name
		}
		out = append(out, om)
	}
	return out
}

func (p *ollamaProvider) streamChat(ctx context.Context, msgs []localMsg, onText func(string)) (localTurn, error) {
	ctx, cancel := context.WithTimeout(ctx, localTurnTimeout)
	defer cancel()
	resp, err := p.local.do(ctx, http.MethodPost, "/api/chat", ollamaChatRequest{
		Model:    p.local.model,
		Messages: ollamaMessages(msgs),
		Tools:    functionTools(),
		Stream:   true,
		Options:  map[string]any{"temperature": localTemperature, "num_ctx": ollamaNumCtx},
	}, p.local.model)
	if err != nil {
		return localTurn{}, err
	}
	defer resp.Body.Close()

	var out localTurn
	var content strings.Builder
	reader := bufio.NewReader(resp.Body)
	for done := false; !done; {
		line, readErr := reader.ReadBytes('\n')
		if line = bytes.TrimSpace(line); len(line) > 0 {
			var chunk ollamaChunk
			if json.Unmarshal(line, &chunk) != nil {
				return out, p.local.badResponse()
			}
			if chunk.Error != "" {
				return out, p.local.statusError(0, chunk.Error, p.local.model)
			}
			if chunk.Message.Content != "" {
				content.WriteString(chunk.Message.Content)
				onText(chunk.Message.Content)
			}
			for _, tc := range chunk.Message.ToolCalls {
				out.calls = append(out.calls, toolCall{id: tc.ID, name: tc.Function.Name, input: normalizeArgs(tc.Function.Arguments)})
			}
			if chunk.Done {
				out.truncated = chunk.DoneReason == "length"
				done = true
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil && !done {
			return out, p.local.streamError(ctx, readErr)
		}
	}
	out.content = content.String()
	return out, nil
}

// extractReceipt usa o modelo de visão com a imagem em base64 e `format` com o JSON schema.
func (p *ollamaProvider) extractReceipt(ctx context.Context, req receiptRequest) (receiptRaw, error) {
	if req.mediaType == "application/pdf" {
		return receiptRaw{}, errReceiptPDFLocal
	}
	ctx, cancel := context.WithTimeout(ctx, localReceiptTimeout)
	defer cancel()
	model := p.local.visionModel
	resp, err := p.local.do(ctx, http.MethodPost, "/api/chat", ollamaChatRequest{
		Model: model,
		Messages: []ollamaMessage{
			{Role: "system", Content: receiptSystemPrompt},
			{Role: "user", Content: req.prompt + receiptJSONInstruction, Images: []string{base64.StdEncoding.EncodeToString(req.data)}},
		},
		Stream:  false,
		Format:  receiptSchema(),
		Options: map[string]any{"temperature": 0},
	}, model)
	if err != nil {
		return receiptRaw{}, err
	}
	defer resp.Body.Close()
	var body ollamaChunk
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return receiptRaw{}, p.local.badResponse()
	}
	if body.Error != "" {
		return receiptRaw{}, p.local.statusError(0, body.Error, model)
	}
	return parseReceiptJSON(body.Message.Content)
}
