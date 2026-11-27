package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/spreadsheet"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

type importFixture struct {
	svcs              *Services
	checking, savings domain.Account
	fixedCat, cafeCat domain.Category
	incomeCat         domain.Category
}

func newImportFixture(t *testing.T, database *db.DB) importFixture {
	t.Helper()
	ctx := context.Background()
	svcs := New(store.New(database), nil)
	fx := importFixture{svcs: svcs}
	var err error
	if fx.checking, err = svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Corrente Import", Type: domain.AccountChecking, InitialBalanceCents: 100000}); err != nil {
		t.Fatal(err)
	}
	if fx.savings, err = svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Reserva Import", Type: domain.AccountSavings}); err != nil {
		t.Fatal(err)
	}
	fixed := domain.SpendFixed
	if fx.fixedCat, err = svcs.CreateCategory(ctx, domain.CreateCategoryInput{Name: "Aluguel Teste XYZ", Kind: domain.CategoryExpense, Icon: "home", Color: "#000000", SpendType: &fixed}); err != nil {
		t.Fatal(err)
	}
	if fx.cafeCat, err = svcs.CreateCategory(ctx, domain.CreateCategoryInput{Name: "Café Especial XYZ", Kind: domain.CategoryExpense, Icon: "coffee", Color: "#000000"}); err != nil {
		t.Fatal(err)
	}
	if fx.incomeCat, err = svcs.CreateCategory(ctx, domain.CreateCategoryInput{Name: "Salário Teste XYZ", Kind: domain.CategoryIncome, Icon: "wallet", Color: "#000000"}); err != nil {
		t.Fatal(err)
	}
	return fx
}

func mustCreateTx(t *testing.T, svcs *Services, in domain.CreateTransactionInput) {
	t.Helper()
	if _, err := svcs.CreateTransaction(context.Background(), in); err != nil {
		t.Fatal(err)
	}
}

func TestExportYearWorkbookService(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		fx := newImportFixture(t, database)
		svcs := fx.svcs
		// Dezembro anterior compõe o saldo de abertura (1.000 + 100 = 1.100).
		mustCreateTx(t, svcs, domain.CreateTransactionInput{Type: domain.TxIncome, Description: "Extra", AmountCents: 10000, Date: "2025-12-15", AccountID: fx.checking.ID})
		mustCreateTx(t, svcs, domain.CreateTransactionInput{Type: domain.TxIncome, Description: "Salário", AmountCents: 500000, Date: "2026-01-05", AccountID: fx.checking.ID})
		mustCreateTx(t, svcs, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "Aluguel", AmountCents: 180000, Date: "2026-01-10", AccountID: fx.checking.ID, CategoryID: &fx.fixedCat.ID})
		mustCreateTx(t, svcs, domain.CreateTransactionInput{Type: domain.TxExpense, Description: "Pendente", AmountCents: 999, Date: "2026-01-11", AccountID: fx.checking.ID, Status: domain.TxPending})
		card, err := svcs.CreateCard(ctx, domain.CreateCardInput{Name: "Visa Import", Brand: domain.BrandVisa, LimitCents: 1000000, ClosingDay: 10, DueDay: 20})
		if err != nil {
			t.Fatal(err)
		}
		// Compra em 15/01 com fechamento dia 10 → fatura de fevereiro, vence 20/02.
		if _, err := svcs.AddCardExpense(ctx, card.ID, domain.CreateCardExpenseInput{Description: "Tênis", TotalCents: 30000, Date: "2026-01-15", Installments: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.Transfer(ctx, domain.TransferInput{FromAccountID: fx.checking.ID, ToAccountID: fx.savings.ID, AmountCents: 50000, Date: "2026-02-01"}); err != nil {
			t.Fatal(err)
		}
		goal, err := svcs.CreateGoal(ctx, domain.CreateGoalInput{Name: "Viagem", Icon: "plane", TargetCents: 100000})
		if err != nil {
			t.Fatal(err)
		}
		march := "2026-03-03"
		if _, err := svcs.ContributeGoal(ctx, goal.ID, domain.ContributeGoalInput{AmountCents: 20000, Date: &march}); err != nil {
			t.Fatal(err)
		}

		year := 2026
		data, filename, err := svcs.ExportYearWorkbook(ctx, &year)
		if err != nil {
			t.Fatal(err)
		}
		if filename != "finflow-planilha-2026.xlsx" {
			t.Errorf("filename = %q", filename)
		}
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		get := func(sheet, ref string) string {
			v, _ := f.GetCellValue(sheet, ref, excelize.Options{RawCellValue: true})
			return v
		}
		checks := []struct{ sheet, ref, want string }{
			{"2026", "B3", "1100"},  // saldo de abertura em 1º/jan
			{"2026", "B7", "5000"},  // salário 05/jan
			{"2026", "C12", "1800"}, // aluguel FIXED → Saída
			{"2026", "D13", ""},     // pendente não entra
			{"2026", "J22", "300"},  // cartão no vencimento (20/fev) → Diário
			{"2026", "I3", "500"},   // transferência para poupança → Saída
			{"Economia", "D6", "500"},
			{"Economia", "D7", "0"}, // aporte em meta não conta: já existe conta de reserva
		}
		for _, c := range checks {
			if got := get(c.sheet, c.ref); got != c.want {
				t.Errorf("%s!%s = %q, want %q", c.sheet, c.ref, got, c.want)
			}
		}

		// Ano fora do intervalo.
		bad := 1990
		var appErr *AppError
		if _, _, err := svcs.ExportYearWorkbook(ctx, &bad); !errors.As(err, &appErr) || appErr.Status != http.StatusBadRequest {
			t.Errorf("ano inválido: %v", err)
		}

		// Extrato em xlsx com os filtros do extrato.
		content, contentType, name, err := svcs.ExportTransactions(ctx, "xlsx", domain.ListTransactionsQuery{Year: &year})
		if err != nil {
			t.Fatal(err)
		}
		if contentType != XLSXContentType || !strings.HasSuffix(name, ".xlsx") {
			t.Errorf("content-type/nome: %q %q", contentType, name)
		}
		tf, err := excelize.OpenReader(strings.NewReader(content))
		if err != nil {
			t.Fatal(err)
		}
		defer tf.Close()
		rows, _ := tf.GetRows("Lançamentos")
		if len(rows) < 6 {
			t.Errorf("extrato xlsx com %d linhas", len(rows))
		}
	})
}

func TestImportPreviewAndCommitService(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		fx := newImportFixture(t, database)
		svcs := fx.svcs
		mustCreateTx(t, svcs, domain.CreateTransactionInput{Type: domain.TxIncome, Description: "Salário", AmountCents: 500000, Date: "2026-03-05", AccountID: fx.checking.ID})

		csv := "Data;Descrição;Valor;Categoria\n" +
			"05/03/2026;SALÁRIO;5.000,00;\n" + // já existe na conta
			"06/03/2026;Cafeteria;-12,50;cafe especial xyz\n" +
			"06/03/2026;Cafeteria;-12,50;cafe especial xyz\n" + // repetido no arquivo
			"07/03/2026;Freela;800,00;Salário Teste XYZ\n" +
			"07/03/2026;Aluguel;-1.800,00;Aluguel Teste XYZ\n" +
			"xx;Quebrado;-1,00;\n"
		accID := fx.checking.ID
		res, err := svcs.ImportPreview(ctx, ImportPreviewInput{Data: []byte(csv), Filename: "banco.csv", AccountID: &accID})
		if err != nil {
			t.Fatal(err)
		}
		if res.Format != spreadsheet.FormatCSV || len(res.Rows) != 6 {
			t.Fatalf("res = %+v", res)
		}
		r := res.Rows
		if !r[0].Duplicate || r[1].Duplicate || !r[2].Duplicate || r[3].Duplicate {
			t.Errorf("duplicados: %v %v %v %v", r[0].Duplicate, r[1].Duplicate, r[2].Duplicate, r[3].Duplicate)
		}
		if r[1].SuggestedCategoryID == nil || *r[1].SuggestedCategoryID != fx.cafeCat.ID {
			t.Errorf("categoria sem acento não casou: %v", r[1].SuggestedCategoryID)
		}
		if r[3].SuggestedCategoryID == nil || *r[3].SuggestedCategoryID != fx.incomeCat.ID {
			t.Errorf("categoria de receita não casou: %v", r[3].SuggestedCategoryID)
		}
		if r[5].Error == nil {
			t.Errorf("linha quebrada sem erro")
		}

		// Conta inexistente no preview.
		missing := "00000000-0000-4000-8000-000000000000"
		var appErr *AppError
		if _, err := svcs.ImportPreview(ctx, ImportPreviewInput{Data: []byte(csv), AccountID: &missing}); !errors.As(err, &appErr) || appErr.Status != http.StatusNotFound {
			t.Errorf("conta inexistente: %v", err)
		}
		if _, err := svcs.ImportPreview(ctx, ImportPreviewInput{Data: []byte{0xD0, 0xCF, 0x11, 0xE0}}); !errors.As(err, &appErr) || appErr.Status != http.StatusBadRequest {
			t.Errorf("xls deveria dar 400: %v", err)
		}

		// Commit: categoria de tipo errado é rejeitada sem gravar nada.
		wrong := ImportCommitInput{AccountID: fx.checking.ID, Rows: []ImportCommitRow{
			{Date: "2026-03-07", Description: "Freela", AmountCents: 80000, Kind: domain.CategoryIncome,
				CategoryID: domain.Optional[string]{Set: true, Valid: true, Value: fx.fixedCat.ID}},
		}}
		if _, err := svcs.ImportCommit(ctx, wrong); !errors.As(err, &appErr) || appErr.Status != http.StatusBadRequest ||
			!strings.Contains(err.Error(), "rows[0].categoryId") {
			t.Fatalf("categoria de tipo errado: %v", err)
		}

		pending := domain.TxPending
		in := ImportCommitInput{AccountID: fx.checking.ID, Rows: []ImportCommitRow{
			{Date: "2026-03-05", Description: "salario", AmountCents: 500000, Kind: domain.CategoryIncome}, // duplicado
			{Date: "2026-03-06", Description: "Cafeteria", AmountCents: 1250, Kind: domain.CategoryExpense,
				CategoryID: domain.Optional[string]{Set: true, Valid: true, Value: fx.cafeCat.ID}},
			{Date: "2026-03-07", Description: "Aluguel", AmountCents: 180000, Kind: domain.CategoryExpense,
				CategoryID: domain.Optional[string]{Set: true}, Status: &pending},
		}}
		out, err := svcs.ImportCommit(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if out.Created != 2 || out.Skipped != 1 {
			t.Fatalf("commit = %+v", out)
		}
		month, year := 3, 2026
		list, err := svcs.ListTransactions(ctx, domain.ListTransactionsQuery{AccountID: &fx.checking.ID, Month: &month, Year: &year})
		if err != nil {
			t.Fatal(err)
		}
		var cafe, aluguel *domain.Transaction
		for i := range list.Items {
			switch list.Items[i].Description {
			case "Cafeteria":
				cafe = &list.Items[i]
			case "Aluguel":
				aluguel = &list.Items[i]
			}
		}
		if cafe == nil || cafe.AmountCents != -1250 || cafe.Type != domain.TxExpense || cafe.CategoryID == nil || *cafe.CategoryID != fx.cafeCat.ID {
			t.Errorf("cafeteria = %+v", cafe)
		}
		if aluguel == nil || aluguel.Status != domain.TxPending || aluguel.CategoryID != nil {
			t.Errorf("aluguel = %+v", aluguel)
		}

		// Sem pular duplicados, o salário repetido é gravado.
		no := false
		out, err = svcs.ImportCommit(ctx, ImportCommitInput{AccountID: fx.checking.ID, SkipDuplicates: &no, Rows: in.Rows[:1]})
		if err != nil || out.Created != 1 || out.Skipped != 0 {
			t.Fatalf("skipDuplicates=false: %+v %v", out, err)
		}

		// Validação por linha.
		_, err = svcs.ImportCommit(ctx, ImportCommitInput{AccountID: fx.checking.ID, Rows: []ImportCommitRow{
			{Date: "2026-13-01", Description: "", AmountCents: 0, Kind: "X"},
		}})
		if !errors.As(err, &appErr) || !strings.Contains(err.Error(), "rows[0].date") || !strings.Contains(err.Error(), "rows[0].kind") ||
			!strings.Contains(err.Error(), "rows[0].amountCents") || !strings.Contains(err.Error(), "rows[0].description") {
			t.Fatalf("validação por linha: %v", err)
		}
		if _, err := svcs.ImportCommit(ctx, ImportCommitInput{AccountID: "x", Rows: nil}); !errors.As(err, &appErr) || appErr.Status != http.StatusBadRequest {
			t.Fatalf("entrada vazia: %v", err)
		}
	})
}

func TestImportCommitRowMarshalJSON(t *testing.T) {
	cases := []struct {
		row  ImportCommitRow
		want string
	}{
		{ImportCommitRow{Date: "2026-01-01", Description: "a", AmountCents: 1, Kind: domain.CategoryIncome}, `{"amountCents":1,"date":"2026-01-01","description":"a","kind":"INCOME"}`},
		{ImportCommitRow{Date: "2026-01-01", Description: "a", AmountCents: 1, Kind: domain.CategoryIncome, CategoryID: domain.Optional[string]{Set: true}}, `{"amountCents":1,"categoryId":null,"date":"2026-01-01","description":"a","kind":"INCOME"}`},
	}
	for _, tc := range cases {
		got, err := tc.row.MarshalJSON()
		if err != nil || string(got) != tc.want {
			t.Errorf("MarshalJSON = %s (%v), want %s", got, err, tc.want)
		}
	}
}
