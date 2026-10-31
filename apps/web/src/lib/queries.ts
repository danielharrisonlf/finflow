import type {
  Account,
  Budget,
  Card,
  CashflowPoint,
  Category,
  CategorySlice,
  ContributeGoalInput,
  CreateAccountInput,
  CreateCardExpenseInput,
  CreateCardInput,
  CreateGoalInput,
  CreateTransactionInput,
  DashboardSummary,
  Goal,
  Invoice,
  Paginated,
  PayInvoiceInput,
  Transaction,
  TransferInput,
  UpcomingItem,
  UpsertBudgetInput,
} from "@finflow/contracts";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";

export interface Period {
  month: number;
  year: number;
}

const periodQS = (p?: Period): string => (p ? `?month=${p.month}&year=${p.year}` : "");

// ───────────────────────── queries ─────────────────────────

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

export const useCreateTransaction = () =>
  useInvalidatingMutation((input: CreateTransactionInput) =>
    api.post<Transaction>("/transactions", input),
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
