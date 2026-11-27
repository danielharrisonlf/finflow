package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"finflow/internal/db"
	"finflow/internal/domain"

	"github.com/google/uuid"
)

// Erros sentinela devolvidos pelo store; a camada de serviço deve usar errors.Is
// para mapeá-los (ErrConflict → 409, ErrNotFound → 404).
var (
	// ErrConflict indica que a operação conflita com o estado atual (ex.: fatura já paga).
	ErrConflict = errors.New("conflito com o estado atual")
	// ErrNotFound indica que o registro alvo da operação não existe mais.
	ErrNotFound = errors.New("registro não encontrado")
)

type Store struct {
	DB *db.DB
}

func New(d *db.DB) *Store {
	return &Store{DB: d}
}

// timestampLayout tem largura fixa (nanossegundos sempre com 9 dígitos, UTC) para
// que a ordenação lexicográfica das colunas TEXT do SQLite coincida com a
// cronológica. RFC3339Nano remove zeros à direita e quebra essa ordem.
const timestampLayout = "2006-01-02T15:04:05.000000000Z"

func nowStr() string {
	return time.Now().UTC().Format(timestampLayout)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ───────────────────────── Accounts ─────────────────────────

type AccountRow struct {
	ID                  string
	Name                string
	Type                domain.AccountType
	InitialBalanceCents int64
	Color               *string
	Archived            bool
}

func (s *Store) ListAccounts(ctx context.Context, includeArchived bool) ([]AccountRow, error) {
	q := `SELECT id, name, type, initial_balance_cents, color, archived FROM accounts`
	if !includeArchived {
		q += ` WHERE archived = ?`
	}
	q += ` ORDER BY created_at ASC`

	var (
		rows *sql.Rows
		err  error
	)
	if includeArchived {
		rows, err = s.DB.Query(ctx, q)
	} else {
		rows, err = s.DB.Query(ctx, q, false)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AccountRow
	for rows.Next() {
		var r AccountRow
		var color sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.InitialBalanceCents, &color, &r.Archived); err != nil {
			return nil, err
		}
		r.Color = db.NullStringPtr(color)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindAccountByID(ctx context.Context, id string) (*AccountRow, error) {
	var r AccountRow
	var color sql.NullString
	err := s.DB.QueryRow(ctx,
		`SELECT id, name, type, initial_balance_cents, color, archived FROM accounts WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.Type, &r.InitialBalanceCents, &color, &r.Archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Color = db.NullStringPtr(color)
	return &r, nil
}

func (s *Store) ConfirmedSumsByAccount(ctx context.Context) (map[string]int64, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT account_id, COALESCE(SUM(amount_cents), 0)
		FROM transactions
		WHERE status = 'CONFIRMED' AND account_id IS NOT NULL
		GROUP BY account_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sums := make(map[string]int64)
	for rows.Next() {
		var id string
		var sum int64
		if err := rows.Scan(&id, &sum); err != nil {
			return nil, err
		}
		sums[id] = sum
	}
	return sums, rows.Err()
}

func (s *Store) CountAccountTransactions(ctx context.Context, accountID string) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM transactions WHERE account_id = ?`, accountID).Scan(&n)
	return n, err
}

func (s *Store) CountAccounts(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&n)
	return n, err
}

func (s *Store) CreateAccount(ctx context.Context, in domain.CreateAccountInput) (*AccountRow, error) {
	id := uuid.NewString()
	now := nowStr()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO accounts (id, name, type, initial_balance_cents, color, archived, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, in.Name, string(in.Type), in.InitialBalanceCents, in.Color, false, now, now)
	if err != nil {
		return nil, err
	}
	return &AccountRow{
		ID:                  id,
		Name:                in.Name,
		Type:                in.Type,
		InitialBalanceCents: in.InitialBalanceCents,
		Color:               in.Color,
		Archived:            false,
	}, nil
}

func (s *Store) UpdateAccount(ctx context.Context, r *AccountRow) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE accounts
		SET name = ?, type = ?, initial_balance_cents = ?, color = ?, archived = ?, updated_at = ?
		WHERE id = ?
	`, r.Name, string(r.Type), r.InitialBalanceCents, r.Color, r.Archived, nowStr(), r.ID)
	return err
}

func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	return err
}

// ───────────────────────── Categories ─────────────────────────

// As categorias saem do store com o spend_type próprio (nil = herda do pai); o tipo
// efetivo é resolvido no serviço.
func (s *Store) ListCategories(ctx context.Context, kind *domain.CategoryKind) ([]domain.Category, error) {
	q := `SELECT id, name, kind, icon, color, parent_id, spend_type FROM categories`
	var args []any
	if kind != nil {
		q += ` WHERE kind = ?`
		args = append(args, string(*kind))
	}
	// INCOME antes de EXPENSE, como na ordem do enum original.
	q += ` ORDER BY CASE kind WHEN 'INCOME' THEN 0 ELSE 1 END, name ASC`

	rows, err := s.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Category, 0)
	for rows.Next() {
		var c domain.Category
		var parentID, spendType sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Kind, &c.Icon, &c.Color, &parentID, &spendType); err != nil {
			return nil, err
		}
		c.ParentID = db.NullStringPtr(parentID)
		c.SpendType = spendTypePtr(spendType)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) FindCategoryByID(ctx context.Context, id string) (*domain.Category, error) {
	var c domain.Category
	var parentID, spendType sql.NullString
	err := s.DB.QueryRow(ctx,
		`SELECT id, name, kind, icon, color, parent_id, spend_type FROM categories WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Kind, &c.Icon, &c.Color, &parentID, &spendType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.ParentID = db.NullStringPtr(parentID)
	c.SpendType = spendTypePtr(spendType)
	return &c, nil
}

func spendTypePtr(ns sql.NullString) *domain.SpendType {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	st := domain.SpendType(ns.String)
	return &st
}

func spendTypeArg(st *domain.SpendType) any {
	if st == nil {
		return nil
	}
	return string(*st)
}

func (s *Store) FindCategoryByName(ctx context.Context, name string, kind domain.CategoryKind) (*domain.Category, error) {
	all, err := s.ListCategories(ctx, &kind)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if strings.EqualFold(all[i].Name, name) {
			return &all[i], nil
		}
	}
	return nil, nil
}

func (s *Store) CreateCategory(ctx context.Context, in domain.CreateCategoryInput) (*domain.Category, error) {
	id := uuid.NewString()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO categories (id, name, kind, icon, color, parent_id, spend_type)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, in.Name, string(in.Kind), in.Icon, in.Color, in.ParentID, spendTypeArg(in.SpendType))
	if err != nil {
		return nil, err
	}
	return &domain.Category{
		ID:        id,
		Name:      in.Name,
		Kind:      in.Kind,
		Icon:      in.Icon,
		Color:     in.Color,
		ParentID:  in.ParentID,
		SpendType: in.SpendType,
	}, nil
}

func (s *Store) UpdateCategory(ctx context.Context, c *domain.Category) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE categories
		SET name = ?, kind = ?, icon = ?, color = ?, parent_id = ?, spend_type = ?
		WHERE id = ?
	`, c.Name, string(c.Kind), c.Icon, c.Color, c.ParentID, spendTypeArg(c.SpendType), c.ID)
	return err
}

func (s *Store) DeleteCategory(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM categories WHERE id = ?`, id)
	return err
}

// ───────────────────────── Cards & Invoices ─────────────────────────

type CardRow struct {
	ID         string
	Name       string
	Brand      domain.CardBrand
	LimitCents int64
	ClosingDay int
	DueDay     int
	Color      *string
	Archived   bool
}

type InvoiceRow struct {
	ID                   string
	CardID               string
	Month                int
	Year                 int
	Status               domain.InvoiceStatus
	PaidAt               *string
	PaymentTransactionID *string
	TotalCents           int64
}

type CardExpenseLeg struct {
	Description       string
	AmountCents       int64
	Date              string
	CategoryID        *string
	Notes             *string
	InstallmentNumber *int
	InstallmentTotal  *int
	InvoicePeriod     domain.InvoicePeriod
}

func (s *Store) ListCards(ctx context.Context, includeArchived bool) ([]CardRow, error) {
	q := `SELECT id, name, brand, limit_cents, closing_day, due_day, color, archived FROM credit_cards`
	var args []any
	if !includeArchived {
		q += ` WHERE archived = ?`
		args = append(args, false)
	}
	q += ` ORDER BY created_at ASC`

	rows, err := s.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CardRow
	for rows.Next() {
		var r CardRow
		var color sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &r.Brand, &r.LimitCents, &r.ClosingDay, &r.DueDay, &color, &r.Archived); err != nil {
			return nil, err
		}
		r.Color = db.NullStringPtr(color)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindCardByID(ctx context.Context, id string) (*CardRow, error) {
	var r CardRow
	var color sql.NullString
	err := s.DB.QueryRow(ctx,
		`SELECT id, name, brand, limit_cents, closing_day, due_day, color, archived FROM credit_cards WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.Brand, &r.LimitCents, &r.ClosingDay, &r.DueDay, &color, &r.Archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Color = db.NullStringPtr(color)
	return &r, nil
}

func (s *Store) CreateCard(ctx context.Context, in domain.CreateCardInput) (*CardRow, error) {
	id := uuid.NewString()
	now := nowStr()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO credit_cards (id, name, brand, limit_cents, closing_day, due_day, color, archived, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, in.Name, string(in.Brand), in.LimitCents, in.ClosingDay, in.DueDay, in.Color, false, now, now)
	if err != nil {
		return nil, err
	}
	return &CardRow{
		ID:         id,
		Name:       in.Name,
		Brand:      in.Brand,
		LimitCents: in.LimitCents,
		ClosingDay: in.ClosingDay,
		DueDay:     in.DueDay,
		Color:      in.Color,
		Archived:   false,
	}, nil
}

func (s *Store) UpdateCard(ctx context.Context, c *CardRow) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE credit_cards
		SET name = ?, brand = ?, limit_cents = ?, closing_day = ?, due_day = ?, color = ?, archived = ?, updated_at = ?
		WHERE id = ?
	`, c.Name, string(c.Brand), c.LimitCents, c.ClosingDay, c.DueDay, c.Color, c.Archived, nowStr(), c.ID)
	return err
}

func (s *Store) CountCardTransactions(ctx context.Context, cardID string) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM transactions WHERE card_id = ?`, cardID).Scan(&n)
	return n, err
}

func (s *Store) DeleteCard(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM credit_cards WHERE id = ?`, id)
	return err
}

func (s *Store) FindInvoiceByID(ctx context.Context, invoiceID string) (*InvoiceRow, error) {
	var r InvoiceRow
	var paidAt db.NullDateText
	var payTx sql.NullString
	err := s.DB.QueryRow(ctx, `
		SELECT id, card_id, month, year, status, paid_at, payment_transaction_id
		FROM invoices WHERE id = ?
	`, invoiceID).Scan(&r.ID, &r.CardID, &r.Month, &r.Year, &r.Status, &paidAt, &payTx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.PaidAt = paidAt.Ptr()
	r.PaymentTransactionID = db.NullStringPtr(payTx)
	return &r, nil
}

func (s *Store) FindInvoicesWithTotals(ctx context.Context, cardID string) ([]InvoiceRow, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT i.id, i.card_id, i.month, i.year, i.status, i.paid_at, i.payment_transaction_id,
		       COALESCE(SUM(t.amount_cents), 0)
		FROM invoices i
		LEFT JOIN transactions t ON t.invoice_id = i.id
		WHERE i.card_id = ?
		GROUP BY i.id, i.card_id, i.month, i.year, i.status, i.paid_at, i.payment_transaction_id
		ORDER BY i.year DESC, i.month DESC
	`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InvoiceRow
	for rows.Next() {
		var r InvoiceRow
		var paidAt db.NullDateText
		var payTx sql.NullString
		var sum int64
		if err := rows.Scan(&r.ID, &r.CardID, &r.Month, &r.Year, &r.Status, &paidAt, &payTx, &sum); err != nil {
			return nil, err
		}
		r.PaidAt = paidAt.Ptr()
		r.PaymentTransactionID = db.NullStringPtr(payTx)
		r.TotalCents = abs64(sum)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindInvoiceItems(ctx context.Context, invoiceID string) ([]domain.Transaction, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT t.id, t.type, t.description, t.amount_cents, t.date, t.status,
		       t.account_id, a.name, t.card_id, cc.name,
		       t.category_id, c.name, c.icon, c.color,
		       t.transfer_group_id, t.installment_number, t.installment_total, t.invoice_id, t.notes, t.recurring_id
		FROM transactions t
		LEFT JOIN accounts a ON a.id = t.account_id
		LEFT JOIN credit_cards cc ON cc.id = t.card_id
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.invoice_id = ?
		ORDER BY t.date ASC, t.created_at ASC
	`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTransactions(rows)
}

func (s *Store) UnpaidCardTotalCents(ctx context.Context, cardID string) (int64, error) {
	var sum int64
	err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN invoices i ON i.id = t.invoice_id
		WHERE i.card_id = ? AND i.status <> 'PAID'
	`, cardID).Scan(&sum)
	return abs64(sum), err
}

func (s *Store) CreateCardExpense(ctx context.Context, cardID string, legs []CardExpenseLeg) (int, error) {
	var groupID *string
	if len(legs) > 1 {
		g := uuid.NewString()
		groupID = &g
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for _, leg := range legs {
		// Upsert atômico: evita a corrida busca-então-insere que estourava o
		// UNIQUE(card_id, year, month) com despesas concorrentes no Postgres.
		var invoiceID string
		err := tx.QueryRow(ctx, `
			INSERT INTO invoices (id, card_id, month, year, status)
			VALUES (?, ?, ?, ?, 'OPEN')
			ON CONFLICT (card_id, year, month) DO UPDATE SET card_id = EXCLUDED.card_id
			RETURNING id
		`, uuid.NewString(), cardID, leg.InvoicePeriod.Month, leg.InvoicePeriod.Year).Scan(&invoiceID)
		if err != nil {
			return 0, err
		}

		now := nowStr()
		_, err = tx.Exec(ctx, `
			INSERT INTO transactions (
				id, type, description, amount_cents, date, status,
				card_id, category_id, invoice_id,
				installment_group_id, installment_number, installment_total, notes,
				created_at, updated_at
			) VALUES (?, 'CARD_EXPENSE', ?, ?, ?, 'CONFIRMED', ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			uuid.NewString(),
			leg.Description,
			-abs64(leg.AmountCents),
			leg.Date,
			cardID,
			leg.CategoryID,
			invoiceID,
			groupID,
			leg.InstallmentNumber,
			leg.InstallmentTotal,
			leg.Notes,
			now,
			now,
		)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(legs), nil
}

func (s *Store) PayInvoice(ctx context.Context, invoiceID, accountID, date, description string, totalCents int64) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	paymentID := uuid.NewString()
	now := nowStr()
	if _, err := tx.Exec(ctx, `
		INSERT INTO transactions (
			id, type, description, amount_cents, date, status, account_id, created_at, updated_at
		) VALUES (?, 'INVOICE_PAYMENT', ?, ?, ?, 'CONFIRMED', ?, ?, ?)
	`, paymentID, description, -abs64(totalCents), date, accountID, now, now); err != nil {
		return err
	}

	res, err := tx.Exec(ctx, `
		UPDATE invoices
		SET status = 'PAID', paid_at = ?, payment_transaction_id = ?
		WHERE id = ? AND status <> 'PAID'
	`, date, paymentID, invoiceID)
	if err != nil {
		return err
	}
	// Nenhuma linha afetada: fatura inexistente ou já paga (inclusive por um
	// pagamento concorrente). O rollback descarta a transação de pagamento.
	if err := expectOneRow(res, ErrConflict); err != nil {
		return err
	}
	return tx.Commit()
}

// ───────────────────────── Transactions ─────────────────────────

// likeEscaper escapa os curingas do LIKE para busca literal (como o `contains` do Prisma).
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func buildTransactionWhere(dialect db.Dialect, q domain.ListTransactionsQuery) (string, []any) {
	var clauses []string
	var args []any

	if q.Year != nil || q.Month != nil {
		year := domain.Now().Year()
		if q.Year != nil {
			year = *q.Year
		}
		var start, end string
		if q.Month != nil {
			start, end = domain.MonthRange(year, *q.Month)
		} else {
			start, end = domain.YearRange(year)
		}
		clauses = append(clauses, "t.date >= ? AND t.date < ?")
		args = append(args, start, end)
	}
	if q.AccountID != nil {
		clauses = append(clauses, "t.account_id = ?")
		args = append(args, *q.AccountID)
	}
	if q.CardID != nil {
		clauses = append(clauses, "t.card_id = ?")
		args = append(args, *q.CardID)
	}
	if q.CategoryID != nil {
		clauses = append(clauses, "t.category_id = ?")
		args = append(args, *q.CategoryID)
	}
	if q.Type != nil {
		clauses = append(clauses, "t.type = ?")
		args = append(args, string(*q.Type))
	}
	if q.Status != nil {
		clauses = append(clauses, "t.status = ?")
		args = append(args, string(*q.Status))
	}
	if q.Search != nil && *q.Search != "" {
		pattern := "%" + likeEscaper.Replace(*q.Search) + "%"
		if dialect == db.DialectPostgres {
			clauses = append(clauses, `t.description ILIKE ? ESCAPE '\'`)
		} else {
			clauses = append(clauses, db.FoldFunc+`(t.description) LIKE `+db.FoldFunc+`(?) ESCAPE '\'`)
		}
		args = append(args, pattern)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

const txSelectCols = `
	SELECT t.id, t.type, t.description, t.amount_cents, t.date, t.status,
	       t.account_id, a.name, t.card_id, cc.name,
	       t.category_id, c.name, c.icon, c.color,
	       t.transfer_group_id, t.installment_number, t.installment_total, t.invoice_id, t.notes, t.recurring_id
	FROM transactions t
	LEFT JOIN accounts a ON a.id = t.account_id
	LEFT JOIN credit_cards cc ON cc.id = t.card_id
	LEFT JOIN categories c ON c.id = t.category_id
`

func scanTransaction(scanner interface{ Scan(...any) error }) (domain.Transaction, error) {
	var (
		t            domain.Transaction
		dt           db.DateText
		accountID    sql.NullString
		accountName  sql.NullString
		cardID       sql.NullString
		cardName     sql.NullString
		categoryID   sql.NullString
		categoryName sql.NullString
		categoryIcon sql.NullString
		categoryCol  sql.NullString
		transferGID  sql.NullString
		instNum      sql.NullInt64
		instTot      sql.NullInt64
		invoiceID    sql.NullString
		notes        sql.NullString
		recurringID  sql.NullString
	)
	err := scanner.Scan(
		&t.ID, &t.Type, &t.Description, &t.AmountCents, &dt, &t.Status,
		&accountID, &accountName, &cardID, &cardName,
		&categoryID, &categoryName, &categoryIcon, &categoryCol,
		&transferGID, &instNum, &instTot, &invoiceID, &notes, &recurringID,
	)
	if err != nil {
		return t, err
	}
	t.Date = string(dt)
	t.AccountID = db.NullStringPtr(accountID)
	t.AccountName = db.NullStringPtr(accountName)
	t.CardID = db.NullStringPtr(cardID)
	t.CardName = db.NullStringPtr(cardName)
	t.CategoryID = db.NullStringPtr(categoryID)
	t.CategoryName = db.NullStringPtr(categoryName)
	t.CategoryIcon = db.NullStringPtr(categoryIcon)
	t.CategoryColor = db.NullStringPtr(categoryCol)
	t.TransferGroupID = db.NullStringPtr(transferGID)
	t.InstallmentNumber = db.NullIntPtr(instNum)
	t.InstallmentTotal = db.NullIntPtr(instTot)
	t.InvoiceID = db.NullStringPtr(invoiceID)
	t.Notes = db.NullStringPtr(notes)
	t.RecurringID = db.NullStringPtr(recurringID)
	return t, nil
}

func scanTransactions(rows *sql.Rows) ([]domain.Transaction, error) {
	out := make([]domain.Transaction, 0)
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ListTransactions(ctx context.Context, q domain.ListTransactionsQuery) ([]domain.Transaction, int, error) {
	where, args := buildTransactionWhere(s.DB.Dialect, q)

	var total int
	if err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM transactions t`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (q.Page - 1) * q.PageSize
	listArgs := append(append([]any{}, args...), q.PageSize, offset)
	rows, err := s.DB.Query(ctx, txSelectCols+where+` ORDER BY t.date DESC, t.created_at DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := scanTransactions(rows)
	return items, total, err
}

func (s *Store) ListAllTransactions(ctx context.Context, q domain.ListTransactionsQuery) ([]domain.Transaction, error) {
	where, args := buildTransactionWhere(s.DB.Dialect, q)
	rows, err := s.DB.Query(ctx, txSelectCols+where+` ORDER BY t.date ASC, t.created_at ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTransactions(rows)
}

type TransactionDetail struct {
	domain.Transaction
	InstallmentGroupID *string
}

func (s *Store) FindTransactionByID(ctx context.Context, id string) (*TransactionDetail, error) {
	var (
		t            domain.Transaction
		dt           db.DateText
		accountID    sql.NullString
		accountName  sql.NullString
		cardID       sql.NullString
		cardName     sql.NullString
		categoryID   sql.NullString
		categoryName sql.NullString
		categoryIcon sql.NullString
		categoryCol  sql.NullString
		transferGID  sql.NullString
		instGID      sql.NullString
		instNum      sql.NullInt64
		instTot      sql.NullInt64
		invoiceID    sql.NullString
		notes        sql.NullString
		recurringID  sql.NullString
	)
	err := s.DB.QueryRow(ctx, `
		SELECT t.id, t.type, t.description, t.amount_cents, t.date, t.status,
		       t.account_id, a.name, t.card_id, cc.name,
		       t.category_id, c.name, c.icon, c.color,
		       t.transfer_group_id, t.installment_group_id, t.installment_number, t.installment_total, t.invoice_id, t.notes, t.recurring_id
		FROM transactions t
		LEFT JOIN accounts a ON a.id = t.account_id
		LEFT JOIN credit_cards cc ON cc.id = t.card_id
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.id = ?
	`, id).Scan(
		&t.ID, &t.Type, &t.Description, &t.AmountCents, &dt, &t.Status,
		&accountID, &accountName, &cardID, &cardName,
		&categoryID, &categoryName, &categoryIcon, &categoryCol,
		&transferGID, &instGID, &instNum, &instTot, &invoiceID, &notes, &recurringID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Date = string(dt)
	t.AccountID = db.NullStringPtr(accountID)
	t.AccountName = db.NullStringPtr(accountName)
	t.CardID = db.NullStringPtr(cardID)
	t.CardName = db.NullStringPtr(cardName)
	t.CategoryID = db.NullStringPtr(categoryID)
	t.CategoryName = db.NullStringPtr(categoryName)
	t.CategoryIcon = db.NullStringPtr(categoryIcon)
	t.CategoryColor = db.NullStringPtr(categoryCol)
	t.TransferGroupID = db.NullStringPtr(transferGID)
	t.InstallmentNumber = db.NullIntPtr(instNum)
	t.InstallmentTotal = db.NullIntPtr(instTot)
	t.InvoiceID = db.NullStringPtr(invoiceID)
	t.Notes = db.NullStringPtr(notes)
	t.RecurringID = db.NullStringPtr(recurringID)
	return &TransactionDetail{
		Transaction:        t,
		InstallmentGroupID: db.NullStringPtr(instGID),
	}, nil
}

func (s *Store) CreateTransaction(ctx context.Context, txType domain.TransactionType, description string, amountCents int64, date string, status domain.TransactionStatus, accountID string, categoryID, notes *string) (*domain.Transaction, error) {
	id := uuid.NewString()
	now := nowStr()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO transactions (
			id, type, description, amount_cents, date, status, account_id, category_id, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, string(txType), description, amountCents, date, string(status), accountID, categoryID, notes, now, now)
	if err != nil {
		return nil, err
	}
	detail, err := s.FindTransactionByID(ctx, id)
	if err != nil || detail == nil {
		return nil, err
	}
	return &detail.Transaction, nil
}

func (s *Store) UpdateTransaction(ctx context.Context, t *domain.Transaction) (*domain.Transaction, error) {
	_, err := s.DB.Exec(ctx, `
		UPDATE transactions
		SET description = ?, amount_cents = ?, date = ?, status = ?, account_id = ?, category_id = ?, notes = ?, updated_at = ?
		WHERE id = ?
	`, t.Description, t.AmountCents, t.Date, string(t.Status), t.AccountID, t.CategoryID, t.Notes, nowStr(), t.ID)
	if err != nil {
		return nil, err
	}
	detail, err := s.FindTransactionByID(ctx, t.ID)
	if err != nil || detail == nil {
		return nil, err
	}
	return &detail.Transaction, nil
}

func (s *Store) CreateTransferPair(ctx context.Context, fromAccountID, toAccountID string, amountCents int64, date, fromDesc, toDesc string) (string, error) {
	groupID := uuid.NewString()
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	now := nowStr()
	if _, err := tx.Exec(ctx, `
		INSERT INTO transactions (
			id, type, description, amount_cents, date, status, account_id, transfer_group_id, created_at, updated_at
		) VALUES (?, 'TRANSFER', ?, ?, ?, 'CONFIRMED', ?, ?, ?, ?)
	`, uuid.NewString(), fromDesc, -abs64(amountCents), date, fromAccountID, groupID, now, now); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO transactions (
			id, type, description, amount_cents, date, status, account_id, transfer_group_id, created_at, updated_at
		) VALUES (?, 'TRANSFER', ?, ?, ?, 'CONFIRMED', ?, ?, ?, ?)
	`, uuid.NewString(), toDesc, abs64(amountCents), date, toAccountID, groupID, now, now); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return groupID, nil
}

func (s *Store) DeleteTransferGroup(ctx context.Context, transferGroupID string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM transactions WHERE transfer_group_id = ?`, transferGroupID)
	return err
}

func (s *Store) DeleteInstallmentGroup(ctx context.Context, installmentGroupID string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM transactions WHERE installment_group_id = ?`, installmentGroupID)
	return err
}

func (s *Store) FindInvoicePaidBy(ctx context.Context, transactionID string) (string, error) {
	var invoiceID string
	err := s.DB.QueryRow(ctx, `SELECT id FROM invoices WHERE payment_transaction_id = ?`, transactionID).Scan(&invoiceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return invoiceID, err
}

func (s *Store) DeleteInvoicePayment(ctx context.Context, transactionID, invoiceID string) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(ctx, `
		UPDATE invoices SET status = 'OPEN', paid_at = NULL, payment_transaction_id = NULL
		WHERE id = ? AND payment_transaction_id = ?
	`, invoiceID, transactionID)
	if err != nil {
		return err
	}
	if err := expectOneRow(res, ErrNotFound); err != nil {
		return err
	}
	res, err = tx.Exec(ctx, `DELETE FROM transactions WHERE id = ?`, transactionID)
	if err != nil {
		return err
	}
	if err := expectOneRow(res, ErrNotFound); err != nil {
		return err
	}
	return tx.Commit()
}

// expectOneRow devolve sentinel quando o comando não afetou exatamente uma linha.
func expectOneRow(res sql.Result, sentinel error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sentinel
	}
	return nil
}

func (s *Store) DeleteTransaction(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM transactions WHERE id = ?`, id)
	return err
}

// ───────────────────────── Budgets ─────────────────────────

type BudgetRow struct {
	ID            string
	CategoryID    string
	CategoryName  string
	CategoryIcon  string
	CategoryColor string
	AmountCents   int64
}

func (s *Store) ListBudgets(ctx context.Context) ([]BudgetRow, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT b.id, b.category_id, c.name, c.icon, c.color, b.amount_cents
		FROM budgets b
		JOIN categories c ON c.id = b.category_id
		ORDER BY c.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BudgetRow
	for rows.Next() {
		var r BudgetRow
		if err := rows.Scan(&r.ID, &r.CategoryID, &r.CategoryName, &r.CategoryIcon, &r.CategoryColor, &r.AmountCents); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindBudgetByID(ctx context.Context, id string) (*BudgetRow, error) {
	var r BudgetRow
	err := s.DB.QueryRow(ctx, `
		SELECT b.id, b.category_id, c.name, c.icon, c.color, b.amount_cents
		FROM budgets b
		JOIN categories c ON c.id = b.category_id
		WHERE b.id = ?
	`, id).Scan(&r.ID, &r.CategoryID, &r.CategoryName, &r.CategoryIcon, &r.CategoryColor, &r.AmountCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) UpsertBudget(ctx context.Context, categoryID string, amountCents int64) (*BudgetRow, error) {
	now := nowStr()
	id := uuid.NewString()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO budgets (id, category_id, amount_cents, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (category_id) DO UPDATE SET amount_cents = EXCLUDED.amount_cents, updated_at = EXCLUDED.updated_at
	`, id, categoryID, amountCents, now, now)
	if err != nil {
		return nil, err
	}

	var r BudgetRow
	err = s.DB.QueryRow(ctx, `
		SELECT b.id, b.category_id, c.name, c.icon, c.color, b.amount_cents
		FROM budgets b
		JOIN categories c ON c.id = b.category_id
		WHERE b.category_id = ?
	`, categoryID).Scan(&r.ID, &r.CategoryID, &r.CategoryName, &r.CategoryIcon, &r.CategoryColor, &r.AmountCents)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) DeleteBudget(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM budgets WHERE id = ?`, id)
	return err
}

func (s *Store) SpentByCategory(ctx context.Context, year, month int) (map[string]int64, error) {
	start, end := domain.MonthRange(year, month)
	rows, err := s.DB.Query(ctx, `
		SELECT category_id, COALESCE(SUM(amount_cents), 0)
		FROM transactions
		WHERE type IN ('EXPENSE', 'CARD_EXPENSE')
		  AND category_id IS NOT NULL
		  AND date >= ? AND date < ?
		GROUP BY category_id
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]int64)
	for rows.Next() {
		var catID string
		var sum int64
		if err := rows.Scan(&catID, &sum); err != nil {
			return nil, err
		}
		m[catID] = abs64(sum)
	}
	return m, rows.Err()
}

type BudgetSnapshot struct {
	CategoryName string
	BudgetCents  int64
	SpentCents   int64
}

func (s *Store) BudgetSnapshot(ctx context.Context, categoryID string, year, month int) (*BudgetSnapshot, error) {
	var snap BudgetSnapshot
	err := s.DB.QueryRow(ctx, `
		SELECT c.name, b.amount_cents
		FROM budgets b
		JOIN categories c ON c.id = b.category_id
		WHERE b.category_id = ?
	`, categoryID).Scan(&snap.CategoryName, &snap.BudgetCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	start, end := domain.MonthRange(year, month)
	var sum int64
	err = s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM transactions
		WHERE category_id = ? AND type IN ('EXPENSE', 'CARD_EXPENSE') AND date >= ? AND date < ?
	`, categoryID, start, end).Scan(&sum)
	if err != nil {
		return nil, err
	}
	snap.SpentCents = abs64(sum)
	return &snap, nil
}

// ───────────────────────── Goals ─────────────────────────

type GoalRow struct {
	ID          string
	Name        string
	Kind        domain.GoalKind
	Icon        string
	Color       *string
	TargetCents int64
	SavedCents  int64
	Deadline    *string
}

func (s *Store) ListGoals(ctx context.Context) ([]GoalRow, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT g.id, g.name, g.kind, g.icon, g.color, g.target_cents, g.deadline,
		       COALESCE(SUM(c.amount_cents), 0)
		FROM goals g
		LEFT JOIN goal_contributions c ON c.goal_id = g.id
		WHERE g.archived = ?
		GROUP BY g.id, g.name, g.kind, g.icon, g.color, g.target_cents, g.deadline, g.created_at
		ORDER BY g.created_at ASC
	`, false)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GoalRow
	for rows.Next() {
		var r GoalRow
		var color sql.NullString
		var deadline db.NullDateText
		if err := rows.Scan(&r.ID, &r.Name, &r.Kind, &r.Icon, &color, &r.TargetCents, &deadline, &r.SavedCents); err != nil {
			return nil, err
		}
		r.Color = db.NullStringPtr(color)
		r.Deadline = deadline.Ptr()
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindGoalByID(ctx context.Context, id string) (*GoalRow, error) {
	var r GoalRow
	var color sql.NullString
	var deadline db.NullDateText
	err := s.DB.QueryRow(ctx, `
		SELECT g.id, g.name, g.kind, g.icon, g.color, g.target_cents, g.deadline,
		       COALESCE(SUM(c.amount_cents), 0)
		FROM goals g
		LEFT JOIN goal_contributions c ON c.goal_id = g.id
		WHERE g.id = ?
		GROUP BY g.id, g.name, g.kind, g.icon, g.color, g.target_cents, g.deadline
	`, id).Scan(&r.ID, &r.Name, &r.Kind, &r.Icon, &color, &r.TargetCents, &deadline, &r.SavedCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Color = db.NullStringPtr(color)
	r.Deadline = deadline.Ptr()
	return &r, nil
}

func (s *Store) CreateGoal(ctx context.Context, in domain.CreateGoalInput) (*GoalRow, error) {
	id := uuid.NewString()
	now := nowStr()
	kind := in.Kind
	if kind == "" {
		kind = domain.GoalKindGoal
	}
	_, err := s.DB.Exec(ctx, `
		INSERT INTO goals (id, name, kind, icon, color, target_cents, deadline, archived, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, in.Name, string(kind), in.Icon, in.Color, in.TargetCents, in.Deadline, false, now, now)
	if err != nil {
		return nil, err
	}
	return &GoalRow{
		ID:          id,
		Name:        in.Name,
		Kind:        kind,
		Icon:        in.Icon,
		Color:       in.Color,
		TargetCents: in.TargetCents,
		SavedCents:  0,
		Deadline:    in.Deadline,
	}, nil
}

func (s *Store) UpdateGoal(ctx context.Context, g *GoalRow) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE goals SET name = ?, icon = ?, color = ?, target_cents = ?, deadline = ?, updated_at = ?
		WHERE id = ?
	`, g.Name, g.Icon, g.Color, g.TargetCents, g.Deadline, nowStr(), g.ID)
	return err
}

// FindActiveReserveGoal devolve a meta RESERVE não arquivada (no máximo uma), ou nil.
func (s *Store) FindActiveReserveGoal(ctx context.Context) (*GoalRow, error) {
	var id string
	err := s.DB.QueryRow(ctx, `SELECT id FROM goals WHERE kind = 'RESERVE' AND archived = ?`, false).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.FindGoalByID(ctx, id)
}

func (s *Store) DeleteGoal(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM goals WHERE id = ?`, id)
	return err
}

func (s *Store) AddGoalContribution(ctx context.Context, goalID string, amountCents int64, date string, note *string) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO goal_contributions (id, goal_id, amount_cents, date, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), goalID, amountCents, date, note, nowStr())
	return err
}

// ───────────────────────── Dashboard ─────────────────────────

func (s *Store) TotalBalanceCents(ctx context.Context) (int64, error) {
	var initSum, txSum int64
	if err := s.DB.QueryRow(ctx, `SELECT COALESCE(SUM(initial_balance_cents), 0) FROM accounts WHERE archived = ?`, false).Scan(&initSum); err != nil {
		return 0, err
	}
	if err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.status = 'CONFIRMED' AND a.archived = ?
	`, false).Scan(&txSum); err != nil {
		return 0, err
	}
	return initSum + txSum, nil
}

func (s *Store) MonthIncomeCents(ctx context.Context, year, month int) (int64, error) {
	start, end := domain.MonthRange(year, month)
	var sum int64
	err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM transactions
		WHERE type = 'INCOME' AND status = 'CONFIRMED' AND date >= ? AND date < ?
	`, start, end).Scan(&sum)
	return sum, err
}

func (s *Store) MonthExpenseCents(ctx context.Context, year, month int) (int64, error) {
	start, end := domain.MonthRange(year, month)
	var sum int64
	err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM transactions
		WHERE type IN ('EXPENSE', 'CARD_EXPENSE') AND status = 'CONFIRMED' AND date >= ? AND date < ?
	`, start, end).Scan(&sum)
	return abs64(sum), err
}

func (s *Store) OpenInvoicesCents(ctx context.Context) (int64, error) {
	var sum int64
	err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN invoices i ON i.id = t.invoice_id
		WHERE i.status <> 'PAID'
	`).Scan(&sum)
	return abs64(sum), err
}

func (s *Store) PendingCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM transactions WHERE status = 'PENDING'`).Scan(&n)
	return n, err
}

type MovementRow struct {
	Date        string
	AmountCents int64
	Type        domain.TransactionType
}

func (s *Store) MonthMovements(ctx context.Context, year, month int) ([]MovementRow, error) {
	start, end := domain.MonthRange(year, month)
	rows, err := s.DB.Query(ctx, `
		SELECT date, amount_cents, type
		FROM transactions
		WHERE type IN ('INCOME', 'EXPENSE', 'CARD_EXPENSE') AND status = 'CONFIRMED' AND date >= ? AND date < ?
		ORDER BY date ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MovementRow
	for rows.Next() {
		var r MovementRow
		var dt db.DateText
		if err := rows.Scan(&dt, &r.AmountCents, &r.Type); err != nil {
			return nil, err
		}
		r.Date = string(dt)
		out = append(out, r)
	}
	return out, rows.Err()
}

type CategoryExpenseGroup struct {
	CategoryID *string
	Name       *string
	Icon       *string
	Color      *string
	TotalCents int64
}

func (s *Store) ExpensesByCategory(ctx context.Context, year, month int) ([]CategoryExpenseGroup, error) {
	start, end := domain.MonthRange(year, month)
	rows, err := s.DB.Query(ctx, `
		SELECT t.category_id, c.name, c.icon, c.color, COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.type IN ('EXPENSE', 'CARD_EXPENSE') AND t.status = 'CONFIRMED' AND t.date >= ? AND t.date < ?
		GROUP BY t.category_id, c.name, c.icon, c.color
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CategoryExpenseGroup
	for rows.Next() {
		var (
			catID sql.NullString
			name  sql.NullString
			icon  sql.NullString
			color sql.NullString
			sum   int64
		)
		if err := rows.Scan(&catID, &name, &icon, &color, &sum); err != nil {
			return nil, err
		}
		out = append(out, CategoryExpenseGroup{
			CategoryID: db.NullStringPtr(catID),
			Name:       db.NullStringPtr(name),
			Icon:       db.NullStringPtr(icon),
			Color:      db.NullStringPtr(color),
			TotalCents: abs64(sum),
		})
	}
	return out, rows.Err()
}

type PendingTxRow struct {
	ID           string
	Description  string
	Date         string
	AmountCents  int64
	CategoryIcon *string
}

func (s *Store) PendingBetween(ctx context.Context, start, end string) ([]PendingTxRow, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT t.id, t.description, t.date, t.amount_cents, c.icon
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.status = 'PENDING' AND t.date >= ? AND t.date <= ?
		ORDER BY t.date ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PendingTxRow
	for rows.Next() {
		var r PendingTxRow
		var dt db.DateText
		var icon sql.NullString
		if err := rows.Scan(&r.ID, &r.Description, &dt, &r.AmountCents, &icon); err != nil {
			return nil, err
		}
		r.Date = string(dt)
		r.CategoryIcon = db.NullStringPtr(icon)
		out = append(out, r)
	}
	return out, rows.Err()
}

type UnpaidInvoiceWithCard struct {
	ID         string
	Year       int
	Month      int
	CardName   string
	ClosingDay int
	DueDay     int
	TotalCents int64
}

func (s *Store) UnpaidInvoicesWithTotals(ctx context.Context) ([]UnpaidInvoiceWithCard, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT i.id, i.year, i.month, cc.name, cc.closing_day, cc.due_day,
		       COALESCE(SUM(t.amount_cents), 0)
		FROM invoices i
		JOIN credit_cards cc ON cc.id = i.card_id
		LEFT JOIN transactions t ON t.invoice_id = i.id
		WHERE i.status <> 'PAID'
		GROUP BY i.id, i.year, i.month, cc.name, cc.closing_day, cc.due_day
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UnpaidInvoiceWithCard
	for rows.Next() {
		var r UnpaidInvoiceWithCard
		var sum int64
		if err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.CardName, &r.ClosingDay, &r.DueDay, &sum); err != nil {
			return nil, err
		}
		r.TotalCents = abs64(sum)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClearAll apaga todas as tabelas respeitando FKs (usado em testes de integração).
func (s *Store) ClearAll(ctx context.Context) error {
	tables := []string{
		"transactions",
		"recurring",
		"category_rules",
		"wishlist",
		"settings",
		"invoices",
		"goal_contributions",
		"goals",
		"budgets",
		"credit_cards",
		"accounts",
		"categories",
	}
	for _, tbl := range tables {
		if _, err := s.DB.Exec(ctx, "DELETE FROM "+tbl); err != nil {
			return err
		}
	}
	return nil
}
