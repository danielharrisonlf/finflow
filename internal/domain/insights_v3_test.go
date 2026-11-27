package domain

import (
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeSubscriptionDescription(t *testing.T) {
	cases := []struct{ in, want string }{
		{"NETFLIX.COM", "netflix com"},
		{"Netflix.com 12/03", "netflix com"},
		{"PAG*Spotify", "pag spotify"},
		{"Spotify AB P0A1B2C3", "spotify ab"},
		{"Academia São João (1/3)", "academia sao joao"},
		{"Curso parcela 2 de 10", "curso"},
		{"Amazon Prime 12/03/2026 #8812", "amazon prime"},
		{"123456", ""},
	}
	for _, c := range cases {
		if got := NormalizeSubscriptionDescription(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if k := SubscriptionKey("Netflix.com 12/03"); k != "netflix-com" {
		t.Fatalf("SubscriptionKey = %q", k)
	}
}

func subTx(desc, date string, cents int64) SubscriptionTx {
	return SubscriptionTx{Description: desc, Date: date, AmountCents: cents}
}

func TestDetectSubscriptions(t *testing.T) {
	today := "2026-11-27"
	cases := []struct {
		name        string
		txs         []SubscriptionTx
		wantCadence string // "" = não detecta
		wantPct     *float64
		wantNext    string
	}{
		{"mensal", []SubscriptionTx{
			subTx("NETFLIX.COM 0911", "2026-09-10", 5590), subTx("NETFLIX.COM 1011", "2026-10-10", 5590), subTx("NETFLIX.COM 1111", "2026-11-10", 5590),
		}, CadenceMonthly, nil, "2026-12-10"},
		{"mensal com aumento", []SubscriptionTx{
			subTx("Spotify", "2026-08-05", 2000), subTx("Spotify", "2026-09-05", 2000), subTx("Spotify", "2026-10-05", 2000), subTx("Spotify", "2026-11-05", 2300),
		}, CadenceMonthly, ptrF(15), "2026-12-05"},
		{"só duas mensais", []SubscriptionTx{subTx("Gym", "2026-10-01", 9900), subTx("Gym", "2026-11-01", 9900)}, "", nil, ""},
		{"semanal", []SubscriptionTx{
			subTx("Feira", "2026-11-02", 5000), subTx("Feira", "2026-11-09", 5100), subTx("Feira", "2026-11-16", 4900), subTx("Feira", "2026-11-23", 5000),
		}, CadenceWeekly, nil, "2026-11-30"},
		{"anual", []SubscriptionTx{subTx("Antivirus", "2025-03-01", 19900), subTx("Antivirus", "2026-03-01", 19900)}, CadenceYearly, nil, "2027-03-01"},
		{"valor instável", []SubscriptionTx{
			subTx("Mercado", "2026-09-10", 10000), subTx("Mercado", "2026-10-10", 30000), subTx("Mercado", "2026-11-10", 10000),
		}, "", nil, ""},
		{"cancelada (último há mais de 1,5 ciclo)", []SubscriptionTx{
			subTx("Disney", "2026-06-01", 3000), subTx("Disney", "2026-07-01", 3000), subTx("Disney", "2026-08-01", 3000),
		}, "", nil, ""},
		{"intervalo irregular", []SubscriptionTx{
			subTx("Uber", "2026-11-01", 3000), subTx("Uber", "2026-11-12", 3000), subTx("Uber", "2026-11-20", 3000),
		}, "", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectSubscriptions(c.txs, today)
			if c.wantCadence == "" {
				if len(got) != 0 {
					t.Fatalf("não deveria detectar: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Cadence != c.wantCadence || got[0].NextExpectedDate != c.wantNext {
				t.Fatalf("detecção: %+v", got)
			}
			if (c.wantPct == nil) != (got[0].PriceChangePct == nil) || (c.wantPct != nil && *c.wantPct != *got[0].PriceChangePct) {
				t.Fatalf("priceChangePct = %v, want %v", got[0].PriceChangePct, c.wantPct)
			}
			if got[0].Occurrences != len(c.txs) || got[0].LastCents != c.txs[len(c.txs)-1].AmountCents {
				t.Fatalf("ocorrências/último: %+v", got[0])
			}
		})
	}

	rec := "rec-1"
	withRec := []SubscriptionTx{subTx("Net", "2026-09-10", 100), subTx("Net", "2026-10-10", 100), {Description: "Net", Date: "2026-11-10", AmountCents: 100, RecurringID: &rec}}
	got := DetectSubscriptions(withRec, today)
	if len(got) != 1 || got[0].RecurringID == nil || *got[0].RecurringID != rec {
		t.Fatalf("recurringId: %+v", got)
	}
}

func ptrF(v float64) *float64 { return &v }

func TestMonthlyEquivalent(t *testing.T) {
	items := []DetectedSubscription{
		{Cadence: CadenceMonthly, AverageCents: 1000},
		{Cadence: CadenceWeekly, AverageCents: 1200},  // 1200*52/12 = 5200
		{Cadence: CadenceYearly, AverageCents: 12000}, // 1000
	}
	if got := SubscriptionsMonthlyTotal(items); got != 7200 {
		t.Fatalf("total mensal = %d", got)
	}
}

func TestBuildPaceSeries(t *testing.T) {
	cur := map[int]int64{1: 100, 3: 50}
	prev := map[int]int64{1: 10, 2: 20, 28: 5}
	s := BuildPaceSeries(cur, prev, 30, 28, 3)
	if len(s) != 30 {
		t.Fatalf("len = %d", len(s))
	}
	if *s[0].CurrentCents != 100 || *s[2].CurrentCents != 150 || s[3].CurrentCents != nil {
		t.Fatalf("série atual: %+v %+v %+v", s[0], s[2], s[3])
	}
	if s[1].PreviousCents != 30 || s[27].PreviousCents != 35 || s[29].PreviousCents != 35 {
		t.Fatalf("série anterior: %+v %+v", s[27], s[29])
	}
	if got := ProjectMonthSpend(1000, 10, 30, true); got != 3000 {
		t.Fatalf("projeção = %d", got)
	}
	if got := ProjectMonthSpend(1000, 10, 30, false); got != 1000 {
		t.Fatalf("mês passado = %d", got)
	}
	if SpendTodayFromBalance(-10, 5) != 0 || SpendTodayFromBalance(1000, 4) != 250 || SpendTodayFromBalance(1000, 0) != 0 {
		t.Fatal("SpendTodayFromBalance")
	}
}

func TestBuildCashflowBases(t *testing.T) {
	due := "2026-11-10"
	food := "cat-food"
	txs := []CashflowTx{
		{Income: true, Date: "2026-10-05", AmountCents: 500000},
		{Income: true, Date: "2026-11-05", AmountCents: 500000},
		{Date: "2026-11-01", AmountCents: 150000, FixedBill: true},
		{Date: "2026-10-20", DueDate: &due, AmountCents: 30000, CategoryID: &food, Name: strp("Alimentação")}, // cartão
		{Date: "2026-11-02", DueDate: strp("2026-12-10"), AmountCents: 20000, Installment: true},
		{Date: "2026-11-15", AmountCents: 10000},
	}
	months := []InvoicePeriod{{2026, 9}, {2026, 10}, {2026, 11}}
	cash := BuildCashflow(txs, months, CashflowBasisCash)
	if len(cash.Months) != 2 {
		t.Fatalf("meses: %+v", cash.Months)
	}
	nov := cash.Months[1]
	if nov.FixedBillsCents != 150000 || nov.DailyCents != 40000 || nov.InstallmentsCents != 0 || nov.ResultCents != 310000 {
		t.Fatalf("novembro (caixa): %+v", nov)
	}
	if cash.Months[0].ResultCents != 500000 || cash.Months[0].ResultChangePct != nil {
		t.Fatalf("outubro (caixa): %+v", cash.Months[0])
	}
	if nov.ResultChangePct == nil || *nov.ResultChangePct != -38 {
		t.Fatalf("variação: %v", nov.ResultChangePct)
	}
	acc := BuildCashflow(txs, months, CashflowBasisAccrual)
	novA := acc.Months[1]
	if novA.InstallmentsCents != 20000 || novA.DailyCents != 10000 || acc.Months[0].DailyCents != 30000 {
		t.Fatalf("competência: %+v %+v", acc.Months[0], novA)
	}
	// Para onde foi (último mês, caixa): fixa 150000 (sem categoria) + 30000 + 10000 sem categoria.
	if len(cash.WhereItWent) != 2 || cash.WhereItWent[0].Name != "Sem categoria" || cash.WhereItWent[0].TotalCents != 160000 || cash.WhereItWent[1].Percent != 15.8 {
		t.Fatalf("whereItWent: %+v", cash.WhereItWent)
	}
	if issues := ValidateCashflowQuery(CashflowQuery{Months: 13, Basis: "X"}); len(issues) != 2 {
		t.Fatalf("validação: %+v", issues)
	}
}

func TestBuildInvoiceBreakdown(t *testing.T) {
	three := 3
	lines := []InvoiceLine{
		{Description: "TV (1/3)", AmountCents: -30000, InstallmentTotal: &three},
		{Description: "NETFLIX.COM 1111", AmountCents: -5590},
		{Description: "Padaria", AmountCents: -1000},
		{Description: "Estorno padaria", AmountCents: 500},
	}
	b := BuildInvoiceBreakdown(lines, -36090, map[string]bool{"netflix-com": true})
	if b.PurchasesCents != 36590 || b.CreditsCents != 500 || b.TotalCents != 36090 || b.PaymentsCents != 36090 {
		t.Fatalf("breakdown: %+v", b)
	}
	if b.Composition.InstallmentsCents != 30000 || b.Composition.SubscriptionsCents != 5590 || b.Composition.OneOffCents != 1000 {
		t.Fatalf("composição: %+v", b.Composition)
	}
}

func TestPairTransferCandidates(t *testing.T) {
	a, b, c := "acc-a", "acc-b", "acc-c"
	tx := func(id, date, acc string, cents int64) Transaction {
		acc2 := acc
		return Transaction{ID: id, Date: date, AccountID: &acc2, AmountCents: cents}
	}
	expenses := []Transaction{
		tx("e1", "2026-11-10", a, -10000),
		tx("e2", "2026-11-12", a, -5000),
		tx("e3", "2026-11-20", b, -7000), // mesma conta da receita i3
	}
	incomes := []Transaction{
		tx("i1", "2026-11-12", b, 10000), // 2 dias de e1
		tx("i2", "2026-11-10", c, 10000), // 0 dias de e1 → vence
		tx("i3", "2026-11-20", b, 7000),
		tx("i4", "2026-11-20", c, 5000), // 8 dias de e2 → fora
	}
	got := PairTransferCandidates(expenses, incomes)
	if len(got) != 1 || got[0].Expense.ID != "e1" || got[0].Income.ID != "i2" || got[0].DaysApart != 0 {
		t.Fatalf("pares: %+v", got)
	}
}

func TestCardPurchaseEventsAndVerdict(t *testing.T) {
	// Fecha dia 5, vence dia 12: compra em 10/11 cai na fatura de 12 e vence 12/12.
	evs, first := CardPurchaseEvents("TV", 100000, 3, "2026-11-10", 5, 12)
	if len(evs) != 3 || first != 33334 || evs[0].Date != "2026-12-12" || evs[2].Date != "2027-02-12" || evs[0].AmountCents != -33334 || evs[1].Description != "TV (2/3)" {
		t.Fatalf("eventos: %+v %d", evs, first)
	}
	// Vencimento antes do fechamento: fatura de dezembro vence em janeiro.
	evs, _ = CardPurchaseEvents("X", 1000, 1, "2026-11-28", 25, 5)
	if evs[0].Date != "2027-01-05" {
		t.Fatalf("vencimento mês seguinte: %+v", evs)
	}
	bands := DefaultBalanceBands()
	cases := []struct {
		low    int64
		limit  bool
		expect string
	}{
		{-1, false, VerdictNo},
		{50000, false, VerdictCareful},
		{150000, true, VerdictCareful},
		{150000, false, VerdictYes},
	}
	for _, c := range cases {
		if got := CanAffordVerdict(c.low, bands, c.limit); got != c.expect {
			t.Errorf("verdict(%d,%v) = %s", c.low, c.limit, got)
		}
	}
	fits := "2027-01-05"
	msg := CanAffordMessage(CanAffordResult{Verdict: VerdictNo, LowestBalanceAfterCents: -100, LowestDate: "2026-12-01", FitsOn: &fits}, 5000, bands)
	if !strings.Contains(msg, "Caberia a partir de 05/01/2027") {
		t.Fatalf("mensagem: %s", msg)
	}
	low := LowestBalance([]ProjectionDay{{Date: "a", BalanceCents: 5}, {Date: "b", BalanceCents: 1}, {Date: "c", BalanceCents: 1}})
	if low.Date != "b" {
		t.Fatalf("LowestBalance: %+v", low)
	}
}

func TestBuildAlerts(t *testing.T) {
	today := "2026-11-10"
	limit := int64(100000)
	pct := 12.5
	cat := "cat-1"
	in := AlertInput{
		Today: today,
		Budgets: []AlertBudget{
			{CategoryID: "b1", Name: "Mercado", Percent: 95, SpentCents: 950, AmountCents: 1000},
			{CategoryID: "b2", Name: "Lazer", Percent: 75},
			{CategoryID: "b3", Name: "Casa", Percent: 50},
		},
		LimitCents:       &limit,
		MonthSpentCents:  50000, // 50000/10*30 = 150000 > 100000
		Pace:             InsightsPace{Status: "ABOVE", CurrentCents: 1200, PreviousSamePeriodCents: 1000, DiffCents: 200, DayOfMonth: 10},
		ProjectionLowest: &ProjectionLowest{Date: "2026-11-20", BalanceCents: -500},
		Bills:            []AlertDue{{Ref: "r1", Description: "Aluguel", DueDate: "2026-11-12", AmountCents: 150000}, {Ref: "r2", Description: "Luz", DueDate: "2026-11-20"}},
		Invoices:         []AlertDue{{Ref: "inv1", Description: "Fatura X", DueDate: "2026-11-09"}, {Ref: "inv0", Description: "Fatura velha", DueDate: "2026-10-01"}},
		Loans:            []AlertDue{{Ref: "l1", Description: "Carro", DueDate: "2026-11-13"}},
		Subscriptions: []DetectedSubscription{
			{Key: "spotify", Description: "Spotify", PriceChangePct: &pct, Occurrences: 5, LastDate: "2026-11-05"},
			{Key: "netflix", Description: "Netflix", Occurrences: 3, LastDate: "2026-11-03"},
		},
		CutSuggestions: []CutSuggestion{{Kind: "ABOVE_AVERAGE", CategoryID: &cat, Name: "Delivery"}, {Kind: "CANCEL", Name: "TV"}},
		Reserve:        &ReserveView{AvgMonthlyCostCents: 100000, MonthsCovered: 1.5},
		Goals:          []AlertGoal{{ID: "g1", Name: "Viagem", MonthlyNeededCents: 50000, Last30DaysCents: 1000}, {ID: "g2", Name: "Ok", MonthlyNeededCents: 100, Last30DaysCents: 100}},
		PendingDates:   []string{"2026-10-01", "2026-11-09"},
		Uncategorized:  5,
		Investments:    []AlertInvestment{{ID: "i1", Name: "CDB", ValueUpdatedAt: "2026-09-01"}, {ID: "i2", Name: "Tesouro", ValueUpdatedAt: "2026-11-01"}},
	}
	alerts := BuildAlerts(in)
	kinds := map[string]int{}
	for _, a := range alerts {
		kinds[a.Kind]++
		if !strings.HasSuffix(a.ID, ":2026-11") || a.Title == "" || a.Message == "" || a.Href == "" || a.Date == "" {
			t.Fatalf("alerta incompleto: %+v", a)
		}
	}
	want := map[string]int{
		AlertBudgetDanger: 1, AlertBudgetWarning: 1, AlertLimitPace: 1, AlertPaceAbove: 1, AlertNegativeProjection: 1,
		AlertBillDue: 1, AlertInvoiceDue: 1, AlertLoanDue: 1, AlertSubscriptionPriceUp: 1, AlertNewSubscription: 1,
		AlertCategoryAboveAverage: 1, AlertReserveLow: 1, AlertGoalBehind: 1, AlertPendingOld: 1, AlertUncategorized: 1,
		AlertInvestmentStale: 1,
	}
	for k, n := range want {
		if kinds[k] != n {
			t.Errorf("%s: %d alertas, want %d (%+v)", k, kinds[k], n, kinds)
		}
	}
	if alerts[0].Severity != SeverityDanger || alerts[len(alerts)-1].Severity != SeverityInfo {
		t.Fatalf("ordem por severidade: %+v", alerts)
	}
	if alerts[0].ID != "BUDGET_DANGER:b1:2026-11" {
		t.Fatalf("primeiro alerta (DANGER mais antigo): %+v", alerts[0])
	}
	// Ritmo só 5% acima não alerta; sem limite não há LIMIT_PACE.
	quiet := BuildAlerts(AlertInput{Today: today, Pace: InsightsPace{Status: "ABOVE", PreviousSamePeriodCents: 1000, DiffCents: 50}, MonthSpentCents: 50000})
	if len(quiet) != 0 {
		t.Fatalf("não deveria alertar: %+v", quiet)
	}

	pruned := PruneDismissed(map[string]string{"a": "2026-11-01", "b": "2026-07-01"}, today)
	if len(pruned) != 1 || pruned["a"] == "" {
		t.Fatalf("PruneDismissed: %+v", pruned)
	}
}

func TestParseLinkPreview(t *testing.T) {
	base, _ := url.Parse("https://loja.example/produto/1")
	cases := []struct {
		name, body     string
		title, img     string
		price          int64
		wantNoPrice    bool
		wantNoImageURL bool
	}{
		{"og", `<html><head><meta property="og:title" content="Fone &amp; Cia"><meta property="product:price:amount" content="1299.90"><meta property='og:image' content='/img/f.jpg'><title>Ignorado</title></head></html>`,
			"Fone & Cia", "https://loja.example/img/f.jpg", 129990, false, false},
		{"title + json-ld", `<title>
			Cadeira   Gamer </title><script type="application/ld+json">{"@context":"https://schema.org","@type":"Product","name":"Cadeira","offers":{"@type":"Offer","price":"899,00","priceCurrency":"BRL"}}</script>`,
			"Cadeira Gamer", "", 89900, false, true},
		{"json-ld graph array", `<title>X</title><script type="application/ld+json">{"@graph":[{"@type":"WebPage"},{"@type":"Product","offers":[{"price":49.9}]}]}</script>`,
			"X", "", 4990, false, true},
		{"nada", `<p>sem metadados</p>`, "", "", 0, true, true},
		{"imagem javascript", `<meta content="javascript:alert(1)" property="og:image"><meta name="twitter:title" content="T">`, "T", "", 0, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ParseLinkPreview(c.body, base)
			if c.title == "" && p.Title != nil || c.title != "" && (p.Title == nil || *p.Title != c.title) {
				t.Fatalf("title = %v", p.Title)
			}
			if c.wantNoPrice != (p.PriceCents == nil) || (!c.wantNoPrice && *p.PriceCents != c.price) {
				t.Fatalf("price = %v", p.PriceCents)
			}
			if c.wantNoImageURL != (p.ImageURL == nil) || (!c.wantNoImageURL && *p.ImageURL != c.img) {
				t.Fatalf("image = %v", p.ImageURL)
			}
		})
	}
	prices := map[string]int64{"1299.90": 129990, "1.299,90": 129990, "R$ 49,90": 4990, "1299": 129900}
	for raw, want := range prices {
		if got, ok := ParsePriceCents(raw); !ok || got != want {
			t.Errorf("ParsePriceCents(%q) = %d %v", raw, got, ok)
		}
	}
	for _, bad := range []string{"", "abc", "-5", "0"} {
		if _, ok := ParsePriceCents(bad); ok {
			t.Errorf("ParsePriceCents(%q) deveria falhar", bad)
		}
	}
	if ValidateLinkPreview(LinkPreviewInput{URL: "ftp://x"}) == nil || ValidateLinkPreview(LinkPreviewInput{URL: "https://"}) == nil || ValidateLinkPreview(LinkPreviewInput{URL: "https://a.b/c"}) != nil {
		t.Fatal("ValidateLinkPreview")
	}
}

func TestValidateTagsAndMatchRule(t *testing.T) {
	if len(ValidateCreateTag(CreateTagInput{Name: "  "})) != 1 || len(ValidateCreateTag(CreateTagInput{Name: "Viagem", Color: strp("red")})) != 1 {
		t.Fatal("ValidateCreateTag")
	}
	if len(ValidateTagIDs([]string{"x"})) != 1 || ValidateTagIDs(nil) != nil {
		t.Fatal("ValidateTagIDs")
	}
	tag := "tag-1"
	rules := []RuleCandidate{{Pattern: "uber", CategoryID: "c1", CategoryKind: CategoryExpense, TagID: &tag}}
	r := MatchRule(rules, "UBER *TRIP", CategoryExpense)
	if r == nil || r.TagID == nil || *r.TagID != tag {
		t.Fatalf("MatchRule: %+v", r)
	}
	if got := UniqueStrings([]string{"a", "b", "a"}); len(got) != 2 {
		t.Fatalf("UniqueStrings: %v", got)
	}
}
