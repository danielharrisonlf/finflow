package domain

import (
	"regexp"
	"unicode/utf8"
)

var (
	hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	// uuidRe aceita só a forma canônica 8-4-4-4-12 (como z.uuid()); rejeita {..}, urn:uuid: e 32-hex.
	uuidRe = regexp.MustCompile(`^(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// runeLen conta caracteres (runas), não bytes — "ç" e "é" contam 1, como no zod.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

type ValidationIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func IsValidUUID(v string) bool {
	return uuidRe.MatchString(v)
}

func IsValidHexColor(v string) bool {
	return hexColorRe.MatchString(v)
}

func ValidateCreateAccount(in CreateAccountInput) []ValidationIssue {
	var issues []ValidationIssue
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 60 {
		issues = append(issues, ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 60 caracteres"})
	}
	if !in.Type.Valid() {
		issues = append(issues, ValidationIssue{Path: "type", Message: "tipo de conta inválido"})
	}
	if in.Color != nil && !IsValidHexColor(*in.Color) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}

func ValidateUpdateAccount(in UpdateAccountInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set {
		if !in.Name.Valid || runeLen(in.Name.Value) < 1 || runeLen(in.Name.Value) > 60 {
			issues = append(issues, ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 60 caracteres"})
		}
	}
	if in.Type.Set && (!in.Type.Valid || !in.Type.Value.Valid()) {
		issues = append(issues, ValidationIssue{Path: "type", Message: "tipo de conta inválido"})
	}
	if in.Color.Set && in.Color.Valid && !IsValidHexColor(in.Color.Value) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}

func ValidateCreateCategory(in CreateCategoryInput) []ValidationIssue {
	var issues []ValidationIssue
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 40 {
		issues = append(issues, ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 40 caracteres"})
	}
	if !in.Kind.Valid() {
		issues = append(issues, ValidationIssue{Path: "kind", Message: "tipo deve ser INCOME ou EXPENSE"})
	}
	if runeLen(in.Icon) < 1 || runeLen(in.Icon) > 40 {
		issues = append(issues, ValidationIssue{Path: "icon", Message: "ícone inválido"})
	}
	if !IsValidHexColor(in.Color) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	if in.ParentID != nil && !IsValidUUID(*in.ParentID) {
		issues = append(issues, ValidationIssue{Path: "parentId", Message: "parentId deve ser UUID válido"})
	}
	if in.SpendType != nil {
		if !in.SpendType.Valid() {
			issues = append(issues, spendTypeIssue)
		} else if in.Kind == CategoryIncome {
			issues = append(issues, ValidationIssue{Path: "spendType", Message: "tipo de gasto só se aplica a categorias de despesa"})
		}
	}
	return issues
}

var spendTypeIssue = ValidationIssue{Path: "spendType", Message: "tipo de gasto deve ser FIXED, CUTTABLE ou VARIABLE"}

func ValidateUpdateCategory(in UpdateCategoryInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set && (!in.Name.Valid || runeLen(in.Name.Value) < 1 || runeLen(in.Name.Value) > 40) {
		issues = append(issues, ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 40 caracteres"})
	}
	if in.Kind.Set && (!in.Kind.Valid || !in.Kind.Value.Valid()) {
		issues = append(issues, ValidationIssue{Path: "kind", Message: "tipo deve ser INCOME ou EXPENSE"})
	}
	if in.Icon.Set && (!in.Icon.Valid || runeLen(in.Icon.Value) < 1 || runeLen(in.Icon.Value) > 40) {
		issues = append(issues, ValidationIssue{Path: "icon", Message: "ícone inválido"})
	}
	if in.Color.Set && (!in.Color.Valid || !IsValidHexColor(in.Color.Value)) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	if in.ParentID.Set && in.ParentID.Valid && !IsValidUUID(in.ParentID.Value) {
		issues = append(issues, ValidationIssue{Path: "parentId", Message: "parentId deve ser UUID válido"})
	}
	if in.SpendType.Set && in.SpendType.Valid && !in.SpendType.Value.Valid() {
		issues = append(issues, spendTypeIssue)
	}
	return issues
}

func ValidateCreateCard(in CreateCardInput) []ValidationIssue {
	var issues []ValidationIssue
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 60 {
		issues = append(issues, ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 60 caracteres"})
	}
	if !in.Brand.Valid() {
		issues = append(issues, ValidationIssue{Path: "brand", Message: "bandeira inválida"})
	}
	if in.LimitCents <= 0 {
		issues = append(issues, ValidationIssue{Path: "limitCents", Message: "limite deve ser positivo"})
	}
	if in.ClosingDay < 1 || in.ClosingDay > 28 {
		issues = append(issues, ValidationIssue{Path: "closingDay", Message: "dia de fechamento deve ser entre 1 e 28"})
	}
	if in.DueDay < 1 || in.DueDay > 28 {
		issues = append(issues, ValidationIssue{Path: "dueDay", Message: "dia de vencimento deve ser entre 1 e 28"})
	}
	if in.Color != nil && !IsValidHexColor(*in.Color) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}

func ValidateUpdateCard(in UpdateCardInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set && (!in.Name.Valid || runeLen(in.Name.Value) < 1 || runeLen(in.Name.Value) > 60) {
		issues = append(issues, ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 60 caracteres"})
	}
	if in.Brand.Set && (!in.Brand.Valid || !in.Brand.Value.Valid()) {
		issues = append(issues, ValidationIssue{Path: "brand", Message: "bandeira inválida"})
	}
	if in.LimitCents.Set && (!in.LimitCents.Valid || in.LimitCents.Value <= 0) {
		issues = append(issues, ValidationIssue{Path: "limitCents", Message: "limite deve ser positivo"})
	}
	if in.ClosingDay.Set && (!in.ClosingDay.Valid || in.ClosingDay.Value < 1 || in.ClosingDay.Value > 28) {
		issues = append(issues, ValidationIssue{Path: "closingDay", Message: "dia de fechamento deve ser entre 1 e 28"})
	}
	if in.DueDay.Set && (!in.DueDay.Valid || in.DueDay.Value < 1 || in.DueDay.Value > 28) {
		issues = append(issues, ValidationIssue{Path: "dueDay", Message: "dia de vencimento deve ser entre 1 e 28"})
	}
	if in.Color.Set && in.Color.Valid && !IsValidHexColor(in.Color.Value) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}

func ValidateCreateCardExpense(in CreateCardExpenseInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.CardID != "" && !IsValidUUID(in.CardID) {
		issues = append(issues, ValidationIssue{Path: "cardId", Message: "cardId deve ser UUID válido"})
	}
	if runeLen(in.Description) < 1 || runeLen(in.Description) > 120 {
		issues = append(issues, ValidationIssue{Path: "description", Message: "descrição deve ter entre 1 e 120 caracteres"})
	}
	if in.TotalCents <= 0 {
		issues = append(issues, ValidationIssue{Path: "totalCents", Message: "valor deve ser positivo"})
	}
	if !IsValidDateString(in.Date) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.CategoryID != nil && !IsValidUUID(*in.CategoryID) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.Installments < 1 || in.Installments > 48 {
		issues = append(issues, ValidationIssue{Path: "installments", Message: "parcelas deve ser entre 1 e 48"})
	}
	if in.Notes != nil && runeLen(*in.Notes) > 500 {
		issues = append(issues, ValidationIssue{Path: "notes", Message: "observações até 500 caracteres"})
	}
	issues = append(issues, ValidateTagIDs(in.TagIDs)...)
	return issues
}

func ValidatePayInvoice(in PayInvoiceInput) []ValidationIssue {
	var issues []ValidationIssue
	if !IsValidUUID(in.AccountID) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	return issues
}

func ValidateCreateTransaction(in CreateTransactionInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Type != TxIncome && in.Type != TxExpense {
		issues = append(issues, ValidationIssue{Path: "type", Message: "tipo deve ser INCOME ou EXPENSE"})
	}
	if runeLen(in.Description) < 1 || runeLen(in.Description) > 120 {
		issues = append(issues, ValidationIssue{Path: "description", Message: "descrição deve ter entre 1 e 120 caracteres"})
	}
	if in.AmountCents <= 0 {
		issues = append(issues, ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"})
	}
	if !IsValidDateString(in.Date) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if !in.Status.Valid() {
		issues = append(issues, ValidationIssue{Path: "status", Message: "status inválido"})
	}
	if !IsValidUUID(in.AccountID) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	if in.CategoryID != nil && !IsValidUUID(*in.CategoryID) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.Notes != nil && runeLen(*in.Notes) > 500 {
		issues = append(issues, ValidationIssue{Path: "notes", Message: "observações até 500 caracteres"})
	}
	issues = append(issues, ValidateTagIDs(in.TagIDs)...)
	return issues
}

func ValidateUpdateTransaction(in UpdateTransactionInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Description.Set && (!in.Description.Valid || runeLen(in.Description.Value) < 1 || runeLen(in.Description.Value) > 120) {
		issues = append(issues, ValidationIssue{Path: "description", Message: "descrição deve ter entre 1 e 120 caracteres"})
	}
	if in.AmountCents.Set && (!in.AmountCents.Valid || in.AmountCents.Value <= 0) {
		issues = append(issues, ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"})
	}
	if in.Date.Set && (!in.Date.Valid || !IsValidDateString(in.Date.Value)) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.Status.Set && (!in.Status.Valid || !in.Status.Value.Valid()) {
		issues = append(issues, ValidationIssue{Path: "status", Message: "status inválido"})
	}
	if in.AccountID.Set && (!in.AccountID.Valid || !IsValidUUID(in.AccountID.Value)) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	if in.CategoryID.Set && in.CategoryID.Valid && !IsValidUUID(in.CategoryID.Value) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.Notes.Set && in.Notes.Valid && runeLen(in.Notes.Value) > 500 {
		issues = append(issues, ValidationIssue{Path: "notes", Message: "observações até 500 caracteres"})
	}
	if in.TagIDs.Set && in.TagIDs.Valid {
		issues = append(issues, ValidateTagIDs(in.TagIDs.Value)...)
	}
	return issues
}

func ValidateTransfer(in TransferInput) []ValidationIssue {
	var issues []ValidationIssue
	if !IsValidUUID(in.FromAccountID) {
		issues = append(issues, ValidationIssue{Path: "fromAccountId", Message: "fromAccountId deve ser UUID válido"})
	}
	if !IsValidUUID(in.ToAccountID) {
		issues = append(issues, ValidationIssue{Path: "toAccountId", Message: "toAccountId deve ser UUID válido"})
	}
	if in.FromAccountID != "" && in.FromAccountID == in.ToAccountID {
		issues = append(issues, ValidationIssue{Path: "toAccountId", Message: "conta de origem e destino devem ser diferentes"})
	}
	if in.AmountCents <= 0 {
		issues = append(issues, ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"})
	}
	if !IsValidDateString(in.Date) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.Description != nil && runeLen(*in.Description) > 120 {
		issues = append(issues, ValidationIssue{Path: "description", Message: "descrição até 120 caracteres"})
	}
	return issues
}

func ValidateUpsertBudget(in UpsertBudgetInput) []ValidationIssue {
	var issues []ValidationIssue
	if !IsValidUUID(in.CategoryID) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.AmountCents <= 0 {
		issues = append(issues, ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"})
	}
	return issues
}

func ValidateCreateGoal(in CreateGoalInput) []ValidationIssue {
	var issues []ValidationIssue
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 60 {
		issues = append(issues, ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 60 caracteres"})
	}
	if in.Kind != "" && !in.Kind.Valid() {
		issues = append(issues, ValidationIssue{Path: "kind", Message: "tipo de meta deve ser GOAL ou RESERVE"})
	}
	if runeLen(in.Icon) < 1 || runeLen(in.Icon) > 40 {
		issues = append(issues, ValidationIssue{Path: "icon", Message: "ícone inválido"})
	}
	if in.Color != nil && !IsValidHexColor(*in.Color) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	if in.TargetCents <= 0 {
		issues = append(issues, ValidationIssue{Path: "targetCents", Message: "meta deve ser positiva"})
	}
	if in.Deadline != nil && !IsValidDateString(*in.Deadline) {
		issues = append(issues, ValidationIssue{Path: "deadline", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	return issues
}

func ValidateUpdateGoal(in UpdateGoalInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set && (!in.Name.Valid || runeLen(in.Name.Value) < 1 || runeLen(in.Name.Value) > 60) {
		issues = append(issues, ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 60 caracteres"})
	}
	if in.Icon.Set && (!in.Icon.Valid || runeLen(in.Icon.Value) < 1 || runeLen(in.Icon.Value) > 40) {
		issues = append(issues, ValidationIssue{Path: "icon", Message: "ícone inválido"})
	}
	if in.Color.Set && in.Color.Valid && !IsValidHexColor(in.Color.Value) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	if in.TargetCents.Set && (!in.TargetCents.Valid || in.TargetCents.Value <= 0) {
		issues = append(issues, ValidationIssue{Path: "targetCents", Message: "meta deve ser positiva"})
	}
	if in.Deadline.Set && in.Deadline.Valid && !IsValidDateString(in.Deadline.Value) {
		issues = append(issues, ValidationIssue{Path: "deadline", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	return issues
}

func ValidateContributeGoal(in ContributeGoalInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.AmountCents == 0 {
		issues = append(issues, ValidationIssue{Path: "amountCents", Message: "valor não pode ser zero"})
	}
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.Note != nil && runeLen(*in.Note) > 120 {
		issues = append(issues, ValidationIssue{Path: "note", Message: "nota até 120 caracteres"})
	}
	return issues
}

// ───────────────────────── Recorrências ─────────────────────────

var (
	recurringDescIssue   = ValidationIssue{Path: "description", Message: "descrição deve ter entre 1 e 120 caracteres"}
	recurringKindIssue   = ValidationIssue{Path: "kind", Message: "tipo deve ser INCOME ou EXPENSE"}
	recurringAmountIssue = ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"}
	recurringDayIssue    = ValidationIssue{Path: "dayOfMonth", Message: "dia do mês deve ser entre 1 e 31"}
)

func ValidateCreateRecurring(in CreateRecurringInput) []ValidationIssue {
	var issues []ValidationIssue
	if runeLen(in.Description) < 1 || runeLen(in.Description) > 120 {
		issues = append(issues, recurringDescIssue)
	}
	if !in.Kind.Valid() {
		issues = append(issues, recurringKindIssue)
	}
	if in.AmountCents <= 0 {
		issues = append(issues, recurringAmountIssue)
	}
	if in.DayOfMonth < 1 || in.DayOfMonth > 31 {
		issues = append(issues, recurringDayIssue)
	}
	if in.AccountID != nil && !IsValidUUID(*in.AccountID) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	if in.CategoryID != nil && !IsValidUUID(*in.CategoryID) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.StartDate != nil && !IsValidDateString(*in.StartDate) {
		issues = append(issues, ValidationIssue{Path: "startDate", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.EndDate != nil && !IsValidDateString(*in.EndDate) {
		issues = append(issues, ValidationIssue{Path: "endDate", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	return issues
}

func ValidateUpdateRecurring(in UpdateRecurringInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Description.Set && (!in.Description.Valid || runeLen(in.Description.Value) < 1 || runeLen(in.Description.Value) > 120) {
		issues = append(issues, recurringDescIssue)
	}
	if in.Kind.Set && (!in.Kind.Valid || !in.Kind.Value.Valid()) {
		issues = append(issues, recurringKindIssue)
	}
	if in.AmountCents.Set && (!in.AmountCents.Valid || in.AmountCents.Value <= 0) {
		issues = append(issues, recurringAmountIssue)
	}
	if in.DayOfMonth.Set && (!in.DayOfMonth.Valid || in.DayOfMonth.Value < 1 || in.DayOfMonth.Value > 31) {
		issues = append(issues, recurringDayIssue)
	}
	if in.AccountID.Set && in.AccountID.Valid && !IsValidUUID(in.AccountID.Value) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	if in.CategoryID.Set && in.CategoryID.Valid && !IsValidUUID(in.CategoryID.Value) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.StartDate.Set && (!in.StartDate.Valid || !IsValidDateString(in.StartDate.Value)) {
		issues = append(issues, ValidationIssue{Path: "startDate", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.EndDate.Set && in.EndDate.Valid && !IsValidDateString(in.EndDate.Value) {
		issues = append(issues, ValidationIssue{Path: "endDate", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.Active.Set && !in.Active.Valid {
		issues = append(issues, ValidationIssue{Path: "active", Message: "active deve ser booleano"})
	}
	return issues
}

// ValidateRecurringPeriod confere o período já mesclado (fim não pode vir antes do início).
func ValidateRecurringPeriod(startDate string, endDate *string) []ValidationIssue {
	if endDate != nil && *endDate < startDate {
		return []ValidationIssue{{Path: "endDate", Message: "data final deve ser igual ou posterior à data inicial"}}
	}
	return nil
}

func ValidatePostRecurring(in PostRecurringInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"})
	}
	if in.AmountCents != nil && *in.AmountCents <= 0 {
		issues = append(issues, ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"})
	}
	if in.AccountID != nil && !IsValidUUID(*in.AccountID) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	return issues
}

// ───────────────────────── Configurações ─────────────────────────

// ValidateSettings confere as configurações já mescladas com o patch.
func ValidateSettings(s Settings) []ValidationIssue {
	var issues []ValidationIssue
	if s.MonthlyLimitCents != nil && *s.MonthlyLimitCents <= 0 {
		issues = append(issues, ValidationIssue{Path: "monthlyLimitCents", Message: "limite mensal deve ser positivo"})
	}
	if s.ReserveMonths < 1 || s.ReserveMonths > 24 {
		issues = append(issues, ValidationIssue{Path: "reserveMonths", Message: "meses de reserva deve ser entre 1 e 24"})
	}
	if s.ReserveRefillRatePct < 0 || s.ReserveRefillRatePct > 100 {
		issues = append(issues, ValidationIssue{Path: "reserveRefillRatePct", Message: "percentual deve ser entre 0 e 100"})
	}
	b := s.BalanceBands
	if !(b.DarkGreen > b.LightGreen && b.LightGreen > b.Yellow && b.Yellow > b.LightRed) {
		issues = append(issues, ValidationIssue{Path: "balanceBands", Message: "faixas devem ser decrescentes: darkGreen > lightGreen > yellow > lightRed"})
	}
	if p := s.SpendPlan; p != nil {
		for _, v := range []float64{p.FixedPct, p.CuttablePct, p.VariablePct} {
			if v < 0 || v > 100 {
				issues = append(issues, ValidationIssue{Path: "spendPlan", Message: "percentuais do plano devem ser entre 0 e 100"})
				break
			}
		}
		if sum := p.FixedPct + p.CuttablePct + p.VariablePct; sum < 99.99 || sum > 100.01 {
			issues = append(issues, ValidationIssue{Path: "spendPlan", Message: "percentuais do plano devem somar 100"})
		}
	}
	return issues
}

// ───────────────────────── Regras de categorização ─────────────────────────

const (
	MaxRulePatternChars = 80
	MaxRulePriority     = 1000
)

var (
	rulePatternIssue  = ValidationIssue{Path: "pattern", Message: "padrão deve ter entre 1 e 80 caracteres"}
	rulePriorityIssue = ValidationIssue{Path: "priority", Message: "prioridade deve ser entre -1000 e 1000"}
)

func validRulePattern(p string) bool {
	n := runeLen(NormalizeText(p))
	return n >= 1 && runeLen(p) <= MaxRulePatternChars
}

func ValidateCreateCategoryRule(in CreateCategoryRuleInput) []ValidationIssue {
	var issues []ValidationIssue
	if !validRulePattern(in.Pattern) {
		issues = append(issues, rulePatternIssue)
	}
	if !IsValidUUID(in.CategoryID) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.Priority != nil && (*in.Priority < -MaxRulePriority || *in.Priority > MaxRulePriority) {
		issues = append(issues, rulePriorityIssue)
	}
	if in.TagID != nil && !IsValidUUID(*in.TagID) {
		issues = append(issues, ruleTagIssue)
	}
	return issues
}

var ruleTagIssue = ValidationIssue{Path: "tagId", Message: "tagId deve ser UUID válido"}

func ValidateUpdateCategoryRule(in UpdateCategoryRuleInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Pattern.Set && (!in.Pattern.Valid || !validRulePattern(in.Pattern.Value)) {
		issues = append(issues, rulePatternIssue)
	}
	if in.CategoryID.Set && (!in.CategoryID.Valid || !IsValidUUID(in.CategoryID.Value)) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.Priority.Set && (!in.Priority.Valid || in.Priority.Value < -MaxRulePriority || in.Priority.Value > MaxRulePriority) {
		issues = append(issues, rulePriorityIssue)
	}
	if in.TagID.Set && in.TagID.Valid && !IsValidUUID(in.TagID.Value) {
		issues = append(issues, ruleTagIssue)
	}
	return issues
}

// ───────────────────────── Desejos ─────────────────────────

const MaxWishPriority = 100000

var (
	wishNameIssue     = ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 80 caracteres"}
	wishPriceIssue    = ValidationIssue{Path: "priceCents", Message: "preço deve ser positivo"}
	wishURLIssue      = ValidationIssue{Path: "url", Message: "url até 500 caracteres"}
	wishNotesIssue    = ValidationIssue{Path: "notes", Message: "observações até 500 caracteres"}
	wishPriorityIssue = ValidationIssue{Path: "priority", Message: "prioridade deve ser entre 0 e 100000"}
)

func ValidateCreateWishItem(in CreateWishItemInput) []ValidationIssue {
	var issues []ValidationIssue
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 80 {
		issues = append(issues, wishNameIssue)
	}
	if in.PriceCents <= 0 {
		issues = append(issues, wishPriceIssue)
	}
	if in.URL != nil && runeLen(*in.URL) > 500 {
		issues = append(issues, wishURLIssue)
	}
	if in.Notes != nil && runeLen(*in.Notes) > 500 {
		issues = append(issues, wishNotesIssue)
	}
	if in.Priority != nil && (*in.Priority < 0 || *in.Priority > MaxWishPriority) {
		issues = append(issues, wishPriorityIssue)
	}
	if in.ImageURL != nil && !validWishImageURL(*in.ImageURL) {
		issues = append(issues, wishImageIssue)
	}
	return issues
}

var wishImageIssue = ValidationIssue{Path: "imageUrl", Message: "imageUrl deve ser uma URL http(s) de até 1000 caracteres"}

func validWishImageURL(v string) bool {
	if runeLen(v) > 1000 {
		return false
	}
	return v == "" || ValidateLinkPreview(LinkPreviewInput{URL: v}) == nil
}

func ValidateUpdateWishItem(in UpdateWishItemInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set && (!in.Name.Valid || runeLen(in.Name.Value) < 1 || runeLen(in.Name.Value) > 80) {
		issues = append(issues, wishNameIssue)
	}
	if in.PriceCents.Set && (!in.PriceCents.Valid || in.PriceCents.Value <= 0) {
		issues = append(issues, wishPriceIssue)
	}
	if in.URL.Set && in.URL.Valid && runeLen(in.URL.Value) > 500 {
		issues = append(issues, wishURLIssue)
	}
	if in.Notes.Set && in.Notes.Valid && runeLen(in.Notes.Value) > 500 {
		issues = append(issues, wishNotesIssue)
	}
	if in.Priority.Set && (!in.Priority.Valid || in.Priority.Value < 0 || in.Priority.Value > MaxWishPriority) {
		issues = append(issues, wishPriorityIssue)
	}
	if in.Status.Set && (!in.Status.Valid || !in.Status.Value.Valid()) {
		issues = append(issues, ValidationIssue{Path: "status", Message: "status deve ser LISTED, BOUGHT ou ARCHIVED"})
	}
	if in.ImageURL.Set && in.ImageURL.Valid && !validWishImageURL(in.ImageURL.Value) {
		issues = append(issues, wishImageIssue)
	}
	return issues
}

// Limites dos filtros de consulta — os mesmos aplicados pela camada HTTP.
const (
	MinQueryYear        = 2000
	MaxQueryYear        = 2100
	MaxQueryPage        = 100000
	MaxQueryPageSize    = 100
	MaxQuerySearchChars = 120
)

func invalidNumber(path string) ValidationIssue {
	return ValidationIssue{Path: path, Message: "valor numérico inválido"}
}

// ValidatePeriodQuery valida mês (1–12) e ano (2000–2100) quando informados.
func ValidatePeriodQuery(q PeriodQuery) []ValidationIssue {
	var issues []ValidationIssue
	if q.Month != nil && (*q.Month < 1 || *q.Month > 12) {
		issues = append(issues, invalidNumber("month"))
	}
	if q.Year != nil && (*q.Year < MinQueryYear || *q.Year > MaxQueryYear) {
		issues = append(issues, invalidNumber("year"))
	}
	return issues
}

// ValidateTransactionFilters valida os filtros de listagem/exportação (sem paginação).
func ValidateTransactionFilters(q ListTransactionsQuery) []ValidationIssue {
	issues := ValidatePeriodQuery(PeriodQuery{Month: q.Month, Year: q.Year})
	if q.AccountID != nil && !IsValidUUID(*q.AccountID) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "UUID inválido"})
	}
	if q.CardID != nil && !IsValidUUID(*q.CardID) {
		issues = append(issues, ValidationIssue{Path: "cardId", Message: "UUID inválido"})
	}
	if q.CategoryID != nil && !IsValidUUID(*q.CategoryID) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "UUID inválido"})
	}
	if q.Type != nil && !q.Type.Valid() {
		issues = append(issues, ValidationIssue{Path: "type", Message: "tipo inválido"})
	}
	if q.Status != nil && !q.Status.Valid() {
		issues = append(issues, ValidationIssue{Path: "status", Message: "status inválido"})
	}
	if q.Search != nil && runeLen(*q.Search) > MaxQuerySearchChars {
		issues = append(issues, ValidationIssue{Path: "search", Message: "busca muito longa"})
	}
	if q.TagID != nil && !IsValidUUID(*q.TagID) {
		issues = append(issues, ValidationIssue{Path: "tagId", Message: "UUID inválido"})
	}
	return issues
}

// ValidateListTransactionsQuery valida paginação e filtros. Page/PageSize 0 significam "padrão".
func ValidateListTransactionsQuery(q ListTransactionsQuery) []ValidationIssue {
	var issues []ValidationIssue
	if q.Page < 0 || q.Page > MaxQueryPage {
		issues = append(issues, invalidNumber("page"))
	}
	if q.PageSize < 0 || q.PageSize > MaxQueryPageSize {
		issues = append(issues, invalidNumber("pageSize"))
	}
	return append(issues, ValidateTransactionFilters(q)...)
}
