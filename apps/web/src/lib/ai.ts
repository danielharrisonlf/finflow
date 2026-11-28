import { ApiError, api, apiFetch, clearAuthToken, getAuthToken, handleUnknownSpace } from "./api";
import { getActiveSpace, spaceHeaders } from "./space";

// Cliente do Assistente (v3 §15). O chat usa SSE sobre fetch + ReadableStream (EventSource não envia
// o header Authorization nem faz POST). No modo navegador, `apiFetch` usa o runtime local, que
// também devolve o corpo em stream.

export type AiRole = "user" | "assistant";

export interface AiChatMessage {
  role: AiRole;
  content: string;
}

export type AiProposalKind =
  | "CREATE_TRANSACTION"
  | "CREATE_RECURRING"
  | "CONTRIBUTE_GOAL"
  | "ADD_WISH";

export interface AiProposal {
  id: string;
  kind: AiProposalKind;
  summary: string;
  /** Corpo do endpoint normal chamado ao confirmar (CONTRIBUTE_GOAL inclui goalId). */
  payload: Record<string, unknown>;
}

export type AiEvent =
  | { type: "text"; delta: string }
  | { type: "tool"; name: string; label: string }
  | { type: "proposal"; proposal: AiProposal }
  | { type: "done"; stopReason: string }
  | { type: "error"; message: string };

export interface AiReceipt {
  description: string;
  amountCents: number;
  date: string;
  kind: "EXPENSE" | "INCOME";
  suggestedCategoryId: string | null;
  merchant: string | null;
  installments: number | null;
  confidence: "HIGH" | "MEDIUM" | "LOW";
}

export interface AiStatus {
  configured: boolean;
  source: "ENV" | "SETTINGS" | null;
  model: string;
}

/** Autorização + espaço ativo (o chat e a chave seguem o espaço como qualquer outra rota). */
function authHeaders(): Record<string, string> {
  const token = getAuthToken();
  return { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...spaceHeaders() };
}

async function toApiError(response: Response, path: string): Promise<ApiError> {
  if (response.status === 401) {
    clearAuthToken();
    window.dispatchEvent(new Event("finflow:unauthorized"));
  }
  let message = `Erro ${response.status}`;
  let issues: { path: string; message: string }[] | undefined;
  try {
    const body = (await response.json()) as {
      message?: string;
      issues?: { path: string; message: string }[];
    };
    message = body.message ?? message;
    issues = body.issues;
  } catch {
    // corpo não-JSON
  }
  handleUnknownSpace(response.status, path, message);
  return new ApiError(message, response.status, issues);
}

/** Converte um bloco SSE (`event: x\ndata: {...}`) no evento tipado; ignora o que não reconhece. */
export function parseSseBlock(block: string): AiEvent | null {
  let name = "message";
  const data: string[] = [];
  for (const line of block.split("\n")) {
    if (line.startsWith("event:")) name = line.slice(6).trim();
    else if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
  }
  if (data.length === 0) return null;
  let parsed: Record<string, unknown>;
  try {
    parsed = JSON.parse(data.join("\n")) as Record<string, unknown>;
  } catch {
    return null;
  }
  switch (name) {
    case "text":
      return { type: "text", delta: String(parsed.delta ?? "") };
    case "tool":
      return { type: "tool", name: String(parsed.name ?? ""), label: String(parsed.label ?? "") };
    case "proposal":
      return { type: "proposal", proposal: parsed as unknown as AiProposal };
    case "done":
      return { type: "done", stopReason: String(parsed.stopReason ?? "") };
    case "error":
      return { type: "error", message: String(parsed.message ?? "Erro no assistente") };
    default:
      return null;
  }
}

/**
 * POST /api/ai/chat em streaming. Chama onEvent para cada evento; erros HTTP antes do stream (412 sem
 * chave, 400…) viram ApiError.
 */
export async function streamChat(
  messages: AiChatMessage[],
  onEvent: (event: AiEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const response = await apiFetch("/ai/chat", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream", ...authHeaders() },
    body: JSON.stringify({ messages }),
    signal,
  });
  if (!response.ok) throw await toApiError(response, "/ai/chat");
  if (!response.body) throw new ApiError("Resposta sem corpo", response.status);

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, "\n");
    let sep = buffer.indexOf("\n\n");
    while (sep !== -1) {
      const event = parseSseBlock(buffer.slice(0, sep));
      buffer = buffer.slice(sep + 2);
      if (event) onEvent(event);
      sep = buffer.indexOf("\n\n");
    }
  }
  const tail = parseSseBlock(buffer + decoder.decode());
  if (tail) onEvent(tail);
}

/** PUT /api/ai/key — `null` remove a chave salva. A chave nunca volta na resposta. */
export async function saveAiKey(apiKey: string | null): Promise<AiStatus> {
  const response = await apiFetch("/ai/key", {
    method: "PUT",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify({ apiKey }),
  });
  if (!response.ok) throw await toApiError(response, "/ai/key");
  return (await response.json()) as AiStatus;
}

/** POST /api/ai/receipt (multipart, campo `file`). */
export function extractReceipt(file: File): Promise<AiReceipt> {
  const form = new FormData();
  form.append("file", file);
  return api.upload<AiReceipt>("/ai/receipt", form);
}

/** Extensões que o chat encaminha para a importação de planilhas. */
export const SPREADSHEET_EXTENSIONS = [".xlsx", ".xls", ".csv", ".ofx"];
export const RECEIPT_ACCEPT = "image/jpeg,image/png,image/webp,image/gif,application/pdf";
export const MAX_RECEIPT_BYTES = 10 * 1024 * 1024;

export function isSpreadsheetFile(name: string): boolean {
  const lower = name.toLowerCase();
  return SPREADSHEET_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

// ───────────────────────── histórico local ─────────────────────────

export type ProposalStatus = "pending" | "confirmed" | "discarded";

export interface StoredProposal extends AiProposal {
  status: ProposalStatus;
}

export interface UiMessage {
  id: string;
  role: AiRole;
  content: string;
  tools?: { name: string; label: string }[];
  proposals?: StoredProposal[];
  error?: string;
  stopReason?: string;
  /** Só em memória: resposta ainda chegando. */
  streaming?: boolean;
  /** Mensagem gerada pela própria interface (ex.: leitura de comprovante); não vai para o modelo. */
  local?: boolean;
}

const HISTORY_LIMIT = 80;

/** Uma conversa por espaço: o histórico de um espaço não pode virar contexto de outro. */
function historyKey(): string {
  const space = getActiveSpace();
  // O espaço pessoal mantém a chave antiga para não perder conversas de versões anteriores.
  return space === "pessoal" ? "finflow_assistant_history" : `finflow_assistant_history:${space}`;
}

export function loadHistory(): UiMessage[] {
  try {
    const raw = window.localStorage.getItem(historyKey());
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (m): m is UiMessage =>
        typeof m === "object" &&
        m !== null &&
        typeof (m as UiMessage).id === "string" &&
        ((m as UiMessage).role === "user" || (m as UiMessage).role === "assistant") &&
        typeof (m as UiMessage).content === "string",
    );
  } catch {
    return [];
  }
}

export function saveHistory(messages: UiMessage[]): void {
  try {
    const toStore = messages.slice(-HISTORY_LIMIT).map(({ streaming: _streaming, ...m }) => m);
    window.localStorage.setItem(historyKey(), JSON.stringify(toStore));
  } catch {
    // storage indisponível ou cheio: o chat segue só em memória
  }
}

export function clearHistory(): void {
  try {
    window.localStorage.removeItem(historyKey());
  } catch {
    // storage indisponível
  }
}

/** Histórico enviado ao servidor: só texto, últimas mensagens, começando por uma do usuário. */
export function toChatPayload(messages: UiMessage[], max = 40): AiChatMessage[] {
  const out = messages
    .filter((m) => !m.local && m.content.trim() !== "")
    .map((m) => ({ role: m.role, content: m.content }))
    .slice(-max);
  while (out.length > 0 && out[0]?.role !== "user") out.shift();
  return out;
}

export function newId(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  }
}
