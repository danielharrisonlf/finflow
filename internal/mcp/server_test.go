package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"finflow/internal/client"
	"finflow/internal/db"
	"finflow/internal/service"
	"finflow/internal/store"
)

func TestMCPStdioServer(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, "", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	st := store.New(database)
	if _, err := store.Seed(ctx, st); err != nil {
		t.Fatal(err)
	}
	svcs := service.New(st, nil)
	backend := &client.LocalBackend{Svcs: svcs}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_accounts","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"create_expense","arguments":{"amountCents":4590,"description":"Café MCP","account":"Nubank","category":"Alimentação"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_summary","arguments":{}}}`,
	}, "\n") + "\n"

	var out, errOut bytes.Buffer
	srv := &Server{
		backend: backend,
		in:      strings.NewReader(input),
		out:     &out,
		errOut:  &errOut,
	}
	if err := srv.Run(ctx, "test"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 JSON-RPC responses (notification ignored), got %d:\n%s", len(lines), out.String())
	}

	// Verify 19 tools listed
	var listResp struct {
		Result struct {
			Tools []toolDef `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Result.Tools) != 19 {
		t.Fatalf("expected 19 MCP tools, got %d", len(listResp.Result.Tools))
	}

	// Verify create_expense succeeded
	var expResp struct {
		Result toolResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[3]), &expResp); err != nil {
		t.Fatal(err)
	}
	if expResp.Result.IsError || !strings.Contains(expResp.Result.Content[0].Text, "Café MCP") {
		t.Fatalf("create_expense unexpected: %+v", expResp.Result)
	}
}
