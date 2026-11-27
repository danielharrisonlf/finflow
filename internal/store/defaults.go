package store

import (
	"context"

	"finflow/internal/domain"
)

// ───────────────────────── Categorias padrão ─────────────────────────

type defaultCategory struct {
	name, icon, color string
	spend             domain.SpendType
	children          []defaultCategory
}

func sub(name string, spend domain.SpendType) defaultCategory {
	return defaultCategory{name: name, spend: spend}
}

// defaultExpenseCategories é a taxonomia brasileira padrão (subcategorias herdam ícone e cor).
var defaultExpenseCategories = []defaultCategory{
	{"Moradia", "house", "#38bdf8", domain.SpendFixed, []defaultCategory{
		sub("Aluguel", domain.SpendFixed), sub("IPTU", domain.SpendFixed), sub("Condomínio", domain.SpendFixed),
		sub("Manutenção da casa", domain.SpendVariable),
	}},
	{"Utilidades", "plug", "#0ea5e9", domain.SpendFixed, []defaultCategory{
		sub("Energia", domain.SpendFixed), sub("Água", domain.SpendFixed), sub("Gás", domain.SpendFixed),
	}},
	{"Serviços", "wifi", "#6366f1", domain.SpendFixed, []defaultCategory{
		sub("Internet", domain.SpendFixed), sub("Celular", domain.SpendFixed), sub("TV", domain.SpendCuttable),
	}},
	{"Serviços digitais", "repeat", "#94a3b8", domain.SpendCuttable, []defaultCategory{
		sub("Streaming", domain.SpendCuttable), sub("Música", domain.SpendCuttable), sub("Jogos", domain.SpendCuttable),
	}},
	{"Alimentação", "utensils", "#fb923c", domain.SpendVariable, []defaultCategory{
		sub("Supermercado", domain.SpendVariable), sub("Restaurantes", domain.SpendVariable), sub("Delivery", domain.SpendVariable),
	}},
	{"Transporte", "bus", "#a78bfa", domain.SpendVariable, []defaultCategory{
		sub("Aplicativos de transporte", domain.SpendVariable), sub("Transporte público", domain.SpendVariable),
	}},
	{"Automotivo", "car", "#8b5cf6", domain.SpendVariable, []defaultCategory{
		sub("Combustível", domain.SpendVariable), sub("Estacionamento", domain.SpendVariable),
		sub("Manutenção do carro", domain.SpendVariable), sub("IPVA e licenciamento", domain.SpendFixed),
	}},
	{"Saúde", "heart-pulse", "#34d399", domain.SpendVariable, []defaultCategory{
		sub("Seguro saúde", domain.SpendFixed), sub("Farmácia", domain.SpendVariable),
		sub("Consultas e exames", domain.SpendVariable), sub("Academia", domain.SpendCuttable),
	}},
	{"Educação", "graduation-cap", "#60a5fa", domain.SpendFixed, []defaultCategory{
		sub("Mensalidade escolar", domain.SpendFixed), sub("Cursos", domain.SpendFixed),
	}},
	{"Lazer", "gamepad-2", "#f472b6", domain.SpendVariable, nil},
	{"Compras", "shopping-cart", "#fbbf24", domain.SpendVariable, []defaultCategory{
		sub("Roupas", domain.SpendVariable), sub("Eletrônicos", domain.SpendVariable), sub("Presentes", domain.SpendVariable),
	}},
	{"Viagens", "plane", "#22d3ee", domain.SpendVariable, nil},
	{"Seguros", "shield", "#14b8a6", domain.SpendFixed, nil},
	{"Empréstimos e financiamentos", "landmark", "#ef4444", domain.SpendFixed, nil},
	{"Impostos", "receipt", "#f87171", domain.SpendFixed, nil},
	{"Tarifas bancárias", "banknote", "#64748b", domain.SpendCuttable, nil},
	{"Doações", "hand-heart", "#e879f9", domain.SpendVariable, nil},
	{"Pet", "paw-print", "#d97706", domain.SpendVariable, nil},
	{"Outros", "tag", "#71717a", domain.SpendVariable, nil},
}

var defaultIncomeCategories = []defaultCategory{
	{name: "Salário", icon: "briefcase", color: "#10b981"},
	{name: "Freelance", icon: "laptop", color: "#22d3ee"},
	{name: "Rendimentos", icon: "trending-up", color: "#84cc16"},
	{name: "Reembolsos", icon: "coins", color: "#2dd4bf"},
	{name: "Outras receitas", icon: "wallet", color: "#a3e635"},
}

// EnsureDefaultCategories cria as categorias padrão que ainda não existem (mesmo nome e
// tipo, sem diferenciar maiúsculas) e devolve o id de cada uma pelo nome.
func EnsureDefaultCategories(ctx context.Context, s *Store) (map[string]string, error) {
	ids := make(map[string]string)
	ensure := func(kind domain.CategoryKind, c defaultCategory, parentID *string, inherited defaultCategory) error {
		existing, err := s.FindCategoryByName(ctx, c.name, kind)
		if err != nil {
			return err
		}
		if existing != nil {
			ids[c.name] = existing.ID
			return nil
		}
		in := domain.CreateCategoryInput{Name: c.name, Kind: kind, Icon: c.icon, Color: c.color, ParentID: parentID}
		if in.Icon == "" {
			in.Icon = inherited.icon
		}
		if in.Color == "" {
			in.Color = inherited.color
		}
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
	for _, c := range defaultExpenseCategories {
		if err := ensure(domain.CategoryExpense, c, nil, c); err != nil {
			return nil, err
		}
		parentID := ids[c.name]
		for _, child := range c.children {
			if err := ensure(domain.CategoryExpense, child, &parentID, c); err != nil {
				return nil, err
			}
		}
	}
	for _, c := range defaultIncomeCategories {
		if err := ensure(domain.CategoryIncome, c, nil, c); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// spendBackfillKey marca, na tabela settings, que o preenchimento de spend_type já rodou.
const spendBackfillKey = "_migration.spendTypeBackfill"

// Bootstrap prepara o banco no boot: cria as categorias padrão quando não há nenhuma e,
// uma única vez por banco, preenche por heurística de nome o spend_type das despesas que
// ainda estão sem valor (categorias raiz sem palavra conhecida ficam VARIABLE; subcategorias
// sem palavra conhecida continuam herdando do pai).
func Bootstrap(ctx context.Context, s *Store) error {
	var n int
	if err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM categories`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := EnsureDefaultCategories(ctx, s); err != nil {
			return err
		}
	}
	values, err := s.SettingValues(ctx)
	if err != nil {
		return err
	}
	if _, done := values[spendBackfillKey]; done {
		return nil
	}
	if err := BackfillSpendTypes(ctx, s); err != nil {
		return err
	}
	return s.SaveSettingValues(ctx, map[string]string{spendBackfillKey: "true"})
}

// BackfillSpendTypes preenche spend_type NULL das despesas pela heurística de nome.
func BackfillSpendTypes(ctx context.Context, s *Store) error {
	expense := domain.CategoryExpense
	cats, err := s.ListCategories(ctx, &expense)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range cats {
		if c.SpendType != nil {
			continue
		}
		guess := domain.GuessSpendType(c.Name)
		if guess == nil {
			if c.ParentID != nil {
				continue
			}
			v := domain.SpendVariable
			guess = &v
		}
		if _, err := tx.Exec(ctx, `UPDATE categories SET spend_type = ? WHERE id = ? AND spend_type IS NULL`, string(*guess), c.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
