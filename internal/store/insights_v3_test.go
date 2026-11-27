package store

import (
	"context"
	"errors"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/testutil"
)

func TestTagsStore(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		acc := mustAccount(t, s, "Conta", 0)
		card := mustCard(t, s, 5)

		trip, err := s.CreateTag(ctx, "Viagem", "#ff0000")
		if err != nil {
			t.Fatal(err)
		}
		work, _ := s.CreateTag(ctx, "Trabalho", "#00ff00")
		if _, err := s.CreateTag(ctx, "Viagem", "#000000"); err == nil {
			t.Fatal("nome de tag repetido deveria falhar (UNIQUE)")
		}

		tx := mustTx(t, s, domain.TxExpense, "Hotel", 50000, "2026-11-10", domain.TxConfirmed, acc.ID, nil)
		if tx.Tags == nil || len(tx.Tags) != 0 {
			t.Fatalf("tags deveria ser [] e não nil: %+v", tx.Tags)
		}
		if err := s.SetTransactionTags(ctx, tx.ID, []string{trip, work}); err != nil {
			t.Fatal(err)
		}
		detail, _ := s.FindTransactionByID(ctx, tx.ID)
		if len(detail.Tags) != 2 || detail.Tags[0].Name != "Trabalho" || detail.Tags[1].Color != "#ff0000" {
			t.Fatalf("tags lidas: %+v", detail.Tags)
		}
		if err := s.SetTransactionTags(ctx, tx.ID, []string{trip}); err != nil {
			t.Fatal(err)
		}

		legs := cardLegs(t, card, "Passagem", 30000, "2026-11-10", 3, nil)
		for i := range legs {
			legs[i].TagIDs = []string{trip}
		}
		if _, err := s.CreateCardExpense(ctx, card.ID, legs); err != nil {
			t.Fatal(err)
		}
		mustTx(t, s, domain.TxExpense, "Sem tag", 1000, "2026-11-11", domain.TxConfirmed, acc.ID, nil)

		items, total, err := s.ListTransactions(ctx, domain.ListTransactionsQuery{Page: 1, PageSize: 50, TagID: &trip})
		if err != nil || total != 4 || len(items) != 4 {
			t.Fatalf("filtro por tag: %d %d %v", total, len(items), err)
		}
		for _, it := range items {
			if len(it.Tags) != 1 || it.Tags[0].ID != trip {
				t.Fatalf("tags da listagem: %+v", it)
			}
		}

		tags, err := s.ListTags(ctx)
		if err != nil || len(tags) != 2 || tags[1].Name != "Viagem" || tags[1].UsageCount != 4 || tags[0].UsageCount != 0 {
			t.Fatalf("ListTags: %+v %v", tags, err)
		}
		spend, err := s.TagSpendingBetween(ctx, "2026-11-01", "2026-12-01")
		if err != nil || len(spend) != 1 || spend[0].TotalCents != 60000 || spend[0].Count != 2 {
			t.Fatalf("TagSpendingBetween: %+v %v", spend, err)
		}
		exist, err := s.ExistingTagIDs(ctx, []string{trip, "11111111-1111-4111-8111-111111111111"})
		if err != nil || !exist[trip] || len(exist) != 1 {
			t.Fatalf("ExistingTagIDs: %+v %v", exist, err)
		}
		n, err := s.AddTransactionTags(ctx, map[string][]string{tx.ID: {trip, work}})
		if err != nil || len(n) != 1 || !n[tx.ID] {
			t.Fatalf("AddTransactionTags: %v %v", n, err)
		}
		if err := s.DeleteTag(ctx, trip); err != nil {
			t.Fatal(err)
		}
		if c := countRows(t, s, `SELECT COUNT(*) FROM transaction_tags WHERE tag_id = ?`, trip); c != 0 {
			t.Fatalf("excluir tag deveria remover vínculos (CASCADE): %d", c)
		}
	})
}

func TestInsightsV3Store(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		a := mustAccount(t, s, "A", 0)
		b := mustAccount(t, s, "B", 0)
		card := mustCard(t, s, 5)

		// Recorrência com is_subscription.
		rid, err := s.CreateRecurring(ctx, RecurringRow{Description: "Netflix", Kind: domain.CategoryExpense, AmountCents: 5590, DayOfMonth: 10, StartDate: "2026-01-01", Active: true, IsSubscription: true})
		if err != nil {
			t.Fatal(err)
		}
		r, _ := s.FindRecurringByID(ctx, rid)
		if !r.IsSubscription {
			t.Fatalf("is_subscription não persistiu: %+v", r)
		}

		mustTx(t, s, domain.TxExpense, "Spotify", 2000, "2026-10-05", domain.TxConfirmed, a.ID, nil)
		if _, err := s.CreateCardExpense(ctx, card.ID, cardLegs(t, card, "TV", 30000, "2026-10-10", 3, nil)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateCardExpense(ctx, card.ID, cardLegs(t, card, "Netflix", 5590, "2026-10-10", 1, nil)); err != nil {
			t.Fatal(err)
		}
		subs, err := s.SubscriptionCandidates(ctx, "2026-01-01")
		if err != nil || len(subs) != 2 || subs[0].AmountCents != 2000 || subs[1].CardID == nil {
			t.Fatalf("SubscriptionCandidates (sem parcelas): %+v %v", subs, err)
		}

		mustTx(t, s, domain.TxIncome, "Salário", 500000, "2026-10-01", domain.TxConfirmed, a.ID, nil)
		mustTx(t, s, domain.TxIncome, "Pendente", 100, "2026-10-02", domain.TxPending, a.ID, nil)
		flows, err := s.CashflowTxs(ctx, "2026-10-01", "2026-11-01")
		if err != nil {
			t.Fatal(err)
		}
		var income, cardOut int64
		var dues []string
		for _, f := range flows {
			if f.Income {
				income += f.AmountCents
			}
			if f.DueDate != nil {
				cardOut += f.AmountCents
				dues = append(dues, *f.DueDate)
			}
		}
		// Fecha dia 5, vence dia 12: compras de 10/10 caem na fatura de novembro.
		if income != 500000 || cardOut != 10000+5590 || len(dues) != 2 || dues[0] != "2026-11-12" {
			t.Fatalf("CashflowTxs: income=%d card=%d dues=%v", income, cardOut, dues)
		}

		// Composição da fatura de novembro.
		invoices, _ := s.FindInvoicesWithTotals(ctx, card.ID)
		var novID string
		for _, inv := range invoices {
			if inv.Month == 11 && inv.Year == 2026 {
				novID = inv.ID
			}
		}
		lines, paid, err := s.InvoiceLines(ctx, novID)
		if err != nil || len(lines) != 2 || paid != 0 {
			t.Fatalf("InvoiceLines: %+v %d %v", lines, paid, err)
		}
		if err := s.PayInvoice(ctx, novID, a.ID, "2026-11-12", "Pagamento", 15590); err != nil {
			t.Fatal(err)
		}
		if _, paid, _ = s.InvoiceLines(ctx, novID); paid != 15590 {
			t.Fatalf("pagamento da fatura: %d", paid)
		}

		// Transferência inteligente.
		exp := mustTx(t, s, domain.TxExpense, "PIX enviado", 10000, "2026-11-20", domain.TxConfirmed, a.ID, nil)
		inc := mustTx(t, s, domain.TxIncome, "PIX recebido", 10000, "2026-11-21", domain.TxConfirmed, b.ID, nil)
		expenses, incomes, err := s.TransferCandidateTxs(ctx, "2026-11-15")
		if err != nil || len(expenses) != 1 || len(incomes) != 1 {
			t.Fatalf("TransferCandidateTxs: %d %d %v", len(expenses), len(incomes), err)
		}
		group, err := s.MergeTransferLegs(ctx, exp.ID, inc.ID)
		if err != nil || group == "" {
			t.Fatalf("MergeTransferLegs: %v", err)
		}
		merged, _ := s.FindTransactionByID(ctx, inc.ID)
		if merged.Type != domain.TxTransfer || merged.TransferGroupID == nil || *merged.TransferGroupID != group || merged.CategoryID != nil {
			t.Fatalf("perna convertida: %+v", merged)
		}
		if _, err := s.MergeTransferLegs(ctx, exp.ID, inc.ID); !errors.Is(err, ErrConflict) {
			t.Fatalf("mesclar de novo deveria ser conflito: %v", err)
		}

		// Insumos dos alertas.
		mustTx(t, s, domain.TxExpense, "Antiga", 100, "2026-01-01", domain.TxPending, a.ID, nil)
		dates, err := s.PendingTransactionDates(ctx)
		if err != nil || len(dates) != 2 {
			t.Fatalf("PendingTransactionDates: %v %v", dates, err)
		}
		if n, err := s.UncategorizedCount(ctx, "2026-10-01", "2026-11-01"); err != nil || n != 5 {
			t.Fatalf("UncategorizedCount: %d %v", n, err)
		}
		goal, _ := s.CreateGoal(ctx, domain.CreateGoalInput{Name: "Viagem", Icon: "x", TargetCents: 100000})
		_ = s.AddGoalContribution(ctx, goal.ID, 3000, "2026-11-20", nil)
		_ = s.AddGoalContribution(ctx, goal.ID, 9000, "2026-01-20", nil)
		if got, err := s.GoalContributionsSince(ctx, "2026-11-01"); err != nil || got[goal.ID] != 3000 {
			t.Fatalf("GoalContributionsSince: %v %v", got, err)
		}
		if bal, err := s.ReserveAccountsBalanceCents(ctx); err != nil || bal != 0 {
			t.Fatalf("ReserveAccountsBalanceCents: %d %v", bal, err)
		}
		if invs, err := s.InvestmentValueDates(ctx); err != nil || len(invs) != 0 {
			t.Fatalf("InvestmentValueDates: %+v %v", invs, err)
		}

		// Regra com tag e desejo com imagem.
		cat := mustCategory(t, s, "Transporte", domain.CategoryExpense, nil)
		tag, _ := s.CreateTag(ctx, "Uber", "#000000")
		ruleID, _ := s.CreateCategoryRule(ctx, "uber", cat.ID, 0)
		if err := s.SetCategoryRuleTag(ctx, ruleID, &tag); err != nil {
			t.Fatal(err)
		}
		rule, _ := s.FindCategoryRuleByID(ctx, ruleID)
		if rule.TagID == nil || *rule.TagID != tag || rule.TagName == nil || *rule.TagName != "Uber" {
			t.Fatalf("regra com tag: %+v", rule)
		}
		img := "https://img.example/x.jpg"
		wid, _ := s.CreateWishItem(ctx, WishItemRow{Name: "Fone", PriceCents: 100, Status: domain.WishListed, ImageURL: &img})
		w, _ := s.FindWishItemByID(ctx, wid)
		if w.ImageURL == nil || *w.ImageURL != img {
			t.Fatalf("image_url: %+v", w)
		}
	})
}
