package domain

import "math"

type BudgetView struct {
	Percent float64
	Status  BudgetStatus
}

// ComputeBudgetView: verde < 70%, amarelo 70–90%, vermelho > 90%.
func ComputeBudgetView(amountCents, spentCents int64) BudgetView {
	var ratio float64
	if amountCents > 0 {
		ratio = (float64(spentCents) / float64(amountCents)) * 100
	}
	status := BudgetOK
	if ratio > 90 {
		status = BudgetDanger
	} else if ratio >= 70 {
		status = BudgetWarning
	}
	return BudgetView{
		Percent: math.Round(ratio*10) / 10,
		Status:  status,
	}
}

// CrossedThreshold retorna 90, 70 ou 0 se nenhum limiar foi cruzado neste gasto.
func CrossedThreshold(beforePercent, afterPercent float64) int {
	if beforePercent < 90 && afterPercent >= 90 {
		return 90
	}
	if beforePercent < 70 && afterPercent >= 70 {
		return 70
	}
	return 0
}
