package store

import (
	"context"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/testutil"
)

func TestRecurringCRUDAndRealizedMonths(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		acc := mustAccount(t, s, "Conta", 0)
		rent := mustCategory(t, s, "Aluguel", domain.CategoryExpense, nil)

		id, err := s.CreateRecurring(ctx, RecurringRow{
			Description: "Aluguel", Kind: domain.CategoryExpense, AmountCents: 200000, DayOfMonth: 31,
			AccountID: &acc.ID, CategoryID: &rent.ID, StartDate: "2026-01-01", Active: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.FindRecurringByID(ctx, id)
		if err != nil || r == nil {
			t.Fatalf("FindRecurringByID: %v %v", r, err)
		}
		if r.AccountName == nil || *r.AccountName != "Conta" || r.CategoryName == nil || *r.CategoryName != "Aluguel" || r.StartDate != "2026-01-01" || r.EndDate != nil || !r.Active {
			t.Fatalf("recorrência lida: %+v", r)
		}

		end := "2026-12-31"
		r.EndDate = &end
		r.Active = false
		r.AmountCents = 210000
		if err := s.UpdateRecurring(ctx, r); err != nil {
			t.Fatal(err)
		}
		list, err := s.ListRecurring(ctx)
		if err != nil || len(list) != 1 || list[0].AmountCents != 210000 || list[0].EndDate == nil || *list[0].EndDate != end || list[0].Active {
			t.Fatalf("ListRecurring: %+v %v", list, err)
		}

		tx, err := s.CreateRecurringTransaction(ctx, id, domain.TxExpense, "Aluguel", -210000, "2026-02-28", acc.ID, &rent.ID)
		if err != nil || tx.RecurringID == nil || *tx.RecurringID != id || tx.Status != domain.TxConfirmed {
			t.Fatalf("CreateRecurringTransaction: %+v %v", tx, err)
		}
		realized, err := s.RealizedRecurringMonths(ctx)
		if err != nil || !realized[id]["2026-02"] || realized[id]["2026-03"] {
			t.Fatalf("RealizedRecurringMonths: %+v %v", realized, err)
		}

		// Excluir a recorrência mantém a transação, só desvincula (ON DELETE SET NULL).
		if err := s.DeleteRecurring(ctx, id); err != nil {
			t.Fatal(err)
		}
		detail, err := s.FindTransactionByID(ctx, tx.ID)
		if err != nil || detail == nil || detail.RecurringID != nil {
			t.Fatalf("transação após excluir recorrência: %+v %v", detail, err)
		}
	})
}

func TestSettingsRulesAndWishlist(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)

		if err := s.SaveSettingValues(ctx, map[string]string{SettingMonthlyLimit: "1000", SettingReserveMonths: "6"}); err != nil {
			t.Fatal(err)
		}
		if err := s.SaveSettingValues(ctx, map[string]string{SettingMonthlyLimit: "2000"}); err != nil {
			t.Fatal(err)
		}
		values, err := s.SettingValues(ctx)
		if err != nil || values[SettingMonthlyLimit] != "2000" || values[SettingReserveMonths] != "6" {
			t.Fatalf("settings: %+v %v", values, err)
		}

		transp := mustCategory(t, s, "Transporte", domain.CategoryExpense, nil)
		delivery := mustCategory(t, s, "Delivery", domain.CategoryExpense, nil)
		for _, r := range []struct {
			p   string
			cat string
			pr  int
		}{{"uber", transp.ID, 0}, {"uber eats", delivery.ID, 0}, {"x", transp.ID, 5}} {
			if _, err := s.CreateCategoryRule(ctx, r.p, r.cat, r.pr); err != nil {
				t.Fatal(err)
			}
		}
		rules, err := s.ListCategoryRules(ctx)
		if err != nil || len(rules) != 3 || rules[0].Pattern != "x" || rules[1].Pattern != "uber eats" || rules[1].CategoryName != "Delivery" || rules[1].CategoryKind != domain.CategoryExpense {
			t.Fatalf("ordem das regras: %+v %v", rules, err)
		}
		// Excluir a categoria apaga as regras dela (CASCADE).
		if err := s.DeleteCategory(ctx, delivery.ID); err != nil {
			t.Fatal(err)
		}
		if rules, _ := s.ListCategoryRules(ctx); len(rules) != 2 {
			t.Fatalf("regras após excluir categoria: %+v", rules)
		}

		acc := mustAccount(t, s, "Conta", 0)
		a := mustTx(t, s, domain.TxExpense, "UBER *TRIP", 1000, "2026-01-01", domain.TxConfirmed, acc.ID, nil)
		mustTx(t, s, domain.TxExpense, "Já categorizada", 1000, "2026-01-01", domain.TxConfirmed, acc.ID, &transp.ID)
		targets, err := s.ListRuleTargets(ctx, true)
		if err != nil || len(targets) != 1 || targets[0].ID != a.ID {
			t.Fatalf("ListRuleTargets(onlyUncategorized): %+v %v", targets, err)
		}
		if all, _ := s.ListRuleTargets(ctx, false); len(all) != 2 {
			t.Fatalf("ListRuleTargets(todos): %+v", all)
		}
		if err := s.AssignCategories(ctx, map[string]string{a.ID: transp.ID}); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.FindTransactionByID(ctx, a.ID); got.CategoryID == nil || *got.CategoryID != transp.ID {
			t.Fatalf("AssignCategories não aplicou: %+v", got)
		}

		if p, _ := s.NextWishPriority(ctx); p != 1 {
			t.Fatalf("NextWishPriority vazio = %d", p)
		}
		w1, _ := s.CreateWishItem(ctx, WishItemRow{Name: "B", PriceCents: 100, Priority: 2, Status: domain.WishListed})
		_, _ = s.CreateWishItem(ctx, WishItemRow{Name: "A", PriceCents: 100, Priority: 1, Status: domain.WishListed})
		_, _ = s.CreateWishItem(ctx, WishItemRow{Name: "Comprado", PriceCents: 100, Priority: 0, Status: domain.WishBought})
		items, err := s.ListWishItems(ctx)
		if err != nil || len(items) != 3 || items[0].Name != "A" || items[1].Name != "B" || items[2].Name != "Comprado" {
			t.Fatalf("ordem da lista de desejos: %+v %v", items, err)
		}
		if p, _ := s.NextWishPriority(ctx); p != 3 {
			t.Fatalf("NextWishPriority = %d", p)
		}
		w, _ := s.FindWishItemByID(ctx, w1)
		url := "https://x"
		w.URL = &url
		w.Status = domain.WishArchived
		if err := s.UpdateWishItem(ctx, w); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.FindWishItemByID(ctx, w1); got.URL == nil || got.Status != domain.WishArchived {
			t.Fatalf("UpdateWishItem: %+v", got)
		}
		if err := s.DeleteWishItem(ctx, w1); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.FindWishItemByID(ctx, w1); got != nil {
			t.Fatal("desejo deveria ter sido excluído")
		}
	})
}

func TestBootstrapDefaultsAndBackfill(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()

		// Banco vazio: cria a taxonomia completa com tipos de gasto; reexecutar não duplica.
		s := New(d)
		if err := Bootstrap(ctx, s); err != nil {
			t.Fatal(err)
		}
		first := countRows(t, s, `SELECT COUNT(*) FROM categories`)
		if first < 50 {
			t.Fatalf("categorias padrão = %d", first)
		}
		if err := Bootstrap(ctx, s); err != nil {
			t.Fatal(err)
		}
		if _, err := EnsureDefaultCategories(ctx, s); err != nil {
			t.Fatal(err)
		}
		if again := countRows(t, s, `SELECT COUNT(*) FROM categories`); again != first {
			t.Fatalf("categorias duplicadas: %d -> %d", first, again)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM categories WHERE kind = 'EXPENSE' AND spend_type IS NULL`); n != 0 {
			t.Fatalf("despesas padrão sem tipo de gasto: %d", n)
		}
		tv, _ := s.FindCategoryByName(ctx, "TV", domain.CategoryExpense)
		if tv == nil || tv.ParentID == nil || tv.SpendType == nil || *tv.SpendType != domain.SpendCuttable {
			t.Fatalf("TV: %+v", tv)
		}
		if salario, _ := s.FindCategoryByName(ctx, "Salário", domain.CategoryIncome); salario == nil || salario.SpendType != nil {
			t.Fatalf("Salário: %+v", salario)
		}
	})

	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		// Banco antigo: categorias sem spend_type. O boot não cria a taxonomia e só preenche os NULL.
		moradia := mustCategory(t, s, "Moradia", domain.CategoryExpense, nil)
		pizza := mustCategory(t, s, "Pizza da sexta", domain.CategoryExpense, &moradia.ID)
		misc := mustCategory(t, s, "Coisas", domain.CategoryExpense, nil)
		netflix := mustCategory(t, s, "Assinaturas", domain.CategoryExpense, &moradia.ID)
		fixed := domain.SpendCuttable
		own := mustCategory(t, s, "Aluguel", domain.CategoryExpense, nil)
		own.SpendType = &fixed
		if err := s.UpdateCategory(ctx, own); err != nil {
			t.Fatal(err)
		}
		if err := Bootstrap(ctx, s); err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM categories`); n != 5 {
			t.Fatalf("boot não deveria criar padrão em banco com categorias: %d", n)
		}
		want := map[string]*domain.SpendType{
			moradia.ID: ptr(domain.SpendFixed),
			pizza.ID:   nil, // subcategoria sem palavra conhecida continua herdando
			misc.ID:    ptr(domain.SpendVariable),
			netflix.ID: ptr(domain.SpendCuttable),
			own.ID:     ptr(domain.SpendCuttable), // valor próprio não muda
		}
		for id, w := range want {
			got, _ := s.FindCategoryByID(ctx, id)
			if (got.SpendType == nil) != (w == nil) || (w != nil && *got.SpendType != *w) {
				t.Errorf("%s: spend_type = %v, want %v", got.Name, got.SpendType, w)
			}
		}
		// A heurística roda uma única vez: um NULL posterior (ex.: herdar do pai) é respeitado.
		misc.SpendType = nil
		if err := s.UpdateCategory(ctx, misc); err != nil {
			t.Fatal(err)
		}
		if err := Bootstrap(ctx, s); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.FindCategoryByID(ctx, misc.ID); got.SpendType != nil {
			t.Fatalf("backfill rodou de novo: %+v", got)
		}
	})
}

func TestInsightsAggregations(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		checking := mustAccount(t, s, "Corrente", 100000)
		savings, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Poupança", Type: domain.AccountSavings, InitialBalanceCents: 0})
		if err != nil {
			t.Fatal(err)
		}
		food := mustCategory(t, s, "Comida", domain.CategoryExpense, nil)

		mustTx(t, s, domain.TxIncome, "Salário", 500000, "2026-03-05", domain.TxConfirmed, checking.ID, nil)
		mustTx(t, s, domain.TxIncome, "A receber", 9999, "2026-03-06", domain.TxPending, checking.ID, nil)
		mustTx(t, s, domain.TxExpense, "Mercado", 20000, "2026-03-07", domain.TxConfirmed, checking.ID, &food.ID)
		mustTx(t, s, domain.TxExpense, "Mercado 2", 5000, "2026-03-07", domain.TxPending, checking.ID, &food.ID)
		mustTx(t, s, domain.TxExpense, "Sem cat", 3000, "2026-03-08", domain.TxConfirmed, checking.ID, nil)
		if _, err := s.CreateTransferPair(ctx, checking.ID, savings.ID, 70000, "2026-03-10", "→", "←"); err != nil {
			t.Fatal(err)
		}
		card, err := s.CreateCard(ctx, domain.CreateCardInput{Name: "Cartão", Brand: domain.BrandVisa, LimitCents: 100000, ClosingDay: 10, DueDay: 17})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateCardExpense(ctx, card.ID, cardLegs(t, card, "TV", 30000, "2026-03-01", 3, &food.ID)); err != nil {
			t.Fatal(err)
		}

		rows, err := s.SpendingRows(ctx, "2026-03-01", "2026-04-01")
		if err != nil {
			t.Fatal(err)
		}
		var foodTotal, noCat int64
		for _, r := range rows {
			if r.CategoryID == nil {
				noCat += r.TotalCents
			} else if *r.CategoryID == food.ID {
				foodTotal += r.TotalCents
				if r.Name == nil || *r.Name != "Comida" {
					t.Fatalf("nome da categoria: %+v", r)
				}
			}
		}
		// 20000 + 5000 (pendente) + 10000 (1ª parcela da TV)
		if foodTotal != 35000 || noCat != 3000 {
			t.Fatalf("gasto por categoria: comida=%d sem=%d", foodTotal, noCat)
		}
		income, _ := s.IncomeByDate(ctx, "2026-03-01", "2026-04-01")
		if len(income) != 1 || income[0].Cents != 500000 || income[0].Date != "2026-03-05" {
			t.Fatalf("IncomeByDate (pendente não conta): %+v", income)
		}
		saved, _ := s.SavingsTransfersByDate(ctx, "2026-03-01", "2026-04-01")
		if len(saved) != 1 || saved[0].Cents != 70000 {
			t.Fatalf("SavingsTransfersByDate: %+v", saved)
		}
		g, _ := s.CreateGoal(ctx, domain.CreateGoalInput{Name: "Meta", Icon: "x", TargetCents: 100})
		_ = s.AddGoalContribution(ctx, g.ID, 4000, "2026-03-15", nil)
		_ = s.AddGoalContribution(ctx, g.ID, -1000, "2026-03-15", nil)
		contribs, _ := s.GoalContributionsByDate(ctx, "2026-03-01", "2026-04-01")
		if len(contribs) != 1 || contribs[0].Cents != 3000 {
			t.Fatalf("GoalContributionsByDate: %+v", contribs)
		}
		before, _ := s.BalanceBefore(ctx, "2026-03-08")
		if before != 100000+500000-20000 {
			t.Fatalf("BalanceBefore = %d", before)
		}
		flows, _ := s.ConfirmedFlowsByDate(ctx, "2026-03-08", "2026-04-01")
		var flowSum int64
		for _, f := range flows {
			flowSum += f.Cents
		}
		if flowSum != -3000 {
			t.Fatalf("ConfirmedFlowsByDate (transferência entre contas ativas soma zero) = %d", flowSum)
		}
		legs, err := s.InstallmentLegs(ctx, domain.InvoicePeriod{Year: 2026, Month: 3}, domain.InvoicePeriod{Year: 2026, Month: 4})
		if err != nil || len(legs) != 2 || legs[0].AmountCents != 10000 || legs[0].InstallmentTotal != 3 || legs[0].CardName != "Cartão" || legs[0].DueDate() != "2026-03-17" {
			t.Fatalf("InstallmentLegs: %+v %v", legs, err)
		}
		pend, _ := s.PendingAccountTransactions(ctx)
		if len(pend) != 2 || pend[0].AmountCents != 9999 || pend[1].AmountCents != -5000 {
			t.Fatalf("PendingAccountTransactions: %+v", pend)
		}

		if r, _ := s.FindActiveReserveGoal(ctx); r != nil {
			t.Fatalf("sem reserva: %+v", r)
		}
		res, _ := s.CreateGoal(ctx, domain.CreateGoalInput{Name: "Reserva", Kind: domain.GoalKindReserve, Icon: "shield", TargetCents: 100})
		if r, _ := s.FindActiveReserveGoal(ctx); r == nil || r.ID != res.ID || r.Kind != domain.GoalKindReserve {
			t.Fatalf("FindActiveReserveGoal: %+v", r)
		}
		if plain, _ := s.FindGoalByID(ctx, g.ID); plain == nil || plain.Kind != domain.GoalKindGoal {
			t.Fatalf("kind padrão: %+v", plain)
		}
	})
}
