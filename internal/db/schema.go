package db

import "context"

func (d *DB) Migrate(ctx context.Context) error {
	stmts := sqliteSchema
	if d.Dialect == DialectPostgres {
		stmts = postgresSchema
	}
	for _, stmt := range stmts {
		if _, err := d.SQL.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
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
}
