package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// FoldFunc é a função escalar registrada no SQLite para comparação sem
// distinção de maiúsculas/minúsculas com suporte a Unicode (LOWER/LIKE nativos
// do SQLite só tratam ASCII). Os drivers ficam em arquivos com build tags:
// open_native.go (modernc + pgx) e sqlitejs.go (navegador, js/wasm).
const FoldFunc = "finflow_fold"

type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

type DB struct {
	SQL     *sql.DB
	Dialect Dialect
	Target  string
}

type Tx struct {
	tx      *sql.Tx
	dialect Dialect
}

// normalizePostgresURL adapta URLs no estilo Prisma para o pgx:
//   - "schema=X" vira "search_path=X" (o Postgres não conhece o parâmetro "schema");
//   - força "timezone=UTC" quando não informado, para que datas gravadas como
//     'YYYY-MM-DD' em colunas TIMESTAMPTZ não mudem de dia ao serem lidas.
func normalizePostgresURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	q := u.Query()
	if q.Has("schema") {
		if !q.Has("search_path") && q.Get("schema") != "" {
			q.Set("search_path", q.Get("schema"))
		}
		q.Del("schema")
	}
	hasTZ := false
	for k := range q {
		if strings.EqualFold(k, "timezone") {
			hasTZ = true
			break
		}
	}
	if !hasTZ {
		q.Set("timezone", "UTC")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (d *DB) Close() error {
	return d.SQL.Close()
}

func (d *DB) Rebind(query string) string {
	return rebind(d.Dialect, query)
}

func rebind(dialect Dialect, query string) string {
	if dialect != DialectPostgres {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 16)
	idx := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(idx))
			idx++
		} else {
			b.WriteByte(query[i])
		}
	}
	return b.String()
}

func (d *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.SQL.ExecContext(ctx, d.Rebind(query), args...)
}

func (d *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.SQL.QueryContext(ctx, d.Rebind(query), args...)
}

func (d *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return d.SQL.QueryRowContext(ctx, d.Rebind(query), args...)
}

func (d *DB) BeginTx(ctx context.Context) (*Tx, error) {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx, dialect: d.Dialect}, nil
}

func (t *Tx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, rebind(t.dialect, query), args...)
}

func (t *Tx) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, rebind(t.dialect, query), args...)
}

func (t *Tx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, rebind(t.dialect, query), args...)
}

func (t *Tx) Commit() error {
	return t.tx.Commit()
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

// DateText lê colunas de data tanto do SQLite (TEXT "YYYY-MM-DD") quanto do PostgreSQL (DATE -> time.Time).
type DateText string

func (dt *DateText) Scan(src any) error {
	if src == nil {
		*dt = ""
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*dt = DateText(v.UTC().Format("2006-01-02"))
	case string:
		if len(v) >= 10 {
			*dt = DateText(v[:10])
		} else {
			*dt = DateText(v)
		}
	case []byte:
		s := string(v)
		if len(s) >= 10 {
			*dt = DateText(s[:10])
		} else {
			*dt = DateText(s)
		}
	default:
		return fmt.Errorf("DateText: tipo não suportado %T", src)
	}
	return nil
}

type NullDateText struct {
	String string
	Valid  bool
}

func (nd *NullDateText) Scan(src any) error {
	if src == nil {
		nd.String = ""
		nd.Valid = false
		return nil
	}
	var dt DateText
	if err := dt.Scan(src); err != nil {
		return err
	}
	nd.String = string(dt)
	nd.Valid = nd.String != ""
	return nil
}

func (nd NullDateText) Ptr() *string {
	if !nd.Valid {
		return nil
	}
	s := nd.String
	return &s
}

func NullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func NullIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}
