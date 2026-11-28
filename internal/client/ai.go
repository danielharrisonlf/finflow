package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"finflow/internal/domain"
)

// AIChatMessage é uma mensagem da conversa enviada a POST /api/ai/chat.
type AIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIChat envia a conversa a POST /api/ai/chat e chama onEvent para cada evento SSE (text, tool,
// proposal, done, error) com o JSON do campo data. Erros antes do stream (ex.: 412 sem assistente
// configurado) voltam como *APIError. Sem timeout total: a resposta chega em streaming e pode demorar.
func (c *HTTPClient) AIChat(ctx context.Context, messages []AIChatMessage, onEvent func(event string, data json.RawMessage)) error {
	body, err := json.Marshal(map[string]any{"messages": messages})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ai/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.space != "" {
		req.Header.Set("X-FinFlow-Space", c.space)
	}
	streaming := &http.Client{Transport: c.http.Transport}
	resp, err := streaming.Do(req)
	if err != nil {
		return &APIError{Message: fmt.Sprintf("não consegui falar com a API em %s — ela está rodando? (%v)", c.baseURL, err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return decodeAPIError(resp.StatusCode, raw)
	}

	reader := bufio.NewReader(resp.Body)
	event := "message"
	var data []string
	dispatch := func() {
		if len(data) > 0 {
			onEvent(event, json.RawMessage(strings.Join(data, "\n")))
		}
		event, data = "message", nil
	}
	for {
		line, readErr := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			dispatch()
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimSpace(line[len("data:"):]))
		}
		if errors.Is(readErr, io.EOF) {
			dispatch()
			return nil
		}
		if readErr != nil {
			return &APIError{Message: fmt.Sprintf("a conexão com a API caiu no meio da resposta (%v)", readErr)}
		}
	}
}

// CreateWishItem adiciona um item à lista de desejos (usado ao aplicar propostas do Assistente).
func (c *HTTPClient) CreateWishItem(ctx context.Context, in domain.CreateWishItemInput) (domain.WishItem, error) {
	var out domain.WishItem
	return out, c.do(ctx, http.MethodPost, "/wishlist", in, &out)
}

// CreateWishItem adiciona um item à lista de desejos (usado ao aplicar propostas do Assistente).
func (l *LocalBackend) CreateWishItem(ctx context.Context, in domain.CreateWishItemInput) (domain.WishItem, error) {
	return l.Svcs.CreateWishItem(ctx, in)
}
