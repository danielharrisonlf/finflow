# FinFlow no navegador — contrato do runtime

Objetivo: o usuário abre `https://finflow-landing-umber.vercel.app/app/`, instala como PWA (computador ou celular)
e usa o FinFlow completo **sem baixar nada**, com os dados guardados **no próprio navegador**. Nada vai para um
servidor. O binário desktop continua existindo e usa o mesmo formato de banco (arquivo SQLite), então um backup
`.db` do navegador abre no desktop e vice-versa.

## 1. Arquitetura

```
página (React)                         Dedicated Worker
  api.ts / ai.ts / downloads  ──msg──▶  runtime worker
        ▲  Response (stream)            ├─ @sqlite.org/sqlite-wasm (oo1, síncrono) + VFS "opfs-sahpool"
        └──────────────────────────────  └─ finflow.wasm (Go, GOOS=js GOARCH=wasm)
                                             internal/httpapi Server.ServeHTTP em processo
```

- **Mesmo código de API**: o Go compilado para WebAssembly monta `service.Services` + `httpapi.Server` e atende as
  requisições em processo (sem rede), com um `http.ResponseWriter` próprio que repassa os bytes ao JS conforme
  são escritos (`Flush` → chunk). Todas as rotas `/api/*` funcionam iguais, incluindo SSE do assistente, uploads
  multipart e downloads.
- **SQLite no navegador**: `@sqlite.org/sqlite-wasm` (pacote oficial do sqlite.org, versão fixa publicada há pelo
  menos 2 semanas) com o VFS `opfs-sahpool` (persistente, transacional, síncrono num Dedicated Worker, não exige
  COOP/COEP). Sem OPFS disponível: banco em memória + aviso visível de que os dados não serão guardados.
- **Driver Go**: `database/sql/driver` registrado como `"sqlitejs"` (só em `js && wasm`) que chama a API oo1 do
  SQLite via `syscall/js` de forma síncrona: prepare/bind/step/finalize, `changes()`, `last_insert_rowid`,
  transações (BEGIN/COMMIT/ROLLBACK). INTEGER volta como `int64` sem perda (use as funções que devolvem BigInt ou
  leia como texto); REAL `float64`; TEXT `string`; BLOB `[]byte`; NULL `nil`. `SetMaxOpenConns(1)`.
  `db.DB{Dialect: DialectSQLite}` — o SQL do store não muda. `PRAGMA foreign_keys = ON` ao abrir.
- **Build tags**: `internal/db` separa os drivers em arquivos com tags (`!js` para modernc/pgx, `js && wasm`
  para sqlitejs) para o wasm não compilar modernc nem pgx. Idem para qualquer pacote que use `os` arquivos,
  `net.Dialer` ou sinais: catálogo de espaços, prévia de link (no navegador devolve `{title:null,...}` porque o
  CORS bloqueia sites de terceiros), modo LAN, Telegram (desativado), `time/tzdata` embutido.
- **Espaços**: cada espaço é um arquivo no sahpool (`/finflow/<slug>.db`; o padrão `pessoal`). O catálogo, que no
  desktop é `spaces.json`, passa a ter uma interface (`spaces.Catalog`) com implementação em arquivo (desktop) e em
  SQLite (navegador: tabela `spaces` num banco `/finflow/_catalog.db`).
- **Assistente**: o SDK do Claude roda dentro do wasm usando o `fetch` do navegador (transporte padrão do Go em
  `js/wasm`), com o cabeçalho `anthropic-dangerous-direct-browser-access: true` só nesse build. A chave fica na
  tabela `settings` do banco local, como no desktop. Streaming SSE preservado.
- **Fuso**: o JS passa `Intl.DateTimeFormat().resolvedOptions().timeZone` como `FINFLOW_TZ`.

## 2. Ponte JS ↔ Go

O Go exporta, ao iniciar, `globalThis.__finflow = { handle, exportDb, importDb, reset, info }`:

```ts
handle(req: { method: string; url: string /* "/api/..." com query */; headers: [string, string][];
              body: Uint8Array | null },
       onChunk: (chunk: Uint8Array) => void)
  : Promise<{ status: number; headers: [string, string][] }>   // resolve quando o handler termina;
                                                               // os bytes do corpo chegam por onChunk (em ordem)
exportDb(space: string): Uint8Array        // via sqlite3_js_db_export / exportFile do sahpool
importDb(space: string, bytes: Uint8Array): Promise<void>     // valida (abre, roda Migrate+Bootstrap), substitui
reset(): Promise<void>                     // apaga todos os espaços e o catálogo
info(): { version: string; persistent: boolean; spaces: string[] }
```

Regra do `syscall/js`: funções chamadas pelo JS não podem bloquear — `handle`/`importDb`/`reset` disparam uma
goroutine e devolvem uma Promise.

## 3. Cliente do runtime (para o frontend)

`apps/web/src/runtime/client.ts` (único ponto que o resto do app importa):

```ts
export const isBrowserRuntime: boolean            // import.meta.env.VITE_FINFLOW_RUNTIME === "browser"
export function startRuntime(): Promise<RuntimeInfo>   // cria o worker, espera o Go ficar pronto
export function runtimeFetch(input: string, init?: RequestInit): Promise<Response>
  // aceita qualquer init (JSON, FormData, signal p/ abortar); devolve um Response real com corpo em stream,
  // então ai.ts lê SSE igual e api.ts faz .json() igual
export function exportDatabase(space: string): Promise<Blob>
export function importDatabase(space: string, file: Blob): Promise<void>
export function resetAllData(): Promise<void>
export function requestPersistentStorage(): Promise<boolean>   // navigator.storage.persist()
export type RuntimeInfo = { version: string; persistent: boolean; spaces: string[] }
```

Protocolo página ↔ worker (interno ao `runtime/`): mensagens com `id`; `fetch` → `head` (status/headers) +
`chunk`* + `end` | `error`; `abort` cancela (o Go recebe `ctx` cancelado). Arquivos servidos pelo app em
`/app/runtime/`: `finflow.wasm` (nome com hash ou query de versão), `wasm_exec.js` da mesma versão do Go
(`$(go env GOROOT)/lib/wasm/wasm_exec.js`), e os arquivos do sqlite-wasm.

**Uma aba por vez**: o sahpool permite uma conexão. O worker só abre o banco depois de obter um Web Lock
(`navigator.locks.request("finflow-db", ...)`). Outra aba mostra "O FinFlow está aberto em outra aba" com
"Usar aqui", que pede (BroadcastChannel) à outra aba para liberar.

## 4. Frontend no modo navegador

- Build `pnpm build:browser` (`VITE_FINFLOW_RUNTIME=browser`, `base: "/app/"`), saída em `apps/web/dist-browser/`.
  O build embutido no binário (`pnpm build:web`) não muda.
- `api.ts`, `ai.ts` e uploads usam `runtimeFetch` no modo navegador; downloads (`downloadUrl`/exportações) viram
  `downloadFile(path, filename)` que baixa via runtime e salva um Blob (no modo servidor continua link).
- Tela de abertura (só modo navegador, enquanto o runtime carrega): marca + "Carregando o FinFlow…". Primeira vez
  (banco vazio): "Começar do zero" | "Explorar com dados de exemplo" (`POST /api/system/seed`, que só aceita banco
  vazio — rota nova no Go, disponível nos dois modos) | "Restaurar um backup (.db)".
- Configurações → **Dados neste navegador**: armazenamento persistente (status + pedir), **Fazer backup**
  (um `.db` por espaço, ou `.zip` com todos), **Restaurar backup**, **Trazer dados do app instalado** (o
  `finflow.db` do computador: caminho `%AppData%\finflow\finflow.db` no Windows, `~/.config/finflow/finflow.db`
  no Linux, `~/Library/Application Support/finflow/finflow.db` no macOS), **Apagar todos os dados** (confirmação
  digitando "apagar"). Aviso: limpar os dados do site no navegador apaga o FinFlow.
- Esconder no modo navegador: modo rede local/QR, MCP stdio/HTTP, Telegram, "dados num arquivo no computador".
  Prévia de link de desejo mostra "indisponível na versão web".
- PWA do modo navegador: `manifest` com `start_url`/`scope` `/app/`, service worker que guarda o shell e os
  arquivos do runtime (cache versionado; atualização com aviso "Nova versão disponível — recarregar"), funciona
  offline.

## 5. Publicação

O pacote público (Vercel) passa a ter a landing em `/` e o app em `/app/` (fallback de SPA para `/app/*`,
`application/wasm`, cache imutável para arquivos com hash, `no-cache` para `sw.js` e `index.html`). Landing: botão
principal **Usar no navegador** (→ `/app/`), secundário **Baixar para o computador**. A verificação pública cobre
os dois. O FinFlow no navegador não faz nenhuma requisição de rede além dos próprios arquivos — exceto as perguntas
ao assistente, que vão direto do navegador para a API do Claude.

## 6. Verificação

- `GOOS=js GOARCH=wasm go build ./cmd/finflow-wasm` e `go vet` com essas variáveis no CI; os testes Go normais
  continuam passando.
- Playwright contra o build do navegador servido localmente (Chromium; Firefox se OPFS sahpool funcionar):
  carregar, começar com dados de exemplo, criar lançamento, recarregar e ver que persistiu, exportar xlsx,
  backup → apagar → restaurar, importar planilha, segunda aba bloqueada, offline após o primeiro carregamento.
