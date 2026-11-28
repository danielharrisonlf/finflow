// Cliente do runtime do navegador (docs/browser-runtime-contract.md §3).
// Único ponto que o resto do app importa: o worker e o protocolo ficam internos a src/runtime/.
//
// O worker (worker.ts) roda o SQLite WASM (opfs-sahpool) e a API Go compilada para WebAssembly.
// runtimeFetch serializa a requisição, manda ao worker e devolve um Response cujo corpo chega em
// stream (head → chunk* → end), então SSE e .json() funcionam como num fetch comum.
//
// Uma aba por vez: o worker só abre o banco com o Web Lock "finflow-db". Sem o lock, a aba fica
// "bloqueada" (onRuntimeBlocked) e takeOver() pede à outra aba, por BroadcastChannel, que libere o
// banco (e rouba o lock); a aba que perdeu o banco é avisada por onRuntimeLost.

import { type FromWorker, TAB_CHANNEL, type ToWorker, type WireRequest } from "./protocol";

export type RuntimeInfo = { version: string; persistent: boolean; spaces: string[] };

export const isBrowserRuntime: boolean = import.meta.env.VITE_FINFLOW_RUNTIME === "browser";

type FetchPending = {
  kind: "fetch";
  resolve: (res: Response) => void;
  reject: (error: unknown) => void;
  controller: ReadableStreamDefaultController<Uint8Array> | null;
  headReceived: boolean;
  cleanup: () => void;
};

type CallPending = {
  kind: "call";
  resolve: (value: unknown) => void;
  reject: (error: unknown) => void;
};

let worker: Worker | null = null;
let starting: Promise<RuntimeInfo> | null = null;
let ready = false;
let nextId = 1;
const pending = new Map<number, FetchPending | CallPending>();
const blockedListeners = new Set<() => void>();
const lostListeners = new Set<() => void>();
let channel: BroadcastChannel | null = null;

/** URL absoluta de /app/runtime/ (finflow.wasm, wasm_exec.js, runtime.json). */
function runtimeBase(): string {
  return new URL(`${import.meta.env.BASE_URL}runtime/`, self.location.href).href;
}

function send(msg: ToWorker, transfer: Transferable[] = []) {
  if (!worker) throw new Error("runtime ainda não carregado");
  worker.postMessage(msg, transfer);
}

function failAll(error: Error) {
  for (const [id, p] of pending) {
    pending.delete(id);
    if (p.kind === "fetch") {
      p.cleanup();
      if (p.headReceived) p.controller?.error(error);
      else p.reject(error);
    } else {
      p.reject(error);
    }
  }
}

/** Encerra o worker desta aba (libera o lock e os arquivos do OPFS). */
function stopWorker(reason: string) {
  worker?.terminate();
  worker = null;
  ready = false;
  starting = null;
  failAll(new Error(reason));
}

function setupChannel() {
  if (channel || typeof BroadcastChannel === "undefined") return;
  channel = new BroadcastChannel(TAB_CHANNEL);
  channel.addEventListener("message", (event: MessageEvent<{ type: string }>) => {
    // Outra aba pediu "Usar aqui": quem está com o banco aberto libera.
    if (event.data?.type === "release" && ready) loseRuntime();
  });
}

function loseRuntime() {
  stopWorker("O FinFlow foi aberto em outra aba.");
  for (const cb of lostListeners) cb();
}

function onWorkerMessage(
  msg: FromWorker,
  resolveStart: (info: RuntimeInfo) => void,
  rejectStart: (e: Error) => void,
) {
  switch (msg.type) {
    case "blocked":
      for (const cb of blockedListeners) cb();
      return;
    case "ready":
      ready = true;
      resolveStart(msg.info);
      return;
    case "lost":
      loseRuntime();
      return;
    case "fatal": {
      const error = new Error(msg.message);
      const wasReady = ready;
      stopWorker(msg.message);
      if (!wasReady) rejectStart(error);
      return;
    }
    case "head": {
      const p = pending.get(msg.id);
      if (p?.kind !== "fetch") return;
      p.headReceived = true;
      // Respostas sem corpo por definição não aceitam stream no construtor do Response.
      const nullBody = msg.status === 204 || msg.status === 205 || msg.status === 304;
      const body = nullBody
        ? null
        : new ReadableStream<Uint8Array>({
            start(controller) {
              p.controller = controller;
            },
            cancel() {
              if (pending.delete(msg.id)) {
                p.cleanup();
                send({ type: "abort", id: msg.id });
              }
            },
          });
      p.resolve(new Response(body, { status: msg.status, headers: msg.headers }));
      return;
    }
    case "chunk": {
      const p = pending.get(msg.id);
      if (p?.kind === "fetch") p.controller?.enqueue(msg.chunk);
      return;
    }
    case "end": {
      const p = pending.get(msg.id);
      if (p?.kind !== "fetch") return;
      pending.delete(msg.id);
      p.cleanup();
      p.controller?.close();
      return;
    }
    case "error": {
      const p = pending.get(msg.id);
      if (!p) return;
      pending.delete(msg.id);
      const error = new Error(msg.message);
      if (p.kind === "fetch") {
        p.cleanup();
        if (p.headReceived) p.controller?.error(error);
        else p.reject(error);
      } else {
        p.reject(error);
      }
      return;
    }
    case "result": {
      const p = pending.get(msg.id);
      if (p?.kind !== "call") return;
      pending.delete(msg.id);
      p.resolve(msg.value);
      return;
    }
  }
}

/** Cria o worker e espera o Go ficar pronto (fica pendente enquanto outra aba tiver o banco). */
export function startRuntime(): Promise<RuntimeInfo> {
  if (starting) return starting;
  setupChannel();
  starting = new Promise<RuntimeInfo>((resolve, reject) => {
    const w = new Worker(new URL("./worker.ts", import.meta.url), {
      type: "module",
      name: "finflow-runtime",
    });
    worker = w;
    w.addEventListener("message", (event: MessageEvent<FromWorker>) => {
      if (worker === w) onWorkerMessage(event.data, resolve, reject);
    });
    w.addEventListener("error", (event) => {
      if (worker !== w) return;
      const msg = event.message || "O navegador não conseguiu iniciar o FinFlow.";
      const wasReady = ready;
      stopWorker(msg);
      if (!wasReady) reject(new Error(msg));
    });
    send({
      type: "init",
      runtimeBase: runtimeBase(),
      env: { FINFLOW_TZ: Intl.DateTimeFormat().resolvedOptions().timeZone || "America/Sao_Paulo" },
    });
  });
  return starting;
}

function abortError(signal: AbortSignal): unknown {
  return signal.reason ?? new DOMException("A requisição foi cancelada.", "AbortError");
}

/** Caminho "/api/..." (com query) a partir do que o app passar: caminho ou URL absoluta. */
function apiPath(input: string): string {
  const url = new URL(input, self.location.origin);
  let path = url.pathname;
  if (!path.startsWith("/api") && path.includes("/api/")) path = path.slice(path.indexOf("/api/"));
  return path + url.search;
}

/**
 * fetch atendido pela API dentro da página. Aceita qualquer RequestInit (JSON, FormData, signal) e
 * devolve um Response real com o corpo em stream.
 */
export async function runtimeFetch(input: string, init?: RequestInit): Promise<Response> {
  // new Request serializa FormData/URLSearchParams/Blob com o Content-Type certo (boundary).
  const request = new Request(new URL(input, self.location.origin), init);
  const signal = request.signal;
  if (signal.aborted) throw abortError(signal);
  await startRuntime();
  const method = request.method.toUpperCase();
  const body =
    method === "GET" || method === "HEAD" ? null : new Uint8Array(await request.arrayBuffer());
  if (signal.aborted) throw abortError(signal);
  const req: WireRequest = {
    method,
    url: apiPath(input),
    headers: [...request.headers],
    body: body && body.byteLength > 0 ? body : null,
  };
  const id = nextId++;
  return new Promise<Response>((resolve, reject) => {
    const p: FetchPending = {
      kind: "fetch",
      resolve,
      reject,
      controller: null,
      headReceived: false,
      cleanup: () => signal.removeEventListener("abort", onAbort),
    };
    function onAbort() {
      if (!pending.delete(id)) return;
      p.cleanup();
      try {
        send({ type: "abort", id });
      } catch {
        // worker já encerrado
      }
      if (p.headReceived) p.controller?.error(abortError(signal));
      else reject(abortError(signal));
    }
    signal.addEventListener("abort", onAbort);
    pending.set(id, p);
    try {
      send({ type: "fetch", id, req }, req.body ? [req.body.buffer] : []);
    } catch (error) {
      pending.delete(id);
      p.cleanup();
      reject(error);
    }
  });
}

async function call<T>(msg: ToWorker & { id: number }, transfer: Transferable[] = []): Promise<T> {
  await startRuntime();
  return new Promise<T>((resolve, reject) => {
    pending.set(msg.id, { kind: "call", resolve: resolve as (v: unknown) => void, reject });
    try {
      send(msg, transfer);
    } catch (error) {
      pending.delete(msg.id);
      reject(error);
    }
  });
}

/** Backup do espaço: o arquivo SQLite (abre no FinFlow do computador). */
export async function exportDatabase(space: string): Promise<Blob> {
  const bytes = await call<Uint8Array>({ type: "export", id: nextId++, space });
  return new Blob([bytes as BlobPart], { type: "application/vnd.sqlite3" });
}

/** Restaura um backup .db no espaço (valida antes; se for inválido, os dados atuais ficam). */
export async function importDatabase(space: string, file: Blob): Promise<void> {
  const bytes = new Uint8Array(await file.arrayBuffer());
  await call<RuntimeInfo>({ type: "import", id: nextId++, space, bytes }, [bytes.buffer]);
}

/** Apaga todos os espaços e o catálogo deste navegador. */
export async function resetAllData(): Promise<void> {
  await call<RuntimeInfo>({ type: "reset", id: nextId++ });
}

/** Pede armazenamento persistente (o navegador não apaga os dados sob pressão de espaço). */
export async function requestPersistentStorage(): Promise<boolean> {
  if (!navigator.storage?.persist) return false;
  if (await navigator.storage.persisted?.()) return true;
  return navigator.storage.persist();
}

/** Outra aba está com o banco aberto. Devolve a função que cancela a inscrição. */
export function onRuntimeBlocked(cb: () => void): () => void {
  blockedListeners.add(cb);
  return () => blockedListeners.delete(cb);
}

/** Esta aba perdeu o banco para outra ("Usar aqui" lá). Devolve a função que cancela a inscrição. */
export function onRuntimeLost(cb: () => void): () => void {
  lostListeners.add(cb);
  return () => lostListeners.delete(cb);
}

/** Pede à outra aba que libere o banco ("Usar aqui") e continua a inicialização nesta. */
export function takeOver(): void {
  setupChannel();
  channel?.postMessage({ type: "release" });
  if (!worker) {
    void startRuntime();
  }
  worker?.postMessage({ type: "takeover" } satisfies ToWorker);
}
