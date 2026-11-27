package domain

import (
	"strings"
	"testing"
)

func strp(s string) *string { return &s }

func TestRecurringOccurrenceDate(t *testing.T) {
	cases := []struct {
		day, year, month int
		want             string
	}{
		{5, 2026, 3, "2026-03-05"},
		{31, 2026, 2, "2026-02-28"},
		{31, 2028, 2, "2028-02-29"},
		{31, 2026, 4, "2026-04-30"},
		{30, 2026, 12, "2026-12-30"},
	}
	for _, tc := range cases {
		if got := RecurringOccurrenceDate(tc.day, tc.year, tc.month); got != tc.want {
			t.Errorf("RecurringOccurrenceDate(%d, %d, %d) = %q, want %q", tc.day, tc.year, tc.month, got, tc.want)
		}
	}
}

func TestNextRecurringDate(t *testing.T) {
	sched := RecurringSchedule{DayOfMonth: 10, StartDate: "2026-01-01"}
	cases := []struct {
		name     string
		sched    RecurringSchedule
		active   bool
		realized map[string]bool
		today    string
		want     *string
	}{
		{"futura no mês", sched, true, nil, "2026-11-05", strp("2026-11-10")},
		{"atrasada do mês corrente", sched, true, nil, "2026-11-20", strp("2026-11-10")},
		{"mês corrente já lançado", sched, true, map[string]bool{"2026-11": true}, "2026-11-20", strp("2026-12-10")},
		{"inativa", sched, false, nil, "2026-11-05", nil},
		{"início no futuro", RecurringSchedule{DayOfMonth: 10, StartDate: "2027-03-01"}, true, nil, "2026-11-05", strp("2027-03-10")},
		{"início depois do dia no mês corrente", RecurringSchedule{DayOfMonth: 10, StartDate: "2026-11-15"}, true, nil, "2026-11-16", strp("2026-12-10")},
		{"encerrada", RecurringSchedule{DayOfMonth: 10, StartDate: "2026-01-01", EndDate: strp("2026-10-31")}, true, nil, "2026-11-05", nil},
		{"último mês", RecurringSchedule{DayOfMonth: 10, StartDate: "2026-01-01", EndDate: strp("2026-11-10")}, true, nil, "2026-11-05", strp("2026-11-10")},
		{"dia 31 em fevereiro", RecurringSchedule{DayOfMonth: 31, StartDate: "2026-01-01"}, true, map[string]bool{"2026-01": true}, "2026-02-01", strp("2026-02-28")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NextRecurringDate(tc.sched, tc.active, tc.realized, tc.today)
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("NextRecurringDate = %v, want %v", deref(got), deref(tc.want))
			}
		})
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestNormalizeText(t *testing.T) {
	cases := map[string]string{
		"Padaria  São João":  "padaria sao joao",
		"  AÇAÍ   Ünico  ":   "acai unico",
		"Pão de Açúcar":      "pao de acucar",
		"":                   "",
		"IFOOD *Restaurante": "ifood *restaurante",
	}
	for in, want := range cases {
		if got := NormalizeText(in); got != want {
			t.Errorf("NormalizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchCategoryRule(t *testing.T) {
	rules := []RuleCandidate{
		{Pattern: "uber", CategoryID: "transporte", CategoryKind: CategoryExpense, Priority: 0},
		{Pattern: "uber eats", CategoryID: "delivery", CategoryKind: CategoryExpense, Priority: 0},
		{Pattern: "ifood", CategoryID: "delivery", CategoryKind: CategoryExpense, Priority: 0},
		{Pattern: "mercado", CategoryID: "supermercado", CategoryKind: CategoryExpense, Priority: 0},
		{Pattern: "mercado livre", CategoryID: "compras", CategoryKind: CategoryExpense, Priority: 0},
		{Pattern: "farmácia", CategoryID: "farmacia", CategoryKind: CategoryExpense, Priority: 0},
		{Pattern: "posto", CategoryID: "combustivel", CategoryKind: CategoryExpense, Priority: 5},
		{Pattern: "posto shell conveniencia", CategoryID: "lanches", CategoryKind: CategoryExpense, Priority: 1},
		{Pattern: "salario", CategoryID: "salario", CategoryKind: CategoryIncome, Priority: 0},
	}
	cases := []struct {
		desc string
		kind CategoryKind
		want string
	}{
		{"UBER *TRIP", CategoryExpense, "transporte"},
		{"Uber Eats - pedido", CategoryExpense, "delivery"},
		{"Mercado Livre *compra", CategoryExpense, "compras"},
		{"Supermercado Dia", CategoryExpense, "supermercado"},
		{"FARMACIA DROGA RAIA", CategoryExpense, "farmacia"},
		{"Posto Shell Conveniência", CategoryExpense, "combustivel"},
		{"Salário outubro", CategoryIncome, "salario"},
		{"Salário outubro", CategoryExpense, ""},
		{"uber", CategoryIncome, ""},
		{"Cinema", CategoryExpense, ""},
		{"", CategoryExpense, ""},
	}
	for _, tc := range cases {
		got := MatchCategoryRule(rules, tc.desc, tc.kind)
		if deref(got) != tc.want && !(got == nil && tc.want == "") {
			t.Errorf("MatchCategoryRule(%q, %s) = %s, want %q", tc.desc, tc.kind, deref(got), tc.want)
		}
	}
}

func TestGuessSpendType(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Aluguel", "FIXED"},
		{"Moradia", "FIXED"},
		{"Energia elétrica", "FIXED"},
		{"Assinaturas", "CUTTABLE"},
		{"Serviços digitais", "CUTTABLE"},
		{"Tarifas bancárias", "CUTTABLE"},
		{"Seguro saúde", "FIXED"},
		{"Mercado", "VARIABLE"},
		{"Saúde", "VARIABLE"},
		{"Lazer", "VARIABLE"},
		{"Gasolina", "VARIABLE"},
		{"Educação", "FIXED"},
		{"Coisas aleatórias", ""},
	}
	for _, tc := range cases {
		got := GuessSpendType(tc.name)
		gotS := ""
		if got != nil {
			gotS = string(*got)
		}
		if gotS != tc.want {
			t.Errorf("GuessSpendType(%q) = %q, want %q", tc.name, gotS, tc.want)
		}
	}
}

func TestEffectiveSpendType(t *testing.T) {
	fixed, cut := SpendFixed, SpendCuttable
	nodes := map[string]CategoryNode{
		"moradia": {Kind: CategoryExpense, SpendType: &fixed},
		"aluguel": {Kind: CategoryExpense, ParentID: strp("moradia")},
		"neto":    {Kind: CategoryExpense, ParentID: strp("aluguel")},
		"tv":      {Kind: CategoryExpense, ParentID: strp("moradia"), SpendType: &cut},
		"solta":   {Kind: CategoryExpense},
		"salario": {Kind: CategoryIncome, SpendType: &fixed},
		"ciclo-a": {Kind: CategoryExpense, ParentID: strp("ciclo-b")},
		"ciclo-b": {Kind: CategoryExpense, ParentID: strp("ciclo-a")},
	}
	cases := map[string]string{
		"moradia": "FIXED",
		"aluguel": "FIXED",
		"neto":    "FIXED",
		"tv":      "CUTTABLE",
		"solta":   "VARIABLE",
		"salario": "",
		"ciclo-a": "VARIABLE",
		"nao-tem": "",
	}
	for id, want := range cases {
		got := EffectiveSpendType(id, nodes)
		gotS := ""
		if got != nil {
			gotS = string(*got)
		}
		if gotS != want {
			t.Errorf("EffectiveSpendType(%q) = %q, want %q", id, gotS, want)
		}
	}
}

func TestValidateSettings(t *testing.T) {
	base := Settings{ReserveMonths: 6, ReserveRefillRatePct: 10, BalanceBands: DefaultBalanceBands()}
	if issues := ValidateSettings(base); len(issues) != 0 {
		t.Fatalf("padrão deveria ser válido: %+v", issues)
	}
	bad := base
	zero := int64(0)
	bad.MonthlyLimitCents = &zero
	bad.ReserveMonths = 0
	bad.BalanceBands.Yellow = 100000
	bad.SpendPlan = &SpendPlan{FixedPct: 50, CuttablePct: 30, VariablePct: 30}
	issues := ValidateSettings(bad)
	var paths []string
	for _, is := range issues {
		paths = append(paths, is.Path)
	}
	joined := strings.Join(paths, ",")
	for _, want := range []string{"monthlyLimitCents", "reserveMonths", "balanceBands", "spendPlan"} {
		if !strings.Contains(joined, want) {
			t.Errorf("esperava issue em %s, veio %v", want, paths)
		}
	}
}

func TestValidateRecurringAndRules(t *testing.T) {
	if issues := ValidateCreateRecurring(CreateRecurringInput{Description: "Aluguel", Kind: CategoryExpense, AmountCents: 1, DayOfMonth: 31}); len(issues) != 0 {
		t.Fatalf("recorrência válida: %+v", issues)
	}
	issues := ValidateCreateRecurring(CreateRecurringInput{Kind: "X", DayOfMonth: 32, StartDate: strp("2026-13-01")})
	if len(issues) != 5 {
		t.Fatalf("esperava 5 issues (description, kind, amountCents, dayOfMonth, startDate), veio %+v", issues)
	}
	if issues := ValidateRecurringPeriod("2026-05-01", strp("2026-04-30")); len(issues) != 1 {
		t.Fatalf("fim antes do início deveria falhar")
	}
	if issues := ValidateCreateCategoryRule(CreateCategoryRuleInput{Pattern: "   ", CategoryID: "x"}); len(issues) != 2 {
		t.Fatalf("padrão vazio e categoryId inválido: %+v", issues)
	}
	if issues := ValidateCreateCategory(CreateCategoryInput{Name: "Salário", Kind: CategoryIncome, Icon: "i", Color: "#ffffff", SpendType: func() *SpendType { s := SpendFixed; return &s }()}); len(issues) != 1 || issues[0].Path != "spendType" {
		t.Fatalf("spendType em receita deveria falhar: %+v", issues)
	}
}
