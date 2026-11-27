package store

import (
	"context"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/testutil"
)

func TestInvestmentsCRUDAndSnapshots(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		src := mustAccount(t, s, "Corrente", 100000)
		broker, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Corretora", Type: domain.AccountInvestment})
		if err != nil {
			t.Fatal(err)
		}

		id, err := s.CreateInvestment(ctx, InvestmentRow{
			Name: "CDB", Kind: domain.InvestmentFixedIncome, Institution: ptr("Banco"), AccountID: &broker.ID,
			InvestedCents: 50000, CurrentCents: 50000, ValueUpdatedAt: ptr("2026-10-01"),
		}, &domain.InvestmentSnapshot{Date: "2026-10-01", ValueCents: 50000, InvestedCents: 50000})
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.FindInvestmentByID(ctx, id)
		if err != nil || got == nil || got.AccountName == nil || *got.AccountName != "Corretora" || *got.ValueUpdatedAt != "2026-10-01" || got.Archived {
			t.Fatalf("posição criada = %+v %v", got, err)
		}

		// Aporte com transferência: posição, snapshot e as duas pernas numa transação.
		got.InvestedCents, got.CurrentCents = 80000, 81000
		err = s.SaveInvestment(ctx, got, &domain.InvestmentSnapshot{Date: "2026-10-01", ValueCents: 81000, InvestedCents: 80000}, &TransferSpec{
			FromAccountID: src.ID, ToAccountID: broker.ID, AmountCents: 30000, Date: "2026-10-01", FromDesc: "Aporte →", ToDesc: "Aporte ←",
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM investment_snapshots WHERE investment_id = ?`, id); n != 1 {
			t.Fatalf("snapshot do mesmo dia deveria ser substituído: %d", n)
		}
		sums, err := s.ConfirmedSumsByAccount(ctx)
		if err != nil || sums[src.ID] != -30000 || sums[broker.ID] != 30000 {
			t.Fatalf("transferência do aporte: %+v %v", sums, err)
		}
		if err := s.SaveInvestment(ctx, got, &domain.InvestmentSnapshot{Date: "2026-11-01", ValueCents: 82000, InvestedCents: 80000}, nil); err != nil {
			t.Fatal(err)
		}
		snaps, err := s.InvestmentSnapshots(ctx)
		if err != nil || len(snaps[id]) != 2 || snaps[id][0].Date != "2026-10-01" || snaps[id][1].ValueCents != 82000 {
			t.Fatalf("snapshots = %+v %v", snaps, err)
		}

		got.Archived = true
		if err := s.SaveInvestment(ctx, got, nil, nil); err != nil {
			t.Fatal(err)
		}
		if active, _ := s.ListInvestments(ctx, false); len(active) != 0 {
			t.Fatalf("arquivada não deveria aparecer: %+v", active)
		}
		if all, _ := s.ListInvestments(ctx, true); len(all) != 1 {
			t.Fatalf("com arquivadas: %+v", all)
		}
		missing := *got
		missing.ID = "11111111-1111-4111-8111-111111111111"
		if err := s.SaveInvestment(ctx, &missing, nil, nil); err != ErrNotFound {
			t.Fatalf("salvar inexistente = %v", err)
		}
		if err := s.DeleteInvestment(ctx, id); err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM investment_snapshots`); n != 0 {
			t.Fatalf("snapshots órfãos = %d", n)
		}
	})
}

func TestLoansCRUDAndPayments(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		acc := mustAccount(t, s, "Corrente", 500000)
		cat := mustCategory(t, s, "Empréstimos", domain.CategoryExpense, nil)

		id, err := s.CreateLoan(ctx, LoanRow{
			Name: "Carro", Kind: domain.LoanVehicle, PrincipalCents: 100000, RateBP: 200, InstallmentCents: 21216,
			TotalInstallments: 5, BasePaid: 1, FirstDueDate: "2026-09-10", AccountID: &acc.ID, Active: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		l, err := s.FindLoanByID(ctx, id)
		if err != nil || l == nil || l.RateBP != 200 || l.FirstDueDate != "2026-09-10" || l.BasePaid != 1 || !l.Active || l.Lender != nil {
			t.Fatalf("empréstimo = %+v %v", l, err)
		}
		tx, err := s.CreateLoanPayment(ctx, id, "Carro — parcela 2/5", 21216, "2026-10-10", acc.ID, &cat.ID)
		if err != nil || tx.Type != domain.TxExpense || tx.AmountCents != -21216 || tx.CategoryID == nil {
			t.Fatalf("pagamento = %+v %v", tx, err)
		}
		if _, err := s.CreateLoanPayment(ctx, id, "Carro — parcela 3/5", 21216, "2026-11-10", acc.ID, nil); err != nil {
			t.Fatal(err)
		}
		dates, err := s.LoanPaymentDates(ctx)
		if err != nil || len(dates[id]) != 2 || dates[id][0] != "2026-10-10" {
			t.Fatalf("datas de pagamento = %+v %v", dates, err)
		}
		until, err := s.ConfirmedSumsByAccountUntil(ctx, "2026-10-31")
		if err != nil || until[acc.ID] != -21216 {
			t.Fatalf("soma até a data = %+v %v", until, err)
		}

		l.Name, l.Lender, l.Active = "Carro novo", ptr("Banco"), false
		if err := s.UpdateLoan(ctx, l); err != nil {
			t.Fatal(err)
		}
		list, err := s.ListLoans(ctx)
		if err != nil || len(list) != 1 || list[0].Name != "Carro novo" || list[0].Active || *list[0].Lender != "Banco" {
			t.Fatalf("lista = %+v %v", list, err)
		}

		// Excluir o empréstimo mantém os pagamentos como despesas comuns (loan_id vira NULL).
		if err := s.DeleteLoan(ctx, id); err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE loan_id IS NULL AND type = 'EXPENSE'`); n != 2 {
			t.Fatalf("pagamentos após excluir o empréstimo = %d", n)
		}
	})
}

func TestInvoiceDebtRows(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		acc := mustAccount(t, s, "Corrente", 500000)
		card := mustCard(t, s, 5)
		if _, err := s.CreateCardExpense(ctx, card.ID, cardLegs(t, card, "TV", 30000, "2026-09-10", 3, nil)); err != nil {
			t.Fatal(err)
		}
		rows, err := s.InvoiceDebtRows(ctx)
		if err != nil || len(rows) != 3 {
			t.Fatalf("faturas = %+v %v", rows, err)
		}
		inv, err := s.FindInvoicesWithTotals(ctx, card.ID)
		if err != nil || len(inv) == 0 {
			t.Fatal(err)
		}
		if err := s.PayInvoice(ctx, inv[0].ID, acc.ID, "2026-11-01", "Pagamento", inv[0].TotalCents); err != nil {
			t.Fatal(err)
		}
		rows, _ = s.InvoiceDebtRows(ctx)
		var paid *InvoiceDebtRow
		for i := range rows {
			if rows[i].Paid {
				paid = &rows[i]
			}
		}
		if paid == nil || paid.PaymentDate == nil || *paid.PaymentDate != "2026-11-01" || paid.TotalCents != 10000 {
			t.Fatalf("fatura paga = %+v", paid)
		}
		if !paid.UnpaidAt("2026-10-31") || paid.UnpaidAt("2026-11-01") {
			t.Errorf("UnpaidAt deveria usar a data do pagamento")
		}
		if paid.DueDate() == "" {
			t.Errorf("vencimento vazio")
		}
	})
}

func TestSeedWealth(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		if _, err := Seed(ctx, s); err != nil {
			t.Fatal(err)
		}
		inv, err := s.ListInvestments(ctx, false)
		if err != nil || len(inv) != 2 {
			t.Fatalf("investimentos do seed = %+v %v", inv, err)
		}
		snaps, err := s.InvestmentSnapshots(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range inv {
			if p.AccountName == nil || *p.AccountName != "XP Investimentos" || len(snaps[p.ID]) != 6 {
				t.Fatalf("posição do seed = %+v (%d snapshots)", p, len(snaps[p.ID]))
			}
			last := snaps[p.ID][len(snaps[p.ID])-1]
			if last.ValueCents != p.CurrentCents || p.ValueUpdatedAt == nil || *p.ValueUpdatedAt != last.Date || last.Date > domain.TodayString() {
				t.Fatalf("último snapshot %+v não bate com a posição %+v", last, p)
			}
		}
		loans, err := s.ListLoans(ctx)
		if err != nil || len(loans) != 1 || loans[0].Kind != domain.LoanVehicle || loans[0].BasePaid != 7 {
			t.Fatalf("empréstimo do seed = %+v %v", loans, err)
		}
		dates, _ := s.LoanPaymentDates(ctx)
		if n := len(dates[loans[0].ID]); n < 2 || n > 3 {
			t.Fatalf("pagamentos lançados no seed = %d", n)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE loan_id IS NOT NULL AND category_id IS NULL`); n != 0 {
			t.Fatalf("pagamentos do seed sem categoria = %d", n)
		}
		if err := s.ClearAll(ctx); err != nil {
			t.Fatal(err)
		}
		for _, tbl := range []string{"investments", "investment_snapshots", "loans"} {
			if n := countRows(t, s, `SELECT COUNT(*) FROM `+tbl); n != 0 {
				t.Fatalf("%s após ClearAll = %d", tbl, n)
			}
		}
	})
}
