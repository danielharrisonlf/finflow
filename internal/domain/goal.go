package domain

import "math"

// MonthlyNeededCents retorna o aporte mensal necessário (arredondado para cima); nil sem prazo ou concluída.
func MonthlyNeededCents(targetCents, savedCents int64, deadline *string, today string) *int64 {
	if deadline == nil || *deadline == "" {
		return nil
	}
	remaining := targetCents - savedCents
	if remaining <= 0 {
		return nil
	}
	months := MonthsUntil(*deadline, today)
	needed := int64(math.Ceil(float64(remaining) / float64(months)))
	return &needed
}

func GoalPercent(targetCents, savedCents int64) float64 {
	if targetCents <= 0 {
		return 0
	}
	pct := math.Round((float64(savedCents)/float64(targetCents))*1000) / 10
	if pct > 100 {
		return 100
	}
	return pct
}
