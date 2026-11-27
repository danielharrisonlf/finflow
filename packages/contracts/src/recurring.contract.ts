import { z } from "zod";
import { centsSchema, dateStringSchema, idSchema, positiveCentsSchema } from "./common";

export const recurringKindSchema = z.enum(["INCOME", "EXPENSE"]);
export type RecurringKind = z.infer<typeof recurringKindSchema>;

/** Conta fixa ou receita recorrente (Compromissos). */
export const createRecurringSchema = z.object({
  description: z.string().min(1).max(120),
  kind: recurringKindSchema,
  amountCents: positiveCentsSchema,
  /** 1-31; em meses curtos a ocorrência cai no último dia. */
  dayOfMonth: z.number().int().min(1).max(31),
  accountId: idSchema.nullish(),
  categoryId: idSchema.nullish(),
  startDate: dateStringSchema.optional(),
  endDate: dateStringSchema.nullish(),
  /** Assinatura (v3 §2): conta fixa cortável vinda da detecção. */
  isSubscription: z.boolean().optional(),
});
export type CreateRecurringInput = z.infer<typeof createRecurringSchema>;

export const updateRecurringSchema = createRecurringSchema.partial().extend({
  active: z.boolean().optional(),
});
export type UpdateRecurringInput = z.infer<typeof updateRecurringSchema>;

/** Lança a ocorrência do mês de `date` como transação confirmada. */
export const postRecurringSchema = z.object({
  date: dateStringSchema.optional(),
  amountCents: positiveCentsSchema.optional(),
  accountId: idSchema.optional(),
});
export type PostRecurringInput = z.infer<typeof postRecurringSchema>;

export const recurringSchema = z.object({
  id: idSchema,
  description: z.string(),
  kind: recurringKindSchema,
  amountCents: centsSchema,
  dayOfMonth: z.number().int(),
  accountId: idSchema.nullable(),
  accountName: z.string().nullable(),
  categoryId: idSchema.nullable(),
  categoryName: z.string().nullable(),
  categoryIcon: z.string().nullable(),
  categoryColor: z.string().nullable(),
  startDate: dateStringSchema,
  endDate: dateStringSchema.nullable(),
  active: z.boolean(),
  /** Próxima ocorrência ainda não lançada (>= hoje ou atrasada do mês corrente). */
  nextDate: dateStringSchema.nullable(),
  /** Opcional para tolerar backend sem a coluna nova (default false). */
  isSubscription: z.boolean().optional(),
});
export type Recurring = z.infer<typeof recurringSchema>;
