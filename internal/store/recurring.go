package store

import (
	"context"
	"database/sql"
	"errors"

	"finflow/internal/db"
	"finflow/internal/domain"

	"github.com/google/uuid"
)

// ───────────────────────── Recurring ─────────────────────────

type RecurringRow struct {
	ID            string
	Description   string
	Kind          domain.CategoryKind
	AmountCents   int64
	DayOfMonth    int
	AccountID     *string
	AccountName   *string
	CategoryID    *string
	CategoryName  *string
	CategoryIcon  *string
	CategoryColor *string
	StartDate     string
	EndDate       *string
	Active        bool
}

const recurringSelect = `
	SELECT r.id, r.description, r.kind, r.amount_cents, r.day_of_month,
	       r.account_id, a.name, r.category_id, c.name, c.icon, c.color,
	       r.start_date, r.end_date, r.active
	FROM recurring r
	LEFT JOIN accounts a ON a.id = r.account_id
	LEFT JOIN categories c ON c.id = r.category_id
`

func scanRecurring(scanner interface{ Scan(...any) error }) (RecurringRow, error) {
	var (
		r                                  RecurringRow
		accountID, accountName, categoryID sql.NullString
		categoryName, categoryIcon, catCol sql.NullString
		start                              db.DateText
		end                                db.NullDateText
	)
	err := scanner.Scan(&r.ID, &r.Description, &r.Kind, &r.AmountCents, &r.DayOfMonth,
		&accountID, &accountName, &categoryID, &categoryName, &categoryIcon, &catCol,
		&start, &end, &r.Active)
	if err != nil {
		return r, err
	}
	r.AccountID = db.NullStringPtr(accountID)
	r.AccountName = db.NullStringPtr(accountName)
	r.CategoryID = db.NullStringPtr(categoryID)
	r.CategoryName = db.NullStringPtr(categoryName)
	r.CategoryIcon = db.NullStringPtr(categoryIcon)
	r.CategoryColor = db.NullStringPtr(catCol)
	r.StartDate = string(start)
	r.EndDate = end.Ptr()
	return r, nil
}

func (s *Store) ListRecurring(ctx context.Context) ([]RecurringRow, error) {
	rows, err := s.DB.Query(ctx, recurringSelect+` ORDER BY r.active DESC, r.day_of_month ASC, r.description ASC, r.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecurringRow
	for rows.Next() {
		r, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindRecurringByID(ctx context.Context, id string) (*RecurringRow, error) {
	r, err := scanRecurring(s.DB.QueryRow(ctx, recurringSelect+` WHERE r.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) CreateRecurring(ctx context.Context, r RecurringRow) (string, error) {
	id := uuid.NewString()
	now := nowStr()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO recurring (
			id, description, kind, amount_cents, day_of_month, account_id, category_id,
			start_date, end_date, active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, r.Description, string(r.Kind), r.AmountCents, r.DayOfMonth, r.AccountID, r.CategoryID,
		r.StartDate, r.EndDate, r.Active, now, now)
	return id, err
}

func (s *Store) UpdateRecurring(ctx context.Context, r *RecurringRow) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE recurring
		SET description = ?, kind = ?, amount_cents = ?, day_of_month = ?, account_id = ?, category_id = ?,
		    start_date = ?, end_date = ?, active = ?, updated_at = ?
		WHERE id = ?
	`, r.Description, string(r.Kind), r.AmountCents, r.DayOfMonth, r.AccountID, r.CategoryID,
		r.StartDate, r.EndDate, r.Active, nowStr(), r.ID)
	return err
}

func (s *Store) DeleteRecurring(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM recurring WHERE id = ?`, id)
	return err
}

// RealizedRecurringMonths devolve, por recorrência, os meses ("YYYY-MM") que já têm
// transação lançada com o recurring_id.
func (s *Store) RealizedRecurringMonths(ctx context.Context) (map[string]map[string]bool, error) {
	rows, err := s.DB.Query(ctx, `SELECT recurring_id, date FROM transactions WHERE recurring_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]map[string]bool)
	for rows.Next() {
		var id string
		var dt db.DateText
		if err := rows.Scan(&id, &dt); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = make(map[string]bool)
		}
		out[id][domain.MonthKey(string(dt))] = true
	}
	return out, rows.Err()
}

// CreateRecurringTransaction lança uma ocorrência (INCOME/EXPENSE confirmada) ligada à recorrência.
func (s *Store) CreateRecurringTransaction(ctx context.Context, recurringID string, txType domain.TransactionType, description string, amountCents int64, date, accountID string, categoryID *string) (*domain.Transaction, error) {
	id := uuid.NewString()
	now := nowStr()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO transactions (
			id, type, description, amount_cents, date, status, account_id, category_id, recurring_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'CONFIRMED', ?, ?, ?, ?, ?)
	`, id, string(txType), description, amountCents, date, accountID, categoryID, recurringID, now, now)
	if err != nil {
		return nil, err
	}
	detail, err := s.FindTransactionByID(ctx, id)
	if err != nil || detail == nil {
		return nil, err
	}
	return &detail.Transaction, nil
}
