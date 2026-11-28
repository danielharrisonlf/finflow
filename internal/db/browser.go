//go:build js && wasm

package db

// Bancos do runtime do navegador: cada espaço é um arquivo no VFS do worker ("/finflow/<slug>.db" no
// opfs-sahpool; em memória quando o navegador não tem OPFS). As funções que esperam Promises
// bloqueiam a goroutine chamadora: nunca as chame direto de um callback js.FuncOf.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"syscall/js"
)

// Open no navegador: PostgreSQL não existe; qualquer caminho SQLite vira um arquivo do VFS do runtime.
func Open(ctx context.Context, databaseURL, dbPath string) (*DB, error) {
	lower := strings.ToLower(strings.TrimSpace(databaseURL))
	switch {
	case lower == "":
	case strings.HasPrefix(lower, "sqlite://"):
		dbPath = strings.TrimSpace(databaseURL)[len("sqlite://"):]
	case strings.HasPrefix(lower, "file:"):
		dbPath = strings.TrimSpace(databaseURL)[len("file:"):]
	default:
		return nil, errors.New("no navegador só há SQLite local (DATABASE_URL não é suportada)")
	}
	if dbPath == "" {
		dbPath = "finflow.db"
	}
	return OpenBrowser(ctx, dbPath)
}

// OpenBrowser abre (ou cria) o arquivo name no VFS do runtime e aplica o schema.
func OpenBrowser(ctx context.Context, name string) (*DB, error) {
	d, err := OpenBrowserRaw(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := d.Migrate(ctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("aplicar schema (%s): %w", name, err)
	}
	return d, nil
}

// OpenBrowserRaw abre o arquivo sem aplicar o schema do FinFlow (ex.: o catálogo de espaços).
func OpenBrowserRaw(ctx context.Context, name string) (*DB, error) {
	if !isMemoryName(name) {
		if err := BrowserEnsureCapacity(); err != nil {
			return nil, err
		}
	}
	sqlDB, err := sql.Open(DriverSQLiteJS, name)
	if err != nil {
		return nil, fmt.Errorf("abrir sqlite do navegador: %w", err)
	}
	return finishBrowserOpen(ctx, sqlDB, name)
}

// OpenBrowserBytes abre em memória uma cópia de um banco (validação de backup antes de importar).
func OpenBrowserBytes(ctx context.Context, data []byte) (*DB, error) {
	sqlDB := sql.OpenDB(&jsConnector{name: ":memory:", init: data})
	return finishBrowserOpen(ctx, sqlDB, ":memory:")
}

func finishBrowserOpen(ctx context.Context, sqlDB *sql.DB, target string) (*DB, error) {
	// Uma conexão: o sahpool só permite um handle por arquivo, e um banco em memória só existe na
	// conexão que o criou.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(0)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("conectar ao banco (%s): %w", target, err)
	}
	return &DB{SQL: sqlDB, Dialect: DialectSQLite, Target: target}, nil
}

// BrowserPersistent indica se o runtime guarda os dados (OPFS) ou está só em memória.
func BrowserPersistent() bool {
	h, err := host()
	return err == nil && h.Get("persistent").Truthy()
}

// BrowserEnv devolve a configuração passada pelo JS (FINFLOW_TZ etc.).
func BrowserEnv() map[string]string {
	out := map[string]string{}
	h, err := host()
	if err != nil {
		return out
	}
	env := h.Get("env")
	if env.Type() != js.TypeObject {
		return out
	}
	keys := js.Global().Get("Object").Call("keys", env)
	for i := 0; i < keys.Length(); i++ {
		k := keys.Index(i).String()
		if v := env.Get(k); v.Type() == js.TypeString {
			out[k] = v.String()
		}
	}
	return out
}

// BrowserFileExists indica se o arquivo existe no VFS.
func BrowserFileExists(name string) bool {
	h, err := host()
	if err != nil {
		return false
	}
	var ok bool
	_ = jsCall(func() { ok = h.Call("exists", name).Truthy() })
	return ok
}

// BrowserFiles lista os arquivos de banco do VFS.
func BrowserFiles() ([]string, error) {
	h, err := host()
	if err != nil {
		return nil, err
	}
	var out []string
	err = jsCall(func() {
		list := h.Call("list")
		for i := 0; i < list.Length(); i++ {
			out = append(out, list.Index(i).String())
		}
	})
	return out, err
}

// BrowserExport devolve os bytes do banco: serializado pela conexão aberta, se houver, ou lido do VFS.
func BrowserExport(name string) ([]byte, error) {
	if data, ok, err := exportOpen(name); ok {
		return data, err
	}
	h, err := host()
	if err != nil {
		return nil, err
	}
	var out []byte
	err = jsCall(func() { out = bytesFromJS(h.Call("exportFile", name)) })
	return out, err
}

// BrowserImport grava bytes como o arquivo name (sobrescreve). O arquivo não pode estar aberto.
func BrowserImport(name string, data []byte) error {
	h, err := host()
	if err != nil {
		return err
	}
	if err := BrowserEnsureCapacity(); err != nil {
		return err
	}
	var p js.Value
	if err := jsCall(func() { p = h.Call("importFile", name, bytesToJS(data)) }); err != nil {
		return err
	}
	_, err = Await(p)
	return err
}

// BrowserRemove apaga o arquivo do VFS (sem erro se não existir). O arquivo não pode estar aberto.
func BrowserRemove(name string) error {
	h, err := host()
	if err != nil {
		return err
	}
	return jsCall(func() { h.Call("unlink", name) })
}

// BrowserWipe apaga todos os arquivos do VFS. Todos os bancos precisam estar fechados.
func BrowserWipe() error {
	h, err := host()
	if err != nil {
		return err
	}
	var p js.Value
	if err := jsCall(func() { p = h.Call("wipe") }); err != nil {
		return err
	}
	_, err = Await(p)
	return err
}

// BrowserEnsureCapacity garante vagas livres no pool do sahpool (cada banco usa um arquivo e o seu
// journal) antes de criar um arquivo novo.
func BrowserEnsureCapacity() error {
	h, err := host()
	if err != nil {
		return err
	}
	var p js.Value
	if err := jsCall(func() { p = h.Call("ensureCapacity") }); err != nil {
		return err
	}
	_, err = Await(p)
	return err
}

// Await espera uma Promise JS. Bloqueia a goroutine atual (não use dentro de js.FuncOf).
func Await(p js.Value) (js.Value, error) {
	if p.Type() != js.TypeObject || p.Get("then").Type() != js.TypeFunction {
		return p, nil
	}
	type result struct {
		v   js.Value
		err error
	}
	ch := make(chan result, 1)
	onOK := js.FuncOf(func(_ js.Value, args []js.Value) any {
		v := js.Undefined()
		if len(args) > 0 {
			v = args[0]
		}
		ch <- result{v: v}
		return nil
	})
	onErr := js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "erro desconhecido"
		if len(args) > 0 {
			msg = jsErrorMessage(args[0])
		}
		ch <- result{err: errors.New(msg)}
		return nil
	})
	defer onOK.Release()
	defer onErr.Release()
	p.Call("then", onOK, onErr)
	r := <-ch
	return r.v, r.err
}
