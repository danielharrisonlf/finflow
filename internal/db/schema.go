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
}

// Índices que dependem das colunas de addedColumns (aplicados depois delas).
var sqlitePostSchema = []string{
	`CREATE INDEX IF NOT EXISTS idx_transactions_recurring_id ON transactions(recurring_id)`,
	// No máximo uma reserva de emergência não arquivada.
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_goals_single_reserve ON goals(kind) WHERE kind = 'RESERVE' AND archived = 0`,
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
}

var postgresPostSchema = []string{
	`CREATE INDEX IF NOT EXISTS idx_transactions_recurring_id ON transactions(recurring_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_goals_single_reserve ON goals(kind) WHERE kind = 'RESERVE' AND archived = FALSE`,
}
