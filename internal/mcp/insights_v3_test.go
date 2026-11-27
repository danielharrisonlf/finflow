package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/testutil"
)

func TestInsightsV3Tools(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		backend := seededBackend(t, database)
		resps := runLines(t, backend,
			callLine(1, "list_tags", `{}`),
			callLine(2, "get_subscriptions", `{}`),
			callLine(3, "get_alerts", `{}`),
			callLine(4, "can_afford", `{"amountCents":1000}`),
			callLine(5, "can_afford", `{"amountCents":120000,"installments":3,"card":"nubank","description":"TV"}`),
			callLine(6, "get_cashflow_basis", `{"months":2,"basis":"ACCRUAL"}`),
			callLine(7, "get_cashflow_basis", `{}`),
			// inválidas
			callLine(8, "can_afford", `{}`),
			callLine(9, "can_afford", `{"amountCents":100,"installments":49}`),
			callLine(10, "can_afford", `{"amountCents":100,"card":"Inexistente"}`),
			callLine(11, "get_cashflow_basis", `{"basis":"cash"}`),
			callLine(12, "get_cashflow_basis", `{"months":13}`),
		)
		if len(resps) != 12 {
			t.Fatalf("esperava 12 respostas, veio %d", len(resps))
		}
		wants := []string{`[`, `"monthlyTotalCents"`, `[`, `"verdict"`, `"verdict"`, `"basis": "ACCRUAL"`, `"basis": "CASH"`}
		for i, want := range wants {
			tr := toolResultOf(t, resps[i])
			if tr.IsError || !strings.Contains(tr.Content[0].Text, want) {
				t.Errorf("chamada %d: esperava sucesso contendo %q, veio isError=%v %.200q", i+1, want, tr.IsError, tr.Content[0].Text)
			}
		}
		var cf domain.CashflowView
		if err := json.Unmarshal([]byte(toolResultOf(t, resps[6]).Content[0].Text), &cf); err != nil || len(cf.Months) != 3 {
			t.Fatalf("get_cashflow_basis padrão deveria trazer 3 meses: %+v %v", cf, err)
		}
		errs := []string{"amountCents: obrigatório", "installments: deve ser <= 48", `cartão "Inexistente" não encontrado`, "basis: deve ser um de", "months: deve ser <= 12"}
		for i, want := range errs {
			tr := toolResultOf(t, resps[7+i])
			if !tr.IsError || !strings.Contains(tr.Content[0].Text, want) {
				t.Errorf("chamada %d: esperava erro com %q, veio isError=%v %q", 8+i, want, tr.IsError, tr.Content[0].Text)
			}
		}
	})
}
