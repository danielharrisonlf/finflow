package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"finflow/internal/client"
	"finflow/internal/domain"
)

func TestHTTPClientPlanningRoutes(t *testing.T) {
	type call struct{ method, path, query, body string }
	var got []call
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, call{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/recurring":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[{"id":"r1","description":"Aluguel","kind":"EXPENSE","amountCents":150000,"dayOfMonth":10,"nextDate":"2026-12-10"}]`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"r2","description":"Netflix","kind":"EXPENSE","amountCents":5590,"dayOfMonth":3}`))
		case "/api/insights/projection":
			_, _ = w.Write([]byte(`{"startDate":"2026-11-27","startBalanceCents":100,"days":[{"date":"2026-11-27","balanceCents":100,"band":"YELLOW","events":[]}],"lowest":{"date":"2026-11-27","balanceCents":100},"months":[]}`))
		case "/api/insights/year":
			_, _ = w.Write([]byte(`{"year":2026,"months":[],"totals":{"incomeCents":1}}`))
		default:
			_, _ = w.Write([]byte(`{"month":11,"year":2026}`))
		}
	})
	cli := client.NewHTTPClient(ts.URL, "t")
	ctx := context.Background()
	m, y := 11, 2026

	ov, err := cli.InsightsOverview(ctx, domain.PeriodQuery{Month: &m, Year: &y})
	if err != nil || ov.Month != 11 {
		t.Fatalf("InsightsOverview: %+v %v", ov, err)
	}
	p, err := cli.InsightsProjection(ctx, 30)
	if err != nil || len(p.Days) != 1 || p.Days[0].Band != "YELLOW" {
		t.Fatalf("InsightsProjection: %+v %v", p, err)
	}
	if _, err := cli.InsightsProjection(ctx, 0); err != nil {
		t.Fatal(err)
	}
	yr, err := cli.InsightsYear(ctx, &y)
	if err != nil || yr.Year != 2026 || yr.Totals.IncomeCents != 1 {
		t.Fatalf("InsightsYear: %+v %v", yr, err)
	}
	if _, err := cli.InsightsCommitments(ctx, domain.PeriodQuery{}); err != nil {
		t.Fatal(err)
	}
	recs, err := cli.Recurring(ctx)
	if err != nil || len(recs) != 1 || recs[0].NextDate == nil || *recs[0].NextDate != "2026-12-10" {
		t.Fatalf("Recurring: %+v %v", recs, err)
	}
	created, err := cli.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "Netflix", Kind: domain.CategoryExpense, AmountCents: 5590, DayOfMonth: 3})
	if err != nil || created.ID != "r2" {
		t.Fatalf("CreateRecurring: %+v %v", created, err)
	}

	want := []call{
		{"GET", "/api/insights/overview", "month=11&year=2026", ""},
		{"GET", "/api/insights/projection", "days=30", ""},
		{"GET", "/api/insights/projection", "", ""},
		{"GET", "/api/insights/year", "year=2026", ""},
		{"GET", "/api/insights/commitments", "", ""},
		{"GET", "/api/recurring", "", ""},
	}
	if len(got) != len(want)+1 {
		t.Fatalf("chamadas = %+v", got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("chamada %d = %+v, want %+v", i, got[i], w)
		}
	}
	last := got[len(got)-1]
	var body map[string]any
	if last.method != "POST" || last.path != "/api/recurring" || json.Unmarshal([]byte(last.body), &body) != nil || body["dayOfMonth"] != float64(3) || body["kind"] != "EXPENSE" {
		t.Fatalf("POST /recurring = %+v", last)
	}
}
