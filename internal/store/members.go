package store

import (
	"context"
	"database/sql"
	"errors"

	"finflow/internal/db"
	"finflow/internal/domain"

	"github.com/google/uuid"
)

// ───────────────────────── Casa: membros, divisão e acertos ─────────────────────────

func (s *Store) ListMembers(ctx context.Context) ([]domain.Member, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, name, color FROM members ORDER BY created_at ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Member, 0)
	for rows.Next() {
		var m domain.Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Color); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) FindMemberByID(ctx context.Context, id string) (*domain.Member, error) {
	var m domain.Member
	err := s.DB.QueryRow(ctx, `SELECT id, name, color FROM members WHERE id = ?`, id).Scan(&m.ID, &m.Name, &m.Color)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) CreateMember(ctx context.Context, name, color string) (string, error) {
	id := uuid.NewString()
	_, err := s.DB.Exec(ctx, `INSERT INTO members (id, name, color, created_at) VALUES (?, ?, ?, ?)`, id, name, color, nowStr())
	return id, err
}

func (s *Store) UpdateMember(ctx context.Context, m domain.Member) error {
	_, err := s.DB.Exec(ctx, `UPDATE members SET name = ?, color = ? WHERE id = ?`, m.Name, m.Color, m.ID)
	return err
}

func (s *Store) DeleteMember(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM members WHERE id = ?`, id)
	return err
}

// insertTransactionSplits grava a divisão de uma transação dentro de uma transação de banco.
func insertTransactionSplits(ctx context.Context, tx *db.Tx, transactionID string, splits []domain.SplitShare) error {
	for _, sp := range splits {
		if _, err := tx.Exec(ctx, `
			INSERT INTO transaction_splits (transaction_id, member_id, share_bp) VALUES (?, ?, ?)
		`, transactionID, sp.MemberID, sp.ShareBP); err != nil {
			return err
		}
	}
	return nil
}

// SetTransactionSharing grava quem pagou e substitui a divisão da transação; setPaidBy/setSplits
// indicam quais dos dois campos mudam.
func (s *Store) SetTransactionSharing(ctx context.Context, transactionID string, setPaidBy bool, paidBy *string, setSplits bool, splits []domain.SplitShare) error {
	if !setPaidBy && !setSplits {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if setPaidBy {
		if _, err := tx.Exec(ctx, `UPDATE transactions SET paid_by_member_id = ?, updated_at = ? WHERE id = ?`, paidBy, nowStr(), transactionID); err != nil {
			return err
		}
	}
	if setSplits {
		if _, err := tx.Exec(ctx, `DELETE FROM transaction_splits WHERE transaction_id = ?`, transactionID); err != nil {
			return err
		}
		if err := insertTransactionSplits(ctx, tx, transactionID, splits); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AttachRelations preenche as tags e a divisão (sempre não nulas) das transações informadas.
func (s *Store) AttachRelations(ctx context.Context, items []domain.Transaction) error {
	if err := s.AttachTags(ctx, items); err != nil {
		return err
	}
	return s.AttachSplits(ctx, items)
}

// AttachSplits preenche Transaction.Splits (sempre não nulo) das transações informadas.
func (s *Store) AttachSplits(ctx context.Context, items []domain.Transaction) error {
	idx := make(map[string][]int, len(items))
	ids := make([]string, 0, len(items))
	for i := range items {
		if items[i].Splits == nil {
			items[i].Splits = []domain.TransactionSplit{}
		}
		if _, ok := idx[items[i].ID]; !ok {
			ids = append(ids, items[i].ID)
		}
		idx[items[i].ID] = append(idx[items[i].ID], i)
	}
	if len(ids) == 0 {
		return nil
	}
	// Atalho barato: espaços sem divisão nenhuma (a maioria) não pagam a consulta por lote.
	var hasSplits int
	if err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM (SELECT 1 FROM transaction_splits LIMIT 1) x`).Scan(&hasSplits); err != nil {
		return err
	}
	if hasSplits == 0 {
		return nil
	}
	for start := 0; start < len(ids); start += tagIDsChunk {
		chunk := ids[start:min(start+tagIDsChunk, len(ids))]
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := s.DB.Query(ctx, `
			SELECT sp.transaction_id, sp.member_id, sp.share_bp
			FROM transaction_splits sp
			JOIN members m ON m.id = sp.member_id
			WHERE sp.transaction_id IN (`+placeholders(len(chunk))+`)
			ORDER BY m.created_at ASC, m.name ASC
		`, args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var txID, memberID string
			var bp int
			if err := rows.Scan(&txID, &memberID, &bp); err != nil {
				rows.Close()
				return err
			}
			for _, i := range idx[txID] {
				items[i].Splits = append(items[i].Splits, domain.TransactionSplit{MemberID: memberID, SharePct: domain.SharePctFromBP(bp)})
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// SharedExpensesBetween lista as despesas confirmadas (EXPENSE e CARD_EXPENSE) com quem pagou
// definido em [start, end), com a divisão gravada (vazia = divisão igual).
func (s *Store) SharedExpensesBetween(ctx context.Context, start, end string) ([]domain.SharedExpense, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT t.id, t.paid_by_member_id, t.amount_cents, sp.member_id, sp.share_bp
		FROM transactions t
		LEFT JOIN transaction_splits sp ON sp.transaction_id = t.id
		WHERE t.type IN ('EXPENSE', 'CARD_EXPENSE') AND t.status = 'CONFIRMED'
		  AND t.paid_by_member_id IS NOT NULL AND t.date >= ? AND t.date < ?
		ORDER BY t.date ASC, t.id ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.SharedExpense, 0)
	pos := map[string]int{}
	for rows.Next() {
		var id, paidBy string
		var amount int64
		var memberID sql.NullString
		var bp sql.NullInt64
		if err := rows.Scan(&id, &paidBy, &amount, &memberID, &bp); err != nil {
			return nil, err
		}
		i, ok := pos[id]
		if !ok {
			i = len(out)
			pos[id] = i
			out = append(out, domain.SharedExpense{PaidBy: paidBy, AmountCents: abs64(amount)})
		}
		if memberID.Valid {
			out[i].Splits = append(out[i].Splits, domain.SplitShare{MemberID: memberID.String, ShareBP: int(bp.Int64)})
		}
	}
	return out, rows.Err()
}

// CreateSettlement registra um acerto entre membros.
func (s *Store) CreateSettlement(ctx context.Context, fromID, toID string, amountCents int64, date string, transferGroupID *string) (string, error) {
	id := uuid.NewString()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO settlements (id, from_member_id, to_member_id, amount_cents, date, transfer_group_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, fromID, toID, amountCents, date, transferGroupID, nowStr())
	return id, err
}

// SettlementsBetween lista os acertos registrados em [start, end).
func (s *Store) SettlementsBetween(ctx context.Context, start, end string) ([]domain.Settlement, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, from_member_id, to_member_id, amount_cents, date, transfer_group_id
		FROM settlements
		WHERE date >= ? AND date < ?
		ORDER BY date ASC, created_at ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Settlement, 0)
	for rows.Next() {
		var st domain.Settlement
		var dt db.DateText
		var group sql.NullString
		if err := rows.Scan(&st.ID, &st.FromMemberID, &st.ToMemberID, &st.AmountCents, &dt, &group); err != nil {
			return nil, err
		}
		st.Date = string(dt)
		st.TransferGroupID = db.NullStringPtr(group)
		out = append(out, st)
	}
	return out, rows.Err()
}
