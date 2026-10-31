import { z } from "zod";
import { centsSchema, idSchema, monthSchema, positiveCentsSchema, yearSchema } from "./common";

export const upsertBudgetSchema = z.object({
  categoryId: idSchema,
  /** Teto mensal recorrente para a categoria. */
  amountCents: positiveCentsSchema,
});
export type UpsertBudgetInput = z.infer<typeof upsertBudgetSchema>;

export const budgetStatusSchema = z.enum(["OK", "WARNING", "DANGER"]);
export type BudgetStatus = z.infer<typeof budgetStatusSchema>;

export const budgetSchema = z.object({
  id: idSchema,
  categoryId: idSchema,
  categoryName: z.string(),
  categoryIcon: z.string(),
  categoryColor: z.string(),
  amountCents: centsSchema,
  spentCents: centsSchema,
  /** 0-100+, pode passar de 100 quando estourado. */
  percent: z.number(),
  /** OK < 70%, WARNING 70-90%, DANGER > 90%. */
  status: budgetStatusSchema,
});
export type Budget = z.infer<typeof budgetSchema>;

export const listBudgetsQuerySchema = z.object({
  month: monthSchema.optional(),
  year: yearSchema.optional(),
});
export type ListBudgetsQuery = z.infer<typeof listBudgetsQuerySchema>;
