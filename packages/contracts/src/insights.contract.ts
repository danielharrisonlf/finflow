import { z } from "zod";
import { spendTypeSchema } from "./category.contract";
import {
  centsSchema,
  dateStringSchema,
  idSchema,
  monthSchema,
  positiveCentsSchema,
  yearSchema,
} from "./common";
import { spendPlanSchema } from "./settings.contract";

export const insightsQuerySchema = z.object({
  month: monthSchema.optional(),
  year: yearSchema.optional(),
});
export type InsightsQuery = z.infer<typeof insightsQuerySchema>;

// ───────────────────────── overview (3.1) ─────────────────────────

export const paceStatusSchema = z.enum(["BELOW", "SAME", "ABOVE"]);
export type PaceStatus = z.infer<typeof paceStatusSchema>;

/** Ponto do gasto acumulado do mês (v3 §4); `currentCents` null depois de hoje. */
export const pacePointSchema = z.object({
  day: z.number().int(),
  currentCents: centsSchema.nullable(),
  previousCents: centsSchema,
});
export type PacePoint = z.infer<typeof pacePointSchema>;

/** Gasto do dia 1 até hoje vs. o mesmo intervalo do mês anterior. */
export const paceSchema = z.object({
  currentCents: centsSchema,
  previousSamePeriodCents: centsSchema,
  diffCents: centsSchema,
  dayOfMonth: z.number().int(),
  status: paceStatusSchema,
  /** v3: opcional para tolerar backend antigo. */
  series: z.array(pacePointSchema).optional(),
  /** Gasto até hoje ÷ dia × dias do mês (mês corrente; senão o total). */
  projectedMonthCents: centsSchema.optional(),
});
export type Pace = z.infer<typeof paceSchema>;

export const monthlyLimitSchema = z.object({
  limitCents: centsSchema.nullable(),
  spentCents: centsSchema,
  remainingCents: centsSchema.nullable(),
  /** remaining ÷ dias restantes incluindo hoje (só mês corrente). */
  dailyAllowanceCents: centsSchema.nullable(),
  percent: z.number().nullable(),
});
export type MonthlyLimit = z.infer<typeof monthlyLimitSchema>;

export const performanceVerdictSchema = z.enum([
  "SAVING",
  "POSITIVE",
  "TIGHT",
  "NEGATIVE_SAVED",
  "NEGATIVE",
]);
export type PerformanceVerdict = z.infer<typeof performanceVerdictSchema>;

/** Método da planilha diária: Performance = Entradas − (Saídas + Diário). */
export const performanceSchema = z.object({
  incomeCents: centsSchema,
  fixedCents: centsSchema,
  variableCents: centsSchema,
  totalOutCents: centsSchema,
  savedCents: centsSchema,
  performanceCents: centsSchema,
  savingsRatePct: z.number().nullable(),
  previousPerformanceCents: centsSchema,
  verdict: performanceVerdictSchema,
  message: z.string(),
});
export type Performance = z.infer<typeof performanceSchema>;

export const spendTypeCategorySchema = z.object({
  categoryId: idSchema.nullable(),
  name: z.string(),
  icon: z.string().nullable(),
  color: z.string().nullable(),
  totalCents: centsSchema,
});
export type SpendTypeCategory = z.infer<typeof spendTypeCategorySchema>;

export const spendTypeBreakdownSchema = z.object({
  spendType: spendTypeSchema,
  totalCents: centsSchema,
  percent: z.number(),
  categories: z.array(spendTypeCategorySchema),
});
export type SpendTypeBreakdown = z.infer<typeof spendTypeBreakdownSchema>;

export const reserveStatusSchema = z.enum(["NONE", "BUILDING", "PROTECTED"]);
export type ReserveStatus = z.infer<typeof reserveStatusSchema>;

export const reserveInsightSchema = z.object({
  goalId: idSchema.nullable(),
  avgMonthlyCostCents: centsSchema,
  suggestedTargetCents: centsSchema,
  targetCents: centsSchema,
  savedCents: centsSchema,
  monthsCovered: z.number(),
  status: reserveStatusSchema,
});
export type ReserveInsight = z.infer<typeof reserveInsightSchema>;

export const cutSuggestionKindSchema = z.enum(["CANCEL", "REDUCE", "ABOVE_AVERAGE"]);
export type CutSuggestionKind = z.infer<typeof cutSuggestionKindSchema>;

export const cutSuggestionSchema = z.object({
  kind: cutSuggestionKindSchema,
  categoryId: idSchema.nullable(),
  name: z.string(),
  currentCents: centsSchema,
  averageCents: centsSchema,
});
export type CutSuggestion = z.infer<typeof cutSuggestionSchema>;

/** "Você pode gastar R$ X hoje" (v3 §4). */
export const spendTodaySchema = z.object({
  cents: centsSchema,
  basis: z.enum(["LIMIT", "BALANCE"]),
});
export type SpendToday = z.infer<typeof spendTodaySchema>;

export const insightsOverviewSchema = z.object({
  month: z.number().int(),
  year: z.number().int(),
  pace: paceSchema,
  monthlyLimit: monthlyLimitSchema,
  performance: performanceSchema,
  spendTypes: z.array(spendTypeBreakdownSchema),
  plan: spendPlanSchema.nullable(),
  /** Saldos das contas ativas − faturas não pagas. */
  netWorthCents: centsSchema,
  reserve: reserveInsightSchema,
  cutSuggestions: z.array(cutSuggestionSchema),
  spendToday: spendTodaySchema.optional(),
});
export type InsightsOverview = z.infer<typeof insightsOverviewSchema>;

// ───────────────────────── projection (3.2) ─────────────────────────

export const balanceBandSchema = z.enum([
  "DARK_GREEN",
  "LIGHT_GREEN",
  "YELLOW",
  "LIGHT_RED",
  "RED",
]);
export type BalanceBand = z.infer<typeof balanceBandSchema>;

export const projectionEventSchema = z.object({
  kind: z.enum(["RECURRING", "INVOICE", "PENDING", "DAILY_SPEND", "LOAN"]),
  description: z.string(),
  /** Com sinal: + entra, − sai. */
  amountCents: centsSchema,
});
export type ProjectionEvent = z.infer<typeof projectionEventSchema>;

export const projectionDaySchema = z.object({
  date: dateStringSchema,
  inCents: centsSchema,
  outCents: centsSchema,
  balanceCents: centsSchema,
  band: balanceBandSchema,
  events: z.array(projectionEventSchema),
});
export type ProjectionDay = z.infer<typeof projectionDaySchema>;

export const projectionMonthSchema = z.object({
  month: z.number().int(),
  year: z.number().int(),
  incomeCents: centsSchema,
  recurringOutCents: centsSchema,
  installmentsCents: centsSchema,
  invoicesCents: centsSchema,
  dailySpendCents: centsSchema,
  endBalanceCents: centsSchema,
});
export type ProjectionMonth = z.infer<typeof projectionMonthSchema>;

export const projectionQuerySchema = z.object({
  days: z.coerce.number().int().min(1).max(365).default(90),
});
export type ProjectionQuery = z.infer<typeof projectionQuerySchema>;

export const projectionSchema = z.object({
  startDate: dateStringSchema,
  startBalanceCents: centsSchema,
  limitConfigured: z.boolean(),
  days: z.array(projectionDaySchema),
  /** Menor saldo nos próximos 30 dias. */
  lowest: z.object({ date: dateStringSchema, balanceCents: centsSchema }),
  nextIncomeDate: dateStringSchema.nullable(),
  canSaveNowCents: centsSchema,
  months: z.array(projectionMonthSchema),
});
export type Projection = z.infer<typeof projectionSchema>;

// ───────────────────────── year (3.3) ─────────────────────────

export const yearMonthSchema = z.object({
  month: z.number().int(),
  incomeCents: centsSchema,
  fixedCents: centsSchema,
  variableCents: centsSchema,
  totalOutCents: centsSchema,
  performanceCents: centsSchema,
  savedCents: centsSchema,
  savingsRatePct: z.number().nullable(),
  endBalanceCents: centsSchema,
});
export type YearMonth = z.infer<typeof yearMonthSchema>;

export const yearOverviewSchema = z.object({
  year: z.number().int(),
  months: z.array(yearMonthSchema),
  totals: z.object({
    incomeCents: centsSchema,
    fixedCents: centsSchema,
    variableCents: centsSchema,
    totalOutCents: centsSchema,
    performanceCents: centsSchema,
    savedCents: centsSchema,
    savingsRatePct: z.number().nullable(),
  }),
});
export type YearOverview = z.infer<typeof yearOverviewSchema>;

// ───────────────────────── commitments (3.4) ─────────────────────────

export const commitmentMonthSchema = z.object({
  month: z.number().int(),
  year: z.number().int(),
  recurringExpenseCents: centsSchema,
  recurringIncomeCents: centsSchema,
  installmentsCents: centsSchema,
  /** Parcelas de empréstimos (v3 §11); opcional para tolerar backend antigo. */
  loansCents: centsSchema.optional(),
});
export type CommitmentMonth = z.infer<typeof commitmentMonthSchema>;

export const commitmentItemSchema = z.object({
  kind: z.enum(["RECURRING", "INSTALLMENT", "LOAN"]),
  direction: z.enum(["PAY", "RECEIVE"]),
  description: z.string(),
  dueDate: dateStringSchema,
  amountCents: centsSchema,
  recurringId: idSchema.nullable(),
  realized: z.boolean(),
  installmentNumber: z.number().int().nullable(),
  installmentTotal: z.number().int().nullable(),
  cardName: z.string().nullable(),
});
export type CommitmentItem = z.infer<typeof commitmentItemSchema>;

export const commitmentsSchema = z.object({
  /** 6 meses a partir do mês consultado. */
  months: z.array(commitmentMonthSchema),
  items: z.array(commitmentItemSchema),
  totals: z.object({
    payCents: centsSchema,
    receiveCents: centsSchema,
    /** Saldo atual + receber − pagar ainda não realizados. */
    balanceAfterCents: centsSchema,
  }),
});
export type Commitments = z.infer<typeof commitmentsSchema>;

// ───────────────────────── assinaturas detectadas (v3 §2) ─────────────────────────

export const subscriptionCadenceSchema = z.enum(["WEEKLY", "MONTHLY", "YEARLY"]);
export type SubscriptionCadence = z.infer<typeof subscriptionCadenceSchema>;

export const detectedSubscriptionSchema = z.object({
  key: z.string(),
  description: z.string(),
  averageCents: centsSchema,
  lastCents: centsSchema,
  lastDate: dateStringSchema,
  occurrences: z.number().int(),
  cadence: subscriptionCadenceSchema,
  nextExpectedDate: dateStringSchema,
  /** Último vs. média anterior, quando |Δ| >= 5%. */
  priceChangePct: z.number().nullable(),
  categoryId: idSchema.nullable(),
  accountId: idSchema.nullable(),
  cardId: idSchema.nullable(),
  /** Já acompanhada como recorrência. */
  recurringId: idSchema.nullable(),
});
export type DetectedSubscription = z.infer<typeof detectedSubscriptionSchema>;

export const subscriptionsSchema = z.object({
  items: z.array(detectedSubscriptionSchema),
  /** Soma normalizada para mês (semanal × 52/12, anual ÷ 12). */
  monthlyTotalCents: centsSchema,
});
export type Subscriptions = z.infer<typeof subscriptionsSchema>;

// ───────────────────────── fluxo de caixa (v3 §5) ─────────────────────────

/** CASH: cartão conta no vencimento da fatura; ACCRUAL: na data da compra. */
export const cashflowBasisSchema = z.enum(["CASH", "ACCRUAL"]);
export type CashflowBasis = z.infer<typeof cashflowBasisSchema>;

export const cashflowReportQuerySchema = z.object({
  months: z.coerce.number().int().min(1).max(12).default(3),
  basis: cashflowBasisSchema.default("CASH"),
  month: monthSchema.optional(),
  year: yearSchema.optional(),
});
export type CashflowReportQuery = z.infer<typeof cashflowReportQuerySchema>;

export const cashflowMonthSchema = z.object({
  month: z.number().int(),
  year: z.number().int(),
  incomeCents: centsSchema,
  /** Despesas com recorrência ou empréstimo. */
  fixedBillsCents: centsSchema,
  /** Lançamentos parcelados (installment_total > 1). */
  installmentsCents: centsSchema,
  /** O resto: gastos do dia a dia. */
  dailyCents: centsSchema,
  totalOutCents: centsSchema,
  resultCents: centsSchema,
  /** Variação do resultado vs. mês anterior. */
  resultChangePct: z.number().nullable(),
});
export type CashflowMonth = z.infer<typeof cashflowMonthSchema>;

export const cashflowWhereSchema = z.object({
  categoryId: idSchema.nullable(),
  name: z.string(),
  icon: z.string().nullable(),
  color: z.string().nullable(),
  totalCents: centsSchema,
  percent: z.number(),
});
export type CashflowWhere = z.infer<typeof cashflowWhereSchema>;

export const cashflowReportSchema = z.object({
  basis: cashflowBasisSchema,
  months: z.array(cashflowMonthSchema),
  /** Para onde foi o dinheiro no último mês. */
  whereItWent: z.array(cashflowWhereSchema),
});
export type CashflowReport = z.infer<typeof cashflowReportSchema>;

// ───────────────────────── "Posso comprar?" (v3 §8) ─────────────────────────

export const canAffordSchema = z.object({
  amountCents: positiveCentsSchema,
  date: dateStringSchema.optional(),
  installments: z.number().int().min(1).max(48).optional(),
  cardId: idSchema.optional(),
  description: z.string().max(120).optional(),
});
export type CanAffordInput = z.infer<typeof canAffordSchema>;

export const canAffordVerdictSchema = z.enum(["YES", "CAREFUL", "NO"]);
export type CanAffordVerdict = z.infer<typeof canAffordVerdictSchema>;

export const canAffordResultSchema = z.object({
  verdict: canAffordVerdictSchema,
  /** Menor saldo projetado (365 dias) com a compra. */
  lowestBalanceAfterCents: centsSchema,
  lowestDate: dateStringSchema,
  /** Impacto no limite mensal do mês da compra. */
  limit: z
    .object({ limitCents: centsSchema, spentAfterCents: centsSchema, percentAfter: z.number() })
    .nullable(),
  /** Quando a compra precisaria sair da reserva. */
  reserveMonthsAfter: z.number().nullable(),
  /** 1ª data em que caberia sem saldo negativo (verdict NO). */
  fitsOn: dateStringSchema.nullable(),
  message: z.string(),
});
export type CanAffordResult = z.infer<typeof canAffordResultSchema>;

// ───────────────────────── evolução do patrimônio (v3 §12) ─────────────────────────

export const netWorthPointSchema = z.object({
  month: z.number().int(),
  year: z.number().int(),
  cashCents: centsSchema,
  investmentsCents: centsSchema,
  debtsCents: centsSchema,
  netWorthCents: centsSchema,
});
export type NetWorthPoint = z.infer<typeof netWorthPointSchema>;
