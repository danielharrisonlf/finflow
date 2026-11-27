package domain

import (
	"math"
	"sort"
	"strings"
)

// ───────────────────────── Casa: membros e divisão (v3 fase 2 §4) ─────────────────────────

type Member struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CreateMemberInput struct {
	Name  string  `json:"name"`
	Color *string `json:"color,omitempty"`
}

type UpdateMemberInput struct {
	Name  Optional[string] `json:"name"`
	Color Optional[string] `json:"color"`
}

// TransactionSplit é a parte de um membro numa despesa (sharePct em %, até 2 casas decimais).
type TransactionSplit struct {
	MemberID string  `json:"memberId"`
	SharePct float64 `json:"sharePct"`
}

// SplitShare é a parte gravada no banco, em centésimos de % (10000 = 100%).
type SplitShare struct {
	MemberID string
	ShareBP  int
}

const (
	// DefaultMemberColor é a cor usada quando o membro é criado sem cor.
	DefaultMemberColor = "#0ea5e9"
	MaxMemberNameChars = 60
	// MaxSplitsPerTransaction limita a divisão de uma despesa.
	MaxSplitsPerTransaction = 50
	// FullShareBP é 100% em centésimos de %.
	FullShareBP = 10000
)

// SharePctToBP converte % (ex.: 33.33) em centésimos de % (3333).
func SharePctToBP(pct float64) int {
	return int(math.Round(pct * 100))
}

// SharePctFromBP converte centésimos de % em %.
func SharePctFromBP(bp int) float64 {
	return float64(bp) / 100
}

// SplitsToShares converte a divisão recebida na API para o formato gravado.
func SplitsToShares(splits []TransactionSplit) []SplitShare {
	out := make([]SplitShare, 0, len(splits))
	for _, s := range splits {
		out = append(out, SplitShare{MemberID: s.MemberID, ShareBP: SharePctToBP(s.SharePct)})
	}
	return out
}

var memberNameIssue = ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 60 caracteres"}

func ValidateCreateMember(in CreateMemberInput) []ValidationIssue {
	var issues []ValidationIssue
	name := strings.TrimSpace(in.Name)
	if runeLen(name) < 1 || runeLen(name) > MaxMemberNameChars {
		issues = append(issues, memberNameIssue)
	}
	if in.Color != nil && !IsValidHexColor(*in.Color) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}

func ValidateUpdateMember(in UpdateMemberInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set {
		name := strings.TrimSpace(in.Name.Value)
		if !in.Name.Valid || runeLen(name) < 1 || runeLen(name) > MaxMemberNameChars {
			issues = append(issues, memberNameIssue)
		}
	}
	if in.Color.Set && (!in.Color.Valid || !IsValidHexColor(in.Color.Value)) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}

// ValidateSplits confere a divisão de uma despesa: membros (UUID, sem repetir), partes
// positivas com até 2 casas decimais e soma exatamente 100%. Vazia = divisão igual.
func ValidateSplits(splits []TransactionSplit) []ValidationIssue {
	if len(splits) == 0 {
		return nil
	}
	if len(splits) > MaxSplitsPerTransaction {
		return []ValidationIssue{{Path: "splits", Message: "no máximo 50 membros na divisão"}}
	}
	seen := make(map[string]bool, len(splits))
	total := 0
	for _, s := range splits {
		if !IsValidUUID(s.MemberID) {
			return []ValidationIssue{{Path: "splits", Message: "memberId deve ser UUID válido"}}
		}
		if seen[s.MemberID] {
			return []ValidationIssue{{Path: "splits", Message: "membro repetido na divisão"}}
		}
		seen[s.MemberID] = true
		bp := SharePctToBP(s.SharePct)
		if s.SharePct <= 0 || s.SharePct > 100 || math.Abs(float64(bp)-s.SharePct*100) > 1e-6 {
			return []ValidationIssue{{Path: "splits", Message: "sharePct deve ser maior que 0 e até 100, com no máximo 2 casas decimais"}}
		}
		total += bp
	}
	if total != FullShareBP {
		return []ValidationIssue{{Path: "splits", Message: "a soma das partes (sharePct) deve ser 100"}}
	}
	return nil
}

// ValidateSharing confere paidByMemberId e splits de um lançamento.
func ValidateSharing(paidBy *string, splits []TransactionSplit) []ValidationIssue {
	var issues []ValidationIssue
	if paidBy != nil && !IsValidUUID(*paidBy) {
		issues = append(issues, ValidationIssue{Path: "paidByMemberId", Message: "paidByMemberId deve ser UUID válido"})
	}
	return append(issues, ValidateSplits(splits)...)
}

// SharedExpense é uma despesa paga por um membro (valor positivo); Splits vazio = divisão igual
// entre todos os membros do espaço.
type SharedExpense struct {
	PaidBy      string
	AmountCents int64
	Splits      []SplitShare
}

// SettlementRecord é um acerto registrado (from pagou to).
type SettlementRecord struct {
	FromMemberID string
	ToMemberID   string
	AmountCents  int64
}

// ExpenseShares reparte o valor entre os membros: pelas partes informadas ou, sem partes, igual
// entre memberIDs. O resto do arredondamento (centavos) fica com quem pagou.
func ExpenseShares(e SharedExpense, memberIDs []string) map[string]int64 {
	out := map[string]int64{}
	var assigned int64
	if len(e.Splits) > 0 {
		for _, s := range e.Splits {
			share := e.AmountCents * int64(s.ShareBP) / FullShareBP
			out[s.MemberID] += share
			assigned += share
		}
	} else if len(memberIDs) > 0 {
		base := e.AmountCents / int64(len(memberIDs))
		for _, id := range memberIDs {
			out[id] += base
			assigned += base
		}
	}
	if rest := e.AmountCents - assigned; rest != 0 {
		out[e.PaidBy] += rest
	}
	return out
}

type MemberBalance struct {
	MemberID     string `json:"memberId"`
	Name         string `json:"name"`
	PaidCents    int64  `json:"paidCents"`
	OwedCents    int64  `json:"owedCents"`
	BalanceCents int64  `json:"balanceCents"`
}

type SettlementSuggestion struct {
	FromMemberID string `json:"fromMemberId"`
	ToMemberID   string `json:"toMemberId"`
	AmountCents  int64  `json:"amountCents"`
}

type MemberBalances struct {
	Members     []MemberBalance        `json:"members"`
	Settlements []SettlementSuggestion `json:"settlements"`
}

// ComputeMemberBalances soma, por membro, o que pagou e a sua parte nas despesas; acertos
// registrados contam como pagamento de quem pagou e abatem o crédito de quem recebeu.
// balance > 0: tem a receber. Membros na ordem recebida.
func ComputeMemberBalances(members []Member, expenses []SharedExpense, settlements []SettlementRecord) []MemberBalance {
	ids := make([]string, len(members))
	idx := make(map[string]int, len(members))
	out := make([]MemberBalance, len(members))
	for i, m := range members {
		ids[i] = m.ID
		idx[m.ID] = i
		out[i] = MemberBalance{MemberID: m.ID, Name: m.Name}
	}
	for _, e := range expenses {
		if i, ok := idx[e.PaidBy]; ok {
			out[i].PaidCents += e.AmountCents
		}
		for id, share := range ExpenseShares(e, ids) {
			if i, ok := idx[id]; ok {
				out[i].OwedCents += share
			}
		}
	}
	for _, s := range settlements {
		if i, ok := idx[s.FromMemberID]; ok {
			out[i].PaidCents += s.AmountCents
		}
		if i, ok := idx[s.ToMemberID]; ok {
			out[i].OwedCents += s.AmountCents
		}
	}
	for i := range out {
		out[i].BalanceCents = out[i].PaidCents - out[i].OwedCents
	}
	return out
}

// SuggestSettlements propõe o mínimo de acertos pelo método guloso: o maior devedor paga ao
// maior credor até um dos dois zerar (empates pela ordem dos membros).
func SuggestSettlements(balances []MemberBalance) []SettlementSuggestion {
	type entry struct {
		id    string
		cents int64
		order int
	}
	var debtors, creditors []entry
	for i, b := range balances {
		switch {
		case b.BalanceCents < 0:
			debtors = append(debtors, entry{b.MemberID, -b.BalanceCents, i})
		case b.BalanceCents > 0:
			creditors = append(creditors, entry{b.MemberID, b.BalanceCents, i})
		}
	}
	byAmount := func(list []entry) func(i, j int) bool {
		return func(i, j int) bool {
			if list[i].cents != list[j].cents {
				return list[i].cents > list[j].cents
			}
			return list[i].order < list[j].order
		}
	}
	out := []SettlementSuggestion{}
	for len(debtors) > 0 && len(creditors) > 0 {
		sort.SliceStable(debtors, byAmount(debtors))
		sort.SliceStable(creditors, byAmount(creditors))
		d, c := &debtors[0], &creditors[0]
		amt := min(d.cents, c.cents)
		out = append(out, SettlementSuggestion{FromMemberID: d.id, ToMemberID: c.id, AmountCents: amt})
		d.cents -= amt
		c.cents -= amt
		if d.cents == 0 {
			debtors = debtors[1:]
		}
		if c.cents == 0 {
			creditors = creditors[1:]
		}
	}
	return out
}

// SettleInput registra um acerto entre membros; com as duas contas, também cria a transferência.
type SettleInput struct {
	FromMemberID  string  `json:"fromMemberId"`
	ToMemberID    string  `json:"toMemberId"`
	AmountCents   int64   `json:"amountCents"`
	Date          *string `json:"date,omitempty"`
	FromAccountID *string `json:"fromAccountId,omitempty"`
	ToAccountID   *string `json:"toAccountId,omitempty"`
}

type Settlement struct {
	ID              string  `json:"id"`
	FromMemberID    string  `json:"fromMemberId"`
	ToMemberID      string  `json:"toMemberId"`
	AmountCents     int64   `json:"amountCents"`
	Date            string  `json:"date"`
	TransferGroupID *string `json:"transferGroupId"`
}

func ValidateSettle(in SettleInput) []ValidationIssue {
	var issues []ValidationIssue
	if !IsValidUUID(in.FromMemberID) {
		issues = append(issues, ValidationIssue{Path: "fromMemberId", Message: "fromMemberId deve ser UUID válido"})
	}
	if !IsValidUUID(in.ToMemberID) {
		issues = append(issues, ValidationIssue{Path: "toMemberId", Message: "toMemberId deve ser UUID válido"})
	}
	if in.FromMemberID != "" && in.FromMemberID == in.ToMemberID {
		issues = append(issues, ValidationIssue{Path: "toMemberId", Message: "quem paga e quem recebe devem ser membros diferentes"})
	}
	if in.AmountCents <= 0 || !validMoneyRange(in.AmountCents) {
		issues = append(issues, ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"})
	}
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	issues = validOptionalUUID("fromAccountId", in.FromAccountID, issues)
	issues = validOptionalUUID("toAccountId", in.ToAccountID, issues)
	if (in.FromAccountID == nil) != (in.ToAccountID == nil) {
		issues = append(issues, ValidationIssue{Path: "toAccountId", Message: "informe as duas contas (origem e destino) ou nenhuma"})
	}
	if in.FromAccountID != nil && in.ToAccountID != nil && *in.FromAccountID == *in.ToAccountID {
		issues = append(issues, ValidationIssue{Path: "toAccountId", Message: "contas de origem e destino devem ser diferentes"})
	}
	return issues
}
