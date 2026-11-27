import { z } from "zod";
import { centsSchema } from "./common";

/** Limites inferiores (centavos) de cada faixa de saldo projetado. */
export const balanceBandsSchema = z.object({
  darkGreen: centsSchema,
  lightGreen: centsSchema,
  yellow: centsSchema,
  lightRed: centsSchema,
});
export type BalanceBands = z.infer<typeof balanceBandsSchema>;

const pctSchema = z.number().min(0).max(100);

/** "Seu plano" de distribuição dos gastos por tipo (soma 100). */
export const spendPlanSchema = z.object({
  fixedPct: pctSchema,
  cuttablePct: pctSchema,
  variablePct: pctSchema,
});
export type SpendPlan = z.infer<typeof spendPlanSchema>;

export const settingsSchema = z.object({
  monthlyLimitCents: centsSchema.nullable(),
  reserveMonths: z.number().int().min(1),
  hasDependents: z.boolean(),
  reserveRefillRatePct: pctSchema,
  balanceBands: balanceBandsSchema,
  spendPlan: spendPlanSchema.nullable(),
});
export type Settings = z.infer<typeof settingsSchema>;

/** PATCH /api/settings faz merge parcial. */
export const updateSettingsSchema = settingsSchema.partial();
export type UpdateSettingsInput = z.infer<typeof updateSettingsSchema>;
