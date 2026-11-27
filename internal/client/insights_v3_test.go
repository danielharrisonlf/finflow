package client_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"finflow/internal/client"
	"finflow/internal/domain"
)

func TestHTTPClientInsightsV3Routes(t *testing.T) {
	type call struct{ method, path, query, body string }
	var got []call
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, call{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`[{"id":"t1","name":"Viagem","color":"#ff0000","usageCount":2}]`))
		case "/api/insights/subscriptions":
			_, _ = w.Write([]byte(`{"items":[{"key":"spotify","cadence":"MONTHLY"}],"monthlyTotalCents":2190}`))
		case "/api/alerts":
			_, _ = w.Write([]byte(`[{"id":"PACE_ABOVE:pace:2026-11","kind":"PACE_ABOVE","severity":"WARNING"}]`))
		case "/api/insights/can-afford":
			_, _ = w.Write([]byte(`{"verdict":"CAREFUL","lowestBalanceAfterCents":500,"message":"ok"}`))
		case "/api/insights/cashflow":
			_, _ = w.Write([]byte(`{"basis":"ACCRUAL","months":[],"whereItWent":[]}`))
		default:
			_, _ = w.Write([]byte(`{"items":[],"total":0}`))
		}
	})
	cli := client.NewHTTPClient(ts.URL, "t")
	ctx := context.Background()

	tags, err := cli.Tags(ctx)
	if err != nil || len(tags) != 1 || tags[0].UsageCount != 2 {
		t.Fatalf("Tags: %+v %v", tags, err)
	}
	subs, err := cli.Subscriptions(ctx)
	if err != nil || subs.MonthlyTotalCents != 2190 || subs.Items[0].Key != "spotify" {
		t.Fatalf("Subscriptions: %+v %v", subs, err)
	}
	alerts, err := cli.Alerts(ctx)
	if err != nil || len(alerts) != 1 || alerts[0].Kind != "PACE_ABOVE" {
		t.Fatalf("Alerts: %+v %v", alerts, err)
	}
	n := 3
	ca, err := cli.CanAfford(ctx, domain.CanAffordInput{AmountCents: 1000, Installments: &n})
	if err != nil || ca.Verdict != "CAREFUL" {
		t.Fatalf("CanAfford: %+v %v", ca, err)
	}
	m, y := 6, 2025
	cf, err := cli.InsightsCashflow(ctx, domain.CashflowQuery{Months: 2, Basis: "ACCRUAL", Month: &m, Year: &y})
	if err != nil || cf.Basis != "ACCRUAL" {
		t.Fatalf("InsightsCashflow: %+v %v", cf, err)
	}
	tag := "t1"
	if _, err := cli.Transactions(ctx, domain.ListTransactionsQuery{TagID: &tag}); err != nil {
		t.Fatal(err)
	}

	want := []call{
		{"GET", "/api/tags", "", ""},
		{"GET", "/api/insights/subscriptions", "", ""},
		{"GET", "/api/alerts", "", ""},
		{"POST", "/api/insights/can-afford", "", `{"amountCents":1000,"installments":3}`},
		{"GET", "/api/insights/cashflow", "basis=ACCRUAL&month=6&months=2&year=2025", ""},
		{"GET", "/api/transactions", "tagId=t1", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("chamadas: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chamada %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
