package domain

import "math"

// ───────────────────────── Ritmo e "quanto pode gastar hoje" ─────────────────────────

// PacePoint é o gasto acumulado até o dia no mês consultado e no mês anterior.
// CurrentCents é null nos dias depois de hoje (e no mês inteiro, se o mês for futuro).
type PacePoint struct {
	Day           int    `json:"day"`
	CurrentCents  *int64 `json:"currentCents"`
	PreviousCents int64  `json:"previousCents"`
}

type SpendToday struct {
	Cents int64  `json:"cents"`
	Basis string `json:"basis"` // "LIMIT" | "BALANCE"
}

const (
	SpendTodayLimit   = "LIMIT"
	SpendTodayBalance = "BALANCE"
)

// BuildPaceSeries monta a série acumulada dia a dia. currentDaily/previousDaily são os gastos por
// dia do mês; dim e prevDim os dias de cada mês; lastDay é o último dia com valor "atual"
// (hoje no mês corrente, dim em mês passado, 0 em mês futuro). No mês anterior, dias além do
// último dele repetem o total.
func BuildPaceSeries(currentDaily, previousDaily map[int]int64, dim, prevDim, lastDay int) []PacePoint {
	out := make([]PacePoint, 0, dim)
	var cur, prev int64
	for d := 1; d <= dim; d++ {
		cur += currentDaily[d]
		if d <= prevDim {
			prev += previousDaily[d]
		}
		p := PacePoint{Day: d, PreviousCents: prev}
		if d <= lastDay {
			v := cur
			p.CurrentCents = &v
		}
		out = append(out, p)
	}
	return out
}

// ProjectMonthSpend estima o gasto do mês: no mês corrente, gasto até hoje ÷ dia × dias do mês;
// nos demais, o próprio total.
func ProjectMonthSpend(spentCents int64, dayOfMonth, dim int, isCurrentMonth bool) int64 {
	if !isCurrentMonth || dayOfMonth <= 0 {
		return spentCents
	}
	return int64(math.Round(float64(spentCents) / float64(dayOfMonth) * float64(dim)))
}

// SpendTodayFromBalance divide o menor saldo projetado até o fim do mês pelos dias restantes
// (incluindo hoje), nunca negativo.
func SpendTodayFromBalance(lowestUntilMonthEnd int64, daysLeft int) int64 {
	if daysLeft <= 0 || lowestUntilMonthEnd <= 0 {
		return 0
	}
	return lowestUntilMonthEnd / int64(daysLeft)
}
