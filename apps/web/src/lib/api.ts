import { DEFAULT_SPACE_SLUG, getActiveSpace, resetToDefaultSpace, spaceHeaders } from "./space";

const DEFAULT_API_ORIGIN = import.meta.env.DEV ? "http://localhost:3333" : "";
const BASE_URL = `${import.meta.env.VITE_API_URL ?? DEFAULT_API_ORIGIN}/api`;
const TOKEN_KEY = "finflow_token";

export function getAuthToken(): string | null {
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

  const response = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: { ...headers, ...(init?.headers as Record<string, string> | undefined) },
  });
  if (!response.ok) {
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
    throw new ApiError(message, response.status, issues);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
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
  const query = new URLSearchParams(params);
  query.set("format", format);
  return downloadUrl("/transactions/export", query);
}
