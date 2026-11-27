package domain

import "testing"

func TestBalanceBand(t *testing.T) {
	b := DefaultBalanceBands()
	cases := []struct {
		balance int64
		want    string
	}{
		{250000, "DARK_GREEN"},
		{200000, "DARK_GREEN"},
		{199999, "LIGHT_GREEN"},
		{100000, "LIGHT_GREEN"},
		{50000, "YELLOW"},
		{0, "YELLOW"},
		{-1, "LIGHT_RED"},
		{-50000, "LIGHT_RED"},
		{-50001, "RED"},
	}
	for _, tc := range cases {
		if got := BalanceBand(tc.balance, b); got != tc.want {
			t.Errorf("BalanceBand(%d) = %s, want %s", tc.balance, got, tc.want)
		}
	}
}

func dayOf(p Projection, date string) ProjectionDay {
	for _, d := range p.Days {
		if d.Date == date {
			return d
		}
	}
	return ProjectionDay{}
}

func TestSimulateProjectionEvents(t *testing.T) {
	in := ProjectionInput{
		Today:             "2026-11-15",
		StartBalanceCents: 100000,
		Bands:             DefaultBalanceBands(),
		Recurring: []ProjectionRecurring{
			{Description: "Salário", Kind: CategoryIncome, AmountCents: 500000, Schedule: RecurringSchedule{DayOfMonth: 5, StartDate: "2026-01-01"}, Realized: map[string]bool{"2026-11": true}},
			{Description: "Aluguel", Kind: CategoryExpense, AmountCents: 200000, Schedule: RecurringSchedule{DayOfMonth: 10, StartDate: "2026-01-01"}},
			{Description: "Academia", Kind: CategoryExpense, AmountCents: 10000, Schedule: RecurringSchedule{DayOfMonth: 31, StartDate: "2026-01-01", EndDate: strp("2027-01-31")}},
		},
		Pending: []ProjectionPending{
			{Description: "Conta atrasada", Date: "2026-11-01", AmountCents: -5000},
			{Description: "Reembolso", Date: "2026-11-20", AmountCents: 22000},
		},
		Invoices: []ProjectionInvoice{
			{Description: "Fatura Nubank", DueDate: "2026-12-10", TotalCents: 80000, InstallmentsCents: 30000},
		},
	}
	p := SimulateProjection(in)
	if p.StartDate != "2026-11-15" || p.LimitConfigured || len(p.Days) < ProjectionHorizonDays {
		t.Fatalf("cabeçalho: start=%s limit=%v days=%d", p.StartDate, p.LimitConfigured, len(p.Days))
	}

	// Hoje: aluguel atrasado do mês (dia 10, não realizado) + pendência atrasada.
	today := p.Days[0]
	if today.OutCents != 205000 || today.BalanceCents != 100000-205000 || today.Band != "RED" || len(today.Events) != 2 {
		t.Fatalf("hoje: %+v", today)
	}
	if d := dayOf(p, "2026-11-20"); d.InCents != 22000 {
		t.Fatalf("reembolso pendente: %+v", d)
	}
	if d := dayOf(p, "2026-11-30"); d.OutCents != 10000 {
		t.Fatalf("academia dia 31 em novembro cai no dia 30: %+v", d)
	}
	if d := dayOf(p, "2026-12-05"); d.InCents != 500000 {
		t.Fatalf("salário de dezembro: %+v", d)
	}
	if d := dayOf(p, "2026-12-10"); d.OutCents != 280000 {
		t.Fatalf("aluguel + fatura em 10/12: %+v", d)
	}
	if d := dayOf(p, "2027-02-28"); d.OutCents != 0 {
		t.Fatalf("academia encerrada em jan/2027: %+v", d)
	}
	// Próxima entrada: reembolso pendente em 20/11; o menor saldo antes dela é o de hoje (negativo).
	if p.NextIncomeDate == nil || *p.NextIncomeDate != "2026-11-20" || p.CanSaveNowCents != 0 {
		t.Fatalf("nextIncome=%v canSave=%d", deref(p.NextIncomeDate), p.CanSaveNowCents)
	}
	if p.Lowest.BalanceCents != -105000 || p.Lowest.Date != "2026-11-15" {
		t.Fatalf("menor saldo em 30 dias: %+v", p.Lowest)
	}
	if len(p.Months) != 12 || p.Months[0].Month != 11 || p.Months[11].Month != 10 || p.Months[11].Year != 2027 {
		t.Fatalf("meses: %+v", p.Months)
	}
	nov, dec := p.Months[0], p.Months[1]
	if nov.IncomeCents != 22000 || nov.RecurringOutCents != 210000 || nov.PendingOutCents != 5000 || nov.EndBalanceCents != dayOf(p, "2026-11-30").BalanceCents {
		t.Fatalf("novembro: %+v", nov)
	}
	if dec.IncomeCents != 500000 || dec.InvoicesCents != 80000 || dec.InstallmentsCents != 30000 || dec.RecurringOutCents != 210000 {
		t.Fatalf("dezembro: %+v", dec)
	}
}

func TestSimulateProjectionDailySpend(t *testing.T) {
	limit := int64(300000)
	in := ProjectionInput{
		Today:             "2026-11-21",
		StartBalanceCents: 1000000,
		MonthlyLimitCents: &limit,
		Bands:             DefaultBalanceBands(),
		Recurring: []ProjectionRecurring{
			// Realizada no mês corrente: sai da projeção, mas continua abatendo o orçamento variável.
			{Description: "Aluguel", Kind: CategoryExpense, AmountCents: 150000, Schedule: RecurringSchedule{DayOfMonth: 5, StartDate: "2026-01-01"}, Realized: map[string]bool{"2026-11": true}},
		},
		InstallmentsByMonth:       map[string]int64{"2026-11": 30000, "2026-12": 31000},
		CurrentMonthVariableCents: 60000,
	}
	p := SimulateProjection(in)
	if !p.LimitConfigured {
		t.Fatal("limitConfigured deveria ser true")
	}
	// Novembro: (300000 − 150000 − 30000 − 60000) = 60000 em 10 dias (21..30) = 6000/dia.
	var novDaily int64
	for _, d := range p.Days[:10] {
		for _, ev := range d.Events {
			if ev.Kind == EventDailySpend {
				novDaily -= ev.AmountCents
				if ev.AmountCents != -6000 {
					t.Fatalf("diária de novembro = %d", ev.AmountCents)
				}
			}
		}
	}
	if novDaily != 60000 || p.Months[0].DailySpendCents != 60000 {
		t.Fatalf("gasto diário de novembro = %d / %d", novDaily, p.Months[0].DailySpendCents)
	}
	// Dezembro: (300000 − 150000 − 31000) = 119000 em 31 dias, soma exata.
	if p.Months[1].DailySpendCents != 119000 || p.Months[1].RecurringOutCents != 150000 {
		t.Fatalf("dezembro: %+v", p.Months[1])
	}
	if d := dayOf(p, "2026-12-01"); d.OutCents != 3839 {
		t.Fatalf("1º de dezembro recebe o resto da divisão: %+v", d)
	}
}

func TestSimulateProjectionLeapYearCoversTwelveMonths(t *testing.T) {
	p := SimulateProjection(ProjectionInput{Today: "2027-11-01", Bands: DefaultBalanceBands()})
	last := p.Days[len(p.Days)-1].Date
	if last != "2028-10-31" || len(p.Days) != 366 {
		t.Fatalf("simulação deveria cobrir até 31/10/2028, terminou em %s (%d dias)", last, len(p.Days))
	}
	if p.NextIncomeDate != nil || p.CanSaveNowCents != 0 {
		t.Fatalf("sem eventos: next=%v canSave=%d", deref(p.NextIncomeDate), p.CanSaveNowCents)
	}
}

func TestAffordableDates(t *testing.T) {
	days := []ProjectionDay{
		{Date: "d0", BalanceCents: 100},
		{Date: "d1", BalanceCents: 50},
		{Date: "d2", BalanceCents: 300},
		{Date: "d3", BalanceCents: 200},
	}
	got := AffordableDates(days, []int64{40, 150, 200, 201})
	want := []string{"d0", "d2", "d2", "<nil>"}
	for i := range want {
		if deref(got[i]) != want[i] {
			t.Errorf("custo %d: %s, want %s", i, deref(got[i]), want[i])
		}
	}
}
