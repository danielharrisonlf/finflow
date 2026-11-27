package store

import (
	"context"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/testutil"
)

func TestMembersSplitsAndSettlements(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		acc := mustAccount(t, s, "Conta da casa", 100000)
		ana, err := s.CreateMember(ctx, "Ana", "#ec4899")
		if err != nil {
			t.Fatal(err)
		}
		bruno, err := s.CreateMember(ctx, "Bruno", "#3b82f6")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.UpdateMember(ctx, domain.Member{ID: bruno, Name: "Bruno S.", Color: "#000000"}); err != nil {
			t.Fatal(err)
		}
		members, err := s.ListMembers(ctx)
		if err != nil || len(members) != 2 || members[0].Name != "Ana" || members[1].Name != "Bruno S." {
			t.Fatalf("membros: %+v %v", members, err)
		}

		// Despesa com divisão 60/40 e outra sem divisão (igual), ambas pagas.
		rent := mustTx(t, s, domain.TxExpense, "Aluguel", 200000, "2026-05-05", domain.TxConfirmed, acc.ID, nil)
		if err := s.SetTransactionSharing(ctx, rent.ID, true, &ana, true, []domain.SplitShare{{MemberID: ana, ShareBP: 6000}, {MemberID: bruno, ShareBP: 4000}}); err != nil {
			t.Fatal(err)
		}
		food := mustTx(t, s, domain.TxExpense, "Mercado", 30001, "2026-05-10", domain.TxConfirmed, acc.ID, nil)
		if err := s.SetTransactionSharing(ctx, food.ID, true, &bruno, false, nil); err != nil {
			t.Fatal(err)
		}
		pending := mustTx(t, s, domain.TxExpense, "Gás", 9000, "2026-05-12", domain.TxPending, acc.ID, nil)
		if err := s.SetTransactionSharing(ctx, pending.ID, true, &ana, false, nil); err != nil {
			t.Fatal(err)
		}
		mustTx(t, s, domain.TxExpense, "Sem pagador", 5000, "2026-05-15", domain.TxConfirmed, acc.ID, nil)

		card := mustCard(t, s, 5)
		legs := cardLegs(t, card, "Sofá", 30000, "2026-05-20", 3, nil)
		for i := range legs {
			legs[i].PaidByMemberID = &bruno
			legs[i].Splits = []domain.SplitShare{{MemberID: ana, ShareBP: 5000}, {MemberID: bruno, ShareBP: 5000}}
		}
		if _, err := s.CreateCardExpense(ctx, card.ID, legs); err != nil {
			t.Fatal(err)
		}

		got, err := s.FindTransactionByID(ctx, rent.ID)
		if err != nil || got.PaidByMemberID == nil || *got.PaidByMemberID != ana || len(got.Splits) != 2 || got.Splits[0].SharePct != 60 || got.Splits[1].SharePct != 40 {
			t.Fatalf("aluguel: %+v %v", got, err)
		}
		list, _, err := s.ListTransactions(ctx, domain.ListTransactionsQuery{Page: 1, PageSize: 50})
		if err != nil {
			t.Fatal(err)
		}
		for _, tx := range list {
			if tx.Splits == nil {
				t.Fatalf("splits nil em %s", tx.Description)
			}
			if tx.CardID != nil && (tx.PaidByMemberID == nil || len(tx.Splits) != 2) {
				t.Fatalf("parcela sem divisão: %+v", tx)
			}
		}

		shared, err := s.SharedExpensesBetween(ctx, "2026-05-01", "2026-06-01")
		if err != nil {
			t.Fatal(err)
		}
		// aluguel, mercado e a 1ª parcela do sofá (a pendente e a sem pagador ficam fora).
		if len(shared) != 3 || shared[0].AmountCents != 200000 || len(shared[0].Splits) != 2 || shared[1].PaidBy != bruno || len(shared[1].Splits) != 0 || shared[2].AmountCents != 10000 {
			t.Fatalf("despesas compartilhadas: %+v", shared)
		}

		if _, err := s.CreateSettlement(ctx, bruno, ana, 5000, "2026-05-28", nil); err != nil {
			t.Fatal(err)
		}
		settled, err := s.SettlementsBetween(ctx, "2026-05-01", "2026-06-01")
		if err != nil || len(settled) != 1 || settled[0].Date != "2026-05-28" || settled[0].FromMemberID != bruno {
			t.Fatalf("acertos: %+v %v", settled, err)
		}

		// Limpar a divisão volta à divisão igual; apagar o membro remove divisões, acertos e "quem pagou".
		if err := s.SetTransactionSharing(ctx, rent.ID, false, nil, true, nil); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.FindTransactionByID(ctx, rent.ID); len(got.Splits) != 0 || got.PaidByMemberID == nil {
			t.Fatalf("sem divisão: %+v", got)
		}
		if err := s.DeleteMember(ctx, bruno); err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transaction_splits WHERE member_id = ?`, bruno); n != 0 {
			t.Fatalf("divisões do membro apagado = %d", n)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM settlements`); n != 0 {
			t.Fatalf("acertos do membro apagado = %d", n)
		}
		if got, _ := s.FindTransactionByID(ctx, food.ID); got.PaidByMemberID != nil {
			t.Fatalf("quem pagou deveria ficar nulo: %+v", got.PaidByMemberID)
		}
		if err := s.ClearAll(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBusinessStore(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		if err := BootstrapKind(ctx, s, domain.SpaceBusiness); err != nil {
			t.Fatal(err)
		}
		cats, err := s.ListCategories(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		byName := map[string]domain.Category{}
		for _, c := range cats {
			byName[c.Name] = c
		}
		checks := map[string]domain.DREGroup{
			"Vendas": domain.DRERevenue, "Impostos sobre vendas": domain.DREDeductions, "Infraestrutura/cloud": domain.DRECosts,
			"Pró-labore": domain.DREOpex, "Tarifas bancárias": domain.DREOpex, "Juros": domain.DREFinancial,
		}
		for name, want := range checks {
			c, ok := byName[name]
			if !ok || c.DREGroup == nil || *c.DREGroup != want {
				t.Errorf("categoria %s: %+v", name, c)
			}
		}
		if c := byName["Infraestrutura/cloud"]; c.SpendType == nil || *c.SpendType != domain.SpendCuttable || c.ParentID == nil {
			t.Errorf("Infraestrutura/cloud: %+v", c)
		}
		if _, ok := byName["Moradia"]; ok {
			t.Error("espaço Empresa não deveria ter a taxonomia pessoal")
		}
		// dre_group editável e nulo de volta.
		c := byName["Marketing"]
		g := domain.DREOther
		c.DREGroup = &g
		if err := s.UpdateCategory(ctx, &c); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.FindCategoryByID(ctx, c.ID); got.DREGroup == nil || *got.DREGroup != domain.DREOther {
			t.Fatalf("dre_group editado: %+v", got)
		}
		c.DREGroup = nil
		if err := s.UpdateCategory(ctx, &c); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.FindCategoryByID(ctx, c.ID); got.DREGroup != nil {
			t.Fatalf("dre_group nulo: %+v", got)
		}

		acc := mustAccount(t, s, "PJ", 0)
		id, err := s.CreateRecurring(ctx, RecurringRow{Description: "Cliente", Kind: domain.CategoryIncome, AmountCents: 1000, DayOfMonth: 5, StartDate: "2026-01-01", Active: false, DeactivatedAt: ptr("2026-06-10")})
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.FindRecurringByID(ctx, id)
		if err != nil || r.DeactivatedAt == nil || *r.DeactivatedAt != "2026-06-10" {
			t.Fatalf("deactivated_at: %+v %v", r, err)
		}
		r.DeactivatedAt = nil
		r.Active = true
		if err := s.UpdateRecurring(ctx, r); err != nil {
			t.Fatal(err)
		}
		if r, _ := s.FindRecurringByID(ctx, id); r.DeactivatedAt != nil {
			t.Fatalf("deactivated_at limpo: %+v", r)
		}

		mustTx(t, s, domain.TxIncome, "Venda", 5000, "2026-03-05", domain.TxConfirmed, acc.ID, ptr(byName["Vendas"].ID))
		mustTx(t, s, domain.TxExpense, "Pendente", 100, "2026-03-06", domain.TxPending, acc.ID, nil)
		mustTx(t, s, domain.TxExpense, "Ano anterior", 100, "2025-12-31", domain.TxConfirmed, acc.ID, nil)
		rows, err := s.DRETransactionsBetween(ctx, "2026-01-01", "2027-01-01")
		if err != nil || len(rows) != 1 || rows[0].AmountCents != 5000 || rows[0].Date != "2026-03-05" || rows[0].CategoryID == nil {
			t.Fatalf("DRE: %+v %v", rows, err)
		}
	})
}

func TestSpaceSeeds(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		if err := BootstrapKind(ctx, s, domain.SpaceHome); err != nil {
			t.Fatal(err)
		}
		ran, err := SeedHome(ctx, s)
		if err != nil || !ran {
			t.Fatalf("seed casa: %v %v", ran, err)
		}
		if ran, err := SeedHome(ctx, s); err != nil || ran {
			t.Fatalf("seed casa idempotente: %v %v", ran, err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM members`); n != 2 {
			t.Fatalf("membros = %d", n)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE paid_by_member_id IS NOT NULL`); n < 6 {
			t.Fatalf("despesas com quem pagou = %d", n)
		}
		if err := s.ClearAll(ctx); err != nil {
			t.Fatal(err)
		}

		ran, err = SeedBusiness(ctx, s)
		if err != nil || !ran {
			t.Fatalf("seed empresa: %v %v", ran, err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM recurring WHERE kind = 'INCOME' AND active = ? AND deactivated_at IS NOT NULL`, false); n != 1 {
			t.Fatalf("recorrências desativadas = %d", n)
		}
	})
}
