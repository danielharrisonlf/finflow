package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"finflow/internal/domain"
	"finflow/internal/spreadsheet"
	"finflow/internal/store"
)

// XLSXContentType é o MIME das pastas de trabalho geradas.
const XLSXContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// MaxImportRows limita as linhas de um commit de importação.
const MaxImportRows = 20000

// ───────────────────────── Exportação ─────────────────────────

// ExportYearWorkbook gera a planilha anual (abas <ano>, Economia, Lançamentos e Categorias).
func (s *Services) ExportYearWorkbook(ctx context.Context, year *int) (content []byte, filename string, err error) {
	y := domain.Now().Year()
	if year != nil {
		y = *year
	}
	if y < 2000 || y > 2100 {
		return nil, "", ValidationFailed([]domain.ValidationIssue{{Path: "year", Message: "ano deve estar entre 2000 e 2100"}})
	}
	yearStart, yearEnd := domain.YearRange(y)
	// Compras no cartão de até 3 meses antes podem vencer em janeiro.
	rows, err := s.Store.ListSpreadsheetTransactions(ctx, domain.AddMonthsToDateString(yearStart, -3), yearEnd)
	if err != nil {
		return nil, "", err
	}
	txs := make([]spreadsheet.Transaction, 0, len(rows))
	for _, r := range rows {
		t := toSpreadsheetTx(r)
		inYear := t.Date >= yearStart && t.Date < yearEnd
		cashInYear := t.CashDate >= yearStart && t.CashDate < yearEnd
		if inYear || cashInYear {
			txs = append(txs, t)
		}
	}
	opening, err := s.Store.BalanceBeforeCents(ctx, yearStart)
	if err != nil {
		return nil, "", err
	}
	// Mesma regra dos insights: aportes em metas só contam como Economia sem contas de reserva.
	var goals [12]int64
	hasReserve, err := s.Store.HasReserveAccounts(ctx)
	if err != nil {
		return nil, "", err
	}
	if !hasReserve {
		if goals, err = s.Store.GoalContributionsByMonth(ctx, y); err != nil {
			return nil, "", err
		}
	}
	bands, err := s.spreadsheetBands(ctx)
	if err != nil {
		return nil, "", err
	}
	content, err = spreadsheet.BuildYearWorkbook(spreadsheet.YearInput{
		Year:                     y,
		OpeningBalanceCents:      opening,
		Transactions:             txs,
		GoalContributionsByMonth: goals,
		Bands:                    bands,
	})
	if err != nil {
		return nil, "", err
	}
	return content, fmt.Sprintf("finflow-planilha-%d.xlsx", y), nil
}

func toSpreadsheetTx(r store.SpreadsheetTxRow) spreadsheet.Transaction {
	t := spreadsheet.Transaction{
		Date:        r.Date,
		CashDate:    r.Date,
		Description: r.Description,
		Type:        r.Type,
		Status:      r.Status,
		AmountCents: r.AmountCents,
	}
	if r.CategoryName != nil {
		t.CategoryName = *r.CategoryName
	}
	if r.SpendType != nil {
		t.SpendType = spreadsheet.SpendType(*r.SpendType)
	}
	if r.AccountName != nil {
		t.AccountName = *r.AccountName
	} else if r.CardName != nil {
		t.AccountName = *r.CardName
	}
	if r.AccountType != nil {
		t.AccountType = *r.AccountType
	}
	if r.CounterpartAccountType != nil {
		t.CounterpartAccountType = *r.CounterpartAccountType
	}
	if r.InstallmentNumber != nil && r.InstallmentTotal != nil {
		t.InstallmentNumber, t.InstallmentTotal = *r.InstallmentNumber, *r.InstallmentTotal
	}
	// Regime de caixa: compra no cartão entra no vencimento da fatura.
	if r.Type == domain.TxCardExpense && r.InvoiceYear != nil && r.InvoiceMonth != nil && r.CardClosingDay != nil && r.CardDueDay != nil {
		t.CashDate = domain.InvoiceDueDate(domain.InvoicePeriod{Year: *r.InvoiceYear, Month: *r.InvoiceMonth}, *r.CardClosingDay, *r.CardDueDay)
	}
	return t
}

// spreadsheetBands lê as faixas de cor do saldo das configurações (settings.balanceBands).
func (s *Services) spreadsheetBands(ctx context.Context) (spreadsheet.Bands, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return spreadsheet.Bands{}, err
	}
	b := settings.BalanceBands
	return spreadsheet.Bands{DarkGreen: b.DarkGreen, LightGreen: b.LightGreen, Yellow: b.Yellow, LightRed: b.LightRed}, nil
}

func domainToSpreadsheetTx(t domain.Transaction) spreadsheet.Transaction {
	out := spreadsheet.Transaction{
		Date:        t.Date,
		Description: t.Description,
		Type:        t.Type,
		Status:      t.Status,
		AmountCents: t.AmountCents,
	}
	if t.CategoryName != nil {
		out.CategoryName = *t.CategoryName
	}
	if t.AccountName != nil {
		out.AccountName = *t.AccountName
	} else if t.CardName != nil {
		out.AccountName = *t.CardName
	}
	if t.InstallmentNumber != nil && t.InstallmentTotal != nil {
		out.InstallmentNumber, out.InstallmentTotal = *t.InstallmentNumber, *t.InstallmentTotal
	}
	return out
}

// exportTransactionsXLSX gera o extrato filtrado numa aba "Lançamentos".
func exportTransactionsXLSX(items []domain.Transaction) ([]byte, error) {
	txs := make([]spreadsheet.Transaction, 0, len(items))
	for _, t := range items {
		txs = append(txs, domainToSpreadsheetTx(t))
	}
	return spreadsheet.BuildTransactionsWorkbook(txs)
}

// ImportTemplate gera o modelo de importação (aba Lançamentos com exemplos + Como usar).
func (s *Services) ImportTemplate() (content []byte, filename string, err error) {
	content, err = spreadsheet.BuildImportTemplate(domain.Now())
	if err != nil {
		return nil, "", err
	}
	return content, "finflow-modelo-importacao.xlsx", nil
}

// ───────────────────────── Importação ─────────────────────────

// ImportPreviewInput é o arquivo enviado e as opções da prévia.
type ImportPreviewInput struct {
	Data      []byte
	Filename  string
	Sheet     string
	Mapping   *spreadsheet.Mapping
	AccountID *string
}

// ImportPreview lê o arquivo, marca duplicados (no arquivo e, com accountId, na conta) e sugere categorias.
func (s *Services) ImportPreview(ctx context.Context, in ImportPreviewInput) (*spreadsheet.Result, error) {
	if len(in.Data) == 0 {
		return nil, BadRequest("envie um arquivo (campo file)")
	}
	if in.AccountID != nil {
		if !domain.IsValidUUID(*in.AccountID) {
			return nil, ValidationFailed([]domain.ValidationIssue{{Path: "accountId", Message: "accountId deve ser UUID válido"}})
		}
		acc, err := s.Store.FindAccountByID(ctx, *in.AccountID)
		if err != nil {
			return nil, err
		}
		if acc == nil {
			return nil, NotFound("conta não encontrada")
		}
	}
	res, err := spreadsheet.Parse(in.Data, spreadsheet.Options{Filename: in.Filename, Sheet: in.Sheet, Mapping: in.Mapping})
	if err != nil {
		return nil, BadRequest(err.Error())
	}

	if in.AccountID != nil {
		existing, err := s.accountDuplicateKeys(ctx, *in.AccountID, previewRange(res.Rows))
		if err != nil {
			return nil, err
		}
		for i := range res.Rows {
			r := &res.Rows[i]
			if r.Error == nil && existing[spreadsheet.DuplicateKey(r.Date, r.AmountCents, r.Kind, r.Description)] {
				r.Duplicate = true
			}
		}
	}

	matcher, err := s.newCategoryMatcher(ctx)
	if err != nil {
		return nil, err
	}
	for i := range res.Rows {
		r := &res.Rows[i]
		if r.Error != nil {
			continue
		}
		if r.CategoryName != nil {
			if id := matcher.byName(*r.CategoryName, r.Kind); id != nil {
				r.SuggestedCategoryID = id
				continue
			}
		}
		id, err := s.SuggestCategory(ctx, r.Description, r.Kind)
		if err != nil {
			return nil, err
		}
		r.SuggestedCategoryID = id
	}
	return res, nil
}

type dateRange struct{ start, end string }

func previewRange(rows []spreadsheet.Row) *dateRange {
	var out *dateRange
	for _, r := range rows {
		if r.Error != nil || r.Date == "" {
			continue
		}
		if out == nil {
			out = &dateRange{r.Date, r.Date}
			continue
		}
		out.start = min(out.start, r.Date)
		out.end = max(out.end, r.Date)
	}
	return out
}

// accountDuplicateKeys devolve as chaves de duplicado dos lançamentos da conta no intervalo.
func (s *Services) accountDuplicateKeys(ctx context.Context, accountID string, rng *dateRange) (map[string]bool, error) {
	keys := map[string]bool{}
	if rng == nil {
		return keys, nil
	}
	existing, err := s.Store.ListAccountFingerprints(ctx, accountID, rng.start, rng.end)
	if err != nil {
		return nil, err
	}
	for _, e := range existing {
		kind := domain.CategoryExpense
		if e.AmountCents > 0 {
			kind = domain.CategoryIncome
		}
		keys[spreadsheet.DuplicateKey(e.Date, e.AmountCents, kind, e.Description)] = true
	}
	return keys, nil
}

// categoryMatcher casa nomes de categoria sem diferenciar maiúsculas nem acentos.
type categoryMatcher struct {
	ids   map[string]string // kind|nome normalizado → id
	valid map[string]domain.CategoryKind
}

func (s *Services) newCategoryMatcher(ctx context.Context) (*categoryMatcher, error) {
	cats, err := s.Store.ListCategories(ctx, nil)
	if err != nil {
		return nil, err
	}
	m := &categoryMatcher{ids: map[string]string{}, valid: map[string]domain.CategoryKind{}}
	for _, c := range cats {
		key := string(c.Kind) + "|" + spreadsheet.Normalize(c.Name)
		if _, dup := m.ids[key]; !dup {
			m.ids[key] = c.ID
		}
		m.valid[c.ID] = c.Kind
	}
	return m, nil
}

func (m *categoryMatcher) byName(name string, kind domain.CategoryKind) *string {
	if id, ok := m.ids[string(kind)+"|"+spreadsheet.Normalize(name)]; ok {
		return &id
	}
	return nil
}

// ImportCommitRow é uma linha confirmada pelo usuário. CategoryID ausente aplica as regras de automação;
// null explícito grava sem categoria.
type ImportCommitRow struct {
	Date        string                    `json:"date"`
	Description string                    `json:"description"`
	AmountCents int64                     `json:"amountCents"`
	Kind        domain.CategoryKind       `json:"kind"`
	CategoryID  domain.Optional[string]   `json:"categoryId"`
	Status      *domain.TransactionStatus `json:"status,omitempty"`
}

// MarshalJSON omite categoryId quando ausente e o envia como null quando limpo (usado pelo cliente HTTP).
func (r ImportCommitRow) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"date":        r.Date,
		"description": r.Description,
		"amountCents": r.AmountCents,
		"kind":        r.Kind,
	}
	if r.CategoryID.Set {
		if r.CategoryID.Valid {
			out["categoryId"] = r.CategoryID.Value
		} else {
			out["categoryId"] = nil
		}
	}
	if r.Status != nil {
		out["status"] = *r.Status
	}
	return json.Marshal(out)
}

// ImportCommitInput é o corpo de POST /api/import/commit.
type ImportCommitInput struct {
	AccountID      string            `json:"accountId"`
	SkipDuplicates *bool             `json:"skipDuplicates,omitempty"`
	Rows           []ImportCommitRow `json:"rows"`
}

// ImportCommitResult resume a importação.
type ImportCommitResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}

// ImportCommit grava as linhas na conta escolhida numa única transação de banco, pulando duplicados.
func (s *Services) ImportCommit(ctx context.Context, in ImportCommitInput) (ImportCommitResult, error) {
	var issues []domain.ValidationIssue
	if !domain.IsValidUUID(in.AccountID) {
		issues = append(issues, domain.ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	if len(in.Rows) == 0 {
		issues = append(issues, domain.ValidationIssue{Path: "rows", Message: "envie ao menos uma linha"})
	}
	if len(in.Rows) > MaxImportRows {
		issues = append(issues, domain.ValidationIssue{Path: "rows", Message: fmt.Sprintf("no máximo %d linhas por importação", MaxImportRows)})
	}
	if len(issues) > 0 {
		return ImportCommitResult{}, ValidationFailed(issues)
	}
	matcher, err := s.newCategoryMatcher(ctx)
	if err != nil {
		return ImportCommitResult{}, err
	}
	for i, r := range in.Rows {
		prefix := "rows[" + strconv.Itoa(i) + "]."
		status := domain.TxConfirmed
		if r.Status != nil {
			status = *r.Status
		}
		var catID *string
		if r.CategoryID.Set && r.CategoryID.Valid {
			v := r.CategoryID.Value
			catID = &v
		}
		txType := domain.TxExpense
		if r.Kind == domain.CategoryIncome {
			txType = domain.TxIncome
		} else if r.Kind != domain.CategoryExpense {
			issues = append(issues, domain.ValidationIssue{Path: prefix + "kind", Message: "kind deve ser INCOME ou EXPENSE"})
		}
		for _, is := range domain.ValidateCreateTransaction(domain.CreateTransactionInput{
			Type: txType, Description: strings.TrimSpace(r.Description), AmountCents: r.AmountCents, Date: r.Date,
			Status: status, AccountID: in.AccountID, CategoryID: catID,
		}) {
			if is.Path == "accountId" || is.Path == "type" {
				continue
			}
			issues = append(issues, domain.ValidationIssue{Path: prefix + is.Path, Message: is.Message})
		}
		if r.AmountCents > domain.MaxParsedCents {
			issues = append(issues, domain.ValidationIssue{Path: prefix + "amountCents", Message: "valor muito alto"})
		}
		if catID != nil && domain.IsValidUUID(*catID) {
			kind, ok := matcher.valid[*catID]
			if !ok {
				issues = append(issues, domain.ValidationIssue{Path: prefix + "categoryId", Message: "categoria não encontrada"})
			} else if kind != r.Kind {
				issues = append(issues, domain.ValidationIssue{Path: prefix + "categoryId", Message: "categoria de tipo diferente do lançamento"})
			}
		}
		if len(issues) >= 50 {
			break
		}
	}
	if len(issues) > 0 {
		return ImportCommitResult{}, ValidationFailed(issues)
	}
	if err := s.requireActiveAccount(ctx, in.AccountID); err != nil {
		return ImportCommitResult{}, err
	}

	skip := in.SkipDuplicates == nil || *in.SkipDuplicates
	existing := map[string]bool{}
	if skip {
		var rng *dateRange
		for _, r := range in.Rows {
			if rng == nil {
				rng = &dateRange{r.Date, r.Date}
				continue
			}
			rng.start = min(rng.start, r.Date)
			rng.end = max(rng.end, r.Date)
		}
		existing, err = s.accountDuplicateKeys(ctx, in.AccountID, rng)
		if err != nil {
			return ImportCommitResult{}, err
		}
	}

	items := make([]store.ImportTxRow, 0, len(in.Rows))
	skipped := 0
	for _, r := range in.Rows {
		desc := strings.TrimSpace(r.Description)
		if skip && existing[spreadsheet.DuplicateKey(r.Date, r.AmountCents, r.Kind, desc)] {
			skipped++
			continue
		}
		txType := domain.TxExpense
		if r.Kind == domain.CategoryIncome {
			txType = domain.TxIncome
		}
		status := domain.TxConfirmed
		if r.Status != nil {
			status = *r.Status
		}
		var catID *string
		switch {
		case r.CategoryID.Set && r.CategoryID.Valid:
			v := r.CategoryID.Value
			catID = &v
		case !r.CategoryID.Set:
			catID, err = s.SuggestCategory(ctx, desc, r.Kind)
			if err != nil {
				return ImportCommitResult{}, err
			}
		}
		items = append(items, store.ImportTxRow{
			Type:        txType,
			Description: desc,
			AmountCents: domain.SignedAmount(txType, r.AmountCents),
			Date:        r.Date,
			Status:      status,
			CategoryID:  catID,
		})
	}
	created, err := s.Store.CreateImportedTransactions(ctx, in.AccountID, items)
	if err != nil {
		return ImportCommitResult{}, err
	}
	return ImportCommitResult{Created: created, Skipped: skipped}, nil
}
