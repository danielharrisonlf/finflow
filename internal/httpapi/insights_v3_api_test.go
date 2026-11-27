package httpapi_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

type staticResolver map[string]string

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if ip, ok := r[host]; ok {
		return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
	}
	return nil, fmt.Errorf("host desconhecido: %s", host)
}

func TestInsightsV3API(t *testing.T) {
	testutil.ForEachDialect(t, runInsightsV3API)
}

func runInsightsV3API(t *testing.T, database *db.DB) {
	t.Setenv("FINFLOW_PASSWORD", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	svcs := service.New(store.New(database), nil)
	srv := httpapi.NewServer(svcs, httpapi.ServerOptions{})
	today := domain.TodayString()
	missing := "11111111-1111-4111-8111-111111111111"

	a := decodeBody[domain.Account](t, doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "A", "type": "CHECKING", "initialBalanceCents": 300000}, nil))
	b := decodeBody[domain.Account](t, doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "B", "type": "CHECKING", "initialBalanceCents": 0}, nil))

	// ───── Tags
	res := doReq(t, srv, "POST", "/api/tags", map[string]any{"name": "Viagem", "color": "#ff0000"}, nil)
	expectCode(t, res, 201, "criar tag")
	tag := decodeBody[domain.Tag](t, res)
	expectCode(t, doReq(t, srv, "POST", "/api/tags", map[string]any{"name": "viagem"}, nil), 409, "tag repetida")
	expectIssue(t, doReq(t, srv, "POST", "/api/tags", map[string]any{"name": ""}, nil), "name", "tag sem nome")
	expectCode(t, doReq(t, srv, "PATCH", "/api/tags/"+tag.ID, map[string]any{"color": "#00ff00"}, nil), 200, "patch tag")
	expectCode(t, doReq(t, srv, "PATCH", "/api/tags/"+missing, map[string]any{"color": "#00ff00"}, nil), 404, "patch tag inexistente")

	res = doReq(t, srv, "POST", "/api/transactions", map[string]any{
		"type": "EXPENSE", "description": "Hotel", "amountCents": 20000, "date": today, "accountId": a.ID, "tagIds": []string{tag.ID},
	}, nil)
	expectCode(t, res, 201, "transação com tag")
	hotel := decodeBody[domain.Transaction](t, res)
	if len(hotel.Tags) != 1 || hotel.Tags[0].Color != "#00ff00" {
		t.Fatalf("tags na transação: %+v", hotel.Tags)
	}
	expectIssue(t, doReq(t, srv, "POST", "/api/transactions", map[string]any{
		"type": "EXPENSE", "description": "X", "amountCents": 1, "date": today, "accountId": a.ID, "tagIds": []string{"x"},
	}, nil), "tagIds", "tagIds inválido")
	if raw := doReq(t, srv, "GET", "/api/transactions?pageSize=5", nil, nil).Body.String(); !strings.Contains(raw, `"tags":[`) {
		t.Fatalf("listagem sem tags: %s", raw)
	}
	page := decodeBody[domain.Paginated[domain.Transaction]](t, doReq(t, srv, "GET", "/api/transactions?tagId="+tag.ID, nil, nil))
	if page.Total != 1 {
		t.Fatalf("filtro tagId: %+v", page)
	}
	expectIssue(t, doReq(t, srv, "GET", "/api/transactions?tagId=abc", nil, nil), "tagId", "tagId inválido")
	upd := decodeBody[domain.Transaction](t, doReq(t, srv, "PATCH", "/api/transactions/"+hotel.ID, map[string]any{"tagIds": []string{}}, nil))
	if len(upd.Tags) != 0 {
		t.Fatalf("tagIds [] deveria limpar: %+v", upd.Tags)
	}
	_ = doReq(t, srv, "PATCH", "/api/transactions/"+hotel.ID, map[string]any{"tagIds": []string{tag.ID}}, nil)
	spend := decodeBody[[]domain.TagSpending](t, doReq(t, srv, "GET", "/api/insights/tags", nil, nil))
	if len(spend) != 1 || spend[0].TotalCents != 20000 || spend[0].Count != 1 {
		t.Fatalf("insights/tags: %+v", spend)
	}
	tags := decodeBody[[]domain.Tag](t, doReq(t, srv, "GET", "/api/tags", nil, nil))
	if len(tags) != 1 || tags[0].UsageCount != 1 {
		t.Fatalf("GET /api/tags: %+v", tags)
	}

	// ───── Assinaturas
	for i := 2; i >= 0; i-- {
		date := domain.AddMonthsToDateString(today, -i)
		expectCode(t, doReq(t, srv, "POST", "/api/transactions", map[string]any{
			"type": "EXPENSE", "description": "Spotify", "amountCents": 2190, "date": date, "accountId": a.ID,
		}, nil), 201, "lançar assinatura")
	}
	subs := decodeBody[domain.SubscriptionsView](t, doReq(t, srv, "GET", "/api/insights/subscriptions", nil, nil))
	if len(subs.Items) != 1 || subs.Items[0].Key != "spotify" || subs.MonthlyTotalCents != 2190 {
		t.Fatalf("assinaturas: %+v", subs)
	}
	res = doReq(t, srv, "POST", "/api/insights/subscriptions/spotify/track", nil, nil)
	expectCode(t, res, 201, "acompanhar assinatura")
	if rec := decodeBody[domain.Recurring](t, res); !rec.IsSubscription {
		t.Fatalf("recorrência da assinatura: %+v", rec)
	}
	expectCode(t, doReq(t, srv, "POST", "/api/insights/subscriptions/spotify/track", nil, nil), 409, "acompanhar 2x")
	expectCode(t, doReq(t, srv, "POST", "/api/insights/subscriptions/spotify/dismiss", nil, nil), 204, "ignorar")
	expectCode(t, doReq(t, srv, "POST", "/api/insights/subscriptions/nada/track", nil, nil), 404, "assinatura inexistente")

	// ───── Alertas
	alerts := decodeBody[[]domain.Alert](t, doReq(t, srv, "GET", "/api/alerts", nil, nil))
	for _, al := range alerts {
		if al.ID == "" || al.Kind == "" || al.Href == "" {
			t.Fatalf("alerta incompleto: %+v", al)
		}
	}
	expectCode(t, doReq(t, srv, "POST", "/api/alerts/"+url.PathEscape("PENDING_OLD:pending:"+today[:7])+"/dismiss", nil, nil), 204, "dispensar alerta")
	expectCode(t, doReq(t, srv, "POST", "/api/alerts/x/dismiss", nil, nil), 400, "id de alerta inválido")

	// ───── Ritmo/spendToday na visão geral
	if raw := doReq(t, srv, "GET", "/api/insights/overview", nil, nil).Body.String(); !strings.Contains(raw, `"series":[{"day":1,`) ||
		!strings.Contains(raw, `"projectedMonthCents":`) || !strings.Contains(raw, `"spendToday":{"cents":`) {
		t.Fatalf("overview v3: %.400s", raw)
	}

	// ───── Fluxo de caixa
	cf := decodeBody[domain.CashflowView](t, doReq(t, srv, "GET", "/api/insights/cashflow?months=2&basis=ACCRUAL", nil, nil))
	if cf.Basis != "ACCRUAL" || len(cf.Months) != 2 || cf.WhereItWent == nil {
		t.Fatalf("cashflow: %+v", cf)
	}
	expectIssue(t, doReq(t, srv, "GET", "/api/insights/cashflow?months=13", nil, nil), "months", "months > 12")
	expectIssue(t, doReq(t, srv, "GET", "/api/insights/cashflow?basis=OUTRO", nil, nil), "basis", "basis inválido")

	// ───── Composição da fatura
	card := decodeBody[domain.Card](t, doReq(t, srv, "POST", "/api/cards", map[string]any{"name": "Cartão", "brand": "VISA", "limitCents": 500000, "closingDay": 5, "dueDay": 12}, nil))
	expectCode(t, doReq(t, srv, "POST", "/api/cards/"+card.ID+"/expenses", map[string]any{"description": "TV", "totalCents": 30000, "date": today, "installments": 3, "tagIds": []string{tag.ID}}, nil), 201, "compra parcelada com tag")
	invs := decodeBody[[]domain.Invoice](t, doReq(t, srv, "GET", "/api/cards/"+card.ID+"/invoices", nil, nil))
	bd := decodeBody[domain.InvoiceBreakdown](t, doReq(t, srv, "GET", "/api/cards/"+card.ID+"/invoices/"+invs[len(invs)-1].ID+"/breakdown", nil, nil))
	if bd.PurchasesCents != 10000 || bd.Composition.InstallmentsCents != 10000 {
		t.Fatalf("breakdown: %+v", bd)
	}
	expectCode(t, doReq(t, srv, "GET", "/api/cards/"+card.ID+"/invoices/"+missing+"/breakdown", nil, nil), 404, "fatura inexistente")

	// ───── Transferências inteligentes
	exp := decodeBody[domain.Transaction](t, doReq(t, srv, "POST", "/api/transactions", map[string]any{"type": "EXPENSE", "description": "PIX B", "amountCents": 7777, "date": today, "accountId": a.ID}, nil))
	inc := decodeBody[domain.Transaction](t, doReq(t, srv, "POST", "/api/transactions", map[string]any{"type": "INCOME", "description": "PIX A", "amountCents": 7777, "date": today, "accountId": b.ID}, nil))
	cands := decodeBody[[]domain.TransferCandidate](t, doReq(t, srv, "GET", "/api/transactions/transfer-candidates", nil, nil))
	if len(cands) != 1 || cands[0].Expense.ID != exp.ID || cands[0].Income.ID != inc.ID || cands[0].Expense.Tags == nil {
		t.Fatalf("candidatas: %+v", cands)
	}
	expectIssue(t, doReq(t, srv, "POST", "/api/transactions/merge-transfer", map[string]any{"expenseId": "x"}, nil), "incomeId", "merge inválido")
	res = doReq(t, srv, "POST", "/api/transactions/merge-transfer", map[string]any{"expenseId": exp.ID, "incomeId": inc.ID}, nil)
	expectCode(t, res, 200, "merge-transfer")
	if decodeBody[domain.TransferResult](t, res).TransferGroupID == "" {
		t.Fatal("merge sem transferGroupId")
	}

	// ───── Posso comprar?
	ca := decodeBody[domain.CanAffordResult](t, doReq(t, srv, "POST", "/api/insights/can-afford", map[string]any{"amountCents": 1000}, nil))
	if ca.Verdict != "YES" || ca.Message == "" || ca.LowestDate == "" {
		t.Fatalf("can-afford: %+v", ca)
	}
	expectIssue(t, doReq(t, srv, "POST", "/api/insights/can-afford", map[string]any{"amountCents": 0}, nil), "amountCents", "can-afford sem valor")
	expectCode(t, doReq(t, srv, "POST", "/api/insights/can-afford", map[string]any{"amountCents": 1, "cardId": missing}, nil), 404, "cartão inexistente")

	// ───── Prévia de link (upstream local, resolver e política de IP injetados)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<meta property="og:title" content="Cadeira"><meta property="product:price:amount" content="899.00">`))
	}))
	defer upstream.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	svcs.LinkPreview = &service.LinkPreviewer{
		Resolver: staticResolver{"loja.test": "127.0.0.1", "lan.test": "192.168.0.2"},
		AllowIP:  func(ip net.IP) bool { return ip.Equal(net.IPv4(127, 0, 0, 1)) },
	}
	lp := decodeBody[domain.LinkPreview](t, doReq(t, srv, "POST", "/api/wishlist/preview-link", map[string]any{"url": "http://loja.test:" + port + "/p"}, nil))
	if lp.Title == nil || *lp.Title != "Cadeira" || lp.PriceCents == nil || *lp.PriceCents != 89900 || lp.ImageURL != nil {
		t.Fatalf("preview-link: %+v", lp)
	}
	expectCode(t, doReq(t, srv, "POST", "/api/wishlist/preview-link", map[string]any{"url": "http://lan.test/"}, nil), 400, "host privado")
	expectIssue(t, doReq(t, srv, "POST", "/api/wishlist/preview-link", map[string]any{"url": "file:///etc/passwd"}, nil), "url", "esquema inválido")
	res = doReq(t, srv, "POST", "/api/wishlist", map[string]any{"name": "Cadeira", "priceCents": 89900, "imageUrl": "https://img.test/c.jpg"}, nil)
	expectCode(t, res, 201, "desejo com imagem")
	if w := decodeBody[domain.WishItem](t, res); w.ImageURL == nil {
		t.Fatalf("desejo sem imageUrl: %+v", w)
	}

	expectCode(t, doReq(t, srv, "DELETE", "/api/tags/"+tag.ID, nil, nil), 204, "excluir tag")
}
