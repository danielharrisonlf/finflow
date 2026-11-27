package db_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"finflow/internal/db"
	"finflow/internal/testutil"
)

func TestFoldFunctionSQLite(t *testing.T) {
	d := testutil.Open(t, db.DialectSQLite)
	var got string
	if err := d.QueryRow(context.Background(), `SELECT `+db.FoldFunc+`(?)`, "AÇÃO Ünico").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "ação ünico" {
		t.Fatalf("%s = %q", db.FoldFunc, got)
	}
}

func TestCheckConstraints(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		_, err := d.Exec(context.Background(), `
			INSERT INTO categories (id, name, kind, icon, color) VALUES (?, 'X', 'OUTRO', 'i', '#fff')
		`, uuid.NewString())
		if err == nil {
			t.Fatal("kind inválido deveria violar o CHECK")
		}
	})
}

// TestPostgresSessionUTC garante que a sessão usa UTC mesmo quando o ambiente
// (PGTZ) aponta outro fuso, para que paid_at 'YYYY-MM-DD' não mude de dia.
func TestPostgresSessionUTC(t *testing.T) {
	if strings.TrimSpace(os.Getenv(testutil.EnvPostgresURL)) == "" {
		t.Skipf("%s não definido", testutil.EnvPostgresURL)
	}
	t.Setenv("PGTZ", "Asia/Tokyo")
	d := testutil.Open(t, db.DialectPostgres)
	ctx := context.Background()

	var tz string
	if err := d.QueryRow(ctx, `SHOW timezone`).Scan(&tz); err != nil {
		t.Fatal(err)
	}
	if tz != "UTC" {
		t.Fatalf("timezone da sessão = %q, esperado UTC", tz)
	}

	var paid db.NullDateText
	if err := d.QueryRow(ctx, `SELECT CAST(? AS TIMESTAMPTZ)`, "2026-03-15").Scan(&paid); err != nil {
		t.Fatal(err)
	}
	if paid.String != "2026-03-15" {
		t.Fatalf("data lida = %q, esperado 2026-03-15", paid.String)
	}
}

// TestPostgresPrismaSchemaParam abre o banco com ?schema=X (formato do Prisma).
func TestPostgresPrismaSchemaParam(t *testing.T) {
	base := strings.TrimSpace(os.Getenv(testutil.EnvPostgresURL))
	if base == "" {
		t.Skipf("%s não definido", testutil.EnvPostgresURL)
	}
	ctx := context.Background()
	schema := "p_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA %q`, schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema))
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("schema", schema)
	u.RawQuery = q.Encode()

	d, err := db.Open(ctx, u.String(), "")
	if err != nil {
		t.Fatalf("db.Open com ?schema=: %v", err)
	}
	defer d.Close()

	var n int
	if err := admin.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name = 'accounts'`, schema,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("tabela accounts não foi criada no schema %s", schema)
	}
}

func TestOpenRejectsUnrecognizedDatabaseURL(t *testing.T) {
	for _, dsn := range []string{"host=localhost dbname=finflow", "mysql://u:p@localhost/db"} {
		database, err := db.Open(context.Background(), dsn, filepath.Join(t.TempDir(), "nao-deve-criar.db"))
		if err == nil {
			_ = database.Close()
			t.Fatalf("%q: esperava erro, abriu %s (%s)", dsn, database.Dialect, database.Target)
		}
		if !strings.Contains(err.Error(), "DATABASE_URL não reconhecida") {
			t.Fatalf("%q: erro inesperado: %v", dsn, err)
		}
	}
}

// TestMigrateIdempotentAndAdditive reaplica o schema e confere as colunas e tabelas da v2.
func TestMigrateIdempotentAndAdditive(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, d *db.DB) {
		ctx := context.Background()
		for i := 0; i < 2; i++ {
			if err := d.Migrate(ctx); err != nil {
				t.Fatalf("Migrate #%d: %v", i+1, err)
			}
		}
		for _, q := range []string{
			`SELECT spend_type FROM categories`,
			`SELECT recurring_id FROM transactions`,
			`SELECT kind FROM goals`,
			`SELECT id, description, kind, amount_cents, day_of_month, account_id, category_id, start_date, end_date, active FROM recurring`,
			`SELECT key, value FROM settings`,
			`SELECT id, pattern, category_id, priority FROM category_rules`,
			`SELECT id, name, price_cents, url, notes, priority, status FROM wishlist`,
		} {
			rows, err := d.Query(ctx, q)
			if err != nil {
				t.Fatalf("%s: %v", q, err)
			}
			_ = rows.Close()
		}
		// Segunda reserva ativa viola o índice único parcial.
		ins := `INSERT INTO goals (id, name, icon, target_cents, kind, archived, created_at, updated_at) VALUES (?, ?, 'shield', 100, ?, ?, ?, ?)`
		now := "2026-01-01T00:00:00.000000000Z"
		if _, err := d.Exec(ctx, ins, uuid.NewString(), "R1", "RESERVE", false, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Exec(ctx, ins, uuid.NewString(), "R2", "RESERVE", false, now, now); err == nil {
			t.Fatal("segunda reserva ativa deveria violar idx_goals_single_reserve")
		}
		if _, err := d.Exec(ctx, ins, uuid.NewString(), "R3", "RESERVE", true, now, now); err != nil {
			t.Fatalf("reserva arquivada não deveria conflitar: %v", err)
		}
	})
}

// TestMigrateUpgradesLegacySQLite simula um banco criado antes da v2 (sem as colunas novas).
func TestMigrateUpgradesLegacySQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legado.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL, kind TEXT NOT NULL, icon TEXT NOT NULL, color TEXT NOT NULL, parent_id TEXT, UNIQUE(name, kind))`,
		`CREATE TABLE goals (id TEXT PRIMARY KEY, name TEXT NOT NULL, icon TEXT NOT NULL DEFAULT 'piggy-bank', color TEXT, target_cents INTEGER NOT NULL, deadline TEXT, archived INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`INSERT INTO categories VALUES ('c1', 'Aluguel', 'EXPENSE', 'house', '#ffffff', NULL)`,
		`INSERT INTO goals (id, name, target_cents, created_at, updated_at) VALUES ('g1', 'Viagem', 100, 'x', 'x')`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	_ = raw.Close()

	d, err := db.Open(context.Background(), "", path)
	if err != nil {
		t.Fatalf("db.Open em banco legado: %v", err)
	}
	defer d.Close()
	var kind string
	var spend sql.NullString
	if err := d.QueryRow(context.Background(), `SELECT kind FROM goals WHERE id = 'g1'`).Scan(&kind); err != nil || kind != "GOAL" {
		t.Fatalf("goals.kind = %q (%v), esperado GOAL", kind, err)
	}
	if err := d.QueryRow(context.Background(), `SELECT spend_type FROM categories WHERE id = 'c1'`).Scan(&spend); err != nil || spend.Valid {
		t.Fatalf("spend_type = %+v (%v), esperado NULL", spend, err)
	}
}
