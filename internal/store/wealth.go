package store

import (
	"context"
	"database/sql"
	"errors"

	"finflow/internal/db"
	"finflow/internal/domain"

	"github.com/google/uuid"
)

// ───────────────────────── Investimentos ─────────────────────────

type InvestmentRow struct {
	ID             string
	Name           string
	Kind           domain.InvestmentKind
	Institution    *string
	AccountID      *string
	AccountName    *string
	InvestedCents  int64
	CurrentCents   int64
	Notes          *string
	Archived       bool
	ValueUpdatedAt *string
}

const investmentSelect = `
	SELECT i.id, i.name, i.kind, i.institution, i.account_id, a.name,
	       i.invested_cents, i.current_cents, i.notes, i.archived, i.value_updated_at
	FROM investments i
	LEFT JOIN accounts a ON a.id = i.account_id
`

func scanInvestment(scanner interface{ Scan(...any) error }) (InvestmentRow, error) {
	var (
		r                                         InvestmentRow
		institution, accountID, accountName, note sql.NullString
		updated                                   db.NullDateText
	)
	err := scanner.Scan(&r.ID, &r.Name, &r.Kind, &institution, &accountID, &accountName,
		&r.InvestedCents, &r.CurrentCents, &note, &r.Archived, &updated)
	if err != nil {
		return r, err
	}
	r.Institution = db.NullStringPtr(institution)
	r.AccountID = db.NullStringPtr(accountID)
	r.AccountName = db.NullStringPtr(accountName)
	r.Notes = db.NullStringPtr(note)
	r.ValueUpdatedAt = updated.Ptr()
	return r, nil
}

func (s *Store) ListInvestments(ctx context.Context, includeArchived bool) ([]InvestmentRow, error) {
	query := investmentSelect
	var args []any
	if !includeArchived {
		query += ` WHERE i.archived = ?`
		args = append(args, false)
	}
	rows, err := s.DB.Query(ctx, query+` ORDER BY i.archived ASC, i.current_cents DESC, i.name ASC, i.created_at ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InvestmentRow
	for rows.Next() {
		r, err := scanInvestment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindInvestmentByID(ctx context.Context, id string) (*InvestmentRow, error) {
	r, err := scanInvestment(s.DB.QueryRow(ctx, investmentSelect+` WHERE i.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// TransferSpec descreve as duas pernas de uma transferência entre contas próprias.
type TransferSpec struct {
	FromAccountID, ToAccountID string
	AmountCents                int64
	Date                       string
	FromDesc, ToDesc           string
}

// execer é o mínimo comum a *db.DB e *db.Tx usado pelos helpers abaixo.
type execer interface {
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertTransferPair(ctx context.Context, x execer, t TransferSpec) error {
	groupID := uuid.NewString()
	now := nowStr()
	for _, leg := range []struct {
		account, desc string
		amount        int64
	}{
		{t.FromAccountID, t.FromDesc, -abs64(t.AmountCents)},
		{t.ToAccountID, t.ToDesc, abs64(t.AmountCents)},
	} {
		if _, err := x.Exec(ctx, `
			INSERT INTO transactions (
				id, type, description, amount_cents, date, status, account_id, transfer_group_id, created_at, updated_at
			) VALUES (?, 'TRANSFER', ?, ?, ?, 'CONFIRMED', ?, ?, ?, ?)
		`, uuid.NewString(), leg.desc, leg.amount, t.Date, leg.account, groupID, now, now); err != nil {
			return err
		}
	}
	return nil
}

// upsertSnapshot grava o valor da posição na data, substituindo um snapshot do mesmo dia.
func upsertSnapshot(ctx context.Context, x execer, investmentID string, snap domain.InvestmentSnapshot) error {
	if _, err := x.Exec(ctx, `DELETE FROM investment_snapshots WHERE investment_id = ? AND date = ?`, investmentID, snap.Date); err != nil {
		return err
	}
	_, err := x.Exec(ctx, `
		INSERT INTO investment_snapshots (id, investment_id, date, value_cents, invested_cents)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.NewString(), investmentID, snap.Date, snap.ValueCents, snap.InvestedCents)
	return err
}

// CreateInvestment cria a posição e, se snap != nil, o snapshot inicial (numa transação).
func (s *Store) CreateInvestment(ctx context.Context, r InvestmentRow, snap *domain.InvestmentSnapshot) (string, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	id := uuid.NewString()
	now := nowStr()
	if _, err := tx.Exec(ctx, `
		INSERT INTO investments (
			id, account_id, name, kind, institution, invested_cents, current_cents, notes, archived,
			value_updated_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, r.AccountID, r.Name, string(r.Kind), r.Institution, r.InvestedCents, r.CurrentCents, r.Notes, r.Archived,
		r.ValueUpdatedAt, now, now); err != nil {
		return "", err
	}
	if snap != nil {
		if err := upsertSnapshot(ctx, tx, id, *snap); err != nil {
			return "", err
		}
	}
	return id, tx.Commit()
}

// SaveInvestment grava a posição inteira e, opcionalmente, um snapshot e uma transferência
// (aporte/resgate com movimentação de conta) — tudo numa transação de banco.
func (s *Store) SaveInvestment(ctx context.Context, r *InvestmentRow, snap *domain.InvestmentSnapshot, transfer *TransferSpec) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(ctx, `
		UPDATE investments
		SET account_id = ?, name = ?, kind = ?, institution = ?, invested_cents = ?, current_cents = ?,
		    notes = ?, archived = ?, value_updated_at = ?, updated_at = ?
		WHERE id = ?
	`, r.AccountID, r.Name, string(r.Kind), r.Institution, r.InvestedCents, r.CurrentCents,
		r.Notes, r.Archived, r.ValueUpdatedAt, nowStr(), r.ID)
	if err != nil {
		return err
	}
	if err := expectOneRow(res, ErrNotFound); err != nil {
		return err
	}
	if snap != nil {
		if err := upsertSnapshot(ctx, tx, r.ID, *snap); err != nil {
			return err
		}
	}
	if transfer != nil {
		if err := insertTransferPair(ctx, tx, *transfer); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteInvestment(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM investments WHERE id = ?`, id)
	return err
}

// InvestmentSnapshots devolve os snapshots por posição, ordenados por data.
func (s *Store) InvestmentSnapshots(ctx context.Context) (map[string][]domain.InvestmentSnapshot, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT investment_id, date, value_cents, invested_cents
		FROM investment_snapshots
		ORDER BY investment_id, date
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]domain.InvestmentSnapshot)
	for rows.Next() {
		var id string
		var dt db.DateText
		var snap domain.InvestmentSnapshot
		if err := rows.Scan(&id, &dt, &snap.ValueCents, &snap.InvestedCents); err != nil {
			return nil, err
		}
		snap.Date = string(dt)
		out[id] = append(out[id], snap)
	}
	return out, rows.Err()
}

// ───────────────────────── Empréstimos ─────────────────────────

// LoanRow espelha a tabela loans. BasePaid é a coluna paid_installments: parcelas pagas fora
// do FinFlow (informadas no cadastro/edição). As parcelas pagas pelo app são as transações com
// loan_id, então "pagas" = BasePaid + pagamentos lançados (excluir um pagamento devolve a parcela).
type LoanRow struct {
	ID                string
	Name              string
	Lender            *string
	Kind              domain.LoanKind
	PrincipalCents    int64
	RateBP            int
	InstallmentCents  int64
	TotalInstallments int
	BasePaid          int
	FirstDueDate      string
	AccountID         *string
	CategoryID        *string
	Active            bool
	Notes             *string
}

const loanSelect = `
	SELECT id, name, lender, kind, principal_cents, monthly_rate_bp, installment_cents,
	       total_installments, paid_installments, first_due_date, account_id, category_id, active, notes
	FROM loans
`

func scanLoan(scanner interface{ Scan(...any) error }) (LoanRow, error) {
	var (
		r                                    LoanRow
		lender, accountID, categoryID, notes sql.NullString
		first                                db.DateText
	)
	err := scanner.Scan(&r.ID, &r.Name, &lender, &r.Kind, &r.PrincipalCents, &r.RateBP, &r.InstallmentCents,
		&r.TotalInstallments, &r.BasePaid, &first, &accountID, &categoryID, &r.Active, &notes)
	if err != nil {
		return r, err
	}
	r.Lender = db.NullStringPtr(lender)
	r.AccountID = db.NullStringPtr(accountID)
	r.CategoryID = db.NullStringPtr(categoryID)
	r.Notes = db.NullStringPtr(notes)
	r.FirstDueDate = string(first)
	return r, nil
}

func (s *Store) ListLoans(ctx context.Context) ([]LoanRow, error) {
	rows, err := s.DB.Query(ctx, loanSelect+` ORDER BY active DESC, first_due_date ASC, name ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LoanRow
	for rows.Next() {
		r, err := scanLoan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindLoanByID(ctx context.Context, id string) (*LoanRow, error) {
	r, err := scanLoan(s.DB.QueryRow(ctx, loanSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) CreateLoan(ctx context.Context, r LoanRow) (string, error) {
	id := uuid.NewString()
	now := nowStr()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO loans (
			id, name, lender, kind, principal_cents, monthly_rate_bp, installment_cents, total_installments,
			paid_installments, first_due_date, account_id, category_id, active, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, r.Name, r.Lender, string(r.Kind), r.PrincipalCents, r.RateBP, r.InstallmentCents, r.TotalInstallments,
		r.BasePaid, r.FirstDueDate, r.AccountID, r.CategoryID, r.Active, r.Notes, now, now)
	return id, err
}

func (s *Store) UpdateLoan(ctx context.Context, r *LoanRow) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE loans
		SET name = ?, lender = ?, kind = ?, principal_cents = ?, monthly_rate_bp = ?, installment_cents = ?,
		    total_installments = ?, paid_installments = ?, first_due_date = ?, account_id = ?, category_id = ?,
		    active = ?, notes = ?, updated_at = ?
		WHERE id = ?
	`, r.Name, r.Lender, string(r.Kind), r.PrincipalCents, r.RateBP, r.InstallmentCents,
		r.TotalInstallments, r.BasePaid, r.FirstDueDate, r.AccountID, r.CategoryID,
		r.Active, r.Notes, nowStr(), r.ID)
	return err
}

func (s *Store) DeleteLoan(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM loans WHERE id = ?`, id)
	return err
}

// LoanPaymentDates devolve, por empréstimo, as datas dos pagamentos lançados (transações com loan_id), em ordem.
func (s *Store) LoanPaymentDates(ctx context.Context) (map[string][]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT loan_id, date FROM transactions WHERE loan_id IS NOT NULL ORDER BY loan_id, date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]string)
	for rows.Next() {
		var id string
		var dt db.DateText
		if err := rows.Scan(&id, &dt); err != nil {
			return nil, err
		}
		out[id] = append(out[id], string(dt))
	}
	return out, rows.Err()
}

// CreateLoanPayment lança o pagamento de uma parcela (EXPENSE confirmada com loan_id).
func (s *Store) CreateLoanPayment(ctx context.Context, loanID, description string, amountCents int64, date, accountID string, categoryID *string) (*domain.Transaction, error) {
	id := uuid.NewString()
	now := nowStr()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO transactions (
			id, type, description, amount_cents, date, status, account_id, category_id, loan_id, created_at, updated_at
		) VALUES (?, 'EXPENSE', ?, ?, ?, 'CONFIRMED', ?, ?, ?, ?, ?)
	`, id, description, -abs64(amountCents), date, accountID, categoryID, loanID, now, now)
	if err != nil {
		return nil, err
	}
	detail, err := s.FindTransactionByID(ctx, id)
	if err != nil || detail == nil {
		return nil, err
	}
	return &detail.Transaction, nil
}

// ───────────────────────── Patrimônio ─────────────────────────

// ConfirmedSumsByAccountUntil soma as transações confirmadas por conta com data <= date.
func (s *Store) ConfirmedSumsByAccountUntil(ctx context.Context, date string) (map[string]int64, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT account_id, COALESCE(SUM(amount_cents), 0)
		FROM transactions
		WHERE status = 'CONFIRMED' AND account_id IS NOT NULL AND date <= ?
		GROUP BY account_id
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var id string
		var sum int64
		if err := rows.Scan(&id, &sum); err != nil {
			return nil, err
		}
		out[id] = sum
	}
	return out, rows.Err()
}

// InvoiceDebtRow é uma fatura com total, vencimento (via cartão) e a data do pagamento, se paga.
type InvoiceDebtRow struct {
	Year, Month        int
	ClosingDay, DueDay int
	Paid               bool
	PaymentDate        *string
	TotalCents         int64
}

// DueDate é o vencimento da fatura.
func (r InvoiceDebtRow) DueDate() string {
	return domain.InvoiceDueDate(domain.InvoicePeriod{Year: r.Year, Month: r.Month}, r.ClosingDay, r.DueDay)
}

// UnpaidAt informa se a fatura ainda estava em aberto na data (paga depois dela ou nunca).
func (r InvoiceDebtRow) UnpaidAt(date string) bool {
	if !r.Paid {
		return true
	}
	return r.PaymentDate != nil && *r.PaymentDate > date
}

// InvoiceDebtRows lista as faturas com total positivo para o histórico do patrimônio.
func (s *Store) InvoiceDebtRows(ctx context.Context) ([]InvoiceDebtRow, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT i.year, i.month, cc.closing_day, cc.due_day, i.status, p.date, COALESCE(SUM(t.amount_cents), 0)
		FROM invoices i
		JOIN credit_cards cc ON cc.id = i.card_id
		LEFT JOIN transactions p ON p.id = i.payment_transaction_id
		LEFT JOIN transactions t ON t.invoice_id = i.id
		GROUP BY i.id, i.year, i.month, cc.closing_day, cc.due_day, i.status, p.date
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InvoiceDebtRow
	for rows.Next() {
		var r InvoiceDebtRow
		var status domain.InvoiceStatus
		var paidOn db.NullDateText
		var sum int64
		if err := rows.Scan(&r.Year, &r.Month, &r.ClosingDay, &r.DueDay, &status, &paidOn, &sum); err != nil {
			return nil, err
		}
		r.Paid = status == domain.InvoicePaid
		r.PaymentDate = paidOn.Ptr()
		r.TotalCents = abs64(sum)
		if r.TotalCents > 0 {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}
