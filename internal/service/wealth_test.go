package service

import (
	"context"
	"net/http"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func TestInvestmentsServiceFlow(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		checking, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Corrente", Type: domain.AccountChecking, InitialBalanceCents: 1000000})
		if err != nil {
			t.Fatal(err)
		}
		broker, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Corretora", Type: domain.AccountInvestment, InitialBalanceCents: 200000})
		if err != nil {
			t.Fatal(err)
		}
		today := domain.TodayString()

		_, err = svcs.CreateInvestment(ctx, domain.CreateInvestmentInput{Name: "", Kind: "ACOES", InvestedCents: -1})
		assertValidation(t, err, "kind")
		_, err = svcs.CreateInvestment(ctx, domain.CreateInvestmentInput{Name: "X", Kind: domain.InvestmentStocks, AccountID: &checking.ID})
		assertStatus(t, err, http.StatusBadRequest)

		cdb, err := svcs.CreateInvestment(ctx, domain.CreateInvestmentInput{
			Name: "CDB", Kind: domain.InvestmentFixedIncome, Institution: ptrTo("Banco"), AccountID: &broker.ID, InvestedCents: 200000,
		})
		if err != nil {
			t.Fatal(err)
		}
		if cdb.CurrentCents != 200000 || cdb.ReturnCents != 0 || cdb.ReturnPct == nil || cdb.ValueUpdatedAt == nil || *cdb.ValueUpdatedAt != today || *cdb.AccountName != "Corretora" {
			t.Fatalf("posição criada = %+v", cdb)
		}
		loose, err := svcs.CreateInvestment(ctx, domain.CreateInvestmentInput{Name: "Cripto", Kind: domain.InvestmentCrypto, InvestedCents: 50000, CurrentCents: ptrTo(int64(40000))})
		if err != nil || loose.ReturnCents != -10000 || *loose.ReturnPct != -20 {
			t.Fatalf("posição avulsa = %+v %v", loose, err)
		}

		// Aporte com transferência da conta corrente para a conta vinculada.
		cdb, err = svcs.ContributeInvestment(ctx, cdb.ID, domain.InvestmentMovementInput{AmountCents: 100000, FromAccountID: &checking.ID})
		if err != nil || cdb.InvestedCents != 300000 || cdb.CurrentCents != 300000 {
			t.Fatalf("aporte = %+v %v", cdb, err)
		}
		acc, _ := svcs.GetAccount(ctx, checking.ID)
		if acc.BalanceCents != 900000 {
			t.Fatalf("saldo da origem após aporte = %d", acc.BalanceCents)
		}
		_, err = svcs.ContributeInvestment(ctx, loose.ID, domain.InvestmentMovementInput{AmountCents: 1000, FromAccountID: &checking.ID})
		assertValidation(t, err, "fromAccountId")
		_, err = svcs.ContributeInvestment(ctx, cdb.ID, domain.InvestmentMovementInput{AmountCents: 1000, FromAccountID: &broker.ID})
		assertValidation(t, err, "fromAccountId")

		cdb, err = svcs.UpdateInvestmentValue(ctx, cdb.ID, domain.UpdateInvestmentValueInput{CurrentCents: 330000})
		if err != nil || cdb.CurrentCents != 330000 || cdb.ReturnCents != 30000 || *cdb.ReturnPct != 10 {
			t.Fatalf("valor atualizado = %+v %v", cdb, err)
		}
		// Valor retroativo só entra no histórico.
		cdb, err = svcs.UpdateInvestmentValue(ctx, cdb.ID, domain.UpdateInvestmentValueInput{CurrentCents: 1, Date: ptrTo(domain.AddDays(today, -40))})
		if err != nil || cdb.CurrentCents != 330000 || *cdb.ValueUpdatedAt != today {
			t.Fatalf("valor retroativo alterou a posição: %+v %v", cdb, err)
		}

		_, err = svcs.RedeemInvestment(ctx, cdb.ID, domain.InvestmentMovementInput{AmountCents: 999999})
		assertStatus(t, err, http.StatusBadRequest)
		cdb, err = svcs.RedeemInvestment(ctx, cdb.ID, domain.InvestmentMovementInput{AmountCents: 30000, ToAccountID: &checking.ID})
		if err != nil || cdb.CurrentCents != 300000 || cdb.InvestedCents != 270000 {
			t.Fatalf("resgate = %+v %v", cdb, err)
		}
		acc, _ = svcs.GetAccount(ctx, checking.ID)
		if acc.BalanceCents != 930000 {
			t.Fatalf("saldo após resgate = %d", acc.BalanceCents)
		}

		sum, err := svcs.InvestmentSummary(ctx)
		if err != nil || sum.TotalCurrentCents != 340000 || sum.TotalInvestedCents != 320000 || len(sum.History) != 12 || len(sum.ByKind) != 2 {
			t.Fatalf("resumo = %+v %v", sum, err)
		}
		if last := sum.History[11]; last.CurrentCents != 340000 || last.InvestedCents != 320000 {
			t.Fatalf("histórico do mês corrente = %+v", last)
		}

		// Patrimônio: a conta Corretora vale as posições (300.000), não o saldo contábil.
		nw, err := svcs.netWorthBreakdown(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if nw.CashCents != 930000 || nw.InvestmentsCents != 300000+40000 || nw.DebtsCents != 0 {
			t.Fatalf("patrimônio = %+v", nw)
		}

		patch := domain.UpdateInvestmentInput{Archived: domain.Optional[bool]{Set: true, Valid: true, Value: true}}
		if _, err := svcs.UpdateInvestment(ctx, loose.ID, patch); err != nil {
			t.Fatal(err)
		}
		if list, _ := svcs.ListInvestments(ctx, false); len(list) != 1 {
			t.Fatalf("arquivada ainda listada: %+v", list)
		}
		_, err = svcs.ContributeInvestment(ctx, loose.ID, domain.InvestmentMovementInput{AmountCents: 1})
		assertStatus(t, err, http.StatusBadRequest)
		if err := svcs.DeleteInvestment(ctx, loose.ID); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, svcs.DeleteInvestment(ctx, loose.ID), http.StatusNotFound)
	})
}

func TestLoansServiceFlow(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		svcs := New(store.New(database), nil)
		acc, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Corrente", Type: domain.AccountChecking, InitialBalanceCents: 1000000})
		if err != nil {
			t.Fatal(err)
		}
		today := domain.TodayString()
		t0 := domain.FromDateString(today)
		y, m := t0.Year(), int(t0.Month())
		// 1º vencimento no dia 1 de dois meses atrás: as parcelas 1 e 2 já venceram e a 3 vence este mês.
		py, pm := domain.AddMonths(y, m, -2)
		first := domain.MonthStart(py, pm)

		_, err = svcs.CreateLoan(ctx, domain.CreateLoanInput{Name: "X", Kind: "CARRO", PrincipalCents: 100000, MonthlyRatePct: 2, TotalInstallments: 5, FirstDueDate: first})
		assertValidation(t, err, "kind")
		_, err = svcs.CreateLoan(ctx, domain.CreateLoanInput{Name: "X", Kind: domain.LoanVehicle, PrincipalCents: 100000, MonthlyRatePct: 2, TotalInstallments: 5, PaidInstallments: 6, FirstDueDate: first})
		assertValidation(t, err, "paidInstallments")

		loan, err := svcs.CreateLoan(ctx, domain.CreateLoanInput{
			Name: "Carro", Kind: domain.LoanVehicle, PrincipalCents: 100000, MonthlyRatePct: 2, TotalInstallments: 5,
			PaidInstallments: 1, FirstDueDate: first, AccountID: &acc.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if loan.InstallmentCents != 21216 || loan.MonthlyRatePct != 2 || loan.RemainingInstallments != 4 || loan.PaidInstallments != 1 {
			t.Fatalf("empréstimo criado = %+v", loan)
		}
		if loan.NextDueDate == nil || *loan.NextDueDate != domain.AddMonthsToDateString(first, 1) {
			t.Fatalf("próximo vencimento = %v", loan.NextDueDate)
		}
		if loan.OutstandingCents != domain.LoanOutstandingCents(21216, 200, 4) || loan.TotalInterestRemainingCents != 21216*4-loan.OutstandingCents {
			t.Fatalf("saldo devedor = %+v", loan)
		}

		// Pagar lança EXPENSE com loan_id na categoria padrão (criada se não existir).
		tx, err := svcs.PayLoan(ctx, loan.ID, domain.PayLoanInput{})
		if err != nil || tx.Type != domain.TxExpense || tx.AmountCents != -21216 || tx.CategoryName == nil || *tx.CategoryName != domain.LoanCategoryName || tx.Description != "Carro — parcela 2/5" || tx.LoanID == nil || *tx.LoanID != loan.ID {
			t.Fatalf("pagamento = %+v %v", tx, err)
		}
		loan, _ = svcs.GetLoan(ctx, loan.ID)
		if loan.PaidInstallments != 2 || loan.RemainingInstallments != 3 {
			t.Fatalf("após pagar: %+v", loan)
		}

		// Compromissos do mês: a parcela 3 vence agora e ainda não foi paga.
		c, err := svcs.InsightsCommitments(ctx, domain.PeriodQuery{Month: &m, Year: &y})
		if err != nil {
			t.Fatal(err)
		}
		var item *domain.CommitmentItem
		for i := range c.Items {
			if c.Items[i].Kind == "LOAN" {
				item = &c.Items[i]
			}
		}
		if item == nil || item.Realized || *item.InstallmentNumber != 3 || *item.LoanID != loan.ID || item.DueDate != domain.MonthStart(y, m) {
			t.Fatalf("item LOAN = %+v", item)
		}
		if c.Months[0].LoansCents != 21216 || c.Months[2].LoansCents != 21216 || c.Months[3].LoansCents != 0 {
			t.Fatalf("loansCents = %+v", c.Months)
		}

		// Projeção: a parcela 3 (vencida no dia 1) entra hoje; as 4 e 5 nos próximos meses.
		p, err := svcs.InsightsProjection(ctx, 365)
		if err != nil {
			t.Fatal(err)
		}
		var loanEvents int
		for _, d := range p.Days {
			for _, ev := range d.Events {
				if ev.Kind == domain.EventLoan {
					loanEvents++
				}
			}
		}
		if loanEvents != 3 || p.Months[0].LoansCents != 21216 {
			t.Fatalf("eventos LOAN = %d, meses = %+v", loanEvents, p.Months[:3])
		}

		// Patrimônio desconta o saldo devedor.
		nw, err := svcs.netWorthBreakdown(ctx)
		if err != nil || nw.DebtsCents != loan.OutstandingCents || nw.CashCents != 1000000-21216 {
			t.Fatalf("patrimônio = %+v (saldo devedor %d) %v", nw, loan.OutstandingCents, err)
		}
		hist, err := svcs.NetWorthHistory(ctx, 0)
		if err != nil || len(hist) != 12 {
			t.Fatalf("histórico = %d %v", len(hist), err)
		}
		if last := hist[11]; last.DebtsCents != loan.OutstandingCents || last.NetWorthCents != last.CashCents+last.InvestmentsCents-last.DebtsCents {
			t.Fatalf("último ponto = %+v", last)
		}
		if old := hist[0]; old.DebtsCents != 0 {
			t.Fatalf("antes da contratação não há dívida: %+v", old)
		}
		_, err = svcs.NetWorthHistory(ctx, 61)
		assertValidation(t, err, "months")

		// paidInstallments é o total: não pode ficar abaixo dos pagamentos lançados.
		_, err = svcs.UpdateLoan(ctx, loan.ID, domain.UpdateLoanInput{PaidInstallments: domain.Optional[int]{Set: true, Valid: true, Value: 0}})
		assertValidation(t, err, "paidInstallments")
		loan, err = svcs.UpdateLoan(ctx, loan.ID, domain.UpdateLoanInput{PaidInstallments: domain.Optional[int]{Set: true, Valid: true, Value: 4}})
		if err != nil || loan.PaidInstallments != 4 || loan.RemainingInstallments != 1 {
			t.Fatalf("após corrigir pagas = %+v %v", loan, err)
		}
		if _, err := svcs.PayLoan(ctx, loan.ID, domain.PayLoanInput{AmountCents: ptrTo(int64(21300))}); err != nil {
			t.Fatal(err)
		}
		_, err = svcs.PayLoan(ctx, loan.ID, domain.PayLoanInput{})
		assertStatus(t, err, http.StatusConflict)
		loan, _ = svcs.GetLoan(ctx, loan.ID)
		if loan.NextDueDate != nil || loan.OutstandingCents != 0 {
			t.Fatalf("quitado = %+v", loan)
		}

		// Excluir o pagamento devolve a parcela (pagas = informadas + lançadas).
		if err := svcs.DeleteTransaction(ctx, tx.ID, ""); err != nil {
			t.Fatal(err)
		}
		loan, _ = svcs.GetLoan(ctx, loan.ID)
		if loan.PaidInstallments != 4 {
			t.Fatalf("após excluir um pagamento: %+v", loan)
		}

		// Sem conta no empréstimo nem no corpo: 400 pedindo a conta.
		other, err := svcs.CreateLoan(ctx, domain.CreateLoanInput{Name: "Pessoal", Kind: domain.LoanPersonal, PrincipalCents: 50000, MonthlyRatePct: 5, InstallmentCents: ptrTo(int64(12000)), TotalInstallments: 6, FirstDueDate: today})
		if err != nil {
			t.Fatal(err)
		}
		_, err = svcs.PayLoan(ctx, other.ID, domain.PayLoanInput{})
		assertValidation(t, err, "accountId")

		strat, err := svcs.DebtStrategy(ctx, 10000)
		if err != nil || len(strat.Avalanche.Order) != 2 || strat.Avalanche.Order[0] != other.ID || len(strat.MinimumOnly.PerLoan) != 2 {
			t.Fatalf("estratégia = %+v %v", strat, err)
		}
		_, err = svcs.DebtStrategy(ctx, -1)
		assertValidation(t, err, "extraCents")

		if err := svcs.DeleteLoan(ctx, other.ID); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, svcs.DeleteLoan(ctx, other.ID), http.StatusNotFound)
		if list, _ := svcs.ListLoans(ctx); len(list) != 1 {
			t.Fatalf("lista = %+v", list)
		}
	})
}
