// Package testutil oferece bancos isolados para testes de integração.
//
// Por padrão todo teste roda em SQLite em memória. Quando FINFLOW_TEST_DATABASE_URL
// aponta para um PostgreSQL, os mesmos testes também rodam nele, cada um em um
// schema descartável — assim o dialeto Postgres é exercitado com a mesma suíte.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"finflow/internal/db"
)

// EnvPostgresURL é a variável que habilita a execução dos testes também em PostgreSQL.
const EnvPostgresURL = "FINFLOW_TEST_DATABASE_URL"

// Dialects devolve os dialetos disponíveis para este processo de teste.
func Dialects() []db.Dialect {
	out := []db.Dialect{db.DialectSQLite}
	if strings.TrimSpace(os.Getenv(EnvPostgresURL)) != "" {
		out = append(out, db.DialectPostgres)
	}
	return out
}

// ForEachDialect executa fn como subteste para cada dialeto disponível,
// com um banco novo e vazio (schema já aplicado).
func ForEachDialect(t *testing.T, fn func(t *testing.T, database *db.DB)) {
	t.Helper()
	for _, d := range Dialects() {
		t.Run(string(d), func(t *testing.T) {
			fn(t, Open(t, d))
		})
	}
}

// Open abre um banco isolado do dialeto pedido e registra a limpeza no t.Cleanup.
func Open(t testing.TB, dialect db.Dialect) *db.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch dialect {
	case db.DialectSQLite:
		database, err := db.Open(ctx, "", ":memory:")
		if err != nil {
			t.Fatalf("abrir sqlite em memória: %v", err)
		}
		t.Cleanup(func() { _ = database.Close() })
		return database

	case db.DialectPostgres:
		base := strings.TrimSpace(os.Getenv(EnvPostgresURL))
		if base == "" {
			t.Skipf("%s não definido", EnvPostgresURL)
		}
		schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")

		admin, err := sql.Open("pgx", base)
		if err != nil {
			t.Fatalf("abrir postgres: %v", err)
		}
		defer admin.Close()
		if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA %q`, schema)); err != nil {
			t.Fatalf("criar schema de teste: %v", err)
		}
		t.Cleanup(func() {
			c, err := sql.Open("pgx", base)
			if err != nil {
				return
			}
			defer c.Close()
			_, _ = c.ExecContext(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema))
		})

		dsn, err := withSearchPath(base, schema)
		if err != nil {
			t.Fatalf("montar DSN: %v", err)
		}
		database, err := db.Open(ctx, dsn, "")
		if err != nil {
			t.Fatalf("abrir postgres de teste: %v", err)
		}
		t.Cleanup(func() { _ = database.Close() })
		return database
	}
	t.Fatalf("dialeto desconhecido: %s", dialect)
	return nil
}

func withSearchPath(rawURL, schema string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Del("schema")
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
