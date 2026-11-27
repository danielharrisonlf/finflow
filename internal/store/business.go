package store

import (
	"context"
	"database/sql"

	"finflow/internal/db"
	"finflow/internal/domain"
)

// ───────────────────────── Empresa: categorias padrão e dados do DRE ─────────────────────────

type businessCategory struct {
	name, icon, color string
	kind              domain.CategoryKind
	spend             domain.SpendType
	dre               domain.DREGroup
	children          []businessCategory
}

func bizSub(name string, spend domain.SpendType) businessCategory {
	return businessCategory{name: name, spend: spend}
}

// defaultBusinessCategories são as categorias de um espaço Empresa (v3 fase 2 §5): receitas na
// raiz e despesas agrupadas pelo grupo do DRE (as subcategorias herdam ícone, cor e grupo).
var defaultBusinessCategories = []businessCategory{
	{name: "Vendas", icon: "shopping-bag", color: "#10b981", kind: domain.CategoryIncome, dre: domain.DRERevenue},
	{name: "Serviços", icon: "briefcase", color: "#22d3ee", kind: domain.CategoryIncome, dre: domain.DRERevenue},
	{name: "Assinaturas de clientes", icon: "repeat", color: "#84cc16", kind: domain.CategoryIncome, dre: domain.DRERevenue},
	{name: "Outras receitas", icon: "wallet", color: "#a3e635", kind: domain.CategoryIncome, dre: domain.DRERevenue},
	{name: "Deduções", icon: "receipt", color: "#f87171", kind: domain.CategoryExpense, spend: domain.SpendFixed, dre: domain.DREDeductions, children: []businessCategory{
		bizSub("Impostos sobre vendas", domain.SpendFixed),
	}},
	{name: "Custos", icon: "package", color: "#fb923c", kind: domain.CategoryExpense, spend: domain.SpendVariable, dre: domain.DRECosts, children: []businessCategory{
		bizSub("Fornecedores", domain.SpendVariable), bizSub("Infraestrutura/cloud", domain.SpendCuttable),
		bizSub("Comissões", domain.SpendVariable),
	}},
	{name: "Despesas operacionais", icon: "building", color: "#6366f1", kind: domain.CategoryExpense, spend: domain.SpendFixed, dre: domain.DREOpex, children: []businessCategory{
		bizSub("Pró-labore", domain.SpendFixed), bizSub("Salários e encargos", domain.SpendFixed),
		bizSub("Aluguel", domain.SpendFixed), bizSub("Contabilidade", domain.SpendFixed),
		bizSub("Software e ferramentas", domain.SpendCuttable), bizSub("Marketing", domain.SpendVariable),
		bizSub("Tarifas bancárias", domain.SpendCuttable),
	}},
	{name: "Financeiras", icon: "landmark", color: "#ef4444", kind: domain.CategoryExpense, spend: domain.SpendFixed, dre: domain.DREFinancial, children: []businessCategory{
		bizSub("Juros", domain.SpendFixed), bizSub("Empréstimos", domain.SpendFixed),
	}},
}

// EnsureBusinessCategories cria as categorias de empresa que ainda não existem (mesmo nome e tipo)
// e devolve o id de cada uma pelo nome. O grupo do DRE fica gravado em todas (editável depois).
func EnsureBusinessCategories(ctx context.Context, s *Store) (map[string]string, error) {
	ids := make(map[string]string)
	ensure := func(c businessCategory, parent *businessCategory, parentID *string) error {
		kind, icon, color, dre := c.kind, c.icon, c.color, c.dre
		if parent != nil {
			kind, icon, color, dre = parent.kind, parent.icon, parent.color, parent.dre
		}
		existing, err := s.FindCategoryByName(ctx, c.name, kind)
		if err != nil {
			return err
		}
		if existing != nil {
			ids[c.name] = existing.ID
			return nil
		}
		in := domain.CreateCategoryInput{Name: c.name, Kind: kind, Icon: icon, Color: color, ParentID: parentID, DREGroup: &dre}
		if kind == domain.CategoryExpense {
			st := c.spend
			in.SpendType = &st
		}
		created, err := s.CreateCategory(ctx, in)
		if err != nil {
			return err
		}
		ids[c.name] = created.ID
		return nil
	}
	for i := range defaultBusinessCategories {
		c := &defaultBusinessCategories[i]
		if err := ensure(*c, nil, nil); err != nil {
			return nil, err
		}
		parentID := ids[c.name]
		for _, child := range c.children {
			if err := ensure(child, c, &parentID); err != nil {
				return nil, err
			}
		}
	}
	return ids, nil
}

// DRETransaction é um lançamento confirmado que entra no DRE (valor com sinal).
type DRETransaction struct {
	Date        string
	AmountCents int64
	Type        domain.TransactionType
	CategoryID  *string
}

// DRETransactionsBetween lista receitas, despesas e compras no cartão confirmadas em [start, end)
// (regime de competência: pela data do lançamento; transferências e pagamentos de fatura ficam fora).
func (s *Store) DRETransactionsBetween(ctx context.Context, start, end string) ([]DRETransaction, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT date, amount_cents, type, category_id
		FROM transactions
		WHERE type IN ('INCOME', 'EXPENSE', 'CARD_EXPENSE') AND status = 'CONFIRMED' AND date >= ? AND date < ?
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DRETransaction, 0)
	for rows.Next() {
		var t DRETransaction
		var dt db.DateText
		var cat sql.NullString
		if err := rows.Scan(&dt, &t.AmountCents, &t.Type, &cat); err != nil {
			return nil, err
		}
		t.Date = string(dt)
		t.CategoryID = db.NullStringPtr(cat)
		out = append(out, t)
	}
	return out, rows.Err()
}
