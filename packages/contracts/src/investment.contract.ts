import { z } from "zod";
import { centsSchema, dateStringSchema, idSchema, positiveCentsSchema } from "./common";

/** Investimentos: posições, rentabilidade e alocação (v3 §10). */
export const investmentKindSchema = z.enum([
  "FIXED_INCOME",
  "TREASURY",
  "STOCKS",
  "FII",
  "FUNDS",
  "CRYPTO",
  "PENSION",
  "OTHER",
]);
export type InvestmentKind = z.infer<typeof investmentKindSchema>;

export const createInvestmentSchema = z.object({
  name: z.string().trim().min(1).max(80),
  kind: investmentKindSchema,
  institution: z.string().max(80).nullish(),
  /** Conta INVESTMENT vinculada (aportes/resgates com transferência). */
  accountId: idSchema.nullish(),
  investedCents: centsSchema.min(0),
  currentCents: centsSchema.min(0),
  notes: z.string().max(500).nullish(),
});
export type CreateInvestmentInput = z.infer<typeof createInvestmentSchema>;

export const updateInvestmentSchema = createInvestmentSchema.partial().extend({
  archived: z.boolean().optional(),
});
export type UpdateInvestmentInput = z.infer<typeof updateInvestmentSchema>;

export const investmentSchema = z.object({
  id: idSchema,
  name: z.string(),
  kind: investmentKindSchema,
  institution: z.string().nullable(),
  accountId: idSchema.nullable(),
  accountName: z.string().nullable(),
  investedCents: centsSchema,
  currentCents: centsSchema,
  returnCents: centsSchema,
  returnPct: z.number().nullable(),
  valueUpdatedAt: dateStringSchema.nullable(),
  notes: z.string().nullable(),
  archived: z.boolean(),
});
export type Investment = z.infer<typeof investmentSchema>;

/** POST /api/investments/{id}/value: atualiza o valor e grava snapshot. */
export const updateInvestmentValueSchema = z.object({
  currentCents: centsSchema.min(0),
  date: dateStringSchema.optional(),
});
export type UpdateInvestmentValueInput = z.infer<typeof updateInvestmentValueSchema>;

/** Aporte; com `fromAccountId` cria TRANSFER para a conta vinculada. */
export const investmentContributionSchema = z.object({
  amountCents: positiveCentsSchema,
  date: dateStringSchema.optional(),
  fromAccountId: idSchema.optional(),
});
export type InvestmentContributionInput = z.infer<typeof investmentContributionSchema>;

/** Resgate; com `toAccountId` cria TRANSFER da conta vinculada. */
export const investmentRedeemSchema = z.object({
  amountCents: positiveCentsSchema,
  date: dateStringSchema.optional(),
  toAccountId: idSchema.optional(),
});
export type InvestmentRedeemInput = z.infer<typeof investmentRedeemSchema>;

export const investmentSummarySchema = z.object({
  totalInvestedCents: centsSchema,
  totalCurrentCents: centsSchema,
  returnCents: centsSchema,
  returnPct: z.number().nullable(),
  byKind: z.array(
    z.object({ kind: investmentKindSchema, currentCents: centsSchema, percent: z.number() }),
  ),
  byInstitution: z.array(
    z.object({ institution: z.string(), currentCents: centsSchema, percent: z.number() }),
  ),
  /** 12 meses a partir dos snapshots. */
  history: z.array(
    z.object({
      month: z.number().int(),
      year: z.number().int(),
      currentCents: centsSchema,
      investedCents: centsSchema,
    }),
  ),
});
export type InvestmentSummary = z.infer<typeof investmentSummarySchema>;
