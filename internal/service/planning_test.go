package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func ptrTo[T any](v T) *T { return &v }

func assertStatus(t *testing.T, err error, status int) {
	t.Helper()
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != status {
		t.Fatalf("erro = %v, want %d", err, status)
	}
}

func mustCat(t *testing.T, svcs *Services, in domain.CreateCategoryInput) domain.Category {
	t.Helper()
	if in.Icon == "" {
		in.Icon = "tag"
	}
	if in.Color == "" {
		in.Color = "#000000"
	}
	c, err := svcs.CreateCategory(context.Background(), in)
	if err != nil {
		t.Fatalf("criar categoria %s: %v", in.Name, err)
	}
	return c
}

func TestCategorySpendType(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)

		root := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Diversos", Kind: domain.CategoryExpense})
		if root.SpendType == nil || *root.SpendType != domain.SpendVariable {
			t.Fatalf("despesa raiz sem tipo deveria ser VARIABLE: %+v", root)
		}
		moradia := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Moradia", Kind: domain.CategoryExpense, SpendType: ptrTo(domain.SpendFixed)})
		aluguel := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Aluguel", Kind: domain.CategoryExpense, ParentID: &moradia.ID})
		if aluguel.SpendType == nil || *aluguel.SpendType != domain.SpendFixed {
			t.Fatalf("subcategoria deveria herdar FIXED: %+v", aluguel)
		}
		income := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Salário", Kind: domain.CategoryIncome})
		if income.SpendType != nil {
			t.Fatalf("receita não tem tipo de gasto: %+v", income)
		}
		_, err := svcs.CreateCategory(ctx, domain.CreateCategoryInput{Name: "Bônus", Kind: domain.CategoryIncome, Icon: "x", Color: "#000000", SpendType: ptrTo(domain.SpendFixed)})
		assertValidation(t, err, "spendType")

		// Valor próprio na subcategoria; null volta a herdar.
		var patch domain.UpdateCategoryInput
		patch.SpendType = domain.Optional[domain.SpendType]{Set: true, Valid: true, Value: domain.SpendCuttable}
		updated, err := svcs.UpdateCategory(ctx, aluguel.ID, patch)
		if err != nil || *updated.SpendType != domain.SpendCuttable {
			t.Fatalf("update spendType: %+v %v", updated, err)
		}
		patch.SpendType = domain.Optional[domain.SpendType]{Set: true}
		updated, err = svcs.UpdateCategory(ctx, aluguel.ID, patch)
		if err != nil || *updated.SpendType != domain.SpendFixed {
			t.Fatalf("null deveria voltar a herdar: %+v %v", updated, err)
		}
		patch.SpendType = domain.Optional[domain.SpendType]{Set: true, Valid: true, Value: domain.SpendFixed}
		_, err = svcs.UpdateCategory(ctx, income.ID, patch)
		assertValidation(t, err, "spendType")

		expense := domain.CategoryExpense
		list, err := svcs.ListCategories(ctx, &expense)
		if err != nil || len(list) != 3 {
			t.Fatalf("ListCategories: %+v %v", list, err)
		}
		for _, c := range list {
			if c.SpendType == nil {
				t.Fatalf("despesa listada sem tipo efetivo: %+v", c)
			}
		}
	})
}

func TestRecurringServiceFlow(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		acc, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta", Type: domain.AccountChecking, InitialBalanceCents: 100000})
		if err != nil {
			t.Fatal(err)
		}
		rent := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Aluguel", Kind: domain.CategoryExpense, SpendType: ptrTo(domain.SpendFixed)})
		salary := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Salário", Kind: domain.CategoryIncome})

		_, err = svcs.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "Aluguel", Kind: domain.CategoryExpense, AmountCents: 1000, DayOfMonth: 5, CategoryID: &salary.ID})
		assertStatus(t, err, http.StatusBadRequest)
		_, err = svcs.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "X", Kind: domain.CategoryExpense, AmountCents: 1000, DayOfMonth: 5, AccountID: ptrTo("11111111-1111-4111-8111-111111111111")})
		assertStatus(t, err, http.StatusNotFound)
		_, err = svcs.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "X", Kind: domain.CategoryExpense, AmountCents: 1000, DayOfMonth: 5, StartDate: ptrTo("2026-05-01"), EndDate: ptrTo("2026-04-01")})
		assertValidation(t, err, "endDate")

		today := domain.TodayString()
		monthStart := today[:8] + "01"
		r, err := svcs.CreateRecurring(ctx, domain.CreateRecurringInput{
			Description: "Aluguel", Kind: domain.CategoryExpense, AmountCents: 150000, DayOfMonth: 1,
			AccountID: &acc.ID, CategoryID: &rent.ID, StartDate: &monthStart,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Active || r.StartDate != monthStart || r.NextDate == nil || *r.NextDate != monthStart || r.AccountName == nil || *r.CategoryName != "Aluguel" {
			t.Fatalf("recorrência criada: %+v", r)
		}
		noAccount, err := svcs.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "Freela", Kind: domain.CategoryIncome, AmountCents: 1000, DayOfMonth: 28})
		if err != nil || noAccount.StartDate != today {
			t.Fatalf("startDate padrão = hoje: %+v %v", noAccount, err)
		}
		_, err = svcs.PostRecurring(ctx, noAccount.ID, domain.PostRecurringInput{})
		assertValidation(t, err, "accountId")

		tx, err := svcs.PostRecurring(ctx, r.ID, domain.PostRecurringInput{})
		if err != nil {
			t.Fatal(err)
		}
		if tx.RecurringID == nil || *tx.RecurringID != r.ID || tx.AmountCents != -150000 || tx.Date != monthStart || tx.Status != domain.TxConfirmed || tx.CategoryID == nil {
			t.Fatalf("transação lançada: %+v", tx)
		}
		_, err = svcs.PostRecurring(ctx, r.ID, domain.PostRecurringInput{})
		assertStatus(t, err, http.StatusConflict)

		got, err := svcs.GetRecurring(ctx, r.ID)
		nextMonth := domain.AddMonthsToDateString(monthStart, 1)
		if err != nil || got.NextDate == nil || *got.NextDate != nextMonth {
			t.Fatalf("nextDate após lançar = %v (%v), want %s", got.NextDate, err, nextMonth)
		}
		// Valor e data customizados para outro mês.
		amt := int64(160000)
		tx2, err := svcs.PostRecurring(ctx, r.ID, domain.PostRecurringInput{Date: &nextMonth, AmountCents: &amt})
		if err != nil || tx2.AmountCents != -160000 || tx2.Date != nextMonth {
			t.Fatalf("lançamento customizado: %+v %v", tx2, err)
		}

		var patch domain.UpdateRecurringInput
		patch.Active = domain.Optional[bool]{Set: true, Valid: true, Value: false}
		patch.AmountCents = domain.Optional[int64]{Set: true, Valid: true, Value: 155000}
		updated, err := svcs.UpdateRecurring(ctx, r.ID, patch)
		if err != nil || updated.Active || updated.AmountCents != 155000 || updated.NextDate != nil {
			t.Fatalf("update: %+v %v", updated, err)
		}
		list, err := svcs.ListRecurring(ctx)
		if err != nil || len(list) != 2 || !list[0].Active {
			t.Fatalf("lista (ativas primeiro): %+v %v", list, err)
		}
		if err := svcs.DeleteRecurring(ctx, r.ID); err != nil {
			t.Fatal(err)
		}
		_, err = svcs.GetRecurring(ctx, r.ID)
		assertStatus(t, err, http.StatusNotFound)
		kept, err := svcs.GetTransaction(ctx, tx.ID)
		if err != nil || kept.RecurringID != nil {
			t.Fatalf("transação deveria continuar, sem vínculo: %+v %v", kept, err)
		}
	})
}

func TestSettingsService(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		def, err := svcs.GetSettings(ctx)
		if err != nil || def.MonthlyLimitCents != nil || def.ReserveMonths != 6 || def.ReserveRefillRatePct != 10 || def.BalanceBands != domain.DefaultBalanceBands() || def.SpendPlan != nil {
			t.Fatalf("padrões: %+v %v", def, err)
		}
		var in domain.UpdateSettingsInput
		in.MonthlyLimitCents = domain.Optional[int64]{Set: true, Valid: true, Value: 500000}
		in.HasDependents = domain.Optional[bool]{Set: true, Valid: true, Value: true}
		in.BalanceBands = domain.Optional[domain.BalanceBandsPatch]{Set: true, Valid: true, Value: domain.BalanceBandsPatch{DarkGreen: ptrTo(int64(300000))}}
		in.SpendPlan = domain.Optional[domain.SpendPlan]{Set: true, Valid: true, Value: domain.SpendPlan{FixedPct: 50, CuttablePct: 20, VariablePct: 30}}
		got, err := svcs.UpdateSettings(ctx, in)
		if err != nil || *got.MonthlyLimitCents != 500000 || !got.HasDependents || got.BalanceBands.DarkGreen != 300000 || got.BalanceBands.LightGreen != 100000 || got.SpendPlan.CuttablePct != 20 {
			t.Fatalf("merge: %+v %v", got, err)
		}
		reread, _ := svcs.GetSettings(ctx)
		if *reread.MonthlyLimitCents != 500000 || reread.SpendPlan == nil || reread.ReserveMonths != 6 {
			t.Fatalf("persistência: %+v", reread)
		}

		var clear domain.UpdateSettingsInput
		clear.MonthlyLimitCents = domain.Optional[int64]{Set: true}
		clear.SpendPlan = domain.Optional[domain.SpendPlan]{Set: true}
		got, err = svcs.UpdateSettings(ctx, clear)
		if err != nil || got.MonthlyLimitCents != nil || got.SpendPlan != nil || !got.HasDependents {
			t.Fatalf("null limpa: %+v %v", got, err)
		}

		var bad domain.UpdateSettingsInput
		bad.ReserveMonths = domain.Optional[int]{Set: true, Valid: true, Value: 0}
		bad.BalanceBands = domain.Optional[domain.BalanceBandsPatch]{Set: true, Valid: true, Value: domain.BalanceBandsPatch{Yellow: ptrTo(int64(999999))}}
		_, err = svcs.UpdateSettings(ctx, bad)
		assertValidation(t, err, "reserveMonths")
		assertValidation(t, err, "balanceBands")
		if after, _ := svcs.GetSettings(ctx); after.ReserveMonths != 6 {
			t.Fatalf("patch inválido não pode gravar nada: %+v", after)
		}
	})
}

func TestCategoryRulesService(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		acc, _ := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Nubank", Type: domain.AccountChecking, InitialBalanceCents: 100000})
		transp := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Transporte", Kind: domain.CategoryExpense})
		delivery := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Delivery", Kind: domain.CategoryExpense})
		salary := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Salário", Kind: domain.CategoryIncome})

		// Lançada antes da regra existir: fica sem categoria até o apply.
		before, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "UBER *VIAGEM", AmountCents: 1500, Date: domain.TodayString(), AccountID: acc.ID})
		if err != nil || before.CategoryID != nil {
			t.Fatalf("sem regras: %+v %v", before, err)
		}

		_, err = svcs.CreateCategoryRule(ctx, domain.CreateCategoryRuleInput{Pattern: "uber", CategoryID: "11111111-1111-4111-8111-111111111111"})
		assertStatus(t, err, http.StatusNotFound)
		_, err = svcs.CreateCategoryRule(ctx, domain.CreateCategoryRuleInput{Pattern: " ", CategoryID: transp.ID})
		assertValidation(t, err, "pattern")

		uber, err := svcs.CreateCategoryRule(ctx, domain.CreateCategoryRuleInput{Pattern: "  Úber ", CategoryID: transp.ID})
		if err != nil || uber.Pattern != "Úber" || uber.CategoryName != "Transporte" || uber.Priority != 0 {
			t.Fatalf("regra criada: %+v %v", uber, err)
		}
		if _, err := svcs.CreateCategoryRule(ctx, domain.CreateCategoryRuleInput{Pattern: "uber eats", CategoryID: delivery.ID}); err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.CreateCategoryRule(ctx, domain.CreateCategoryRuleInput{Pattern: "salario", CategoryID: salary.ID, Priority: ptrTo(3)}); err != nil {
			t.Fatal(err)
		}

		if got, _ := svcs.SuggestCategory(ctx, "Uber Eats *pedido", domain.CategoryExpense); got == nil || *got != delivery.ID {
			t.Fatalf("padrão mais longo deveria vencer: %v", got)
		}
		if got, _ := svcs.SuggestCategory(ctx, "Salário", domain.CategoryExpense); got != nil {
			t.Fatalf("regra de receita não pode categorizar despesa: %v", *got)
		}

		tx, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxIncome, Description: "SALÁRIO ACME", AmountCents: 500000, Date: domain.TodayString(), AccountID: acc.ID})
		if err != nil || tx.CategoryID == nil || *tx.CategoryID != salary.ID {
			t.Fatalf("receita categorizada pela regra: %+v %v", tx, err)
		}
		explicit, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "uber", AmountCents: 100, Date: domain.TodayString(), AccountID: acc.ID, CategoryID: &delivery.ID})
		if err != nil || *explicit.CategoryID != delivery.ID {
			t.Fatalf("categoria explícita não pode ser trocada: %+v %v", explicit, err)
		}

		card, err := svcs.CreateCard(ctx, domain.CreateCardInput{Name: "Cartão", Brand: domain.BrandVisa, LimitCents: 100000, ClosingDay: 10, DueDay: 17})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.AddCardExpense(ctx, card.ID, domain.CreateCardExpenseInput{Description: "Uber Eats", TotalCents: 3000, Date: domain.TodayString(), Installments: 1}); err != nil {
			t.Fatal(err)
		}
		search := "Uber Eats"
		page, _ := svcs.ListTransactions(ctx, domain.ListTransactionsQuery{Search: &search})
		if len(page.Items) != 1 || page.Items[0].CategoryID == nil || *page.Items[0].CategoryID != delivery.ID {
			t.Fatalf("compra no cartão categorizada: %+v", page.Items)
		}

		// Telegram sem hashtag usa a regra e cita a categoria.
		reply, err := svcs.Telegram.BuildReply(ctx, "32,50 uber eats")
		if err != nil || !strings.Contains(reply, "<b>Delivery</b>") {
			t.Fatalf("telegram: %q %v", reply, err)
		}

		res, err := svcs.ApplyCategoryRules(ctx, domain.ApplyRulesInput{})
		if err != nil || res.Updated != 1 {
			t.Fatalf("apply (só sem categoria): %+v %v", res, err)
		}
		if got, _ := svcs.GetTransaction(ctx, before.ID); got.CategoryID == nil || *got.CategoryID != transp.ID {
			t.Fatalf("apply não categorizou: %+v", got)
		}
		res, err = svcs.ApplyCategoryRules(ctx, domain.ApplyRulesInput{OnlyUncategorized: ptrTo(false)})
		if err != nil || res.Updated != 1 {
			t.Fatalf("apply em todas (só a explícita diverge): %+v %v", res, err)
		}

		var patch domain.UpdateCategoryRuleInput
		patch.Priority = domain.Optional[int]{Set: true, Valid: true, Value: 50}
		updated, err := svcs.UpdateCategoryRule(ctx, uber.ID, patch)
		if err != nil || updated.Priority != 50 {
			t.Fatalf("update regra: %+v %v", updated, err)
		}
		rules, _ := svcs.ListCategoryRules(ctx)
		if len(rules) != 3 || rules[0].ID != uber.ID {
			t.Fatalf("ordem por prioridade: %+v", rules)
		}
		if err := svcs.DeleteCategoryRule(ctx, uber.ID); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, svcs.DeleteCategoryRule(ctx, uber.ID), http.StatusNotFound)
	})
}

func TestReserveGoalAndWishlist(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)

		g, err := svcs.CreateGoal(ctx, domain.CreateGoalInput{Name: "Viagem", TargetCents: 1000})
		if err != nil || g.Kind != domain.GoalKindGoal {
			t.Fatalf("kind padrão: %+v %v", g, err)
		}
		r, err := svcs.CreateGoal(ctx, domain.CreateGoalInput{Name: "Reserva", Kind: domain.GoalKindReserve, TargetCents: 1000})
		if err != nil || r.Kind != domain.GoalKindReserve {
			t.Fatalf("reserva: %+v %v", r, err)
		}
		_, err = svcs.CreateGoal(ctx, domain.CreateGoalInput{Name: "Outra reserva", Kind: domain.GoalKindReserve, TargetCents: 1000})
		assertStatus(t, err, http.StatusConflict)
		_, err = svcs.CreateGoal(ctx, domain.CreateGoalInput{Name: "X", Kind: "OUTRO", TargetCents: 1000})
		assertValidation(t, err, "kind")

		if _, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta", Type: domain.AccountChecking, InitialBalanceCents: 100000}); err != nil {
			t.Fatal(err)
		}
		cheap, err := svcs.CreateWishItem(ctx, domain.CreateWishItemInput{Name: "Livro", PriceCents: 60000})
		if err != nil || cheap.Priority != 1 || cheap.Status != domain.WishListed || cheap.AffordableOn == nil || *cheap.AffordableOn != domain.TodayString() {
			t.Fatalf("desejo barato: %+v %v", cheap, err)
		}
		// Junto com o anterior (mais prioritário) passa do saldo: não cabe no horizonte.
		pricey, err := svcs.CreateWishItem(ctx, domain.CreateWishItemInput{Name: "TV", PriceCents: 50000, URL: ptrTo("https://x")})
		if err != nil || pricey.Priority != 2 || pricey.AffordableOn != nil {
			t.Fatalf("desejo que não cabe: %+v %v", pricey, err)
		}
		var patch domain.UpdateWishItemInput
		patch.Status = domain.Optional[domain.WishStatus]{Set: true, Valid: true, Value: domain.WishBought}
		bought, err := svcs.UpdateWishItem(ctx, cheap.ID, patch)
		if err != nil || bought.Status != domain.WishBought || bought.AffordableOn != nil {
			t.Fatalf("comprado: %+v %v", bought, err)
		}
		items, err := svcs.ListWishlist(ctx)
		if err != nil || len(items) != 2 || items[0].ID != pricey.ID || items[0].AffordableOn == nil {
			t.Fatalf("lista após comprar o 1º: %+v %v", items, err)
		}
		_, err = svcs.CreateWishItem(ctx, domain.CreateWishItemInput{Name: "", PriceCents: 0})
		assertValidation(t, err, "priceCents")
		if err := svcs.DeleteWishItem(ctx, pricey.ID); err != nil {
			t.Fatal(err)
		}
		_, err = svcs.UpdateWishItem(ctx, pricey.ID, patch)
		assertStatus(t, err, http.StatusNotFound)
	})
}

func TestInsightsService(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		checking, _ := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Corrente", Type: domain.AccountChecking, InitialBalanceCents: 1000000})
		savings, _ := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Poupança", Type: domain.AccountSavings})
		rent := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Aluguel", Kind: domain.CategoryExpense, SpendType: ptrTo(domain.SpendFixed)})
		netflix := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Netflix", Kind: domain.CategoryExpense, SpendType: ptrTo(domain.SpendCuttable)})
		food := mustCat(t, svcs, domain.CreateCategoryInput{Name: "Mercado", Kind: domain.CategoryExpense})

		month, year := domain.CurrentMonthYear()
		prevY, prevM := domain.AddMonths(year, month, -1)
		day1 := domain.MonthStart(year, month)
		prevDay1 := domain.MonthStart(prevY, prevM)
		add := func(typ domain.TransactionType, desc string, cents int64, date string, cat *string) {
			t.Helper()
			if _, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{Type: typ, Description: desc, AmountCents: cents, Date: date, AccountID: checking.ID, CategoryID: cat}); err != nil {
				t.Fatal(err)
			}
		}
		add(domain.TxIncome, "Salário", 500000, day1, nil)
		add(domain.TxExpense, "Aluguel", 200000, day1, &rent.ID)
		add(domain.TxExpense, "Netflix", 5000, day1, &netflix.ID)
		add(domain.TxExpense, "Mercado", 90000, day1, &food.ID)
		add(domain.TxExpense, "Avulso", 10000, day1, nil)
		add(domain.TxExpense, "Mercado anterior", 30000, prevDay1, &food.ID)
		if _, err := svcs.Transfer(ctx, domain.TransferInput{FromAccountID: checking.ID, ToAccountID: savings.ID, AmountCents: 50000, Date: day1}); err != nil {
			t.Fatal(err)
		}
		reserve, _ := svcs.CreateGoal(ctx, domain.CreateGoalInput{Name: "Reserva", Kind: domain.GoalKindReserve, TargetCents: 100000})
		if _, err := svcs.ContributeGoal(ctx, reserve.ID, domain.ContributeGoalInput{AmountCents: 20000, Date: &day1}); err != nil {
			t.Fatal(err)
		}
		var limit domain.UpdateSettingsInput
		limit.MonthlyLimitCents = domain.Optional[int64]{Set: true, Valid: true, Value: 400000}
		if _, err := svcs.UpdateSettings(ctx, limit); err != nil {
			t.Fatal(err)
		}

		ov, err := svcs.InsightsOverview(ctx, domain.PeriodQuery{})
		if err != nil {
			t.Fatal(err)
		}
		perf := ov.Performance
		if ov.Month != month || perf.IncomeCents != 500000 || perf.FixedCents != 205000 || perf.VariableCents != 100000 || perf.SavedCents != 50000 || perf.PerformanceCents != 195000 || perf.Verdict != "SAVING" || *perf.SavingsRatePct != 10 {
			t.Fatalf("performance: %+v", perf)
		}
		if perf.PreviousPerformanceCents != -30000 {
			t.Fatalf("performance anterior = %d", perf.PreviousPerformanceCents)
		}
		if ov.Pace.CurrentCents != 305000 || ov.Pace.PreviousSamePeriodCents != 30000 || ov.Pace.Status != "ABOVE" {
			t.Fatalf("ritmo: %+v", ov.Pace)
		}
		ml := ov.MonthlyLimit
		if *ml.LimitCents != 400000 || ml.SpentCents != 305000 || *ml.RemainingCents != 95000 || ml.DailyAllowanceCents == nil {
			t.Fatalf("limite: %+v", ml)
		}
		if len(ov.SpendTypes) != 3 || ov.SpendTypes[0].TotalCents != 200000 || ov.SpendTypes[1].TotalCents != 5000 || ov.SpendTypes[2].TotalCents != 100000 {
			t.Fatalf("tipos de gasto: %+v", ov.SpendTypes)
		}
		balance, _ := svcs.Store.TotalBalanceCents(ctx)
		if ov.NetWorthCents != balance {
			t.Fatalf("patrimônio = %d, saldo = %d", ov.NetWorthCents, balance)
		}
		if ov.Reserve.GoalID == nil || ov.Reserve.SavedCents != 20000 || ov.Reserve.AvgMonthlyCostCents != 30000 || ov.Reserve.SuggestedTargetCents != 180000 || ov.Reserve.Status != "BUILDING" {
			t.Fatalf("reserva: %+v", ov.Reserve)
		}
		if len(ov.CutSuggestions) == 0 || ov.CutSuggestions[0].Kind != "CANCEL" || ov.CutSuggestions[0].Name != "Netflix" {
			t.Fatalf("cortes: %+v", ov.CutSuggestions)
		}
		m13 := 13
		_, err = svcs.InsightsOverview(ctx, domain.PeriodQuery{Month: &m13})
		assertValidation(t, err, "month")

		yr, err := svcs.InsightsYear(ctx, nil)
		if err != nil || yr.Year != year || len(yr.Months) != 12 {
			t.Fatalf("ano: %+v %v", yr, err)
		}
		cur := yr.Months[month-1]
		if cur.IncomeCents != 500000 || cur.TotalOutCents != 305000 || cur.SavedCents != 50000 {
			t.Fatalf("mês corrente no ano: %+v", cur)
		}
		if yr.Months[11].EndBalanceCents != balance {
			t.Fatalf("saldo de dezembro = %d, saldo atual = %d", yr.Months[11].EndBalanceCents, balance)
		}
		var sumIncome int64
		for _, m := range yr.Months {
			sumIncome += m.IncomeCents
		}
		if yr.Totals.IncomeCents != sumIncome {
			t.Fatalf("totais do ano: %+v", yr.Totals)
		}

		// Compromissos: recorrência de despesa + receita no mês e parcelas do cartão.
		if _, err := svcs.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "Aluguel fixo", Kind: domain.CategoryExpense, AmountCents: 200000, DayOfMonth: 28, AccountID: &checking.ID, StartDate: &day1}); err != nil {
			t.Fatal(err)
		}
		inc, err := svcs.CreateRecurring(ctx, domain.CreateRecurringInput{Description: "Salário fixo", Kind: domain.CategoryIncome, AmountCents: 500000, DayOfMonth: 1, AccountID: &checking.ID, StartDate: &day1})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.PostRecurring(ctx, inc.ID, domain.PostRecurringInput{}); err != nil {
			t.Fatal(err)
		}
		card, _ := svcs.CreateCard(ctx, domain.CreateCardInput{Name: "Cartão", Brand: domain.BrandVisa, LimitCents: 900000, ClosingDay: 10, DueDay: 17})
		if _, err := svcs.AddCardExpense(ctx, card.ID, domain.CreateCardExpenseInput{Description: "Geladeira", TotalCents: 300000, Date: day1, Installments: 3}); err != nil {
			t.Fatal(err)
		}
		cm, err := svcs.InsightsCommitments(ctx, domain.PeriodQuery{})
		if err != nil || len(cm.Months) != 6 {
			t.Fatalf("compromissos: %+v %v", cm, err)
		}
		if cm.Months[0].RecurringExpenseCents != 200000 || cm.Months[0].RecurringIncomeCents != 500000 || cm.Months[0].InstallmentsCents != 100000 || cm.Months[2].InstallmentsCents != 100000 || cm.Months[3].InstallmentsCents != 0 {
			t.Fatalf("meses dos compromissos: %+v", cm.Months)
		}
		var kinds []string
		for _, it := range cm.Items {
			kinds = append(kinds, it.Kind+":"+it.Direction)
		}
		if len(cm.Items) != 3 || cm.Items[0].DueDate != day1 || !cm.Items[0].Realized || cm.Items[0].Direction != "RECEIVE" {
			t.Fatalf("itens: %v %+v", kinds, cm.Items)
		}
		balance, _ = svcs.Store.LiquidBalanceCents(ctx)
		if cm.Totals.PayCents != 300000 || cm.Totals.ReceiveCents != 500000 || cm.Totals.BalanceAfterCents != balance-300000 {
			t.Fatalf("totais: %+v (saldo %d)", cm.Totals, balance)
		}

		p, err := svcs.InsightsProjection(ctx, 30)
		if err != nil || len(p.Days) != 30 || !p.LimitConfigured || p.StartBalanceCents != balance || len(p.Months) != 12 || p.StartDate != domain.TodayString() {
			t.Fatalf("projeção: days=%d limit=%v start=%d (saldo %d) %v", len(p.Days), p.LimitConfigured, p.StartBalanceCents, balance, err)
		}
		if p.Days[0].Band == "" || p.Days[0].Events == nil {
			t.Fatalf("dia 0: %+v", p.Days[0])
		}
		def, _ := svcs.InsightsProjection(ctx, 0)
		if len(def.Days) != DefaultProjectionDays {
			t.Fatalf("days padrão = %d", len(def.Days))
		}
		_, err = svcs.InsightsProjection(ctx, 366)
		assertValidation(t, err, "days")
	})
}
