package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"finflow/internal/db"
	"finflow/internal/domain"

	"github.com/google/uuid"
)

// ───────────────────────── Tags ─────────────────────────

// tagIDsChunk limita a quantidade de parâmetros por consulta IN (...).
const tagIDsChunk = 500

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func (s *Store) ListTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT g.id, g.name, g.color, COUNT(tt.transaction_id)
		FROM tags g
		LEFT JOIN transaction_tags tt ON tt.tag_id = g.id
		GROUP BY g.id, g.name, g.color
		ORDER BY g.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Tag, 0)
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.UsageCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) FindTagByID(ctx context.Context, id string) (*domain.Tag, error) {
	var t domain.Tag
	err := s.DB.QueryRow(ctx, `
		SELECT g.id, g.name, g.color, (SELECT COUNT(*) FROM transaction_tags tt WHERE tt.tag_id = g.id)
		FROM tags g WHERE g.id = ?
	`, id).Scan(&t.ID, &t.Name, &t.Color, &t.UsageCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) CreateTag(ctx context.Context, name, color string) (string, error) {
	id := uuid.NewString()
	_, err := s.DB.Exec(ctx, `INSERT INTO tags (id, name, color, created_at) VALUES (?, ?, ?, ?)`, id, name, color, nowStr())
	return id, err
}

func (s *Store) UpdateTag(ctx context.Context, id, name, color string) error {
	_, err := s.DB.Exec(ctx, `UPDATE tags SET name = ?, color = ? WHERE id = ?`, name, color, id)
	return err
}

func (s *Store) DeleteTag(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM tags WHERE id = ?`, id)
	return err
}

// ExistingTagIDs devolve quais dos ids informados existem.
func (s *Store) ExistingTagIDs(ctx context.Context, ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for start := 0; start < len(ids); start += tagIDsChunk {
		chunk := ids[start:min(start+tagIDsChunk, len(ids))]
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := s.DB.Query(ctx, `SELECT id FROM tags WHERE id IN (`+placeholders(len(chunk))+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// insertTransactionTags grava as tags de uma transação dentro de uma transação de banco (ignora repetidas).
func insertTransactionTags(ctx context.Context, tx *db.Tx, transactionID string, tagIDs []string) error {
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO transaction_tags (transaction_id, tag_id) VALUES (?, ?)
			ON CONFLICT (transaction_id, tag_id) DO NOTHING
		`, transactionID, tagID); err != nil {
			return err
		}
	}
	return nil
}

// SetTransactionTags substitui o conjunto de tags da transação.
func (s *Store) SetTransactionTags(ctx context.Context, transactionID string, tagIDs []string) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(ctx, `DELETE FROM transaction_tags WHERE transaction_id = ?`, transactionID); err != nil {
		return err
	}
	if err := insertTransactionTags(ctx, tx, transactionID, tagIDs); err != nil {
		return err
	}
	return tx.Commit()
}

// AddTransactionTags acrescenta tags (transação → tags) em lote e devolve as transações que ganharam alguma tag nova.
func (s *Store) AddTransactionTags(ctx context.Context, assignments map[string][]string) (map[string]bool, error) {
	changed := map[string]bool{}
	if len(assignments) == 0 {
		return changed, nil
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for txID, tagIDs := range assignments {
		added := false
		for _, tagID := range tagIDs {
			res, err := tx.Exec(ctx, `
				INSERT INTO transaction_tags (transaction_id, tag_id) VALUES (?, ?)
				ON CONFLICT (transaction_id, tag_id) DO NOTHING
			`, txID, tagID)
			if err != nil {
				return nil, err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				added = true
			}
		}
		if added {
			changed[txID] = true
		}
	}
	return changed, tx.Commit()
}

// AttachTags preenche Transaction.Tags (sempre não nulo) das transações informadas.
func (s *Store) AttachTags(ctx context.Context, items []domain.Transaction) error {
	idx := make(map[string][]int, len(items))
	ids := make([]string, 0, len(items))
	for i := range items {
		if items[i].Tags == nil {
			items[i].Tags = []domain.TagRef{}
		}
		if _, ok := idx[items[i].ID]; !ok {
			ids = append(ids, items[i].ID)
		}
		idx[items[i].ID] = append(idx[items[i].ID], i)
	}
	for start := 0; start < len(ids); start += tagIDsChunk {
		chunk := ids[start:min(start+tagIDsChunk, len(ids))]
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := s.DB.Query(ctx, `
			SELECT tt.transaction_id, g.id, g.name, g.color
			FROM transaction_tags tt
			JOIN tags g ON g.id = tt.tag_id
			WHERE tt.transaction_id IN (`+placeholders(len(chunk))+`)
			ORDER BY g.name ASC
		`, args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var txID string
			var ref domain.TagRef
			if err := rows.Scan(&txID, &ref.ID, &ref.Name, &ref.Color); err != nil {
				rows.Close()
				return err
			}
			for _, i := range idx[txID] {
				items[i].Tags = append(items[i].Tags, ref)
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

// TagSpendingBetween soma o gasto (EXPENSE + CARD_EXPENSE, pendentes inclusive, pela data da compra)
// por tag em [start, end); só tags com algum lançamento no período.
func (s *Store) TagSpendingBetween(ctx context.Context, start, end string) ([]domain.TagSpending, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT g.id, g.name, g.color, COALESCE(SUM(t.amount_cents), 0), COUNT(t.id)
		FROM tags g
		JOIN transaction_tags tt ON tt.tag_id = g.id
		JOIN transactions t ON t.id = tt.transaction_id
		WHERE t.type IN ('EXPENSE', 'CARD_EXPENSE') AND t.date >= ? AND t.date < ?
		GROUP BY g.id, g.name, g.color
		ORDER BY COALESCE(SUM(t.amount_cents), 0) ASC, g.name ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.TagSpending, 0)
	for rows.Next() {
		var ts domain.TagSpending
		var sum int64
		if err := rows.Scan(&ts.TagID, &ts.Name, &ts.Color, &sum, &ts.Count); err != nil {
			return nil, err
		}
		ts.TotalCents = -sum
		out = append(out, ts)
	}
	return out, rows.Err()
}
