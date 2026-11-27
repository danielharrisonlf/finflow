package httpapi_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/spaces"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestSpacesAPI(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "")
	base := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	reg := spaces.New(base, spaces.Options{DataDir: t.TempDir()})
	t.Cleanup(func() { _ = reg.Close() })
	srv := httpapi.NewServer(base, httpapi.ServerOptions{Spaces: reg})
	casaH := map[string]string{httpapi.SpaceHeader: "casa"}

	list := decodeBody[[]domain.Space](t, doReq(t, srv, "GET", "/api/spaces", nil, nil))
	if len(list) != 1 || list[0].Slug != "pessoal" || !list[0].IsDefault {
		t.Fatalf("espaços iniciais: %+v", list)
	}
	expectIssue(t, doReq(t, srv, "POST", "/api/spaces", map[string]any{"name": "", "kind": "HOME"}, nil), "name", "nome vazio")
	expectIssue(t, doReq(t, srv, "POST", "/api/spaces", map[string]any{"name": "X", "kind": "HOME", "slug": "../x"}, nil), "slug", "slug inválido")
	res := doReq(t, srv, "POST", "/api/spaces", map[string]any{"name": "Casa", "kind": "HOME"}, nil)
	expectCode(t, res, 201, "criar casa")
	casa := decodeBody[domain.Space](t, res)
	if casa.Slug != "casa" || casa.Kind != domain.SpaceHome || casa.IsDefault {
		t.Fatalf("casa: %+v", casa)
	}
	expectCode(t, doReq(t, srv, "POST", "/api/spaces", map[string]any{"name": "Outra", "kind": "HOME", "slug": "casa"}, nil), 409, "slug repetido")
	biz := decodeBody[domain.Space](t, doReq(t, srv, "POST", "/api/spaces", map[string]any{"name": "Empresa", "kind": "BUSINESS", "color": "#123456"}, nil))
	if biz.Slug != "empresa" || biz.Color != "#123456" {
		t.Fatalf("empresa: %+v", biz)
	}

	// Cada espaço é um banco separado, escolhido pelo cabeçalho (ou ?space= em links).
	expectCode(t, doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "Conta da casa", "type": "CHECKING", "initialBalanceCents": 50000}, casaH), 201, "conta na casa")
	expectCode(t, doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "Pessoal", "type": "CHECKING", "initialBalanceCents": 20000}, nil), 201, "conta pessoal")
	if got := decodeBody[[]domain.Account](t, doReq(t, srv, "GET", "/api/accounts", nil, casaH)); len(got) != 1 || got[0].Name != "Conta da casa" {
		t.Fatalf("contas da casa: %+v", got)
	}
	if got := decodeBody[[]domain.Account](t, doReq(t, srv, "GET", "/api/accounts", nil, map[string]string{httpapi.SpaceHeader: "pessoal"})); len(got) != 1 || got[0].Name != "Pessoal" {
		t.Fatalf("contas do pessoal: %+v", got)
	}
	if got := decodeBody[[]domain.Account](t, doReq(t, srv, "GET", "/api/accounts?space=casa", nil, nil)); len(got) != 1 || got[0].Name != "Conta da casa" {
		t.Fatalf("contas via ?space=: %+v", got)
	}
	expectCode(t, doReq(t, srv, "GET", "/api/accounts", nil, map[string]string{httpapi.SpaceHeader: "nao-existe"}), 404, "espaço desconhecido")
	expectCode(t, doReq(t, srv, "GET", "/api/accounts", nil, map[string]string{httpapi.SpaceHeader: "../casa"}), 404, "slug malicioso")
	// Rotas globais não dependem do espaço (um cabeçalho velho não trava a troca de espaço).
	expectCode(t, doReq(t, srv, "GET", "/api/spaces", nil, map[string]string{httpapi.SpaceHeader: "nao-existe"}), 200, "listar espaços com cabeçalho velho")
	expectCode(t, doReq(t, srv, "GET", "/api/system/info", nil, map[string]string{httpapi.SpaceHeader: "nao-existe"}), 200, "system info")
	cats := decodeBody[[]domain.Category](t, doReq(t, srv, "GET", "/api/categories?kind=EXPENSE", nil, map[string]string{httpapi.SpaceHeader: "empresa"}))
	found := false
	for _, c := range cats {
		if c.Name == "Impostos sobre vendas" && c.DREGroup != nil && *c.DREGroup == domain.DREDeductions {
			found = true
		}
	}
	if !found {
		t.Fatalf("categorias de empresa: %+v", cats)
	}

	// CORS libera o cabeçalho do espaço.
	req := httptest.NewRequest("OPTIONS", "/api/accounts", nil)
	req.Header.Set("Origin", "http://localhost:5273")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if !strings.Contains(rr.Header().Get("Access-Control-Allow-Headers"), httpapi.SpaceHeader) {
		t.Fatalf("CORS: %q", rr.Header().Get("Access-Control-Allow-Headers"))
	}

	// MCP via HTTP respeita o cabeçalho.
	call := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_accounts","arguments":{}}}`)
	if body := doReq(t, srv, "POST", "/api/mcp", call, casaH).Body.String(); !strings.Contains(body, "Conta da casa") || strings.Contains(body, `\"Pessoal\"`) {
		t.Fatalf("MCP no espaço casa: %s", body)
	}

	ov := decodeBody[domain.SpacesOverview](t, doReq(t, srv, "GET", "/api/spaces/overview", nil, nil))
	if len(ov.Spaces) != 3 || ov.Totals.BalanceCents != 70000 || ov.Spaces[1].Slug != "casa" || ov.Spaces[1].BalanceCents != 50000 {
		t.Fatalf("visão unificada: %+v", ov)
	}
	var rawOV map[string]any
	_ = json.Unmarshal(doReq(t, srv, "GET", "/api/spaces/overview", nil, nil).Body.Bytes(), &rawOV)
	first := rawOV["spaces"].([]any)[0].(map[string]any)
	for _, k := range []string{"slug", "name", "kind", "color", "balanceCents", "netWorthCents", "monthIncomeCents", "monthOutCents", "monthResultCents"} {
		if _, ok := first[k]; !ok {
			t.Errorf("visão unificada sem %q: %v", k, first)
		}
	}

	res = doReq(t, srv, "PATCH", "/api/spaces/casa", map[string]any{"name": "Casa da Praia", "color": "#00ff00"}, nil)
	expectCode(t, res, 200, "renomear")
	if got := decodeBody[domain.Space](t, res); got.Name != "Casa da Praia" || got.Color != "#00ff00" {
		t.Fatalf("renomear: %+v", got)
	}
	expectCode(t, doReq(t, srv, "PATCH", "/api/spaces/pessoal", map[string]any{"name": "Eu"}, nil), 200, "renomear pessoal")
	expectCode(t, doReq(t, srv, "PATCH", "/api/spaces/nao-existe", map[string]any{"name": "X"}, nil), 404, "renomear inexistente")
	expectCode(t, doReq(t, srv, "DELETE", "/api/spaces/pessoal?confirm=pessoal", nil, nil), 409, "apagar pessoal")
	expectCode(t, doReq(t, srv, "DELETE", "/api/spaces/casa", nil, nil), 400, "apagar sem confirmação")
	expectCode(t, doReq(t, srv, "DELETE", "/api/spaces/casa?confirm=casa", nil, nil), 204, "apagar casa")
	expectCode(t, doReq(t, srv, "GET", "/api/accounts", nil, casaH), 404, "espaço apagado")
	if got := decodeBody[[]domain.Space](t, doReq(t, srv, "GET", "/api/spaces", nil, nil)); len(got) != 2 || got[0].Name != "Eu" {
		t.Fatalf("espaços depois de apagar: %+v", got)
	}

	// Sem pasta de dados (banco em memória / PostgreSQL): só o pessoal e 501 ao criar.
	single := httpapi.NewServer(base, httpapi.ServerOptions{})
	res = doReq(t, single, "POST", "/api/spaces", map[string]any{"name": "Casa", "kind": "HOME"}, nil)
	expectCode(t, res, 501, "criar sem suporte")
	if !strings.Contains(res.Body.String(), "não estão disponíveis") {
		t.Fatalf("mensagem 501: %s", res.Body.String())
	}
	if got := decodeBody[domain.SpacesOverview](t, doReq(t, single, "GET", "/api/spaces/overview", nil, nil)); len(got.Spaces) != 1 {
		t.Fatalf("visão unificada sem suporte: %+v", got)
	}
}

func TestHomeAndBusinessAPI(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		t.Setenv("FINFLOW_PASSWORD", "")
		srv := httpapi.NewServer(service.New(store.New(database), nil), httpapi.ServerOptions{})
		today := domain.TodayString()
		missing := "11111111-1111-4111-8111-111111111111"

		acc := decodeBody[domain.Account](t, doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "Conta da casa", "type": "CHECKING", "initialBalanceCents": 100000}, nil))
		acc2 := decodeBody[domain.Account](t, doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "Conta do Bruno", "type": "CHECKING"}, nil))

		// ───── Membros
		ana := decodeBody[domain.Member](t, doReq(t, srv, "POST", "/api/members", map[string]any{"name": "Ana", "color": "#ec4899"}, nil))
		res := doReq(t, srv, "POST", "/api/members", map[string]any{"name": "Bruno"}, nil)
		expectCode(t, res, 201, "criar Bruno")
		bruno := decodeBody[domain.Member](t, res)
		if bruno.Color != domain.DefaultMemberColor {
			t.Fatalf("cor padrão: %+v", bruno)
		}
		expectCode(t, doReq(t, srv, "POST", "/api/members", map[string]any{"name": "ana"}, nil), 409, "nome repetido")
		expectIssue(t, doReq(t, srv, "POST", "/api/members", map[string]any{"name": "Caio", "color": "azul"}, nil), "color", "cor inválida")
		caio := decodeBody[domain.Member](t, doReq(t, srv, "POST", "/api/members", map[string]any{"name": "Caio"}, nil))
		res = doReq(t, srv, "PATCH", "/api/members/"+caio.ID, map[string]any{"name": "Caio R."}, nil)
		expectCode(t, res, 200, "renomear membro")
		expectCode(t, doReq(t, srv, "DELETE", "/api/members/"+caio.ID, nil, nil), 204, "apagar membro")
		expectCode(t, doReq(t, srv, "DELETE", "/api/members/"+missing, nil, nil), 404, "apagar membro inexistente")
		if got := decodeBody[[]domain.Member](t, doReq(t, srv, "GET", "/api/members", nil, nil)); len(got) != 2 || got[0].ID != ana.ID {
			t.Fatalf("membros: %+v", got)
		}

		// ───── Despesas divididas
		expectIssue(t, doReq(t, srv, "POST", "/api/transactions", map[string]any{
			"type": "EXPENSE", "description": "X", "amountCents": 1000, "date": today, "accountId": acc.ID,
			"splits": []map[string]any{{"memberId": ana.ID, "sharePct": 60}, {"memberId": bruno.ID, "sharePct": 30}},
		}, nil), "splits", "soma ≠ 100")
		expectIssue(t, doReq(t, srv, "POST", "/api/transactions", map[string]any{
			"type": "INCOME", "description": "X", "amountCents": 1000, "date": today, "accountId": acc.ID, "paidByMemberId": ana.ID,
		}, nil), "paidByMemberId", "receita com quem pagou")
		expectCode(t, doReq(t, srv, "POST", "/api/transactions", map[string]any{
			"type": "EXPENSE", "description": "X", "amountCents": 1000, "date": today, "accountId": acc.ID, "paidByMemberId": missing,
		}, nil), 404, "membro inexistente")
		res = doReq(t, srv, "POST", "/api/transactions", map[string]any{
			"type": "EXPENSE", "description": "Mercado", "amountCents": 10001, "date": today, "accountId": acc.ID,
			"paidByMemberId": ana.ID, "splits": []map[string]any{{"memberId": ana.ID, "sharePct": 60}, {"memberId": bruno.ID, "sharePct": 40}},
		}, nil)
		expectCode(t, res, 201, "despesa 60/40")
		market := decodeBody[domain.Transaction](t, res)
		if market.PaidByMemberID == nil || *market.PaidByMemberID != ana.ID || len(market.Splits) != 2 || market.Splits[0].SharePct != 60 {
			t.Fatalf("despesa 60/40: %+v", market)
		}
		plain := decodeBody[domain.Transaction](t, doReq(t, srv, "POST", "/api/transactions", map[string]any{
			"type": "EXPENSE", "description": "Padaria", "amountCents": 500, "date": today, "accountId": acc.ID,
		}, nil))
		var rawTx map[string]any
		_ = json.Unmarshal(doReq(t, srv, "GET", "/api/transactions/"+plain.ID, nil, nil).Body.Bytes(), &rawTx)
		if splits, ok := rawTx["splits"].([]any); !ok || len(splits) != 0 || rawTx["paidByMemberId"] != nil {
			t.Fatalf("transação sem divisão deve ter splits [] e paidByMemberId null: %v", rawTx)
		}

		card := decodeBody[domain.Card](t, doReq(t, srv, "POST", "/api/cards", map[string]any{"name": "Cartão", "brand": "VISA", "limitCents": 500000, "closingDay": 28, "dueDay": 5}, nil))
		res = doReq(t, srv, "POST", "/api/cards/"+card.ID+"/expenses", map[string]any{
			"description": "Sofá", "totalCents": 30000, "date": today, "installments": 2, "paidByMemberId": bruno.ID,
		}, nil)
		expectCode(t, res, 201, "compra parcelada dividida")
		items := decodeBody[domain.Paginated[domain.Transaction]](t, doReq(t, srv, "GET", "/api/transactions?cardId="+card.ID, nil, nil))
		if len(items.Items) != 2 || items.Items[0].PaidByMemberID == nil || items.Items[1].PaidByMemberID == nil {
			t.Fatalf("parcelas com quem pagou: %+v", items.Items)
		}

		// PATCH: limpar a divisão volta à divisão igual; quem pagou só em despesas.
		res = doReq(t, srv, "PATCH", "/api/transactions/"+market.ID, map[string]any{"splits": nil}, nil)
		expectCode(t, res, 200, "limpar divisão")
		if got := decodeBody[domain.Transaction](t, res); len(got.Splits) != 0 || got.PaidByMemberID == nil {
			t.Fatalf("limpar divisão: %+v", got)
		}
		res = doReq(t, srv, "PATCH", "/api/transactions/"+market.ID, map[string]any{"splits": []map[string]any{{"memberId": ana.ID, "sharePct": 60}, {"memberId": bruno.ID, "sharePct": 40}}}, nil)
		expectCode(t, res, 200, "voltar 60/40")
		income := decodeBody[domain.Transaction](t, doReq(t, srv, "POST", "/api/transactions", map[string]any{"type": "INCOME", "description": "Pix", "amountCents": 100, "date": today, "accountId": acc.ID}, nil))
		expectIssue(t, doReq(t, srv, "PATCH", "/api/transactions/"+income.ID, map[string]any{"paidByMemberId": ana.ID}, nil), "paidByMemberId", "quem pagou em receita")

		// ───── Saldos: Ana pagou 100,01 (60/40: Ana 60,01 / Bruno 40,00); Bruno pagou 150,00 da 1ª parcela (75/75).
		bal := decodeBody[domain.MemberBalances](t, doReq(t, srv, "GET", "/api/members/balances", nil, nil))
		if len(bal.Members) != 2 || bal.Members[0].PaidCents != 10001 || bal.Members[0].OwedCents != 13501 || bal.Members[0].BalanceCents != -3500 ||
			bal.Members[1].BalanceCents != 3500 || len(bal.Settlements) != 1 || bal.Settlements[0] != (domain.SettlementSuggestion{FromMemberID: ana.ID, ToMemberID: bruno.ID, AmountCents: 3500}) {
			t.Fatalf("saldos: %+v", bal)
		}
		expectIssue(t, doReq(t, srv, "GET", "/api/members/balances?month=13", nil, nil), "month", "mês inválido")

		expectIssue(t, doReq(t, srv, "POST", "/api/members/settle", map[string]any{"fromMemberId": ana.ID, "toMemberId": ana.ID, "amountCents": 100}, nil), "toMemberId", "acerto consigo mesmo")
		expectIssue(t, doReq(t, srv, "POST", "/api/members/settle", map[string]any{"fromMemberId": ana.ID, "toMemberId": bruno.ID, "amountCents": 100, "fromAccountId": acc.ID}, nil), "toAccountId", "só uma conta")
		res = doReq(t, srv, "POST", "/api/members/settle", map[string]any{"fromMemberId": ana.ID, "toMemberId": bruno.ID, "amountCents": 1500}, nil)
		expectCode(t, res, 201, "acerto registrado")
		if st := decodeBody[domain.Settlement](t, res); st.TransferGroupID != nil || st.Date != today || st.ID == "" {
			t.Fatalf("acerto: %+v", st)
		}
		res = doReq(t, srv, "POST", "/api/members/settle", map[string]any{"fromMemberId": ana.ID, "toMemberId": bruno.ID, "amountCents": 2000, "fromAccountId": acc.ID, "toAccountId": acc2.ID}, nil)
		expectCode(t, res, 201, "acerto com transferência")
		if st := decodeBody[domain.Settlement](t, res); st.TransferGroupID == nil {
			t.Fatalf("acerto com transferência: %+v", st)
		}
		if got := decodeBody[domain.Account](t, doReq(t, srv, "GET", "/api/accounts/"+acc2.ID, nil, nil)); got.BalanceCents != 2000 {
			t.Fatalf("transferência do acerto: %+v", got)
		}
		bal = decodeBody[domain.MemberBalances](t, doReq(t, srv, "GET", "/api/members/balances", nil, nil))
		if bal.Members[0].BalanceCents != 0 || bal.Members[1].BalanceCents != 0 || len(bal.Settlements) != 0 {
			t.Fatalf("saldos depois dos acertos: %+v", bal)
		}

		// ───── Empresa: grupo do DRE editável, recorrências desativadas e métricas
		sales := decodeBody[domain.Category](t, doReq(t, srv, "POST", "/api/categories", map[string]any{"name": "Vendas", "kind": "INCOME", "icon": "tag", "color": "#000000"}, nil))
		if sales.DREGroup == nil || *sales.DREGroup != domain.DRERevenue {
			t.Fatalf("grupo padrão de receita: %+v", sales)
		}
		tax := decodeBody[domain.Category](t, doReq(t, srv, "POST", "/api/categories", map[string]any{"name": "Simples Nacional", "kind": "EXPENSE", "icon": "tag", "color": "#000000"}, nil))
		if tax.DREGroup == nil || *tax.DREGroup != domain.DREDeductions {
			t.Fatalf("heurística: %+v", tax)
		}
		ads := decodeBody[domain.Category](t, doReq(t, srv, "POST", "/api/categories", map[string]any{"name": "Anúncios", "kind": "EXPENSE", "icon": "tag", "color": "#000000", "dreGroup": "COSTS"}, nil))
		if ads.DREGroup == nil || *ads.DREGroup != domain.DRECosts {
			t.Fatalf("grupo informado: %+v", ads)
		}
		expectIssue(t, doReq(t, srv, "PATCH", "/api/categories/"+ads.ID, map[string]any{"dreGroup": "LUCRO"}, nil), "dreGroup", "grupo inválido")
		res = doReq(t, srv, "PATCH", "/api/categories/"+ads.ID, map[string]any{"dreGroup": nil}, nil)
		expectCode(t, res, 200, "grupo de volta à heurística")
		if got := decodeBody[domain.Category](t, res); got.DREGroup == nil || *got.DREGroup != domain.DREOpex {
			t.Fatalf("grupo nulo: %+v", got)
		}

		twoMonthsAgo := domain.AddMonthsToDateString(domain.MonthStart(domain.Now().Year(), int(domain.Now().Month())), -2)
		alfa := decodeBody[domain.Recurring](t, doReq(t, srv, "POST", "/api/recurring", map[string]any{"description": "Cliente Alfa", "kind": "INCOME", "amountCents": 300000, "dayOfMonth": 5, "accountId": acc.ID, "categoryId": sales.ID, "startDate": twoMonthsAgo}, nil))
		beta := decodeBody[domain.Recurring](t, doReq(t, srv, "POST", "/api/recurring", map[string]any{"description": "Cliente Beta", "kind": "INCOME", "amountCents": 100000, "dayOfMonth": 5, "accountId": acc.ID, "startDate": twoMonthsAgo}, nil))
		if alfa.DeactivatedAt != nil {
			t.Fatalf("recorrência nova: %+v", alfa)
		}
		res = doReq(t, srv, "PATCH", "/api/recurring/"+beta.ID, map[string]any{"active": false}, nil)
		expectCode(t, res, 200, "desativar recorrência")
		if got := decodeBody[domain.Recurring](t, res); got.DeactivatedAt == nil || *got.DeactivatedAt != today {
			t.Fatalf("deactivatedAt: %+v", got)
		}
		m := decodeBody[domain.BusinessMetrics](t, doReq(t, srv, "GET", "/api/business/metrics", nil, nil))
		if m.MRRCents != 300000 || m.ARRCents != 3600000 || m.ChurnedMRRCents != 100000 || m.ChurnPct == nil || *m.ChurnPct != 25 || m.NewMRRCents != 0 {
			t.Fatalf("métricas: %+v", m)
		}
		if m.BurnCents != 0 || m.RunwayMonths != nil || m.CashCents == 0 {
			t.Fatalf("queima sem histórico: %+v", m)
		}
		var rawM map[string]any
		_ = json.Unmarshal(doReq(t, srv, "GET", "/api/business/metrics", nil, nil).Body.Bytes(), &rawM)
		for _, k := range []string{"mrrCents", "arrCents", "newMrrCents", "churnedMrrCents", "churnPct", "burnCents", "runwayMonths", "cashCents"} {
			if _, ok := rawM[k]; !ok {
				t.Errorf("métricas sem %q", k)
			}
		}
		expectIssue(t, doReq(t, srv, "GET", "/api/business/metrics?year=1999", nil, nil), "year", "ano inválido")

		expectCode(t, doReq(t, srv, "POST", "/api/transactions", map[string]any{"type": "INCOME", "description": "Venda", "amountCents": 100000, "date": today, "accountId": acc.ID, "categoryId": sales.ID}, nil), 201, "venda")
		expectCode(t, doReq(t, srv, "POST", "/api/transactions", map[string]any{"type": "EXPENSE", "description": "DAS", "amountCents": 6000, "date": today, "accountId": acc.ID, "categoryId": tax.ID}, nil), 201, "imposto")
		dre := decodeBody[domain.DRE](t, doReq(t, srv, "GET", "/api/business/dre", nil, nil))
		month := dre.Months[int(domain.Now().Month())-1]
		// Receita: Venda + Pix; despesas sem categoria (mercado, padaria, sofá) = OPEX.
		if len(dre.Months) != 12 || month.RevenueCents != 100100 || month.DeductionsCents != 6000 || month.NetRevenueCents != 94100 ||
			month.OpexCents != 10001+500+15000 || month.NetResultCents != 94100-25501 || dre.Totals.RevenueCents != 100100 {
			t.Fatalf("DRE do mês: %+v", month)
		}
		expectIssue(t, doReq(t, srv, "GET", "/api/business/dre?year=abc", nil, nil), "year", "ano inválido")
	})
}
