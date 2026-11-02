const BASE_URL = `${import.meta.env.VITE_API_URL ?? "http://localhost:3333"}/api`;
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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getAuthToken();
  const headers: Record<string, string> = { "Content-Type": "application/json" };
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
};

export function exportUrl(format: "csv" | "ofx", params: URLSearchParams): string {
  const query = new URLSearchParams(params);
  query.set("format", format);
  const token = getAuthToken();
  if (token) query.set("token", token);
  return `${BASE_URL}/transactions/export?${query.toString()}`;
}
