const BASE_URL = `${import.meta.env.VITE_API_URL ?? "http://localhost:3333"}/api`;

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
  const response = await fetch(`${BASE_URL}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!response.ok) {
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
  return `${BASE_URL}/transactions/export?${query.toString()}`;
}
