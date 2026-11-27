package domain

import (
	"strings"
	"testing"
)

func TestComputePace(t *testing.T) {
	cases := []struct {
		name          string
		current, prev int64
		want          string
	}{
		{"ambos zero", 0, 0, "SAME"},
		{"dentro de 5%", 10500, 10000, "SAME"},
		{"limite exato de 5% abaixo", 9500, 10000, "SAME"},
		{"acima", 10501, 10000, "ABOVE"},
		{"abaixo", 9499, 10000, "BELOW"},
		{"sem histórico com gasto", 100, 0, "ABOVE"},
		{"sem gasto com histórico", 0, 100, "BELOW"},
	}
	for _, tc := range cases {
		got := ComputePace(tc.current, tc.prev, 12)
		if got.Status != tc.want || got.DiffCents != tc.current-tc.prev || got.DayOfMonth != 12 {
			t.Errorf("%s: ComputePace = %+v, want status %s", tc.name, got, tc.want)
		}
	}
}

func TestComputeMonthlyLimit(t *testing.T) {
	if v := ComputeMonthlyLimit(nil, 5000, 10); v.LimitCents != nil || v.RemainingCents != nil || v.DailyAllowanceCents != nil || v.Percent != nil || v.SpentCents != 5000 {
		t.Fatalf("sem limite: %+v", v)
	}
	limit := int64(300000)
	v := ComputeMonthlyLimit(&limit, 120000, 10)
	if *v.RemainingCents != 180000 || *v.DailyAllowanceCents != 18000 || *v.Percent != 40 {
		t.Fatalf("mês corrente: %+v", v)
	}
	v = ComputeMonthlyLimit(&limit, 350000, 5)
	if *v.RemainingCents != -50000 || *v.DailyAllowanceCents != 0 || *v.Percent != 116.7 {
		t.Fatalf("estourado: rem=%d daily=%d pct=%v", *v.RemainingCents, *v.DailyAllowanceCents, *v.Percent)
	}
	if v := ComputeMonthlyLimit(&limit, 1000, 0); v.DailyAllowanceCents != nil {
		t.Fatalf("mês passado não tem diária: %+v", v)
	}
}

func TestPerformanceVerdict(t *testing.T) {
	cases := []struct {
		name string
		f    MonthFlows
		want string
	}{
		{"guardou e sobrou", MonthFlows{IncomeCents: 1000000, FixedCents: 400000, VariableCents: 200000, SavedCents: 100000}, "SAVING"},
		{"guardou exatamente o que sobrou", MonthFlows{IncomeCents: 100000, FixedCents: 60000, SavedCents: 40000}, "SAVING"},
		{"guardou mais do que sobrou", MonthFlows{IncomeCents: 100000, FixedCents: 60000, SavedCents: 60000}, "NEGATIVE_SAVED"},
		{"guardou com performance zero", MonthFlows{IncomeCents: 100, FixedCents: 100, SavedCents: 1}, "NEGATIVE_SAVED"},
		{"gastos acima das entradas mesmo guardando", MonthFlows{IncomeCents: 100000, FixedCents: 150000, SavedCents: 60000}, "NEGATIVE"},
		{"negativo sem guardar", MonthFlows{IncomeCents: 100000, FixedCents: 150000}, "NEGATIVE"},
		{"apertado", MonthFlows{IncomeCents: 100000, FixedCents: 96000}, "TIGHT"},
		{"positivo", MonthFlows{IncomeCents: 100000, FixedCents: 90000}, "POSITIVE"},
		{"exatamente 5% é positivo", MonthFlows{IncomeCents: 100000, FixedCents: 95000}, "POSITIVE"},
	}
	for _, tc := range cases {
		if got := PerformanceVerdict(tc.f); got != tc.want {
			t.Errorf("%s: verdict = %s, want %s", tc.name, got, tc.want)
		}
	}
	pv := ComputePerformance(MonthFlows{IncomeCents: 1000000, FixedCents: 400000, VariableCents: 200000, SavedCents: 150000}, -5000)
	if pv.TotalOutCents != 600000 || pv.PerformanceCents != 400000 || pv.SavingsRatePct == nil || *pv.SavingsRatePct != 15 || pv.PreviousPerformanceCents != -5000 {
		t.Fatalf("ComputePerformance = %+v", pv)
	}
	if !strings.Contains(pv.Message, "R$ 1.500,00") {
		t.Fatalf("mensagem sem valor guardado: %q", pv.Message)
	}
	if ComputePerformance(MonthFlows{}, 0).SavingsRatePct != nil {
		t.Fatal("sem entradas a taxa de economia deveria ser nil")
	}
}

func TestGroupBySpendType(t *testing.T) {
	items := []CategorySpend{
		{CategoryID: strp("aluguel"), Name: "Aluguel", SpendType: SpendFixed, TotalCents: 200000},
		{CategoryID: strp("streaming"), Name: "Streaming", SpendType: SpendCuttable, TotalCents: 5000},
		{CategoryID: strp("mercado"), Name: "Mercado", SpendType: SpendVariable, TotalCents: 80000},
		{Name: "Sem categoria", SpendType: SpendVariable, TotalCents: 115000},
	}
	groups := GroupBySpendType(items)
	if len(groups) != 3 || groups[0].SpendType != SpendFixed || groups[1].SpendType != SpendCuttable || groups[2].SpendType != SpendVariable {
		t.Fatalf("ordem dos grupos: %+v", groups)
	}
	if groups[0].Percent != 50 || groups[2].TotalCents != 195000 || groups[2].Categories[0].Name != "Sem categoria" {
		t.Fatalf("grupos: %+v", groups)
	}
	empty := GroupBySpendType(nil)
	if len(empty) != 3 || empty[0].Categories == nil || empty[0].Percent != 0 {
		t.Fatalf("sem gastos: %+v", empty)
	}
	fixed, variable := SplitFlows(items)
	if fixed != 205000 || variable != 195000 {
		t.Fatalf("SplitFlows = %d, %d", fixed, variable)
	}
}

func TestReserve(t *testing.T) {
	if got := AverageRecentCost([]int64{0, 300000, 0, 200000, 100000, 999999}); got != 200000 {
		t.Fatalf("AverageRecentCost = %d (pula meses sem gasto e usa só 3)", got)
	}
	if got := AverageRecentCost([]int64{100, 101}); got != 101 {
		t.Fatalf("média arredondada = %d", got)
	}
	if AverageRecentCost(nil) != 0 {
		t.Fatal("sem gastos a média é zero")
	}
	settings := Settings{ReserveMonths: 6}
	v := ComputeReserve(nil, 200000, settings)
	if v.Status != "NONE" || v.SuggestedTargetCents != 1200000 || v.TargetCents != 1200000 || v.GoalID != nil {
		t.Fatalf("sem meta: %+v", v)
	}
	settings.HasDependents = true
	v = ComputeReserve(&ReserveGoalInfo{ID: "g", TargetCents: 3000000, SavedCents: 500000}, 200000, settings)
	if v.Status != "BUILDING" || v.SuggestedTargetCents != 2400000 || v.TargetCents != 3000000 || v.MonthsCovered != 2.5 {
		t.Fatalf("construindo: %+v", v)
	}
	v = ComputeReserve(&ReserveGoalInfo{ID: "g", TargetCents: 100, SavedCents: 100}, 0, settings)
	if v.Status != "PROTECTED" || v.MonthsCovered != 0 {
		t.Fatalf("protegida: %+v", v)
	}
}

func TestComputeCutSuggestions(t *testing.T) {
	cs := func(id string, st SpendType, total int64) CategorySpend {
		return CategorySpend{CategoryID: strp(id), Name: id, SpendType: st, TotalCents: total}
	}
	current := []CategorySpend{
		cs("streaming", SpendCuttable, 5000),
		cs("aluguel", SpendFixed, 200000),
		cs("restaurantes", SpendVariable, 60000),
		cs("mercado", SpendVariable, 90000),
		cs("delivery", SpendVariable, 20000),
		cs("farmacia", SpendVariable, 10000),
		cs("pet", SpendVariable, 8000),
	}
	history := [][]CategorySpend{
		{cs("restaurantes", SpendVariable, 30000), cs("mercado", SpendVariable, 90000)},
		{cs("restaurantes", SpendVariable, 30000), cs("mercado", SpendVariable, 90000)},
		{cs("restaurantes", SpendVariable, 30000), cs("mercado", SpendVariable, 90000)},
	}
	got := ComputeCutSuggestions(current, history)
	var summary []string
	for _, s := range got {
		summary = append(summary, s.Kind+":"+s.Name)
	}
	want := "CANCEL:streaming,ABOVE_AVERAGE:restaurantes,REDUCE:mercado,REDUCE:delivery,REDUCE:farmacia"
	if strings.Join(summary, ",") != want {
		t.Fatalf("sugestões = %v, want %s", summary, want)
	}
	if got[1].AverageCents != 30000 || got[1].CurrentCents != 60000 {
		t.Fatalf("média de restaurantes: %+v", got[1])
	}
}
