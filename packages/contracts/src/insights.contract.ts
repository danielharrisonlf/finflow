import { z } from "zod";
import { spendTypeSchema } from "./category.contract";
import { centsSchema, dateStringSchema, idSchema, monthSchema, yearSchema } from "./common";
import { spendPlanSchema } from "./settings.contract";

export const insightsQuerySchema = z.object({
  month: monthSchema.optional(),
  year: yearSchema.optional(),
});
export type InsightsQuery = z.infer<typeof insightsQuerySchema>;

// ───────────────────────── overview (3.1) ─────────────────────────

export const paceStatusSchema = z.enum(["BELOW", "SAME", "ABOVE"]);
export type PaceStatus = z.infer<typeof paceStatusSchema>;

/** Gasto do dia 1 até hoje vs. o mesmo intervalo do mês anterior. */
export const paceSchema = z.object({
  currentCents: centsSchema,
  previousSamePeriodCents: centsSchema,
  diffCents: centsSchema,
  dayOfMonth: z.number().int(),
  status: paceStatusSchema,
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
  kind: z.enum(["RECURRING", "INVOICE", "PENDING", "DAILY_SPEND"]),
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
});
export type CommitmentMonth = z.infer<typeof commitmentMonthSchema>;

export const commitmentItemSchema = z.object({
  kind: z.enum(["RECURRING", "INSTALLMENT"]),
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
