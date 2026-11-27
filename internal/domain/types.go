package domain

import "encoding/json"

// Optional representa um campo PATCH que distingue omitido, null e valor presente.
type Optional[T any] struct {
	Set   bool
	Valid bool
	Value T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Valid = false
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Valid = true
	o.Value = v
	return nil
}

// ───────────────────────── Enums ─────────────────────────

type AccountType string

const (
	AccountChecking   AccountType = "CHECKING"
	AccountSavings    AccountType = "SAVINGS"
	AccountWallet     AccountType = "WALLET"
	AccountSalary     AccountType = "SALARY"
	AccountInvestment AccountType = "INVESTMENT"
)

func (t AccountType) Valid() bool {
	switch t {
	case AccountChecking, AccountSavings, AccountWallet, AccountSalary, AccountInvestment:
		return true
	}
	return false
}

type CategoryKind string

const (
	CategoryIncome  CategoryKind = "INCOME"
	CategoryExpense CategoryKind = "EXPENSE"
)

func (k CategoryKind) Valid() bool {
	return k == CategoryIncome || k == CategoryExpense
}

type CardBrand string

const (
	BrandVisa       CardBrand = "VISA"
	BrandMastercard CardBrand = "MASTERCARD"
	BrandElo        CardBrand = "ELO"
	BrandAmex       CardBrand = "AMEX"
	BrandHipercard  CardBrand = "HIPERCARD"
	BrandOther      CardBrand = "OTHER"
)

func (b CardBrand) Valid() bool {
	switch b {
	case BrandVisa, BrandMastercard, BrandElo, BrandAmex, BrandHipercard, BrandOther:
		return true
	}
	return false
}

type InvoiceStatus string

const (
	InvoiceOpen   InvoiceStatus = "OPEN"
	InvoiceClosed InvoiceStatus = "CLOSED"
	InvoicePaid   InvoiceStatus = "PAID"
)

type TransactionType string

const (
	TxIncome         TransactionType = "INCOME"
	TxExpense        TransactionType = "EXPENSE"
	TxTransfer       TransactionType = "TRANSFER"
	TxCardExpense    TransactionType = "CARD_EXPENSE"
	TxInvoicePayment TransactionType = "INVOICE_PAYMENT"
)

func (t TransactionType) Valid() bool {
	switch t {
	case TxIncome, TxExpense, TxTransfer, TxCardExpense, TxInvoicePayment:
		return true
	}
	return false
}

type TransactionStatus string

const (
	TxPending   TransactionStatus = "PENDING"
	TxConfirmed TransactionStatus = "CONFIRMED"
)

func (s TransactionStatus) Valid() bool {
	return s == TxPending || s == TxConfirmed
}

type BudgetStatus string

const (
	BudgetOK      BudgetStatus = "OK"
	BudgetWarning BudgetStatus = "WARNING"
	BudgetDanger  BudgetStatus = "DANGER"
)

// ───────────────────────── Auth ─────────────────────────

type LoginInput struct {
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

type AuthStatus struct {
	Required      bool `json:"required"`
	Authenticated bool `json:"authenticated"`
}

// ───────────────────────── Accounts ─────────────────────────

type Account struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name"`
	Type                AccountType `json:"type"`
	InitialBalanceCents int64       `json:"initialBalanceCents"`
	BalanceCents        int64       `json:"balanceCents"`
	Color               *string     `json:"color"`
	Archived            bool        `json:"archived"`
}

type CreateAccountInput struct {
	Name                string      `json:"name"`
	Type                AccountType `json:"type"`
	InitialBalanceCents int64       `json:"initialBalanceCents"`
	Color               *string     `json:"color,omitempty"`
}

type UpdateAccountInput struct {
	Name                Optional[string]      `json:"name"`
	Type                Optional[AccountType] `json:"type"`
	InitialBalanceCents Optional[int64]       `json:"initialBalanceCents"`
	Color               Optional[string]      `json:"color"`
	Archived            Optional[bool]        `json:"archived"`
}

type TransferInput struct {
	FromAccountID string  `json:"fromAccountId"`
	ToAccountID   string  `json:"toAccountId"`
	AmountCents   int64   `json:"amountCents"`
	Date          string  `json:"date"`
	Description   *string `json:"description,omitempty"`
}

type TransferResult struct {
	TransferGroupID string `json:"transferGroupId"`
}

// ───────────────────────── Categories ─────────────────────────

type Category struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Kind     CategoryKind `json:"kind"`
	Icon     string       `json:"icon"`
	Color    string       `json:"color"`
	ParentID *string      `json:"parentId"`
}

type CreateCategoryInput struct {
	Name     string       `json:"name"`
	Kind     CategoryKind `json:"kind"`
	Icon     string       `json:"icon"`
	Color    string       `json:"color"`
	ParentID *string      `json:"parentId,omitempty"`
}

type UpdateCategoryInput struct {
	Name     Optional[string]       `json:"name"`
	Kind     Optional[CategoryKind] `json:"kind"`
	Icon     Optional[string]       `json:"icon"`
	Color    Optional[string]       `json:"color"`
	ParentID Optional[string]       `json:"parentId"`
}

// ───────────────────────── Cards & Invoices ─────────────────────────

type Invoice struct {
	ID          string        `json:"id"`
	CardID      string        `json:"cardId"`
	Month       int           `json:"month"`
	Year        int           `json:"year"`
	Status      InvoiceStatus `json:"status"`
	TotalCents  int64         `json:"totalCents"`
	ClosingDate string        `json:"closingDate"`
	DueDate     string        `json:"dueDate"`
	PaidAt      *string       `json:"paidAt"`
}

type Card struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Brand               CardBrand `json:"brand"`
	LimitCents          int64     `json:"limitCents"`
	ClosingDay          int       `json:"closingDay"`
	DueDay              int       `json:"dueDay"`
	Color               *string   `json:"color"`
	Archived            bool      `json:"archived"`
	UsedLimitCents      int64     `json:"usedLimitCents"`
	AvailableLimitCents int64     `json:"availableLimitCents"`
	CurrentInvoice      *Invoice  `json:"currentInvoice"`
}

type CreateCardInput struct {
	Name       string    `json:"name"`
	Brand      CardBrand `json:"brand"`
	LimitCents int64     `json:"limitCents"`
	ClosingDay int       `json:"closingDay"`
	DueDay     int       `json:"dueDay"`
	Color      *string   `json:"color,omitempty"`
}

type UpdateCardInput struct {
	Name       Optional[string]    `json:"name"`
	Brand      Optional[CardBrand] `json:"brand"`
	LimitCents Optional[int64]     `json:"limitCents"`
	ClosingDay Optional[int]       `json:"closingDay"`
	DueDay     Optional[int]       `json:"dueDay"`
	Color      Optional[string]    `json:"color"`
	Archived   Optional[bool]      `json:"archived"`
}

type CreateCardExpenseInput struct {
	CardID       string  `json:"cardId"`
	Description  string  `json:"description"`
	TotalCents   int64   `json:"totalCents"`
	Date         string  `json:"date"`
	CategoryID   *string `json:"categoryId,omitempty"`
	Installments int     `json:"installments"`
	Notes        *string `json:"notes,omitempty"`
}

// UnmarshalJSON aplica o padrão de 1 parcela só quando "installments" está ausente.
// Um 0 (ou null) explícito chega como 0 e é rejeitado pela validação (min 1), como no zod.
func (in *CreateCardExpenseInput) UnmarshalJSON(data []byte) error {
	type plain CreateCardExpenseInput
	aux := struct {
		*plain
		Installments json.RawMessage `json:"installments"`
	}{plain: (*plain)(in)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(aux.Installments) == 0 {
		in.Installments = 1
		return nil
	}
	in.Installments = 0
	return json.Unmarshal(aux.Installments, &in.Installments)
}

type CardExpenseResult struct {
	Created int `json:"created"`
}

type PayInvoiceInput struct {
	AccountID string  `json:"accountId"`
	Date      *string `json:"date,omitempty"`
}

// ───────────────────────── Transactions ─────────────────────────

type Transaction struct {
	ID                string            `json:"id"`
	Type              TransactionType   `json:"type"`
	Description       string            `json:"description"`
	AmountCents       int64             `json:"amountCents"`
	Date              string            `json:"date"`
	Status            TransactionStatus `json:"status"`
	AccountID         *string           `json:"accountId"`
	AccountName       *string           `json:"accountName"`
	CardID            *string           `json:"cardId"`
	CardName          *string           `json:"cardName"`
	CategoryID        *string           `json:"categoryId"`
	CategoryName      *string           `json:"categoryName"`
	CategoryIcon      *string           `json:"categoryIcon"`
	CategoryColor     *string           `json:"categoryColor"`
	TransferGroupID   *string           `json:"transferGroupId"`
	InstallmentNumber *int              `json:"installmentNumber"`
	InstallmentTotal  *int              `json:"installmentTotal"`
	InvoiceID         *string           `json:"invoiceId"`
	Notes             *string           `json:"notes"`
}

type CreateTransactionInput struct {
	Type        TransactionType   `json:"type"`
	Description string            `json:"description"`
	AmountCents int64             `json:"amountCents"`
	Date        string            `json:"date"`
	Status      TransactionStatus `json:"status"`
	AccountID   string            `json:"accountId"`
	CategoryID  *string           `json:"categoryId,omitempty"`
	Notes       *string           `json:"notes,omitempty"`
}

type UpdateTransactionInput struct {
	Description Optional[string]            `json:"description"`
	AmountCents Optional[int64]             `json:"amountCents"`
	Date        Optional[string]            `json:"date"`
	Status      Optional[TransactionStatus] `json:"status"`
	AccountID   Optional[string]            `json:"accountId"`
	CategoryID  Optional[string]            `json:"categoryId"`
	Notes       Optional[string]            `json:"notes"`
}

type ListTransactionsQuery struct {
	Page       int
	PageSize   int
	Month      *int
	Year       *int
	AccountID  *string
	CardID     *string
	CategoryID *string
	Type       *TransactionType
	Status     *TransactionStatus
	Search     *string
}

type Paginated[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// ───────────────────────── Budgets ─────────────────────────

type UpsertBudgetInput struct {
	CategoryID  string `json:"categoryId"`
	AmountCents int64  `json:"amountCents"`
}

type Budget struct {
	ID            string       `json:"id"`
	CategoryID    string       `json:"categoryId"`
	CategoryName  string       `json:"categoryName"`
	CategoryIcon  string       `json:"categoryIcon"`
	CategoryColor string       `json:"categoryColor"`
	AmountCents   int64        `json:"amountCents"`
	SpentCents    int64        `json:"spentCents"`
	Percent       float64      `json:"percent"`
	Status        BudgetStatus `json:"status"`
}

// ───────────────────────── Goals ─────────────────────────

type Goal struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Icon               string  `json:"icon"`
	Color              *string `json:"color"`
	TargetCents        int64   `json:"targetCents"`
	SavedCents         int64   `json:"savedCents"`
	Deadline           *string `json:"deadline"`
	Percent            float64 `json:"percent"`
	MonthlyNeededCents *int64  `json:"monthlyNeededCents"`
}

type CreateGoalInput struct {
	Name        string  `json:"name"`
	Icon        string  `json:"icon"`
	Color       *string `json:"color,omitempty"`
	TargetCents int64   `json:"targetCents"`
	Deadline    *string `json:"deadline,omitempty"`
}

type UpdateGoalInput struct {
	Name        Optional[string] `json:"name"`
	Icon        Optional[string] `json:"icon"`
	Color       Optional[string] `json:"color"`
	TargetCents Optional[int64]  `json:"targetCents"`
	Deadline    Optional[string] `json:"deadline"`
}

type ContributeGoalInput struct {
	AmountCents int64   `json:"amountCents"`
	Date        *string `json:"date,omitempty"`
	Note        *string `json:"note,omitempty"`
}

// ───────────────────────── Dashboard ─────────────────────────

type PeriodQuery struct {
	Month *int
	Year  *int
}

type DashboardSummary struct {
	Month             int   `json:"month"`
	Year              int   `json:"year"`
	TotalBalanceCents int64 `json:"totalBalanceCents"`
	MonthIncomeCents  int64 `json:"monthIncomeCents"`
	MonthExpenseCents int64 `json:"monthExpenseCents"`
	MonthNetCents     int64 `json:"monthNetCents"`
	OpenInvoicesCents int64 `json:"openInvoicesCents"`
	PendingCount      int   `json:"pendingCount"`
}

type CashflowPoint struct {
	Date         string `json:"date"`
	IncomeCents  int64  `json:"incomeCents"`
	ExpenseCents int64  `json:"expenseCents"`
}

type CategorySlice struct {
	CategoryID *string `json:"categoryId"`
	Name       string  `json:"name"`
	Icon       *string `json:"icon"`
	Color      *string `json:"color"`
	TotalCents int64   `json:"totalCents"`
	Percent    float64 `json:"percent"`
}

type UpcomingItem struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"` // "TRANSACTION" | "INVOICE"
	Description  string  `json:"description"`
	DueDate      string  `json:"dueDate"`
	AmountCents  int64   `json:"amountCents"`
	CategoryIcon *string `json:"categoryIcon"`
}

// ───────────────────────── Telegram ─────────────────────────

type TelegramStatus struct {
	Configured bool `json:"configured"`
	ChatLinked bool `json:"chatLinked"`
}

type TelegramChat struct {
	ID int64 `json:"id"`
}

type TelegramMessage struct {
	MessageID int64        `json:"message_id"`
	Text      string       `json:"text"`
	Chat      TelegramChat `json:"chat"`
}

type TelegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *TelegramMessage `json:"message,omitempty"`
}
