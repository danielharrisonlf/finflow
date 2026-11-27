import { z } from "zod";
import { centsSchema, hexColorSchema, idSchema, monthSchema, yearSchema } from "./common";

/** Etiqueta livre aplicada a transações (v3 §1). */
export const createTagSchema = z.object({
  name: z.string().trim().min(1).max(40),
  color: hexColorSchema.optional(),
});
export type CreateTagInput = z.infer<typeof createTagSchema>;

export const updateTagSchema = createTagSchema.partial();
export type UpdateTagInput = z.infer<typeof updateTagSchema>;

export const tagSchema = z.object({
  id: idSchema,
  name: z.string(),
  color: z.string(),
  usageCount: z.number().int().optional(),
});
export type Tag = z.infer<typeof tagSchema>;

/** Forma resumida embutida em `Transaction.tags`. */
export const transactionTagSchema = z.object({
  id: idSchema,
  name: z.string(),
  color: z.string(),
});
export type TransactionTag = z.infer<typeof transactionTagSchema>;

export const tagSpendingQuerySchema = z.object({
  month: monthSchema.optional(),
  year: yearSchema.optional(),
});
export type TagSpendingQuery = z.infer<typeof tagSpendingQuerySchema>;

/** GET /api/insights/tags: gasto (EXPENSE + CARD_EXPENSE) por tag no mês. */
export const tagSpendingSchema = z.object({
  tagId: idSchema,
  name: z.string(),
  color: z.string(),
  totalCents: centsSchema,
  count: z.number().int(),
});
export type TagSpending = z.infer<typeof tagSpendingSchema>;
