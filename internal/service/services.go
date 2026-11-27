package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	"finflow/internal/domain"
	"finflow/internal/store"
)

type Services struct {
	Store         *store.Store
	Auth          *AuthService
	TelegramAPI   *TelegramAPIClient
	Notifications *NotificationsService
	Telegram      *TelegramService
	// LinkPreview busca a prévia de links de desejos (SSRF-safe; substituível nos testes).
	LinkPreview *LinkPreviewer
}

func New(st *store.Store, httpClient *http.Client) *Services {
	tgAPI := NewTelegramAPIClient(httpClient)
	notif := NewNotificationsService(st, tgAPI)
	svcs := &Services{
		Store:         st,
		Auth:          NewAuthService(),
		TelegramAPI:   tgAPI,
		Notifications: notif,
		LinkPreview:   NewLinkPreviewer(),
	}
	svcs.Telegram = NewTelegramService(svcs)
	return svcs
}

// ───────────────────────── Accounts ─────────────────────────

func toAccount(r store.AccountRow, confirmedSum int64) domain.Account {
	return domain.Account{
		ID:                  r.ID,
		Name:                r.Name,
		Type:                r.Type,
		InitialBalanceCents: r.InitialBalanceCents,
		BalanceCents:        r.InitialBalanceCents + confirmedSum,
		Color:               r.Color,
		Archived:            r.Archived,
	}
}

func (s *Services) ListAccounts(ctx context.Context, includeArchived bool) ([]domain.Account, error) {
	rows, err := s.Store.ListAccounts(ctx, includeArchived)
	if err != nil {
		return nil, err
	}
	sums, err := s.Store.ConfirmedSumsByAccount(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAccount(r, sums[r.ID]))
	}
	return out, nil
}

func (s *Services) GetAccount(ctx context.Context, id string) (domain.Account, error) {
	r, err := s.Store.FindAccountByID(ctx, id)
	if err != nil {
		return domain.Account{}, err
	}
	if r == nil {
		return domain.Account{}, NotFound("conta não encontrada")
	}
	sums, err := s.Store.ConfirmedSumsByAccount(ctx)
	if err != nil {
		return domain.Account{}, err
	}
	return toAccount(*r, sums[r.ID]), nil
}

func (s *Services) CreateAccount(ctx context.Context, in domain.CreateAccountInput) (domain.Account, error) {
	if issues := domain.ValidateCreateAccount(in); len(issues) > 0 {
		return domain.Account{}, ValidationFailed(issues)
	}
	created, err := s.Store.CreateAccount(ctx, in)
	if err != nil {
		return domain.Account{}, err
	}
	return toAccount(*created, 0), nil
}

func (s *Services) UpdateAccount(ctx context.Context, id string, in domain.UpdateAccountInput) (domain.Account, error) {
	if issues := domain.ValidateUpdateAccount(in); len(issues) > 0 {
		return domain.Account{}, ValidationFailed(issues)
	}
	r, err := s.Store.FindAccountByID(ctx, id)
	if err != nil {
		return domain.Account{}, err
	}
	if r == nil {
		return domain.Account{}, NotFound("conta não encontrada")
	}
	if in.Name.Set && in.Name.Valid {
		r.Name = in.Name.Value
	}
	if in.Type.Set && in.Type.Valid {
		r.Type = in.Type.Value
	}
	if in.InitialBalanceCents.Set && in.InitialBalanceCents.Valid {
		r.InitialBalanceCents = in.InitialBalanceCents.Value
	}
	if in.Color.Set {
		if in.Color.Valid {
			v := in.Color.Value
			r.Color = &v
		} else {
			r.Color = nil
		}
	}
	if in.Archived.Set && in.Archived.Valid {
		r.Archived = in.Archived.Value
	}
	if err := s.Store.UpdateAccount(ctx, r); err != nil {
		return domain.Account{}, err
	}
	sums, err := s.Store.ConfirmedSumsByAccount(ctx)
	if err != nil {
		return domain.Account{}, err
	}
	return toAccount(*r, sums[r.ID]), nil
}

func (s *Services) DeleteAccount(ctx context.Context, id string) error {
	r, err := s.Store.FindAccountByID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return NotFound("conta não encontrada")
	}
	txCount, err := s.Store.CountAccountTransactions(ctx, id)
	if err != nil {
		return err
	}
	if txCount > 0 {
		return Conflict("conta possui transações — arquive-a em vez de excluir para preservar o histórico")
	}
	return s.Store.DeleteAccount(ctx, id)
}

// ───────────────────────── Categories ─────────────────────────

// ListCategories devolve as categorias com o tipo de gasto efetivo (próprio, herdado ou VARIABLE).
func (s *Services) ListCategories(ctx context.Context, kind *domain.CategoryKind) ([]domain.Category, error) {
	all, err := s.Store.ListCategories(ctx, nil)
	if err != nil {
		return nil, err
	}
	nodes := categoryNodes(all)
	out := make([]domain.Category, 0, len(all))
	for _, c := range all {
		if kind != nil && c.Kind != *kind {
			continue
		}
		c.SpendType = domain.EffectiveSpendType(c.ID, nodes)
		out = append(out, c)
	}
	return out, nil
}

func categoryNodes(cats []domain.Category) map[string]domain.CategoryNode {
	nodes := make(map[string]domain.CategoryNode, len(cats))
	for _, c := range cats {
		nodes[c.ID] = domain.CategoryNode{Kind: c.Kind, ParentID: c.ParentID, SpendType: c.SpendType}
	}
	return nodes
}

// categoryView devolve a categoria com o tipo de gasto efetivo.
func (s *Services) categoryView(ctx context.Context, id string) (domain.Category, error) {
	all, err := s.Store.ListCategories(ctx, nil)
	if err != nil {
		return domain.Category{}, err
	}
	nodes := categoryNodes(all)
	for _, c := range all {
		if c.ID == id {
			c.SpendType = domain.EffectiveSpendType(c.ID, nodes)
			return c, nil
		}
	}
	return domain.Category{}, NotFound("categoria não encontrada")
}

// normalizeSpendType aplica as regras de armazenamento: receitas não têm tipo de gasto e
// despesas raiz sem valor ficam VARIABLE (subcategorias sem valor herdam do pai).
func normalizeSpendType(kind domain.CategoryKind, parentID *string, st *domain.SpendType) *domain.SpendType {
	if kind != domain.CategoryExpense {
		return nil
	}
	if st == nil && parentID == nil {
		v := domain.SpendVariable
		return &v
	}
	return st
}

func (s *Services) CreateCategory(ctx context.Context, in domain.CreateCategoryInput) (domain.Category, error) {
	if issues := domain.ValidateCreateCategory(in); len(issues) > 0 {
		return domain.Category{}, ValidationFailed(issues)
	}
	dup, err := s.Store.FindCategoryByName(ctx, in.Name, in.Kind)
	if err != nil {
		return domain.Category{}, err
	}
	if dup != nil {
		return domain.Category{}, Conflict(fmt.Sprintf(`categoria %q (%s) já existe`, in.Name, in.Kind))
	}
	if in.ParentID != nil {
		if err := s.validateCategoryParent(ctx, *in.ParentID, in.Kind); err != nil {
			return domain.Category{}, err
		}
	}
	in.SpendType = normalizeSpendType(in.Kind, in.ParentID, in.SpendType)
	created, err := s.Store.CreateCategory(ctx, in)
	if err != nil {
		return domain.Category{}, err
	}
	return s.categoryView(ctx, created.ID)
}

func (s *Services) UpdateCategory(ctx context.Context, id string, in domain.UpdateCategoryInput) (domain.Category, error) {
	if issues := domain.ValidateUpdateCategory(in); len(issues) > 0 {
		return domain.Category{}, ValidationFailed(issues)
	}
	existing, err := s.Store.FindCategoryByID(ctx, id)
	if err != nil {
		return domain.Category{}, err
	}
	if existing == nil {
		return domain.Category{}, NotFound("categoria não encontrada")
	}
	if in.ParentID.Set && in.ParentID.Valid && in.ParentID.Value == id {
		return domain.Category{}, BadRequest("uma categoria não pode ser subcategoria dela mesma")
	}
	targetKind := existing.Kind
	if in.Kind.Set && in.Kind.Valid {
		targetKind = in.Kind.Value
	}
	if in.ParentID.Set && in.ParentID.Valid {
		if err := s.validateCategoryParent(ctx, in.ParentID.Value, targetKind); err != nil {
			return domain.Category{}, err
		}
	}
	if in.Name.Set && in.Name.Valid {
		existing.Name = in.Name.Value
	}
	if in.Kind.Set && in.Kind.Valid {
		existing.Kind = in.Kind.Value
	}
	if in.Icon.Set && in.Icon.Valid {
		existing.Icon = in.Icon.Value
	}
	if in.Color.Set && in.Color.Valid {
		existing.Color = in.Color.Value
	}
	if in.ParentID.Set {
		if in.ParentID.Valid {
			v := in.ParentID.Value
			existing.ParentID = &v
		} else {
			existing.ParentID = nil
		}
	}
	if in.SpendType.Set {
		if in.SpendType.Valid && existing.Kind == domain.CategoryIncome {
			return domain.Category{}, ValidationFailed([]domain.ValidationIssue{{Path: "spendType", Message: "tipo de gasto só se aplica a categorias de despesa"}})
		}
		if in.SpendType.Valid {
			v := in.SpendType.Value
			existing.SpendType = &v
		} else {
			existing.SpendType = nil
		}
	}
	existing.SpendType = normalizeSpendType(existing.Kind, existing.ParentID, existing.SpendType)
	if err := s.Store.UpdateCategory(ctx, existing); err != nil {
		return domain.Category{}, err
	}
	return s.categoryView(ctx, existing.ID)
}

func (s *Services) DeleteCategory(ctx context.Context, id string) error {
	existing, err := s.Store.FindCategoryByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return NotFound("categoria não encontrada")
	}
	return s.Store.DeleteCategory(ctx, id)
}

func (s *Services) validateCategoryParent(ctx context.Context, parentID string, kind domain.CategoryKind) error {
	parent, err := s.Store.FindCategoryByID(ctx, parentID)
	if err != nil {
		return err
	}
	if parent == nil {
		return NotFound("categoria pai não encontrada")
	}
	if parent.Kind != kind {
		return BadRequest("subcategoria deve ter o mesmo tipo (receita/despesa) da categoria pai")
	}
	return nil
}

// ───────────────────────── Cards & Invoices ─────────────────────────

func toInvoice(r store.InvoiceRow, card store.CardRow) domain.Invoice {
	period := domain.InvoicePeriod{Year: r.Year, Month: r.Month}
	closingDate := domain.InvoiceClosingDate(period, card.ClosingDay)
	return domain.Invoice{
		ID:          r.ID,
		CardID:      r.CardID,
		Month:       r.Month,
		Year:        r.Year,
		Status:      domain.EffectiveInvoiceStatus(r.Status == domain.InvoicePaid, closingDate, domain.TodayString()),
		TotalCents:  r.TotalCents,
		ClosingDate: closingDate,
		DueDate:     domain.InvoiceDueDate(period, card.ClosingDay, card.DueDay),
		PaidAt:      r.PaidAt,
	}
}

func (s *Services) toCard(ctx context.Context, r store.CardRow) (domain.Card, error) {
	usedLimit, err := s.Store.UnpaidCardTotalCents(ctx, r.ID)
	if err != nil {
		return domain.Card{}, err
	}
	invoices, err := s.Store.FindInvoicesWithTotals(ctx, r.ID)
	if err != nil {
		return domain.Card{}, err
	}
	current := domain.InvoicePeriodForPurchase(domain.TodayString(), r.ClosingDay)
	var currentInv *domain.Invoice
	for _, inv := range invoices {
		if inv.Year == current.Year && inv.Month == current.Month {
			c := toInvoice(inv, r)
			currentInv = &c
			break
		}
	}
	return domain.Card{
		ID:                  r.ID,
		Name:                r.Name,
		Brand:               r.Brand,
		LimitCents:          r.LimitCents,
		ClosingDay:          r.ClosingDay,
		DueDay:              r.DueDay,
		Color:               r.Color,
		Archived:            r.Archived,
		UsedLimitCents:      usedLimit,
		AvailableLimitCents: r.LimitCents - usedLimit,
		CurrentInvoice:      currentInv,
	}, nil
}

func (s *Services) ListCards(ctx context.Context, includeArchived bool) ([]domain.Card, error) {
	rows, err := s.Store.ListCards(ctx, includeArchived)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Card, 0, len(rows))
	for _, r := range rows {
		c, err := s.toCard(ctx, r)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Services) GetCard(ctx context.Context, id string) (domain.Card, error) {
	r, err := s.Store.FindCardByID(ctx, id)
	if err != nil {
		return domain.Card{}, err
	}
	if r == nil {
		return domain.Card{}, NotFound("cartão não encontrado")
	}
	return s.toCard(ctx, *r)
}

func (s *Services) CreateCard(ctx context.Context, in domain.CreateCardInput) (domain.Card, error) {
	if issues := domain.ValidateCreateCard(in); len(issues) > 0 {
		return domain.Card{}, ValidationFailed(issues)
	}
	created, err := s.Store.CreateCard(ctx, in)
	if err != nil {
		return domain.Card{}, err
	}
	return s.toCard(ctx, *created)
}

func (s *Services) UpdateCard(ctx context.Context, id string, in domain.UpdateCardInput) (domain.Card, error) {
	if issues := domain.ValidateUpdateCard(in); len(issues) > 0 {
		return domain.Card{}, ValidationFailed(issues)
	}
	r, err := s.Store.FindCardByID(ctx, id)
	if err != nil {
		return domain.Card{}, err
	}
	if r == nil {
		return domain.Card{}, NotFound("cartão não encontrado")
	}
	if in.Name.Set && in.Name.Valid {
		r.Name = in.Name.Value
	}
	if in.Brand.Set && in.Brand.Valid {
		r.Brand = in.Brand.Value
	}
	if in.LimitCents.Set && in.LimitCents.Valid {
		r.LimitCents = in.LimitCents.Value
	}
	if in.ClosingDay.Set && in.ClosingDay.Valid {
		r.ClosingDay = in.ClosingDay.Value
	}
	if in.DueDay.Set && in.DueDay.Valid {
		r.DueDay = in.DueDay.Value
	}
	if in.Color.Set {
		if in.Color.Valid {
			v := in.Color.Value
			r.Color = &v
		} else {
			r.Color = nil
		}
	}
	if in.Archived.Set && in.Archived.Valid {
		r.Archived = in.Archived.Value
	}
	if err := s.Store.UpdateCard(ctx, r); err != nil {
		return domain.Card{}, err
	}
	return s.toCard(ctx, *r)
}

func (s *Services) DeleteCard(ctx context.Context, id string) error {
	r, err := s.Store.FindCardByID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return NotFound("cartão não encontrado")
	}
	txCount, err := s.Store.CountCardTransactions(ctx, id)
	if err != nil {
		return err
	}
	if txCount > 0 {
		return Conflict("cartão possui lançamentos — arquive-o em vez de excluir para preservar o histórico")
	}
	return s.Store.DeleteCard(ctx, id)
}

func (s *Services) AddCardExpense(ctx context.Context, cardID string, in domain.CreateCardExpenseInput) (domain.CardExpenseResult, error) {
	// Installments 0 não vira 1 aqui: o padrão só vale quando o campo JSON está ausente
	// (ver CreateCardExpenseInput.UnmarshalJSON); 0 explícito é erro de validação.
	if in.CardID == "" {
		in.CardID = cardID
	}
	if issues := domain.ValidateCreateCardExpense(in); len(issues) > 0 {
		return domain.CardExpenseResult{}, ValidationFailed(issues)
	}
	card, err := s.Store.FindCardByID(ctx, cardID)
	if err != nil {
		return domain.CardExpenseResult{}, err
	}
	if card == nil {
		return domain.CardExpenseResult{}, NotFound("cartão não encontrado")
	}
	if in.CategoryID == nil {
		if in.CategoryID, err = s.SuggestCategory(ctx, in.Description, domain.CategoryExpense); err != nil {
			return domain.CardExpenseResult{}, err
		}
	}
	if err := s.requireTags(ctx, in.TagIDs); err != nil {
		return domain.CardExpenseResult{}, err
	}
	tagIDs, err := s.withRuleTag(ctx, in.TagIDs, in.Description, domain.CategoryExpense)
	if err != nil {
		return domain.CardExpenseResult{}, err
	}
	amounts, err := domain.SplitInstallments(in.TotalCents, in.Installments)
	if err != nil {
		return domain.CardExpenseResult{}, BadRequest(err.Error())
	}
	basePeriod := domain.InvoicePeriodForPurchase(in.Date, card.ClosingDay)
	many := in.Installments > 1

	legs := make([]store.CardExpenseLeg, in.Installments)
	for i, amt := range amounts {
		desc := in.Description
		var num, tot *int
		if many {
			desc = fmt.Sprintf("%s (%d/%d)", in.Description, i+1, in.Installments)
			n := i + 1
			t := in.Installments
			num = &n
			tot = &t
		}
		legs[i] = store.CardExpenseLeg{
			Description:       desc,
			AmountCents:       amt,
			Date:              domain.AddMonthsToDateString(in.Date, i),
			CategoryID:        in.CategoryID,
			Notes:             in.Notes,
			InstallmentNumber: num,
			InstallmentTotal:  tot,
			InvoicePeriod:     domain.AddMonthsToPeriod(basePeriod, i),
			TagIDs:            tagIDs,
		}
	}

	created, err := s.Store.CreateCardExpense(ctx, cardID, legs)
	if err != nil {
		return domain.CardExpenseResult{}, err
	}
	// Só a 1ª parcela (data da compra) pode afetar o orçamento do mês corrente.
	if in.CategoryID != nil && len(amounts) > 0 && domain.IsInCurrentMonth(in.Date) {
		catID := *in.CategoryID
		firstAmt := amounts[0]
		go s.Notifications.NotifyBudgetThresholdCrossed(context.Background(), catID, firstAmt)
	}
	return domain.CardExpenseResult{Created: created}, nil
}

func (s *Services) ListInvoices(ctx context.Context, cardID string) ([]domain.Invoice, error) {
	card, err := s.Store.FindCardByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, NotFound("cartão não encontrado")
	}
	rows, err := s.Store.FindInvoicesWithTotals(ctx, cardID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Invoice, 0, len(rows))
	for _, r := range rows {
		out = append(out, toInvoice(r, *card))
	}
	return out, nil
}

func (s *Services) ListInvoiceItems(ctx context.Context, cardID, invoiceID string) ([]domain.Transaction, error) {
	inv, err := s.Store.FindInvoiceByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil || inv.CardID != cardID {
		return nil, NotFound("fatura não encontrada para este cartão")
	}
	return s.Store.FindInvoiceItems(ctx, invoiceID)
}

func (s *Services) PayInvoice(ctx context.Context, cardID, invoiceID string, in domain.PayInvoiceInput) (domain.Invoice, error) {
	if issues := domain.ValidatePayInvoice(in); len(issues) > 0 {
		return domain.Invoice{}, ValidationFailed(issues)
	}
	card, err := s.Store.FindCardByID(ctx, cardID)
	if err != nil {
		return domain.Invoice{}, err
	}
	if card == nil {
		return domain.Invoice{}, NotFound("cartão não encontrado")
	}
	inv, err := s.Store.FindInvoiceByID(ctx, invoiceID)
	if err != nil {
		return domain.Invoice{}, err
	}
	if inv == nil || inv.CardID != cardID {
		return domain.Invoice{}, NotFound("fatura não encontrada para este cartão")
	}
	if inv.Status == domain.InvoicePaid {
		return domain.Invoice{}, Conflict("fatura já está paga")
	}
	invoices, err := s.Store.FindInvoicesWithTotals(ctx, cardID)
	if err != nil {
		return domain.Invoice{}, err
	}
	var totalCents int64
	for _, row := range invoices {
		if row.ID == invoiceID {
			totalCents = row.TotalCents
			break
		}
	}
	if totalCents <= 0 {
		return domain.Invoice{}, Conflict("fatura sem valor a pagar")
	}
	date := domain.TodayString()
	if in.Date != nil && *in.Date != "" {
		date = *in.Date
	}
	desc := fmt.Sprintf("Pagamento fatura %s %02d/%d — %s", card.Name, inv.Month, inv.Year, domain.FormatBRL(totalCents))
	if err := s.Store.PayInvoice(ctx, invoiceID, in.AccountID, date, desc, totalCents); err != nil {
		// A checagem acima é otimista; dentro da transação o store detecta pagamento concorrente.
		if errors.Is(err, store.ErrConflict) {
			return domain.Invoice{}, Conflict("fatura já está paga")
		}
		return domain.Invoice{}, err
	}
	refreshed, err := s.Store.FindInvoicesWithTotals(ctx, cardID)
	if err != nil {
		return domain.Invoice{}, err
	}
	for _, row := range refreshed {
		if row.ID == invoiceID {
			return toInvoice(row, *card), nil
		}
	}
	return domain.Invoice{}, NotFound("fatura não encontrada após pagamento")
}

// ───────────────────────── Transactions ─────────────────────────

func (s *Services) ListTransactions(ctx context.Context, q domain.ListTransactionsQuery) (domain.Paginated[domain.Transaction], error) {
	if issues := domain.ValidateListTransactionsQuery(q); len(issues) > 0 {
		return domain.Paginated[domain.Transaction]{}, ValidationFailed(issues)
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 50
	}
	items, total, err := s.Store.ListTransactions(ctx, q)
	if err != nil {
		return domain.Paginated[domain.Transaction]{}, err
	}
	return domain.Paginated[domain.Transaction]{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}, nil
}

func (s *Services) GetTransaction(ctx context.Context, id string) (domain.Transaction, error) {
	detail, err := s.Store.FindTransactionByID(ctx, id)
	if err != nil {
		return domain.Transaction{}, err
	}
	if detail == nil {
		return domain.Transaction{}, NotFound("transação não encontrada")
	}
	return detail.Transaction, nil
}

func (s *Services) CreateTransaction(ctx context.Context, in domain.CreateTransactionInput) (domain.Transaction, error) {
	if in.Status == "" {
		in.Status = domain.TxConfirmed
	}
	if issues := domain.ValidateCreateTransaction(in); len(issues) > 0 {
		return domain.Transaction{}, ValidationFailed(issues)
	}
	if err := s.requireActiveAccount(ctx, in.AccountID); err != nil {
		return domain.Transaction{}, err
	}
	if in.CategoryID == nil {
		kind, _ := domain.CategoryKindForTransaction(in.Type)
		suggested, err := s.SuggestCategory(ctx, in.Description, kind)
		if err != nil {
			return domain.Transaction{}, err
		}
		in.CategoryID = suggested
	}
	if err := s.requireTags(ctx, in.TagIDs); err != nil {
		return domain.Transaction{}, err
	}
	ruleKind, _ := domain.CategoryKindForTransaction(in.Type)
	tagIDs, err := s.withRuleTag(ctx, in.TagIDs, in.Description, ruleKind)
	if err != nil {
		return domain.Transaction{}, err
	}
	created, err := s.Store.CreateTransaction(
		ctx,
		in.Type,
		in.Description,
		domain.SignedAmount(in.Type, in.AmountCents),
		in.Date,
		in.Status,
		in.AccountID,
		in.CategoryID,
		in.Notes,
	)
	if err != nil {
		return domain.Transaction{}, err
	}
	if len(tagIDs) > 0 {
		if err := s.Store.SetTransactionTags(ctx, created.ID, tagIDs); err != nil {
			return domain.Transaction{}, err
		}
		if created, err = s.reloadTransaction(ctx, created.ID); err != nil {
			return domain.Transaction{}, err
		}
	}
	if in.Type == domain.TxExpense && in.CategoryID != nil && domain.IsInCurrentMonth(in.Date) {
		catID := *in.CategoryID
		amt := in.AmountCents
		go s.Notifications.NotifyBudgetThresholdCrossed(context.Background(), catID, amt)
	}
	return *created, nil
}

func (s *Services) UpdateTransaction(ctx context.Context, id string, in domain.UpdateTransactionInput) (domain.Transaction, error) {
	if issues := domain.ValidateUpdateTransaction(in); len(issues) > 0 {
		return domain.Transaction{}, ValidationFailed(issues)
	}
	detail, err := s.Store.FindTransactionByID(ctx, id)
	if err != nil {
		return domain.Transaction{}, err
	}
	if detail == nil {
		return domain.Transaction{}, NotFound("transação não encontrada")
	}
	tx := detail.Transaction
	isSimple := tx.Type == domain.TxIncome || tx.Type == domain.TxExpense
	if !isSimple && (in.AmountCents.Set || in.AccountID.Set) {
		return domain.Transaction{}, Conflict("valor e conta de transferências, compras no cartão e pagamentos de fatura não podem ser editados — exclua e lance novamente")
	}
	if in.AccountID.Set && in.AccountID.Valid {
		if err := s.requireActiveAccount(ctx, in.AccountID.Value); err != nil {
			return domain.Transaction{}, err
		}
		v := in.AccountID.Value
		tx.AccountID = &v
	}
	if in.Description.Set && in.Description.Valid {
		tx.Description = in.Description.Value
	}
	if in.AmountCents.Set && in.AmountCents.Valid && isSimple {
		tx.AmountCents = domain.SignedAmount(tx.Type, in.AmountCents.Value)
	}
	if in.Date.Set && in.Date.Valid {
		tx.Date = in.Date.Value
	}
	if in.Status.Set && in.Status.Valid {
		tx.Status = in.Status.Value
	}
	if in.CategoryID.Set {
		if in.CategoryID.Valid {
			v := in.CategoryID.Value
			tx.CategoryID = &v
		} else {
			tx.CategoryID = nil
		}
	}
	if in.Notes.Set {
		if in.Notes.Valid {
			v := in.Notes.Value
			tx.Notes = &v
		} else {
			tx.Notes = nil
		}
	}
	var tagIDs []string
	if in.TagIDs.Set && in.TagIDs.Valid {
		tagIDs = domain.UniqueStrings(in.TagIDs.Value)
	}
	if err := s.requireTags(ctx, tagIDs); err != nil {
		return domain.Transaction{}, err
	}
	updated, err := s.Store.UpdateTransaction(ctx, &tx)
	if err != nil {
		return domain.Transaction{}, err
	}
	if in.TagIDs.Set {
		if err := s.Store.SetTransactionTags(ctx, id, tagIDs); err != nil {
			return domain.Transaction{}, err
		}
		if updated, err = s.reloadTransaction(ctx, id); err != nil {
			return domain.Transaction{}, err
		}
	}
	return *updated, nil
}

// reloadTransaction relê a transação (com as tags) depois de uma alteração.
func (s *Services) reloadTransaction(ctx context.Context, id string) (*domain.Transaction, error) {
	detail, err := s.Store.FindTransactionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, NotFound("transação não encontrada")
	}
	return &detail.Transaction, nil
}

func (s *Services) ConfirmTransaction(ctx context.Context, id string) (domain.Transaction, error) {
	detail, err := s.Store.FindTransactionByID(ctx, id)
	if err != nil {
		return domain.Transaction{}, err
	}
	if detail == nil {
		return domain.Transaction{}, NotFound("transação não encontrada")
	}
	if detail.Status == domain.TxConfirmed {
		return detail.Transaction, nil
	}
	detail.Status = domain.TxConfirmed
	updated, err := s.Store.UpdateTransaction(ctx, &detail.Transaction)
	if err != nil {
		return domain.Transaction{}, err
	}
	return *updated, nil
}

func (s *Services) DeleteTransaction(ctx context.Context, id, scope string) error {
	if scope == "" {
		scope = "single"
	}
	if scope != "single" && scope != "group" {
		return ValidationFailed([]domain.ValidationIssue{{Path: "scope", Message: "scope deve ser single ou group"}})
	}
	detail, err := s.Store.FindTransactionByID(ctx, id)
	if err != nil {
		return err
	}
	if detail == nil {
		return NotFound("transação não encontrada")
	}
	if detail.TransferGroupID != nil {
		return s.Store.DeleteTransferGroup(ctx, *detail.TransferGroupID)
	}
	if detail.Type == domain.TxInvoicePayment {
		invoiceID, err := s.Store.FindInvoicePaidBy(ctx, id)
		if err != nil {
			return err
		}
		if invoiceID != "" {
			if err := s.Store.DeleteInvoicePayment(ctx, id, invoiceID); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return NotFound("pagamento de fatura não encontrado")
				}
				return err
			}
			return nil
		}
	}
	if scope == "group" && detail.InstallmentGroupID != nil {
		return s.Store.DeleteInstallmentGroup(ctx, *detail.InstallmentGroupID)
	}
	return s.Store.DeleteTransaction(ctx, id)
}

func (s *Services) Transfer(ctx context.Context, in domain.TransferInput) (domain.TransferResult, error) {
	if issues := domain.ValidateTransfer(in); len(issues) > 0 {
		return domain.TransferResult{}, ValidationFailed(issues)
	}
	from, err := s.Store.FindAccountByID(ctx, in.FromAccountID)
	if err != nil {
		return domain.TransferResult{}, err
	}
	if from == nil {
		return domain.TransferResult{}, NotFound("conta de origem não encontrada")
	}
	to, err := s.Store.FindAccountByID(ctx, in.ToAccountID)
	if err != nil {
		return domain.TransferResult{}, err
	}
	if to == nil {
		return domain.TransferResult{}, NotFound("conta de destino não encontrada")
	}
	if from.Archived || to.Archived {
		return domain.TransferResult{}, BadRequest("não é possível transferir usando contas arquivadas")
	}
	base := ""
	if in.Description != nil {
		base = strings.TrimSpace(*in.Description)
	}
	fromDesc := fmt.Sprintf("Transferência para %s", to.Name)
	toDesc := fmt.Sprintf("Transferência de %s", from.Name)
	if base != "" {
		fromDesc = fmt.Sprintf("%s → %s", base, to.Name)
		toDesc = fmt.Sprintf("%s ← %s", base, from.Name)
	}
	groupID, err := s.Store.CreateTransferPair(ctx, in.FromAccountID, in.ToAccountID, in.AmountCents, in.Date, fromDesc, toDesc)
	if err != nil {
		return domain.TransferResult{}, err
	}
	return domain.TransferResult{TransferGroupID: groupID}, nil
}

func (s *Services) ExportTransactions(ctx context.Context, format string, q domain.ListTransactionsQuery) (content, contentType, filename string, err error) {
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "ofx" && format != "xlsx" {
		return "", "", "", ValidationFailed([]domain.ValidationIssue{{Path: "format", Message: "formato deve ser csv, ofx ou xlsx"}})
	}
	if issues := domain.ValidateTransactionFilters(q); len(issues) > 0 {
		return "", "", "", ValidationFailed(issues)
	}
	items, err := s.Store.ListAllTransactions(ctx, q)
	if err != nil {
		return "", "", "", err
	}
	stamp := domain.TodayString()
	if format == "ofx" {
		return domain.BuildOFX(items), "application/x-ofx", fmt.Sprintf("finflow-extrato-%s.ofx", stamp), nil
	}
	if format == "xlsx" {
		data, err := exportTransactionsXLSX(items)
		if err != nil {
			return "", "", "", err
		}
		return string(data), XLSXContentType, fmt.Sprintf("finflow-extrato-%s.xlsx", stamp), nil
	}
	return domain.BuildCSV(items), "text/csv; charset=utf-8", fmt.Sprintf("finflow-extrato-%s.csv", stamp), nil
}

func (s *Services) requireActiveAccount(ctx context.Context, accountID string) error {
	acc, err := s.Store.FindAccountByID(ctx, accountID)
	if err != nil {
		return err
	}
	if acc == nil {
		return NotFound("conta não encontrada")
	}
	if acc.Archived {
		return BadRequest("conta está arquivada")
	}
	return nil
}

// ───────────────────────── Budgets ─────────────────────────

func toBudget(r store.BudgetRow, spentCents int64) domain.Budget {
	view := domain.ComputeBudgetView(r.AmountCents, spentCents)
	return domain.Budget{
		ID:            r.ID,
		CategoryID:    r.CategoryID,
		CategoryName:  r.CategoryName,
		CategoryIcon:  r.CategoryIcon,
		CategoryColor: r.CategoryColor,
		AmountCents:   r.AmountCents,
		SpentCents:    spentCents,
		Percent:       view.Percent,
		Status:        view.Status,
	}
}

// resolvePeriod valida o período (mesmas regras da camada HTTP — CLI e MCP locais chamam
// o serviço direto) e completa mês/ano ausentes com o mês corrente.
func resolvePeriod(q domain.PeriodQuery) (month, year int, err error) {
	if issues := domain.ValidatePeriodQuery(q); len(issues) > 0 {
		return 0, 0, ValidationFailed(issues)
	}
	curM, curY := domain.CurrentMonthYear()
	month, year = curM, curY
	if q.Month != nil {
		month = *q.Month
	}
	if q.Year != nil {
		year = *q.Year
	}
	return month, year, nil
}

func (s *Services) ListBudgets(ctx context.Context, q domain.PeriodQuery) ([]domain.Budget, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListBudgets(ctx)
	if err != nil {
		return nil, err
	}
	spent, err := s.Store.SpentByCategory(ctx, year, month)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Budget, 0, len(rows))
	for _, r := range rows {
		out = append(out, toBudget(r, spent[r.CategoryID]))
	}
	return out, nil
}

func (s *Services) UpsertBudget(ctx context.Context, in domain.UpsertBudgetInput) (domain.Budget, error) {
	if issues := domain.ValidateUpsertBudget(in); len(issues) > 0 {
		return domain.Budget{}, ValidationFailed(issues)
	}
	cat, err := s.Store.FindCategoryByID(ctx, in.CategoryID)
	if err != nil {
		return domain.Budget{}, err
	}
	if cat == nil {
		return domain.Budget{}, NotFound("categoria não encontrada")
	}
	if cat.Kind != domain.CategoryExpense {
		return domain.Budget{}, BadRequest("orçamentos só se aplicam a categorias de despesa")
	}
	saved, err := s.Store.UpsertBudget(ctx, in.CategoryID, in.AmountCents)
	if err != nil {
		return domain.Budget{}, err
	}
	month, year := domain.CurrentMonthYear()
	spent, err := s.Store.SpentByCategory(ctx, year, month)
	if err != nil {
		return domain.Budget{}, err
	}
	return toBudget(*saved, spent[in.CategoryID]), nil
}

func (s *Services) DeleteBudget(ctx context.Context, id string) error {
	found, err := s.Store.FindBudgetByID(ctx, id)
	if err != nil {
		return err
	}
	if found == nil {
		return NotFound("orçamento não encontrado")
	}
	return s.Store.DeleteBudget(ctx, id)
}

// ───────────────────────── Goals ─────────────────────────

func toGoal(r store.GoalRow) domain.Goal {
	return domain.Goal{
		ID:                 r.ID,
		Name:               r.Name,
		Kind:               r.Kind,
		Icon:               r.Icon,
		Color:              r.Color,
		TargetCents:        r.TargetCents,
		SavedCents:         r.SavedCents,
		Deadline:           r.Deadline,
		Percent:            domain.GoalPercent(r.TargetCents, r.SavedCents),
		MonthlyNeededCents: domain.MonthlyNeededCents(r.TargetCents, r.SavedCents, r.Deadline, domain.TodayString()),
	}
}

func (s *Services) ListGoals(ctx context.Context) ([]domain.Goal, error) {
	rows, err := s.Store.ListGoals(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Goal, 0, len(rows))
	for _, r := range rows {
		out = append(out, toGoal(r))
	}
	return out, nil
}

func (s *Services) GetGoal(ctx context.Context, id string) (domain.Goal, error) {
	r, err := s.Store.FindGoalByID(ctx, id)
	if err != nil {
		return domain.Goal{}, err
	}
	if r == nil {
		return domain.Goal{}, NotFound("meta não encontrada")
	}
	return toGoal(*r), nil
}

func (s *Services) CreateGoal(ctx context.Context, in domain.CreateGoalInput) (domain.Goal, error) {
	if in.Icon == "" {
		in.Icon = "piggy-bank"
	}
	if in.Kind == "" {
		in.Kind = domain.GoalKindGoal
	}
	if issues := domain.ValidateCreateGoal(in); len(issues) > 0 {
		return domain.Goal{}, ValidationFailed(issues)
	}
	if in.Kind == domain.GoalKindReserve {
		existing, err := s.Store.FindActiveReserveGoal(ctx)
		if err != nil {
			return domain.Goal{}, err
		}
		if existing != nil {
			return domain.Goal{}, Conflict(fmt.Sprintf("já existe uma reserva de emergência ativa (%q)", existing.Name))
		}
	}
	created, err := s.Store.CreateGoal(ctx, in)
	if err != nil {
		return domain.Goal{}, err
	}
	return toGoal(*created), nil
}

func (s *Services) UpdateGoal(ctx context.Context, id string, in domain.UpdateGoalInput) (domain.Goal, error) {
	if issues := domain.ValidateUpdateGoal(in); len(issues) > 0 {
		return domain.Goal{}, ValidationFailed(issues)
	}
	r, err := s.Store.FindGoalByID(ctx, id)
	if err != nil {
		return domain.Goal{}, err
	}
	if r == nil {
		return domain.Goal{}, NotFound("meta não encontrada")
	}
	if in.Name.Set && in.Name.Valid {
		r.Name = in.Name.Value
	}
	if in.Icon.Set && in.Icon.Valid {
		r.Icon = in.Icon.Value
	}
	if in.Color.Set {
		if in.Color.Valid {
			v := in.Color.Value
			r.Color = &v
		} else {
			r.Color = nil
		}
	}
	if in.TargetCents.Set && in.TargetCents.Valid {
		r.TargetCents = in.TargetCents.Value
	}
	if in.Deadline.Set {
		if in.Deadline.Valid {
			v := in.Deadline.Value
			r.Deadline = &v
		} else {
			r.Deadline = nil
		}
	}
	if err := s.Store.UpdateGoal(ctx, r); err != nil {
		return domain.Goal{}, err
	}
	refreshed, err := s.Store.FindGoalByID(ctx, id)
	if err != nil || refreshed == nil {
		return domain.Goal{}, NotFound("meta não encontrada")
	}
	return toGoal(*refreshed), nil
}

func (s *Services) DeleteGoal(ctx context.Context, id string) error {
	r, err := s.Store.FindGoalByID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return NotFound("meta não encontrada")
	}
	return s.Store.DeleteGoal(ctx, id)
}

func (s *Services) ContributeGoal(ctx context.Context, id string, in domain.ContributeGoalInput) (domain.Goal, error) {
	if issues := domain.ValidateContributeGoal(in); len(issues) > 0 {
		return domain.Goal{}, ValidationFailed(issues)
	}
	r, err := s.Store.FindGoalByID(ctx, id)
	if err != nil {
		return domain.Goal{}, err
	}
	if r == nil {
		return domain.Goal{}, NotFound("meta não encontrada")
	}
	if r.SavedCents+in.AmountCents < 0 {
		return domain.Goal{}, BadRequest("resgate maior que o valor guardado na meta")
	}
	date := domain.TodayString()
	if in.Date != nil && *in.Date != "" {
		date = *in.Date
	}
	if err := s.Store.AddGoalContribution(ctx, id, in.AmountCents, date, in.Note); err != nil {
		return domain.Goal{}, err
	}
	refreshed, err := s.Store.FindGoalByID(ctx, id)
	if err != nil || refreshed == nil {
		return domain.Goal{}, NotFound("meta não encontrada")
	}
	return toGoal(*refreshed), nil
}

// ───────────────────────── Dashboard ─────────────────────────

func (s *Services) DashboardSummary(ctx context.Context, q domain.PeriodQuery) (domain.DashboardSummary, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return domain.DashboardSummary{}, err
	}
	totalBalance, err := s.Store.TotalBalanceCents(ctx)
	if err != nil {
		return domain.DashboardSummary{}, err
	}
	income, err := s.Store.MonthIncomeCents(ctx, year, month)
	if err != nil {
		return domain.DashboardSummary{}, err
	}
	expense, err := s.Store.MonthExpenseCents(ctx, year, month)
	if err != nil {
		return domain.DashboardSummary{}, err
	}
	openInv, err := s.Store.OpenInvoicesCents(ctx)
	if err != nil {
		return domain.DashboardSummary{}, err
	}
	pending, err := s.Store.PendingCount(ctx)
	if err != nil {
		return domain.DashboardSummary{}, err
	}
	return domain.DashboardSummary{
		Month:             month,
		Year:              year,
		TotalBalanceCents: totalBalance,
		MonthIncomeCents:  income,
		MonthExpenseCents: expense,
		MonthNetCents:     income - expense,
		OpenInvoicesCents: openInv,
		PendingCount:      pending,
	}, nil
}

func (s *Services) DashboardCashflow(ctx context.Context, q domain.PeriodQuery) ([]domain.CashflowPoint, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return nil, err
	}
	movements, err := s.Store.MonthMovements(ctx, year, month)
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]*domain.CashflowPoint)
	for _, m := range movements {
		p, ok := byDay[m.Date]
		if !ok {
			p = &domain.CashflowPoint{Date: m.Date}
			byDay[m.Date] = p
		}
		if m.Type == domain.TxIncome {
			p.IncomeCents += m.AmountCents
		} else {
			amt := m.AmountCents
			if amt < 0 {
				amt = -amt
			}
			p.ExpenseCents += amt
		}
	}
	out := make([]domain.CashflowPoint, 0, len(byDay))
	for _, p := range byDay {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

func (s *Services) DashboardCategories(ctx context.Context, q domain.PeriodQuery) ([]domain.CategorySlice, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return nil, err
	}
	groups, err := s.Store.ExpensesByCategory(ctx, year, month)
	if err != nil {
		return nil, err
	}
	var total int64
	for _, g := range groups {
		total += g.TotalCents
	}
	out := make([]domain.CategorySlice, 0, len(groups))
	for _, g := range groups {
		name := "Sem categoria"
		if g.Name != nil && *g.Name != "" {
			name = *g.Name
		}
		var pct float64
		if total > 0 {
			pct = math.Round((float64(g.TotalCents)/float64(total))*1000) / 10
		}
		out = append(out, domain.CategorySlice{
			CategoryID: g.CategoryID,
			Name:       name,
			Icon:       g.Icon,
			Color:      g.Color,
			TotalCents: g.TotalCents,
			Percent:    pct,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalCents > out[j].TotalCents })
	return out, nil
}

func (s *Services) DashboardUpcoming(ctx context.Context) ([]domain.UpcomingItem, error) {
	today := domain.TodayString()
	limit := domain.AddDays(today, 7)

	pending, err := s.Store.PendingBetween(ctx, today, limit)
	if err != nil {
		return nil, err
	}
	invoices, err := s.Store.UnpaidInvoicesWithTotals(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]domain.UpcomingItem, 0, len(pending)+len(invoices))
	for _, t := range pending {
		items = append(items, domain.UpcomingItem{
			ID:           t.ID,
			Kind:         "TRANSACTION",
			Description:  t.Description,
			DueDate:      t.Date,
			AmountCents:  t.AmountCents,
			CategoryIcon: t.CategoryIcon,
		})
	}
	creditCardIcon := "credit-card"
	for _, inv := range invoices {
		if inv.TotalCents <= 0 {
			continue
		}
		dueDate := domain.InvoiceDueDate(
			domain.InvoicePeriod{Year: inv.Year, Month: inv.Month},
			inv.ClosingDay,
			inv.DueDay,
		)
		if dueDate >= today && dueDate <= limit {
			items = append(items, domain.UpcomingItem{
				ID:           inv.ID,
				Kind:         "INVOICE",
				Description:  "Fatura " + inv.CardName,
				DueDate:      dueDate,
				AmountCents:  -inv.TotalCents,
				CategoryIcon: &creditCardIcon,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DueDate < items[j].DueDate })
	return items, nil
}
