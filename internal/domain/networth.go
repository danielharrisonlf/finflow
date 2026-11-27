package domain

// ───────────────────────── Patrimônio (v3 §10–§12) ─────────────────────────

// NetWorthPoint é o patrimônio no fim de um mês (GET /api/insights/net-worth).
type NetWorthPoint struct {
	Month            int   `json:"month"`
	Year             int   `json:"year"`
	CashCents        int64 `json:"cashCents"`
	InvestmentsCents int64 `json:"investmentsCents"`
	DebtsCents       int64 `json:"debtsCents"`
	NetWorthCents    int64 `json:"netWorthCents"`
}

// NetWorthAccount é o saldo de uma conta ativa numa data.
type NetWorthAccount struct {
	ID           string
	Type         AccountType
	BalanceCents int64
}

// NetWorthPosition é o valor de uma posição de investimento numa data; AccountID aponta a
// conta INVESTMENT vinculada (nil = posição avulsa).
type NetWorthPosition struct {
	AccountID  *string
	ValueCents int64
}

// NetWorthBreakdown separa o patrimônio em caixa, investimentos e dívidas.
type NetWorthBreakdown struct {
	CashCents        int64
	InvestmentsCents int64
	DebtsCents       int64
}

// NetWorthCents = caixa + investimentos − dívidas.
func (b NetWorthBreakdown) NetWorthCents() int64 {
	return b.CashCents + b.InvestmentsCents - b.DebtsCents
}

// ComposeNetWorth soma o patrimônio: contas não-INVESTMENT entram como caixa; cada conta
// INVESTMENT com posições vinculadas vale a soma das posições (no lugar do saldo contábil,
// que só registra o dinheiro que entrou/saiu); sem posições, vale o saldo contábil. Posições
// sem conta (ou ligadas a uma conta que não está na lista, p.ex. arquivada) somam direto.
// debtsCents já vem somado (faturas não pagas + saldo devedor de empréstimos).
func ComposeNetWorth(accounts []NetWorthAccount, positions []NetWorthPosition, debtsCents int64) NetWorthBreakdown {
	out := NetWorthBreakdown{DebtsCents: debtsCents}
	investAccounts := map[string]bool{}
	for _, a := range accounts {
		if a.Type == AccountInvestment {
			investAccounts[a.ID] = true
		}
	}
	linked := map[string]bool{}
	for _, p := range positions {
		out.InvestmentsCents += p.ValueCents
		if p.AccountID != nil && investAccounts[*p.AccountID] {
			linked[*p.AccountID] = true
		}
	}
	for _, a := range accounts {
		switch {
		case a.Type != AccountInvestment:
			out.CashCents += a.BalanceCents
		case !linked[a.ID]:
			out.InvestmentsCents += a.BalanceCents
		}
	}
	return out
}
