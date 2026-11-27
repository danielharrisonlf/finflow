package store

import (
	"context"
	"time"

	"finflow/internal/domain"
)

// ───────────────────────── Seeds dos espaços Casa e Empresa (v3 fase 2) ─────────────────────────

// seedClock devolve hoje (fuso de negócio) e um gerador de datas "dia X do mês corrente + offset";
// no mês corrente o dia é limitado a hoje, para não criar lançamentos confirmados no futuro.
func seedClock() (time.Time, func(monthOffset, day int) string) {
	now := domain.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthDay := func(monthOffset, day int) string {
		if monthOffset == 0 && day > today.Day() {
			day = today.Day()
		}
		return domain.ToDateString(time.Date(today.Year(), today.Month()+time.Month(monthOffset), day, 0, 0, 0, 0, time.UTC))
	}
	return today, monthDay
}

// SeedHome popula um espaço Casa: 2 membros, conta e cartão da casa, contas fixas divididas
// (iguais e 60/40) nos últimos 3 meses, uma compra parcelada dividida, um acerto e uma meta.
// Idempotente (não roda se já houver contas).
func SeedHome(ctx context.Context, s *Store) (bool, error) {
	count, err := s.CountAccounts(ctx)
	if err != nil || count > 0 {
		return false, err
	}
	today, monthDay := seedClock()
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
	ana, err := s.CreateMember(ctx, "Ana", "#ec4899")
	if err != nil {
		return false, err
	}
	bruno, err := s.CreateMember(ctx, "Bruno", "#3b82f6")
	if err != nil {
		return false, err
	}
	conta, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta da casa", Type: domain.AccountChecking, InitialBalanceCents: 300000, Color: strPtr("#f59e0b")})
	if err != nil {
		return false, err
	}
	cartao, err := s.CreateCard(ctx, domain.CreateCardInput{Name: "Cartão da casa", Brand: domain.BrandVisa, LimitCents: 800000, ClosingDay: 5, DueDay: 12, Color: strPtr("#f97316")})
	if err != nil {
		return false, err
	}

	sixtyForty := []domain.SplitShare{{MemberID: ana, ShareBP: 6000}, {MemberID: bruno, ShareBP: 4000}}
	bills := []struct {
		desc, category string
		cents          int64
		day            int
		paidBy         string
		splits         []domain.SplitShare
	}{
		{"Aluguel", "Aluguel", 220000, 5, ana, nil},
		{"Condomínio", "Condomínio", 65000, 8, bruno, nil},
		{"Energia", "Energia", 18500, 10, ana, nil},
		{"Internet", "Internet", 11990, 12, bruno, nil},
		{"Supermercado do mês", "Supermercado", 92000, 15, bruno, sixtyForty},
		{"Feira", "Supermercado", 14500, 20, ana, sixtyForty},
	}
	// Cada pessoa deposita a sua parte na conta da casa no começo do mês (proporção 60/40 da renda).
	contributions := []struct {
		desc  string
		cents int64
	}{{"Aporte da Ana", 260000}, {"Aporte do Bruno", 180000}}
	for _, m := range []int{-2, -1, 0} {
		if m == 0 && today.Day() < 3 {
			break
		}
		for _, c := range contributions {
			if _, err := s.CreateTransaction(ctx, domain.TxIncome, c.desc, c.cents, monthDay(m, 3), domain.TxConfirmed, conta.ID, catPtr("Outras receitas"), nil); err != nil {
				return false, err
			}
		}
	}
	for _, m := range []int{-2, -1, 0} {
		for _, b := range bills {
			date := monthDay(m, b.day)
			if m == 0 && b.day > today.Day() {
				continue
			}
			tx, err := s.CreateTransaction(ctx, domain.TxExpense, b.desc, -b.cents, date, domain.TxConfirmed, conta.ID, catPtr(b.category), nil)
			if err != nil {
				return false, err
			}
			if err := s.SetTransactionSharing(ctx, tx.ID, true, strPtr(b.paidBy), true, b.splits); err != nil {
				return false, err
			}
		}
	}

	// Sofá em 6x no cartão, pago pelo Bruno e dividido igualmente.
	start := monthDay(-1, 18)
	basePeriod := domain.InvoicePeriodForPurchase(start, 5)
	amounts, err := domain.SplitInstallments(360000, 6)
	if err != nil {
		return false, err
	}
	legs := make([]CardExpenseLeg, len(amounts))
	for i, amt := range amounts {
		n, t := i+1, len(amounts)
		legs[i] = CardExpenseLeg{
			Description: "Sofá da sala", AmountCents: amt, Date: domain.AddMonthsToDateString(start, i),
			CategoryID: catPtr("Manutenção da casa"), InstallmentNumber: &n, InstallmentTotal: &t,
			InvoicePeriod: domain.AddMonthsToPeriod(basePeriod, i), PaidByMemberID: strPtr(bruno),
		}
	}
	if _, err := s.CreateCardExpense(ctx, cartao.ID, legs); err != nil {
		return false, err
	}

	if _, err := s.CreateSettlement(ctx, ana, bruno, 15000, monthDay(-1, 28), nil); err != nil {
		return false, err
	}
	if _, err := s.CreateGoal(ctx, domain.CreateGoalInput{
		Name: "Viagem de férias da casa", Icon: "plane", Color: strPtr("#38bdf8"), TargetCents: 1200000, Deadline: strPtr(monthDay(8, 1)),
	}); err != nil {
		return false, err
	}
	return true, nil
}

// SeedBusiness popula um espaço Empresa: conta PJ, receitas recorrentes de clientes (uma desativada
// no mês, para o churn), despesas fixas e variáveis e os lançamentos dos últimos 3 meses.
// Idempotente (não roda se já houver contas).
func SeedBusiness(ctx context.Context, s *Store) (bool, error) {
	count, err := s.CountAccounts(ctx)
	if err != nil || count > 0 {
		return false, err
	}
	today, monthDay := seedClock()
	cat, err := EnsureBusinessCategories(ctx, s)
	if err != nil {
		return false, err
	}
	catPtr := func(name string) *string {
		if id, ok := cat[name]; ok {
			return strPtr(id)
		}
		return nil
	}
	pj, err := s.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta PJ", Type: domain.AccountChecking, InitialBalanceCents: 4500000, Color: strPtr("#6366f1")})
	if err != nil {
		return false, err
	}

	type rec struct {
		desc, category string
		kind           domain.CategoryKind
		cents          int64
		day            int
		startMonth     int
		deactivated    bool
	}
	recs := []rec{
		{"Cliente Alfa — plano anual pago mensal", "Assinaturas de clientes", domain.CategoryIncome, 490000, 5, -6, false},
		{"Cliente Beta — suporte", "Assinaturas de clientes", domain.CategoryIncome, 320000, 10, -4, false},
		{"Cliente Delta — plano básico", "Assinaturas de clientes", domain.CategoryIncome, 250000, 15, -5, true},
		{"Cliente Gama — plano básico", "Assinaturas de clientes", domain.CategoryIncome, 180000, 1, 0, false},
		{"Pró-labore", "Pró-labore", domain.CategoryExpense, 800000, 5, -6, false},
		{"Aluguel do escritório", "Aluguel", domain.CategoryExpense, 350000, 10, -6, false},
		{"Contabilidade", "Contabilidade", domain.CategoryExpense, 90000, 10, -6, false},
		{"Ferramentas SaaS", "Software e ferramentas", domain.CategoryExpense, 45000, 12, -6, false},
		{"Servidores cloud", "Infraestrutura/cloud", domain.CategoryExpense, 120000, 20, -6, false},
	}
	for _, r := range recs {
		row := RecurringRow{
			Description: r.desc, Kind: r.kind, AmountCents: r.cents, DayOfMonth: r.day,
			AccountID: strPtr(pj.ID), CategoryID: catPtr(r.category),
			StartDate: monthDay(r.startMonth, 1), Active: !r.deactivated,
		}
		if r.deactivated {
			row.DeactivatedAt = strPtr(monthDay(0, 1))
		}
		if _, err := s.CreateRecurring(ctx, row); err != nil {
			return false, err
		}
		txType := domain.TxExpense
		if r.kind == domain.CategoryIncome {
			txType = domain.TxIncome
		}
		for _, m := range []int{-3, -2, -1, 0} {
			if m < r.startMonth || (m == 0 && (r.deactivated || r.day > today.Day())) {
				continue
			}
			if _, err := s.CreateTransaction(ctx, txType, r.desc, domain.SignedAmount(txType, r.cents), monthDay(m, r.day), domain.TxConfirmed, pj.ID, catPtr(r.category), nil); err != nil {
				return false, err
			}
		}
	}

	// Despesas variáveis e impostos (6% do faturado no mês anterior) dos últimos 3 meses.
	extras := []struct {
		desc, category string
		cents          int64
		day            int
	}{
		{"Impostos (Simples Nacional)", "Impostos sobre vendas", 74400, 20},
		{"Campanha de anúncios", "Marketing", 150000, 8},
		{"Tarifa de manutenção da conta", "Tarifas bancárias", 4500, 2},
		{"Comissão de indicação", "Comissões", 49000, 25},
	}
	for _, m := range []int{-3, -2, -1, 0} {
		for _, e := range extras {
			if m == 0 && e.day > today.Day() {
				continue
			}
			if _, err := s.CreateTransaction(ctx, domain.TxExpense, e.desc, -e.cents, monthDay(m, e.day), domain.TxConfirmed, pj.ID, catPtr(e.category), nil); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
