import { isBrowserRuntime, runtimeFetch } from "../runtime/client";
import { DEFAULT_SPACE_SLUG, getActiveSpace, resetToDefaultSpace, spaceHeaders } from "./space";

const DEFAULT_API_ORIGIN = import.meta.env.DEV ? "http://localhost:3333" : "";
const BASE_URL = `${import.meta.env.VITE_API_URL ?? DEFAULT_API_ORIGIN}/api`;
const TOKEN_KEY = "finflow_token";

/**
 * Transporte da API. No modo navegador (docs/browser-runtime-contract.md §4) a API roda num worker
 * na própria página: `runtimeFetch` devolve um Response real (stream, .json(), SSE). No modo
 * servidor é o fetch normal para o FinFlow local.
 */
export function apiFetch(path: string, init?: RequestInit): Promise<Response> {
  return isBrowserRuntime ? runtimeFetch(`/api${path}`, init) : fetch(`${BASE_URL}${path}`, init);
}

export function getAuthToken(): string | null {
  // modo navegador: não há senha nem token (os dados nunca saem deste navegador)
  if (isBrowserRuntime) return null;
  try {
    return window.localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setAuthToken(token: string): void {
  try {
    window.localStorage.setItem(TOKEN_KEY, token);
  } catch {
    // storage indisponível
  }
}

export function clearAuthToken(): void {
  try {
    window.localStorage.removeItem(TOKEN_KEY);
  } catch {
    // storage indisponível
  }
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly issues?: { path: string; message: string }[],
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * 404 de um espaço que não existe mais (apagado em outro aparelho, catálogo trocado): volta para
 * o `pessoal`. O contrato não fixa a mensagem, então aceita qualquer texto que fale de espaço,
 * além de 404 na própria lista de espaços.
 */
export function handleUnknownSpace(status: number, path: string, message: string): void {
  if (status !== 404 || getActiveSpace() === DEFAULT_SPACE_SLUG) return;
  if (path.startsWith("/spaces") || /espa[çc]o|space/i.test(message)) resetToDefaultSpace();
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getAuthToken();
  // FormData: o navegador define o Content-Type com o boundary do multipart.
  const headers: Record<string, string> = {
    ...(init?.body instanceof FormData ? {} : { "Content-Type": "application/json" }),
    ...spaceHeaders(),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await apiFetch(path, {
    ...init,
    headers: { ...headers, ...(init?.headers as Record<string, string> | undefined) },
  });
  if (!response.ok) throw await toApiError(response, path);
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

/** Converte uma resposta de erro da API em ApiError (e trata 401 e espaço desconhecido). */
export async function toApiError(response: Response, path: string): Promise<ApiError> {
  if (response.status === 401 && path !== "/auth/login") {
    clearAuthToken();
    window.dispatchEvent(new Event("finflow:unauthorized"));
  }
  let message = `Erro ${response.status}`;
  let issues: { path: string; message: string }[] | undefined;
  try {
    const body = (await response.json()) as {
      message?: string | string[];
      issues?: { path: string; message: string }[];
    };
    message = Array.isArray(body.message) ? body.message.join(", ") : (body.message ?? message);
    issues = body.issues;
  } catch {
    // corpo não-JSON: mantém a mensagem genérica
  }
  handleUnknownSpace(response.status, path, message);
  return new ApiError(message, response.status, issues);
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "POST",
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "PATCH",
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
  /** POST multipart/form-data (upload de arquivo). */
  upload: <T>(path: string, form: FormData) => request<T>(path, { method: "POST", body: form }),
};

/**
 * URL absoluta para download direto via `<a href>`: o token vai em `?token=`,
 * já que o navegador não envia o header Authorization em navegação. O espaço ativo vai em
 * `?space=` pelo mesmo motivo (ausente = `pessoal`).
 */
export function downloadUrl(path: string, params?: URLSearchParams): string {
  const query = new URLSearchParams(params);
  const token = getAuthToken();
  if (token) query.set("token", token);
  const space = getActiveSpace();
  if (space !== DEFAULT_SPACE_SLUG) query.set("space", space);
  const qs = query.toString();
  return `${BASE_URL}${path}${qs ? `?${qs}` : ""}`;
}

export function exportUrl(format: "csv" | "ofx" | "xlsx", params: URLSearchParams): string {
  return downloadUrl("/transactions/export", exportParams(format, params));
}

/** Nome do arquivo no Content-Disposition (`attachment; filename="x.xlsx"`), se houver. */
function filenameFromDisposition(header: string | null): string | null {
  if (!header) return null;
  const utf8 = /filename\*=UTF-8''([^;]+)/i.exec(header);
  if (utf8?.[1]) {
    try {
      return decodeURIComponent(utf8[1]);
    } catch {
      // cai no filename simples
    }
  }
  return /filename="?([^";]+)"?/i.exec(header)?.[1] ?? null;
}

/** Salva um Blob como arquivo baixado (link temporário com `download`). */
export function saveBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.rel = "noopener";
  link.style.display = "none";
  document.body.append(link);
  link.click();
  link.remove();
  // dá tempo ao navegador de começar o download antes de liberar a URL
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

/**
 * Baixa um arquivo da API. Modo servidor: navegação comum para `downloadUrl` (o servidor manda o
 * arquivo). Modo navegador: busca pelo runtime e salva o Blob, com o nome do Content-Disposition
 * (ou `filename`, se a resposta não trouxer um).
 */
export async function downloadFile(
  path: string,
  filename: string,
  params?: URLSearchParams,
): Promise<void> {
  if (!isBrowserRuntime) {
    const link = document.createElement("a");
    link.href = downloadUrl(path, params);
    link.download = filename;
    document.body.append(link);
    link.click();
    link.remove();
    return;
  }
  const qs = params?.toString();
  const response = await apiFetch(`${path}${qs ? `?${qs}` : ""}`, { headers: spaceHeaders() });
  if (!response.ok) throw await toApiError(response, path);
  const name = filenameFromDisposition(response.headers.get("Content-Disposition")) ?? filename;
  saveBlob(await response.blob(), name);
}

export function exportParams(
  format: "csv" | "ofx" | "xlsx",
  params: URLSearchParams,
): URLSearchParams {
  const query = new URLSearchParams(params);
  query.set("format", format);
  return query;
}
