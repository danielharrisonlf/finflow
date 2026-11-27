package domain

import (
	"math"
	"sort"
)

// ───────────────────────── Empréstimos e financiamentos (v3 §11) ─────────────────────────

type LoanKind string

const (
	LoanPersonal  LoanKind = "PERSONAL"
	LoanPayroll   LoanKind = "PAYROLL"
	LoanVehicle   LoanKind = "VEHICLE"
	LoanMortgage  LoanKind = "MORTGAGE"
	LoanStudent   LoanKind = "STUDENT"
	LoanCardDebt  LoanKind = "CARD_DEBT"
	LoanOverdraft LoanKind = "OVERDRAFT"
	LoanOther     LoanKind = "OTHER"
)

func (k LoanKind) Valid() bool {
	switch k {
	case LoanPersonal, LoanPayroll, LoanVehicle, LoanMortgage, LoanStudent, LoanCardDebt, LoanOverdraft, LoanOther:
		return true
	}
	return false
}

// LoanCategoryName é a categoria padrão (FIXED) dos pagamentos sem categoria escolhida.
const LoanCategoryName = "Empréstimos e financiamentos"

type Loan struct {
	ID                          string   `json:"id"`
	Name                        string   `json:"name"`
	Lender                      *string  `json:"lender"`
	Kind                        LoanKind `json:"kind"`
	PrincipalCents              int64    `json:"principalCents"`
	MonthlyRatePct              float64  `json:"monthlyRatePct"`
	InstallmentCents            int64    `json:"installmentCents"`
	TotalInstallments           int      `json:"totalInstallments"`
	PaidInstallments            int      `json:"paidInstallments"`
	RemainingInstallments       int      `json:"remainingInstallments"`
	FirstDueDate                string   `json:"firstDueDate"`
	NextDueDate                 *string  `json:"nextDueDate"`
	OutstandingCents            int64    `json:"outstandingCents"`
	TotalInterestRemainingCents int64    `json:"totalInterestRemainingCents"`
	AccountID                   *string  `json:"accountId"`
	CategoryID                  *string  `json:"categoryId"`
	Active                      bool     `json:"active"`
	Notes                       *string  `json:"notes"`
}

// CreateLoanInput.InstallmentCents omitido = parcela da tabela Price (principal, taxa, prazo).
type CreateLoanInput struct {
	Name              string   `json:"name"`
	Lender            *string  `json:"lender,omitempty"`
	Kind              LoanKind `json:"kind"`
	PrincipalCents    int64    `json:"principalCents"`
	MonthlyRatePct    float64  `json:"monthlyRatePct"`
	InstallmentCents  *int64   `json:"installmentCents,omitempty"`
	TotalInstallments int      `json:"totalInstallments"`
	PaidInstallments  int      `json:"paidInstallments"`
	FirstDueDate      string   `json:"firstDueDate"`
	AccountID         *string  `json:"accountId,omitempty"`
	CategoryID        *string  `json:"categoryId,omitempty"`
	Notes             *string  `json:"notes,omitempty"`
}

type UpdateLoanInput struct {
	Name              Optional[string]   `json:"name"`
	Lender            Optional[string]   `json:"lender"`
	Kind              Optional[LoanKind] `json:"kind"`
	PrincipalCents    Optional[int64]    `json:"principalCents"`
	MonthlyRatePct    Optional[float64]  `json:"monthlyRatePct"`
	InstallmentCents  Optional[int64]    `json:"installmentCents"`
	TotalInstallments Optional[int]      `json:"totalInstallments"`
	PaidInstallments  Optional[int]      `json:"paidInstallments"`
	FirstDueDate      Optional[string]   `json:"firstDueDate"`
	AccountID         Optional[string]   `json:"accountId"`
	CategoryID        Optional[string]   `json:"categoryId"`
	Active            Optional[bool]     `json:"active"`
	Notes             Optional[string]   `json:"notes"`
}

type PayLoanInput struct {
	Date        *string `json:"date,omitempty"`
	AmountCents *int64  `json:"amountCents,omitempty"`
	AccountID   *string `json:"accountId,omitempty"`
}

type DebtPlanLoan struct {
	LoanID        string `json:"loanId"`
	PayoffDate    string `json:"payoffDate"`
	InterestCents int64  `json:"interestCents"`
}

type DebtPlan struct {
	Order              []string       `json:"order"`
	Months             int            `json:"months"`
	TotalInterestCents int64          `json:"totalInterestCents"`
	PayoffDate         string         `json:"payoffDate"`
	PerLoan            []DebtPlanLoan `json:"perLoan"`
}

type DebtStrategy struct {
	Avalanche   DebtPlan `json:"avalanche"`
	Snowball    DebtPlan `json:"snowball"`
	MinimumOnly DebtPlan `json:"minimumOnly"`
}

// ───────────────────────── Taxa ─────────────────────────

// RatePctToBP converte % ao mês (1.99) para centésimos de ponto percentual (199), arredondando.
func RatePctToBP(pct float64) int {
	return int(math.Round(pct * 100))
}

// RateBPToPct é o inverso de RatePctToBP (199 → 1.99).
func RateBPToPct(bp int) float64 {
	return float64(bp) / 100
}

// roundHalfUp arredonda para o inteiro mais próximo (0,5 para cima, em módulo).
func roundHalfUp(v float64) int64 {
	if v < 0 {
		return -int64(math.Floor(-v + 0.5))
	}
	return int64(math.Floor(v + 0.5))
}

// interestCents são os juros de um mês sobre o saldo, em aritmética inteira com
// arredondamento meio para cima: saldo × bp ÷ 10000.
func interestCents(balance int64, rateBP int) int64 {
	if balance <= 0 || rateBP <= 0 {
		return 0
	}
	return (balance*int64(rateBP) + 5000) / 10000
}

// ───────────────────────── Tabela Price ─────────────────────────

// PriceInstallment é a parcela fixa da tabela Price: P·i ÷ (1 − (1+i)^−n), arredondada para
// centavos (meio para cima). Sem juros, divide o principal (a última parcela absorve o resto
// na tabela de amortização).
func PriceInstallment(principalCents int64, rateBP, n int) int64 {
	if n <= 0 || principalCents <= 0 {
		return 0
	}
	if rateBP <= 0 {
		return (principalCents + int64(n) - 1) / int64(n)
	}
	i := float64(rateBP) / 10000
	return roundHalfUp(float64(principalCents) * i / (1 - math.Pow(1+i, -float64(n))))
}

// AmortizationRow é uma linha da tabela de amortização.
type AmortizationRow struct {
	Number            int   `json:"number"`
	PaymentCents      int64 `json:"paymentCents"`
	InterestCents     int64 `json:"interestCents"`
	AmortizationCents int64 `json:"amortizationCents"`
	BalanceCents      int64 `json:"balanceCents"`
}

// PriceSchedule monta a tabela Price com arredondamento a cada passo: juros = saldo × i
// (meio para cima), amortização = parcela − juros; a última parcela quita o saldo restante
// (absorve o resíduo dos arredondamentos).
func PriceSchedule(principalCents int64, rateBP, n int) []AmortizationRow {
	if n <= 0 || principalCents <= 0 {
		return nil
	}
	pmt := PriceInstallment(principalCents, rateBP, n)
	balance := principalCents
	rows := make([]AmortizationRow, 0, n)
	for k := 1; k <= n; k++ {
		interest := interestCents(balance, rateBP)
		amort := pmt - interest
		if k == n || amort > balance {
			amort = balance
		}
		balance -= amort
		rows = append(rows, AmortizationRow{
			Number:            k,
			PaymentCents:      interest + amort,
			InterestCents:     interest,
			AmortizationCents: amort,
			BalanceCents:      balance,
		})
		if balance == 0 {
			break
		}
	}
	return rows
}

// LoanOutstandingCents é o saldo devedor: valor presente das `remaining` parcelas restantes
// descontadas à taxa mensal (Price), arredondado para centavos; sem taxa, a soma das parcelas.
func LoanOutstandingCents(installmentCents int64, rateBP, remaining int) int64 {
	if remaining <= 0 || installmentCents <= 0 {
		return 0
	}
	if rateBP <= 0 {
		return installmentCents * int64(remaining)
	}
	i := float64(rateBP) / 10000
	return roundHalfUp(float64(installmentCents) * (1 - math.Pow(1+i, -float64(remaining))) / i)
}

// LoanDueDate é o vencimento da parcela `number` (1 = firstDueDate); o dia é mantido e
// limitado ao último dia do mês.
func LoanDueDate(firstDueDate string, number int) string {
	return AddMonthsToDateString(firstDueDate, number-1)
}

// LoanNextDueDate é o vencimento da próxima parcela não paga; nil se inativo ou quitado.
func LoanNextDueDate(firstDueDate string, paidInstallments, totalInstallments int, active bool) *string {
	if !active || paidInstallments >= totalInstallments {
		return nil
	}
	d := LoanDueDate(firstDueDate, paidInstallments+1)
	return &d
}

// LoanSchedule é o mínimo de um empréstimo para calcular parcelas e saldo.
type LoanSchedule struct {
	InstallmentCents  int64
	RateBP            int
	TotalInstallments int
	PaidInstallments  int
	FirstDueDate      string
	Active            bool
}

// Remaining devolve as parcelas que faltam pagar (>= 0).
func (l LoanSchedule) Remaining() int {
	return max(0, l.TotalInstallments-l.PaidInstallments)
}

// OutstandingCents é o saldo devedor atual (0 se inativo).
func (l LoanSchedule) OutstandingCents() int64 {
	if !l.Active {
		return 0
	}
	return LoanOutstandingCents(l.InstallmentCents, l.RateBP, l.Remaining())
}

// UnpaidDueDates lista os vencimentos das parcelas ainda não pagas (vazio se inativo).
func (l LoanSchedule) UnpaidDueDates() []string {
	if !l.Active {
		return nil
	}
	out := make([]string, 0, l.Remaining())
	for k := l.PaidInstallments + 1; k <= l.TotalInstallments; k++ {
		out = append(out, LoanDueDate(l.FirstDueDate, k))
	}
	return out
}

// InstallmentIn devolve o número da parcela que vence no mês (0 se nenhuma).
func (l LoanSchedule) InstallmentIn(year, month int) int {
	first := FromDateString(l.FirstDueDate)
	k := (year-first.Year())*12 + month - int(first.Month()) + 1
	if k < 1 || k > l.TotalInstallments {
		return 0
	}
	return k
}

// OutstandingAt é o saldo devedor em `date` sabendo quantas parcelas estavam pagas nessa data.
// Antes da contratação (um mês antes do primeiro vencimento) não há dívida.
func (l LoanSchedule) OutstandingAt(date string, paidAtDate int) int64 {
	if !l.Active || date < AddMonthsToDateString(l.FirstDueDate, -1) {
		return 0
	}
	return LoanOutstandingCents(l.InstallmentCents, l.RateBP, max(0, l.TotalInstallments-paidAtDate))
}

// ───────────────────────── Estratégias de quitação ─────────────────────────

// DebtInput é uma dívida em aberto para a simulação de quitação.
type DebtInput struct {
	ID               string
	BalanceCents     int64 // saldo devedor atual
	RateBP           int
	InstallmentCents int64
	Remaining        int    // parcelas restantes
	NextDueDate      string // vencimento da próxima parcela (mês 1 da simulação)
}

// maxDebtMonths limita a simulação (dívidas cuja parcela não cobre os juros nunca acabam).
const maxDebtMonths = 1200

type debtStrategyKind int

const (
	strategyMinimum debtStrategyKind = iota
	strategyAvalanche
	strategySnowball
)

// SimulateDebtStrategies compara três planos mês a mês: todas as dívidas pagam a parcela;
// em avalanche o extra (e as parcelas liberadas pelas dívidas já quitadas) vai para a de
// maior taxa, em bola de neve para a de menor saldo; minimumOnly só paga as parcelas.
func SimulateDebtStrategies(debts []DebtInput, extraCents int64) DebtStrategy {
	return DebtStrategy{
		Avalanche:   simulateDebtPlan(debts, extraCents, strategyAvalanche),
		Snowball:    simulateDebtPlan(debts, extraCents, strategySnowball),
		MinimumOnly: simulateDebtPlan(debts, 0, strategyMinimum),
	}
}

func debtPriority(debts []DebtInput, kind debtStrategyKind) []int {
	idx := make([]int, len(debts))
	for i := range debts {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		da, db := debts[idx[a]], debts[idx[b]]
		switch kind {
		case strategyAvalanche:
			if da.RateBP != db.RateBP {
				return da.RateBP > db.RateBP
			}
			return da.BalanceCents < db.BalanceCents
		case strategySnowball:
			if da.BalanceCents != db.BalanceCents {
				return da.BalanceCents < db.BalanceCents
			}
			return da.RateBP > db.RateBP
		}
		return false
	})
	return idx
}

func simulateDebtPlan(input []DebtInput, extraCents int64, kind debtStrategyKind) DebtPlan {
	var debts []DebtInput
	for _, d := range input {
		if d.BalanceCents > 0 && d.NextDueDate != "" {
			debts = append(debts, d)
		}
	}
	plan := DebtPlan{Order: []string{}, PerLoan: []DebtPlanLoan{}}
	if len(debts) == 0 {
		plan.PayoffDate = TodayString()
		return plan
	}
	extraCents = max(0, extraCents)
	n := len(debts)
	balance := make([]int64, n)
	left := make([]int, n)
	interest := make([]int64, n)
	paidOffAt := make([]int, n) // mês (1-based) em que quitou; 0 = em aberto
	for i, d := range debts {
		balance[i] = d.BalanceCents
		left[i] = max(1, d.Remaining)
	}
	priority := debtPriority(debts, kind)

	month := 0
	var freed int64 // parcelas de dívidas já quitadas (só nas estratégias com extra)
	for open := n; open > 0 && month < maxDebtMonths; {
		month++
		pool := int64(0)
		if kind != strategyMinimum {
			pool = extraCents + freed
		}
		var freedNow int64
		for i := range debts {
			if paidOffAt[i] > 0 {
				continue
			}
			in := interestCents(balance[i], debts[i].RateBP)
			interest[i] += in
			balance[i] += in
			pay := debts[i].InstallmentCents
			left[i]--
			// A última parcela prevista quita o saldo (absorve o resíduo dos arredondamentos).
			if left[i] <= 0 || pay > balance[i] {
				pay = balance[i]
			}
			balance[i] -= pay
			if balance[i] == 0 {
				paidOffAt[i] = month
				open--
				freedNow += debts[i].InstallmentCents
			}
		}
		for _, i := range priority {
			if pool <= 0 {
				break
			}
			if paidOffAt[i] > 0 {
				continue
			}
			pay := min(pool, balance[i])
			balance[i] -= pay
			pool -= pay
			if balance[i] == 0 {
				paidOffAt[i] = month
				open--
				freedNow += debts[i].InstallmentCents
			}
		}
		freed += freedNow
	}

	type payoff struct {
		idx, month int
	}
	order := make([]payoff, 0, n)
	for i := range debts {
		m := paidOffAt[i]
		if m == 0 {
			m = month
		}
		order = append(order, payoff{idx: i, month: m})
		plan.TotalInterestCents += interest[i]
		date := AddMonthsToDateString(debts[i].NextDueDate, m-1)
		plan.PerLoan = append(plan.PerLoan, DebtPlanLoan{LoanID: debts[i].ID, PayoffDate: date, InterestCents: interest[i]})
		if date > plan.PayoffDate {
			plan.PayoffDate = date
		}
	}
	plan.Months = month
	if kind == strategyMinimum {
		sort.SliceStable(order, func(a, b int) bool { return order[a].month < order[b].month })
		for _, o := range order {
			plan.Order = append(plan.Order, debts[o.idx].ID)
		}
	} else {
		for _, i := range priority {
			plan.Order = append(plan.Order, debts[i].ID)
		}
	}
	return plan
}
