package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func mustAcc(t *testing.T, svcs *Services, name string, balance int64) domain.Account {
	t.Helper()
	a, err := svcs.CreateAccount(context.Background(), domain.CreateAccountInput{Name: name, Type: domain.AccountChecking, InitialBalanceCents: balance})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func mustExpense(t *testing.T, svcs *Services, accountID, desc string, cents int64, date string) domain.Transaction {
	t.Helper()
	tx, err := svcs.CreateTransaction(context.Background(), domain.CreateTransactionInput{
		Type: domain.TxExpense, Description: desc, AmountCents: cents, Date: date, AccountID: accountID,
	})
	if err != nil {
		t.Fatalf("despesa %s: %v", desc, err)
	}
	return tx
}

func TestTagsService(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		acc := mustAcc(t, svcs, "Conta", 100000)
		today := domain.TodayString()

		trip, err := svcs.CreateTag(ctx, domain.CreateTagInput{Name: " Viagem "})
		if err != nil || trip.Name != "Viagem" || trip.Color != domain.DefaultTagColor || trip.UsageCount != 0 {
			t.Fatalf("CreateTag: %+v %v", trip, err)
		}
		_, err = svcs.CreateTag(ctx, domain.CreateTagInput{Name: "VIAGEM"})
		assertStatus(t, err, http.StatusConflict)
		_, err = svcs.CreateTag(ctx, domain.CreateTagInput{Name: "x", Color: ptrTo("azul")})
		assertValidation(t, err, "color")
		work, _ := svcs.CreateTag(ctx, domain.CreateTagInput{Name: "Trabalho", Color: ptrTo("#123456")})

		_, err = svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "X", AmountCents: 1, Date: today, AccountID: acc.ID, TagIDs: []string{"11111111-1111-4111-8111-111111111111"}})
		assertStatus(t, err, http.StatusNotFound)
		tx, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "Hotel", AmountCents: 30000, Date: today, AccountID: acc.ID, TagIDs: []string{trip.ID, trip.ID}})
		if err != nil || len(tx.Tags) != 1 || tx.Tags[0].ID != trip.ID {
			t.Fatalf("transação com tag: %+v %v", tx, err)
		}
		var patch domain.UpdateTransactionInput
		patch.TagIDs = domain.Optional[[]string]{Set: true, Valid: true, Value: []string{work.ID}}
		upd, err := svcs.UpdateTransaction(ctx, tx.ID, patch)
		if err != nil || len(upd.Tags) != 1 || upd.Tags[0].ID != work.ID {
			t.Fatalf("substituir tags: %+v %v", upd, err)
		}
		patch.TagIDs = domain.Optional[[]string]{Set: true}
		if upd, err = svcs.UpdateTransaction(ctx, tx.ID, patch); err != nil || upd.Tags == nil || len(upd.Tags) != 0 {
			t.Fatalf("null remove todas as tags: %+v %v", upd.Tags, err)
		}

		card, _ := svcs.CreateCard(ctx, domain.CreateCardInput{Name: "Cartão", Brand: domain.BrandVisa, LimitCents: 1000000, ClosingDay: 5, DueDay: 12})
		if _, err := svcs.AddCardExpense(ctx, card.ID, domain.CreateCardExpenseInput{Description: "Passagem", TotalCents: 90000, Date: today, Installments: 3, TagIDs: []string{trip.ID}}); err != nil {
			t.Fatal(err)
		}
		list, err := svcs.ListTransactions(ctx, domain.ListTransactionsQuery{TagID: &trip.ID})
		if err != nil || list.Total != 3 {
			t.Fatalf("parcelas com tag: %+v %v", list, err)
		}

		// Regra com tag: aplicada ao lançar e no "aplicar regras".
		transp := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Transporte", Kind: domain.CategoryExpense})
		_, err = svcs.CreateCategoryRule(ctx, domain.CreateCategoryRuleInput{Pattern: "uber", CategoryID: transp.ID, TagID: ptrTo("11111111-1111-4111-8111-111111111111")})
		assertStatus(t, err, http.StatusNotFound)
		old := mustExpense(t, svcs, acc.ID, "Uber antigo", 1000, today)
		rule, err := svcs.CreateCategoryRule(ctx, domain.CreateCategoryRuleInput{Pattern: "uber", CategoryID: transp.ID, TagID: &work.ID})
		if err != nil || rule.TagID == nil || rule.TagName == nil || *rule.TagName != "Trabalho" {
			t.Fatalf("regra com tag: %+v %v", rule, err)
		}
		ride := mustExpense(t, svcs, acc.ID, "UBER *TRIP", 2500, today)
		if ride.CategoryID == nil || *ride.CategoryID != transp.ID || len(ride.Tags) != 1 || ride.Tags[0].ID != work.ID {
			t.Fatalf("regra deveria aplicar categoria e tag: %+v", ride)
		}
		res, err := svcs.ApplyCategoryRules(ctx, domain.ApplyRulesInput{})
		if err != nil || res.Updated != 1 {
			t.Fatalf("ApplyCategoryRules: %+v %v", res, err)
		}
		got, _ := svcs.GetTransaction(ctx, old.ID)
		if len(got.Tags) != 1 || got.Tags[0].ID != work.ID {
			t.Fatalf("aplicar regras deveria marcar a tag: %+v", got)
		}
		var rulePatch domain.UpdateCategoryRuleInput
		rulePatch.TagID = domain.Optional[string]{Set: true}
		if r, err := svcs.UpdateCategoryRule(ctx, rule.ID, rulePatch); err != nil || r.TagID != nil {
			t.Fatalf("limpar tag da regra: %+v %v", r, err)
		}

		spend, err := svcs.InsightsTags(ctx, domain.PeriodQuery{})
		if err != nil || len(spend) != 2 || spend[0].TagID != trip.ID || spend[0].TotalCents != 30000 || spend[1].TotalCents != 3500 {
			t.Fatalf("InsightsTags: %+v %v", spend, err)
		}
		var tagPatch domain.UpdateTagInput
		tagPatch.Name = domain.Optional[string]{Set: true, Valid: true, Value: "trabalho"}
		_, err = svcs.UpdateTag(ctx, trip.ID, tagPatch)
		assertStatus(t, err, http.StatusConflict)
		tagPatch.Name = domain.Optional[string]{Set: true, Valid: true, Value: "Férias"}
		if renamed, err := svcs.UpdateTag(ctx, trip.ID, tagPatch); err != nil || renamed.Name != "Férias" || renamed.UsageCount != 3 {
			t.Fatalf("renomear: %+v %v", renamed, err)
		}
		if err := svcs.DeleteTag(ctx, trip.ID); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, svcs.DeleteTag(ctx, trip.ID), http.StatusNotFound)
	})
}

func TestSubscriptionsService(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		acc := mustAcc(t, svcs, "Conta", 100000)
		streaming := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Streaming", Kind: domain.CategoryExpense, SpendType: ptrTo(domain.SpendCuttable)})
		today := domain.TodayString()
		for i := 2; i >= 0; i-- {
			date := domain.AddMonthsToDateString(today, -i)
			tx, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "NETFLIX.COM " + date[5:7], AmountCents: 5590, Date: date, AccountID: acc.ID, CategoryID: &streaming.ID})
			if err != nil {
				t.Fatal(err)
			}
			_ = tx
			mustExpense(t, svcs, acc.ID, "Spotify", 2190, date)
		}
		view, err := svcs.Subscriptions(ctx)
		if err != nil || len(view.Items) != 2 || view.MonthlyTotalCents != 5590+2190 {
			t.Fatalf("Subscriptions: %+v %v", view, err)
		}
		_, err = svcs.TrackSubscription(ctx, "NETFLIX")
		assertStatus(t, err, http.StatusBadRequest)
		_, err = svcs.TrackSubscription(ctx, "nao-existe")
		assertStatus(t, err, http.StatusNotFound)
		rec, err := svcs.TrackSubscription(ctx, "netflix-com")
		if err != nil || !rec.IsSubscription || rec.AmountCents != 5590 || rec.Kind != domain.CategoryExpense ||
			rec.StartDate != domain.AddDays(today, 1) || rec.CategoryID == nil || rec.AccountID == nil {
			t.Fatalf("TrackSubscription: %+v %v", rec, err)
		}
		_, err = svcs.TrackSubscription(ctx, "netflix-com")
		assertStatus(t, err, http.StatusConflict)
		view, _ = svcs.Subscriptions(ctx)
		for _, it := range view.Items {
			if it.Key == "netflix-com" && (it.RecurringID == nil || *it.RecurringID != rec.ID) {
				t.Fatalf("assinatura acompanhada deveria apontar a recorrência: %+v", it)
			}
		}
		if err := svcs.DismissSubscription(ctx, "spotify"); err != nil {
			t.Fatal(err)
		}
		if err := svcs.DismissSubscription(ctx, "spotify"); err != nil {
			t.Fatal(err)
		}
		view, _ = svcs.Subscriptions(ctx)
		if len(view.Items) != 1 || view.Items[0].Key != "netflix-com" {
			t.Fatalf("dispensada deveria sumir: %+v", view.Items)
		}
		assertStatus(t, svcs.DismissSubscription(ctx, "Spotify!"), http.StatusBadRequest)
	})
}

func TestCashflowBreakdownAndTransfers(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		a := mustAcc(t, svcs, "A", 0)
		b := mustAcc(t, svcs, "B", 0)
		card, _ := svcs.CreateCard(ctx, domain.CreateCardInput{Name: "Cartão", Brand: domain.BrandVisa, LimitCents: 1000000, ClosingDay: 5, DueDay: 12})
		_, _ = svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxIncome, Description: "Salário", AmountCents: 500000, Date: "2025-05-05", AccountID: a.ID})
		_, _ = svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxIncome, Description: "Salário", AmountCents: 500000, Date: "2025-06-05", AccountID: a.ID})
		if _, err := svcs.AddCardExpense(ctx, card.ID, domain.CreateCardExpenseInput{Description: "Mercado", TotalCents: 40000, Date: "2025-05-10", Installments: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.AddCardExpense(ctx, card.ID, domain.CreateCardExpenseInput{Description: "TV", TotalCents: 60000, Date: "2025-05-10", Installments: 2}); err != nil {
			t.Fatal(err)
		}

		m, y := 6, 2025
		cash, err := svcs.InsightsCashflow(ctx, domain.CashflowQuery{Months: 2, Month: &m, Year: &y})
		if err != nil || cash.Basis != domain.CashflowBasisCash || len(cash.Months) != 2 {
			t.Fatalf("cashflow: %+v %v", cash, err)
		}
		// Caixa: compras de 10/05 vencem 12/06.
		may, jun := cash.Months[0], cash.Months[1]
		if may.TotalOutCents != 0 || jun.DailyCents != 40000 || jun.InstallmentsCents != 30000 || jun.ResultCents != 430000 {
			t.Fatalf("caixa: %+v %+v", may, jun)
		}
		acc, err := svcs.InsightsCashflow(ctx, domain.CashflowQuery{Months: 2, Basis: domain.CashflowBasisAccrual, Month: &m, Year: &y})
		if err != nil || acc.Months[0].TotalOutCents != 70000 || acc.Months[1].InstallmentsCents != 30000 {
			t.Fatalf("competência: %+v %v", acc, err)
		}
		_, err = svcs.InsightsCashflow(ctx, domain.CashflowQuery{Months: 13})
		assertValidation(t, err, "months")

		invoices, _ := svcs.ListInvoices(ctx, card.ID)
		var june domain.Invoice
		for _, inv := range invoices {
			if inv.Month == 6 && inv.Year == 2025 {
				june = inv
			}
		}
		bd, err := svcs.InvoiceBreakdown(ctx, card.ID, june.ID)
		if err != nil || bd.PurchasesCents != 70000 || bd.TotalCents != 70000 || bd.Composition.InstallmentsCents != 30000 || bd.Composition.OneOffCents != 40000 {
			t.Fatalf("breakdown: %+v %v", bd, err)
		}
		_, err = svcs.InvoiceBreakdown(ctx, "11111111-1111-4111-8111-111111111111", june.ID)
		assertStatus(t, err, http.StatusNotFound)

		today := domain.TodayString()
		exp := mustExpense(t, svcs, a.ID, "PIX para B", 15000, domain.AddDays(today, -1))
		inc, _ := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxIncome, Description: "PIX de A", AmountCents: 15000, Date: today, AccountID: b.ID})
		cands, err := svcs.TransferCandidates(ctx)
		if err != nil || len(cands) != 1 || cands[0].Expense.ID != exp.ID || cands[0].Income.ID != inc.ID || cands[0].DaysApart != 1 {
			t.Fatalf("TransferCandidates: %+v %v", cands, err)
		}
		_, err = svcs.MergeTransfer(ctx, domain.MergeTransferInput{ExpenseID: inc.ID, IncomeID: exp.ID})
		assertStatus(t, err, http.StatusBadRequest)
		res, err := svcs.MergeTransfer(ctx, domain.MergeTransferInput{ExpenseID: exp.ID, IncomeID: inc.ID})
		if err != nil || res.TransferGroupID == "" {
			t.Fatalf("MergeTransfer: %+v %v", res, err)
		}
		if cands, _ = svcs.TransferCandidates(ctx); len(cands) != 0 {
			t.Fatalf("pernas mescladas não são mais candidatas: %+v", cands)
		}
		_, err = svcs.MergeTransfer(ctx, domain.MergeTransferInput{ExpenseID: exp.ID, IncomeID: inc.ID})
		assertStatus(t, err, http.StatusBadRequest)
	})
}

func TestCanAffordService(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		acc := mustAcc(t, svcs, "Conta", 300000)
		today := domain.TodayString()

		res, err := svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 10000})
		if err != nil || res.Verdict != domain.VerdictYes || res.LowestBalanceAfterCents != 290000 || res.Limit != nil || res.FitsOn != nil {
			t.Fatalf("YES: %+v %v", res, err)
		}
		res, _ = svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 250000})
		if res.Verdict != domain.VerdictCareful || !strings.Contains(res.Message, "faixa amarela") {
			t.Fatalf("CAREFUL (faixa amarela): %+v", res)
		}
		res, _ = svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 400000})
		if res.Verdict != domain.VerdictNo || res.FitsOn != nil || res.LowestBalanceAfterCents != -100000 || !strings.Contains(res.Message, "Não cabe") {
			t.Fatalf("NO sem entrada prevista: %+v", res)
		}

		// Com receita recorrente, a compra caberia a partir da próxima entrada.
		ty, tm := domain.FromDateString(today).Year(), int(domain.FromDateString(today).Month())
		ny, nm := domain.AddMonths(ty, tm, 1)
		next := domain.MonthStart(ny, nm)
		if _, err := svcs.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "Salário", Kind: domain.CategoryIncome, AmountCents: 500000, DayOfMonth: 1, AccountID: &acc.ID, StartDate: &next}); err != nil {
			t.Fatal(err)
		}
		res, _ = svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 400000})
		if res.Verdict != domain.VerdictNo || res.FitsOn == nil || *res.FitsOn != next {
			t.Fatalf("fitsOn: %+v", res)
		}
		// Parcelado sem cartão (carnê): 4 × R$ 1.000 saem mês a mês e a próxima entrada cobre as seguintes.
		res, err = svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 400000, Installments: ptrTo(4)})
		if err != nil || res.Verdict != domain.VerdictYes || res.LowestBalanceAfterCents != 200000 {
			t.Fatalf("parcelado sem cartão: %+v %v", res, err)
		}

		// Cartão: parcelas saem nos vencimentos, a 1ª conta no limite do mês.
		card, _ := svcs.CreateCard(ctx, domain.CreateCardInput{Name: "Cartão", Brand: domain.BrandVisa, LimitCents: 1000000, ClosingDay: 5, DueDay: 12})
		if _, err := svcs.UpdateSettings(ctx, domain.UpdateSettingsInput{MonthlyLimitCents: domain.Optional[int64]{Set: true, Valid: true, Value: 20000}}); err != nil {
			t.Fatal(err)
		}
		res, err = svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 60000, Installments: ptrTo(3), CardID: &card.ID, Description: ptrTo("Fone")})
		if err != nil || res.Limit == nil || res.Limit.SpentAfterCents != 20000 || res.Limit.PercentAfter != 100 {
			t.Fatalf("cartão com limite: %+v %v", res, err)
		}
		res, _ = svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 30000, CardID: &card.ID})
		if res.Verdict != domain.VerdictCareful || res.Limit.SpentAfterCents <= res.Limit.LimitCents {
			t.Fatalf("estourar o limite = CAREFUL: %+v", res)
		}

		_, err = svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 100, CardID: ptrTo("11111111-1111-4111-8111-111111111111")})
		assertStatus(t, err, http.StatusNotFound)
		_, err = svcs.CanAfford(ctx, domain.CanAffordInput{AmountCents: 0, Installments: ptrTo(49)})
		assertValidation(t, err, "installments")
	})
}

func TestAlertsAndOverviewV3(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		acc := mustAcc(t, svcs, "Conta", 100000)
		food := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Mercado", Kind: domain.CategoryExpense})
		today := domain.TodayString()
		if _, err := svcs.UpsertBudget(ctx, domain.UpsertBudgetInput{CategoryID: food.ID, AmountCents: 10000}); err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "Feira", AmountCents: 9500, Date: today, AccountID: acc.ID, CategoryID: &food.ID}); err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "Boleto", AmountCents: 100, Date: domain.AddDays(today, -10), Status: domain.TxPending, AccountID: acc.ID}); err != nil {
			t.Fatal(err)
		}

		alerts, err := svcs.Alerts(ctx)
		if err != nil {
			t.Fatal(err)
		}
		month := domain.MonthKey(today)
		ids := map[string]bool{}
		for _, a := range alerts {
			ids[a.ID] = true
		}
		budgetID := "BUDGET_DANGER:" + food.ID + ":" + month
		if !ids[budgetID] || !ids["PENDING_OLD:pending:"+month] {
			t.Fatalf("alertas esperados ausentes: %+v", alerts)
		}
		if alerts[0].Severity != domain.SeverityDanger {
			t.Fatalf("DANGER primeiro: %+v", alerts)
		}
		if err := svcs.DismissAlert(ctx, budgetID); err != nil {
			t.Fatal(err)
		}
		alerts, _ = svcs.Alerts(ctx)
		for _, a := range alerts {
			if a.ID == budgetID {
				t.Fatal("alerta dispensado continua no feed")
			}
		}
		assertStatus(t, svcs.DismissAlert(ctx, "sem-formato"), http.StatusBadRequest)

		ov, err := svcs.InsightsOverview(ctx, domain.PeriodQuery{})
		if err != nil {
			t.Fatal(err)
		}
		tt := domain.FromDateString(today)
		if len(ov.Pace.Series) != domain.DaysInMonth(tt.Year(), int(tt.Month())) {
			t.Fatalf("série do ritmo: %d pontos", len(ov.Pace.Series))
		}
		todayPoint := ov.Pace.Series[tt.Day()-1]
		if todayPoint.CurrentCents == nil || *todayPoint.CurrentCents != ov.Pace.CurrentCents {
			t.Fatalf("ponto de hoje: %+v (current %d)", todayPoint, ov.Pace.CurrentCents)
		}
		if tt.Day() < len(ov.Pace.Series) && ov.Pace.Series[tt.Day()].CurrentCents != nil {
			t.Fatal("dias depois de hoje deveriam ser null")
		}
		if ov.SpendToday.Basis != domain.SpendTodayBalance || ov.SpendToday.Cents < 0 {
			t.Fatalf("spendToday sem limite: %+v", ov.SpendToday)
		}
		if _, err := svcs.UpdateSettings(ctx, domain.UpdateSettingsInput{MonthlyLimitCents: domain.Optional[int64]{Set: true, Valid: true, Value: 400000}}); err != nil {
			t.Fatal(err)
		}
		ov, _ = svcs.InsightsOverview(ctx, domain.PeriodQuery{})
		if ov.SpendToday.Basis != domain.SpendTodayLimit || ov.MonthlyLimit.DailyAllowanceCents == nil || ov.SpendToday.Cents != *ov.MonthlyLimit.DailyAllowanceCents {
			t.Fatalf("spendToday com limite: %+v %+v", ov.SpendToday, ov.MonthlyLimit)
		}
	})
}

func TestRecurringIsSubscriptionAndWishImage(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		r, err := svcs.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "Gym", Kind: domain.CategoryExpense, AmountCents: 9900, DayOfMonth: 5, IsSubscription: ptrTo(true)})
		if err != nil || !r.IsSubscription {
			t.Fatalf("isSubscription: %+v %v", r, err)
		}
		var patch domain.UpdateRecurringInput
		patch.IsSubscription = domain.Optional[bool]{Set: true}
		if r, err = svcs.UpdateRecurring(ctx, r.ID, patch); err != nil || r.IsSubscription {
			t.Fatalf("null volta a false: %+v %v", r, err)
		}
		w, err := svcs.CreateWishItem(ctx, domain.CreateWishItemInput{Name: "Fone", PriceCents: 100, ImageURL: ptrTo("https://img.example/f.jpg")})
		if err != nil || w.ImageURL == nil {
			t.Fatalf("imageUrl: %+v %v", w, err)
		}
		_, err = svcs.CreateWishItem(ctx, domain.CreateWishItemInput{Name: "X", PriceCents: 100, ImageURL: ptrTo("javascript:alert(1)")})
		assertValidation(t, err, "imageUrl")
	})
}
