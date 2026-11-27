package db

import (
	"context"
	"fmt"
)

func (d *DB) Migrate(ctx context.Context) error {
	stmts, post := sqliteSchema, sqlitePostSchema
	if d.Dialect == DialectPostgres {
		stmts, post = postgresSchema, postgresPostSchema
	}
	for _, stmt := range stmts {
		if _, err := d.SQL.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	for _, c := range addedColumns {
		if err := d.addColumnIfMissing(ctx, c); err != nil {
			return fmt.Errorf("adicionar coluna %s.%s: %w", c.table, c.column, err)
		}
	}
	for _, stmt := range post {
		if _, err := d.SQL.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// addedColumn descreve uma coluna adicionada depois da criação da tabela (migração aditiva).
type addedColumn struct {
	table, column string
	sqliteDDL     string
	postgresDDL   string
}

// addedColumns são aplicadas em bancos novos e antigos; reexecutar o Migrate não muda nada.
var addedColumns = []addedColumn{
	{
		table: "categories", column: "spend_type",
		sqliteDDL:   `spend_type TEXT CHECK (spend_type IN ('FIXED', 'CUTTABLE', 'VARIABLE'))`,
		postgresDDL: `spend_type TEXT CHECK (spend_type IN ('FIXED', 'CUTTABLE', 'VARIABLE'))`,
	},
	{
		table: "transactions", column: "recurring_id",
		sqliteDDL:   `recurring_id TEXT REFERENCES recurring(id) ON DELETE SET NULL`,
		postgresDDL: `recurring_id UUID REFERENCES recurring(id) ON DELETE SET NULL`,
	},
	{
		table: "goals", column: "kind",
		sqliteDDL:   `kind TEXT NOT NULL DEFAULT 'GOAL' CHECK (kind IN ('GOAL', 'RESERVE'))`,
		postgresDDL: `kind TEXT NOT NULL DEFAULT 'GOAL' CHECK (kind IN ('GOAL', 'RESERVE'))`,
	},
	{
		table: "transactions", column: "loan_id",
		sqliteDDL:   `loan_id TEXT REFERENCES loans(id) ON DELETE SET NULL`,
		postgresDDL: `loan_id UUID REFERENCES loans(id) ON DELETE SET NULL`,
	},
	// v3 §1–§9 (tags, assinaturas, prévia de link).
	{
		table: "recurring", column: "is_subscription",
		sqliteDDL:   `is_subscription INTEGER NOT NULL DEFAULT 0`,
		postgresDDL: `is_subscription BOOLEAN NOT NULL DEFAULT FALSE`,
	},
	{
		table: "category_rules", column: "tag_id",
		sqliteDDL:   `tag_id TEXT REFERENCES tags(id) ON DELETE SET NULL`,
		postgresDDL: `tag_id UUID REFERENCES tags(id) ON DELETE SET NULL`,
	},
	{
		table: "wishlist", column: "image_url",
		sqliteDDL:   `image_url TEXT`,
		postgresDDL: `image_url TEXT`,
	},
	// v3 fase 2 (espaços): divisão de despesas, grupo do DRE e data de desativação das recorrências.
	{
		table: "transactions", column: "paid_by_member_id",
		sqliteDDL:   `paid_by_member_id TEXT REFERENCES members(id) ON DELETE SET NULL`,
		postgresDDL: `paid_by_member_id UUID REFERENCES members(id) ON DELETE SET NULL`,
	},
	{
		table: "categories", column: "dre_group",
		sqliteDDL:   `dre_group TEXT CHECK (dre_group IN ('REVENUE', 'DEDUCTIONS', 'COSTS', 'OPEX', 'FINANCIAL', 'OTHER'))`,
		postgresDDL: `dre_group TEXT CHECK (dre_group IN ('REVENUE', 'DEDUCTIONS', 'COSTS', 'OPEX', 'FINANCIAL', 'OTHER'))`,
	},
	{
		table: "recurring", column: "deactivated_at",
		sqliteDDL:   `deactivated_at TEXT`,
		postgresDDL: `deactivated_at DATE`,
	},
}

// addColumnIfMissing usa ADD COLUMN IF NOT EXISTS no PostgreSQL; o SQLite não tem essa
// forma, então a existência da coluna é conferida antes em pragma_table_info.
func (d *DB) addColumnIfMissing(ctx context.Context, c addedColumn) error {
	if d.Dialect == DialectPostgres {
		_, err := d.SQL.ExecContext(ctx, "ALTER TABLE "+c.table+" ADD COLUMN IF NOT EXISTS "+c.postgresDDL)
		return err
	}
	var n int
	if err := d.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, c.table, c.column,
	).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := d.SQL.ExecContext(ctx, "ALTER TABLE "+c.table+" ADD COLUMN "+c.sqliteDDL)
	return err
}

// As restrições CHECK dos campos "enum" (espelhando internal/domain/types.go) só
// valem para bancos novos: CREATE TABLE IF NOT EXISTS não altera tabelas já
// existentes, o que é aceitável — o domínio já valida esses valores.
var sqliteSchema = []string{
	`CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL CHECK (type IN ('CHECKING', 'SAVINGS', 'WALLET', 'SALARY', 'INVESTMENT')),
		initial_balance_cents INTEGER NOT NULL DEFAULT 0,
		color TEXT,
		archived INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS categories (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('INCOME', 'EXPENSE')),
		icon TEXT NOT NULL,
		color TEXT NOT NULL,
		parent_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
		UNIQUE(name, kind)
	)`,
	`CREATE TABLE IF NOT EXISTS credit_cards (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		brand TEXT NOT NULL CHECK (brand IN ('VISA', 'MASTERCARD', 'ELO', 'AMEX', 'HIPERCARD', 'OTHER')),
		limit_cents INTEGER NOT NULL,
		closing_day INTEGER NOT NULL,
		due_day INTEGER NOT NULL,
		color TEXT,
		archived INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS invoices (
		id TEXT PRIMARY KEY,
		card_id TEXT NOT NULL REFERENCES credit_cards(id) ON DELETE CASCADE,
		month INTEGER NOT NULL,
		year INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED', 'PAID')),
		paid_at TEXT,
		payment_transaction_id TEXT UNIQUE REFERENCES transactions(id) ON DELETE SET NULL,
		UNIQUE(card_id, year, month)
	)`,
	`CREATE TABLE IF NOT EXISTS transactions (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL CHECK (type IN ('INCOME', 'EXPENSE', 'TRANSFER', 'CARD_EXPENSE', 'INVOICE_PAYMENT')),
		description TEXT NOT NULL,
		amount_cents INTEGER NOT NULL,
		date TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'CONFIRMED' CHECK (status IN ('PENDING', 'CONFIRMED')),
		account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
		card_id TEXT REFERENCES credit_cards(id) ON DELETE CASCADE,
		category_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
		invoice_id TEXT REFERENCES invoices(id) ON DELETE CASCADE,
		transfer_group_id TEXT,
		installment_group_id TEXT,
		installment_number INTEGER,
		installment_total INTEGER,
		notes TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions(account_id)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_category_id ON transactions(category_id)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_invoice_id ON transactions(invoice_id)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_type_status ON transactions(type, status)`,
	`CREATE TABLE IF NOT EXISTS budgets (
		id TEXT PRIMARY KEY,
		category_id TEXT NOT NULL UNIQUE REFERENCES categories(id) ON DELETE CASCADE,
		amount_cents INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS goals (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		icon TEXT NOT NULL DEFAULT 'piggy-bank',
		color TEXT,
		target_cents INTEGER NOT NULL,
		deadline TEXT,
		archived INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS goal_contributions (
		id TEXT PRIMARY KEY,
		goal_id TEXT NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
		amount_cents INTEGER NOT NULL,
		date TEXT NOT NULL,
		note TEXT,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_goal_contributions_goal_id ON goal_contributions(goal_id)`,
	`CREATE TABLE IF NOT EXISTS recurring (
		id TEXT PRIMARY KEY,
		description TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('INCOME', 'EXPENSE')),
		amount_cents INTEGER NOT NULL,
		day_of_month INTEGER NOT NULL,
		account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL,
		category_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
		start_date TEXT NOT NULL,
		end_date TEXT,
		active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS category_rules (
		id TEXT PRIMARY KEY,
		pattern TEXT NOT NULL,
		category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
		priority INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS wishlist (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		price_cents INTEGER NOT NULL,
		url TEXT,
		notes TEXT,
		priority INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'LISTED' CHECK (status IN ('LISTED', 'BOUGHT', 'ARCHIVED')),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS investments (
		id TEXT PRIMARY KEY,
		account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL,
		name TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('FIXED_INCOME', 'TREASURY', 'STOCKS', 'FII', 'FUNDS', 'CRYPTO', 'PENSION', 'OTHER')),
		institution TEXT,
		invested_cents INTEGER NOT NULL DEFAULT 0,
		current_cents INTEGER NOT NULL DEFAULT 0,
		notes TEXT,
		archived INTEGER NOT NULL DEFAULT 0,
		value_updated_at TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS investment_snapshots (
		id TEXT PRIMARY KEY,
		investment_id TEXT NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
		date TEXT NOT NULL,
		value_cents INTEGER NOT NULL,
		invested_cents INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_investment_snapshots_investment_date ON investment_snapshots(investment_id, date)`,
	`CREATE TABLE IF NOT EXISTS loans (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		lender TEXT,
		kind TEXT NOT NULL CHECK (kind IN ('PERSONAL', 'PAYROLL', 'VEHICLE', 'MORTGAGE', 'STUDENT', 'CARD_DEBT', 'OVERDRAFT', 'OTHER')),
		principal_cents INTEGER NOT NULL,
		monthly_rate_bp INTEGER NOT NULL DEFAULT 0,
		installment_cents INTEGER NOT NULL,
		total_installments INTEGER NOT NULL,
		paid_installments INTEGER NOT NULL DEFAULT 0,
		first_due_date TEXT NOT NULL,
		account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL,
		category_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
		active INTEGER NOT NULL DEFAULT 1,
		notes TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS tags (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		color TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS transaction_tags (
		transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
		tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
		PRIMARY KEY (transaction_id, tag_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_transaction_tags_tag_id ON transaction_tags(tag_id)`,
	`CREATE TABLE IF NOT EXISTS members (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		color TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS transaction_splits (
		transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
		member_id TEXT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		share_bp INTEGER NOT NULL,
		PRIMARY KEY (transaction_id, member_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_transaction_splits_member_id ON transaction_splits(member_id)`,
	`CREATE TABLE IF NOT EXISTS settlements (
		id TEXT PRIMARY KEY,
		from_member_id TEXT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		to_member_id TEXT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		amount_cents INTEGER NOT NULL,
		date TEXT NOT NULL,
		transfer_group_id TEXT,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_settlements_date ON settlements(date)`,
}

// Índices que dependem das colunas de addedColumns (aplicados depois delas).
var sqlitePostSchema = []string{
	`CREATE INDEX IF NOT EXISTS idx_transactions_recurring_id ON transactions(recurring_id)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_loan_id ON transactions(loan_id)`,
	// No máximo uma reserva de emergência não arquivada.
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_goals_single_reserve ON goals(kind) WHERE kind = 'RESERVE' AND archived = 0`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_paid_by_member_id ON transactions(paid_by_member_id)`,
}

var postgresSchema = []string{
	`CREATE TABLE IF NOT EXISTS accounts (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL CHECK (type IN ('CHECKING', 'SAVINGS', 'WALLET', 'SALARY', 'INVESTMENT')),
		initial_balance_cents BIGINT NOT NULL DEFAULT 0,
		color TEXT,
		archived BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS categories (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('INCOME', 'EXPENSE')),
		icon TEXT NOT NULL,
		color TEXT NOT NULL,
		parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
		UNIQUE(name, kind)
	)`,
	`CREATE TABLE IF NOT EXISTS credit_cards (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		brand TEXT NOT NULL CHECK (brand IN ('VISA', 'MASTERCARD', 'ELO', 'AMEX', 'HIPERCARD', 'OTHER')),
		limit_cents BIGINT NOT NULL,
		closing_day INTEGER NOT NULL,
		due_day INTEGER NOT NULL,
		color TEXT,
		archived BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS invoices (
		id UUID PRIMARY KEY,
		card_id UUID NOT NULL REFERENCES credit_cards(id) ON DELETE CASCADE,
		month INTEGER NOT NULL,
		year INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED', 'PAID')),
		paid_at TIMESTAMPTZ,
		payment_transaction_id UUID UNIQUE,
		UNIQUE(card_id, year, month)
	)`,
	`CREATE TABLE IF NOT EXISTS transactions (
		id UUID PRIMARY KEY,
		type TEXT NOT NULL CHECK (type IN ('INCOME', 'EXPENSE', 'TRANSFER', 'CARD_EXPENSE', 'INVOICE_PAYMENT')),
		description TEXT NOT NULL,
		amount_cents BIGINT NOT NULL,
		date DATE NOT NULL,
		status TEXT NOT NULL DEFAULT 'CONFIRMED' CHECK (status IN ('PENDING', 'CONFIRMED')),
		account_id UUID REFERENCES accounts(id) ON DELETE CASCADE,
		card_id UUID REFERENCES credit_cards(id) ON DELETE CASCADE,
		category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
		invoice_id UUID REFERENCES invoices(id) ON DELETE CASCADE,
		transfer_group_id UUID,
		installment_group_id UUID,
		installment_number INTEGER,
		installment_total INTEGER,
		notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`DO $$ BEGIN
		ALTER TABLE invoices ADD CONSTRAINT invoices_payment_transaction_id_fkey
			FOREIGN KEY (payment_transaction_id) REFERENCES transactions(id) ON DELETE SET NULL;
	EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions(account_id)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_category_id ON transactions(category_id)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_invoice_id ON transactions(invoice_id)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_type_status ON transactions(type, status)`,
	`CREATE TABLE IF NOT EXISTS budgets (
		id UUID PRIMARY KEY,
		category_id UUID NOT NULL UNIQUE REFERENCES categories(id) ON DELETE CASCADE,
		amount_cents BIGINT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS goals (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		icon TEXT NOT NULL DEFAULT 'piggy-bank',
		color TEXT,
		target_cents BIGINT NOT NULL,
		deadline DATE,
		archived BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS goal_contributions (
		id UUID PRIMARY KEY,
		goal_id UUID NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
		amount_cents BIGINT NOT NULL,
		date DATE NOT NULL,
		note TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_goal_contributions_goal_id ON goal_contributions(goal_id)`,
	`CREATE TABLE IF NOT EXISTS recurring (
		id UUID PRIMARY KEY,
		description TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('INCOME', 'EXPENSE')),
		amount_cents BIGINT NOT NULL,
		day_of_month INTEGER NOT NULL,
		account_id UUID REFERENCES accounts(id) ON DELETE SET NULL,
		category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
		start_date DATE NOT NULL,
		end_date DATE,
		active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS category_rules (
		id UUID PRIMARY KEY,
		pattern TEXT NOT NULL,
		category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
		priority INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS wishlist (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		price_cents BIGINT NOT NULL,
		url TEXT,
		notes TEXT,
		priority INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'LISTED' CHECK (status IN ('LISTED', 'BOUGHT', 'ARCHIVED')),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS investments (
		id UUID PRIMARY KEY,
		account_id UUID REFERENCES accounts(id) ON DELETE SET NULL,
		name TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('FIXED_INCOME', 'TREASURY', 'STOCKS', 'FII', 'FUNDS', 'CRYPTO', 'PENSION', 'OTHER')),
		institution TEXT,
		invested_cents BIGINT NOT NULL DEFAULT 0,
		current_cents BIGINT NOT NULL DEFAULT 0,
		notes TEXT,
		archived BOOLEAN NOT NULL DEFAULT FALSE,
		value_updated_at DATE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS investment_snapshots (
		id UUID PRIMARY KEY,
		investment_id UUID NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
		date DATE NOT NULL,
		value_cents BIGINT NOT NULL,
		invested_cents BIGINT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_investment_snapshots_investment_date ON investment_snapshots(investment_id, date)`,
	`CREATE TABLE IF NOT EXISTS loans (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		lender TEXT,
		kind TEXT NOT NULL CHECK (kind IN ('PERSONAL', 'PAYROLL', 'VEHICLE', 'MORTGAGE', 'STUDENT', 'CARD_DEBT', 'OVERDRAFT', 'OTHER')),
		principal_cents BIGINT NOT NULL,
		monthly_rate_bp INTEGER NOT NULL DEFAULT 0,
		installment_cents BIGINT NOT NULL,
		total_installments INTEGER NOT NULL,
		paid_installments INTEGER NOT NULL DEFAULT 0,
		first_due_date DATE NOT NULL,
		account_id UUID REFERENCES accounts(id) ON DELETE SET NULL,
		category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
		active BOOLEAN NOT NULL DEFAULT TRUE,
		notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS tags (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		color TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS transaction_tags (
		transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
		tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
		PRIMARY KEY (transaction_id, tag_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_transaction_tags_tag_id ON transaction_tags(tag_id)`,
	`CREATE TABLE IF NOT EXISTS members (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		color TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS transaction_splits (
		transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
		member_id UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		share_bp INTEGER NOT NULL,
		PRIMARY KEY (transaction_id, member_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_transaction_splits_member_id ON transaction_splits(member_id)`,
	`CREATE TABLE IF NOT EXISTS settlements (
		id UUID PRIMARY KEY,
		from_member_id UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		to_member_id UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		amount_cents BIGINT NOT NULL,
		date DATE NOT NULL,
		transfer_group_id UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_settlements_date ON settlements(date)`,
}

var postgresPostSchema = []string{
	`CREATE INDEX IF NOT EXISTS idx_transactions_recurring_id ON transactions(recurring_id)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_loan_id ON transactions(loan_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_goals_single_reserve ON goals(kind) WHERE kind = 'RESERVE' AND archived = FALSE`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_paid_by_member_id ON transactions(paid_by_member_id)`,
}
