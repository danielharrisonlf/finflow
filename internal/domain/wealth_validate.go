package domain

import "math"

// ───────────────────────── Validação: investimentos e empréstimos ─────────────────────────

var (
	investmentNameIssue  = ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 80 caracteres"}
	investmentKindIssue  = ValidationIssue{Path: "kind", Message: "tipo de investimento inválido"}
	investmentInstIssue  = ValidationIssue{Path: "institution", Message: "instituição deve ter até 80 caracteres"}
	wealthNotesIssue     = ValidationIssue{Path: "notes", Message: "observações devem ter até 500 caracteres"}
	loanNameIssue        = ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 80 caracteres"}
	loanLenderIssue      = ValidationIssue{Path: "lender", Message: "credor deve ter até 80 caracteres"}
	loanKindIssue        = ValidationIssue{Path: "kind", Message: "tipo de empréstimo inválido"}
	loanPrincipalIssue   = ValidationIssue{Path: "principalCents", Message: "valor contratado deve ser positivo"}
	loanRateIssue        = ValidationIssue{Path: "monthlyRatePct", Message: "taxa deve ser entre 0 e 100% ao mês"}
	loanInstallmentIssue = ValidationIssue{Path: "installmentCents", Message: "parcela deve ser positiva"}
	loanTotalIssue       = ValidationIssue{Path: "totalInstallments", Message: "parcelas deve ser entre 1 e 600"}
	loanPaidIssue        = ValidationIssue{Path: "paidInstallments", Message: "parcelas pagas deve ser entre 0 e o total"}
	loanFirstDueIssue    = ValidationIssue{Path: "firstDueDate", Message: "data deve estar no formato YYYY-MM-DD"}
	wealthDateIssue      = ValidationIssue{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"}
	wealthAmountIssue    = ValidationIssue{Path: "amountCents", Message: "valor deve ser positivo"}
)

const (
	maxLoanInstallments   = 600
	maxInvestmentTextSize = 80
	maxWealthNotesSize    = 500
)

func validOptionalUUID(path string, v *string, issues []ValidationIssue) []ValidationIssue {
	if v != nil && !IsValidUUID(*v) {
		issues = append(issues, ValidationIssue{Path: path, Message: path + " deve ser UUID válido"})
	}
	return issues
}

func validMoneyRange(v int64) bool {
	return v >= 0 && v <= MaxParsedCents
}

func validRatePct(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 100
}

func ValidateCreateInvestment(in CreateInvestmentInput) []ValidationIssue {
	var issues []ValidationIssue
	if runeLen(in.Name) < 1 || runeLen(in.Name) > maxInvestmentTextSize {
		issues = append(issues, investmentNameIssue)
	}
	if !in.Kind.Valid() {
		issues = append(issues, investmentKindIssue)
	}
	if in.Institution != nil && runeLen(*in.Institution) > maxInvestmentTextSize {
		issues = append(issues, investmentInstIssue)
	}
	issues = validOptionalUUID("accountId", in.AccountID, issues)
	if !validMoneyRange(in.InvestedCents) {
		issues = append(issues, ValidationIssue{Path: "investedCents", Message: "valor investido não pode ser negativo"})
	}
	if in.CurrentCents != nil && !validMoneyRange(*in.CurrentCents) {
		issues = append(issues, ValidationIssue{Path: "currentCents", Message: "valor atual não pode ser negativo"})
	}
	if in.Notes != nil && runeLen(*in.Notes) > maxWealthNotesSize {
		issues = append(issues, wealthNotesIssue)
	}
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, wealthDateIssue)
	}
	return issues
}

func ValidateUpdateInvestment(in UpdateInvestmentInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set && (!in.Name.Valid || runeLen(in.Name.Value) < 1 || runeLen(in.Name.Value) > maxInvestmentTextSize) {
		issues = append(issues, investmentNameIssue)
	}
	if in.Kind.Set && (!in.Kind.Valid || !in.Kind.Value.Valid()) {
		issues = append(issues, investmentKindIssue)
	}
	if in.Institution.Set && in.Institution.Valid && runeLen(in.Institution.Value) > maxInvestmentTextSize {
		issues = append(issues, investmentInstIssue)
	}
	if in.AccountID.Set && in.AccountID.Valid && !IsValidUUID(in.AccountID.Value) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	if in.InvestedCents.Set && (!in.InvestedCents.Valid || !validMoneyRange(in.InvestedCents.Value)) {
		issues = append(issues, ValidationIssue{Path: "investedCents", Message: "valor investido não pode ser negativo"})
	}
	if in.CurrentCents.Set && (!in.CurrentCents.Valid || !validMoneyRange(in.CurrentCents.Value)) {
		issues = append(issues, ValidationIssue{Path: "currentCents", Message: "valor atual não pode ser negativo"})
	}
	if in.Notes.Set && in.Notes.Valid && runeLen(in.Notes.Value) > maxWealthNotesSize {
		issues = append(issues, wealthNotesIssue)
	}
	if in.Archived.Set && !in.Archived.Valid {
		issues = append(issues, ValidationIssue{Path: "archived", Message: "archived deve ser booleano"})
	}
	return issues
}

func ValidateUpdateInvestmentValue(in UpdateInvestmentValueInput) []ValidationIssue {
	var issues []ValidationIssue
	if !validMoneyRange(in.CurrentCents) {
		issues = append(issues, ValidationIssue{Path: "currentCents", Message: "valor atual não pode ser negativo"})
	}
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, wealthDateIssue)
	}
	return issues
}

func ValidateInvestmentMovement(in InvestmentMovementInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.AmountCents <= 0 || in.AmountCents > MaxParsedCents {
		issues = append(issues, wealthAmountIssue)
	}
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, wealthDateIssue)
	}
	issues = validOptionalUUID("fromAccountId", in.FromAccountID, issues)
	issues = validOptionalUUID("toAccountId", in.ToAccountID, issues)
	return issues
}

func ValidateCreateLoan(in CreateLoanInput) []ValidationIssue {
	var issues []ValidationIssue
	if runeLen(in.Name) < 1 || runeLen(in.Name) > maxInvestmentTextSize {
		issues = append(issues, loanNameIssue)
	}
	if in.Lender != nil && runeLen(*in.Lender) > maxInvestmentTextSize {
		issues = append(issues, loanLenderIssue)
	}
	if !in.Kind.Valid() {
		issues = append(issues, loanKindIssue)
	}
	if in.PrincipalCents <= 0 || in.PrincipalCents > MaxParsedCents {
		issues = append(issues, loanPrincipalIssue)
	}
	if !validRatePct(in.MonthlyRatePct) {
		issues = append(issues, loanRateIssue)
	}
	if in.InstallmentCents != nil && (*in.InstallmentCents <= 0 || *in.InstallmentCents > MaxParsedCents) {
		issues = append(issues, loanInstallmentIssue)
	}
	if in.TotalInstallments < 1 || in.TotalInstallments > maxLoanInstallments {
		issues = append(issues, loanTotalIssue)
	}
	if in.PaidInstallments < 0 || in.PaidInstallments > in.TotalInstallments {
		issues = append(issues, loanPaidIssue)
	}
	if !IsValidDateString(in.FirstDueDate) {
		issues = append(issues, loanFirstDueIssue)
	}
	issues = validOptionalUUID("accountId", in.AccountID, issues)
	issues = validOptionalUUID("categoryId", in.CategoryID, issues)
	if in.Notes != nil && runeLen(*in.Notes) > maxWealthNotesSize {
		issues = append(issues, wealthNotesIssue)
	}
	return issues
}

func ValidateUpdateLoan(in UpdateLoanInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set && (!in.Name.Valid || runeLen(in.Name.Value) < 1 || runeLen(in.Name.Value) > maxInvestmentTextSize) {
		issues = append(issues, loanNameIssue)
	}
	if in.Lender.Set && in.Lender.Valid && runeLen(in.Lender.Value) > maxInvestmentTextSize {
		issues = append(issues, loanLenderIssue)
	}
	if in.Kind.Set && (!in.Kind.Valid || !in.Kind.Value.Valid()) {
		issues = append(issues, loanKindIssue)
	}
	if in.PrincipalCents.Set && (!in.PrincipalCents.Valid || in.PrincipalCents.Value <= 0 || in.PrincipalCents.Value > MaxParsedCents) {
		issues = append(issues, loanPrincipalIssue)
	}
	if in.MonthlyRatePct.Set && (!in.MonthlyRatePct.Valid || !validRatePct(in.MonthlyRatePct.Value)) {
		issues = append(issues, loanRateIssue)
	}
	if in.InstallmentCents.Set && (!in.InstallmentCents.Valid || in.InstallmentCents.Value <= 0 || in.InstallmentCents.Value > MaxParsedCents) {
		issues = append(issues, loanInstallmentIssue)
	}
	if in.TotalInstallments.Set && (!in.TotalInstallments.Valid || in.TotalInstallments.Value < 1 || in.TotalInstallments.Value > maxLoanInstallments) {
		issues = append(issues, loanTotalIssue)
	}
	if in.PaidInstallments.Set && (!in.PaidInstallments.Valid || in.PaidInstallments.Value < 0) {
		issues = append(issues, loanPaidIssue)
	}
	if in.FirstDueDate.Set && (!in.FirstDueDate.Valid || !IsValidDateString(in.FirstDueDate.Value)) {
		issues = append(issues, loanFirstDueIssue)
	}
	if in.AccountID.Set && in.AccountID.Valid && !IsValidUUID(in.AccountID.Value) {
		issues = append(issues, ValidationIssue{Path: "accountId", Message: "accountId deve ser UUID válido"})
	}
	if in.CategoryID.Set && in.CategoryID.Valid && !IsValidUUID(in.CategoryID.Value) {
		issues = append(issues, ValidationIssue{Path: "categoryId", Message: "categoryId deve ser UUID válido"})
	}
	if in.Active.Set && !in.Active.Valid {
		issues = append(issues, ValidationIssue{Path: "active", Message: "active deve ser booleano"})
	}
	if in.Notes.Set && in.Notes.Valid && runeLen(in.Notes.Value) > maxWealthNotesSize {
		issues = append(issues, wealthNotesIssue)
	}
	return issues
}

func ValidatePayLoan(in PayLoanInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Date != nil && !IsValidDateString(*in.Date) {
		issues = append(issues, wealthDateIssue)
	}
	if in.AmountCents != nil && (*in.AmountCents <= 0 || *in.AmountCents > MaxParsedCents) {
		issues = append(issues, wealthAmountIssue)
	}
	issues = validOptionalUUID("accountId", in.AccountID, issues)
	return issues
}
