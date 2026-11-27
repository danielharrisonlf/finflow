package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"finflow/internal/client"
)

// ───────────────────────── Streamable HTTP e uso em processo ─────────────────────────

// HandleJSONRPC processa o corpo de um POST do transporte Streamable HTTP: uma mensagem JSON-RPC
// ou um lote (array). Devolve a resposta serializada e true; ou nil e false quando o corpo só tem
// notificações/respostas (o transporte responde 202 sem corpo). O protocolo é o mesmo do stdio.
func (s *Server) HandleJSONRPC(ctx context.Context, body []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil || len(batch) == 0 {
			return mustMarshal(parseErrorResponse()), true
		}
		var out []rpcResponse
		for _, raw := range batch {
			if resp, ok := s.handleRaw(ctx, raw); ok {
				out = append(out, resp)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return mustMarshal(out), true
	}
	resp, ok := s.handleRaw(ctx, trimmed)
	if !ok {
		return nil, false
	}
	return mustMarshal(resp), true
}

// handleRaw decodifica e despacha uma mensagem; notificações (sem id) não têm resposta.
func (s *Server) handleRaw(ctx context.Context, raw []byte) (rpcResponse, bool) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return parseErrorResponse(), true
	}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return rpcResponse{}, false
	}
	return s.handle(ctx, req), true
}

func parseErrorResponse() rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "JSON inválido"}}
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32603, Message: "erro interno"}})
	}
	return b
}

// Tool descreve uma tool do MCP para quem usa o dispatcher em processo (ex.: o Assistente).
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	// ReadOnly indica tools que só consultam (get_*, list_*, export_*); as demais gravam dados.
	ReadOnly bool
}

// Tools lista as mesmas tools de `tools/list`, na mesma ordem.
func Tools() []Tool {
	defs := allTools()
	out := make([]Tool, 0, len(defs))
	for _, d := range defs {
		out = append(out, Tool{Name: d.Name, Description: d.Description, InputSchema: d.InputSchema, ReadOnly: isReadOnlyTool(d.Name)})
	}
	return out
}

func isReadOnlyTool(name string) bool {
	for _, prefix := range []string{"get_", "list_", "export_"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// Call executa uma tool pelo nome com os argumentos JSON (nil = {}), com a mesma validação de
// `tools/call`. Devolve o texto do resultado (JSON indentado na maioria das tools) ou um erro
// com a mensagem que o MCP mostraria com isError.
func Call(ctx context.Context, backend client.Backend, name string, args json.RawMessage) (string, error) {
	res := (&Server{backend: backend}).callTool(ctx, name, args)
	text := ""
	if len(res.Content) > 0 {
		text = res.Content[0].Text
	}
	if res.IsError {
		return "", errors.New(strings.TrimPrefix(text, "erro: "))
	}
	return text, nil
}
