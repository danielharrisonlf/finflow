package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/testutil"
)

func TestPlanningToolsValidation(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		backend := seededBackend(t, database)
		before, err := backend.Recurring(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		invalid := []struct {
			name, tool, args, wantMsg string
		}{
			{"dias 0", "get_projection", `{"days":0}`, "days: deve ser >= 1"},
			{"dias 366", "get_projection", `{"days":366}`, "days: deve ser <= 365"},
			{"mês 13", "get_insights", `{"month":13}`, "month: deve ser <= 12"},
			{"ano 1999", "get_year_overview", `{"year":1999}`, "year: valor numérico inválido"},
			{"recorrência vazia", "create_recurring", `{}`, "description: obrigatório; kind: obrigatório; amountCents: obrigatório; dayOfMonth: obrigatório"},
			{"kind inválido", "create_recurring", `{"description":"x","kind":"income","amountCents":1,"dayOfMonth":1}`, "kind: deve ser um de"},
			{"dia 32", "create_recurring", `{"description":"x","kind":"EXPENSE","amountCents":1,"dayOfMonth":32}`, "dayOfMonth: deve ser <= 31"},
			{"data inválida", "create_recurring", `{"description":"x","kind":"EXPENSE","amountCents":1,"dayOfMonth":1,"startDate":"01/01/2026"}`, "startDate: formato inválido"},
			{"conta inexistente", "create_recurring", `{"description":"x","kind":"EXPENSE","amountCents":1,"dayOfMonth":1,"account":"Bradesco"}`, `conta "Bradesco" não encontrada`},
			{"categoria de outro tipo", "create_recurring", `{"description":"x","kind":"INCOME","amountCents":1,"dayOfMonth":1,"category":"Aluguel"}`, `categoria "Aluguel" não encontrada`},
			{"fim antes do início", "create_recurring", `{"description":"x","kind":"EXPENSE","amountCents":1,"dayOfMonth":1,"startDate":"2026-05-01","endDate":"2026-01-01"}`, "endDate"},
		}
		lines := make([]string, len(invalid))
		for i, tc := range invalid {
			lines[i] = callLine(i+1, tc.tool, tc.args)
		}
		resps := runLines(t, backend, lines...)
		if len(resps) != len(invalid) {
			t.Fatalf("esperava %d respostas, veio %d", len(invalid), len(resps))
		}
		for i, tc := range invalid {
			tr := toolResultOf(t, resps[i])
			if !tr.IsError || !strings.Contains(tr.Content[0].Text, tc.wantMsg) {
				t.Errorf("%s: esperava isError com %q, veio isError=%v %q", tc.name, tc.wantMsg, tr.IsError, tr.Content[0].Text)
			}
		}
		after, err := backend.Recurring(context.Background())
		if err != nil || len(after) != len(before) {
			t.Fatalf("chamadas inválidas gravaram recorrências: %d -> %d (%v)", len(before), len(after), err)
		}
	})
}

func TestPlanningToolsHappyPath(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		backend := seededBackend(t, database)
		resps := runLines(t, backend,
			callLine(1, "get_insights", `{}`),
			callLine(2, "get_projection", `{}`),
			callLine(3, "get_projection", `{"days":5}`),
			callLine(4, "get_year_overview", `{}`),
			callLine(5, "list_recurring", `{}`),
			callLine(6, "create_recurring", `{"description":"Netflix","kind":"EXPENSE","amountCents":5590,"dayOfMonth":3,"account":"nubank","category":"Streaming"}`),
		)
		if len(resps) != 6 {
			t.Fatalf("esperava 6 respostas, veio %d", len(resps))
		}
		wants := []string{`"performance"`, `"days"`, `"lowest"`, `"months"`, `"Aluguel"`, `"Netflix"`}
		for i, want := range wants {
			tr := toolResultOf(t, resps[i])
			if tr.IsError || !strings.Contains(tr.Content[0].Text, want) {
				t.Errorf("chamada %d: esperava sucesso contendo %q, veio isError=%v %.200q", i+1, want, tr.IsError, tr.Content[0].Text)
			}
		}
		var p domain.Projection
		if err := json.Unmarshal([]byte(toolResultOf(t, resps[1]).Content[0].Text), &p); err != nil || len(p.Days) != 30 {
			t.Fatalf("get_projection padrão deveria trazer 30 dias: %d %v", len(p.Days), err)
		}
		if err := json.Unmarshal([]byte(toolResultOf(t, resps[2]).Content[0].Text), &p); err != nil || len(p.Days) != 5 {
			t.Fatalf("get_projection days=5: %d %v", len(p.Days), err)
		}
		var created domain.Recurring
		if err := json.Unmarshal([]byte(toolResultOf(t, resps[5]).Content[0].Text), &created); err != nil ||
			created.AccountName == nil || *created.AccountName != "Nubank" || created.CategoryName == nil || *created.CategoryName != "Streaming" || !created.Active {
			t.Fatalf("create_recurring: %+v %v", created, err)
		}
	})
}
