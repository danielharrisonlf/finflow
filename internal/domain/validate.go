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
	return issues
}

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
