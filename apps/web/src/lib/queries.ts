import type {
  Account,
  ApplyRulesInput,
  ApplyRulesResult,
  AuthStatus,
  Budget,
  Card,
  CashflowPoint,
  Category,
  CategoryRule,
  CategorySlice,
  Commitments,
  ContributeGoalInput,
  CreateAccountInput,
  CreateCardExpenseInput,
  CreateCardInput,
  CreateCategoryInput,
  CreateCategoryRuleInput,
  CreateGoalInput,
  CreateRecurringInput,
  CreateTransactionInput,
  CreateWishItemInput,
  DashboardSummary,
  Goal,
  ImportCommitInput,
  ImportCommitResult,
  ImportMapping,
  ImportPreview,
  InsightsOverview,
  Invoice,
  LoginInput,
  LoginResponse,
  Paginated,
  PayInvoiceInput,
  PostRecurringInput,
  Projection,
  Recurring,
  Settings,
  Transaction,
  TransferInput,
  UpcomingItem,
  UpdateCategoryInput,
  UpdateCategoryRuleInput,
  UpdateGoalInput,
  UpdateRecurringInput,
  UpdateSettingsInput,
  UpdateTransactionInput,
  UpdateWishItemInput,
  UpsertBudgetInput,
  WishItem,
  YearOverview,
} from "@finflow/contracts";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, setAuthToken } from "./api";

export interface Period {
  month: number;
  year: number;
}

const periodQS = (p?: Period): string => (p ? `?month=${p.month}&year=${p.year}` : "");

// ───────────────────────── queries ─────────────────────────

export const useAuthStatus = () =>
  useQuery({
    queryKey: ["auth", "status"],
    queryFn: () => api.get<AuthStatus>("/auth/status"),
  });

export const useSummary = (p?: Period) =>
  useQuery({
    queryKey: ["dashboard", "summary", p],
    queryFn: () => api.get<DashboardSummary>(`/dashboard/summary${periodQS(p)}`),
  });

export const useCashflow = (p?: Period) =>
  useQuery({
    queryKey: ["dashboard", "cashflow", p],
    queryFn: () => api.get<CashflowPoint[]>(`/dashboard/cashflow${periodQS(p)}`),
  });

export const useCategorySlices = (p?: Period) =>
  useQuery({
    queryKey: ["dashboard", "categories", p],
    queryFn: () => api.get<CategorySlice[]>(`/dashboard/categories${periodQS(p)}`),
  });

export const useUpcoming = () =>
  useQuery({
    queryKey: ["dashboard", "upcoming"],
    queryFn: () => api.get<UpcomingItem[]>("/dashboard/upcoming"),
  });

export const useAccounts = () =>
  useQuery({ queryKey: ["accounts"], queryFn: () => api.get<Account[]>("/accounts") });

export const useCards = () =>
  useQuery({ queryKey: ["cards"], queryFn: () => api.get<Card[]>("/cards") });

export const useInvoices = (cardId: string | null) =>
  useQuery({
    queryKey: ["cards", cardId, "invoices"],
    queryFn: () => api.get<Invoice[]>(`/cards/${cardId}/invoices`),
    enabled: Boolean(cardId),
  });

export const useInvoiceItems = (cardId: string | null, invoiceId: string | null) =>
  useQuery({
    queryKey: ["cards", cardId, "invoices", invoiceId, "items"],
    queryFn: () => api.get<Transaction[]>(`/cards/${cardId}/invoices/${invoiceId}/items`),
    enabled: Boolean(cardId && invoiceId),
  });

export const useCategories = (kind?: "INCOME" | "EXPENSE") =>
  useQuery({
    queryKey: ["categories", kind],
    queryFn: () => api.get<Category[]>(`/categories${kind ? `?kind=${kind}` : ""}`),
  });

export interface TransactionFilters {
  month?: number;
  year?: number;
  accountId?: string;
  cardId?: string;
  categoryId?: string;
  type?: Transaction["type"];
  status?: Transaction["status"];
  search?: string;
  page?: number;
}

export function filtersToParams(filters: TransactionFilters): URLSearchParams {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value !== undefined && value !== "") params.set(key, String(value));
  }
  return params;
}

export const useTransactions = (filters: TransactionFilters) =>
  useQuery({
    queryKey: ["transactions", filters],
    queryFn: () =>
      api.get<Paginated<Transaction>>(`/transactions?${filtersToParams(filters).toString()}`),
  });

export const useBudgets = (p?: Period) =>
  useQuery({
    queryKey: ["budgets", p],
    queryFn: () => api.get<Budget[]>(`/budgets${periodQS(p)}`),
  });

export const useGoals = () =>
  useQuery({ queryKey: ["goals"], queryFn: () => api.get<Goal[]>("/goals") });

export const useRecurring = () =>
  useQuery({ queryKey: ["recurring"], queryFn: () => api.get<Recurring[]>("/recurring") });

export const useSettings = () =>
  useQuery({ queryKey: ["settings"], queryFn: () => api.get<Settings>("/settings") });

export const useRules = () =>
  useQuery({ queryKey: ["rules"], queryFn: () => api.get<CategoryRule[]>("/rules") });

export const useWishlist = () =>
  useQuery({ queryKey: ["wishlist"], queryFn: () => api.get<WishItem[]>("/wishlist") });

// insights: tudo sob a chave "insights" para invalidar junto após qualquer escrita

export const useInsightsOverview = (p?: Period) =>
  useQuery({
    queryKey: ["insights", "overview", p],
    queryFn: () => api.get<InsightsOverview>(`/insights/overview${periodQS(p)}`),
  });

export const useProjection = (days = 90) =>
  useQuery({
    queryKey: ["insights", "projection", days],
    queryFn: () => api.get<Projection>(`/insights/projection?days=${days}`),
  });

export const useYearOverview = (year: number) =>
  useQuery({
    queryKey: ["insights", "year", year],
    queryFn: () => api.get<YearOverview>(`/insights/year?year=${year}`),
  });

export const useCommitments = (p?: Period) =>
  useQuery({
    queryKey: ["insights", "commitments", p],
    queryFn: () => api.get<Commitments>(`/insights/commitments${periodQS(p)}`),
  });

// ───────────────────────── mutations ─────────────────────────
// Finanças são interligadas (saldo ↔ extrato ↔ dashboard ↔ orçamento):
// invalidar tudo após cada escrita é correto e barato nesta escala.

function useInvalidatingMutation<TInput, TOutput>(fn: (input: TInput) => Promise<TOutput>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => queryClient.invalidateQueries(),
  });
}

export const useLogin = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: LoginInput) => api.post<LoginResponse>("/auth/login", input),
    onSuccess: (res) => {
      setAuthToken(res.token);
      queryClient.invalidateQueries();
    },
  });
};

export const useCreateTransaction = () =>
  useInvalidatingMutation((input: CreateTransactionInput) =>
    api.post<Transaction>("/transactions", input),
  );

export const useUpdateTransaction = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateTransactionInput & { id: string }) =>
    api.patch<Transaction>(`/transactions/${id}`, input),
  );

export const useTransfer = () =>
  useInvalidatingMutation((input: TransferInput) =>
    api.post<{ transferGroupId: string }>("/transactions/transfer", input),
  );

export const useCreateCardExpense = () =>
  useInvalidatingMutation((input: CreateCardExpenseInput) =>
    api.post<{ created: number }>(`/cards/${input.cardId}/expenses`, input),
  );

export const useConfirmTransaction = () =>
  useInvalidatingMutation((id: string) => api.patch<Transaction>(`/transactions/${id}/confirm`));

export const useDeleteTransaction = () =>
  useInvalidatingMutation(({ id, scope }: { id: string; scope?: "single" | "group" }) =>
    api.delete<void>(`/transactions/${id}${scope ? `?scope=${scope}` : ""}`),
  );

export const useCreateCategory = () =>
  useInvalidatingMutation((input: CreateCategoryInput) => api.post<Category>("/categories", input));

export const useUpdateCategory = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateCategoryInput & { id: string }) =>
    api.patch<Category>(`/categories/${id}`, input),
  );

export const useDeleteCategory = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/categories/${id}`));

export const useCreateAccount = () =>
  useInvalidatingMutation((input: CreateAccountInput) => api.post<Account>("/accounts", input));

export const useCreateCard = () =>
  useInvalidatingMutation((input: CreateCardInput) => api.post<Card>("/cards", input));

export const usePayInvoice = () =>
  useInvalidatingMutation(
    ({ cardId, invoiceId, ...input }: PayInvoiceInput & { cardId: string; invoiceId: string }) =>
      api.post<Invoice>(`/cards/${cardId}/invoices/${invoiceId}/pay`, input),
  );

export const useUpsertBudget = () =>
  useInvalidatingMutation((input: UpsertBudgetInput) => api.post<Budget>("/budgets", input));

export const useDeleteBudget = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/budgets/${id}`));

export const useCreateGoal = () =>
  useInvalidatingMutation((input: CreateGoalInput) => api.post<Goal>("/goals", input));

export const useContributeGoal = () =>
  useInvalidatingMutation(({ id, ...input }: ContributeGoalInput & { id: string }) =>
    api.post<Goal>(`/goals/${id}/contributions`, input),
  );

export const useUpdateGoal = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateGoalInput & { id: string }) =>
    api.patch<Goal>(`/goals/${id}`, input),
  );

export const useCreateRecurring = () =>
  useInvalidatingMutation((input: CreateRecurringInput) =>
    api.post<Recurring>("/recurring", input),
  );

export const useUpdateRecurring = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateRecurringInput & { id: string }) =>
    api.patch<Recurring>(`/recurring/${id}`, input),
  );

export const useDeleteRecurring = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/recurring/${id}`));

/** Lança a ocorrência como transação: mexe em saldo, extrato, dashboard e insights. */
export const usePostRecurring = () =>
  useInvalidatingMutation(({ id, ...input }: PostRecurringInput & { id: string }) =>
    api.post<Transaction>(`/recurring/${id}/post`, input),
  );

export const useUpdateSettings = () =>
  useInvalidatingMutation((input: UpdateSettingsInput) => api.patch<Settings>("/settings", input));

export const useCreateRule = () =>
  useInvalidatingMutation((input: CreateCategoryRuleInput) =>
    api.post<CategoryRule>("/rules", input),
  );

export const useUpdateRule = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateCategoryRuleInput & { id: string }) =>
    api.patch<CategoryRule>(`/rules/${id}`, input),
  );

export const useDeleteRule = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/rules/${id}`));

export const useApplyRules = () =>
  useInvalidatingMutation((input: ApplyRulesInput) =>
    api.post<ApplyRulesResult>("/rules/apply", input),
  );

export const useCreateWish = () =>
  useInvalidatingMutation((input: CreateWishItemInput) => api.post<WishItem>("/wishlist", input));

export const useUpdateWish = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateWishItemInput & { id: string }) =>
    api.patch<WishItem>(`/wishlist/${id}`, input),
  );

export const useDeleteWish = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/wishlist/${id}`));

/**
 * Reordena desejos: recebe a lista já na nova ordem e renumera `priority` (0..n-1),
 * enviando PATCH só para quem mudou. Robusto a prioridades repetidas.
 */
export const useReorderWishes = () =>
  useInvalidatingMutation(async (ordered: WishItem[]) => {
    for (const [index, item] of ordered.entries()) {
      if (item.priority !== index) {
        await api.patch<WishItem>(`/wishlist/${item.id}`, { priority: index });
      }
    }
  });

export interface ImportPreviewInput {
  file: File;
  mapping?: ImportMapping;
  sheet?: string | null;
  accountId?: string;
}

/** Só lê o arquivo — nada é gravado, então não invalida cache. */
export const useImportPreview = () =>
  useMutation({
    mutationFn: ({ file, mapping, sheet, accountId }: ImportPreviewInput) => {
      const form = new FormData();
      form.append("file", file);
      if (mapping) form.append("mapping", JSON.stringify(mapping));
      if (sheet) form.append("sheet", sheet);
      if (accountId) form.append("accountId", accountId);
      return api.upload<ImportPreview>("/import/preview", form);
    },
  });

export const useCommitImport = () =>
  useInvalidatingMutation((input: ImportCommitInput) =>
    api.post<ImportCommitResult>("/import/commit", input),
  );
