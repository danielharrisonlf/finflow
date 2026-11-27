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

// SpendType classifica despesas: Fixo / Cortável / Variável. FIXED + CUTTABLE formam as
// "Saídas" (custo de vida); VARIABLE (e despesa sem categoria) é o "Diário".
type SpendType string

const (
	SpendFixed    SpendType = "FIXED"
	SpendCuttable SpendType = "CUTTABLE"
	SpendVariable SpendType = "VARIABLE"
)

func (t SpendType) Valid() bool {
	return t == SpendFixed || t == SpendCuttable || t == SpendVariable
}

// GoalKind distingue metas comuns da reserva de emergência (no máximo uma ativa).
type GoalKind string

const (
	GoalKindGoal    GoalKind = "GOAL"
	GoalKindReserve GoalKind = "RESERVE"
)

func (k GoalKind) Valid() bool {
	return k == GoalKindGoal || k == GoalKindReserve
}

type WishStatus string

const (
	WishListed   WishStatus = "LISTED"
	WishBought   WishStatus = "BOUGHT"
	WishArchived WishStatus = "ARCHIVED"
)

func (s WishStatus) Valid() bool {
	return s == WishListed || s == WishBought || s == WishArchived
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

// Category.SpendType é o tipo de gasto efetivo: o próprio, o herdado do pai ou VARIABLE
// (despesas); sempre null em receitas.
type Category struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Kind      CategoryKind `json:"kind"`
	Icon      string       `json:"icon"`
	Color     string       `json:"color"`
	ParentID  *string      `json:"parentId"`
	SpendType *SpendType   `json:"spendType"`
}

// CreateCategoryInput.SpendType omitido: VARIABLE em despesa raiz; subcategoria herda do pai.
type CreateCategoryInput struct {
	Name      string       `json:"name"`
	Kind      CategoryKind `json:"kind"`
	Icon      string       `json:"icon"`
	Color     string       `json:"color"`
	ParentID  *string      `json:"parentId,omitempty"`
	SpendType *SpendType   `json:"spendType,omitempty"`
}

// UpdateCategoryInput.SpendType null volta a herdar do pai (ou VARIABLE sem pai).
type UpdateCategoryInput struct {
	Name      Optional[string]       `json:"name"`
	Kind      Optional[CategoryKind] `json:"kind"`
	Icon      Optional[string]       `json:"icon"`
	Color     Optional[string]       `json:"color"`
	ParentID  Optional[string]       `json:"parentId"`
	SpendType Optional[SpendType]    `json:"spendType"`
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
	// TagIDs é aplicado a todas as parcelas.
	TagIDs []string `json:"tagIds,omitempty"`
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
	RecurringID       *string           `json:"recurringId"`
	LoanID            *string           `json:"loanId"`
	// Tags sempre presente (array vazio quando não há tags).
	Tags []TagRef `json:"tags"`
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
	TagIDs      []string          `json:"tagIds,omitempty"`
}

// UpdateTransactionInput.TagIDs substitui o conjunto de tags (null ou [] remove todas).
type UpdateTransactionInput struct {
	Description Optional[string]            `json:"description"`
	AmountCents Optional[int64]             `json:"amountCents"`
	Date        Optional[string]            `json:"date"`
	Status      Optional[TransactionStatus] `json:"status"`
	AccountID   Optional[string]            `json:"accountId"`
	CategoryID  Optional[string]            `json:"categoryId"`
	Notes       Optional[string]            `json:"notes"`
	TagIDs      Optional[[]string]          `json:"tagIds"`
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
	TagID      *string
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
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Kind               GoalKind `json:"kind"`
	Icon               string   `json:"icon"`
	Color              *string  `json:"color"`
	TargetCents        int64    `json:"targetCents"`
	SavedCents         int64    `json:"savedCents"`
	Deadline           *string  `json:"deadline"`
	Percent            float64  `json:"percent"`
	MonthlyNeededCents *int64   `json:"monthlyNeededCents"`
}

type CreateGoalInput struct {
	Name        string   `json:"name"`
	Kind        GoalKind `json:"kind,omitempty"`
	Icon        string   `json:"icon"`
	Color       *string  `json:"color,omitempty"`
	TargetCents int64    `json:"targetCents"`
	Deadline    *string  `json:"deadline,omitempty"`
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

// ───────────────────────── Recurring ─────────────────────────

// Recurring é um compromisso mensal: conta fixa (EXPENSE) ou receita recorrente (INCOME).
type Recurring struct {
	ID            string       `json:"id"`
	Description   string       `json:"description"`
	Kind          CategoryKind `json:"kind"`
	AmountCents   int64        `json:"amountCents"`
	DayOfMonth    int          `json:"dayOfMonth"`
	AccountID     *string      `json:"accountId"`
	AccountName   *string      `json:"accountName"`
	CategoryID    *string      `json:"categoryId"`
	CategoryName  *string      `json:"categoryName"`
	CategoryIcon  *string      `json:"categoryIcon"`
	CategoryColor *string      `json:"categoryColor"`
	StartDate     string       `json:"startDate"`
	EndDate       *string      `json:"endDate"`
	Active        bool         `json:"active"`
	NextDate      *string      `json:"nextDate"`
	// IsSubscription marca assinaturas (acompanhadas a partir da detecção, v3 §2).
	IsSubscription bool `json:"isSubscription"`
}

type CreateRecurringInput struct {
	Description    string       `json:"description"`
	Kind           CategoryKind `json:"kind"`
	AmountCents    int64        `json:"amountCents"`
	DayOfMonth     int          `json:"dayOfMonth"`
	AccountID      *string      `json:"accountId,omitempty"`
	CategoryID     *string      `json:"categoryId,omitempty"`
	StartDate      *string      `json:"startDate,omitempty"`
	EndDate        *string      `json:"endDate,omitempty"`
	IsSubscription *bool        `json:"isSubscription,omitempty"`
}

type UpdateRecurringInput struct {
	Description Optional[string]       `json:"description"`
	Kind        Optional[CategoryKind] `json:"kind"`
	AmountCents Optional[int64]        `json:"amountCents"`
	DayOfMonth  Optional[int]          `json:"dayOfMonth"`
	AccountID   Optional[string]       `json:"accountId"`
	CategoryID  Optional[string]       `json:"categoryId"`
	StartDate   Optional[string]       `json:"startDate"`
	EndDate     Optional[string]       `json:"endDate"`
	Active      Optional[bool]         `json:"active"`
	// IsSubscription null volta a false.
	IsSubscription Optional[bool] `json:"isSubscription"`
}

type PostRecurringInput struct {
	Date        *string `json:"date,omitempty"`
	AmountCents *int64  `json:"amountCents,omitempty"`
	AccountID   *string `json:"accountId,omitempty"`
}

// ───────────────────────── Settings ─────────────────────────

// BalanceBands são os limites inferiores (centavos) de cada faixa de saldo projetado.
type BalanceBands struct {
	DarkGreen  int64 `json:"darkGreen"`
	LightGreen int64 `json:"lightGreen"`
	Yellow     int64 `json:"yellow"`
	LightRed   int64 `json:"lightRed"`
}

// SpendPlan é o "Seu plano": divisão desejada do gasto entre Fixo, Cortável e Variável.
type SpendPlan struct {
	FixedPct    float64 `json:"fixedPct"`
	CuttablePct float64 `json:"cuttablePct"`
	VariablePct float64 `json:"variablePct"`
}

type Settings struct {
	MonthlyLimitCents    *int64       `json:"monthlyLimitCents"`
	ReserveMonths        int          `json:"reserveMonths"`
	HasDependents        bool         `json:"hasDependents"`
	ReserveRefillRatePct float64      `json:"reserveRefillRatePct"`
	BalanceBands         BalanceBands `json:"balanceBands"`
	SpendPlan            *SpendPlan   `json:"spendPlan"`
}

// BalanceBandsPatch permite alterar só algumas faixas.
type BalanceBandsPatch struct {
	DarkGreen  *int64 `json:"darkGreen,omitempty"`
	LightGreen *int64 `json:"lightGreen,omitempty"`
	Yellow     *int64 `json:"yellow,omitempty"`
	LightRed   *int64 `json:"lightRed,omitempty"`
}

// UpdateSettingsInput é um merge parcial; null em monthlyLimitCents/spendPlan limpa o valor
// e null em balanceBands volta às faixas padrão.
type UpdateSettingsInput struct {
	MonthlyLimitCents    Optional[int64]             `json:"monthlyLimitCents"`
	ReserveMonths        Optional[int]               `json:"reserveMonths"`
	HasDependents        Optional[bool]              `json:"hasDependents"`
	ReserveRefillRatePct Optional[float64]           `json:"reserveRefillRatePct"`
	BalanceBands         Optional[BalanceBandsPatch] `json:"balanceBands"`
	SpendPlan            Optional[SpendPlan]         `json:"spendPlan"`
}

// ───────────────────────── Category rules ─────────────────────────

type CategoryRule struct {
	ID           string `json:"id"`
	Pattern      string `json:"pattern"`
	CategoryID   string `json:"categoryId"`
	CategoryName string `json:"categoryName"`
	Priority     int    `json:"priority"`
	// TagID é a tag aplicada junto com a categoria (v3 §1).
	TagID   *string `json:"tagId"`
	TagName *string `json:"tagName"`
}

type CreateCategoryRuleInput struct {
	Pattern    string  `json:"pattern"`
	CategoryID string  `json:"categoryId"`
	Priority   *int    `json:"priority,omitempty"`
	TagID      *string `json:"tagId,omitempty"`
}

type UpdateCategoryRuleInput struct {
	Pattern    Optional[string] `json:"pattern"`
	CategoryID Optional[string] `json:"categoryId"`
	Priority   Optional[int]    `json:"priority"`
	TagID      Optional[string] `json:"tagId"`
}

type ApplyRulesInput struct {
	OnlyUncategorized *bool `json:"onlyUncategorized,omitempty"`
}

type ApplyRulesResult struct {
	Updated int `json:"updated"`
}

// ───────────────────────── Wishlist ─────────────────────────

type WishItem struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	PriceCents   int64      `json:"priceCents"`
	URL          *string    `json:"url"`
	Notes        *string    `json:"notes"`
	Priority     int        `json:"priority"`
	Status       WishStatus `json:"status"`
	AffordableOn *string    `json:"affordableOn"`
	ImageURL     *string    `json:"imageUrl"`
}

type CreateWishItemInput struct {
	Name       string  `json:"name"`
	PriceCents int64   `json:"priceCents"`
	URL        *string `json:"url,omitempty"`
	Notes      *string `json:"notes,omitempty"`
	Priority   *int    `json:"priority,omitempty"`
	ImageURL   *string `json:"imageUrl,omitempty"`
}

type UpdateWishItemInput struct {
	Name       Optional[string]     `json:"name"`
	PriceCents Optional[int64]      `json:"priceCents"`
	URL        Optional[string]     `json:"url"`
	Notes      Optional[string]     `json:"notes"`
	Priority   Optional[int]        `json:"priority"`
	Status     Optional[WishStatus] `json:"status"`
	ImageURL   Optional[string]     `json:"imageUrl"`
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
