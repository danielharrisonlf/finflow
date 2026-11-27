package store

import (
	"context"
	"database/sql"

	"finflow/internal/db"
	"finflow/internal/domain"
)

// ───────────────────────── Insights ─────────────────────────

// SpendingRow é o gasto (EXPENSE + CARD_EXPENSE, pendentes inclusive) de uma categoria num dia.
type SpendingRow struct {
	Date       string
	CategoryID *string
	Name       *string
	Icon       *string
	Color      *string
	TotalCents int64
}

// SpendingRows agrupa o gasto por dia e categoria em [start, end), pela data da compra
// (competência, como os orçamentos). Pagamentos de fatura não entram.
func (s *Store) SpendingRows(ctx context.Context, start, end string) ([]SpendingRow, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT t.date, t.category_id, c.name, c.icon, c.color, COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.type IN ('EXPENSE', 'CARD_EXPENSE') AND t.date >= ? AND t.date < ?
		GROUP BY t.date, t.category_id, c.name, c.icon, c.color
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SpendingRow
	for rows.Next() {
		var (
			r                        SpendingRow
			dt                       db.DateText
			catID, name, icon, color sql.NullString
			sum                      int64
		)
		if err := rows.Scan(&dt, &catID, &name, &icon, &color, &sum); err != nil {
			return nil, err
		}
		r.Date = string(dt)
		r.CategoryID = db.NullStringPtr(catID)
		r.Name = db.NullStringPtr(name)
		r.Icon = db.NullStringPtr(icon)
		r.Color = db.NullStringPtr(color)
		r.TotalCents = -sum
		out = append(out, r)
	}
	return out, rows.Err()
}

// DatedAmount é uma soma de valores num dia.
type DatedAmount struct {
	Date  string
	Cents int64
}

func (s *Store) datedSums(ctx context.Context, query string, args ...any) ([]DatedAmount, error) {
	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DatedAmount
	for rows.Next() {
		var d DatedAmount
		var dt db.DateText
		if err := rows.Scan(&dt, &d.Cents); err != nil {
			return nil, err
		}
		d.Date = string(dt)
		out = append(out, d)
	}
	return out, rows.Err()
}

// IncomeByDate soma as receitas confirmadas por dia em [start, end) (transferências não contam).
func (s *Store) IncomeByDate(ctx context.Context, start, end string) ([]DatedAmount, error) {
	return s.datedSums(ctx, `
		SELECT date, COALESCE(SUM(amount_cents), 0)
		FROM transactions
		WHERE type = 'INCOME' AND status = 'CONFIRMED' AND date >= ? AND date < ?
		GROUP BY date
	`, start, end)
}

// SavingsTransfersByDate soma as transferências que entram em contas SAVINGS/INVESTMENT vindas
// de contas líquidas. Movimentações entre duas contas de reserva não são economia nova.
func (s *Store) SavingsTransfersByDate(ctx context.Context, start, end string) ([]DatedAmount, error) {
	return s.datedSums(ctx, `
		SELECT t.date, COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.type = 'TRANSFER' AND t.amount_cents > 0 AND a.type IN ('SAVINGS', 'INVESTMENT')
		  AND NOT EXISTS (
			SELECT 1 FROM transactions o
			JOIN accounts oa ON oa.id = o.account_id
			WHERE o.transfer_group_id = t.transfer_group_id AND o.id <> t.id
			  AND oa.type IN ('SAVINGS', 'INVESTMENT')
		  )
		  AND t.date >= ? AND t.date < ?
		GROUP BY t.date
	`, start, end)
}

// GoalContributionsByDate soma os aportes (e resgates, negativos) em metas por dia.
func (s *Store) GoalContributionsByDate(ctx context.Context, start, end string) ([]DatedAmount, error) {
	return s.datedSums(ctx, `
		SELECT date, COALESCE(SUM(amount_cents), 0)
		FROM goal_contributions
		WHERE date >= ? AND date < ?
		GROUP BY date
	`, start, end)
}

// ConfirmedFlowsByDate soma as transações confirmadas das contas ativas por dia em [start, end).
func (s *Store) ConfirmedFlowsByDate(ctx context.Context, start, end string) ([]DatedAmount, error) {
	return s.datedSums(ctx, `
		SELECT t.date, COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.status = 'CONFIRMED' AND a.archived = ? AND t.date >= ? AND t.date < ?
		GROUP BY t.date
	`, false, start, end)
}

// BalanceBefore é o saldo total das contas ativas considerando as transações confirmadas
// com data anterior a `date`.
func (s *Store) BalanceBefore(ctx context.Context, date string) (int64, error) {
	var initSum, txSum int64
	if err := s.DB.QueryRow(ctx, `SELECT COALESCE(SUM(initial_balance_cents), 0) FROM accounts WHERE archived = ?`, false).Scan(&initSum); err != nil {
		return 0, err
	}
	if err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.status = 'CONFIRMED' AND a.archived = ? AND t.date < ?
	`, false, date).Scan(&txSum); err != nil {
		return 0, err
	}
	return initSum + txSum, nil
}

// InstallmentLeg é uma parcela (CARD_EXPENSE com installment_total > 1) com a fatura e o cartão.
type InstallmentLeg struct {
	Description       string
	AmountCents       int64
	InstallmentNumber int
	InstallmentTotal  int
	InvoiceID         string
	InvoiceYear       int
	InvoiceMonth      int
	InvoicePaid       bool
	CardName          string
	ClosingDay        int
	DueDay            int
}

// DueDate é o vencimento da fatura da parcela.
func (l InstallmentLeg) DueDate() string {
	return domain.InvoiceDueDate(domain.InvoicePeriod{Year: l.InvoiceYear, Month: l.InvoiceMonth}, l.ClosingDay, l.DueDay)
}

// InstallmentLegs lista as parcelas cujas faturas estão entre os períodos (inclusive).
func (s *Store) InstallmentLegs(ctx context.Context, from, to domain.InvoicePeriod) ([]InstallmentLeg, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT t.description, t.amount_cents, t.installment_number, t.installment_total,
		       i.id, i.year, i.month, i.status, cc.name, cc.closing_day, cc.due_day
		FROM transactions t
		JOIN invoices i ON i.id = t.invoice_id
		JOIN credit_cards cc ON cc.id = t.card_id
		WHERE t.type = 'CARD_EXPENSE' AND t.installment_total > 1
		  AND (i.year * 12 + i.month) >= ? AND (i.year * 12 + i.month) <= ?
		ORDER BY i.year, i.month, t.description
	`, from.Year*12+from.Month, to.Year*12+to.Month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InstallmentLeg
	for rows.Next() {
		var l InstallmentLeg
		var status domain.InvoiceStatus
		if err := rows.Scan(&l.Description, &l.AmountCents, &l.InstallmentNumber, &l.InstallmentTotal,
			&l.InvoiceID, &l.InvoiceYear, &l.InvoiceMonth, &status, &l.CardName, &l.ClosingDay, &l.DueDay); err != nil {
			return nil, err
		}
		l.AmountCents = abs64(l.AmountCents)
		l.InvoicePaid = status == domain.InvoicePaid
		out = append(out, l)
	}
	return out, rows.Err()
}

// PendingAccountTx é uma transação pendente de uma conta ativa (valor com sinal).
type PendingAccountTx struct {
	Description string
	Date        string
	AmountCents int64
}

func (s *Store) PendingAccountTransactions(ctx context.Context) ([]PendingAccountTx, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT t.description, t.date, t.amount_cents
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.status = 'PENDING' AND a.archived = ?
		ORDER BY t.date ASC, t.created_at ASC
	`, false)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PendingAccountTx
	for rows.Next() {
		var p PendingAccountTx
		var dt db.DateText
		if err := rows.Scan(&p.Description, &dt, &p.AmountCents); err != nil {
			return nil, err
		}
		p.Date = string(dt)
		out = append(out, p)
	}
	return out, rows.Err()
}

// LiquidBalanceCents soma os saldos confirmados das contas ativas que não são de reserva
// (exclui SAVINGS/INVESTMENT): é o dinheiro que circula no dia a dia e alimenta a projeção.
func (s *Store) LiquidBalanceCents(ctx context.Context) (int64, error) {
	var initSum, txSum int64
	if err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(initial_balance_cents), 0) FROM accounts
		WHERE archived = ? AND type NOT IN ('SAVINGS', 'INVESTMENT')
	`, false).Scan(&initSum); err != nil {
		return 0, err
	}
	if err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.status = 'CONFIRMED' AND a.archived = ? AND a.type NOT IN ('SAVINGS', 'INVESTMENT')
	`, false).Scan(&txSum); err != nil {
		return 0, err
	}
	return initSum + txSum, nil
}

// HasReserveAccounts informa se existe alguma conta ativa de reserva (SAVINGS/INVESTMENT).
func (s *Store) HasReserveAccounts(ctx context.Context) (bool, error) {
	var n int
	err := s.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM accounts WHERE archived = ? AND type IN ('SAVINGS', 'INVESTMENT')
	`, false).Scan(&n)
	return n > 0, err
}
