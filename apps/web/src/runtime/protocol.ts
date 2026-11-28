// Protocolo página ↔ worker do runtime (docs/browser-runtime-contract.md §3). Interno a src/runtime/.

import type { RuntimeInfo } from "./client";

export type WireRequest = {
  method: string;
  url: string;
  headers: [string, string][];
  body: Uint8Array | null;
};

/** Página → worker. */
export type ToWorker =
  | { type: "init"; runtimeBase: string; env: Record<string, string> }
  | { type: "fetch"; id: number; req: WireRequest }
  | { type: "abort"; id: number }
  | { type: "takeover" }
  | { type: "export"; id: number; space: string }
  | { type: "import"; id: number; space: string; bytes: Uint8Array }
  | { type: "reset"; id: number }
  | { type: "info"; id: number };

/** Worker → página. */
export type FromWorker =
  | { type: "blocked" }
  | { type: "lost" }
  | { type: "ready"; info: RuntimeInfo }
  | { type: "fatal"; message: string }
  | { type: "head"; id: number; status: number; headers: [string, string][] }
  | { type: "chunk"; id: number; chunk: Uint8Array }
  | { type: "end"; id: number }
  | { type: "error"; id: number; message: string }
  | { type: "result"; id: number; value: unknown };

/** Nome do Web Lock que garante uma aba por vez com o banco aberto. */
export const DB_LOCK = "finflow-db";
/** Canal entre abas para o "Usar aqui". */
export const TAB_CHANNEL = "finflow-runtime";
