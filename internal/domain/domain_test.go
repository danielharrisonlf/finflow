package domain

import (
	"strings"
	"testing"
)

func TestMoney(t *testing.T) {
	if got := FormatBRL(123456); got != "R$ 1.234,56" {
		t.Fatalf("FormatBRL(123456) = %q", got)
	}
	if got := FormatBRL(-4590); got != "-R$ 45,90" {
		t.Fatalf("FormatBRL(-4590) = %q", got)
	}
	if got := FormatBRL(0); got != "R$ 0,00" {
		t.Fatalf("FormatBRL(0) = %q", got)
	}

	parts, err := SplitInstallments(10000, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 || parts[0] != 3334 || parts[1] != 3333 || parts[2] != 3333 {
		t.Fatalf("SplitInstallments(10000, 3) = %v", parts)
	}

	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"45,90", 4590, true},
		{"1.234,56", 123456, true},
		{"R$ 50", 5000, true},
		{"0", 0, false},
		{"abc", 0, false},
	}
	for _, tc := range cases {
		got, ok := ParseBRLToCents(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ParseBRLToCents(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestDateHelpers(t *testing.T) {
	if got := AddMonthsToDateString("2026-01-31", 1); got != "2026-02-28" {
		t.Fatalf("AddMonthsToDateString jan31+1 = %q", got)
	}
	if got := AddDays("2026-12-30", 3); got != "2027-01-02" {
		t.Fatalf("AddDays = %q", got)
	}
	s, e := MonthRange(2026, 12)
	if s != "2026-12-01" || e != "2027-01-01" {
		t.Fatalf("MonthRange(2026, 12) = %q..%q", s, e)
	}
	if m := MonthsUntil("2026-10-01", "2026-01-01"); m != 9 {
		t.Fatalf("MonthsUntil = %d, want 9", m)
	}
	if m := MonthsUntil("2026-01-01", "2026-06-01"); m != 1 {
		t.Fatalf("MonthsUntil past = %d, want 1", m)
	}
}

func TestInvoiceCycle(t *testing.T) {
	// Compra antes do fechamento entra no mesmo mês
	p1 := InvoicePeriodForPurchase("2026-05-09", 10)
	if p1.Year != 2026 || p1.Month != 5 {
		t.Fatalf("before closing = %+v", p1)
	}
	// Compra no dia do fechamento vai para a próxima fatura
	p2 := InvoicePeriodForPurchase("2026-05-10", 10)
	if p2.Year != 2026 || p2.Month != 6 {
		t.Fatalf("on closing = %+v", p2)
	}
	// Virada de ano
	p3 := InvoicePeriodForPurchase("2026-12-15", 10)
	if p3.Year != 2027 || p3.Month != 1 {
		t.Fatalf("year wrap = %+v", p3)
	}
	// Vencimento no mesmo mês quando dueDay > closingDay
	if due := InvoiceDueDate(InvoicePeriod{2026, 5}, 10, 17); due != "2026-05-17" {
		t.Fatalf("due same month = %q", due)
	}
	// Vencimento no mês seguinte quando dueDay <= closingDay
	if due := InvoiceDueDate(InvoicePeriod{2026, 12}, 25, 5); due != "2027-01-05" {
		t.Fatalf("due next month = %q", due)
	}
	if st := EffectiveInvoiceStatus(true, "2026-05-10", "2026-05-01"); st != InvoicePaid {
		t.Fatalf("paid status = %s", st)
	}
	if st := EffectiveInvoiceStatus(false, "2026-05-10", "2026-05-10"); st != InvoiceClosed {
		t.Fatalf("closed status = %s", st)
	}
	if st := EffectiveInvoiceStatus(false, "2026-05-10", "2026-05-09"); st != InvoiceOpen {
		t.Fatalf("open status = %s", st)
	}
}

func TestBudgetAndGoalMath(t *testing.T) {
	if v := ComputeBudgetView(10000, 6900); v.Status != BudgetOK {
		t.Fatalf("expected OK, got %+v", v)
	}
	if v := ComputeBudgetView(10000, 7000); v.Status != BudgetWarning {
		t.Fatalf("expected WARNING, got %+v", v)
	}
	if v := ComputeBudgetView(10000, 9100); v.Status != BudgetDanger {
		t.Fatalf("expected DANGER, got %+v", v)
	}
	if th := CrossedThreshold(65, 72); th != 70 {
		t.Fatalf("expected 70, got %d", th)
	}
	if th := CrossedThreshold(75, 85); th != 0 {
		t.Fatalf("expected 0, got %d", th)
	}
	if th := CrossedThreshold(85, 92); th != 90 {
		t.Fatalf("expected 90, got %d", th)
	}

	dl := "2026-07-01"
	needed := MonthlyNeededCents(100000, 40000, &dl, "2026-01-01")
	if needed == nil || *needed != 10000 {
		t.Fatalf("MonthlyNeededCents = %v", needed)
	}
	if pct := GoalPercent(120000, 30000); pct != 25 {
		t.Fatalf("GoalPercent = %v", pct)
	}
}

func TestExportFormats(t *testing.T) {
	cat := "Alimentação"
	acc := "Nubank"
	txs := []Transaction{{
		ID:           "tx-1",
		Type:         TxExpense,
		Description:  "Almoço; especial",
		AmountCents:  -4590,
		Date:         "2026-10-12",
		Status:       TxConfirmed,
		AccountName:  &acc,
		CategoryName: &cat,
	}}
	csv := BuildCSV(txs)
	if !strings.HasPrefix(csv, "\uFEFFData;Descrição") {
		t.Fatalf("CSV missing BOM/header: %q", csv[:30])
	}
	if !strings.Contains(csv, `"Almoço; especial"`) || !strings.Contains(csv, "-45,90") {
		t.Fatalf("CSV malformed: %s", csv)
	}
	ofx := BuildOFX(txs)
	if !strings.Contains(ofx, "<OFX>") || !strings.Contains(ofx, "<TRNAMT>-45.90") || !strings.Contains(ofx, "<TRNTYPE>DEBIT") {
		t.Fatalf("OFX malformed: %s", ofx)
	}
}
