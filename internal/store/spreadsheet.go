package store

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"finflow/internal/db"
	"finflow/internal/domain"
)

// ───────────────────────── Planilhas (import/export) ─────────────────────────

// SpreadsheetTxRow é um lançamento com o contexto que a exportação anual precisa:
// tipo da conta, dados do cartão/fatura (para o vencimento) e tipo de gasto efetivo.
type SpreadsheetTxRow struct {
	domain.Transaction
	AccountType *domain.AccountType
	// CounterpartAccountType é o tipo da conta da outra perna (só transferências).
	CounterpartAccountType *domain.AccountType
	CardClosingDay         *int
	CardDueDay             *int
	InvoiceYear            *int
	InvoiceMonth           *int
	// SpendType já resolvido: o da categoria ou, se nulo, o da categoria pai.
	SpendType *domain.SpendType
}

// ListSpreadsheetTransactions lista os lançamentos com data em [start, end), com o contexto da exportação.
func (s *Store) ListSpreadsheetTransactions(ctx context.Context, start, end string) ([]SpreadsheetTxRow, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT t.id, t.type, t.description, t.amount_cents, t.date, t.status,
		       t.account_id, a.name, a.type, t.card_id, cc.name, cc.closing_day, cc.due_day,
		       i.year, i.month, t.category_id, c.name, COALESCE(c.spend_type, p.spend_type),
		       t.transfer_group_id,
		       (SELECT a2.type FROM transactions t2 JOIN accounts a2 ON a2.id = t2.account_id
		        WHERE t2.transfer_group_id = t.transfer_group_id AND t2.id <> t.id LIMIT 1),
		       t.installment_number, t.installment_total
		FROM transactions t
		LEFT JOIN accounts a ON a.id = t.account_id
		LEFT JOIN credit_cards cc ON cc.id = t.card_id
		LEFT JOIN invoices i ON i.id = t.invoice_id
		LEFT JOIN categories c ON c.id = t.category_id
		LEFT JOIN categories p ON p.id = c.parent_id
		WHERE t.date >= ? AND t.date < ?
		ORDER BY t.date ASC, t.created_at ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SpreadsheetTxRow, 0)
	for rows.Next() {
		var (
			r                              SpreadsheetTxRow
			dt                             db.DateText
			accountID, accountName, accTyp sql.NullString
			cardID, cardName               sql.NullString
			closingDay, dueDay             sql.NullInt64
			invYear, invMonth              sql.NullInt64
			categoryID, categoryName       sql.NullString
			spendType, transferGID, cpType sql.NullString
			instNum, instTot               sql.NullInt64
		)
		if err := rows.Scan(
			&r.ID, &r.Type, &r.Description, &r.AmountCents, &dt, &r.Status,
			&accountID, &accountName, &accTyp, &cardID, &cardName, &closingDay, &dueDay,
			&invYear, &invMonth, &categoryID, &categoryName, &spendType,
			&transferGID, &cpType, &instNum, &instTot,
		); err != nil {
			return nil, err
		}
		r.Date = string(dt)
		r.AccountID = db.NullStringPtr(accountID)
		r.AccountName = db.NullStringPtr(accountName)
		r.CardID = db.NullStringPtr(cardID)
		r.CardName = db.NullStringPtr(cardName)
		r.CategoryID = db.NullStringPtr(categoryID)
		r.CategoryName = db.NullStringPtr(categoryName)
		r.TransferGroupID = db.NullStringPtr(transferGID)
		r.InstallmentNumber = db.NullIntPtr(instNum)
		r.InstallmentTotal = db.NullIntPtr(instTot)
		r.CardClosingDay = db.NullIntPtr(closingDay)
		r.CardDueDay = db.NullIntPtr(dueDay)
		r.InvoiceYear = db.NullIntPtr(invYear)
		r.InvoiceMonth = db.NullIntPtr(invMonth)
		r.SpendType = spendTypePtr(spendType)
		if accTyp.Valid {
			v := domain.AccountType(accTyp.String)
			r.AccountType = &v
		}
		if cpType.Valid {
			v := domain.AccountType(cpType.String)
			r.CounterpartAccountType = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BalanceBeforeCents é o saldo das contas líquidas ativas (sem SAVINGS/INVESTMENT) considerando só lançamentos confirmados antes de date.
func (s *Store) BalanceBeforeCents(ctx context.Context, date string) (int64, error) {
	var initSum, txSum int64
	if err := s.DB.QueryRow(ctx, `SELECT COALESCE(SUM(initial_balance_cents), 0) FROM accounts WHERE archived = ? AND type NOT IN ('SAVINGS', 'INVESTMENT')`, false).Scan(&initSum); err != nil {
		return 0, err
	}
	if err := s.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.amount_cents), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.status = 'CONFIRMED' AND a.archived = ? AND a.type NOT IN ('SAVINGS', 'INVESTMENT') AND t.date < ?
	`, false, date).Scan(&txSum); err != nil {
		return 0, err
	}
	return initSum + txSum, nil
}

// GoalContributionsByMonth soma os aportes em metas (resgates negativos inclusos) por mês do ano.
func (s *Store) GoalContributionsByMonth(ctx context.Context, year int) ([12]int64, error) {
	var out [12]int64
	start, end := domain.YearRange(year)
	rows, err := s.DB.Query(ctx, `SELECT date, amount_cents FROM goal_contributions WHERE date >= ? AND date < ?`, start, end)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var dt db.DateText
		var amount int64
		if err := rows.Scan(&dt, &amount); err != nil {
			return out, err
		}
		d := string(dt)
		if len(d) >= 7 {
			m := int(d[5]-'0')*10 + int(d[6]-'0')
			if m >= 1 && m <= 12 {
				out[m-1] += amount
			}
		}
	}
	return out, rows.Err()
}

// AccountTxFingerprint é o mínimo para detectar duplicados de importação numa conta.
type AccountTxFingerprint struct {
	Date        string
	AmountCents int64
	Description string
}

// ListAccountFingerprints lista data, valor e descrição dos lançamentos da conta com data em [start, end].
func (s *Store) ListAccountFingerprints(ctx context.Context, accountID, start, end string) ([]AccountTxFingerprint, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT date, amount_cents, description FROM transactions
		WHERE account_id = ? AND date >= ? AND date <= ?
	`, accountID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountTxFingerprint
	for rows.Next() {
		var f AccountTxFingerprint
		var dt db.DateText
		if err := rows.Scan(&dt, &f.AmountCents, &f.Description); err != nil {
			return nil, err
		}
		f.Date = string(dt)
		out = append(out, f)
	}
	return out, rows.Err()
}

// ImportTxRow é um lançamento simples (receita/despesa) a gravar na importação.
type ImportTxRow struct {
	Type        domain.TransactionType
	Description string
	// AmountCents com sinal (despesa negativa).
	AmountCents int64
	Date        string
	Status      domain.TransactionStatus
	CategoryID  *string
}

// CreateImportedTransactions grava todos os lançamentos na conta numa única transação de banco.
func (s *Store) CreateImportedTransactions(ctx context.Context, accountID string, items []ImportTxRow) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	now := nowStr()
	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO transactions (
				id, type, description, amount_cents, date, status, account_id, category_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, uuid.NewString(), string(it.Type), it.Description, it.AmountCents, it.Date, string(it.Status), accountID, it.CategoryID, now, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(items), nil
}
