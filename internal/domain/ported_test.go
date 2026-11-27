package domain

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// Casos portados de date.util.test.ts, money.util.test.ts, invoice-cycle.test.ts,
// budget-view.test.ts, goal-math.test.ts e budget-threshold.test.ts.

func TestDateUtilCases(t *testing.T) {
	t.Run("monthRange", func(t *testing.T) {
		cases := []struct {
			year, month int
			start, end  string
		}{
			{2026, 10, "2026-10-01", "2026-11-01"},
			{2026, 12, "2026-12-01", "2027-01-01"},
		}
		for _, tc := range cases {
			s, e := MonthRange(tc.year, tc.month)
			if s != tc.start || e != tc.end {
				t.Errorf("MonthRange(%d, %d) = %s..%s, want %s..%s", tc.year, tc.month, s, e, tc.start, tc.end)
			}
		}
	})

	t.Run("addMonthsToDateString", func(t *testing.T) {
		cases := []struct {
			in    string
			delta int
			want  string
		}{
			{"2026-01-15", 2, "2026-03-15"},
			{"2026-01-31", 1, "2026-02-28"},
		}
		for _, tc := range cases {
			if got := AddMonthsToDateString(tc.in, tc.delta); got != tc.want {
				t.Errorf("AddMonthsToDateString(%q, %d) = %q, want %q", tc.in, tc.delta, got, tc.want)
			}
		}
	})

	t.Run("addDays", func(t *testing.T) {
		cases := []struct {
			in   string
			days int
			want string
		}{
			{"2026-10-30", 7, "2026-11-06"},
			{"2026-12-30", 3, "2027-01-02"},
		}
		for _, tc := range cases {
			if got := AddDays(tc.in, tc.days); got != tc.want {
				t.Errorf("AddDays(%q, %d) = %q, want %q", tc.in, tc.days, got, tc.want)
			}
		}
	})

	t.Run("monthsUntil", func(t *testing.T) {
		cases := []struct {
			deadline, from string
			want           int
		}{
			{"2027-10-30", "2026-10-30", 12},
			{"2026-01-01", "2026-10-30", 1},
		}
		for _, tc := range cases {
			if got := MonthsUntil(tc.deadline, tc.from); got != tc.want {
				t.Errorf("MonthsUntil(%q, %q) = %d, want %d", tc.deadline, tc.from, got, tc.want)
			}
		}
	})
}

// withClock fixa o relógio e o fuso de negócio durante o teste.
func withClock(t *testing.T, tz string, now time.Time) {
	t.Helper()
	t.Setenv("FINFLOW_TZ", tz)
	locOnce = sync.Once{}
	loc = nil
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() {
		nowFunc = time.Now
		locOnce = sync.Once{}
		loc = nil
	})
}

func TestBusinessTimezone(t *testing.T) {
	cases := []struct {
		name       string
		tz         string
		now        time.Time
		today      string
		month, yr  int
		inMonth    string
		notInMonth string
	}{
		{
			name:  "padrão São Paulo: 01:30Z ainda é o dia anterior",
			tz:    "",
			now:   time.Date(2026, 11, 28, 1, 30, 0, 0, time.UTC),
			today: "2026-11-27", month: 11, yr: 2026,
			inMonth: "2026-11-30", notInMonth: "2026-12-01",
		},
		{
			name:  "virada de mês em UTC ainda é o mês anterior em São Paulo",
			tz:    "America/Sao_Paulo",
			now:   time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC),
			today: "2026-11-30", month: 11, yr: 2026,
			inMonth: "2026-11-01", notInMonth: "2026-10-31",
		},
		{
			name:  "FINFLOW_TZ=UTC sobrescreve o padrão",
			tz:    "UTC",
			now:   time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC),
			today: "2026-12-01", month: 12, yr: 2026,
			inMonth: "2026-12-31", notInMonth: "2026-11-30",
		},
		{
			name:  "FINFLOW_TZ inválido cai no padrão",
			tz:    "Nada/Existe",
			now:   time.Date(2026, 11, 28, 1, 30, 0, 0, time.UTC),
			today: "2026-11-27", month: 11, yr: 2026,
			inMonth: "2026-11-27", notInMonth: "2026-12-01",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withClock(t, tc.tz, tc.now)
			if got := TodayString(); got != tc.today {
				t.Errorf("TodayString() = %q, want %q", got, tc.today)
			}
			m, y := CurrentMonthYear()
			if m != tc.month || y != tc.yr {
				t.Errorf("CurrentMonthYear() = %d/%d, want %d/%d", m, y, tc.month, tc.yr)
			}
			if !IsInCurrentMonth(tc.inMonth) {
				t.Errorf("IsInCurrentMonth(%q) = false", tc.inMonth)
			}
			if IsInCurrentMonth(tc.notInMonth) {
				t.Errorf("IsInCurrentMonth(%q) = true", tc.notInMonth)
			}
		})
	}
}

func TestMoneyUtilCases(t *testing.T) {
	t.Run("splitInstallments", func(t *testing.T) {
		cases := []struct {
			total int64
			n     int
			want  []int64
		}{
			{350000, 10, []int64{35000, 35000, 35000, 35000, 35000, 35000, 35000, 35000, 35000, 35000}},
			{10000, 3, []int64{3334, 3333, 3333}},
			{4590, 1, []int64{4590}},
		}
		for _, tc := range cases {
			got, err := SplitInstallments(tc.total, tc.n)
			if err != nil {
				t.Fatalf("SplitInstallments(%d, %d): %v", tc.total, tc.n, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("SplitInstallments(%d, %d) = %v, want %v", tc.total, tc.n, got, tc.want)
			}
			var sum int64
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("SplitInstallments(%d, %d) = %v, want %v", tc.total, tc.n, got, tc.want)
				}
				sum += got[i]
			}
			if sum != tc.total {
				t.Errorf("soma das parcelas = %d, want %d", sum, tc.total)
			}
		}
		if _, err := SplitInstallments(1000, 0); err == nil {
			t.Error("SplitInstallments(1000, 0) deveria falhar")
		}
	})

	t.Run("parseBRLToCents", func(t *testing.T) {
		cases := []struct {
			in   string
			want int64
			ok   bool
		}{
			{"45,90", 4590, true},
			{"1.234,56", 123456, true},
			{"50", 5000, true},
			{"R$ 12,30", 1230, true},
			{"abc", 0, false},
			{"0", 0, false},
			{"", 0, false},
			{"0,001", 0, false},
			{"100.000.000.000,00", 1e13, true},
			{"100.000.000.000,01", 0, false},
			{"10000000000000000000000", 0, false}, // 1e22: antes virava MinInt64 com ok=true
		}
		for _, tc := range cases {
			got, ok := ParseBRLToCents(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Errorf("ParseBRLToCents(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
			}
		}
	})
}

func TestInvoiceCycleCases(t *testing.T) {
	periods := []struct {
		date       string
		closingDay int
		want       InvoicePeriod
	}{
		{"2026-10-04", 5, InvoicePeriod{2026, 10}},
		{"2026-10-05", 5, InvoicePeriod{2026, 11}},
		{"2026-12-28", 20, InvoicePeriod{2027, 1}},
	}
	for _, tc := range periods {
		if got := InvoicePeriodForPurchase(tc.date, tc.closingDay); got != tc.want {
			t.Errorf("InvoicePeriodForPurchase(%q, %d) = %+v, want %+v", tc.date, tc.closingDay, got, tc.want)
		}
	}
	if got := AddMonthsToPeriod(InvoicePeriod{2026, 11}, 3); got != (InvoicePeriod{2027, 2}) {
		t.Errorf("AddMonthsToPeriod({2026,11}, 3) = %+v, want {2027 2}", got)
	}
	dues := []struct {
		period             InvoicePeriod
		closingDay, dueDay int
		want               string
	}{
		{InvoicePeriod{2026, 10}, 5, 12, "2026-10-12"},
		{InvoicePeriod{2026, 12}, 25, 4, "2027-01-04"},
	}
	for _, tc := range dues {
		if got := InvoiceDueDate(tc.period, tc.closingDay, tc.dueDay); got != tc.want {
			t.Errorf("InvoiceDueDate(%+v, %d, %d) = %q, want %q", tc.period, tc.closingDay, tc.dueDay, got, tc.want)
		}
	}
	closing := InvoiceClosingDate(InvoicePeriod{2026, 10}, 15)
	statuses := []struct {
		paid  bool
		today string
		want  InvoiceStatus
	}{
		{false, "2026-10-14", InvoiceOpen},
		{false, "2026-10-15", InvoiceClosed},
		{true, "2026-10-01", InvoicePaid},
	}
	for _, tc := range statuses {
		if got := EffectiveInvoiceStatus(tc.paid, closing, tc.today); got != tc.want {
			t.Errorf("EffectiveInvoiceStatus(%v, %q, %q) = %s, want %s", tc.paid, closing, tc.today, got, tc.want)
		}
	}
}

func TestBudgetViewCases(t *testing.T) {
	cases := []struct {
		name          string
		amount, spent int64
		want          BudgetView
	}{
		{"abaixo de 70% é OK", 150000, 100000, BudgetView{66.7, BudgetOK}},
		{"70% é WARNING", 100000, 70000, BudgetView{70, BudgetWarning}},
		{"exatamente 90% é WARNING", 100000, 90000, BudgetView{90, BudgetWarning}},
		{"acima de 90% é DANGER", 100000, 90001, BudgetView{90, BudgetDanger}},
		{"estourado passa de 100%", 100000, 125000, BudgetView{125, BudgetDanger}},
		{"orçamento zerado não divide por zero", 0, 5000, BudgetView{0, BudgetOK}},
	}
	for _, tc := range cases {
		if got := ComputeBudgetView(tc.amount, tc.spent); got != tc.want {
			t.Errorf("%s: ComputeBudgetView(%d, %d) = %+v, want %+v", tc.name, tc.amount, tc.spent, got, tc.want)
		}
	}
}

func TestGoalMathCases(t *testing.T) {
	ptr := func(s string) *string { return &s }
	cases := []struct {
		name          string
		target, saved int64
		deadline      *string
		today         string
		want          *int64
	}{
		{"restante dividido pelos meses", 1_000_000, 400_000, ptr("2027-10-30"), "2026-10-30", new(int64(50_000))},
		{"prazo no mês seguinte exige o restante", 100_000, 40_000, ptr("2026-11-30"), "2026-10-30", new(int64(60_000))},
		{"sem prazo retorna nil", 100_000, 0, nil, "2026-10-30", nil},
		{"meta concluída retorna nil", 100_000, 100_000, ptr("2027-01-01"), "2026-10-30", nil},
		{"prazo vencido usa no mínimo 1 mês", 100_000, 50_000, ptr("2026-10-01"), "2026-10-30", new(int64(50_000))},
	}
	for _, tc := range cases {
		got := MonthlyNeededCents(tc.target, tc.saved, tc.deadline, tc.today)
		switch {
		case tc.want == nil && got != nil:
			t.Errorf("%s: got %d, want nil", tc.name, *got)
		case tc.want != nil && (got == nil || *got != *tc.want):
			t.Errorf("%s: got %v, want %d", tc.name, got, *tc.want)
		}
	}

	percents := []struct {
		target, saved int64
		want          float64
	}{
		{300_000, 100_000, 33.3},
		{100_000, 150_000, 100},
	}
	for _, tc := range percents {
		if got := GoalPercent(tc.target, tc.saved); got != tc.want {
			t.Errorf("GoalPercent(%d, %d) = %v, want %v", tc.target, tc.saved, got, tc.want)
		}
	}
}

func TestBudgetThresholdCases(t *testing.T) {
	cases := []struct {
		before, after float64
		want          int
	}{
		{65, 72, 70},
		{65, 95, 90},
		{85, 91, 90},
		{75, 80, 0},
		{92, 99, 0},
		{10, 50, 0},
	}
	for _, tc := range cases {
		if got := CrossedThreshold(tc.before, tc.after); got != tc.want {
			t.Errorf("CrossedThreshold(%v, %v) = %d, want %d", tc.before, tc.after, got, tc.want)
		}
	}
}

func TestValidationCountsRunesAndCanonicalUUID(t *testing.T) {
	validUUID := "11111111-1111-4111-8111-111111111111"
	uuids := []struct {
		in   string
		want bool
	}{
		{validUUID, true},
		{strings.ToUpper("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"), true},
		{"{" + validUUID + "}", false},
		{"urn:uuid:" + validUUID, false},
		{strings.ReplaceAll(validUUID, "-", ""), false},
		{"", false},
	}
	for _, tc := range uuids {
		if got := IsValidUUID(tc.in); got != tc.want {
			t.Errorf("IsValidUUID(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}

	names := []struct {
		name    string
		invalid bool
	}{
		{strings.Repeat("ç", 60), false}, // 120 bytes, 60 caracteres
		{strings.Repeat("ç", 61), true},
		{"", true},
	}
	for _, tc := range names {
		issues := ValidateCreateAccount(CreateAccountInput{Name: tc.name, Type: AccountChecking})
		if (len(issues) > 0) != tc.invalid {
			t.Errorf("ValidateCreateAccount(nome de %d runas) issues = %v", len([]rune(tc.name)), issues)
		}
	}

	tx := CreateTransactionInput{
		Type: TxExpense, Description: strings.Repeat("é", 120), AmountCents: 100,
		Date: "2026-10-30", Status: TxConfirmed, AccountID: validUUID,
	}
	if issues := ValidateCreateTransaction(tx); len(issues) > 0 {
		t.Errorf("descrição de 120 caracteres acentuados deveria passar: %v", issues)
	}
}

func TestQueryValidation(t *testing.T) {
	m := func(v int) *int { return &v }
	periods := []struct {
		q     PeriodQuery
		paths []string
	}{
		{PeriodQuery{}, nil},
		{PeriodQuery{Month: m(12), Year: m(2026)}, nil},
		{PeriodQuery{Month: m(13)}, []string{"month"}},
		{PeriodQuery{Month: m(0), Year: m(1999)}, []string{"month", "year"}},
	}
	for _, tc := range periods {
		assertPaths(t, ValidatePeriodQuery(tc.q), tc.paths)
	}

	foo := TransactionType("foo")
	bad := TransactionStatus("bar")
	notUUID := "abc"
	long := strings.Repeat("ã", 121)
	ok := strings.Repeat("ã", 120)
	lists := []struct {
		q     ListTransactionsQuery
		paths []string
	}{
		{ListTransactionsQuery{}, nil},
		{ListTransactionsQuery{Page: 2, PageSize: 100, Search: &ok}, nil},
		{ListTransactionsQuery{PageSize: 101}, []string{"pageSize"}},
		{ListTransactionsQuery{Page: -1}, []string{"page"}},
		{ListTransactionsQuery{Type: &foo, Status: &bad}, []string{"type", "status"}},
		{ListTransactionsQuery{AccountID: &notUUID, Month: m(13)}, []string{"month", "accountId"}},
		{ListTransactionsQuery{Search: &long}, []string{"search"}},
	}
	for _, tc := range lists {
		assertPaths(t, ValidateListTransactionsQuery(tc.q), tc.paths)
	}
}

func assertPaths(t *testing.T, issues []ValidationIssue, want []string) {
	t.Helper()
	got := make([]string, 0, len(issues))
	for _, is := range issues {
		got = append(got, is.Path)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("issues = %v, want paths %v", issues, want)
	}
}

func TestCreateCardExpenseInstallmentsJSON(t *testing.T) {
	cases := []struct {
		body string
		want int
	}{
		{`{"description":"x"}`, 1},
		{`{"description":"x","installments":3}`, 3},
		{`{"description":"x","installments":0}`, 0},
		{`{"description":"x","installments":null}`, 0},
	}
	for _, tc := range cases {
		var in CreateCardExpenseInput
		if err := json.Unmarshal([]byte(tc.body), &in); err != nil {
			t.Fatalf("Unmarshal(%s): %v", tc.body, err)
		}
		if in.Installments != tc.want || in.Description != "x" {
			t.Errorf("Unmarshal(%s) = %+v, want installments %d", tc.body, in, tc.want)
		}
	}
	var in CreateCardExpenseInput
	if err := json.Unmarshal([]byte(`{"installments":"3"}`), &in); err == nil {
		t.Error("installments string deveria falhar")
	}
	issues := ValidateCreateCardExpense(CreateCardExpenseInput{
		Description: "x", TotalCents: 100, Date: "2026-10-30", Installments: 0,
	})
	assertPaths(t, issues, []string{"installments"})
}
