package domain

import (
	"math"
	"sort"
)

// ───────────────────────── Investimentos (v3 §10) ─────────────────────────

type InvestmentKind string

const (
	InvestmentFixedIncome InvestmentKind = "FIXED_INCOME"
	InvestmentTreasury    InvestmentKind = "TREASURY"
	InvestmentStocks      InvestmentKind = "STOCKS"
	InvestmentFII         InvestmentKind = "FII"
	InvestmentFunds       InvestmentKind = "FUNDS"
	InvestmentCrypto      InvestmentKind = "CRYPTO"
	InvestmentPension     InvestmentKind = "PENSION"
	InvestmentOther       InvestmentKind = "OTHER"
)

func (k InvestmentKind) Valid() bool {
	switch k {
	case InvestmentFixedIncome, InvestmentTreasury, InvestmentStocks, InvestmentFII,
		InvestmentFunds, InvestmentCrypto, InvestmentPension, InvestmentOther:
		return true
	}
	return false
}

// Investment é uma posição (aplicação) com o valor investido e o valor atual.
type Investment struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Kind           InvestmentKind `json:"kind"`
	Institution    *string        `json:"institution"`
	AccountID      *string        `json:"accountId"`
	AccountName    *string        `json:"accountName"`
	InvestedCents  int64          `json:"investedCents"`
	CurrentCents   int64          `json:"currentCents"`
	ReturnCents    int64          `json:"returnCents"`
	ReturnPct      *float64       `json:"returnPct"`
	ValueUpdatedAt *string        `json:"valueUpdatedAt"`
	Notes          *string        `json:"notes"`
	Archived       bool           `json:"archived"`
}

// CreateInvestmentInput.CurrentCents omitido = investedCents; Date é a data do snapshot inicial (padrão hoje).
type CreateInvestmentInput struct {
	Name          string         `json:"name"`
	Kind          InvestmentKind `json:"kind"`
	Institution   *string        `json:"institution,omitempty"`
	AccountID     *string        `json:"accountId,omitempty"`
	InvestedCents int64          `json:"investedCents"`
	CurrentCents  *int64         `json:"currentCents,omitempty"`
	Notes         *string        `json:"notes,omitempty"`
	Date          *string        `json:"date,omitempty"`
}

// UpdateInvestmentInput: currentCents alterado aqui também grava snapshot (hoje).
type UpdateInvestmentInput struct {
	Name          Optional[string]         `json:"name"`
	Kind          Optional[InvestmentKind] `json:"kind"`
	Institution   Optional[string]         `json:"institution"`
	AccountID     Optional[string]         `json:"accountId"`
	InvestedCents Optional[int64]          `json:"investedCents"`
	CurrentCents  Optional[int64]          `json:"currentCents"`
	Notes         Optional[string]         `json:"notes"`
	Archived      Optional[bool]           `json:"archived"`
}

type UpdateInvestmentValueInput struct {
	CurrentCents int64   `json:"currentCents"`
	Date         *string `json:"date,omitempty"`
}

// InvestmentMovementInput é o corpo de aporte (fromAccountId) e de resgate (toAccountId).
type InvestmentMovementInput struct {
	AmountCents   int64   `json:"amountCents"`
	Date          *string `json:"date,omitempty"`
	FromAccountID *string `json:"fromAccountId,omitempty"`
	ToAccountID   *string `json:"toAccountId,omitempty"`
}

type InvestmentKindSlice struct {
	Kind         InvestmentKind `json:"kind"`
	CurrentCents int64          `json:"currentCents"`
	Percent      float64        `json:"percent"`
}

type InvestmentInstitutionSlice struct {
	Institution  string  `json:"institution"`
	CurrentCents int64   `json:"currentCents"`
	Percent      float64 `json:"percent"`
}

type InvestmentHistoryPoint struct {
	Month         int   `json:"month"`
	Year          int   `json:"year"`
	CurrentCents  int64 `json:"currentCents"`
	InvestedCents int64 `json:"investedCents"`
}

type InvestmentSummary struct {
	TotalInvestedCents int64                        `json:"totalInvestedCents"`
	TotalCurrentCents  int64                        `json:"totalCurrentCents"`
	ReturnCents        int64                        `json:"returnCents"`
	ReturnPct          *float64                     `json:"returnPct"`
	ByKind             []InvestmentKindSlice        `json:"byKind"`
	ByInstitution      []InvestmentInstitutionSlice `json:"byInstitution"`
	History            []InvestmentHistoryPoint     `json:"history"`
}

// NoInstitutionLabel agrupa posições sem instituição informada na alocação.
const NoInstitutionLabel = "Sem instituição"

// InvestmentStaleDays é a idade (dias) a partir da qual o valor de uma posição é considerado desatualizado.
const InvestmentStaleDays = 45

// ReturnPct é a rentabilidade (current − invested) ÷ invested em % com 2 casas; nil sem valor investido.
func ReturnPct(investedCents, currentCents int64) *float64 {
	if investedCents <= 0 {
		return nil
	}
	v := math.Round(float64(currentCents-investedCents)/float64(investedCents)*10000) / 100
	return &v
}

// InvestmentSnapshot é o valor (e o investido) de uma posição numa data.
type InvestmentSnapshot struct {
	Date          string
	ValueCents    int64
	InvestedCents int64
}

// LatestSnapshotAt devolve o snapshot mais recente com data <= date (snaps ordenados por data).
func LatestSnapshotAt(snaps []InvestmentSnapshot, date string) (InvestmentSnapshot, bool) {
	idx := sort.Search(len(snaps), func(i int) bool { return snaps[i].Date > date })
	if idx == 0 {
		return InvestmentSnapshot{}, false
	}
	return snaps[idx-1], true
}

// SummarizeInvestments monta totais, alocação por tipo/instituição (maior primeiro) e o
// histórico dos meses pedidos (valor no fim de cada mês pelo snapshot mais recente até lá).
// snapshots é indexado pelo id da posição e deve estar ordenado por data.
func SummarizeInvestments(items []Investment, snapshots map[string][]InvestmentSnapshot, months []InvoicePeriod) InvestmentSummary {
	out := InvestmentSummary{
		ByKind:        []InvestmentKindSlice{},
		ByInstitution: []InvestmentInstitutionSlice{},
		History:       make([]InvestmentHistoryPoint, 0, len(months)),
	}
	byKind := map[InvestmentKind]int64{}
	byInst := map[string]int64{}
	var kindOrder []InvestmentKind
	var instOrder []string
	for _, it := range items {
		out.TotalInvestedCents += it.InvestedCents
		out.TotalCurrentCents += it.CurrentCents
		if _, ok := byKind[it.Kind]; !ok {
			kindOrder = append(kindOrder, it.Kind)
		}
		byKind[it.Kind] += it.CurrentCents
		inst := NoInstitutionLabel
		if it.Institution != nil && *it.Institution != "" {
			inst = *it.Institution
		}
		if _, ok := byInst[inst]; !ok {
			instOrder = append(instOrder, inst)
		}
		byInst[inst] += it.CurrentCents
	}
	out.ReturnCents = out.TotalCurrentCents - out.TotalInvestedCents
	out.ReturnPct = ReturnPct(out.TotalInvestedCents, out.TotalCurrentCents)
	for _, k := range kindOrder {
		out.ByKind = append(out.ByKind, InvestmentKindSlice{Kind: k, CurrentCents: byKind[k], Percent: PercentOf(byKind[k], out.TotalCurrentCents)})
	}
	sort.SliceStable(out.ByKind, func(i, j int) bool { return out.ByKind[i].CurrentCents > out.ByKind[j].CurrentCents })
	for _, inst := range instOrder {
		out.ByInstitution = append(out.ByInstitution, InvestmentInstitutionSlice{Institution: inst, CurrentCents: byInst[inst], Percent: PercentOf(byInst[inst], out.TotalCurrentCents)})
	}
	sort.SliceStable(out.ByInstitution, func(i, j int) bool {
		return out.ByInstitution[i].CurrentCents > out.ByInstitution[j].CurrentCents
	})

	for _, p := range months {
		_, end := MonthRange(p.Year, p.Month)
		last := AddDays(end, -1)
		point := InvestmentHistoryPoint{Month: p.Month, Year: p.Year}
		for _, it := range items {
			if snap, ok := LatestSnapshotAt(snapshots[it.ID], last); ok {
				point.CurrentCents += snap.ValueCents
				point.InvestedCents += snap.InvestedCents
			}
		}
		out.History = append(out.History, point)
	}
	return out
}

// LastMonths devolve os n meses terminando em (year, month), do mais antigo ao mais recente.
func LastMonths(year, month, n int) []InvoicePeriod {
	out := make([]InvoicePeriod, 0, n)
	for i := n - 1; i >= 0; i-- {
		y, m := AddMonths(year, month, -i)
		out = append(out, InvoicePeriod{Year: y, Month: m})
	}
	return out
}
