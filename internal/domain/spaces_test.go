package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestSpaceSlug(t *testing.T) {
	valid := []string{"casa", "empresa-2", "a1", "0-9", strings.Repeat("a", 31)}
	invalid := []string{"", "a", "-casa", "Casa", "casa.db", "../x", `..\x`, "casa/x", "casa x", "ação", strings.Repeat("a", 32)}
	for _, s := range valid {
		if !IsValidSpaceSlug(s) {
			t.Errorf("IsValidSpaceSlug(%q) = false", s)
		}
	}
	for _, s := range invalid {
		if IsValidSpaceSlug(s) {
			t.Errorf("IsValidSpaceSlug(%q) = true", s)
		}
	}
	cases := []struct{ name, want string }{
		{"Casa", "casa"},
		{"Casa da Praia", "casa-da-praia"},
		{"  Empresa — Ação & Cia.  ", "empresa-acao-cia"},
		{"../../etc", "etc"},
		{"!!!", "espaco"},
		{"x", "espaco"},
		{strings.Repeat("palavra ", 10), "palavra-palavra-palavra-palavra"},
	}
	for _, c := range cases {
		got := SlugifySpaceName(c.name)
		if got != c.want || !IsValidSpaceSlug(got) {
			t.Errorf("SlugifySpaceName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestValidateSpaces(t *testing.T) {
	bad := "x.y"
	color := "azul"
	if issues := ValidateCreateSpace(CreateSpaceInput{Name: " ", Kind: "FAMILY", Color: &color, Slug: &bad}); len(issues) != 4 {
		t.Fatalf("ValidateCreateSpace = %+v", issues)
	}
	if issues := ValidateCreateSpace(CreateSpaceInput{Name: "Casa", Kind: SpaceHome}); len(issues) != 0 {
		t.Fatalf("ValidateCreateSpace válido = %+v", issues)
	}
	if issues := ValidateUpdateSpace(UpdateSpaceInput{Name: Optional[string]{Set: true}, Color: Optional[string]{Set: true, Valid: true, Value: "#fff"}}); len(issues) != 2 {
		t.Fatalf("ValidateUpdateSpace = %+v", issues)
	}
	var total SpaceTotals
	total.Add(SpaceTotals{BalanceCents: 1, NetWorthCents: 2, MonthIncomeCents: 3, MonthOutCents: 4, MonthResultCents: -1})
	total.Add(SpaceTotals{BalanceCents: 10, NetWorthCents: 20, MonthIncomeCents: 30, MonthOutCents: 40, MonthResultCents: -10})
	if total != (SpaceTotals{11, 22, 33, 44, -11}) {
		t.Fatalf("SpaceTotals.Add = %+v", total)
	}
}

const (
	mA = "11111111-1111-4111-8111-111111111111"
	mB = "22222222-2222-4222-8222-222222222222"
	mC = "33333333-3333-4333-8333-333333333333"
)

func TestValidateSplits(t *testing.T) {
	cases := []struct {
		name   string
		splits []TransactionSplit
		ok     bool
	}{
		{"vazia = divisão igual", nil, true},
		{"50/50", []TransactionSplit{{mA, 50}, {mB, 50}}, true},
		{"centésimos", []TransactionSplit{{mA, 33.33}, {mB, 33.33}, {mC, 33.34}}, true},
		{"soma 99,99", []TransactionSplit{{mA, 33.33}, {mB, 33.33}, {mC, 33.33}}, false},
		{"três casas decimais", []TransactionSplit{{mA, 33.333}, {mB, 66.667}}, false},
		{"repetido", []TransactionSplit{{mA, 50}, {mA, 50}}, false},
		{"parte zero", []TransactionSplit{{mA, 100}, {mB, 0}}, false},
		{"id inválido", []TransactionSplit{{"x", 100}}, false},
	}
	for _, c := range cases {
		if got := len(ValidateSplits(c.splits)) == 0; got != c.ok {
			t.Errorf("%s: válido = %v, want %v", c.name, got, c.ok)
		}
	}
}

func TestExpenseShares(t *testing.T) {
	cases := []struct {
		name    string
		e       SharedExpense
		members []string
		want    map[string]int64
	}{
		{"igual com resto para quem pagou", SharedExpense{PaidBy: mB, AmountCents: 1000}, []string{mA, mB, mC}, map[string]int64{mA: 333, mB: 334, mC: 333}},
		{"igual exato", SharedExpense{PaidBy: mA, AmountCents: 900}, []string{mA, mB, mC}, map[string]int64{mA: 300, mB: 300, mC: 300}},
		{"60/40", SharedExpense{PaidBy: mA, AmountCents: 10001, Splits: []SplitShare{{mA, 6000}, {mB, 4000}}}, nil, map[string]int64{mA: 6001, mB: 4000}},
		{"quem pagou fora da divisão absorve o resto", SharedExpense{PaidBy: mC, AmountCents: 101, Splits: []SplitShare{{mA, 5000}, {mB, 5000}}}, nil, map[string]int64{mA: 50, mB: 50, mC: 1}},
		{"sem membros: tudo de quem pagou", SharedExpense{PaidBy: mA, AmountCents: 500}, nil, map[string]int64{mA: 500}},
	}
	for _, c := range cases {
		got := ExpenseShares(c.e, c.members)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
		var sum int64
		for _, v := range got {
			sum += v
		}
		if sum != c.e.AmountCents {
			t.Errorf("%s: soma %d ≠ %d", c.name, sum, c.e.AmountCents)
		}
	}
}

func TestMemberBalancesAndSettlements(t *testing.T) {
	members := []Member{{ID: mA, Name: "Ana"}, {ID: mB, Name: "Bruno"}, {ID: mC, Name: "Caio"}}
	expenses := []SharedExpense{
		{PaidBy: mA, AmountCents: 30000},                                              // 10.000 de cada
		{PaidBy: mB, AmountCents: 6000, Splits: []SplitShare{{mA, 5000}, {mB, 5000}}}, // 3.000 Ana, 3.000 Bruno
	}
	settled := []SettlementRecord{{FromMemberID: mC, ToMemberID: mA, AmountCents: 4000}}
	got := ComputeMemberBalances(members, expenses, settled)
	want := []MemberBalance{
		{MemberID: mA, Name: "Ana", PaidCents: 30000, OwedCents: 17000, BalanceCents: 13000},
		{MemberID: mB, Name: "Bruno", PaidCents: 6000, OwedCents: 13000, BalanceCents: -7000},
		{MemberID: mC, Name: "Caio", PaidCents: 4000, OwedCents: 10000, BalanceCents: -6000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ComputeMemberBalances =\n%+v\nwant\n%+v", got, want)
	}
	sugg := SuggestSettlements(got)
	wantSugg := []SettlementSuggestion{{mB, mA, 7000}, {mC, mA, 6000}}
	if !reflect.DeepEqual(sugg, wantSugg) {
		t.Fatalf("SuggestSettlements = %+v, want %+v", sugg, wantSugg)
	}

	cases := []struct {
		name     string
		balances []int64
		want     []SettlementSuggestion
	}{
		{"tudo zerado", []int64{0, 0, 0}, []SettlementSuggestion{}},
		{"um credor, dois devedores", []int64{100, -40, -60}, []SettlementSuggestion{{mC, mA, 60}, {mB, mA, 40}}},
		{"dois credores, um devedor", []int64{-100, 70, 30}, []SettlementSuggestion{{mA, mB, 70}, {mA, mC, 30}}},
		{"empate pela ordem", []int64{-50, -50, 100}, []SettlementSuggestion{{mA, mC, 50}, {mB, mC, 50}}},
	}
	for _, c := range cases {
		bs := make([]MemberBalance, len(c.balances))
		for i, v := range c.balances {
			bs[i] = MemberBalance{MemberID: members[i].ID, BalanceCents: v}
		}
		if got := SuggestSettlements(bs); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestValidateSettle(t *testing.T) {
	acc1, acc2 := mA, mB
	cases := []struct {
		name string
		in   SettleInput
		ok   bool
	}{
		{"registro simples", SettleInput{FromMemberID: mA, ToMemberID: mB, AmountCents: 100}, true},
		{"com transferência", SettleInput{FromMemberID: mA, ToMemberID: mB, AmountCents: 100, FromAccountID: &acc1, ToAccountID: &acc2}, true},
		{"mesmo membro", SettleInput{FromMemberID: mA, ToMemberID: mA, AmountCents: 100}, false},
		{"valor zero", SettleInput{FromMemberID: mA, ToMemberID: mB}, false},
		{"só uma conta", SettleInput{FromMemberID: mA, ToMemberID: mB, AmountCents: 100, FromAccountID: &acc1}, false},
		{"mesma conta", SettleInput{FromMemberID: mA, ToMemberID: mB, AmountCents: 100, FromAccountID: &acc1, ToAccountID: &acc1}, false},
	}
	for _, c := range cases {
		if got := len(ValidateSettle(c.in)) == 0; got != c.ok {
			t.Errorf("%s: válido = %v, want %v (%+v)", c.name, got, c.ok, ValidateSettle(c.in))
		}
	}
}

func TestDREGroups(t *testing.T) {
	cases := []struct {
		name string
		kind CategoryKind
		want *DREGroup
	}{
		{"Impostos sobre vendas", CategoryExpense, wealthPtr(DREDeductions)},
		{"Simples Nacional", CategoryExpense, wealthPtr(DREDeductions)},
		{"Fornecedores", CategoryExpense, wealthPtr(DRECosts)},
		{"Infraestrutura/cloud", CategoryExpense, wealthPtr(DRECosts)},
		{"Comissões", CategoryExpense, wealthPtr(DRECosts)},
		{"Juros", CategoryExpense, wealthPtr(DREFinancial)},
		{"Empréstimos", CategoryExpense, wealthPtr(DREFinancial)},
		{"Rendimentos", CategoryIncome, wealthPtr(DREFinancial)},
		{"Marketing", CategoryExpense, nil},
		{"Vendas", CategoryIncome, nil},
		{"Imposto", CategoryIncome, nil},
	}
	for _, c := range cases {
		got := GuessDREGroup(c.name, c.kind)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("GuessDREGroup(%q, %s) = %v, want %v", c.name, c.kind, got, c.want)
		}
	}

	root, child, own, orphan := "root", "child", "own", "orphan"
	nodes := map[string]DRENode{
		root:   {Name: "Custos", Kind: CategoryExpense},
		child:  {Name: "Hospedagem", Kind: CategoryExpense, ParentID: &root},
		own:    {Name: "Marketing", Kind: CategoryExpense, ParentID: &root, DREGroup: wealthPtr(DREOther)},
		orphan: {Name: "Diversos", Kind: CategoryExpense, ParentID: &child},
		"inc":  {Name: "Vendas", Kind: CategoryIncome},
		"opx":  {Name: "Pró-labore", Kind: CategoryExpense},
	}
	for id, want := range map[string]DREGroup{root: DRECosts, child: DRECosts, own: DREOther, orphan: DRECosts, "inc": DRERevenue, "opx": DREOpex, "missing": DREOpex} {
		if got := EffectiveDREGroup(id, nodes); got != want {
			t.Errorf("EffectiveDREGroup(%s) = %s, want %s", id, got, want)
		}
	}
}

func TestRecurringIncomeActiveOn(t *testing.T) {
	end := "2026-05-20"
	deact := "2026-06-10"
	cases := []struct {
		name string
		r    RecurringIncome
		date string
		want bool
	}{
		{"antes do início", RecurringIncome{StartDate: "2026-05-01", Active: true}, "2026-04-30", false},
		{"no início", RecurringIncome{StartDate: "2026-05-01", Active: true}, "2026-05-01", true},
		{"depois do fim", RecurringIncome{StartDate: "2026-01-01", EndDate: &end, Active: true}, "2026-05-31", false},
		{"desativada depois da data", RecurringIncome{StartDate: "2026-01-01", DeactivatedAt: &deact}, "2026-05-31", true},
		{"desativada antes da data", RecurringIncome{StartDate: "2026-01-01", DeactivatedAt: &deact}, "2026-06-30", false},
		{"desativada sem data (legado)", RecurringIncome{StartDate: "2026-01-01"}, "2026-02-01", false},
	}
	for _, c := range cases {
		if got := c.r.ActiveOn(c.date); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestComputeBusinessMetrics(t *testing.T) {
	deact := "2026-06-10"
	end := "2026-06-15"
	recs := []RecurringIncome{
		{AmountCents: 100000, StartDate: "2026-01-01", Active: true},               // ativa o mês todo
		{AmountCents: 50000, StartDate: "2026-06-05", Active: true},                // nova no mês
		{AmountCents: 30000, StartDate: "2026-02-01", DeactivatedAt: &deact},       // desativada no mês
		{AmountCents: 20000, StartDate: "2026-03-01", EndDate: &end, Active: true}, // encerrada no mês
		{AmountCents: 99999, StartDate: "2026-01-01"},                              // legado inativo
		{AmountCents: 10000, StartDate: "2026-07-01", Active: true},                // começa depois
	}
	prev := []MonthCashFlow{{InCents: 100000, OutCents: 160000}, {InCents: 100000, OutCents: 130000}, {InCents: 100000, OutCents: 100000}}
	got := ComputeBusinessMetrics(2026, 6, recs, prev, 300000)
	if got.MRRCents != 150000 || got.ARRCents != 1800000 || got.NewMRRCents != 50000 || got.ChurnedMRRCents != 50000 {
		t.Fatalf("MRR: %+v", got)
	}
	if got.ChurnPct == nil || *got.ChurnPct != 33.3 { // 50.000 ÷ 150.000 no início do mês
		t.Fatalf("churn: %v", got.ChurnPct)
	}
	if got.BurnCents != 30000 || got.RunwayMonths == nil || *got.RunwayMonths != 10 || got.CashCents != 300000 {
		t.Fatalf("queima/runway: %+v runway=%v", got, got.RunwayMonths)
	}

	positive := ComputeBusinessMetrics(2026, 6, nil, []MonthCashFlow{{InCents: 200, OutCents: 100}}, 1000)
	if positive.BurnCents != 0 || positive.RunwayMonths != nil || positive.ChurnPct != nil || positive.MRRCents != 0 {
		t.Fatalf("sem queima: %+v", positive)
	}
	negativeCash := ComputeBusinessMetrics(2026, 6, nil, []MonthCashFlow{{OutCents: 100}}, -500)
	if negativeCash.RunwayMonths == nil || *negativeCash.RunwayMonths != 0 {
		t.Fatalf("caixa negativo: %v", negativeCash.RunwayMonths)
	}
}

func TestBuildDRE(t *testing.T) {
	entries := []DREEntry{
		{Month: 1, Group: DRERevenue, AmountCents: 100000},
		{Month: 1, Group: DREDeductions, AmountCents: -6000},
		{Month: 1, Group: DRECosts, AmountCents: -14000},
		{Month: 1, Group: DREOpex, AmountCents: -50000},
		{Month: 1, Group: DREFinancial, AmountCents: -2000},
		{Month: 1, Group: DREFinancial, AmountCents: 500},
		{Month: 1, Group: DREOther, AmountCents: -500},
		{Month: 2, Group: DREOpex, AmountCents: -1000},
		{Month: 13, Group: DRERevenue, AmountCents: 999},
	}
	dre := BuildDRE(2026, entries)
	if len(dre.Months) != 12 || dre.Months[11].Month != 12 {
		t.Fatalf("meses: %d", len(dre.Months))
	}
	jan := dre.Months[0].DRELine
	want := DRELine{
		RevenueCents: 100000, DeductionsCents: 6000, NetRevenueCents: 94000, CostsCents: 14000, GrossProfitCents: 80000,
		OpexCents: 50000, OperatingResultCents: 30000, FinancialCents: -1500, OtherCents: -500, NetResultCents: 28000,
	}
	if jan.MarginPct == nil || *jan.MarginPct != 29.8 {
		t.Fatalf("margem jan: %v", jan.MarginPct)
	}
	jan.MarginPct = nil
	if jan != want {
		t.Fatalf("jan =\n%+v\nwant\n%+v", jan, want)
	}
	feb := dre.Months[1]
	if feb.NetResultCents != -1000 || feb.MarginPct != nil {
		t.Fatalf("fev: %+v", feb)
	}
	if dre.Totals.NetResultCents != 27000 || dre.Totals.RevenueCents != 100000 || dre.Totals.MarginPct == nil || *dre.Totals.MarginPct != 28.7 {
		t.Fatalf("totais: %+v margem=%v", dre.Totals, dre.Totals.MarginPct)
	}
	loss := BuildDRE(2026, []DREEntry{{Month: 3, Group: DRERevenue, AmountCents: 1000}, {Month: 3, Group: DREOpex, AmountCents: -1500}})
	if m := loss.Months[2].MarginPct; m == nil || *m != -50 {
		t.Fatalf("margem negativa: %v", m)
	}
}
