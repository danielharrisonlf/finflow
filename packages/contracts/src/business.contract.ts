import { z } from "zod";
import { centsSchema } from "./common";

/** Grupo do DRE de uma categoria (v3 fase 2 §5). */
export const dreGroupSchema = z.enum([
  "REVENUE",
  "DEDUCTIONS",
  "COSTS",
  "OPEX",
  "FINANCIAL",
  "OTHER",
]);
export type DreGroup = z.infer<typeof dreGroupSchema>;

/** GET /api/business/metrics?month&year. */
export const businessMetricsSchema = z.object({
  mrrCents: centsSchema,
  arrCents: centsSchema,
  newMrrCents: centsSchema,
  churnedMrrCents: centsSchema,
  /** churnedMrr ÷ MRR no início do mês. */
  churnPct: z.number().nullable(),
  /** Média de (saídas − entradas) dos 3 meses anteriores; 0 se positivo. */
  burnCents: centsSchema,
  runwayMonths: z.number().nullable(),
  cashCents: centsSchema,
});
export type BusinessMetrics = z.infer<typeof businessMetricsSchema>;

const dreLineSchema = z.object({
  revenueCents: centsSchema,
  deductionsCents: centsSchema,
  netRevenueCents: centsSchema,
  costsCents: centsSchema,
  grossProfitCents: centsSchema,
  opexCents: centsSchema,
  operatingResultCents: centsSchema,
  financialCents: centsSchema,
  netResultCents: centsSchema,
  marginPct: z.number().nullable(),
});
export type DreLine = z.infer<typeof dreLineSchema>;

/** GET /api/business/dre?year. */
export const businessDreSchema = z.object({
  months: z.array(dreLineSchema.extend({ month: z.number().int() })),
  totals: dreLineSchema,
});
export type BusinessDre = z.infer<typeof businessDreSchema>;
export type DreMonth = BusinessDre["months"][number];
