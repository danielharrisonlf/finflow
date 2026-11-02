import type {
  Account,
  AuthStatus,
  Budget,
  Card,
  CashflowPoint,
  Category,
  CategorySlice,
  ContributeGoalInput,
  CreateAccountInput,
  CreateCardExpenseInput,
  CreateCardInput,
  CreateCategoryInput,
  CreateGoalInput,
  CreateTransactionInput,
  DashboardSummary,
  ExportFormat,
  Goal,
  Invoice,
  LoginResponse,
  Paginated,
  TelegramStatus,
  Transaction,
  TransferInput,
  UpcomingItem,
  UpdateCategoryInput,
  UpdateTransactionInput,
  UpsertBudgetInput,
} from "@finflow/contracts";

export class FinFlowApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly issues?: { path: string; message: string }[],
  ) {
    super(message);
    this.name = "FinFlowApiError";
  }
}

export interface ListTransactionsOptions {
  month?: number;
  year?: number;
  accountId?: string;
  cardId?: string;
  categoryId?: string;
  type?: Transaction["type"];
  status?: Transaction["status"];
  search?: string;
  page?: number;
  pageSize?: number;
}

function toQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") search.set(key, String(value));
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

/** Client fino e tipado da API REST do FinFlow. */
export class FinFlowClient {
  private readonly baseUrl: string;
  private readonly token: string | undefined;

  constructor(
    baseUrl = process.env.FINFLOW_API_URL ?? "http://localhost:3333",
    token = process.env.FINFLOW_TOKEN ??
      process.env.FINFLOW_API_TOKEN ??
      process.env.FINFLOW_PASSWORD,
  ) {
    this.baseUrl = `${baseUrl.replace(/\/$/, "")}/api`;
    this.token = token?.trim() || undefined;
  }

  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    let response: Response;
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (this.token) {
      headers.Authorization = `Bearer ${this.token}`;
    }
    try {
      response = await fetch(`${this.baseUrl}${path}`, {
        ...init,
        headers: { ...headers, ...(init?.headers as Record<string, string> | undefined) },
      });
    } catch (error) {
      throw new FinFlowApiError(
        `não consegui falar com a API em ${this.baseUrl} — ela está rodando? (${(error as Error).message})`,
        0,
      );
    }
    if (!response.ok) {
      let message = `erro HTTP ${response.status}`;
      let issues: { path: string; message: string }[] | undefined;
      try {
        const body = (await response.json()) as {
          message?: string | string[];
          issues?: { path: string; message: string }[];
        };
        message = Array.isArray(body.message) ? body.message.join(", ") : (body.message ?? message);
        issues = body.issues;
      } catch {
        // mantém mensagem genérica
      }
      throw new FinFlowApiError(message, response.status, issues);
    }
    if (response.status === 204) return undefined as T;
    const text = await response.text();
    try {
      return JSON.parse(text) as T;
    } catch {
      return text as T;
    }
  }

  // dashboard
  dashboardSummary(month?: number, year?: number): Promise<DashboardSummary> {
    return this.request(`/dashboard/summary${toQuery({ month, year })}`);
  }
  cashflow(month?: number, year?: number): Promise<CashflowPoint[]> {
    return this.request(`/dashboard/cashflow${toQuery({ month, year })}`);
  }
  categorySpending(month?: number, year?: number): Promise<CategorySlice[]> {
    return this.request(`/dashboard/categories${toQuery({ month, year })}`);
  }
  upcoming(): Promise<UpcomingItem[]> {
    return this.request("/dashboard/upcoming");
  }

  // contas
  accounts(includeArchived = false): Promise<Account[]> {
    return this.request(`/accounts${includeArchived ? "?includeArchived=true" : ""}`);
  }
  createAccount(input: CreateAccountInput): Promise<Account> {
    return this.request("/accounts", { method: "POST", body: JSON.stringify(input) });
  }

  // autenticação
  authStatus(): Promise<AuthStatus> {
    return this.request("/auth/status");
  }
  login(password: string): Promise<LoginResponse> {
    return this.request("/auth/login", { method: "POST", body: JSON.stringify({ password }) });
  }

  // categorias
  categories(kind?: Category["kind"]): Promise<Category[]> {
    return this.request(`/categories${toQuery({ kind })}`);
  }
  createCategory(input: CreateCategoryInput): Promise<Category> {
    return this.request("/categories", { method: "POST", body: JSON.stringify(input) });
  }
  updateCategory(id: string, input: UpdateCategoryInput): Promise<Category> {
    return this.request(`/categories/${id}`, { method: "PATCH", body: JSON.stringify(input) });
  }
  deleteCategory(id: string): Promise<void> {
    return this.request(`/categories/${id}`, { method: "DELETE" });
  }

  // transações
  transactions(options: ListTransactionsOptions = {}): Promise<Paginated<Transaction>> {
    return this.request(`/transactions${toQuery({ ...options })}`);
  }
  createTransaction(input: CreateTransactionInput): Promise<Transaction> {
    return this.request("/transactions", { method: "POST", body: JSON.stringify(input) });
  }
  updateTransaction(id: string, input: UpdateTransactionInput): Promise<Transaction> {
    return this.request(`/transactions/${id}`, { method: "PATCH", body: JSON.stringify(input) });
  }
  confirmTransaction(id: string): Promise<Transaction> {
    return this.request(`/transactions/${id}/confirm`, { method: "PATCH" });
  }
  deleteTransaction(id: string, scope?: "single" | "group"): Promise<void> {
    return this.request(`/transactions/${id}${toQuery({ scope })}`, { method: "DELETE" });
  }
  transfer(input: TransferInput): Promise<{ transferGroupId: string }> {
    return this.request("/transactions/transfer", { method: "POST", body: JSON.stringify(input) });
  }
  exportTransactions(format: ExportFormat, options: ListTransactionsOptions = {}): Promise<string> {
    return this.request(`/transactions/export${toQuery({ ...options, format })}`);
  }

  // cartões
  cards(): Promise<Card[]> {
    return this.request("/cards");
  }
  createCard(input: CreateCardInput): Promise<Card> {
    return this.request("/cards", { method: "POST", body: JSON.stringify(input) });
  }
  addCardExpense(input: CreateCardExpenseInput): Promise<{ created: number }> {
    return this.request(`/cards/${input.cardId}/expenses`, {
      method: "POST",
      body: JSON.stringify(input),
    });
  }
  invoices(cardId: string): Promise<Invoice[]> {
    return this.request(`/cards/${cardId}/invoices`);
  }
  invoiceItems(cardId: string, invoiceId: string): Promise<Transaction[]> {
    return this.request(`/cards/${cardId}/invoices/${invoiceId}/items`);
  }
  payInvoice(
    cardId: string,
    invoiceId: string,
    accountId: string,
    date?: string,
  ): Promise<Invoice> {
    return this.request(`/cards/${cardId}/invoices/${invoiceId}/pay`, {
      method: "POST",
      body: JSON.stringify({ accountId, date }),
    });
  }

  // orçamentos
  budgets(month?: number, year?: number): Promise<Budget[]> {
    return this.request(`/budgets${toQuery({ month, year })}`);
  }
  upsertBudget(input: UpsertBudgetInput): Promise<Budget> {
    return this.request("/budgets", { method: "POST", body: JSON.stringify(input) });
  }
  deleteBudget(id: string): Promise<void> {
    return this.request(`/budgets/${id}`, { method: "DELETE" });
  }

  // metas
  goals(): Promise<Goal[]> {
    return this.request("/goals");
  }
  createGoal(input: CreateGoalInput): Promise<Goal> {
    return this.request("/goals", { method: "POST", body: JSON.stringify(input) });
  }
  contributeGoal(id: string, input: ContributeGoalInput): Promise<Goal> {
    return this.request(`/goals/${id}/contributions`, {
      method: "POST",
      body: JSON.stringify(input),
    });
  }

  // telegram
  telegramStatus(): Promise<TelegramStatus> {
    return this.request("/telegram/status");
  }
}

/** Resolve nome (parcial, sem caixa) ou id exato em uma lista nomeada. */
export function resolveByName<T extends { id: string; name: string }>(
  items: T[],
  ref: string,
): T | null {
  const exact = items.find((i) => i.id === ref || i.name.toLowerCase() === ref.toLowerCase());
  if (exact) return exact;
  const partial = items.filter((i) => i.name.toLowerCase().includes(ref.toLowerCase()));
  return partial.length === 1 ? (partial[0] ?? null) : null;
}

/** "1.234,56" | "45,9" | "50" → centavos; null quando inválido. */
export function parseBRLToCents(raw: string): number | null {
  const cleaned = raw.trim().replace(/^r\$\s*/i, "");
  if (!cleaned || !/^[\d.,]+$/.test(cleaned)) return null;
  const normalized = cleaned.includes(",") ? cleaned.replace(/\./g, "").replace(",", ".") : cleaned;
  const value = Number(normalized);
  if (!Number.isFinite(value) || value <= 0) return null;
  return Math.round(value * 100);
}

export function formatBRL(cents: number): string {
  return new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" }).format(cents / 100);
}
