/// <reference lib="webworker" />
// Worker do runtime do navegador (docs/browser-runtime-contract.md §1–§3).
//  1. obtém o Web Lock "finflow-db" (uma aba por vez com o banco aberto; "Usar aqui" rouba o lock);
//  2. inicia o SQLite WASM com o VFS "opfs-sahpool" (sem OPFS: bancos em memória, persistent=false);
//  3. publica globalThis.__finflowHost (operações de arquivo usadas pelo driver Go "sqlitejs");
//  4. carrega wasm_exec.js + finflow.wasm e atende as mensagens da página (fetch em stream, abort,
//     exportar, importar, apagar, info).

import sqlite3InitModule, {
  type Database,
  type SAHPoolUtil,
  type Sqlite3Static,
  type SqlValue,
} from "@sqlite.org/sqlite-wasm";
import { DB_LOCK, type FromWorker, type ToWorker, type WireRequest } from "./protocol";

declare const self: DedicatedWorkerGlobalScope;

type GoInstance = {
  env: Record<string, string>;
  argv: string[];
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
};

/** O que o Go exporta (cmd/finflow-wasm). */
type FinFlowExports = {
  handle(
    req: WireRequest & { signal?: AbortSignal },
    onChunk: (chunk: Uint8Array) => void,
    onHead?: (status: number, headers: [string, string][]) => void,
  ): Promise<{ status: number; headers: [string, string][] }>;
  exportDb(space: string): Uint8Array | Error;
  importDb(space: string, bytes: Uint8Array): Promise<void>;
  reset(): Promise<void>;
  info(): { version: string; persistent: boolean; spaces: string[] };
};

/** Operações que o driver Go chama via syscall/js (internal/db/sqlitejs.go e browser.go). */
type Host = {
  sqlite3: Sqlite3Static;
  persistent: boolean;
  env: Record<string, string>;
  openDb(name: string): Database;
  closeDb(name: string, db: Database): void;
  exists(name: string): boolean;
  list(): string[];
  exportFile(name: string): Uint8Array;
  importFile(name: string, bytes: Uint8Array): Promise<void>;
  unlink(name: string): void;
  wipe(): Promise<void>;
  ensureCapacity(): Promise<void>;
  deserialize(db: Database, bytes: Uint8Array): void;
  fold(ctx: number, value: SqlValue): SqlValue;
  onReady?: () => void;
  onError?: (message: string) => void;
};

const scope = self as unknown as {
  Go: new () => GoInstance;
  __finflowHost: Host;
  __finflow: FinFlowExports;
};

function post(msg: FromWorker, transfer: Transferable[] = []) {
  self.postMessage(msg, transfer);
}

function message(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

// ───────────────────────── Web Lock ─────────────────────────

const holdForever = new Promise<never>(() => {});
let lockGranted = false;
let grantLock: () => void = () => {};
const lockReady = new Promise<void>((resolve) => {
  grantLock = resolve;
});

function onGranted(lock: Lock | null): Promise<never> | undefined {
  if (!lock) return undefined;
  if (!lockGranted) {
    lockGranted = true;
    grantLock();
  }
  return holdForever;
}

/** Se o lock for roubado por outra aba ("Usar aqui"), esta aba perde o banco. */
function watchLock(request: Promise<unknown>) {
  request.catch(() => {
    if (lockGranted) post({ type: "lost" });
  });
}

function acquireLock() {
  if (!("locks" in navigator)) {
    lockGranted = true;
    grantLock();
    return;
  }
  watchLock(
    navigator.locks.request(DB_LOCK, { ifAvailable: true }, (lock) => {
      if (lock) return onGranted(lock);
      post({ type: "blocked" });
      watchLock(navigator.locks.request(DB_LOCK, onGranted));
      return undefined;
    }),
  );
}

function stealLock() {
  if (lockGranted || !("locks" in navigator)) return;
  watchLock(navigator.locks.request(DB_LOCK, { steal: true }, onGranted));
}

// ───────────────────────── SQLite + VFS ─────────────────────────

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Instala o opfs-sahpool. A aba anterior pode ainda estar soltando os arquivos depois do "Usar aqui",
 * então tenta por alguns segundos. preserveOnInitFailure é obrigatório: sem ele uma falha de
 * inicialização apagaria o pool (os dados).
 */
async function installPool(sqlite3: Sqlite3Static): Promise<SAHPoolUtil | null> {
  let lastError: unknown;
  for (let attempt = 0; attempt < 12; attempt++) {
    try {
      const pool = await sqlite3.installOpfsSAHPoolVfs({
        initialCapacity: 8,
        preserveOnInitFailure: true,
        forceReinitIfPreviouslyFailed: true,
      } as Parameters<Sqlite3Static["installOpfsSAHPoolVfs"]>[0]);
      await pool.reserveMinimumCapacity(8);
      return pool;
    } catch (error) {
      lastError = error;
      if (message(error).includes("Missing required OPFS APIs")) break;
      await sleep(Math.min(250 * (attempt + 1), 1000));
    }
  }
  console.warn("FinFlow: OPFS indisponível, usando banco em memória:", message(lastError));
  return null;
}

function createHost(sqlite3: Sqlite3Static, pool: SAHPoolUtil | null, env: Record<string, string>) {
  const capi = sqlite3.capi;
  const decoder = new TextDecoder();

  const deserialize = (db: Database, bytes: Uint8Array) => {
    if (bytes.byteLength === 0) return;
    const p = sqlite3.wasm.allocFromTypedArray(bytes);
    // Com FREEONCLOSE o SQLite libera o buffer, inclusive em caso de falha.
    const rc = capi.sqlite3_deserialize(
      db,
      "main",
      p,
      bytes.byteLength,
      bytes.byteLength,
      capi.SQLITE_DESERIALIZE_FREEONCLOSE | capi.SQLITE_DESERIALIZE_RESIZEABLE,
    );
    if (rc !== 0) throw new Error(`sqlite3_deserialize: ${capi.sqlite3_js_rc_str(rc)}`);
  };

  const common = {
    sqlite3,
    env,
    deserialize,
    fold: (_ctx: number, value: SqlValue): SqlValue => {
      if (typeof value === "string") return value.toLowerCase();
      if (value instanceof Uint8Array) return decoder.decode(value).toLowerCase();
      return value;
    },
  };

  if (pool) {
    const dbFiles = () => pool.getFileNames().filter((name) => !name.endsWith("-journal"));
    const host: Host = {
      ...common,
      persistent: true,
      openDb: (name) => new pool.OpfsSAHPoolDb(name),
      closeDb: (_name, db) => {
        db.close();
      },
      exists: (name) => pool.getFileNames().includes(name),
      list: dbFiles,
      exportFile: (name) => pool.exportFile(name),
      importFile: async (name, bytes) => {
        await pool.importDb(name, bytes);
      },
      unlink: (name) => {
        pool.unlink(name);
        pool.unlink(`${name}-journal`);
      },
      wipe: () => pool.wipeFiles(),
      // Cada banco precisa do arquivo e, durante uma transação, do journal; mais uma folga.
      ensureCapacity: async () => {
        const need = 2 * (dbFiles().length + 1) + 2;
        if (Number(pool.getCapacity()) < need) await pool.reserveMinimumCapacity(need + 2);
      },
    };
    return host;
  }

  // Sem OPFS: cada "arquivo" é um banco em memória; ao fechar, o conteúdo fica num Map até o fim da aba.
  const files = new Map<string, Uint8Array>();
  const host: Host = {
    ...common,
    persistent: false,
    openDb: (name) => {
      const db = new sqlite3.oo1.DB(":memory:");
      try {
        deserialize(db, files.get(name) ?? new Uint8Array());
      } catch (error) {
        db.close();
        throw error;
      }
      if (!files.has(name)) files.set(name, new Uint8Array());
      return db;
    },
    closeDb: (name, db) => {
      files.set(name, capi.sqlite3_js_db_export(db));
      db.close();
    },
    exists: (name) => files.has(name),
    list: () => [...files.keys()],
    exportFile: (name) => {
      const bytes = files.get(name);
      if (!bytes) throw new Error(`arquivo inexistente: ${name}`);
      return bytes.slice();
    },
    importFile: async (name, bytes) => {
      files.set(name, bytes.slice());
    },
    unlink: (name) => {
      files.delete(name);
    },
    wipe: async () => {
      files.clear();
    },
    ensureCapacity: async () => {},
  };
  return host;
}

// ───────────────────────── Go ─────────────────────────

async function startGo(runtimeBase: string, env: Record<string, string>) {
  await lockReady;
  const sqlite3 = await sqlite3InitModule();
  // O driver Go liga inteiros de 64 bits como BigInt.
  if (typeof BigInt64Array === "undefined") throw new Error("Este navegador é antigo demais.");
  const pool = await installPool(sqlite3);
  const host = createHost(sqlite3, pool, env);
  scope.__finflowHost = host;

  // wasm_exec.js é um script clássico que define globalThis.Go; num worker de módulo, import() o executa.
  await import(/* @vite-ignore */ `${runtimeBase}wasm_exec.js`);
  let wasmUrl = `${runtimeBase}finflow.wasm`;
  try {
    const manifest = (await (await fetch(`${runtimeBase}runtime.json`)).json()) as {
      hash?: string;
    };
    if (manifest.hash) wasmUrl += `?v=${manifest.hash}`;
  } catch {
    // sem runtime.json (ex.: build antigo): carrega sem a versão na URL
  }

  const go = new scope.Go();
  go.env = { ...env };
  go.argv = ["finflow-wasm"];
  let instance: WebAssembly.Instance;
  try {
    ({ instance } = await WebAssembly.instantiateStreaming(fetch(wasmUrl), go.importObject));
  } catch {
    // servidor sem o MIME application/wasm: instancia a partir dos bytes
    const response = await fetch(wasmUrl);
    if (!response.ok) throw new Error(`não foi possível baixar o FinFlow (${response.status})`);
    ({ instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject));
  }

  const ready = new Promise<void>((resolve, reject) => {
    host.onReady = resolve;
    host.onError = (msg) => reject(new Error(msg));
  });
  go.run(instance).then(
    () => post({ type: "fatal", message: "O runtime do FinFlow encerrou." }),
    (error: unknown) => post({ type: "fatal", message: message(error) }),
  );
  await ready;
}

// ───────────────────────── Mensagens ─────────────────────────

const inflight = new Map<number, AbortController>();
let started: Promise<void> | null = null;

function handleFetch(id: number, req: WireRequest) {
  const controller = new AbortController();
  inflight.set(id, controller);
  scope.__finflow
    .handle(
      { ...req, signal: controller.signal },
      (chunk) => post({ type: "chunk", id, chunk }, [chunk.buffer]),
      (status, headers) => post({ type: "head", id, status, headers }),
    )
    .then(
      () => post({ type: "end", id }),
      (error: unknown) => post({ type: "error", id, message: message(error) }),
    )
    .finally(() => inflight.delete(id));
}

async function handleCall(
  msg: Exclude<ToWorker, { type: "init" | "fetch" | "abort" | "takeover" }>,
) {
  const finflow = scope.__finflow;
  switch (msg.type) {
    case "export": {
      const out = finflow.exportDb(msg.space);
      if (out instanceof Error) throw out;
      return out;
    }
    case "import":
      await finflow.importDb(msg.space, msg.bytes);
      return finflow.info();
    case "reset":
      await finflow.reset();
      return finflow.info();
    case "info":
      return finflow.info();
  }
}

self.addEventListener("message", (event: MessageEvent<ToWorker>) => {
  const msg = event.data;
  switch (msg.type) {
    case "init":
      if (started) return;
      acquireLock();
      started = startGo(msg.runtimeBase, msg.env).then(
        () => post({ type: "ready", info: scope.__finflow.info() }),
        (error: unknown) => post({ type: "fatal", message: message(error) }),
      );
      return;
    case "takeover":
      stealLock();
      return;
    case "fetch":
      handleFetch(msg.id, msg.req);
      return;
    case "abort":
      inflight.get(msg.id)?.abort();
      return;
    default: {
      const id = msg.id;
      handleCall(msg).then(
        (value) => {
          const transfer = value instanceof Uint8Array ? [value.buffer] : [];
          post({ type: "result", id, value }, transfer);
        },
        (error: unknown) => post({ type: "error", id, message: message(error) }),
      );
    }
  }
});
