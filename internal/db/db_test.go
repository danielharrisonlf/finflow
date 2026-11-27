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
