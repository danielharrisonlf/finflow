//go:build !(js && wasm)

package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction(FoldFunc, 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		switch v := args[0].(type) {
		case nil:
			return nil, nil
		case string:
			return strings.ToLower(v), nil
		case []byte:
			return strings.ToLower(string(v)), nil
		default:
			return v, nil
		}
	})
}

// Open abre SQLite por padrão (ou PostgreSQL quando databaseURL começa com postgres:// ou postgresql://)
// e garante que o schema está criado.
func Open(ctx context.Context, databaseURL, dbPath string) (*DB, error) {
	var (
		sqlDB   *sql.DB
		dialect Dialect
		target  string
		err     error
	)

	lower := strings.ToLower(strings.TrimSpace(databaseURL))
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		dialect = DialectPostgres
		target = "PostgreSQL"
		dsn, nErr := normalizePostgresURL(databaseURL)
		if nErr != nil {
			return nil, fmt.Errorf("URL do postgres inválida: %w", nErr)
		}
		sqlDB, err = sql.Open("pgx", dsn)
		if err != nil {
			return nil, fmt.Errorf("abrir postgres: %w", err)
		}
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
	} else if lower != "" && !strings.HasPrefix(lower, "sqlite://") && !strings.HasPrefix(lower, "file:") {
		// Evita cair no SQLite em silêncio com um DSN que o usuário acha que é PostgreSQL
		// (ex.: "host=localhost dbname=finflow").
		return nil, errors.New("DATABASE_URL não reconhecida (formatos aceitos: postgres://, postgresql://, sqlite:// ou file:)")
	} else {
		dialect = DialectSQLite
		path := dbPath
		if strings.HasPrefix(lower, "sqlite://") {
			path = strings.TrimPrefix(databaseURL, "sqlite://")
		} else if strings.HasPrefix(lower, "file:") {
			path = strings.TrimPrefix(databaseURL, "file:")
		}
		if path == "" {
			path = "finflow.db"
		}
		target = path
		dsn := sqliteDSN(path)
		if path != ":memory:" && !strings.HasPrefix(path, "file::memory:") {
			if dir := filepath.Dir(path); dir != "" && dir != "." {
				if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
					return nil, fmt.Errorf("criar diretório do banco %q: %w", dir, mkErr)
				}
			}
		}
		sqlDB, err = sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("abrir sqlite: %w", err)
		}
		// SQLite em modo single-writer evita SQLITE_BUSY sob escritas concorrentes.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("conectar ao banco (%s): %w", dialect, err)
	}

	d := &DB{SQL: sqlDB, Dialect: dialect, Target: target}
	if err := d.Migrate(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("aplicar schema (%s): %w", dialect, err)
	}
	return d, nil
}

func sqliteDSN(path string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	if path == ":memory:" {
		return path + sep + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	return path + sep + "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}
