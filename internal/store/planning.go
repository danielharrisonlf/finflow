package store

import (
	"context"
	"database/sql"
	"errors"

	"finflow/internal/db"
	"finflow/internal/domain"

	"github.com/google/uuid"
)

// ───────────────────────── Settings ─────────────────────────

// Chaves da tabela settings (o valor de cada uma é JSON).
const (
	SettingMonthlyLimit      = "monthlyLimitCents"
	SettingReserveMonths     = "reserveMonths"
	SettingHasDependents     = "hasDependents"
	SettingReserveRefillRate = "reserveRefillRatePct"
	SettingBalanceBands      = "balanceBands"
	SettingSpendPlan         = "spendPlan"
)

// SettingValues lê a tabela settings (chave → JSON).
func (s *Store) SettingValues(ctx context.Context) (map[string]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// SaveSettingValues grava várias chaves numa única transação.
func (s *Store) SaveSettingValues(ctx context.Context, values map[string]string) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range values {
		if _, err := tx.Exec(ctx, `
			INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
		`, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ───────────────────────── Category rules ─────────────────────────

type CategoryRuleRow struct {
	ID           string
	Pattern      string
	CategoryID   string
	CategoryName string
	CategoryKind domain.CategoryKind
	Priority     int
}

const ruleSelect = `
	SELECT r.id, r.pattern, r.category_id, c.name, c.kind, r.priority
	FROM category_rules r
	JOIN categories c ON c.id = r.category_id
`

func scanRule(scanner interface{ Scan(...any) error }) (CategoryRuleRow, error) {
	var r CategoryRuleRow
	err := scanner.Scan(&r.ID, &r.Pattern, &r.CategoryID, &r.CategoryName, &r.CategoryKind, &r.Priority)
	return r, err
}

// ListCategoryRules devolve as regras na ordem de avaliação (prioridade desc, padrão mais longo).
func (s *Store) ListCategoryRules(ctx context.Context) ([]CategoryRuleRow, error) {
	rows, err := s.DB.Query(ctx, ruleSelect+` ORDER BY r.priority DESC, LENGTH(r.pattern) DESC, r.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CategoryRuleRow
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindCategoryRuleByID(ctx context.Context, id string) (*CategoryRuleRow, error) {
	r, err := scanRule(s.DB.QueryRow(ctx, ruleSelect+` WHERE r.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) CreateCategoryRule(ctx context.Context, pattern, categoryID string, priority int) (string, error) {
	id := uuid.NewString()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO category_rules (id, pattern, category_id, priority, created_at) VALUES (?, ?, ?, ?, ?)
	`, id, pattern, categoryID, priority, nowStr())
	return id, err
}

func (s *Store) UpdateCategoryRule(ctx context.Context, r *CategoryRuleRow) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE category_rules SET pattern = ?, category_id = ?, priority = ? WHERE id = ?
	`, r.Pattern, r.CategoryID, r.Priority, r.ID)
	return err
}

func (s *Store) DeleteCategoryRule(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM category_rules WHERE id = ?`, id)
	return err
}

// RuleTarget é uma transação candidata a receber categoria pelas regras.
type RuleTarget struct {
	ID          string
	Description string
	Type        domain.TransactionType
	CategoryID  *string
}

// ListRuleTargets lista receitas, despesas e compras no cartão (só as sem categoria, se pedido).
func (s *Store) ListRuleTargets(ctx context.Context, onlyUncategorized bool) ([]RuleTarget, error) {
	q := `SELECT id, description, type, category_id FROM transactions WHERE type IN ('INCOME', 'EXPENSE', 'CARD_EXPENSE')`
	if onlyUncategorized {
		q += ` AND category_id IS NULL`
	}
	rows, err := s.DB.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RuleTarget
	for rows.Next() {
		var t RuleTarget
		var catID sql.NullString
		if err := rows.Scan(&t.ID, &t.Description, &t.Type, &catID); err != nil {
			return nil, err
		}
		t.CategoryID = db.NullStringPtr(catID)
		out = append(out, t)
	}
	return out, rows.Err()
}

// AssignCategories aplica category_id em lote (transação id → categoria) numa única transação.
func (s *Store) AssignCategories(ctx context.Context, assignments map[string]string) error {
	if len(assignments) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := nowStr()
	for txID, catID := range assignments {
		if _, err := tx.Exec(ctx, `UPDATE transactions SET category_id = ?, updated_at = ? WHERE id = ?`, catID, now, txID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ───────────────────────── Wishlist ─────────────────────────

type WishItemRow struct {
	ID         string
	Name       string
	PriceCents int64
	URL        *string
	Notes      *string
	Priority   int
	Status     domain.WishStatus
}

const wishSelect = `SELECT id, name, price_cents, url, notes, priority, status FROM wishlist`

func scanWish(scanner interface{ Scan(...any) error }) (WishItemRow, error) {
	var r WishItemRow
	var url, notes sql.NullString
	if err := scanner.Scan(&r.ID, &r.Name, &r.PriceCents, &url, &notes, &r.Priority, &r.Status); err != nil {
		return r, err
	}
	r.URL = db.NullStringPtr(url)
	r.Notes = db.NullStringPtr(notes)
	return r, nil
}

// ListWishItems devolve a lista na ordem de exibição: LISTED primeiro, depois prioridade (menor = antes).
func (s *Store) ListWishItems(ctx context.Context) ([]WishItemRow, error) {
	rows, err := s.DB.Query(ctx, wishSelect+`
		ORDER BY CASE status WHEN 'LISTED' THEN 0 WHEN 'BOUGHT' THEN 1 ELSE 2 END, priority ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WishItemRow
	for rows.Next() {
		r, err := scanWish(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FindWishItemByID(ctx context.Context, id string) (*WishItemRow, error) {
	r, err := scanWish(s.DB.QueryRow(ctx, wishSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// NextWishPriority é a prioridade para colocar um item no fim da lista.
func (s *Store) NextWishPriority(ctx context.Context) (int, error) {
	var maxPriority sql.NullInt64
	if err := s.DB.QueryRow(ctx, `SELECT MAX(priority) FROM wishlist`).Scan(&maxPriority); err != nil {
		return 0, err
	}
	if !maxPriority.Valid {
		return 1, nil
	}
	return int(maxPriority.Int64) + 1, nil
}

func (s *Store) CreateWishItem(ctx context.Context, r WishItemRow) (string, error) {
	id := uuid.NewString()
	now := nowStr()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO wishlist (id, name, price_cents, url, notes, priority, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, r.Name, r.PriceCents, r.URL, r.Notes, r.Priority, string(r.Status), now, now)
	return id, err
}

func (s *Store) UpdateWishItem(ctx context.Context, r *WishItemRow) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE wishlist SET name = ?, price_cents = ?, url = ?, notes = ?, priority = ?, status = ?, updated_at = ?
		WHERE id = ?
	`, r.Name, r.PriceCents, r.URL, r.Notes, r.Priority, string(r.Status), nowStr(), r.ID)
	return err
}

func (s *Store) DeleteWishItem(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM wishlist WHERE id = ?`, id)
	return err
}
