package domain

import "sort"

// ───────────────────────── Composição da fatura ─────────────────────────

type InvoiceComposition struct {
	InstallmentsCents  int64 `json:"installmentsCents"`
	SubscriptionsCents int64 `json:"subscriptionsCents"`
	OneOffCents        int64 `json:"oneOffCents"`
}

// InvoiceBreakdown: compras (+), estornos (valores positivos), pago e total (compras − estornos).
type InvoiceBreakdown struct {
	PurchasesCents int64              `json:"purchasesCents"`
	CreditsCents   int64              `json:"creditsCents"`
	PaymentsCents  int64              `json:"paymentsCents"`
	TotalCents     int64              `json:"totalCents"`
	Composition    InvoiceComposition `json:"composition"`
}

// InvoiceLine é um lançamento da fatura com o valor como gravado (compra negativa, estorno positivo).
type InvoiceLine struct {
	Description      string
	AmountCents      int64
	InstallmentTotal *int
}

// BuildInvoiceBreakdown separa compras e estornos e classifica as compras em parcelas,
// assinaturas (descrição normalizada presente em subscriptionKeys) e avulsas.
func BuildInvoiceBreakdown(lines []InvoiceLine, paymentCents int64, subscriptionKeys map[string]bool) InvoiceBreakdown {
	var out InvoiceBreakdown
	for _, l := range lines {
		if l.AmountCents > 0 {
			out.CreditsCents += l.AmountCents
			continue
		}
		amt := -l.AmountCents
		out.PurchasesCents += amt
		switch {
		case l.InstallmentTotal != nil && *l.InstallmentTotal > 1:
			out.Composition.InstallmentsCents += amt
		case subscriptionKeys[SubscriptionKey(l.Description)]:
			out.Composition.SubscriptionsCents += amt
		default:
			out.Composition.OneOffCents += amt
		}
	}
	if paymentCents < 0 {
		paymentCents = -paymentCents
	}
	out.PaymentsCents = paymentCents
	out.TotalCents = out.PurchasesCents - out.CreditsCents
	return out
}

// ───────────────────────── Transferências inteligentes ─────────────────────────

// MaxTransferCandidateDays é a distância máxima entre as datas das duas pernas.
const MaxTransferCandidateDays = 3

type TransferCandidate struct {
	Expense   Transaction `json:"expense"`
	Income    Transaction `json:"income"`
	DaysApart int         `json:"daysApart"`
}

type MergeTransferInput struct {
	ExpenseID string `json:"expenseId"`
	IncomeID  string `json:"incomeId"`
}

func ValidateMergeTransfer(in MergeTransferInput) []ValidationIssue {
	var issues []ValidationIssue
	if !IsValidUUID(in.ExpenseID) {
		issues = append(issues, ValidationIssue{Path: "expenseId", Message: "expenseId deve ser UUID válido"})
	}
	if !IsValidUUID(in.IncomeID) {
		issues = append(issues, ValidationIssue{Path: "incomeId", Message: "incomeId deve ser UUID válido"})
	}
	return issues
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// PairTransferCandidates casa despesas e receitas de contas diferentes com o mesmo valor absoluto
// e datas a até 3 dias; cada transação entra em no máximo um par (a menor distância vence; no
// empate, a despesa mais recente e depois os ids). Resultado do par mais recente para o mais antigo.
func PairTransferCandidates(expenses, incomes []Transaction) []TransferCandidate {
	type pair struct {
		e, i int
		days int
	}
	var pairs []pair
	for ei, e := range expenses {
		if e.AccountID == nil {
			continue
		}
		for ii, in := range incomes {
			if in.AccountID == nil || *in.AccountID == *e.AccountID || absInt64(in.AmountCents) != absInt64(e.AmountCents) {
				continue
			}
			d := int(absInt64(daysBetween(e.Date, in.Date)))
			if d > MaxTransferCandidateDays {
				continue
			}
			pairs = append(pairs, pair{e: ei, i: ii, days: d})
		}
	}
	sort.SliceStable(pairs, func(a, b int) bool {
		pa, pb := pairs[a], pairs[b]
		if pa.days != pb.days {
			return pa.days < pb.days
		}
		ea, eb := expenses[pa.e], expenses[pb.e]
		if ea.Date != eb.Date {
			return ea.Date > eb.Date
		}
		if ea.ID != eb.ID {
			return ea.ID < eb.ID
		}
		return incomes[pa.i].ID < incomes[pb.i].ID
	})
	usedE, usedI := map[int]bool{}, map[int]bool{}
	out := []TransferCandidate{}
	for _, p := range pairs {
		if usedE[p.e] || usedI[p.i] {
			continue
		}
		usedE[p.e], usedI[p.i] = true, true
		out = append(out, TransferCandidate{Expense: expenses[p.e], Income: incomes[p.i], DaysApart: p.days})
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Expense.Date != out[b].Expense.Date {
			return out[a].Expense.Date > out[b].Expense.Date
		}
		return out[a].Expense.ID < out[b].Expense.ID
	})
	return out
}
