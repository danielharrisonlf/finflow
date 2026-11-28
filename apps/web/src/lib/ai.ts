import type {
  AiConfigInput,
  AiModel,
  AiModelsResponse,
  AiProvider,
  AiStatus,
} from "@finflow/contracts";
import { ApiError, api, apiFetch, clearAuthToken, getAuthToken, handleUnknownSpace } from "./api";
import { getActiveSpace, spaceHeaders } from "./space";

// Cliente do Assistente (v3 §15 e docs/local-agent-contract.md). O chat usa SSE sobre fetch +
// ReadableStream (EventSource não envia o header Authorization nem faz POST). No modo navegador,
// `apiFetch` usa o runtime local, que também devolve o corpo em stream.

export type { AiConfigInput, AiModel, AiProvider, AiStatus };

/** Rótulos dos provedores (Configurações → Assistente). */
export const AI_PROVIDER_LABEL: Record<AiProvider, string> = {
  ollama: "Local (Ollama)",
  openai_compatible: "Servidor compatível (LM Studio, llama.cpp…)",
  claude: "Claude (API)",
};

/** Nome curto do provedor ativo (cabeçalho do Assistente). */
export const AI_PROVIDER_SHORT: Record<AiProvider, string> = {
  ollama: "Ollama (local)",
  openai_compatible: "Servidor local",
  claude: "Claude (API)",
};

/** Padrões sugeridos (os mesmos do servidor). */
export const AI_DEFAULTS = {
  ollamaUrl: "http://localhost:11434",
  openaiUrl: "http://localhost:1234/v1",
  model: "qwen2.5:7b",
  altModel: "llama3.1:8b",
  visionModel: "qwen2.5vl:7b",
} as const;

export function defaultBaseUrl(provider: AiProvider): string {
  if (provider === "ollama") return AI_DEFAULTS.ollamaUrl;
  if (provider === "openai_compatible") return AI_DEFAULTS.openaiUrl;
  return "";
}

export function isLocalProvider(provider: AiProvider | null | undefined): boolean {
  return provider === "ollama" || provider === "openai_compatible";
}

/** Provedor efetivo do status; servidores antigos (sem `provider`) só tinham o Claude. */
export function statusProvider(status: AiStatus | undefined): AiProvider | null {
  if (!status) return null;
  if (status.provider) return status.provider;
  return status.configured || status.source ? "claude" : null;
}

/** Provedor e modelo ativos, ex.: "Ollama (local) · qwen2.5:7b" ou "Detectado automaticamente: Ollama · qwen2.5:7b". */
export function describeActiveAi(status: AiStatus | undefined): string | null {
  const provider = statusProvider(status);
  if (!status?.configured || !provider) return null;
  const model = status.model ? ` · ${status.model}` : "";
  if (status.source === "AUTO") return `Detectado automaticamente: Ollama${model}`;
  return `${AI_PROVIDER_SHORT[provider]}${model}`;
}

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

/** PUT /api/ai/config — escolhe o provedor. `apiKey` omitido mantém a chave salva. */
export async function saveAiConfig(input: AiConfigInput): Promise<AiStatus> {
  const response = await apiFetch("/ai/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json", ...authHeaders() },
    body: JSON.stringify(input),
  });
  if (!response.ok) throw await toApiError(response, "/ai/config");
  return (await response.json()) as AiStatus;
}

/** GET /api/ai/models — também serve como "Testar conexão" com o servidor local. */
export async function listAiModels(provider: AiProvider, baseUrl?: string): Promise<AiModel[]> {
  const query = new URLSearchParams({ provider });
  if (baseUrl?.trim()) query.set("baseUrl", baseUrl.trim());
  const res = await api.get<AiModelsResponse>(`/ai/models?${query.toString()}`);
  return Array.isArray(res?.models) ? res.models : [];
}

/** Escolhe o modelo para "Usar o Ollama deste computador": o sugerido se instalado, senão o primeiro com ferramentas. */
export function pickChatModel(models: AiModel[]): string {
  const ids = models.map((m) => m.id);
  for (const preferred of [
    AI_DEFAULTS.model,
    `${AI_DEFAULTS.model}-instruct`,
    AI_DEFAULTS.altModel,
  ]) {
    if (ids.includes(preferred)) return preferred;
  }
  return (
    (models.find((m) => m.tools === true) ?? models.find((m) => m.tools !== false))?.id ??
    AI_DEFAULTS.model
  );
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
