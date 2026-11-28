package ai

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// openAIProvider fala com servidores compatíveis com a API da OpenAI (LM Studio, llama.cpp
// `llama-server`, vLLM, LocalAI): POST {baseUrl}/chat/completions com `tools` e `stream: true` (SSE,
// delta.tool_calls acumulados por index).
type openAIProvider struct {
	local localClient
}

func (p *openAIProvider) ID() string         { return ProviderOpenAICompatible }
func (p *openAIProvider) maxToolRounds() int { return localMaxToolRounds }

func (p *openAIProvider) newSession(_ context.Context, history []ChatMessage, now time.Time) chatSession {
	return newLocalSession(p, history, now)
}

type oaRequest struct {
	Model          string         `json:"model"`
	Messages       []oaMessage    `json:"messages"`
	Tools          []functionTool `json:"tools,omitempty"`
	Stream         bool           `json:"stream"`
	Temperature    float64        `json:"temperature"`
	ResponseFormat any            `json:"response_format,omitempty"`
}

type oaMessage struct {
	Role       string       `json:"role"`
	Content    any          `json:"content"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error json.RawMessage `json:"error"`
}

type oaCompletion struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func oaMessages(msgs []localMsg) []oaMessage {
	out := make([]oaMessage, 0, len(msgs))
	for _, m := range msgs {
		om := oaMessage{Role: m.role, Content: m.content}
		for _, c := range m.calls {
			tc := oaToolCall{ID: c.id, Type: "function"}
			tc.Function.Name = c.name
			tc.Function.Arguments = string(c.input)
			om.ToolCalls = append(om.ToolCalls, tc)
		}
		if len(om.ToolCalls) > 0 && m.content == "" {
			om.Content = nil
		}
		if m.result != nil {
			om.ToolCallID = m.result.call.id
		}
		out = append(out, om)
	}
	return out
}

type partialCall struct {
	id, name string
	args     strings.Builder
}

func (p *openAIProvider) streamChat(ctx context.Context, msgs []localMsg, onText func(string)) (localTurn, error) {
	ctx, cancel := context.WithTimeout(ctx, localTurnTimeout)
	defer cancel()
	resp, err := p.local.do(ctx, http.MethodPost, "/chat/completions", oaRequest{
		Model:       p.local.model,
		Messages:    oaMessages(msgs),
		Tools:       functionTools(),
		Stream:      true,
		Temperature: localTemperature,
	}, p.local.model)
	if err != nil {
		return localTurn{}, err
	}
	defer resp.Body.Close()

	var out localTurn
	var content strings.Builder
	partials := map[int]*partialCall{}
	reader := bufio.NewReader(resp.Body)
	for {
		line, readErr := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if payload, ok := strings.CutPrefix(line, "data:"); ok {
			payload = strings.TrimSpace(payload)
			if payload == "[DONE]" {
				break
			}
			var chunk oaStreamChunk
			if json.Unmarshal([]byte(payload), &chunk) != nil {
				return out, p.local.badResponse()
			}
			if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
				return out, p.local.statusError(0, errorMessage([]byte(`{"error":`+string(chunk.Error)+`}`)), p.local.model)
			}
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					content.WriteString(choice.Delta.Content)
					onText(choice.Delta.Content)
				}
				for _, d := range choice.Delta.ToolCalls {
					pc := partials[d.Index]
					if pc == nil {
						pc = &partialCall{}
						partials[d.Index] = pc
					}
					if d.ID != "" {
						pc.id = d.ID
					}
					if d.Function.Name != "" && d.Function.Name != pc.name {
						pc.name += d.Function.Name
					}
					pc.args.WriteString(d.Function.Arguments)
				}
				if choice.FinishReason != nil && *choice.FinishReason == "length" {
					out.truncated = true
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return out, p.local.streamError(ctx, readErr)
		}
	}
	indexes := make([]int, 0, len(partials))
	for i := range partials {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)
	for _, i := range indexes {
		pc := partials[i]
		out.calls = append(out.calls, toolCall{id: pc.id, name: pc.name, input: normalizeArgs(json.RawMessage(pc.args.String()))})
	}
	out.content = content.String()
	return out, nil
}

// extractReceipt manda a imagem como data URL e pede `response_format` json_schema; se o servidor não
// aceitar, repete sem o schema (o pedido já instrui a responder só com JSON) e interpreta o texto.
func (p *openAIProvider) extractReceipt(ctx context.Context, req receiptRequest) (receiptRaw, error) {
	if req.mediaType == "application/pdf" {
		return receiptRaw{}, errReceiptPDFLocal
	}
	ctx, cancel := context.WithTimeout(ctx, localReceiptTimeout)
	defer cancel()
	model := p.local.visionModel
	body := oaRequest{
		Model: model,
		Messages: []oaMessage{
			{Role: "system", Content: receiptSystemPrompt},
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": req.prompt + receiptJSONInstruction},
				map[string]any{"type": "image_url", "image_url": map[string]any{
					"url": "data:" + req.mediaType + ";base64," + base64.StdEncoding.EncodeToString(req.data),
				}},
			}},
		},
		ResponseFormat: map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": "receipt", "strict": true, "schema": receiptSchema()},
		},
	}
	resp, err := p.local.send(ctx, http.MethodPost, "/chat/completions", body)
	if err != nil {
		return receiptRaw{}, p.local.connectError(ctx, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests:
			return receiptRaw{}, p.local.statusError(resp.StatusCode, errorMessage(raw), model)
		}
		// Provavelmente sem suporte a response_format: tenta de novo só com a instrução de JSON.
		body.ResponseFormat = nil
		if resp, err = p.local.do(ctx, http.MethodPost, "/chat/completions", body, model); err != nil {
			return receiptRaw{}, err
		}
	}
	defer resp.Body.Close()
	var out oaCompletion
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Choices) == 0 {
		return receiptRaw{}, p.local.badResponse()
	}
	return parseReceiptJSON(out.Choices[0].Message.Content)
}
