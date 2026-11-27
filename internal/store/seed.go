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
	todayStr := domain.ToDateString(today)
	d := func(daysOffset int) string {
		return domain.ToDateString(today.AddDate(0, 0, daysOffset))
	}
	monthDay := func(monthOffset, day int) string {
		t := time.Date(today.Year(), today.Month()+time.Month(monthOffset), day, 0, 0, 0, 0, time.UTC)
		return domain.ToDateString(t)
	}

	cat, err := EnsureDefaultCategories(ctx, s)
	if err != nil {
		return false, err
	}
	catPtr := func(name string) *string {
		if id, ok := cat[name]; ok {
			return strPtr(id)
		}
		return nil
	}

	nubank, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Nubank", Type: domain.AccountChecking, InitialBalanceCents: 142550, Color: strPtr("#8a05be")})
	if err != nil {
		return false, err
	}
	itau, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Itaú Salário", Type: domain.AccountSalary, InitialBalanceCents: 35000, Color: strPtr("#ec7000")})
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
	caixinha, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Caixinha Reserva", Type: domain.AccountSavings, InitialBalanceCents: 600000, Color: strPtr("#10b981")})
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

	// Recorrências (contas fixas e salário): os meses anteriores já foram lançados e o mês
	// corrente só quando o dia da ocorrência já chegou — o resto aparece na projeção.
	recurrings := []struct {
		desc     string
		kind     domain.CategoryKind
		cents    int64
		day      int
		account  string
		category string
	}{
		{"Salário", domain.CategoryIncome, 985000, 5, itau.ID, "Salário"},
		{"Aluguel", domain.CategoryExpense, 220000, 10, nubank.ID, "Aluguel"},
		{"Condomínio", domain.CategoryExpense, 45000, 10, nubank.ID, "Condomínio"},
		{"Energia elétrica", domain.CategoryExpense, 21500, 12, nubank.ID, "Energia"},
		{"Internet fibra", domain.CategoryExpense, 11990, 15, nubank.ID, "Internet"},
		{"Academia", domain.CategoryExpense, 14990, 8, nubank.ID, "Academia"},
	}
	for _, r := range recurrings {
		id, err := s.CreateRecurring(ctx, RecurringRow{
			Description: r.desc, Kind: r.kind, AmountCents: r.cents, DayOfMonth: r.day,
			AccountID: strPtr(r.account), CategoryID: catPtr(r.category),
			StartDate: monthDay(-2, 1), Active: true,
		})
		if err != nil {
			return false, err
		}
		txType := domain.TxExpense
		if r.kind == domain.CategoryIncome {
			txType = domain.TxIncome
		}
		for _, m := range []int{-2, -1, 0} {
			date := monthDay(m, r.day)
			if date > todayStr {
				continue
			}
			amount := r.cents
			if r.category == "Energia" {
				amount += int64(m) * 1200
			}
			if _, err := s.CreateRecurringTransaction(ctx, id, txType, r.desc, domain.SignedAmount(txType, amount), date, r.account, catPtr(r.category)); err != nil {
				return false, err
			}
		}
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
			seedTx{domain.TxExpense, "Mercado — compra do mês", 78400 + int64(m)*3500, monthDay(m, 7), nubank.ID, "Supermercado", domain.TxConfirmed},
		)
	}
	txs = append(txs,
		seedTx{domain.TxExpense, "Restaurante — aniversário", 18000, monthDay(-2, 20), nubank.ID, "Restaurantes", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Restaurante — happy hour", 21000, monthDay(-1, 22), nubank.ID, "Restaurantes", domain.TxConfirmed},
		seedTx{domain.TxIncome, "Freelance — app mobile", 320000, monthDay(-1, 18), nubank.ID, "Freelance", domain.TxConfirmed},
		seedTx{domain.TxIncome, "Dividendos FIIs", 18450, monthDay(0, 15), xp.ID, "Rendimentos", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Uber — semana", 8740, d(-6), nubank.ID, "Aplicativos de transporte", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Padaria", 3250, d(-3), carteira.ID, "Alimentação", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Almoço — rodízio japonês", 12900, d(-2), nubank.ID, "Restaurantes", domain.TxConfirmed},
		seedTx{domain.TxExpense, "Cinema IMAX", 9800, d(-1), nubank.ID, "Lazer", domain.TxConfirmed},
		seedTx{domain.TxExpense, "IPVA parcela 2/3", 48700, d(5), nubank.ID, "IPVA e licenciamento", domain.TxPending},
		seedTx{domain.TxIncome, "Reembolso plano de saúde", 22000, d(4), itau.ID, "Reembolsos", domain.TxPending},
	)

	for _, t := range txs {
		if _, err := s.CreateTransaction(
			ctx, t.txType, t.description, domain.SignedAmount(t.txType, t.amountCents), t.date, t.status, t.accountID, catPtr(t.category), nil,
		); err != nil {
			return false, err
		}
	}

	for _, m := range []int{-2, -1, 0} {
		if _, err := s.CreateTransferPair(
			ctx, itau.ID, nubank.ID, 700000, monthDay(m, 6),
			"Transferência → Nubank",
			"Transferência ← Itaú Salário",
		); err != nil {
			return false, err
		}
		if _, err := s.CreateTransferPair(
			ctx, nubank.ID, xp.ID, 300000, monthDay(m, 6),
			"Aporte mensal → XP Investimentos",
			"Aporte mensal ← Nubank",
		); err != nil {
			return false, err
		}
		if _, err := s.CreateTransferPair(
			ctx, itau.ID, caixinha.ID, 100000, monthDay(m, 6),
			"Guardar na reserva → Caixinha Reserva",
			"Guardar na reserva ← Itaú Salário",
		); err != nil {
			return false, err
		}
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
		catID := catPtr(category)
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
		{nuCard, "Notebook Dell Inspiron", 350000, monthDay(-2, 14), "Eletrônicos", 10},
		{nuCard, "iFood — jantar", 6890, d(-4), "Delivery", 1},
		{nuCard, "iFood — pizza", 7450, monthDay(-1, 12), "Delivery", 1},
		{nuCard, "Spotify Família", 3490, d(-9), "Música", 1},
		{nuCard, "Netflix", 5590, d(-11), "Streaming", 1},
		{nuCard, "Posto Shell — gasolina", 28000, d(-7), "Combustível", 1},
		{interCard, "Passagens GOL — feriado", 184000, monthDay(0, 2), "Viagens", 4},
		{interCard, "Farmácia Droga Raia", 11230, d(-5), "Farmácia", 1},
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
		{"Supermercado", 90000},
		{"Restaurantes", 60000},
		{"Combustível", 50000},
		{"Lazer", 60000},
		{"Streaming", 10000},
	}
	for _, b := range budgets {
		if catID, ok := cat[b.name]; ok {
			if _, err := s.UpsertBudget(ctx, catID, b.cents); err != nil {
				return false, err
			}
		}
	}

	reserva, err := s.CreateGoal(ctx, domain.CreateGoalInput{
		Name: "Reserva de emergência", Kind: domain.GoalKindReserve, Icon: "shield", Color: strPtr("#10b981"), TargetCents: 3000000, Deadline: strPtr(monthDay(14, 1)),
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

	rules := []struct {
		pattern  string
		category string
		priority int
	}{
		{"uber", "Aplicativos de transporte", 0},
		{"99 pop", "Aplicativos de transporte", 0},
		{"ifood", "Delivery", 0},
		{"posto", "Combustível", 0},
		{"netflix", "Streaming", 0},
		{"spotify", "Música", 0},
		{"farmácia", "Farmácia", 0},
		{"drogaria", "Farmácia", 0},
		{"mercado", "Supermercado", 0},
		{"mercado livre", "Compras", 10},
		{"salário", "Salário", 0},
	}
	for _, r := range rules {
		if catID, ok := cat[r.category]; ok {
			if _, err := s.CreateCategoryRule(ctx, r.pattern, catID, r.priority); err != nil {
				return false, err
			}
		}
	}

	wishes := []WishItemRow{
		{Name: "Air fryer", PriceCents: 49900, Notes: strPtr("a antiga quebrou"), Priority: 1, Status: domain.WishListed},
		{Name: "Fone com cancelamento de ruído", PriceCents: 129900, URL: strPtr("https://example.com/fone"), Priority: 2, Status: domain.WishListed},
	}
	for _, w := range wishes {
		if _, err := s.CreateWishItem(ctx, w); err != nil {
			return false, err
		}
	}

	if err := seedWealth(ctx, s, todayStr, monthDay, xp.ID, itau.ID, catPtr(domain.LoanCategoryName)); err != nil {
		return false, err
	}

	if err := s.SaveSettingValues(ctx, map[string]string{
		SettingMonthlyLimit: "850000",
		SettingSpendPlan:    `{"fixedPct":50,"cuttablePct":10,"variablePct":40}`,
	}); err != nil {
		return false, err
	}

	return true, nil
}

// seedWealth cria duas posições na conta "XP Investimentos" com snapshots dos últimos 6 meses e
// um financiamento de veículo com 7 parcelas pagas antes do app e as dos últimos meses lançadas.
func seedWealth(ctx context.Context, s *Store, todayStr string, monthDay func(monthOffset, day int) string, xpID, payerID string, loanCategoryID *string) error {
	snapDate := func(m int) string {
		return min(monthDay(m, 28), todayStr)
	}
	positions := []struct {
		name  string
		kind  domain.InvestmentKind
		base  int64
		step  int64
		yield []int64 // rentabilidade acumulada (milésimos) em cada snapshot
		notes *string
	}{
		{"Tesouro Selic 2029", domain.InvestmentTreasury, 1400000, 120000, []int64{9, 18, 27, 36, 45, 54}, strPtr("reserva de longo prazo")},
		{"Fundo imobiliário de logística", domain.InvestmentFII, 1350000, 50000, []int64{-8, 4, 12, 20, 31, 46}, nil},
	}
	for _, p := range positions {
		snaps := make([]domain.InvestmentSnapshot, 0, len(p.yield))
		for i, y := range p.yield {
			invested := p.base + int64(i)*p.step
			snaps = append(snaps, domain.InvestmentSnapshot{
				Date:          snapDate(i - len(p.yield) + 1),
				ValueCents:    invested * (1000 + y) / 1000,
				InvestedCents: invested,
			})
		}
		last := snaps[len(snaps)-1]
		id, err := s.CreateInvestment(ctx, InvestmentRow{
			Name: p.name, Kind: p.kind, Institution: strPtr("XP Investimentos"), AccountID: strPtr(xpID),
			InvestedCents: last.InvestedCents, CurrentCents: last.ValueCents, Notes: p.notes, ValueUpdatedAt: strPtr(last.Date),
		}, nil)
		if err != nil {
			return err
		}
		for _, snap := range snaps {
			if err := upsertSnapshot(ctx, s.DB, id, snap); err != nil {
				return err
			}
		}
	}

	// Financiamento de R$ 48.000 a 1,49% a.m. em 48x: parcelas 1–7 pagas antes de usar o app;
	// 8, 9 e 10 (meses −2, −1 e o corrente, se o dia 15 já passou) lançadas com loan_id.
	const (
		principal = 4800000
		rateBP    = 149
		total     = 48
		basePaid  = 7
	)
	installment := domain.PriceInstallment(principal, rateBP, total)
	loanID, err := s.CreateLoan(ctx, LoanRow{
		Name: "Financiamento do carro", Lender: strPtr("Financeira (exemplo)"), Kind: domain.LoanVehicle,
		PrincipalCents: principal, RateBP: rateBP, InstallmentCents: installment, TotalInstallments: total,
		BasePaid: basePaid, FirstDueDate: monthDay(-9, 15), AccountID: strPtr(payerID), Active: true,
	})
	if err != nil {
		return err
	}
	for i, m := range []int{-2, -1, 0} {
		date := monthDay(m, 15)
		if date > todayStr {
			continue
		}
		desc := fmt.Sprintf("Financiamento do carro — parcela %d/%d", basePaid+i+1, total)
		if _, err := s.CreateLoanPayment(ctx, loanID, desc, installment, date, payerID, loanCategoryID); err != nil {
			return err
		}
	}
	return nil
}
