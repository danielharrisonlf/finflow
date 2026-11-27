import { z } from "zod";
import { centsSchema, dateStringSchema, idSchema, positiveCentsSchema } from "./common";

/** Empréstimos e financiamentos (v3 §11). */
export const loanKindSchema = z.enum([
  "PERSONAL",
  "PAYROLL",
  "VEHICLE",
  "MORTGAGE",
  "STUDENT",
  "CARD_DEBT",
  "OVERDRAFT",
  "OTHER",
]);
export type LoanKind = z.infer<typeof loanKindSchema>;

export const createLoanSchema = z.object({
  name: z.string().trim().min(1).max(80),
  lender: z.string().max(80).nullish(),
  kind: loanKindSchema,
  principalCents: positiveCentsSchema,
  /** Taxa de juros em % ao mês (ex.: 1.99). */
  monthlyRatePct: z.number().min(0).max(100),
  installmentCents: positiveCentsSchema,
  totalInstallments: z.number().int().min(1).max(600),
  paidInstallments: z.number().int().min(0).default(0),
  firstDueDate: dateStringSchema,
  accountId: idSchema.nullish(),
  /** Vazio → categoria padrão "Empréstimos e financiamentos". */
  categoryId: idSchema.nullish(),
  notes: z.string().max(500).nullish(),
});
export type CreateLoanInput = z.input<typeof createLoanSchema>;

export const updateLoanSchema = createLoanSchema.partial().extend({
  active: z.boolean().optional(),
});
export type UpdateLoanInput = z.input<typeof updateLoanSchema>;

export const loanSchema = z.object({
  id: idSchema,
  name: z.string(),
  lender: z.string().nullable(),
  kind: loanKindSchema,
  principalCents: centsSchema,
  monthlyRatePct: z.number(),
  installmentCents: centsSchema,
  totalInstallments: z.number().int(),
  paidInstallments: z.number().int(),
  remainingInstallments: z.number().int(),
  firstDueDate: dateStringSchema,
  nextDueDate: dateStringSchema.nullable(),
  /** Valor presente das parcelas restantes (Price) com a taxa; sem taxa = soma. */
  outstandingCents: centsSchema,
  /** Parcelas restantes − outstanding. */
  totalInterestRemainingCents: centsSchema,
  accountId: idSchema.nullable(),
  categoryId: idSchema.nullable(),
  active: z.boolean(),
  notes: z.string().nullable(),
});
export type Loan = z.infer<typeof loanSchema>;

/** POST /api/loans/{id}/pay → Transaction (EXPENSE com loan_id; paid += 1). */
export const payLoanSchema = z.object({
  date: dateStringSchema.optional(),
  amountCents: positiveCentsSchema.optional(),
  accountId: idSchema.optional(),
});
export type PayLoanInput = z.infer<typeof payLoanSchema>;

export const debtPlanSchema = z.object({
  /** Ordem de quitação (ids). */
  order: z.array(idSchema),
  months: z.number().int(),
  totalInterestCents: centsSchema,
  payoffDate: dateStringSchema,
  perLoan: z.array(
    z.object({ loanId: idSchema, payoffDate: dateStringSchema, interestCents: centsSchema }),
  ),
});
export type DebtPlan = z.infer<typeof debtPlanSchema>;

/** GET /api/loans/strategy?extraCents=0. */
export const debtStrategySchema = z.object({
  avalanche: debtPlanSchema,
  snowball: debtPlanSchema,
  minimumOnly: debtPlanSchema,
});
export type DebtStrategy = z.infer<typeof debtStrategySchema>;
