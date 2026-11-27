package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/testutil"
)

// ───────────────────────── helpers ─────────────────────────

func ptr[T any](v T) *T { return &v }

func mustAccount(t *testing.T, s *Store, name string, initial int64) *AccountRow {
	t.Helper()
	a, err := s.CreateAccount(context.Background(), domain.CreateAccountInput{
		Name: name, Type: domain.AccountChecking, InitialBalanceCents: initial,
	})
	if err != nil {
		t.Fatalf("criar conta %s: %v", name, err)
	}
	return a
}

func mustCategory(t *testing.T, s *Store, name string, kind domain.CategoryKind, parentID *string) *domain.Category {
	t.Helper()
	c, err := s.CreateCategory(context.Background(), domain.CreateCategoryInput{
		Name: name, Kind: kind, Icon: "tag", Color: "#000000", ParentID: parentID,
	})
	if err != nil {
		t.Fatalf("criar categoria %s: %v", name, err)
	}
	return c
}

func mustTx(t *testing.T, s *Store, typ domain.TransactionType, desc string, amount int64, date string, status domain.TransactionStatus, accountID string, categoryID *string) *domain.Transaction {
	t.Helper()
	tx, err := s.CreateTransaction(context.Background(), typ, desc, domain.SignedAmount(typ, amount), date, status, accountID, categoryID, nil)
	if err != nil {
		t.Fatalf("criar transação %s: %v", desc, err)
	}
	return tx
}

func mustCard(t *testing.T, s *Store, closingDay int) *CardRow {
	t.Helper()
	c, err := s.CreateCard(context.Background(), domain.CreateCardInput{
		Name: "Cartão", Brand: domain.BrandVisa, LimitCents: 500000, ClosingDay: closingDay, DueDay: closingDay + 7,
	})
	if err != nil {
		t.Fatalf("criar cartão: %v", err)
	}
	return c
}

// cardLegs monta as parcelas como o serviço faz (domínio decide período e valores).
func cardLegs(t *testing.T, card *CardRow, desc string, total int64, date string, installments int, categoryID *string) []CardExpenseLeg {
	t.Helper()
	amounts, err := domain.SplitInstallments(total, installments)
	if err != nil {
		t.Fatal(err)
	}
	base := domain.InvoicePeriodForPurchase(date, card.ClosingDay)
	legs := make([]CardExpenseLeg, installments)
	for i, amt := range amounts {
		leg := CardExpenseLeg{
			Description:   desc,
			AmountCents:   amt,
			Date:          domain.AddMonthsToDateString(date, i),
			CategoryID:    categoryID,
			InvoicePeriod: domain.AddMonthsToPeriod(base, i),
		}
		if installments > 1 {
			leg.Description = fmt.Sprintf("%s (%d/%d)", desc, i+1, installments)
			leg.InstallmentNumber = ptr(i + 1)
			leg.InstallmentTotal = ptr(installments)
		}
		legs[i] = leg
	}
	return legs
}

func countRows(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("contar (%s): %v", query, err)
	}
	return n
}

func descriptions(items []domain.Transaction) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Description
	}
	return out
}

// ───────────────────────── Accounts ─────────────────────────

func TestAccountsCRUDAndBalance(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)

		nubank := mustAccount(t, s, "Nubank", 100000)
		carteira := mustAccount(t, s, "Carteira", 5000)

		got, err := s.FindAccountByID(ctx, nubank.ID)
		if err != nil || got == nil {
			t.Fatalf("FindAccountByID: %v %v", got, err)
		}
		if got.Name != "Nubank" || got.InitialBalanceCents != 100000 || got.Type != domain.AccountChecking || got.Archived {
			t.Fatalf("conta lida difere: %+v", got)
		}
		if missing, err := s.FindAccountByID(ctx, uuid.NewString()); err != nil || missing != nil {
			t.Fatalf("conta inexistente deveria ser nil: %v %v", missing, err)
		}

		mustTx(t, s, domain.TxIncome, "Salário", 300000, "2026-03-05", domain.TxConfirmed, nubank.ID, nil)
		mustTx(t, s, domain.TxExpense, "Aluguel", 120000, "2026-03-10", domain.TxConfirmed, nubank.ID, nil)
		mustTx(t, s, domain.TxExpense, "Conta futura", 9999, "2026-03-20", domain.TxPending, nubank.ID, nil)
		mustTx(t, s, domain.TxExpense, "Padaria", 1500, "2026-03-11", domain.TxConfirmed, carteira.ID, nil)

		sums, err := s.ConfirmedSumsByAccount(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if sums[nubank.ID] != 180000 || sums[carteira.ID] != -1500 {
			t.Fatalf("somas confirmadas = %v", sums)
		}
		total, err := s.TotalBalanceCents(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if want := int64(100000 + 5000 + 180000 - 1500); total != want {
			t.Fatalf("saldo total = %d, esperado %d", total, want)
		}
		if n, err := s.CountAccountTransactions(ctx, nubank.ID); err != nil || n != 3 {
			t.Fatalf("CountAccountTransactions = %d %v", n, err)
		}

		// Arquivar remove a conta da listagem padrão e do saldo total.
		carteira.Archived = true
		carteira.Name = "Carteira antiga"
		if err := s.UpdateAccount(ctx, carteira); err != nil {
			t.Fatal(err)
		}
		active, err := s.ListAccounts(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(active) != 1 || active[0].ID != nubank.ID {
			t.Fatalf("contas ativas = %+v", active)
		}
		all, err := s.ListAccounts(ctx, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != 2 {
			t.Fatalf("todas as contas = %+v", all)
		}
		total, err = s.TotalBalanceCents(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if want := int64(100000 + 180000); total != want {
			t.Fatalf("saldo total sem arquivadas = %d, esperado %d", total, want)
		}

		// Excluir a conta remove suas transações (ON DELETE CASCADE).
		if err := s.DeleteAccount(ctx, carteira.ID); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.FindAccountByID(ctx, carteira.ID); got != nil {
			t.Fatal("conta excluída ainda encontrada")
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE account_id = ?`, carteira.ID); n != 0 {
			t.Fatalf("transações órfãs = %d", n)
		}
		if n, err := s.CountAccounts(ctx); err != nil || n != 1 {
			t.Fatalf("CountAccounts = %d %v", n, err)
		}
	})
}

// ───────────────────────── Categories ─────────────────────────

func TestCategories(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)

		moradia := mustCategory(t, s, "Moradia", domain.CategoryExpense, nil)
		aluguel := mustCategory(t, s, "Aluguel", domain.CategoryExpense, &moradia.ID)
		mustCategory(t, s, "Salário", domain.CategoryIncome, nil)
		mustCategory(t, s, "Alimentação", domain.CategoryExpense, nil)
		mustCategory(t, s, "Freelance", domain.CategoryIncome, nil)

		all, err := s.ListCategories(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var order []string
		for _, c := range all {
			order = append(order, string(c.Kind)+":"+c.Name)
		}
		// INCOME primeiro (ordem do enum original), depois por nome.
		want := []string{"INCOME:Freelance", "INCOME:Salário", "EXPENSE:Alimentação", "EXPENSE:Aluguel", "EXPENSE:Moradia"}
		if strings.Join(order, ",") != strings.Join(want, ",") {
			t.Fatalf("ordem = %v, esperado %v", order, want)
		}

		kind := domain.CategoryIncome
		incomes, err := s.ListCategories(ctx, &kind)
		if err != nil {
			t.Fatal(err)
		}
		if len(incomes) != 2 {
			t.Fatalf("categorias de receita = %+v", incomes)
		}

		child, err := s.FindCategoryByID(ctx, aluguel.ID)
		if err != nil || child == nil || child.ParentID == nil || *child.ParentID != moradia.ID {
			t.Fatalf("categoria filha = %+v %v", child, err)
		}
		byName, err := s.FindCategoryByName(ctx, "moradia", domain.CategoryExpense)
		if err != nil || byName == nil || byName.ID != moradia.ID {
			t.Fatalf("FindCategoryByName = %+v %v", byName, err)
		}

		// (name, kind) é único.
		if _, err := s.CreateCategory(ctx, domain.CreateCategoryInput{Name: "Moradia", Kind: domain.CategoryExpense, Icon: "x", Color: "#fff"}); err == nil {
			t.Fatal("categoria duplicada deveria falhar")
		}

		child.Name = "Aluguel apto"
		if err := s.UpdateCategory(ctx, child); err != nil {
			t.Fatal(err)
		}
		// Excluir o pai solta o filho (ON DELETE SET NULL).
		if err := s.DeleteCategory(ctx, moradia.ID); err != nil {
			t.Fatal(err)
		}
		child, err = s.FindCategoryByID(ctx, aluguel.ID)
		if err != nil || child == nil {
			t.Fatalf("filha após excluir pai: %v %v", child, err)
		}
		if child.Name != "Aluguel apto" || child.ParentID != nil {
			t.Fatalf("filha após excluir pai = %+v", child)
		}
	})
}

// ───────────────────────── Transactions ─────────────────────────

func TestTimestampLayoutOrdersLexically(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	times := []time.Time{
		base,
		base.Add(450 * time.Millisecond),
		base.Add(500 * time.Millisecond),
		base.Add(time.Second),
	}
	var formatted []string
	for _, tm := range times {
		f := tm.Format(timestampLayout)
		if len(f) != len("2026-03-01T10:00:00.000000000Z") {
			t.Fatalf("largura variável: %q", f)
		}
		parsed, err := time.Parse(time.RFC3339Nano, f)
		if err != nil || !parsed.Equal(tm) {
			t.Fatalf("formato não é RFC3339 válido: %q (%v)", f, err)
		}
		formatted = append(formatted, f)
	}
	if !sort.StringsAreSorted(formatted) {
		t.Fatalf("ordem lexical difere da cronológica: %v", formatted)
	}
	if n := nowStr(); !strings.HasSuffix(n, "Z") || len(n) != len(formatted[0]) {
		t.Fatalf("nowStr fora do formato: %q", n)
	}
}

func TestListTransactionsOrderByCreatedAt(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		acc := mustAccount(t, s, "Conta", 0)

		// Mesma data: o desempate é created_at DESC. Com RFC3339Nano, ".5Z" > ".45Z"
		// lexicalmente estaria certo, mas "00Z" > "00.5Z" não — o layout fixo resolve.
		base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
		for i, off := range []time.Duration{0, 450 * time.Millisecond, 500 * time.Millisecond} {
			ts := base.Add(off).Format(timestampLayout)
			if _, err := s.DB.Exec(ctx, `
				INSERT INTO transactions (id, type, description, amount_cents, date, status, account_id, created_at, updated_at)
				VALUES (?, 'EXPENSE', ?, -100, '2026-03-01', 'CONFIRMED', ?, ?, ?)
			`, uuid.NewString(), fmt.Sprintf("t%d", i), acc.ID, ts, ts); err != nil {
				t.Fatal(err)
			}
		}
		items, _, err := s.ListTransactions(ctx, domain.ListTransactionsQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(descriptions(items), ","); got != "t2,t1,t0" {
			t.Fatalf("ordem = %s, esperado t2,t1,t0", got)
		}
	})
}

func TestListTransactionsFiltersAndPagination(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		a1 := mustAccount(t, s, "A1", 0)
		a2 := mustAccount(t, s, "A2", 0)
		food := mustCategory(t, s, "Comida", domain.CategoryExpense, nil)

		mustTx(t, s, domain.TxExpense, "jan-1", 1000, "2026-01-10", domain.TxConfirmed, a1.ID, &food.ID)
		mustTx(t, s, domain.TxIncome, "jan-2", 5000, "2026-01-20", domain.TxConfirmed, a1.ID, nil)
		mustTx(t, s, domain.TxExpense, "fev-1", 2000, "2026-02-01", domain.TxPending, a2.ID, &food.ID)
		mustTx(t, s, domain.TxExpense, "fev-2", 3000, "2026-02-15", domain.TxConfirmed, a1.ID, nil)
		mustTx(t, s, domain.TxExpense, "fev-3", 4000, "2026-02-28", domain.TxConfirmed, a2.ID, nil)
		mustTx(t, s, domain.TxExpense, "dez-25", 100, "2025-12-31", domain.TxConfirmed, a1.ID, nil)

		list := func(q domain.ListTransactionsQuery) ([]string, int) {
			t.Helper()
			if q.Page == 0 {
				q.Page = 1
			}
			if q.PageSize == 0 {
				q.PageSize = 50
			}
			items, total, err := s.ListTransactions(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			return descriptions(items), total
		}
		check := func(name string, q domain.ListTransactionsQuery, want string, wantTotal int) {
			t.Helper()
			got, total := list(q)
			if strings.Join(got, ",") != want || total != wantTotal {
				t.Errorf("%s: itens=%v total=%d, esperado %s total=%d", name, got, total, want, wantTotal)
			}
		}

		check("sem filtro (data DESC)", domain.ListTransactionsQuery{}, "fev-3,fev-2,fev-1,jan-2,jan-1,dez-25", 6)
		check("mês", domain.ListTransactionsQuery{Year: ptr(2026), Month: ptr(2)}, "fev-3,fev-2,fev-1", 3)
		check("ano", domain.ListTransactionsQuery{Year: ptr(2025)}, "dez-25", 1)
		check("conta", domain.ListTransactionsQuery{AccountID: &a2.ID}, "fev-3,fev-1", 2)
		check("categoria", domain.ListTransactionsQuery{CategoryID: &food.ID}, "fev-1,jan-1", 2)
		check("tipo", domain.ListTransactionsQuery{Type: ptr(domain.TxIncome)}, "jan-2", 1)
		check("status", domain.ListTransactionsQuery{Status: ptr(domain.TxPending)}, "fev-1", 1)
		check("combinado", domain.ListTransactionsQuery{Year: ptr(2026), Month: ptr(2), AccountID: &a2.ID, Status: ptr(domain.TxConfirmed)}, "fev-3", 1)
		check("página 2", domain.ListTransactionsQuery{Page: 2, PageSize: 2}, "fev-1,jan-2", 6)
		check("página além do fim", domain.ListTransactionsQuery{Page: 4, PageSize: 2}, "", 6)

		all, err := s.ListAllTransactions(ctx, domain.ListTransactionsQuery{Year: ptr(2026), Month: ptr(1)})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(descriptions(all), ","); got != "jan-1,jan-2" {
			t.Fatalf("ListAllTransactions (data ASC) = %s", got)
		}

		// Joins de nome de conta e categoria.
		items, _, err := s.ListTransactions(ctx, domain.ListTransactionsQuery{Page: 1, PageSize: 1, CategoryID: &food.ID, AccountID: &a1.ID})
		if err != nil || len(items) != 1 {
			t.Fatalf("item com joins: %v %v", items, err)
		}
		it := items[0]
		if it.AccountName == nil || *it.AccountName != "A1" || it.CategoryName == nil || *it.CategoryName != "Comida" || it.Date != "2026-01-10" || it.AmountCents != -1000 {
			t.Fatalf("item com joins = %+v", it)
		}
	})
}

func TestListTransactionsSearch(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		s := New(d)
		acc := mustAccount(t, s, "Conta", 0)
		for i, desc := range []string{"Doação ação social", "Cashback 100%", "1000 reais", "a_b", "axb", "Mercado"} {
			mustTx(t, s, domain.TxExpense, desc, 100, fmt.Sprintf("2026-04-%02d", i+1), domain.TxConfirmed, acc.ID, nil)
		}
		search := func(term string) string {
			t.Helper()
			items, total, err := s.ListTransactions(context.Background(), domain.ListTransactionsQuery{Page: 1, PageSize: 50, Search: &term})
			if err != nil {
				t.Fatal(err)
			}
			got := descriptions(items)
			if total != len(got) {
				t.Fatalf("total %d != itens %d", total, len(got))
			}
			sort.Strings(got)
			return strings.Join(got, "|")
		}

		cases := map[string]string{
			"AÇÃO":   "Doação ação social",
			"ação":   "Doação ação social",
			"CASH":   "Cashback 100%",
			"100%":   "Cashback 100%",
			"%":      "Cashback 100%",
			"a_b":    "a_b",
			"_":      "a_b",
			"mercad": "Mercado",
			`\`:      "",
		}
		for term, want := range cases {
			if got := search(term); got != want {
				t.Errorf("busca %q = %q, esperado %q", term, got, want)
			}
		}
	})
}

func TestTransferAtomicity(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		from := mustAccount(t, s, "Origem", 10000)
		to := mustAccount(t, s, "Destino", 0)

		groupID, err := s.CreateTransferPair(ctx, from.ID, to.ID, 2500, "2026-05-02", "para destino", "de origem")
		if err != nil {
			t.Fatal(err)
		}
		sums, err := s.ConfirmedSumsByAccount(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if sums[from.ID] != -2500 || sums[to.ID] != 2500 {
			t.Fatalf("somas após transferência = %v", sums)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE transfer_group_id = ? AND type = 'TRANSFER'`, groupID); n != 2 {
			t.Fatalf("pernas da transferência = %d", n)
		}

		// Destino inexistente: a segunda perna viola a FK e a primeira não pode ficar.
		if _, err := s.CreateTransferPair(ctx, from.ID, uuid.NewString(), 999, "2026-05-03", "x", "y"); err == nil {
			t.Fatal("transferência para conta inexistente deveria falhar")
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE account_id = ?`, from.ID); n != 1 {
			t.Fatalf("transferência parcial persistida: %d transações na origem", n)
		}

		if err := s.DeleteTransferGroup(ctx, groupID); err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions`); n != 0 {
			t.Fatalf("transações após excluir grupo = %d", n)
		}
	})
}

// ───────────────────────── Cards & Invoices ─────────────────────────

func TestCardExpenseInstallments(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		card := mustCard(t, s, 10)
		cat := mustCategory(t, s, "Eletrônicos", domain.CategoryExpense, nil)

		// Compra em 15/01 com fechamento dia 10 → faturas 02, 03 e 04/2026.
		n, err := s.CreateCardExpense(ctx, card.ID, cardLegs(t, card, "Notebook", 10000, "2026-01-15", 3, &cat.ID))
		if err != nil || n != 3 {
			t.Fatalf("CreateCardExpense = %d %v", n, err)
		}
		// Compra à vista em 05/02 → fatura 02/2026 (reaproveita a existente).
		if _, err := s.CreateCardExpense(ctx, card.ID, cardLegs(t, card, "Café", 500, "2026-02-05", 1, nil)); err != nil {
			t.Fatal(err)
		}

		invoices, err := s.FindInvoicesWithTotals(ctx, card.ID)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, inv := range invoices {
			got = append(got, fmt.Sprintf("%d-%02d:%d:%s", inv.Year, inv.Month, inv.TotalCents, inv.Status))
		}
		want := []string{"2026-04:3333:OPEN", "2026-03:3333:OPEN", "2026-02:3834:OPEN"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("faturas = %v, esperado %v", got, want)
		}

		feb := invoices[2]
		items, err := s.FindInvoiceItems(ctx, feb.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(descriptions(items), ","); got != "Notebook (1/3),Café" {
			t.Fatalf("itens da fatura 02 = %s", got)
		}
		first := items[0]
		if first.Type != domain.TxCardExpense || first.AmountCents != -3334 || first.CardName == nil || *first.CardName != "Cartão" ||
			first.InstallmentNumber == nil || *first.InstallmentNumber != 1 || first.InstallmentTotal == nil || *first.InstallmentTotal != 3 ||
			first.InvoiceID == nil || *first.InvoiceID != feb.ID {
			t.Fatalf("primeira parcela = %+v", first)
		}

		detail, err := s.FindTransactionByID(ctx, first.ID)
		if err != nil || detail == nil || detail.InstallmentGroupID == nil {
			t.Fatalf("detalhe da parcela = %+v %v", detail, err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE installment_group_id = ?`, *detail.InstallmentGroupID); n != 3 {
			t.Fatalf("parcelas no grupo = %d", n)
		}
		if unpaid, err := s.UnpaidCardTotalCents(ctx, card.ID); err != nil || unpaid != 10500 {
			t.Fatalf("UnpaidCardTotalCents = %d %v", unpaid, err)
		}
		if n, err := s.CountCardTransactions(ctx, card.ID); err != nil || n != 4 {
			t.Fatalf("CountCardTransactions = %d %v", n, err)
		}

		if err := s.DeleteInstallmentGroup(ctx, *detail.InstallmentGroupID); err != nil {
			t.Fatal(err)
		}
		if unpaid, err := s.UnpaidCardTotalCents(ctx, card.ID); err != nil || unpaid != 500 {
			t.Fatalf("UnpaidCardTotalCents após excluir grupo = %d %v", unpaid, err)
		}
	})
}

func TestConcurrentCardExpensesSameNewInvoice(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		card := mustCard(t, s, 10)

		const workers = 8
		var wg sync.WaitGroup
		errs := make(chan error, workers)
		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				legs := []CardExpenseLeg{{
					Description:   fmt.Sprintf("compra %d", i),
					AmountCents:   100,
					Date:          "2026-06-01",
					InvoicePeriod: domain.InvoicePeriod{Year: 2026, Month: 6},
				}}
				if _, err := s.CreateCardExpense(ctx, card.ID, legs); err != nil {
					errs <- err
				}
			}(i)
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Errorf("despesa concorrente falhou: %v", err)
		}

		invoices, err := s.FindInvoicesWithTotals(ctx, card.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(invoices) != 1 || invoices[0].TotalCents != workers*100 {
			t.Fatalf("faturas = %+v, esperado 1 fatura com %d", invoices, workers*100)
		}
	})
}

func TestPayInvoiceAndDeletePayment(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		acc := mustAccount(t, s, "Conta", 100000)
		card := mustCard(t, s, 10)
		if _, err := s.CreateCardExpense(ctx, card.ID, cardLegs(t, card, "Mercado", 25000, "2026-03-01", 1, nil)); err != nil {
			t.Fatal(err)
		}
		invoices, err := s.FindInvoicesWithTotals(ctx, card.ID)
		if err != nil || len(invoices) != 1 {
			t.Fatalf("faturas: %+v %v", invoices, err)
		}
		inv := invoices[0]

		if err := s.PayInvoice(ctx, inv.ID, acc.ID, "2026-03-15", "Pagamento fatura", inv.TotalCents); err != nil {
			t.Fatal(err)
		}
		paid, err := s.FindInvoiceByID(ctx, inv.ID)
		if err != nil || paid == nil {
			t.Fatalf("fatura paga: %v %v", paid, err)
		}
		if paid.Status != domain.InvoicePaid || paid.PaidAt == nil || *paid.PaidAt != "2026-03-15" || paid.PaymentTransactionID == nil {
			t.Fatalf("fatura após pagamento = %+v", paid)
		}

		// Segundo pagamento: conflito e nenhuma transação extra (rollback).
		err = s.PayInvoice(ctx, inv.ID, acc.ID, "2026-03-16", "Pagamento duplicado", inv.TotalCents)
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("segundo pagamento: err = %v, esperado ErrConflict", err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE type = 'INVOICE_PAYMENT'`); n != 1 {
			t.Fatalf("transações de pagamento = %d, esperado 1", n)
		}
		if err := s.PayInvoice(ctx, uuid.NewString(), acc.ID, "2026-03-16", "x", 1); !errors.Is(err, ErrConflict) {
			t.Fatalf("pagar fatura inexistente: err = %v, esperado ErrConflict", err)
		}
		sums, err := s.ConfirmedSumsByAccount(ctx)
		if err != nil || sums[acc.ID] != -25000 {
			t.Fatalf("saldo após pagamento = %v %v", sums, err)
		}
		if unpaid, err := s.UnpaidCardTotalCents(ctx, card.ID); err != nil || unpaid != 0 {
			t.Fatalf("UnpaidCardTotalCents após pagamento = %d %v", unpaid, err)
		}

		payID := *paid.PaymentTransactionID
		if got, err := s.FindInvoicePaidBy(ctx, payID); err != nil || got != inv.ID {
			t.Fatalf("FindInvoicePaidBy = %q %v", got, err)
		}
		if err := s.DeleteInvoicePayment(ctx, payID, inv.ID); err != nil {
			t.Fatal(err)
		}
		reopened, err := s.FindInvoiceByID(ctx, inv.ID)
		if err != nil || reopened == nil || reopened.Status != domain.InvoiceOpen || reopened.PaidAt != nil || reopened.PaymentTransactionID != nil {
			t.Fatalf("fatura após estornar pagamento = %+v %v", reopened, err)
		}
		if err := s.DeleteInvoicePayment(ctx, payID, inv.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("estorno repetido: err = %v, esperado ErrNotFound", err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM transactions WHERE type = 'INVOICE_PAYMENT'`); n != 0 {
			t.Fatalf("transações de pagamento após estorno = %d", n)
		}
	})
}

// ───────────────────────── Budgets & Goals ─────────────────────────

func TestBudgetsUpsert(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)
		acc := mustAccount(t, s, "Conta", 0)
		food := mustCategory(t, s, "Comida", domain.CategoryExpense, nil)

		first, err := s.UpsertBudget(ctx, food.ID, 50000)
		if err != nil {
			t.Fatal(err)
		}
		second, err := s.UpsertBudget(ctx, food.ID, 80000)
		if err != nil {
			t.Fatal(err)
		}
		if first.ID != second.ID || second.AmountCents != 80000 || second.CategoryName != "Comida" {
			t.Fatalf("upsert: primeiro=%+v segundo=%+v", first, second)
		}
		list, err := s.ListBudgets(ctx)
		if err != nil || len(list) != 1 {
			t.Fatalf("ListBudgets = %+v %v", list, err)
		}

		mustTx(t, s, domain.TxExpense, "almoço", 1200, "2026-07-03", domain.TxConfirmed, acc.ID, &food.ID)
		mustTx(t, s, domain.TxExpense, "jantar", 3400, "2026-07-20", domain.TxPending, acc.ID, &food.ID)
		mustTx(t, s, domain.TxExpense, "agosto", 9999, "2026-08-01", domain.TxConfirmed, acc.ID, &food.ID)

		spent, err := s.SpentByCategory(ctx, 2026, 7)
		if err != nil || spent[food.ID] != 4600 {
			t.Fatalf("SpentByCategory = %v %v", spent, err)
		}
		snap, err := s.BudgetSnapshot(ctx, food.ID, 2026, 7)
		if err != nil || snap == nil || snap.BudgetCents != 80000 || snap.SpentCents != 4600 {
			t.Fatalf("BudgetSnapshot = %+v %v", snap, err)
		}

		if err := s.DeleteBudget(ctx, second.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := s.FindBudgetByID(ctx, second.ID); err != nil || got != nil {
			t.Fatalf("orçamento excluído ainda encontrado: %+v %v", got, err)
		}
	})
}

func TestGoalsContributions(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)

		goal, err := s.CreateGoal(ctx, domain.CreateGoalInput{Name: "Viagem", Icon: "plane", TargetCents: 100000, Deadline: ptr("2027-01-31")})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateGoal(ctx, domain.CreateGoalInput{Name: "Vazia", Icon: "x", TargetCents: 1}); err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			cents int64
			date  string
		}{{30000, "2026-01-05"}, {12500, "2026-02-05"}} {
			if err := s.AddGoalContribution(ctx, goal.ID, c.cents, c.date, ptr("aporte")); err != nil {
				t.Fatal(err)
			}
		}

		got, err := s.FindGoalByID(ctx, goal.ID)
		if err != nil || got == nil {
			t.Fatalf("FindGoalByID: %v %v", got, err)
		}
		if got.SavedCents != 42500 || got.Deadline == nil || *got.Deadline != "2027-01-31" {
			t.Fatalf("meta = %+v", got)
		}
		list, err := s.ListGoals(ctx)
		if err != nil || len(list) != 2 {
			t.Fatalf("ListGoals = %+v %v", list, err)
		}
		saved := map[string]int64{}
		for _, g := range list {
			saved[g.Name] = g.SavedCents
		}
		if saved["Viagem"] != 42500 || saved["Vazia"] != 0 {
			t.Fatalf("saldos das metas = %v", saved)
		}

		got.Deadline = nil
		got.TargetCents = 200000
		if err := s.UpdateGoal(ctx, got); err != nil {
			t.Fatal(err)
		}
		if after, err := s.FindGoalByID(ctx, goal.ID); err != nil || after.Deadline != nil || after.TargetCents != 200000 {
			t.Fatalf("meta atualizada = %+v %v", after, err)
		}
		if err := s.DeleteGoal(ctx, goal.ID); err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, s, `SELECT COUNT(*) FROM goal_contributions`); n != 0 {
			t.Fatalf("aportes órfãos = %d", n)
		}
	})
}

// ───────────────────────── Seed ─────────────────────────

func TestSeedIdempotent(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		s := New(d)

		ran, err := Seed(ctx, s)
		if err != nil || !ran {
			t.Fatalf("primeiro seed: ran=%v err=%v", ran, err)
		}
		counts := func() string {
			var parts []string
			for _, tbl := range []string{"accounts", "categories", "credit_cards", "invoices", "transactions", "budgets", "goals", "goal_contributions", "recurring", "category_rules", "wishlist", "settings"} {
				parts = append(parts, fmt.Sprintf("%s=%d", tbl, countRows(t, s, `SELECT COUNT(*) FROM `+tbl)))
			}
			return strings.Join(parts, " ")
		}
		before := counts()
		ran, err = Seed(ctx, s)
		if err != nil || ran {
			t.Fatalf("segundo seed deveria ser no-op: ran=%v err=%v", ran, err)
		}
		if after := counts(); after != before {
			t.Fatalf("seed não idempotente: antes %s, depois %s", before, after)
		}
		if n, _ := s.CountAccounts(ctx); n != 5 {
			t.Fatalf("contas do seed = %d", n)
		}

		if err := s.ClearAll(ctx); err != nil {
			t.Fatal(err)
		}
		if n, _ := s.CountAccounts(ctx); n != 0 {
			t.Fatalf("contas após ClearAll = %d", n)
		}
	})
}
