package store

import (
	"context"
	"database/sql"

	"finflow/internal/db"
	"finflow/internal/domain"

	"github.com/google/uuid"
)

// ───────────────────────── Assinaturas ─────────────────────────

// SubscriptionCandidates lista as despesas sem parcelamento (EXPENSE e CARD_EXPENSE com
// installment_total nulo ou 1) a partir de `since`, exceto parcelas de empréstimo; valor positivo.
func (s *Store) SubscriptionCandidates(ctx context.Context, since string) ([]domain.SubscriptionTx, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT description, date, amount_cents, category_id, account_id, card_id, recurring_id
		FROM transactions
		WHERE type IN ('EXPENSE', 'CARD_EXPENSE') AND (installment_total IS NULL OR installment_total <= 1)
		  AND loan_id IS NULL AND date >= ?
		ORDER BY date ASC, created_at ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.SubscriptionTx
	for rows.Next() {
		var (
			t                         domain.SubscriptionTx
			dt                        db.DateText
			cat, acc, card, recurring sql.NullString
		)
		if err := rows.Scan(&t.Description, &dt, &t.AmountCents, &cat, &acc, &card, &recurring); err != nil {
			return nil, err
		}
		t.Date = string(dt)
		t.AmountCents = -t.AmountCents
		t.CategoryID = db.NullStringPtr(cat)
		t.AccountID = db.NullStringPtr(acc)
		t.CardID = db.NullStringPtr(card)
		t.RecurringID = db.NullStringPtr(recurring)
		out = append(out, t)
	}
	return out, rows.Err()
}

// ───────────────────────── Fluxo de caixa ─────────────────────────

// CashflowTxs lista receitas confirmadas e despesas (EXPENSE + CARD_EXPENSE, pendentes inclusive)
// com data em [start, end). Compras no cartão trazem o vencimento da fatura.
func (s *Store) CashflowTxs(ctx context.Context, start, end string) ([]domain.CashflowTx, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT t.type, t.date, t.amount_cents,
		       CASE WHEN t.recurring_id IS NOT NULL OR t.loan_id IS NOT NULL THEN 1 ELSE 0 END,
		       COALESCE(t.installment_total, 0),
		       t.category_id, c.name, c.icon, c.color,
		       i.year, i.month, cc.closing_day, cc.due_day
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id
		LEFT JOIN invoices i ON i.id = t.invoice_id
		LEFT JOIN credit_cards cc ON cc.id = i.card_id
		WHERE ((t.type = 'INCOME' AND t.status = 'CONFIRMED') OR t.type IN ('EXPENSE', 'CARD_EXPENSE'))
		  AND t.date >= ? AND t.date < ?
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CashflowTx
	for rows.Next() {
		var (
			txType                    domain.TransactionType
			dt                        db.DateText
			amount                    int64
			fixed, instTotal          int
			catID, name, icon, color  sql.NullString
			invY, invM, closing, dueD sql.NullInt64
		)
		if err := rows.Scan(&txType, &dt, &amount, &fixed, &instTotal, &catID, &name, &icon, &color, &invY, &invM, &closing, &dueD); err != nil {
			return nil, err
		}
		t := domain.CashflowTx{
			Income:      txType == domain.TxIncome,
			Date:        string(dt),
			AmountCents: amount,
			FixedBill:   fixed == 1,
			Installment: instTotal > 1,
			CategoryID:  db.NullStringPtr(catID),
			Name:        db.NullStringPtr(name),
			Icon:        db.NullStringPtr(icon),
			Color:       db.NullStringPtr(color),
		}
		if !t.Income {
			t.AmountCents = -amount
		}
		if txType == domain.TxCardExpense && invY.Valid && closing.Valid {
			due := domain.InvoiceDueDate(domain.InvoicePeriod{Year: int(invY.Int64), Month: int(invM.Int64)}, int(closing.Int64), int(dueD.Int64))
			t.DueDate = &due
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ───────────────────────── Composição da fatura ─────────────────────────

// InvoiceLines lista os lançamentos da fatura (valor como gravado) e o valor do pagamento (0 se aberta).
func (s *Store) InvoiceLines(ctx context.Context, invoiceID string) ([]domain.InvoiceLine, int64, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT description, amount_cents, installment_total FROM transactions WHERE invoice_id = ?
	`, invoiceID)
	if err != nil {
		return nil, 0, err
	}
	var lines []domain.InvoiceLine
	for rows.Next() {
		var l domain.InvoiceLine
		var inst sql.NullInt64
		if err := rows.Scan(&l.Description, &l.AmountCents, &inst); err != nil {
			rows.Close()
			return nil, 0, err
		}
		l.InstallmentTotal = db.NullIntPtr(inst)
		lines = append(lines, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	var paid sql.NullInt64
	if err := s.DB.QueryRow(ctx, `
		SELECT t.amount_cents FROM invoices i
		JOIN transactions t ON t.id = i.payment_transaction_id
		WHERE i.id = ? AND i.status = 'PAID'
	`, invoiceID).Scan(&paid); err != nil && err != sql.ErrNoRows {
		return nil, 0, err
	}
	return lines, abs64(paid.Int64), nil
}

// ───────────────────────── Transferências inteligentes ─────────────────────────

// TransferCandidateTxs lista despesas e receitas de contas com data >= since, sem grupo de
// transferência, recorrência ou empréstimo.
func (s *Store) TransferCandidateTxs(ctx context.Context, since string) (expenses, incomes []domain.Transaction, err error) {
	rows, err := s.DB.Query(ctx, txSelectCols+`
		WHERE t.type IN ('EXPENSE', 'INCOME') AND t.account_id IS NOT NULL
		  AND t.transfer_group_id IS NULL AND t.recurring_id IS NULL AND t.loan_id IS NULL
		  AND t.date >= ?
		ORDER BY t.date DESC, t.created_at DESC
	`, since)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	all, err := scanTransactions(rows)
	if err != nil {
		return nil, nil, err
	}
	_ = rows.Close()
	if err := s.AttachRelations(ctx, all); err != nil {
		return nil, nil, err
	}
	for _, t := range all {
		if t.Type == domain.TxExpense {
			expenses = append(expenses, t)
		} else {
			incomes = append(incomes, t)
		}
	}
	return expenses, incomes, nil
}

// MergeTransferLegs converte uma despesa e uma receita em TRANSFER do mesmo grupo (categoria nula,
// descrições mantidas) numa transação de banco. ErrConflict se alguma perna mudou nesse meio tempo.
func (s *Store) MergeTransferLegs(ctx context.Context, expenseID, incomeID string) (string, error) {
	groupID := uuid.NewString()
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	now := nowStr()
	for _, leg := range []struct{ id, typ string }{{expenseID, "EXPENSE"}, {incomeID, "INCOME"}} {
		res, err := tx.Exec(ctx, `
			UPDATE transactions SET type = 'TRANSFER', transfer_group_id = ?, category_id = NULL, updated_at = ?
			WHERE id = ? AND type = ? AND transfer_group_id IS NULL AND recurring_id IS NULL AND loan_id IS NULL
		`, groupID, now, leg.id, leg.typ)
		if err != nil {
			return "", err
		}
		if err := expectOneRow(res, ErrConflict); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return groupID, nil
}

// TransactionLoanID devolve o loan_id da transação (nil se não houver).
func (s *Store) TransactionLoanID(ctx context.Context, id string) (*string, error) {
	var loanID sql.NullString
	err := s.DB.QueryRow(ctx, `SELECT loan_id FROM transactions WHERE id = ?`, id).Scan(&loanID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return db.NullStringPtr(loanID), err
}

// ───────────────────────── Alertas ─────────────────────────

// PendingTransactionDates devolve a data de cada transação pendente.
func (s *Store) PendingTransactionDates(ctx context.Context) ([]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT date FROM transactions WHERE status = 'PENDING'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var dt db.DateText
		if err := rows.Scan(&dt); err != nil {
			return nil, err
		}
		out = append(out, string(dt))
	}
	return out, rows.Err()
}

// UncategorizedCount conta receitas, despesas e compras no cartão sem categoria em [start, end).
func (s *Store) UncategorizedCount(ctx context.Context, start, end string) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM transactions
		WHERE type IN ('INCOME', 'EXPENSE', 'CARD_EXPENSE') AND category_id IS NULL AND date >= ? AND date < ?
	`, start, end).Scan(&n)
	return n, err
}

// GoalContributionsSince soma os aportes por meta com data >= since.
func (s *Store) GoalContributionsSince(ctx context.Context, since string) (map[string]int64, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT goal_id, COALESCE(SUM(amount_cents), 0) FROM goal_contributions WHERE date >= ? GROUP BY goal_id
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
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

// InvestmentValueDates lista as posições ativas com a data da última atualização de valor.
func (s *Store) InvestmentValueDates(ctx context.Context) ([]domain.AlertInvestment, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, name, value_updated_at FROM investments WHERE archived = ? AND value_updated_at IS NOT NULL
	`, false)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AlertInvestment
	for rows.Next() {
		var inv domain.AlertInvestment
		var dt db.DateText
		if err := rows.Scan(&inv.ID, &inv.Name, &dt); err != nil {
			return nil, err
		}
		inv.ValueUpdatedAt = string(dt)
		out = append(out, inv)
	}
	return out, rows.Err()
}

// ReserveAccountsBalanceCents soma os saldos confirmados das contas ativas SAVINGS/INVESTMENT.
func (s *Store) ReserveAccountsBalanceCents(ctx context.Context) (int64, error) {
	var initSum, txSum int64
	if err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(initial_balance_cents), 0) FROM accounts
		WHERE archived = ? AND type IN ('SAVINGS', 'INVESTMENT')
	`, false).Scan(&initSum); err != nil {
		return 0, err
	}
	if err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.status = 'CONFIRMED' AND a.archived = ? AND a.type IN ('SAVINGS', 'INVESTMENT')
	`, false).Scan(&txSum); err != nil {
		return 0, err
	}
	return initSum + txSum, nil
}
