//go:build js && wasm

package db

// Driver database/sql "sqlitejs": SQLite do navegador (@sqlite.org/sqlite-wasm, API oo1/capi) chamado
// de forma síncrona via syscall/js. O worker do runtime (apps/web/src/runtime/worker.ts) publica
// globalThis.__finflowHost com o módulo sqlite3 e as operações de arquivo (VFS opfs-sahpool ou memória);
// aqui só se preparam, ligam, executam e leem comandos.
//
// Tipos: INTEGER volta como int64 lido em texto (sem passar por float, então inteiros grandes não
// perdem precisão), REAL float64, TEXT string, BLOB []byte e NULL nil.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"
)

// DriverSQLiteJS é o nome do driver registrado em database/sql.
const DriverSQLiteJS = "sqlitejs"

// Códigos de tipo de coluna do SQLite (sqlite3_column_type).
const (
	sqliteInteger = 1
	sqliteFloat   = 2
	sqliteText    = 3
	sqliteBlob    = 4
	sqliteNull    = 5
)

func init() {
	sql.Register(DriverSQLiteJS, &jsDriver{})
}

// host devolve globalThis.__finflowHost (erro se o runtime JS não o publicou).
func host() (js.Value, error) {
	h := js.Global().Get("__finflowHost")
	if h.IsUndefined() || h.IsNull() {
		return js.Value{}, errors.New("sqlitejs: globalThis.__finflowHost ausente (runtime do navegador não iniciado)")
	}
	return h, nil
}

// jsCall converte exceções JS (que o syscall/js transforma em panic) em erro Go.
func jsCall(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if jsErr, ok := r.(js.Error); ok {
				err = errors.New(jsErrorMessage(jsErr.Value))
				return
			}
			err = fmt.Errorf("sqlitejs: %v", r)
		}
	}()
	f()
	return nil
}

func jsErrorMessage(v js.Value) string {
	if v.Type() == js.TypeObject {
		if m := v.Get("message"); m.Type() == js.TypeString {
			return m.String()
		}
	}
	return js.Global().Get("String").Invoke(v).String()
}

// bigIntString lê um BigInt (ou Number) JS como texto decimal.
func bigIntString(v js.Value) string {
	return js.Global().Get("String").Invoke(v).String()
}

// openConns guarda as conexões abertas por nome de arquivo (exportação de um banco em uso).
var openConns = struct {
	sync.Mutex
	m map[string]*jsConn
}{m: map[string]*jsConn{}}

type jsDriver struct{}

// Open abre o arquivo name no VFS do runtime (ex.: "/finflow/pessoal.db"; ":memory:" = memória).
func (jsDriver) Open(name string) (driver.Conn, error) {
	return openJSConn(name, nil)
}

// jsConnector abre conexões com bytes iniciais (validação de importação em memória).
type jsConnector struct {
	name string
	init []byte
}

func (c *jsConnector) Connect(context.Context) (driver.Conn, error) {
	return openJSConn(c.name, c.init)
}
func (c *jsConnector) Driver() driver.Driver { return jsDriver{} }

func openJSConn(name string, init []byte) (*jsConn, error) {
	h, err := host()
	if err != nil {
		return nil, err
	}
	c := &jsConn{name: name, host: h, sqlite3: h.Get("sqlite3")}
	c.capi = c.sqlite3.Get("capi")
	if err := jsCall(func() {
		if init != nil {
			c.db = c.sqlite3.Get("oo1").Get("DB").New(":memory:")
			c.name = ":memory:"
			h.Call("deserialize", c.db, bytesToJS(init))
		} else {
			c.db = h.Call("openDb", name)
		}
	}); err != nil {
		return nil, fmt.Errorf("abrir %s: %w", name, err)
	}
	if err := jsCall(func() {
		c.db.Call("exec", "PRAGMA foreign_keys = ON; PRAGMA temp_store = MEMORY;")
		// finflow_fold: minúsculas com Unicode (mesma função que o driver nativo registra).
		c.db.Call("createFunction", FoldFunc, foldFunc(), map[string]any{"arity": 1, "deterministic": true})
	}); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("preparar %s: %w", name, err)
	}
	if !isMemoryName(c.name) {
		openConns.Lock()
		openConns.m[c.name] = c
		openConns.Unlock()
	}
	return c, nil
}

var (
	foldOnce sync.Once
	foldJS   js.Value
)

// foldFunc é a implementação JS de finflow_fold, fornecida pelo host (o worker) para não depender
// de eval/new Function (CSP).
func foldFunc() js.Value {
	foldOnce.Do(func() {
		if h, err := host(); err == nil {
			foldJS = h.Get("fold")
		}
	})
	return foldJS
}

func bytesToJS(b []byte) js.Value {
	arr := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(arr, b)
	return arr
}

func bytesFromJS(v js.Value) []byte {
	out := make([]byte, v.Get("byteLength").Int())
	js.CopyBytesToGo(out, v)
	return out
}

// exportOpen serializa um banco aberto por este processo (nil, false se não estiver aberto).
func exportOpen(name string) ([]byte, bool, error) {
	openConns.Lock()
	c, ok := openConns.m[name]
	openConns.Unlock()
	if !ok {
		return nil, false, nil
	}
	var out []byte
	err := jsCall(func() {
		out = bytesFromJS(c.capi.Call("sqlite3_js_db_export", c.db))
	})
	return out, true, err
}

type jsConn struct {
	name    string
	host    js.Value
	sqlite3 js.Value
	capi    js.Value
	db      js.Value
	closed  bool
}

func (c *jsConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *jsConn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	var st js.Value
	if err := jsCall(func() { st = c.db.Call("prepare", query) }); err != nil {
		return nil, err
	}
	return &jsStmt{conn: c, st: st}, nil
}

func (c *jsConn) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	if !isMemoryName(c.name) {
		openConns.Lock()
		if openConns.m[c.name] == c {
			delete(openConns.m, c.name)
		}
		openConns.Unlock()
		return jsCall(func() { c.host.Call("closeDb", c.name, c.db) })
	}
	return jsCall(func() { c.db.Call("close") })
}

func (c *jsConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *jsConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	if err := c.execNoArgs("BEGIN"); err != nil {
		return nil, err
	}
	return &jsTx{conn: c}, nil
}

func (c *jsConn) Ping(context.Context) error {
	if c.closed {
		return driver.ErrBadConn
	}
	return nil
}

func (c *jsConn) execNoArgs(query string) error {
	return jsCall(func() { c.db.Call("exec", query) })
}

// ExecContext sem argumentos usa db.exec (aceita vários comandos separados por ";"); com argumentos
// prepara um único comando.
func (c *jsConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		if err := c.execNoArgs(query); err != nil {
			return nil, err
		}
		return c.result()
	}
	st, err := c.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	return st.(*jsStmt).ExecContext(ctx, args)
}

func (c *jsConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	st, err := c.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	s := st.(*jsStmt)
	rows, err := s.QueryContext(ctx, args)
	if err != nil {
		_ = s.Close()
		return nil, err
	}
	rows.(*jsRows).closeStmt = true
	return rows, nil
}

func (c *jsConn) result() (driver.Result, error) {
	var changes, lastID string
	if err := jsCall(func() {
		changes = bigIntString(c.db.Call("changes", false, true))
		lastID = bigIntString(c.capi.Call("sqlite3_last_insert_rowid", c.db.Get("pointer")))
	}); err != nil {
		return nil, err
	}
	n, _ := strconv.ParseInt(changes, 10, 64)
	id, _ := strconv.ParseInt(lastID, 10, 64)
	return jsResult{rows: n, lastID: id}, nil
}

type jsResult struct{ rows, lastID int64 }

func (r jsResult) LastInsertId() (int64, error) { return r.lastID, nil }
func (r jsResult) RowsAffected() (int64, error) { return r.rows, nil }

type jsTx struct{ conn *jsConn }

func (t *jsTx) Commit() error   { return t.conn.execNoArgs("COMMIT") }
func (t *jsTx) Rollback() error { return t.conn.execNoArgs("ROLLBACK") }

type jsStmt struct {
	conn   *jsConn
	st     js.Value
	closed bool
}

func (s *jsStmt) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return jsCall(func() { s.st.Call("finalize") })
}

func (s *jsStmt) NumInput() int {
	n := -1
	_ = jsCall(func() { n = s.st.Get("parameterCount").Int() })
	return n
}

func (s *jsStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), valuesToNamed(args))
}

func (s *jsStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), valuesToNamed(args))
}

func valuesToNamed(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, v := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}

// bind liga os argumentos em ordem: int64 vira BigInt (bind_int64 exato), float64 bind_double,
// bool 0/1, time.Time no mesmo formato de texto do modernc.
func (s *jsStmt) bind(args []driver.NamedValue) error {
	return jsCall(func() {
		s.st.Call("reset")
		s.st.Call("clearBindings")
		capi := s.conn.capi
		ptr := s.st.Get("pointer")
		for _, a := range args {
			if a.Name != "" {
				panic(fmt.Sprintf("parâmetros nomeados não são suportados (%s)", a.Name))
			}
			idx := a.Ordinal
			switch v := a.Value.(type) {
			case nil:
				s.st.Call("bind", idx, js.Null())
			case int64:
				s.st.Call("bind", idx, js.Global().Get("BigInt").Invoke(strconv.FormatInt(v, 10)))
			case float64:
				s.conn.checkRC(capi.Call("sqlite3_bind_double", ptr, idx, v))
			case bool:
				n := 0
				if v {
					n = 1
				}
				s.st.Call("bind", idx, n)
			case string:
				s.st.Call("bind", idx, v)
			case []byte:
				if v == nil {
					s.st.Call("bind", idx, js.Null())
				} else {
					s.st.Call("bind", idx, bytesToJS(v))
				}
			case time.Time:
				s.st.Call("bind", idx, v.Format("2006-01-02 15:04:05.999999999-07:00"))
			default:
				panic(fmt.Sprintf("tipo de argumento não suportado: %T", a.Value))
			}
		}
	})
}

func (c *jsConn) checkRC(rc js.Value) {
	if rc.Int() != 0 {
		c.db.Call("checkRc", rc)
	}
}

func (s *jsStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.bind(args); err != nil {
		return nil, err
	}
	if err := jsCall(func() {
		for s.st.Call("step").Bool() {
		}
		s.st.Call("reset")
	}); err != nil {
		return nil, err
	}
	return s.conn.result()
}

func (s *jsStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.bind(args); err != nil {
		return nil, err
	}
	var cols []string
	if err := jsCall(func() {
		names := s.st.Call("getColumnNames")
		cols = make([]string, names.Length())
		for i := range cols {
			cols[i] = names.Index(i).String()
		}
	}); err != nil {
		return nil, err
	}
	return &jsRows{stmt: s, cols: cols}, nil
}

type jsRows struct {
	stmt      *jsStmt
	cols      []string
	done      bool
	closeStmt bool
}

func (r *jsRows) Columns() []string { return r.cols }

func (r *jsRows) Close() error {
	if r.closeStmt {
		return r.stmt.Close()
	}
	return jsCall(func() { r.stmt.st.Call("reset") })
}

func (r *jsRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	var hasRow bool
	err := jsCall(func() {
		st := r.stmt.st
		hasRow = st.Call("step").Bool()
		if !hasRow {
			return
		}
		capi := r.stmt.conn.capi
		ptr := st.Get("pointer")
		for i := range dest {
			switch capi.Call("sqlite3_column_type", ptr, i).Int() {
			case sqliteInteger:
				// Texto decimal do próprio SQLite: exato para todo int64.
				n, perr := strconv.ParseInt(capi.Call("sqlite3_column_text", ptr, i).String(), 10, 64)
				if perr != nil {
					panic(perr.Error())
				}
				dest[i] = n
			case sqliteFloat:
				dest[i] = capi.Call("sqlite3_column_double", ptr, i).Float()
			case sqliteText:
				dest[i] = capi.Call("sqlite3_column_text", ptr, i).String()
			case sqliteBlob:
				dest[i] = bytesFromJS(st.Call("get", i, sqliteBlob))
			case sqliteNull:
				dest[i] = nil
			}
		}
	})
	if err != nil {
		return err
	}
	if !hasRow {
		r.done = true
		return io.EOF
	}
	return nil
}

// isMemoryName indica bancos sem arquivo.
func isMemoryName(name string) bool {
	return name == "" || strings.HasPrefix(name, ":memory:")
}
