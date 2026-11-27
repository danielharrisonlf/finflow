package domain

import (
	"fmt"
	"strings"
)

// ───────────────────────── "Posso comprar?" ─────────────────────────

type CanAffordInput struct {
	AmountCents  int64   `json:"amountCents"`
	Date         *string `json:"date,omitempty"`
	Installments *int    `json:"installments,omitempty"`
	CardID       *string `json:"cardId,omitempty"`
	Description  *string `json:"description,omitempty"`
}

type CanAffordLimit struct {
	LimitCents      int64   `json:"limitCents"`
	SpentAfterCents int64   `json:"spentAfterCents"`
	PercentAfter    float64 `json:"percentAfter"`
}

type CanAffordResult struct {
	Verdict                 string          `json:"verdict"` // "YES" | "CAREFUL" | "NO"
	LowestBalanceAfterCents int64           `json:"lowestBalanceAfterCents"`
	LowestDate              string          `json:"lowestDate"`
	Limit                   *CanAffordLimit `json:"limit"`
	ReserveMonthsAfter      *float64        `json:"reserveMonthsAfter"`
	FitsOn                  *string         `json:"fitsOn"`
	Message                 string          `json:"message"`
}

const (
	VerdictYes     = "YES"
	VerdictCareful = "CAREFUL"
	VerdictNo      = "NO"
)

func ValidateCanAfford(in CanAffordInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.AmountCents <= 0 || in.AmountCents > MaxParsedCents {
		issues = append(issues, ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"})
	}
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.Installments != nil && (*in.Installments < 1 || *in.Installments > 48) {
		issues = append(issues, ValidationIssue{Path: "installments", Message: "parcelas deve ser entre 1 e 48"})
	}
	if in.CardID != nil && !IsValidUUID(*in.CardID) {
		issues = append(issues, ValidationIssue{Path: "cardId", Message: "cardId deve ser UUID válido"})
	}
	if in.Description != nil && runeLen(*in.Description) > 120 {
		issues = append(issues, ValidationIssue{Path: "description", Message: "descrição até 120 caracteres"})
	}
	return issues
}

// FormatDateBR formata YYYY-MM-DD como DD/MM/AAAA.
func FormatDateBR(date string) string {
	if len(date) < 10 {
		return date
	}
	return date[8:10] + "/" + date[5:7] + "/" + date[:4]
}

// CardPurchaseEvents distribui a compra no cartão pelas faturas seguindo o ciclo (mesma regra
// do lançamento: a parcela i cai na fatura base + i) e devolve as saídas no vencimento de cada uma.
// FirstLegCents é a parcela que conta no limite do mês da compra.
func CardPurchaseEvents(description string, totalCents int64, installments int, date string, closingDay, dueDay int) (events []ProjectionPending, firstLegCents int64) {
	if installments < 1 {
		installments = 1
	}
	amounts, _ := SplitInstallments(totalCents, installments)
	base := InvoicePeriodForPurchase(date, closingDay)
	for i, amt := range amounts {
		desc := description
		if installments > 1 {
			desc = fmt.Sprintf("%s (%d/%d)", description, i+1, installments)
		}
		due := InvoiceDueDate(AddMonthsToPeriod(base, i), closingDay, dueDay)
		events = append(events, ProjectionPending{Description: desc, Date: due, AmountCents: -amt})
	}
	return events, amounts[0]
}

// LowestBalance devolve o menor saldo da série (a primeira data em caso de empate).
func LowestBalance(days []ProjectionDay) ProjectionLowest {
	if len(days) == 0 {
		return ProjectionLowest{}
	}
	low := ProjectionLowest{Date: days[0].Date, BalanceCents: days[0].BalanceCents}
	for _, d := range days[1:] {
		if d.BalanceCents < low.BalanceCents {
			low = ProjectionLowest{Date: d.Date, BalanceCents: d.BalanceCents}
		}
	}
	return low
}

// CanAffordVerdict: NO quando o menor saldo fica negativo; CAREFUL quando entra na faixa amarela
// (abaixo do limite inferior do verde claro) ou estoura o limite mensal; YES no resto.
func CanAffordVerdict(lowestCents int64, bands BalanceBands, limitExceeded bool) string {
	switch {
	case lowestCents < 0:
		return VerdictNo
	case lowestCents < bands.LightGreen || limitExceeded:
		return VerdictCareful
	default:
		return VerdictYes
	}
}

// CanAffordMessage explica o veredito em pt-BR.
func CanAffordMessage(r CanAffordResult, amountCents int64, bands BalanceBands) string {
	low := fmt.Sprintf("%s em %s", FormatBRL(r.LowestBalanceAfterCents), FormatDateBR(r.LowestDate))
	var b strings.Builder
	switch r.Verdict {
	case VerdictYes:
		fmt.Fprintf(&b, "Cabe: com a compra de %s, o menor saldo projetado fica em %s.", FormatBRL(amountCents), low)
	case VerdictCareful:
		fmt.Fprintf(&b, "Dá, mas com cuidado: com a compra de %s, o saldo projetado cai para %s", FormatBRL(amountCents), low)
		if r.LowestBalanceAfterCents < bands.LightGreen {
			b.WriteString(" (faixa amarela)")
		}
		b.WriteString(".")
	default:
		fmt.Fprintf(&b, "Agora não: com a compra de %s, o saldo projetado ficaria negativo (%s).", FormatBRL(amountCents), low)
		if r.FitsOn != nil {
			fmt.Fprintf(&b, " Caberia a partir de %s.", FormatDateBR(*r.FitsOn))
		} else {
			b.WriteString(" Não cabe nos próximos 12 meses.")
		}
	}
	if r.Limit != nil && r.Limit.SpentAfterCents > r.Limit.LimitCents {
		fmt.Fprintf(&b, " O gasto do mês passaria do limite de %s (%v%%).", FormatBRL(r.Limit.LimitCents), r.Limit.PercentAfter)
	}
	if r.ReserveMonthsAfter != nil {
		fmt.Fprintf(&b, " Se sair da reserva, ela ainda cobriria %v meses de custo de vida.", *r.ReserveMonthsAfter)
	}
	return b.String()
}
