package domain

import (
	"math"
	"sort"
)

// ───────────────────────── Fluxo de caixa ─────────────────────────

const (
	CashflowBasisCash    = "CASH"
	CashflowBasisAccrual = "ACCRUAL"

	DefaultCashflowMonths = 3
	MaxCashflowMonths     = 12
)

type CashflowMonth struct {
	Month             int      `json:"month"`
	Year              int      `json:"year"`
	IncomeCents       int64    `json:"incomeCents"`
	FixedBillsCents   int64    `json:"fixedBillsCents"`
	InstallmentsCents int64    `json:"installmentsCents"`
	DailyCents        int64    `json:"dailyCents"`
	TotalOutCents     int64    `json:"totalOutCents"`
	ResultCents       int64    `json:"resultCents"`
	ResultChangePct   *float64 `json:"resultChangePct"`
}

type CashflowCategory struct {
	CategoryID *string `json:"categoryId"`
	Name       string  `json:"name"`
	Icon       *string `json:"icon"`
	Color      *string `json:"color"`
	TotalCents int64   `json:"totalCents"`
	Percent    float64 `json:"percent"`
}

type CashflowView struct {
	Basis       string             `json:"basis"`
	Months      []CashflowMonth    `json:"months"`
	WhereItWent []CashflowCategory `json:"whereItWent"`
}

type CashflowQuery struct {
	Months int
	Basis  string
	Month  *int
	Year   *int
}

func ValidateCashflowQuery(q CashflowQuery) []ValidationIssue {
	issues := ValidatePeriodQuery(PeriodQuery{Month: q.Month, Year: q.Year})
	if q.Months < 0 || q.Months > MaxCashflowMonths {
		issues = append(issues, ValidationIssue{Path: "months", Message: "meses deve ser entre 1 e 12"})
	}
	if q.Basis != "" && q.Basis != CashflowBasisCash && q.Basis != CashflowBasisAccrual {
		issues = append(issues, ValidationIssue{Path: "basis", Message: "basis deve ser CASH ou ACCRUAL"})
	}
	return issues
}

// CashflowTx é um lançamento que entra no fluxo de caixa. Income: receita confirmada; senão saída
// (valor positivo). DueDate é o vencimento da fatura para compras no cartão (nil nas demais).
type CashflowTx struct {
	Income      bool
	Date        string
	DueDate     *string
	AmountCents int64
	FixedBill   bool // recurring_id ou loan_id
	Installment bool // installment_total > 1
	CategoryID  *string
	Name        *string
	Icon        *string
	Color       *string
}

// EffectiveDate é a data em que o lançamento conta: no regime de caixa a compra no cartão
// cai no vencimento da fatura; na competência, na data da compra.
func (t CashflowTx) EffectiveDate(basis string) string {
	if basis == CashflowBasisCash && t.DueDate != nil {
		return *t.DueDate
	}
	return t.Date
}

// ChangePct é a variação percentual de cur em relação a prev (nil quando prev = 0).
func ChangePct(cur, prev int64) *float64 {
	if prev == 0 {
		return nil
	}
	v := RoundPct(float64(cur-prev) / math.Abs(float64(prev)) * 100)
	return &v
}

// BuildCashflow resume os meses (ym em ordem cronológica; o primeiro serve só de base para a
// variação do seguinte e não é devolvido) e o "para onde foi" do último mês.
func BuildCashflow(txs []CashflowTx, ym []InvoicePeriod, basis string) CashflowView {
	idx := make(map[string]int, len(ym))
	months := make([]CashflowMonth, len(ym))
	for i, p := range ym {
		idx[MonthKeyOf(p.Year, p.Month)] = i
		months[i] = CashflowMonth{Month: p.Month, Year: p.Year}
	}
	type catKey struct{ id string }
	cats := map[catKey]*CashflowCategory{}
	var catOrder []catKey
	lastKey := ""
	if len(ym) > 0 {
		lastKey = MonthKeyOf(ym[len(ym)-1].Year, ym[len(ym)-1].Month)
	}
	for _, t := range txs {
		key := MonthKey(t.EffectiveDate(basis))
		i, ok := idx[key]
		if !ok {
			continue
		}
		m := &months[i]
		if t.Income {
			m.IncomeCents += t.AmountCents
			continue
		}
		switch {
		case t.FixedBill:
			m.FixedBillsCents += t.AmountCents
		case t.Installment:
			m.InstallmentsCents += t.AmountCents
		default:
			m.DailyCents += t.AmountCents
		}
		if key != lastKey {
			continue
		}
		k := catKey{}
		if t.CategoryID != nil {
			k.id = *t.CategoryID
		}
		c, ok := cats[k]
		if !ok {
			name := "Sem categoria"
			if t.Name != nil && *t.Name != "" {
				name = *t.Name
			}
			c = &CashflowCategory{CategoryID: t.CategoryID, Name: name, Icon: t.Icon, Color: t.Color}
			cats[k] = c
			catOrder = append(catOrder, k)
		}
		c.TotalCents += t.AmountCents
	}
	for i := range months {
		m := &months[i]
		m.TotalOutCents = m.FixedBillsCents + m.InstallmentsCents + m.DailyCents
		m.ResultCents = m.IncomeCents - m.TotalOutCents
		if i > 0 {
			m.ResultChangePct = ChangePct(m.ResultCents, months[i-1].ResultCents)
		}
	}

	out := CashflowView{Basis: basis, Months: []CashflowMonth{}, WhereItWent: []CashflowCategory{}}
	if len(months) > 1 {
		out.Months = months[1:]
	}
	var total int64
	for _, k := range catOrder {
		if cats[k].TotalCents > 0 {
			total += cats[k].TotalCents
		}
	}
	for _, k := range catOrder {
		c := *cats[k]
		if c.TotalCents <= 0 {
			continue
		}
		c.Percent = PercentOf(c.TotalCents, total)
		out.WhereItWent = append(out.WhereItWent, c)
	}
	sort.SliceStable(out.WhereItWent, func(i, j int) bool { return out.WhereItWent[i].TotalCents > out.WhereItWent[j].TotalCents })
	return out
}
