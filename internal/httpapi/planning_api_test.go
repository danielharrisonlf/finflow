package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestPlanningAPI(t *testing.T) {
	testutil.ForEachDialect(t, runPlanningAPI)
}

func expectCode(t *testing.T, rr *httptest.ResponseRecorder, want int, label string) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("%s: status %d (want %d) %s", label, rr.Code, want, rr.Body.String())
	}
}

func expectIssue(t *testing.T, rr *httptest.ResponseRecorder, path, label string) {
	t.Helper()
	expectCode(t, rr, 400, label)
	var body struct {
		Issues []domain.ValidationIssue `json:"issues"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	for _, is := range body.Issues {
		if is.Path == path {
			return
		}
	}
	t.Fatalf("%s: sem issue em %q: %s", label, path, rr.Body.String())
}

func runPlanningAPI(t *testing.T, database *db.DB) {
	t.Setenv("FINFLOW_PASSWORD", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	svcs := service.New(store.New(database), nil)
	srv := httpapi.NewServer(svcs, httpapi.ServerOptions{})
	today := domain.TodayString()
	missing := "/11111111-1111-4111-8111-111111111111"

	acc := decodeBody[domain.Account](t, doReq(t, srv, "POST", "/api/accounts", map[string]any{
		"name": "Conta", "type": "CHECKING", "initialBalanceCents": 500000,
	}, nil))

	// ───── Categorias com spendType
	res := doReq(t, srv, "POST", "/api/categories", map[string]any{
		"name": "Moradia", "kind": "EXPENSE", "icon": "house", "color": "#38bdf8", "spendType": "FIXED",
	}, nil)
	expectCode(t, res, 201, "categoria FIXED")
	moradia := decodeBody[domain.Category](t, res)
	sub := decodeBody[domain.Category](t, doReq(t, srv, "POST", "/api/categories", map[string]any{
		"name": "Aluguel", "kind": "EXPENSE", "icon": "house", "color": "#38bdf8", "parentId": moradia.ID,
	}, nil))
	if sub.SpendType == nil || *sub.SpendType != domain.SpendFixed {
		t.Fatalf("subcategoria deveria herdar FIXED: %+v", sub)
	}
	expectIssue(t, doReq(t, srv, "POST", "/api/categories", map[string]any{
		"name": "X", "kind": "EXPENSE", "icon": "x", "color": "#000000", "spendType": "OUTRO",
	}, nil), "spendType", "spendType inválido")
	if !strings.Contains(doReq(t, srv, "GET", "/api/categories", nil, nil).Body.String(), `"spendType":"FIXED"`) {
		t.Fatal("listagem de categorias sem spendType")
	}

	// ───── Recorrências
	expectIssue(t, doReq(t, srv, "POST", "/api/recurring", map[string]any{"description": "", "kind": "EXPENSE", "amountCents": 0, "dayOfMonth": 40}, nil), "dayOfMonth", "recorrência inválida")
	res = doReq(t, srv, "POST", "/api/recurring", map[string]any{
		"description": "Aluguel", "kind": "EXPENSE", "amountCents": 150000, "dayOfMonth": 1,
		"accountId": acc.ID, "categoryId": sub.ID, "startDate": today[:8] + "01",
	}, nil)
	expectCode(t, res, 201, "criar recorrência")
	rec := decodeBody[domain.Recurring](t, res)
	if rec.NextDate == nil || rec.CategoryName == nil || !rec.Active {
		t.Fatalf("recorrência: %+v", rec)
	}
	list := decodeBody[[]domain.Recurring](t, doReq(t, srv, "GET", "/api/recurring", nil, nil))
	if len(list) != 1 {
		t.Fatalf("lista de recorrências: %+v", list)
	}
	res = doReq(t, srv, "POST", "/api/recurring/"+rec.ID+"/post", nil, nil)
	expectCode(t, res, 201, "lançar recorrência (corpo vazio)")
	posted := decodeBody[domain.Transaction](t, res)
	if posted.RecurringID == nil || *posted.RecurringID != rec.ID || posted.AmountCents != -150000 {
		t.Fatalf("transação da recorrência: %+v", posted)
	}
	expectCode(t, doReq(t, srv, "POST", "/api/recurring/"+rec.ID+"/post", map[string]any{}, nil), 409, "lançar 2x no mês")
	expectIssue(t, doReq(t, srv, "POST", "/api/recurring/"+rec.ID+"/post", map[string]any{"date": "31/12/2026"}, nil), "date", "data inválida no post")
	expectCode(t, doReq(t, srv, "POST", "/api/recurring"+missing+"/post", nil, nil), 404, "post inexistente")
	updated := decodeBody[domain.Recurring](t, doReq(t, srv, "PATCH", "/api/recurring/"+rec.ID, map[string]any{"active": false, "endDate": nil}, nil))
	if updated.Active || updated.NextDate != nil {
		t.Fatalf("recorrência desativada: %+v", updated)
	}
	expectCode(t, doReq(t, srv, "PATCH", "/api/recurring"+missing, map[string]any{"active": true}, nil), 404, "patch inexistente")
	expectCode(t, doReq(t, srv, "PATCH", "/api/recurring/nao-uuid", map[string]any{}, nil), 400, "id não uuid")

	// ───── Configurações
	settings := decodeBody[domain.Settings](t, doReq(t, srv, "GET", "/api/settings", nil, nil))
	if settings.ReserveMonths != 6 || settings.BalanceBands.LightRed != -50000 || settings.MonthlyLimitCents != nil {
		t.Fatalf("settings padrão: %+v", settings)
	}
	res = doReq(t, srv, "PATCH", "/api/settings", map[string]any{
		"monthlyLimitCents": 400000, "spendPlan": map[string]any{"fixedPct": 50, "cuttablePct": 20, "variablePct": 30},
		"balanceBands": map[string]any{"darkGreen": 300000},
	}, nil)
	expectCode(t, res, 200, "patch settings")
	settings = decodeBody[domain.Settings](t, res)
	if *settings.MonthlyLimitCents != 400000 || settings.SpendPlan == nil || settings.BalanceBands.DarkGreen != 300000 || settings.BalanceBands.LightGreen != 100000 {
		t.Fatalf("settings após patch: %+v", settings)
	}
	if raw := doReq(t, srv, "GET", "/api/settings", nil, nil).Body.String(); !strings.Contains(raw, `"monthlyLimitCents":400000`) {
		t.Fatalf("settings não persistiu: %s", raw)
	}
	expectIssue(t, doReq(t, srv, "PATCH", "/api/settings", map[string]any{"spendPlan": map[string]any{"fixedPct": 90, "cuttablePct": 20, "variablePct": 30}}, nil), "spendPlan", "plano somando 140")

	// ───── Regras
	expectIssue(t, doReq(t, srv, "POST", "/api/rules", map[string]any{"pattern": "", "categoryId": "x"}, nil), "pattern", "regra inválida")
	expectCode(t, doReq(t, srv, "POST", "/api/rules", map[string]any{"pattern": "abc", "categoryId": missing[1:]}, nil), 404, "regra com categoria inexistente")
	// Lançada antes da regra: o apply categoriza.
	doReq(t, srv, "POST", "/api/transactions", map[string]any{"type": "EXPENSE", "description": "Condomínio Edifício", "amountCents": 45000, "date": today, "accountId": acc.ID}, nil)
	res = doReq(t, srv, "POST", "/api/rules", map[string]any{"pattern": "condominio", "categoryId": sub.ID, "priority": 5}, nil)
	expectCode(t, res, 201, "criar regra")
	rule := decodeBody[domain.CategoryRule](t, res)
	if rule.CategoryName != "Aluguel" || rule.Priority != 5 {
		t.Fatalf("regra: %+v", rule)
	}
	applied := decodeBody[domain.ApplyRulesResult](t, doReq(t, srv, "POST", "/api/rules/apply", nil, nil))
	if applied.Updated != 1 {
		t.Fatalf("apply: %+v", applied)
	}
	created := decodeBody[domain.Transaction](t, doReq(t, srv, "POST", "/api/transactions", map[string]any{
		"type": "EXPENSE", "description": "CONDOMÍNIO dezembro", "amountCents": 45000, "date": today, "accountId": acc.ID,
	}, nil))
	if created.CategoryID == nil || *created.CategoryID != sub.ID {
		t.Fatalf("regra não aplicada na criação: %+v", created)
	}
	rule = decodeBody[domain.CategoryRule](t, doReq(t, srv, "PATCH", "/api/rules/"+rule.ID, map[string]any{"pattern": "condomínio"}, nil))
	if rule.Pattern != "condomínio" {
		t.Fatalf("patch regra: %+v", rule)
	}
	if rules := decodeBody[[]domain.CategoryRule](t, doReq(t, srv, "GET", "/api/rules", nil, nil)); len(rules) != 1 {
		t.Fatalf("lista de regras: %+v", rules)
	}
	expectCode(t, doReq(t, srv, "DELETE", "/api/rules/"+rule.ID, nil, nil), 204, "excluir regra")
	expectCode(t, doReq(t, srv, "DELETE", "/api/rules/"+rule.ID, nil, nil), 404, "excluir regra 2x")

	// ───── Metas RESERVE
	expectCode(t, doReq(t, srv, "POST", "/api/goals", map[string]any{"name": "Reserva", "kind": "RESERVE", "targetCents": 100000}, nil), 201, "criar reserva")
	expectCode(t, doReq(t, srv, "POST", "/api/goals", map[string]any{"name": "Reserva 2", "kind": "RESERVE", "targetCents": 100000}, nil), 409, "segunda reserva")
	if goals := doReq(t, srv, "GET", "/api/goals", nil, nil).Body.String(); !strings.Contains(goals, `"kind":"RESERVE"`) {
		t.Fatalf("goal.kind ausente: %s", goals)
	}

	// ───── Desejos
	res = doReq(t, srv, "POST", "/api/wishlist", map[string]any{"name": "Air fryer", "priceCents": 49900}, nil)
	expectCode(t, res, 201, "criar desejo")
	wish := decodeBody[domain.WishItem](t, res)
	// Sem receitas e com limite de R$ 4.000/mês o saldo projetado zera: o desejo não cabe no horizonte.
	if wish.Status != domain.WishListed || wish.Priority != 1 || wish.AffordableOn != nil {
		t.Fatalf("desejo: %+v", wish)
	}
	expectIssue(t, doReq(t, srv, "POST", "/api/wishlist", map[string]any{"name": "x", "priceCents": -1}, nil), "priceCents", "desejo inválido")
	wish = decodeBody[domain.WishItem](t, doReq(t, srv, "PATCH", "/api/wishlist/"+wish.ID, map[string]any{"status": "BOUGHT", "notes": "comprado"}, nil))
	if wish.Status != domain.WishBought || wish.AffordableOn != nil || wish.Notes == nil {
		t.Fatalf("desejo comprado: %+v", wish)
	}
	if items := decodeBody[[]domain.WishItem](t, doReq(t, srv, "GET", "/api/wishlist", nil, nil)); len(items) != 1 {
		t.Fatalf("lista de desejos: %+v", items)
	}
	expectCode(t, doReq(t, srv, "DELETE", "/api/wishlist/"+wish.ID, nil, nil), 204, "excluir desejo")
	expectCode(t, doReq(t, srv, "PATCH", "/api/wishlist/"+wish.ID, map[string]any{"name": "y"}, nil), 404, "patch desejo inexistente")

	// ───── Insights
	res = doReq(t, srv, "GET", "/api/insights/overview", nil, nil)
	expectCode(t, res, 200, "overview")
	ov := decodeBody[domain.InsightsOverview](t, res)
	if ov.MonthlyLimit.LimitCents == nil || *ov.MonthlyLimit.LimitCents != 400000 || len(ov.SpendTypes) != 3 || ov.Plan == nil || ov.Reserve.Status == "" {
		t.Fatalf("overview: %+v", ov)
	}
	for _, field := range []string{`"pace"`, `"monthlyLimit"`, `"performance"`, `"verdict"`, `"spendTypes"`, `"netWorthCents"`, `"reserve"`, `"cutSuggestions"`} {
		if !strings.Contains(res.Body.String(), field) {
			t.Fatalf("overview sem %s: %s", field, res.Body.String())
		}
	}
	expectIssue(t, doReq(t, srv, "GET", "/api/insights/overview?month=13", nil, nil), "month", "overview mês 13")

	res = doReq(t, srv, "GET", "/api/insights/projection?days=10", nil, nil)
	expectCode(t, res, 200, "projeção")
	proj := decodeBody[domain.Projection](t, res)
	if len(proj.Days) != 10 || len(proj.Months) != 12 || !proj.LimitConfigured || proj.StartDate != today {
		t.Fatalf("projeção: days=%d months=%d limit=%v", len(proj.Days), len(proj.Months), proj.LimitConfigured)
	}
	if len(decodeBody[domain.Projection](t, doReq(t, srv, "GET", "/api/insights/projection", nil, nil)).Days) != 90 {
		t.Fatal("projeção padrão deveria ter 90 dias")
	}
	expectIssue(t, doReq(t, srv, "GET", "/api/insights/projection?days=366", nil, nil), "days", "projeção 366 dias")

	yr := decodeBody[domain.YearOverview](t, doReq(t, srv, "GET", "/api/insights/year?year=2026", nil, nil))
	if yr.Year != 2026 || len(yr.Months) != 12 {
		t.Fatalf("ano: %+v", yr)
	}
	expectIssue(t, doReq(t, srv, "GET", "/api/insights/year?year=1999", nil, nil), "year", "ano 1999")

	cm := decodeBody[domain.Commitments](t, doReq(t, srv, "GET", "/api/insights/commitments", nil, nil))
	if len(cm.Months) != 6 || cm.Items == nil {
		t.Fatalf("compromissos: %+v", cm)
	}
	expectIssue(t, doReq(t, srv, "GET", "/api/insights/commitments?month=0", nil, nil), "month", "compromissos mês 0")

	expectCode(t, doReq(t, srv, "DELETE", "/api/recurring/"+rec.ID, nil, nil), 204, "excluir recorrência")
	expectCode(t, doReq(t, srv, "DELETE", "/api/recurring/"+rec.ID, nil, nil), 404, "excluir recorrência 2x")
}

// TestInsightsOnSeed garante que os endpoints novos respondem com o seed completo.
func TestInsightsOnSeed(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		t.Setenv("FINFLOW_PASSWORD", "")
		st := store.New(database)
		if _, err := store.Seed(context.Background(), st); err != nil {
			t.Fatal(err)
		}
		srv := httpapi.NewServer(service.New(st, nil), httpapi.ServerOptions{})
		for _, path := range []string{
			"/api/insights/overview", "/api/insights/projection?days=365", "/api/insights/year",
			"/api/insights/commitments", "/api/recurring", "/api/rules", "/api/wishlist", "/api/settings",
		} {
			expectCode(t, doReq(t, srv, "GET", path, nil, nil), 200, path)
		}
		ov := decodeBody[domain.InsightsOverview](t, doReq(t, srv, "GET", "/api/insights/overview", nil, nil))
		if ov.Reserve.GoalID == nil || ov.MonthlyLimit.LimitCents == nil || ov.Performance.SavedCents <= 0 {
			t.Fatalf("seed deveria ter reserva, limite e economia: %+v", ov)
		}
		wishes := decodeBody[[]domain.WishItem](t, doReq(t, srv, "GET", "/api/wishlist", nil, nil))
		recs := decodeBody[[]domain.Recurring](t, doReq(t, srv, "GET", "/api/recurring", nil, nil))
		if len(wishes) != 2 || len(recs) != 6 {
			t.Fatalf("seed: %d desejos, %d recorrências", len(wishes), len(recs))
		}
	})
}
