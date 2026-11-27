package client_test

import (
	"context"
	"net/http"
	"testing"

	"finflow/internal/client"
)

func TestHTTPClientWealthRoutes(t *testing.T) {
	var got []string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/investments":
			_, _ = w.Write([]byte(`[{"id":"i1","name":"CDB","kind":"FIXED_INCOME","investedCents":100,"currentCents":110,"returnCents":10,"returnPct":10}]`))
		case "/api/investments/summary":
			_, _ = w.Write([]byte(`{"totalInvestedCents":100,"totalCurrentCents":110,"byKind":[],"byInstitution":[],"history":[]}`))
		case "/api/loans":
			_, _ = w.Write([]byte(`[{"id":"l1","name":"Carro","kind":"VEHICLE","monthlyRatePct":1.49,"outstandingCents":5000}]`))
		case "/api/loans/strategy":
			_, _ = w.Write([]byte(`{"avalanche":{"order":["l1"],"months":3},"snowball":{"order":["l1"],"months":3},"minimumOnly":{"order":["l1"],"months":5}}`))
		case "/api/insights/net-worth":
			_, _ = w.Write([]byte(`[{"month":11,"year":2026,"netWorthCents":42}]`))
		}
	})
	cli := client.NewHTTPClient(ts.URL, "t")
	ctx := context.Background()

	inv, err := cli.Investments(ctx, true)
	if err != nil || len(inv) != 1 || inv[0].ReturnPct == nil || *inv[0].ReturnPct != 10 {
		t.Fatalf("Investments: %+v %v", inv, err)
	}
	if s, err := cli.InvestmentSummary(ctx); err != nil || s.TotalCurrentCents != 110 {
		t.Fatalf("InvestmentSummary: %+v %v", s, err)
	}
	if l, err := cli.Loans(ctx); err != nil || len(l) != 1 || l[0].MonthlyRatePct != 1.49 {
		t.Fatalf("Loans: %+v %v", l, err)
	}
	if s, err := cli.DebtStrategy(ctx, 50000); err != nil || s.MinimumOnly.Months != 5 {
		t.Fatalf("DebtStrategy: %+v %v", s, err)
	}
	if p, err := cli.NetWorthHistory(ctx, 6); err != nil || len(p) != 1 || p[0].NetWorthCents != 42 {
		t.Fatalf("NetWorthHistory: %+v %v", p, err)
	}
	if _, err := cli.NetWorthHistory(ctx, 0); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /api/investments?includeArchived=true",
		"GET /api/investments/summary?",
		"GET /api/loans?",
		"GET /api/loans/strategy?extraCents=50000",
		"GET /api/insights/net-worth?months=6",
		"GET /api/insights/net-worth?",
	}
	if len(got) != len(want) {
		t.Fatalf("chamadas = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chamada %d = %q, want %q", i, got[i], want[i])
		}
	}
}
