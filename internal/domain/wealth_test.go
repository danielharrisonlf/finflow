package domain

import (
	"reflect"
	"testing"
)

func wealthPtr[T any](v T) *T { return &v }

func TestRatePctToBP(t *testing.T) {
	cases := []struct {
		pct  float64
		want int
	}{
		{1.99, 199}, {0, 0}, {1.49, 149}, {0.5, 50}, {12.345, 1235}, {100, 10000},
	}
	for _, c := range cases {
		if got := RatePctToBP(c.pct); got != c.want {
			t.Errorf("RatePctToBP(%v) = %d, want %d", c.pct, got, c.want)
		}
		if back := RateBPToPct(RatePctToBP(c.pct)); RatePctToBP(back) != c.want {
			t.Errorf("ida e volta de %v: %v", c.pct, back)
		}
	}
}

func TestPriceInstallment(t *testing.T) {
	cases := []struct {
		name      string
		principal int64
		bp, n     int
		want      int64
	}{
		{"R$ 1.000 a 2% em 5x", 100000, 200, 5, 21216},
		{"R$ 10.000 a 1% em 12x (tabela clássica)", 1000000, 100, 12, 88849},
		{"R$ 48.000 a 1,49% em 48x", 4800000, 149, 48, 140699},
		{"sem juros arredonda para cima", 100000, 0, 3, 33334},
		{"uma parcela = principal + juros", 100000, 250, 1, 102500},
		{"prazo inválido", 100000, 100, 0, 0},
	}
	for _, c := range cases {
		if got := PriceInstallment(c.principal, c.bp, c.n); got != c.want {
			t.Errorf("%s: parcela = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestPriceSchedule(t *testing.T) {
	cases := []struct {
		name          string
		principal     int64
		bp, n         int
		want          []AmortizationRow // primeiras linhas conferidas
		lastPayment   int64
		totalInterest int64
	}{
		{
			name: "R$ 1.000 a 2% em 5x", principal: 100000, bp: 200, n: 5,
			want: []AmortizationRow{
				{1, 21216, 2000, 19216, 80784},
				{2, 21216, 1616, 19600, 61184},
				{3, 21216, 1224, 19992, 41192},
				{4, 21216, 824, 20392, 20800},
				{5, 21216, 416, 20800, 0},
			},
			lastPayment: 21216, totalInterest: 6080,
		},
		{
			// Tabela clássica: parcela 888,49 e juros totais 661,86; a última parcela absorve o resíduo.
			name: "R$ 10.000 a 1% em 12x", principal: 1000000, bp: 100, n: 12,
			want: []AmortizationRow{
				{1, 88849, 10000, 78849, 921151},
				{2, 88849, 9212, 79637, 841514},
				{3, 88849, 8415, 80434, 761080},
			},
			lastPayment: 88847, totalInterest: 66186,
		},
		{
			name: "R$ 48.000 a 1,49% em 48x", principal: 4800000, bp: 149, n: 48,
			want: []AmortizationRow{
				{1, 140699, 71520, 69179, 4730821},
				{2, 140699, 70489, 70210, 4660611},
			},
			lastPayment: 140714, totalInterest: 1953567,
		},
		{
			name: "sem juros: última parcela absorve o resto", principal: 100000, bp: 0, n: 3,
			want: []AmortizationRow{
				{1, 33334, 0, 33334, 66666},
				{2, 33334, 0, 33334, 33332},
				{3, 33332, 0, 33332, 0},
			},
			lastPayment: 33332, totalInterest: 0,
		},
	}
	for _, c := range cases {
		rows := PriceSchedule(c.principal, c.bp, c.n)
		if len(rows) != c.n {
			t.Fatalf("%s: %d linhas, want %d", c.name, len(rows), c.n)
		}
		if !reflect.DeepEqual(rows[:len(c.want)], c.want) {
			t.Errorf("%s: linhas = %+v, want %+v", c.name, rows[:len(c.want)], c.want)
		}
		last := rows[len(rows)-1]
		var interest, amort int64
		for _, r := range rows {
			interest += r.InterestCents
			amort += r.AmortizationCents
			if r.PaymentCents != r.InterestCents+r.AmortizationCents {
				t.Errorf("%s: parcela %d não fecha: %+v", c.name, r.Number, r)
			}
		}
		if last.BalanceCents != 0 || last.PaymentCents != c.lastPayment {
			t.Errorf("%s: última linha = %+v, want pagamento %d e saldo 0", c.name, last, c.lastPayment)
		}
		if amort != c.principal || interest != c.totalInterest {
			t.Errorf("%s: amortizado %d (want %d), juros %d (want %d)", c.name, amort, c.principal, interest, c.totalInterest)
		}
	}
}

func TestLoanOutstanding(t *testing.T) {
	cases := []struct {
		name        string
		installment int64
		bp, rem     int
		want        int64
	}{
		{"5 parcelas de 212,16 a 2% ≈ principal", 21216, 200, 5, 100001},
		{"3 restantes = saldo da tabela após 2 pagamentos", 21216, 200, 3, 61184},
		{"12 de 888,49 a 1%", 88849, 100, 12, 1000002},
		{"sem taxa = soma das parcelas", 50000, 0, 4, 200000},
		{"quitado", 50000, 150, 0, 0},
	}
	for _, c := range cases {
		if got := LoanOutstandingCents(c.installment, c.bp, c.rem); got != c.want {
			t.Errorf("%s: saldo = %d, want %d", c.name, got, c.want)
		}
	}
	// O saldo devedor pelo valor presente acompanha a tabela Price (diferença só de arredondamento).
	rows := PriceSchedule(1000000, 100, 12)
	for k, r := range rows[:len(rows)-1] {
		pv := LoanOutstandingCents(88849, 100, 12-(k+1))
		if d := pv - r.BalanceCents; d < -5 || d > 5 {
			t.Errorf("após %d pagamentos: VP %d vs. tabela %d", k+1, pv, r.BalanceCents)
		}
	}
}

func TestLoanDueDates(t *testing.T) {
	if got := LoanDueDate("2026-01-31", 2); got != "2026-02-28" {
		t.Errorf("fevereiro limita ao último dia: %s", got)
	}
	if got := LoanDueDate("2026-01-31", 3); got != "2026-03-31" {
		t.Errorf("março volta ao dia 31 (calculado do 1º vencimento): %s", got)
	}
	cases := []struct {
		name        string
		paid, total int
		active      bool
		want        *string
	}{
		{"nenhuma paga", 0, 12, true, wealthPtr("2026-01-31")},
		{"três pagas", 3, 12, true, wealthPtr("2026-04-30")},
		{"quitado", 12, 12, true, nil},
		{"inativo", 3, 12, false, nil},
	}
	for _, c := range cases {
		got := LoanNextDueDate("2026-01-31", c.paid, c.total, c.active)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: próximo vencimento = %v, want %v", c.name, got, c.want)
		}
	}
	l := LoanSchedule{InstallmentCents: 1000, TotalInstallments: 4, PaidInstallments: 1, FirstDueDate: "2026-11-10", Active: true}
	if got := l.UnpaidDueDates(); !reflect.DeepEqual(got, []string{"2026-12-10", "2027-01-10", "2027-02-10"}) {
		t.Errorf("vencimentos em aberto = %v", got)
	}
	if l.InstallmentIn(2027, 1) != 3 || l.InstallmentIn(2026, 10) != 0 || l.InstallmentIn(2027, 3) != 0 {
		t.Errorf("InstallmentIn errado")
	}
	if l.OutstandingAt("2026-10-09", 0) != 0 || l.OutstandingAt("2026-10-10", 0) != 4000 || l.OutstandingAt("2027-03-01", 4) != 0 {
		t.Errorf("OutstandingAt errado")
	}
}

func TestSimulateDebtStrategies(t *testing.T) {
	// Cartão caro e pequeno x financiamento barato e grande.
	card := DebtInput{ID: "card", BalanceCents: LoanOutstandingCents(50000, 1200, 6), RateBP: 1200, InstallmentCents: 50000, Remaining: 6, NextDueDate: "2026-12-05"}
	car := DebtInput{ID: "car", BalanceCents: LoanOutstandingCents(100000, 150, 24), RateBP: 150, InstallmentCents: 100000, Remaining: 24, NextDueDate: "2026-12-20"}
	personal := DebtInput{ID: "personal", BalanceCents: LoanOutstandingCents(20000, 300, 3), RateBP: 300, InstallmentCents: 20000, Remaining: 3, NextDueDate: "2026-12-10"}
	debts := []DebtInput{car, card, personal}

	minOnly := SimulateDebtStrategies(debts, 0).MinimumOnly
	if minOnly.Months != 24 {
		t.Fatalf("só parcelas deveria levar o prazo da maior (24): %+v", minOnly)
	}
	if !reflect.DeepEqual(minOnly.Order, []string{"personal", "card", "car"}) {
		t.Errorf("ordem de quitação mínima = %v", minOnly.Order)
	}
	if minOnly.PayoffDate != "2028-11-20" {
		t.Errorf("quitação final = %s", minOnly.PayoffDate)
	}
	// Juros só com parcelas = parcelas − saldo (±1 centavo de arredondamento por dívida).
	expected := (50000*6 - card.BalanceCents) + (100000*24 - car.BalanceCents) + (20000*3 - personal.BalanceCents)
	if d := minOnly.TotalInterestCents - expected; d < -3 || d > 3 {
		t.Errorf("juros só parcelas = %d, want ≈ %d", minOnly.TotalInterestCents, expected)
	}

	s := SimulateDebtStrategies(debts, 30000)
	if !reflect.DeepEqual(s.Avalanche.Order, []string{"card", "personal", "car"}) {
		t.Errorf("avalanche deve priorizar maior taxa: %v", s.Avalanche.Order)
	}
	if !reflect.DeepEqual(s.Snowball.Order, []string{"personal", "card", "car"}) {
		t.Errorf("bola de neve deve priorizar menor saldo: %v", s.Snowball.Order)
	}
	for name, p := range map[string]DebtPlan{"avalanche": s.Avalanche, "snowball": s.Snowball} {
		if p.Months >= minOnly.Months || p.TotalInterestCents >= minOnly.TotalInterestCents {
			t.Errorf("%s com extra deveria quitar antes e pagar menos juros: %+v vs %+v", name, p, minOnly)
		}
		if len(p.PerLoan) != 3 {
			t.Errorf("%s: perLoan = %+v", name, p.PerLoan)
		}
	}
	if s.Avalanche.TotalInterestCents > s.Snowball.TotalInterestCents {
		t.Errorf("avalanche nunca paga mais juros que bola de neve: %d > %d", s.Avalanche.TotalInterestCents, s.Snowball.TotalInterestCents)
	}

	empty := SimulateDebtStrategies(nil, 1000)
	if empty.Avalanche.Months != 0 || len(empty.Avalanche.Order) != 0 || empty.Avalanche.PerLoan == nil {
		t.Errorf("sem dívidas: %+v", empty.Avalanche)
	}
}

func TestComposeNetWorth(t *testing.T) {
	xp, other := "xp", "orphan"
	accounts := []NetWorthAccount{
		{ID: "nu", Type: AccountChecking, BalanceCents: 100000},
		{ID: "cx", Type: AccountSavings, BalanceCents: 50000},
		{ID: xp, Type: AccountInvestment, BalanceCents: 999999}, // substituída pelas posições
		{ID: "corretora", Type: AccountInvestment, BalanceCents: 70000},
	}
	positions := []NetWorthPosition{
		{AccountID: &xp, ValueCents: 300000},
		{AccountID: &xp, ValueCents: 200000},
		{AccountID: nil, ValueCents: 10000},
		{AccountID: &other, ValueCents: 5000}, // conta fora da lista (arquivada) soma direto
	}
	got := ComposeNetWorth(accounts, positions, 40000)
	want := NetWorthBreakdown{CashCents: 150000, InvestmentsCents: 300000 + 200000 + 10000 + 5000 + 70000, DebtsCents: 40000}
	if got != want {
		t.Fatalf("patrimônio = %+v, want %+v", got, want)
	}
	if got.NetWorthCents() != 150000+585000-40000 {
		t.Errorf("líquido = %d", got.NetWorthCents())
	}
}

func TestSummarizeInvestments(t *testing.T) {
	inst := "Corretora"
	items := []Investment{
		{ID: "a", Kind: InvestmentTreasury, Institution: &inst, InvestedCents: 100000, CurrentCents: 110000},
		{ID: "b", Kind: InvestmentFII, InvestedCents: 50000, CurrentCents: 40000},
		{ID: "c", Kind: InvestmentTreasury, Institution: &inst, InvestedCents: 0, CurrentCents: 0},
	}
	snaps := map[string][]InvestmentSnapshot{
		"a": {{"2026-09-30", 100000, 100000}, {"2026-10-15", 105000, 100000}, {"2026-11-20", 110000, 100000}},
		"b": {{"2026-11-01", 40000, 50000}},
	}
	s := SummarizeInvestments(items, snaps, LastMonths(2026, 11, 3))
	if s.TotalInvestedCents != 150000 || s.TotalCurrentCents != 150000 || s.ReturnCents != 0 || s.ReturnPct == nil || *s.ReturnPct != 0 {
		t.Fatalf("totais: %+v", s)
	}
	if len(s.ByKind) != 2 || s.ByKind[0].Kind != InvestmentTreasury || s.ByKind[0].Percent != 73.3 || s.ByKind[1].Percent != 26.7 {
		t.Errorf("por tipo: %+v", s.ByKind)
	}
	if len(s.ByInstitution) != 2 || s.ByInstitution[0].Institution != inst || s.ByInstitution[1].Institution != NoInstitutionLabel {
		t.Errorf("por instituição: %+v", s.ByInstitution)
	}
	want := []InvestmentHistoryPoint{
		{Month: 9, Year: 2026, CurrentCents: 100000, InvestedCents: 100000},
		{Month: 10, Year: 2026, CurrentCents: 105000, InvestedCents: 100000},
		{Month: 11, Year: 2026, CurrentCents: 150000, InvestedCents: 150000},
	}
	if !reflect.DeepEqual(s.History, want) {
		t.Errorf("histórico = %+v, want %+v", s.History, want)
	}
	if p := ReturnPct(100000, 112345); p == nil || *p != 12.35 {
		t.Errorf("ReturnPct = %v", p)
	}
	if ReturnPct(0, 10) != nil {
		t.Errorf("sem investido, rentabilidade nula")
	}
}

func TestProjectionWithLoans(t *testing.T) {
	limit := int64(300000)
	in := ProjectionInput{
		Today:             "2026-11-10",
		StartBalanceCents: 500000,
		MonthlyLimitCents: &limit,
		Bands:             DefaultBalanceBands(),
		Loans: []ProjectionLoan{{
			Description:      "Financiamento do carro (12/48)",
			InstallmentCents: 140000,
			DueDates:         []string{"2026-11-05", "2026-12-05", "2027-01-05"},
		}},
		LoansByMonth:        map[string]int64{"2026-11": 140000, "2026-12": 140000, "2027-01": 140000},
		InstallmentsByMonth: map[string]int64{},
	}
	p := SimulateProjection(in)
	// Parcela atrasada entra hoje.
	var loanToday int64
	for _, ev := range p.Days[0].Events {
		if ev.Kind == EventLoan {
			loanToday += ev.AmountCents
		}
	}
	if loanToday != -140000 {
		t.Fatalf("parcela atrasada deveria sair hoje: %+v", p.Days[0].Events)
	}
	if p.Months[0].LoansCents != 140000 || p.Months[1].LoansCents != 140000 || p.Months[2].LoansCents != 140000 || p.Months[3].LoansCents != 0 {
		t.Errorf("loansCents por mês: %+v", p.Months[:4])
	}
	// O limite mensal desconta a parcela antes do gasto diário estimado: dezembro = 300.000 − 140.000.
	if p.Months[1].DailySpendCents != 160000 {
		t.Errorf("gasto diário de dezembro = %d, want 160000", p.Months[1].DailySpendCents)
	}
	if p.Months[3].DailySpendCents != 300000 {
		t.Errorf("gasto diário sem parcela = %d", p.Months[3].DailySpendCents)
	}
}
