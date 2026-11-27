package store

import (
	"context"
	"fmt"
	"time"

	"finflow/internal/domain"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// Seed popula o banco com dados realistas de 3 meses. Idempotente (não roda se já houver contas).
func Seed(ctx context.Context, s *Store) (bool, error) {
	count, err := s.CountAccounts(ctx)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}

	now := domain.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	d := func(daysOffset int) string {
		return domain.ToDateString(today.AddDate(0, 0, daysOffset))
	}
	monthDay := func(monthOffset, day int) string {
		t := time.Date(today.Year(), today.Month()+time.Month(monthOffset), day, 0, 0, 0, 0, time.UTC)
		return domain.ToDateString(t)
	}

	expenseCategories := []struct{ name, icon, color string }{
		{"Alimentação", "utensils", "#fb923c"},
		{"Mercado", "shopping-cart", "#fbbf24"},
		{"Moradia", "house", "#38bdf8"},
		{"Transporte", "car", "#a78bfa"},
		{"Lazer", "gamepad-2", "#f472b6"},
		{"Saúde", "heart-pulse", "#34d399"},
		{"Educação", "graduation-cap", "#60a5fa"},
		{"Assinaturas", "repeat", "#94a3b8"},
	}
	incomeCategories := []struct{ name, icon, color string }{
		{"Salário", "briefcase", "#10b981"},
		{"Freelance", "laptop", "#22d3ee"},
		{"Rendimentos", "trending-up", "#84cc16"},
	}

	cat := make(map[string]string)
	for _, c := range expenseCategories {
		created, err := s.CreateCategory(ctx, domain.CreateCategoryInput{
			Name: c.name, Kind: domain.CategoryExpense, Icon: c.icon, Color: c.color,
		})
		if err != nil {
			return false, err
		}
		cat[c.name] = created.ID
	}
	for _, c := range incomeCategories {
		created, err := s.CreateCategory(ctx, domain.CreateCategoryInput{
			Name: c.name, Kind: domain.CategoryIncome, Icon: c.icon, Color: c.color,
		})
		if err != nil {
			return false, err
		}
		cat[c.name] = created.ID
	}

	nubank, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Nubank", Type: domain.AccountChecking, InitialBalanceCents: 412550, Color: strPtr("#8a05be")})
	if err != nil {
		return false, err
	}
	itau, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Itaú Salário", Type: domain.AccountSalary, InitialBalanceCents: 125000, Color: strPtr("#ec7000")})
	if err != nil {
		return false, err
	}
	carteira, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Carteira", Type: domain.AccountWallet, InitialBalanceCents: 18000, Color: strPtr("#64748b")})
	if err != nil {
		return false, err
	}
	xp, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "XP Investimentos", Type: domain.AccountInvestment, InitialBalanceCents: 2850000, Color: strPtr("#facc15")})
	if err != nil {
		return false, err
	}

	nuCard, err := s.CreateCard(ctx, domain.CreateCardInput{
		Name: "Nubank Ultravioleta", Brand: domain.BrandMastercard, LimitCents: 1200000, ClosingDay: 3, DueDay: 10, Color: strPtr("#7c3aed"),
	})
	if err != nil {
		return false, err
	}
	interCard, err := s.CreateCard(ctx, domain.CreateCardInput{
		Name: "Inter Gold", Brand: domain.BrandVisa, LimitCents: 600000, ClosingDay: 20, DueDay: 27, Color: strPtr("#f97316"),
	})
	if err != nil {
		return false, err
	}

	type seedTx struct {
		txType      domain.TransactionType
		description string
		amountCents int64
		date        string
		accountID   string
		category    string
		status      domain.TransactionStatus
	}
	var txs []seedTx
	for _, m := range []int{-2, -1, 0} {
		txs = append(txs,
			seedTx{domain.TxIncome, "Salário", 985000, monthDay(m, 5), itau.ID, "Salário", domain.TxConfirmed},
			seedTx{domain.TxExpense, "Aluguel + condomínio", 265000, monthDay(m, 10), nubank.ID, "Moradia", domain.TxConfirmed},
			seedTx{domain.TxExpense, "Energia elétrica", 21500 + int64(m)*1200, monthDay(m, 12), nubank.ID, "Moradia", domain.TxConfirmed},
			seedTx{domain.TxExpense, "Internet fibra", 11990, monthDay(m, 15), nubank.ID, "Assinaturas", domain.TxConfirmed},
			seedTx{domain.TxExpense, "Mercado — compra do mês", 78400 + int64(m)*3500, monthDay(m, 7), nubank.ID, "Mercado", domain.TxConfirmed},
			seedTx{domain.TxExpense, "Academia", 14990, monthDay(m, 8), nubank.ID, "Saúde", domain.TxConfirmed},
		)
	}
	txs = append(txs,
		seedTx{domain.TxIncome, "Freelance — app mobile", 320000, monthDay(-1, 18), nubank.ID, "Freelance", domain.TxConfirmed},
		seedTx{domain.TxIncome, "Dividendos FIIs", 18450, monthDay(0, 15), xp.ID, "Rendimentos", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Uber — semana", 8740, d(-6), nubank.ID, "Transporte", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Padaria", 3250, d(-3), carteira.ID, "Alimentação", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Almoço — rodízio japonês", 12900, d(-2), nubank.ID, "Alimentação", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Cinema IMAX", 9800, d(-1), nubank.ID, "Lazer", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Fatura internet (vence)", 11990, d(3), nubank.ID, "Assinaturas", domain.TxPending},
		seedTx{domain.TxExpense, "IPVA parcela 2/3", 48700, d(5), nubank.ID, "Transporte", domain.TxPending},
		seedTx{domain.TxIncome, "Reembolso plano de saúde", 22000, d(4), itau.ID, "Saúde", domain.TxPending},
	)

	for _, t := range txs {
		var catID *string
		if id, ok := cat[t.category]; ok {
			catID = strPtr(id)
		}
		if _, err := s.CreateTransaction(
			ctx, t.txType, t.description, domain.SignedAmount(t.txType, t.amountCents), t.date, t.status, t.accountID, catID, nil,
		); err != nil {
			return false, err
		}
	}

	if _, err := s.CreateTransferPair(
		ctx, nubank.ID, xp.ID, 150000, monthDay(0, 6),
		"Aporte mensal → XP Investimentos",
		"Aporte mensal ← Nubank",
	); err != nil {
		return false, err
	}

	addCardExpense := func(card *CardRow, desc string, total int64, dateStr, category string, installments int) error {
		if installments < 1 {
			installments = 1
		}
		amounts, err := domain.SplitInstallments(total, installments)
		if err != nil {
			return err
		}
		basePeriod := domain.InvoicePeriodForPurchase(dateStr, card.ClosingDay)
		var catID *string
		if id, ok := cat[category]; ok {
			catID = strPtr(id)
		}
		legs := make([]CardExpenseLeg, installments)
		for i, amt := range amounts {
			legDesc := desc
			var num, tot *int
			if installments > 1 {
				legDesc = fmt.Sprintf("%s (%d/%d)", desc, i+1, installments)
				num = intPtr(i + 1)
				tot = intPtr(installments)
			}
			legs[i] = CardExpenseLeg{
				Description:       legDesc,
				AmountCents:       amt,
				Date:              domain.AddMonthsToDateString(dateStr, i),
				CategoryID:        catID,
				InstallmentNumber: num,
				InstallmentTotal:  tot,
				InvoicePeriod:     domain.AddMonthsToPeriod(basePeriod, i),
			}
		}
		_, err = s.CreateCardExpense(ctx, card.ID, legs)
		return err
	}

	cardSeeds := []struct {
		card         *CardRow
		desc         string
		total        int64
		date         string
		category     string
		installments int
	}{
		{nuCard, "Notebook Dell Inspiron", 350000, monthDay(-2, 14), "Educação", 10},
		{nuCard, "iFood — jantar", 6890, d(-4), "Alimentação", 1},
		{nuCard, "Spotify Família", 3490, d(-9), "Assinaturas", 1},
		{nuCard, "Posto Shell — gasolina", 28000, d(-7), "Transporte", 1},
		{interCard, "Passagens GOL — feriado", 184000, monthDay(0, 2), "Lazer", 4},
		{interCard, "Farmácia Droga Raia", 11230, d(-5), "Saúde", 1},
	}
	for _, cs := range cardSeeds {
		if err := addCardExpense(cs.card, cs.desc, cs.total, cs.date, cs.category, cs.installments); err != nil {
			return false, err
		}
	}

	budgets := []struct {
		name  string
		cents int64
	}{
		{"Mercado", 90000},
		{"Alimentação", 60000},
		{"Transporte", 70000},
		{"Lazer", 60000},
		{"Assinaturas", 20000},
	}
	for _, b := range budgets {
		if catID, ok := cat[b.name]; ok {
			if _, err := s.UpsertBudget(ctx, catID, b.cents); err != nil {
				return false, err
			}
		}
	}

	reserva, err := s.CreateGoal(ctx, domain.CreateGoalInput{
		Name: "Reserva de emergência", Icon: "shield", Color: strPtr("#10b981"), TargetCents: 3000000, Deadline: strPtr(monthDay(14, 1)),
	})
	if err != nil {
		return false, err
	}
	viagem, err := s.CreateGoal(ctx, domain.CreateGoalInput{
		Name: "Eurotrip 2027", Icon: "plane", Color: strPtr("#38bdf8"), TargetCents: 1800000, Deadline: strPtr(monthDay(9, 1)),
	})
	if err != nil {
		return false, err
	}
	macbook, err := s.CreateGoal(ctx, domain.CreateGoalInput{
		Name: "MacBook Pro", Icon: "laptop", Color: strPtr("#a78bfa"), TargetCents: 1400000,
	})
	if err != nil {
		return false, err
	}

	contributions := []struct {
		goalID string
		cents  int64
		date   string
		note   *string
	}{
		{reserva.ID, 450000, monthDay(-2, 6), strPtr("aporte inicial")},
		{reserva.ID, 300000, monthDay(-1, 6), nil},
		{reserva.ID, 300000, monthDay(0, 6), nil},
		{viagem.ID, 200000, monthDay(-1, 20), strPtr("bônus freelance")},
		{viagem.ID, 120000, monthDay(0, 20), nil},
		{macbook.ID, 150000, monthDay(0, 12), nil},
	}
	for _, c := range contributions {
		if err := s.AddGoalContribution(ctx, c.goalID, c.cents, c.date, c.note); err != nil {
			return false, err
		}
	}

	return true, nil
}
