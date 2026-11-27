package mcp

import (
	"context"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestSpacesTools(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		st := store.New(database)
		if err := store.BootstrapKind(ctx, st, domain.SpaceBusiness); err != nil {
			t.Fatal(err)
		}
		if _, err := store.SeedBusiness(ctx, st); err != nil {
			t.Fatal(err)
		}
		backend := seededBackend(t, database) // Seed pessoal é no-op: já há contas
		if _, err := backend.CreateMember(ctx, domain.CreateMemberInput{Name: "Sócia"}); err != nil {
			t.Fatal(err)
		}
		resps := runLines(t, backend,
			callLine(1, "list_members", `{}`),
			callLine(2, "get_member_balances", `{}`),
			callLine(3, "get_business_metrics", `{}`),
			callLine(4, "get_business_dre", `{"year":2026}`),
			callLine(5, "get_business_metrics", `{"month":13}`),
		)
		if len(resps) != 5 {
			t.Fatalf("esperava 5 respostas, veio %d", len(resps))
		}
		wants := []string{`"Sócia"`, `"settlements"`, `"mrrCents"`, `"netResultCents"`}
		for i, want := range wants {
			tr := toolResultOf(t, resps[i])
			if tr.IsError || !strings.Contains(tr.Content[0].Text, want) {
				t.Errorf("chamada %d: esperava sucesso contendo %q, veio isError=%v %.200q", i+1, want, tr.IsError, tr.Content[0].Text)
			}
		}
		if tr := toolResultOf(t, resps[4]); !tr.IsError || !strings.Contains(tr.Content[0].Text, "month") {
			t.Errorf("validação do mês: %v %q", tr.IsError, tr.Content[0].Text)
		}
	})
}
