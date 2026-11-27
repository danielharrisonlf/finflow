package domain

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// ───────────────────────── Contratos ─────────────────────────

const (
	CadenceWeekly  = "WEEKLY"
	CadenceMonthly = "MONTHLY"
	CadenceYearly  = "YEARLY"

	// SubscriptionLookbackMonths é a janela de histórico analisada pela detecção.
	SubscriptionLookbackMonths = 13
)

type DetectedSubscription struct {
	Key              string   `json:"key"`
	Description      string   `json:"description"`
	AverageCents     int64    `json:"averageCents"`
	LastCents        int64    `json:"lastCents"`
	LastDate         string   `json:"lastDate"`
	Occurrences      int      `json:"occurrences"`
	Cadence          string   `json:"cadence"` // "WEEKLY" | "MONTHLY" | "YEARLY"
	NextExpectedDate string   `json:"nextExpectedDate"`
	PriceChangePct   *float64 `json:"priceChangePct"`
	CategoryID       *string  `json:"categoryId"`
	AccountID        *string  `json:"accountId"`
	CardID           *string  `json:"cardId"`
	RecurringID      *string  `json:"recurringId"`
}

type SubscriptionsView struct {
	Items             []DetectedSubscription `json:"items"`
	MonthlyTotalCents int64                  `json:"monthlyTotalCents"`
}

// SubscriptionTx é uma despesa candidata (EXPENSE ou CARD_EXPENSE sem parcelamento), valor positivo.
type SubscriptionTx struct {
	Description string
	Date        string
	AmountCents int64
	CategoryID  *string
	AccountID   *string
	CardID      *string
	RecurringID *string
}

// ───────────────────────── Normalização ─────────────────────────

var (
	// Sufixos de parcela e datas: "(1/3)", "01/12", "12/03/2026", "parcela 2 de 10".
	installmentSuffixRe = regexp.MustCompile(`\(?\s*\d+\s*/\s*\d+(\s*/\s*\d+)?\s*\)?|parcela\s*\d+(\s*de\s*\d+)?`)
	nonLetterRe         = regexp.MustCompile(`[^a-z]+`)
)

// NormalizeSubscriptionDescription agrupa descrições do mesmo serviço: minúsculas, sem acento,
// sem datas, sufixos de parcela, "*", códigos (palavras com dígitos) e dígitos; espaços colapsados.
func NormalizeSubscriptionDescription(s string) string {
	norm := NormalizeText(s)
	norm = installmentSuffixRe.ReplaceAllString(norm, " ")
	norm = strings.ReplaceAll(norm, "*", " ")
	words := strings.Fields(norm)
	kept := words[:0]
	for _, w := range words {
		if strings.ContainsAny(w, "0123456789") {
			continue // códigos de transação/autorização
		}
		kept = append(kept, w)
	}
	norm = nonLetterRe.ReplaceAllString(strings.Join(kept, " "), " ")
	return strings.Join(strings.Fields(norm), " ")
}

// SubscriptionKey é a chave estável (e segura em URL) de uma descrição: a forma normalizada com hífens.
func SubscriptionKey(description string) string {
	return strings.ReplaceAll(NormalizeSubscriptionDescription(description), " ", "-")
}

// ───────────────────────── Detecção ─────────────────────────

func medianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]int64(nil), values...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

func daysBetween(a, b string) int64 {
	return int64(FromDateString(b).Sub(FromDateString(a)).Hours() / 24)
}

// cadenceFor classifica a mediana dos intervalos (dias) pelo número de ocorrências.
func cadenceFor(n int, medianDays int64) (string, bool) {
	switch {
	case n >= 3 && medianDays >= 25 && medianDays <= 35:
		return CadenceMonthly, true
	case n >= 4 && medianDays >= 6 && medianDays <= 8:
		return CadenceWeekly, true
	case n >= 2 && medianDays >= 330 && medianDays <= 400:
		return CadenceYearly, true
	}
	return "", false
}

// cadenceDays é o ciclo nominal usado para saber se a assinatura ainda está ativa.
func cadenceDays(cadence string) int64 {
	switch cadence {
	case CadenceWeekly:
		return 7
	case CadenceYearly:
		return 365
	default:
		return 30
	}
}

// NextCadenceDate soma um ciclo à data.
func NextCadenceDate(date, cadence string) string {
	switch cadence {
	case CadenceWeekly:
		return AddDays(date, 7)
	case CadenceYearly:
		return AddMonthsToDateString(date, 12)
	default:
		return AddMonthsToDateString(date, 1)
	}
}

// MonthlyEquivalentCents normaliza o valor para um mês (semanal × 52 ÷ 12, anual ÷ 12).
func MonthlyEquivalentCents(cadence string, cents int64) int64 {
	switch cadence {
	case CadenceWeekly:
		return int64(math.Round(float64(cents) * 52 / 12))
	case CadenceYearly:
		return int64(math.Round(float64(cents) / 12))
	default:
		return cents
	}
}

// DetectSubscriptions aplica a heurística de assinaturas ao histórico (já filtrado para os últimos
// 13 meses de despesas sem parcelamento): agrupa por descrição normalizada; MONTHLY = ≥3 ocorrências
// com mediana dos intervalos de 25–35 dias, WEEKLY = ≥4 com 6–8, YEARLY = ≥2 com 330–400; todos os
// valores a até 20% da mediana; a última ocorrência no máximo 1,5 ciclo antes de hoje.
func DetectSubscriptions(txs []SubscriptionTx, today string) []DetectedSubscription {
	groups := map[string][]SubscriptionTx{}
	var order []string
	for _, tx := range txs {
		if tx.AmountCents <= 0 {
			continue
		}
		key := SubscriptionKey(tx.Description)
		if key == "" {
			continue
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], tx)
	}

	out := []DetectedSubscription{}
	for _, key := range order {
		g := groups[key]
		sort.SliceStable(g, func(i, j int) bool { return g[i].Date < g[j].Date })
		if len(g) < 2 {
			continue
		}
		intervals := make([]int64, 0, len(g)-1)
		amounts := make([]int64, 0, len(g))
		for i, tx := range g {
			amounts = append(amounts, tx.AmountCents)
			if i > 0 {
				intervals = append(intervals, daysBetween(g[i-1].Date, tx.Date))
			}
		}
		cadence, ok := cadenceFor(len(g), medianInt64(intervals))
		if !ok {
			continue
		}
		med := medianInt64(amounts)
		stable := true
		for _, a := range amounts {
			diff := a - med
			if diff < 0 {
				diff = -diff
			}
			if diff*100 > med*20 {
				stable = false
				break
			}
		}
		if !stable {
			continue
		}
		last := g[len(g)-1]
		if daysBetween(last.Date, today)*2 > cadenceDays(cadence)*3 {
			continue // mais de 1,5 ciclo sem cobrança: cancelada
		}

		var sum int64
		for _, a := range amounts {
			sum += a
		}
		n := int64(len(amounts))
		sub := DetectedSubscription{
			Key:              key,
			Description:      last.Description,
			AverageCents:     (sum + n/2) / n,
			LastCents:        last.AmountCents,
			LastDate:         last.Date,
			Occurrences:      len(g),
			Cadence:          cadence,
			NextExpectedDate: NextCadenceDate(last.Date, cadence),
			CategoryID:       last.CategoryID,
			AccountID:        last.AccountID,
			CardID:           last.CardID,
		}
		prevSum := sum - last.AmountCents
		if prevN := n - 1; prevN > 0 && prevSum > 0 {
			prevAvg := float64(prevSum) / float64(prevN)
			pct := RoundPct((float64(last.AmountCents) - prevAvg) / prevAvg * 100)
			if math.Abs(pct) >= 5 {
				sub.PriceChangePct = &pct
			}
		}
		for i := len(g) - 1; i >= 0; i-- {
			if g[i].RecurringID != nil {
				id := *g[i].RecurringID
				sub.RecurringID = &id
				break
			}
		}
		out = append(out, sub)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].NextExpectedDate != out[j].NextExpectedDate {
			return out[i].NextExpectedDate < out[j].NextExpectedDate
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// SubscriptionsMonthlyTotal soma o valor médio das assinaturas normalizado para um mês.
func SubscriptionsMonthlyTotal(items []DetectedSubscription) int64 {
	var total int64
	for _, it := range items {
		total += MonthlyEquivalentCents(it.Cadence, it.AverageCents)
	}
	return total
}
