//go:build js && wasm

// Command finflow-wasm é a API do FinFlow compilada para WebAssembly (GOOS=js GOARCH=wasm), rodando
// num Dedicated Worker do navegador (docs/browser-runtime-contract.md §1–§2). O worker
// (apps/web/src/runtime/worker.ts) publica globalThis.__finflowHost (SQLite WASM + VFS opfs-sahpool,
// configuração) antes de iniciar este programa, que exporta globalThis.__finflow:
//
//	handle(req, onChunk[, onHead]) → Promise<{status, headers}>   httpapi.Server.ServeHTTP em processo
//	exportDb(space) → Uint8Array (ou Error)                       bytes do banco do espaço
//	importDb(space, bytes) → Promise<void>                        valida (Migrate + Bootstrap) e substitui
//	reset() → Promise<void>                                       apaga todos os espaços e o catálogo
//	info() → {version, persistent, spaces}
//
// Callbacks js.FuncOf nunca bloqueiam: o trabalho roda numa goroutine e o resultado volta por Promise.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall/js"
	"time"
	_ "time/tzdata" // o navegador não tem a base de fusos do sistema

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/spaces"
	"finflow/internal/store"
)

// version é preenchida no build: -ldflags "-X main.version=v0.4.0".
var version = "dev"

// Arquivos no VFS do runtime: um banco por espaço e o catálogo.
const (
	dataPrefix   = "/finflow/"
	personalFile = dataPrefix + domain.DefaultSpaceSlug + ".db"
	catalogFile  = dataPrefix + "_catalog.db"
)

// chunkSize é o quanto o ResponseWriter acumula antes de repassar um pedaço ao JS sem Flush.
const chunkSize = 64 << 10

// app é uma instância aberta da API (bancos + serviços + servidor). Importar ou apagar dados fecha a
// instância e abre outra.
type app struct {
	personal *db.DB
	catalog  *db.DB
	reg      *spaces.Registry
	server   *httpapi.Server
	// ctx é cancelado ao fechar a instância (aborta requisições em andamento, ex.: SSE).
	ctx      context.Context
	cancel   context.CancelFunc
	inflight sync.WaitGroup
}

var (
	lifeMu  sync.RWMutex // protege current
	current *app
	opMu    sync.Mutex // serializa importDb/reset
	// spacesCache alimenta info() sem tocar no banco dentro do callback síncrono.
	spacesCache atomic.Pointer[[]string]
)

func main() {
	for k, v := range db.BrowserEnv() {
		_ = os.Setenv(k, v)
	}
	host := js.Global().Get("__finflowHost")
	a, err := openApp(context.Background())
	if err != nil {
		slog.Error("runtime do FinFlow não iniciou", "err", err)
		if f := host.Get("onError"); f.Type() == js.TypeFunction {
			f.Invoke(err.Error())
		}
		return
	}
	setCurrent(a)

	js.Global().Set("__finflow", js.ValueOf(map[string]any{
		"handle":   js.FuncOf(jsHandle),
		"exportDb": js.FuncOf(jsExportDb),
		"importDb": js.FuncOf(jsImportDb),
		"reset":    js.FuncOf(jsReset),
		"info":     js.FuncOf(jsInfo),
	}))
	if f := host.Get("onReady"); f.Type() == js.TypeFunction {
		f.Invoke()
	}
	select {}
}

// openApp abre o espaço pessoal, o catálogo e monta os serviços e o servidor HTTP em processo.
func openApp(ctx context.Context) (*app, error) {
	personal, err := db.OpenBrowser(ctx, personalFile)
	if err != nil {
		return nil, err
	}
	st := store.New(personal)
	if err := store.Bootstrap(ctx, st); err != nil {
		_ = personal.Close()
		return nil, err
	}
	catDB, err := db.OpenBrowserRaw(ctx, catalogFile)
	if err != nil {
		_ = personal.Close()
		return nil, err
	}
	cat, err := spaces.NewSQLCatalog(ctx, catDB)
	if err != nil {
		_ = catDB.Close()
		_ = personal.Close()
		return nil, err
	}
	svcs := service.New(st, nil)
	reg := spaces.New(svcs, spaces.Options{Catalog: cat, Files: spaces.BrowserFiles{Prefix: dataPrefix}})
	server := httpapi.NewServer(svcs, httpapi.ServerOptions{
		Version: version,
		Spaces:  reg,
		Runtime: httpapi.RuntimeBrowser,
	})
	actx, cancel := context.WithCancel(context.Background())
	a := &app{personal: personal, catalog: catDB, reg: reg, server: server, ctx: actx, cancel: cancel}
	a.refreshSpaces()
	return a, nil
}

func (a *app) refreshSpaces() {
	list, err := a.reg.List()
	if err != nil {
		return
	}
	slugs := make([]string, 0, len(list))
	for _, sp := range list {
		slugs = append(slugs, sp.Slug)
	}
	spacesCache.Store(&slugs)
}

// close aborta as requisições em andamento, espera até 5 s que terminem e fecha os bancos.
func (a *app) close() error {
	a.cancel()
	done := make(chan struct{})
	go func() { a.inflight.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		slog.Warn("requisições ainda em andamento ao fechar o runtime")
	}
	return errors.Join(a.reg.Close(), a.catalog.Close(), a.personal.Close())
}

func setCurrent(a *app) {
	lifeMu.Lock()
	current = a
	lifeMu.Unlock()
}

// takeCurrent tira a instância de uso (novas requisições recebem 503 até a próxima abrir).
func takeCurrent() *app {
	lifeMu.Lock()
	defer lifeMu.Unlock()
	a := current
	current = nil
	return a
}

// acquire devolve a instância atual já contada como requisição em andamento.
func acquire() *app {
	lifeMu.RLock()
	defer lifeMu.RUnlock()
	if current != nil {
		current.inflight.Add(1)
	}
	return current
}

// ───────────────────────── Promises ─────────────────────────

// newPromise roda fn numa goroutine e devolve uma Promise resolvida com o valor (ou rejeitada com o erro).
func newPromise(fn func() (any, error)) js.Value {
	executor := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve, reject := args[0], args[1]
		go func() {
			var (
				v   any
				err error
			)
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("falha interna do runtime: %v", r)
					}
				}()
				v, err = fn()
			}()
			if err != nil {
				reject.Invoke(js.Global().Get("Error").New(err.Error()))
				return
			}
			resolve.Invoke(v)
		}()
		return nil
	})
	p := js.Global().Get("Promise").New(executor)
	executor.Release()
	return p
}

func jsError(msg string) js.Value { return js.Global().Get("Error").New(msg) }

// ───────────────────────── handle ─────────────────────────

type jsRequest struct {
	method  string
	url     string
	headers http.Header
	body    []byte
	signal  js.Value
}

// readRequest copia a requisição JS para Go (síncrono, sem bloquear).
func readRequest(v js.Value) jsRequest {
	req := jsRequest{method: v.Get("method").String(), url: v.Get("url").String(), headers: http.Header{}, signal: v.Get("signal")}
	if hs := v.Get("headers"); hs.Type() == js.TypeObject {
		for i := 0; i < hs.Length(); i++ {
			pair := hs.Index(i)
			req.headers.Add(pair.Index(0).String(), pair.Index(1).String())
		}
	}
	if b := v.Get("body"); b.Type() == js.TypeObject {
		req.body = make([]byte, b.Get("byteLength").Int())
		js.CopyBytesToGo(req.body, b)
	}
	return req
}

func jsHandle(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return newPromise(func() (any, error) { return nil, errors.New("handle(req, onChunk) exige 2 argumentos") })
	}
	in := readRequest(args[0])
	onChunk := args[1]
	onHead := js.Undefined()
	if len(args) > 2 {
		onHead = args[2]
	}

	// O sinal de abort (opcional, do worker) cancela o contexto da requisição.
	ctx, cancel := context.WithCancel(context.Background())
	var abortFn js.Func
	hasSignal := in.signal.Type() == js.TypeObject
	if hasSignal {
		if in.signal.Get("aborted").Truthy() {
			cancel()
		}
		abortFn = js.FuncOf(func(js.Value, []js.Value) any { cancel(); return nil })
		in.signal.Call("addEventListener", "abort", abortFn)
	}

	return newPromise(func() (any, error) {
		defer cancel()
		if hasSignal {
			defer func() {
				in.signal.Call("removeEventListener", "abort", abortFn)
				abortFn.Release()
			}()
		}
		w := &streamWriter{header: http.Header{}, onChunk: onChunk, onHead: onHead}
		serve(ctx, in, w)
		w.finish()
		return map[string]any{"status": w.status, "headers": headerPairs(w.sentHeader)}, nil
	})
}

func serve(ctx context.Context, in jsRequest, w *streamWriter) {
	a := acquire()
	if a == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"statusCode":503,"message":"O FinFlow está reiniciando os dados. Tente de novo."}`)
		return
	}
	defer a.inflight.Done()
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	unlink := context.AfterFunc(a.ctx, cancel)
	defer unlink()

	path := in.url
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	r, err := http.NewRequestWithContext(reqCtx, in.method, "http://localhost"+path, bytes.NewReader(in.body))
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"statusCode":400,"message":"requisição inválida"}`)
		return
	}
	r.Header = in.headers
	r.RemoteAddr = "127.0.0.1:0"
	r.RequestURI = path
	func() {
		defer func() {
			if rec := recover(); rec != nil && !errors.Is(asError(rec), http.ErrAbortHandler) {
				slog.Error("panic no handler do runtime", "path", path, "panic", fmt.Sprint(rec))
			}
		}()
		a.server.ServeHTTP(w, r)
	}()
	if in.method != http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/spaces") {
		a.refreshSpaces()
	}
}

func asError(v any) error {
	if err, ok := v.(error); ok {
		return err
	}
	return nil
}

// streamWriter é o http.ResponseWriter do runtime: os cabeçalhos vão para onHead quando são
// enviados e o corpo vai para onChunk a cada Flush (ou a cada chunkSize bytes).
type streamWriter struct {
	header      http.Header
	sentHeader  http.Header
	status      int
	wroteHeader bool
	headSent    bool
	buf         []byte
	onChunk     js.Value
	onHead      js.Value
}

func (w *streamWriter) Header() http.Header { return w.header }

func (w *streamWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.sentHeader = w.header.Clone()
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		if w.header.Get("Content-Type") == "" && len(p) > 0 {
			w.header.Set("Content-Type", http.DetectContentType(p))
		}
		w.WriteHeader(http.StatusOK)
	}
	w.buf = append(w.buf, p...)
	if len(w.buf) >= chunkSize {
		w.sendHead()
		w.flushChunk()
	}
	return len(p), nil
}

// Flush implementa http.Flusher (usado pelo SSE do assistente via http.ResponseController).
func (w *streamWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	w.sendHead()
	w.flushChunk()
}

func (w *streamWriter) sendHead() {
	if w.headSent {
		return
	}
	w.headSent = true
	if w.onHead.Type() == js.TypeFunction {
		w.onHead.Invoke(w.status, headerPairs(w.sentHeader))
	}
}

func (w *streamWriter) flushChunk() {
	if len(w.buf) == 0 {
		return
	}
	arr := js.Global().Get("Uint8Array").New(len(w.buf))
	js.CopyBytesToJS(arr, w.buf)
	w.buf = w.buf[:0]
	w.onChunk.Invoke(arr)
}

func (w *streamWriter) finish() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	w.sendHead()
	w.flushChunk()
}

func headerPairs(h http.Header) []any {
	out := []any{}
	for k, vs := range h {
		for _, v := range vs {
			out = append(out, []any{strings.ToLower(k), v})
		}
	}
	return out
}

// ───────────────────────── dados ─────────────────────────

// spaceFile devolve o arquivo do espaço (slug vazio = pessoal).
func spaceFile(slug string) (string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || slug == domain.DefaultSpaceSlug {
		return personalFile, nil
	}
	if !domain.IsValidSpaceSlug(slug) {
		return "", fmt.Errorf("espaço inválido: %q", slug)
	}
	return dataPrefix + slug + ".db", nil
}

func jsExportDb(_ js.Value, args []js.Value) any {
	slug := ""
	if len(args) > 0 && args[0].Type() == js.TypeString {
		slug = args[0].String()
	}
	file, err := spaceFile(slug)
	if err != nil {
		return jsError(err.Error())
	}
	data, err := db.BrowserExport(file)
	if err != nil {
		return jsError(fmt.Sprintf("exportar espaço %q: %v", slug, err))
	}
	arr := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(arr, data)
	return arr
}

func jsImportDb(_ js.Value, args []js.Value) any {
	if len(args) < 2 || args[1].Type() != js.TypeObject {
		return newPromise(func() (any, error) { return nil, errors.New("importDb(space, bytes) exige os bytes do banco") })
	}
	slug := strings.ToLower(strings.TrimSpace(args[0].String()))
	data := make([]byte, args[1].Get("byteLength").Int())
	js.CopyBytesToGo(data, args[1])
	return newPromise(func() (any, error) { return nil, importDb(slug, data) })
}

// sqliteMagic é o cabeçalho de todo arquivo SQLite 3.
const sqliteMagic = "SQLite format 3\x00"

// importDb valida o backup numa cópia em memória (Migrate + Bootstrap do tipo do espaço) e só então
// substitui o arquivo; se a gravação falhar, o arquivo anterior volta.
func importDb(slug string, data []byte) error {
	opMu.Lock()
	defer opMu.Unlock()
	ctx := context.Background()
	if slug == "" {
		slug = domain.DefaultSpaceSlug
	}
	file, err := spaceFile(slug)
	if err != nil {
		return err
	}
	kind := domain.SpacePersonal
	if a := acquire(); a != nil {
		sp, gerr := a.reg.Get(slug)
		a.inflight.Done()
		if gerr != nil {
			return fmt.Errorf("espaço %q não existe: crie o espaço antes de restaurar o backup", slug)
		}
		kind = sp.Kind
	}
	if len(data) < 100 || string(data[:16]) != sqliteMagic {
		return errors.New("o arquivo não é um banco SQLite")
	}
	// Bancos do desktop usam WAL; o VFS do navegador não. Bytes 18/19 = 1 → journal comum.
	if data[18] == 2 || data[19] == 2 {
		data[18], data[19] = 1, 1
	}
	if err := validateBackup(ctx, data, kind); err != nil {
		return fmt.Errorf("backup inválido: %w", err)
	}

	backup, _ := db.BrowserExport(file) // nil se o arquivo ainda não existe
	old := takeCurrent()
	if old != nil {
		if err := old.close(); err != nil {
			slog.Warn("fechar runtime antes de importar", "err", err)
		}
	}
	importErr := db.BrowserImport(file, data)
	if importErr != nil && backup != nil {
		if rerr := db.BrowserImport(file, backup); rerr != nil {
			slog.Error("restaurar banco anterior após falha na importação", "err", rerr)
		}
	}
	a, err := openApp(ctx)
	if err != nil {
		return errors.Join(importErr, fmt.Errorf("reabrir o FinFlow: %w", err))
	}
	setCurrent(a)
	if importErr != nil {
		return fmt.Errorf("gravar o banco importado: %w", importErr)
	}
	return nil
}

func validateBackup(ctx context.Context, data []byte, kind domain.SpaceKind) error {
	mem, err := db.OpenBrowserBytes(ctx, append([]byte(nil), data...))
	if err != nil {
		return err
	}
	defer mem.Close()
	var check string
	if err := mem.QueryRow(ctx, `PRAGMA quick_check`).Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("banco corrompido (%s)", check)
	}
	if err := mem.Migrate(ctx); err != nil {
		return err
	}
	return store.BootstrapKind(ctx, store.New(mem), kind)
}

func jsReset(js.Value, []js.Value) any {
	return newPromise(func() (any, error) {
		opMu.Lock()
		defer opMu.Unlock()
		if old := takeCurrent(); old != nil {
			if err := old.close(); err != nil {
				slog.Warn("fechar runtime antes de apagar os dados", "err", err)
			}
		}
		wipeErr := db.BrowserWipe()
		a, err := openApp(context.Background())
		if err != nil {
			return nil, errors.Join(wipeErr, err)
		}
		setCurrent(a)
		return nil, wipeErr
	})
}

func jsInfo(js.Value, []js.Value) any {
	slugs := []any{}
	if cached := spacesCache.Load(); cached != nil {
		for _, s := range *cached {
			slugs = append(slugs, s)
		}
	}
	return js.ValueOf(map[string]any{
		"version":    version,
		"persistent": db.BrowserPersistent(),
		"spaces":     slugs,
	})
}
