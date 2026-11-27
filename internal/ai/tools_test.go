package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestToolSpecsDeterministicAndStrictCompatible(t *testing.T) {
	a, _ := json.Marshal(toolParams())
	b, _ := json.Marshal(toolParams())
	if string(a) != string(b) {
		t.Fatal("lista de ferramentas deve ser byte a byte idêntica entre chamadas (cache)")
	}
	for _, unsupported := range []string{`"minimum"`, `"maximum"`, `"minLength"`, `"maxLength"`, `"pattern"`} {
		if strings.Contains(string(a), unsupported) {
			t.Fatalf("schema com %s não é compatível com strict", unsupported)
		}
	}
	for _, spec := range toolSpecs() {
		for _, req := range spec.required {
			if _, ok := spec.props[req]; !ok {
				t.Fatalf("%s: obrigatório %s sem propriedade", spec.name, req)
			}
		}
		if spec.label == "" {
			t.Fatalf("%s sem rótulo", spec.name)
		}
	}
}

func TestValidateToolInput(t *testing.T) {
	spec, _ := findToolSpec("can_afford")
	cases := map[string]string{
		`{"amountCents": 100}`: "",
		`{}`:                   "amountCents: obrigatório",
		`{"amountCents": 0}`:   "deve estar entre",
		`{"amountCents": 1.5}`: "inteiro",
		`{"amountCents": 100, "installments": 49}`:   "installments",
		`{"amountCents": 100, "date": "27/11/2026"}`: "YYYY-MM-DD",
		`{"amountCents": 100, "extra": true}`:        "campo desconhecido",
		`{"amountCents": 10`:                         "objeto JSON",
		`{"amountCents": 100, "description": ""}`:    "caracteres",
	}
	for input, want := range cases {
		got := validateToolInput(spec, json.RawMessage(input))
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("%s: got %q want %q", input, got, want)
		}
	}
}

func TestReadToolsAndProposals(t *testing.T) {
	ctx := context.Background()
	svc, svcs := newTestService(t, nil, false)
	acc, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta", Type: domain.AccountChecking, InitialBalanceCents: 100000})
	if err != nil {
		t.Fatal(err)
	}
	goal, err := svcs.CreateGoal(ctx, domain.CreateGoalInput{Name: "Viagem", TargetCents: 300000, Icon: "plane"})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}

	for _, name := range []string{"get_summary", "get_insights", "get_projection", "get_cashflow", "get_category_spending",
		"list_budgets", "list_goals", "list_accounts", "list_cards", "list_categories", "search_transactions", "list_recurring",
		"get_cashflow_basis", "get_subscriptions", "list_investments", "list_loans"} {
		if out, isErr := svc.runTool(ctx, name, json.RawMessage(`{}`), rec.emit); isErr {
			t.Errorf("%s: %s", name, out)
		}
	}
	// can_afford: 1.000,00 de saldo, compra de 2.000,00 à vista → NO.
	out, isErr := svc.runTool(ctx, "can_afford", json.RawMessage(`{"amountCents": 200000}`), rec.emit)
	if isErr || !strings.Contains(out, `"verdict":"NO"`) {
		t.Fatalf("can_afford: %s", out)
	}
	out, isErr = svc.runTool(ctx, "can_afford", json.RawMessage(`{"amountCents": 1000, "installments": 2, "description": "Livro"}`), rec.emit)
	if isErr || !strings.Contains(out, `"verdict"`) || !strings.Contains(out, `"message"`) {
		t.Fatalf("can_afford parcelado: %s", out)
	}

	if out, isErr := svc.runTool(ctx, "propose_recurring", json.RawMessage(`{"kind":"EXPENSE","description":"Aluguel","amountCents":150000,"dayOfMonth":5,"accountId":"`+acc.ID+`"}`), rec.emit); isErr || out != proposalSentMessage {
		t.Fatalf("propose_recurring: %s", out)
	}
	if out, isErr := svc.runTool(ctx, "propose_goal_contribution", json.RawMessage(`{"goalId":"`+goal.ID+`","amountCents":-5000}`), rec.emit); isErr || out != proposalSentMessage {
		t.Fatalf("propose_goal_contribution: %s", out)
	}
	if out, isErr := svc.runTool(ctx, "propose_wish", json.RawMessage(`{"name":"Celular","priceCents":500000,"url":"https://loja.example/cel"}`), rec.emit); isErr || out != proposalSentMessage {
		t.Fatalf("propose_wish: %s", out)
	}
	if out, isErr := svc.runTool(ctx, "propose_wish", json.RawMessage(`{"name":"X","priceCents":100,"url":"javascript:alert(1)"}`), rec.emit); !isErr {
		t.Fatalf("url não http deveria falhar: %s", out)
	}
	if out, isErr := svc.runTool(ctx, "propose_transaction", json.RawMessage(`{"type":"INCOME","amountCents":100,"description":"x","accountId":"11111111-1111-4111-8111-111111111111"}`), rec.emit); !isErr {
		t.Fatalf("conta inexistente deveria falhar: %s", out)
	}

	props := rec.named("proposal")
	if len(props) != 3 {
		t.Fatalf("propostas: %+v", props)
	}
	kinds := []string{ProposalCreateRecurring, ProposalContributeGoal, ProposalAddWish}
	for i, p := range props {
		if p.data["kind"] != kinds[i] {
			t.Fatalf("proposta %d: %+v", i, p.data)
		}
	}
	contrib := props[1].data
	if contrib["payload"].(map[string]any)["goalId"] != goal.ID || !strings.Contains(contrib["summary"].(string), "Resgate de R$ 50,00") {
		t.Fatalf("aporte: %+v", contrib)
	}
}

func TestForSpaceUsesSpaceDataAndPersonalKey(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t, nil, true)
	other := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	if _, err := other.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta da casa", Type: domain.AccountChecking}); err != nil {
		t.Fatal(err)
	}
	spaced := svc.ForSpace(other)
	out, isErr := spaced.runTool(ctx, "list_accounts", json.RawMessage(`{}`), (&recorder{}).emit)
	if isErr || !strings.Contains(out, "Conta da casa") {
		t.Fatalf("ferramenta no espaço: %s", out)
	}
	if out, _ := svc.runTool(ctx, "list_accounts", json.RawMessage(`{}`), (&recorder{}).emit); strings.Contains(out, "Conta da casa") {
		t.Fatalf("o serviço original não deveria mudar: %s", out)
	}
	st, err := spaced.Status(ctx)
	if err != nil || !st.Configured {
		t.Fatalf("a chave continua a do espaço pessoal: %+v %v", st, err)
	}
}
