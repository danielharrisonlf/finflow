import type {
  Account,
  Alert,
  ApplyRulesInput,
  ApplyRulesResult,
  AuthStatus,
  Budget,
  BusinessDre,
  BusinessMetrics,
  CanAffordInput,
  CanAffordResult,
  Card,
  CashflowBasis,
  CashflowPoint,
  CashflowReport,
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
  CreateInvestmentInput,
  CreateLoanInput,
  CreateMemberInput,
  CreateRecurringInput,
  CreateSpaceInput,
  CreateTagInput,
  CreateTransactionInput,
  CreateWishItemInput,
  DashboardSummary,
  DebtStrategy,
  Goal,
  ImportCommitInput,
  ImportCommitResult,
  ImportMapping,
  ImportPreview,
  InsightsOverview,
  Investment,
  InvestmentContributionInput,
  InvestmentRedeemInput,
  InvestmentSummary,
  Invoice,
  InvoiceBreakdown,
  LinkPreview,
  Loan,
  LoginInput,
  LoginResponse,
  Member,
  MemberBalances,
  MergeTransferInput,
  NetWorthPoint,
  Paginated,
  PayInvoiceInput,
  PayLoanInput,
  PostRecurringInput,
  Projection,
  Recurring,
  Settings,
  SettleMembersInput,
  Space,
  SpacesOverview,
  SpaceTotals,
  Subscriptions,
  SystemInfo,
  Tag,
  TagSpending,
  Transaction,
  TransferCandidate,
  TransferInput,
  UpcomingItem,
  UpdateCategoryInput,
  UpdateCategoryRuleInput,
  UpdateGoalInput,
  UpdateInvestmentInput,
  UpdateInvestmentValueInput,
  UpdateLoanInput,
  UpdateMemberInput,
  UpdateRecurringInput,
  UpdateSettingsInput,
  UpdateTagInput,
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
  tagId?: string;
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

// ───────────────────────── v3: consultas ─────────────────────────
// Respostas novas podem chegar incompletas enquanto o backend não acompanha: os hooks
// normalizam listas e objetos para a tela nunca quebrar.

const asArray = <T>(value: unknown): T[] => (Array.isArray(value) ? (value as T[]) : []);

export const useTags = () =>
  useQuery({
    queryKey: ["tags"],
    queryFn: async () => asArray<Tag>(await api.get<unknown>("/tags")),
  });

export const useTagSpending = (p?: Period) =>
  useQuery({
    queryKey: ["insights", "tags", p],
    queryFn: async () =>
      asArray<TagSpending>(await api.get<unknown>(`/insights/tags${periodQS(p)}`)),
  });

export const useAlerts = () =>
  useQuery({
    queryKey: ["alerts"],
    queryFn: async () => asArray<Alert>(await api.get<unknown>("/alerts")),
  });

export const useSubscriptions = () =>
  useQuery({
    queryKey: ["insights", "subscriptions"],
    queryFn: async (): Promise<Subscriptions> => {
      const res = await api.get<Partial<Subscriptions> | null>("/insights/subscriptions");
      return { items: asArray(res?.items), monthlyTotalCents: res?.monthlyTotalCents ?? 0 };
    },
  });

export const useCashflowReport = (months: number, basis: CashflowBasis) =>
  useQuery({
    queryKey: ["insights", "cashflow", months, basis],
    queryFn: async (): Promise<CashflowReport> => {
      const res = await api.get<Partial<CashflowReport> | null>(
        `/insights/cashflow?months=${months}&basis=${basis}`,
      );
      return {
        basis: res?.basis ?? basis,
        months: asArray(res?.months),
        whereItWent: asArray(res?.whereItWent),
      };
    },
  });

export const useNetWorthHistory = (months = 12) =>
  useQuery({
    queryKey: ["insights", "net-worth", months],
    queryFn: async () =>
      asArray<NetWorthPoint>(await api.get<unknown>(`/insights/net-worth?months=${months}`)),
  });

export const useInvoiceBreakdown = (cardId: string | null, invoiceId: string | null) =>
  useQuery({
    queryKey: ["cards", cardId, "invoices", invoiceId, "breakdown"],
    queryFn: () => api.get<InvoiceBreakdown>(`/cards/${cardId}/invoices/${invoiceId}/breakdown`),
    enabled: Boolean(cardId && invoiceId),
  });

export const useTransferCandidates = () =>
  useQuery({
    queryKey: ["transactions", "transfer-candidates"],
    queryFn: async () =>
      asArray<TransferCandidate>(await api.get<unknown>("/transactions/transfer-candidates")),
  });

/** Busca livre em todo o histórico (paleta de comandos); só dispara com 2+ caracteres. */
export const useTransactionSearch = (term: string) =>
  useQuery({
    queryKey: ["transactions", "search", term],
    queryFn: async () => {
      const res = await api.get<Partial<Paginated<Transaction>> | null>(
        `/transactions?search=${encodeURIComponent(term)}&pageSize=8`,
      );
      return asArray<Transaction>(res?.items).slice(0, 8);
    },
    enabled: term.trim().length >= 2,
    staleTime: 30_000,
  });

export const useInvestments = () =>
  useQuery({
    queryKey: ["investments"],
    queryFn: async () => asArray<Investment>(await api.get<unknown>("/investments")),
  });

export const useInvestmentSummary = () =>
  useQuery({
    queryKey: ["investments", "summary"],
    queryFn: async (): Promise<InvestmentSummary> => {
      const res = await api.get<Partial<InvestmentSummary> | null>("/investments/summary");
      return {
        totalInvestedCents: res?.totalInvestedCents ?? 0,
        totalCurrentCents: res?.totalCurrentCents ?? 0,
        returnCents: res?.returnCents ?? 0,
        returnPct: res?.returnPct ?? null,
        byKind: asArray(res?.byKind),
        byInstitution: asArray(res?.byInstitution),
        history: asArray(res?.history),
      };
    },
  });

export const useLoans = () =>
  useQuery({
    queryKey: ["loans"],
    queryFn: async () => asArray<Loan>(await api.get<unknown>("/loans")),
  });

export const useDebtStrategy = (extraCents: number, enabled = true) =>
  useQuery({
    queryKey: ["loans", "strategy", extraCents],
    queryFn: () => api.get<DebtStrategy>(`/loans/strategy?extraCents=${extraCents}`),
    enabled,
  });

export const useSystemInfo = () =>
  useQuery({
    queryKey: ["system", "info"],
    queryFn: () => api.get<SystemInfo>("/system/info"),
    staleTime: 60_000,
  });

/** GET /api/ai/status (v3 §15); a chave em si nunca volta para o frontend. */
export interface AiStatusInfo {
  configured: boolean;
  source: "ENV" | "SETTINGS" | null;
  model: string;
}

export const useAiStatus = () =>
  useQuery({
    queryKey: ["ai", "status"],
    queryFn: () => api.get<AiStatusInfo>("/ai/status"),
    retry: false,
  });

// ───────────────────────── v3: escritas ─────────────────────────

export const useCreateTag = () =>
  useInvalidatingMutation((input: CreateTagInput) => api.post<Tag>("/tags", input));

export const useUpdateTag = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateTagInput & { id: string }) =>
    api.patch<Tag>(`/tags/${id}`, input),
  );

export const useDeleteTag = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/tags/${id}`));

/** Dispensar some da lista na hora (otimista) e só recarrega o feed de alertas. */
export const useDismissAlert = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post<void>(`/alerts/${encodeURIComponent(id)}/dismiss`),
    onMutate: async (id: string) => {
      await queryClient.cancelQueries({ queryKey: ["alerts"] });
      const previous = queryClient.getQueryData<Alert[]>(["alerts"]);
      queryClient.setQueryData<Alert[]>(["alerts"], (list) => list?.filter((a) => a.id !== id));
      return { previous };
    },
    onError: (_error, _id, context) => {
      if (context?.previous) queryClient.setQueryData(["alerts"], context.previous);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["alerts"] }),
  });
};

export const useTrackSubscription = () =>
  useInvalidatingMutation((key: string) =>
    api.post<Recurring>(`/insights/subscriptions/${encodeURIComponent(key)}/track`),
  );

export const useDismissSubscription = () =>
  useInvalidatingMutation((key: string) =>
    api.post<void>(`/insights/subscriptions/${encodeURIComponent(key)}/dismiss`),
  );

export const useMergeTransfer = () =>
  useInvalidatingMutation((input: MergeTransferInput) =>
    api.post<{ transferGroupId: string }>("/transactions/merge-transfer", input),
  );

/** Simulação: não grava nada, então não invalida cache. */
export const useCanAfford = () =>
  useMutation({
    mutationFn: (input: CanAffordInput) => api.post<CanAffordResult>("/insights/can-afford", input),
  });

/** Só lê a página do produto: não invalida cache. */
export const usePreviewLink = () =>
  useMutation({
    mutationFn: (url: string) => api.post<LinkPreview>("/wishlist/preview-link", { url }),
  });

export const useCreateInvestment = () =>
  useInvalidatingMutation((input: CreateInvestmentInput) =>
    api.post<Investment>("/investments", input),
  );

export const useUpdateInvestment = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateInvestmentInput & { id: string }) =>
    api.patch<Investment>(`/investments/${id}`, input),
  );

export const useDeleteInvestment = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/investments/${id}`));

export const useUpdateInvestmentValue = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateInvestmentValueInput & { id: string }) =>
    api.post<Investment>(`/investments/${id}/value`, input),
  );

export const useContributeInvestment = () =>
  useInvalidatingMutation(({ id, ...input }: InvestmentContributionInput & { id: string }) =>
    api.post<Investment>(`/investments/${id}/contribution`, input),
  );

export const useRedeemInvestment = () =>
  useInvalidatingMutation(({ id, ...input }: InvestmentRedeemInput & { id: string }) =>
    api.post<Investment>(`/investments/${id}/redeem`, input),
  );

export const useCreateLoan = () =>
  useInvalidatingMutation((input: CreateLoanInput) => api.post<Loan>("/loans", input));

export const useUpdateLoan = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateLoanInput & { id: string }) =>
    api.patch<Loan>(`/loans/${id}`, input),
  );

export const useDeleteLoan = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/loans/${id}`));

export const usePayLoan = () =>
  useInvalidatingMutation(({ id, ...input }: PayLoanInput & { id: string }) =>
    api.post<Transaction>(`/loans/${id}/pay`, input),
  );

// ───────────────────────── v3 fase 2: espaços ─────────────────────────
// A lista de espaços e a visão unificada não dependem do espaço ativo, mas vão com o cabeçalho
// como qualquer chamada; trocar de espaço limpa o cache inteiro (main.tsx).

const PERSONAL_SPACE: Space = {
  slug: "pessoal",
  name: "Pessoal",
  kind: "PERSONAL",
  color: "#34d399",
  isDefault: true,
};

/** Lista de espaços; sem catálogo (ou backend antigo) = só o pessoal. */
export const useSpaces = () =>
  useQuery({
    queryKey: ["spaces"],
    queryFn: async () => {
      const list = asArray<Space>(await api.get<unknown>("/spaces")).filter(
        (s) => typeof s?.slug === "string",
      );
      return list.some((s) => s.slug === PERSONAL_SPACE.slug) ? list : [PERSONAL_SPACE, ...list];
    },
    staleTime: 60_000,
  });

const emptyTotals = (t?: Partial<SpaceTotals> | null): SpaceTotals => ({
  balanceCents: t?.balanceCents ?? 0,
  netWorthCents: t?.netWorthCents ?? 0,
  monthIncomeCents: t?.monthIncomeCents ?? 0,
  monthOutCents: t?.monthOutCents ?? 0,
  monthResultCents: t?.monthResultCents ?? 0,
});

export const useSpacesOverview = () =>
  useQuery({
    queryKey: ["spaces", "overview"],
    queryFn: async (): Promise<SpacesOverview> => {
      const res = await api.get<Partial<SpacesOverview> | null>("/spaces/overview");
      return {
        spaces: asArray<SpacesOverview["spaces"][number]>(res?.spaces).map((s) => ({
          ...s,
          ...emptyTotals(s),
        })),
        totals: emptyTotals(res?.totals),
      };
    },
  });

/** Criar espaço não mexe nos dados do espaço atual: só recarrega a lista. */
export const useCreateSpace = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateSpaceInput) => api.post<Space>("/spaces", input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["spaces"] }),
  });
};

// casa: membros e divisão

export const useMembers = (enabled = true) =>
  useQuery({
    queryKey: ["members"],
    queryFn: async () => asArray<Member>(await api.get<unknown>("/members")),
    enabled,
  });

export const useMemberBalances = (p: Period) =>
  useQuery({
    queryKey: ["members", "balances", p],
    queryFn: async (): Promise<MemberBalances> => {
      const res = await api.get<Partial<MemberBalances> | null>(`/members/balances${periodQS(p)}`);
      return { members: asArray(res?.members), settlements: asArray(res?.settlements) };
    },
  });

export const useCreateMember = () =>
  useInvalidatingMutation((input: CreateMemberInput) => api.post<Member>("/members", input));

export const useUpdateMember = () =>
  useInvalidatingMutation(({ id, ...input }: UpdateMemberInput & { id: string }) =>
    api.patch<Member>(`/members/${id}`, input),
  );

export const useDeleteMember = () =>
  useInvalidatingMutation((id: string) => api.delete<void>(`/members/${id}`));

export const useSettleMembers = () =>
  useInvalidatingMutation((input: SettleMembersInput) =>
    api.post<unknown>("/members/settle", input),
  );

// empresa: métricas e DRE

export const useBusinessMetrics = (p: Period) =>
  useQuery({
    queryKey: ["business", "metrics", p],
    queryFn: async (): Promise<BusinessMetrics> => {
      const res = await api.get<Partial<BusinessMetrics> | null>(`/business/metrics${periodQS(p)}`);
      return {
        mrrCents: res?.mrrCents ?? 0,
        arrCents: res?.arrCents ?? 0,
        newMrrCents: res?.newMrrCents ?? 0,
        churnedMrrCents: res?.churnedMrrCents ?? 0,
        churnPct: res?.churnPct ?? null,
        burnCents: res?.burnCents ?? 0,
        runwayMonths: res?.runwayMonths ?? null,
        cashCents: res?.cashCents ?? 0,
      };
    },
  });

export const useBusinessDre = (year: number) =>
  useQuery({
    queryKey: ["business", "dre", year],
    queryFn: async (): Promise<BusinessDre | null> => {
      const res = await api.get<Partial<BusinessDre> | null>(`/business/dre?year=${year}`);
      if (!res?.totals) return null;
      return { months: asArray(res.months), totals: res.totals };
    },
  });
