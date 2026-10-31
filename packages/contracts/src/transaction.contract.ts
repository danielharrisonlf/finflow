import { z } from "zod";
import {
  centsSchema,
  dateStringSchema,
  idSchema,
  monthSchema,
  paginationQuerySchema,
  positiveCentsSchema,
  yearSchema,
} from "./common";

export const transactionTypeSchema = z.enum([
  "INCOME",
  "EXPENSE",
  "TRANSFER",
  "CARD_EXPENSE",
  "INVOICE_PAYMENT",
]);
export type TransactionType = z.infer<typeof transactionTypeSchema>;

export const transactionStatusSchema = z.enum(["PENDING", "CONFIRMED"]);
export type TransactionStatus = z.infer<typeof transactionStatusSchema>;

/** Receita ou despesa lançada diretamente em uma conta. */
export const createTransactionSchema = z.object({
  type: z.enum(["INCOME", "EXPENSE"]),
  description: z.string().min(1).max(120),
  amountCents: positiveCentsSchema,
  date: dateStringSchema,
  status: transactionStatusSchema.default("CONFIRMED"),
  accountId: idSchema,
  categoryId: idSchema.nullish(),
  notes: z.string().max(500).nullish(),
});
export type CreateTransactionInput = z.infer<typeof createTransactionSchema>;

export const updateTransactionSchema = z.object({
  description: z.string().min(1).max(120).optional(),
  amountCents: positiveCentsSchema.optional(),
  date: dateStringSchema.optional(),
  status: transactionStatusSchema.optional(),
  accountId: idSchema.optional(),
  categoryId: idSchema.nullish(),
  notes: z.string().max(500).nullish(),
});
export type UpdateTransactionInput = z.infer<typeof updateTransactionSchema>;

/** Compra no cartão de crédito, à vista ou parcelada em N vezes. */
export const createCardExpenseSchema = z.object({
  cardId: idSchema,
  description: z.string().min(1).max(120),
  /** Valor TOTAL da compra; o serviço divide em parcelas. */
  totalCents: positiveCentsSchema,
  date: dateStringSchema,
  categoryId: idSchema.nullish(),
  installments: z.number().int().min(1).max(48).default(1),
  notes: z.string().max(500).nullish(),
});
export type CreateCardExpenseInput = z.infer<typeof createCardExpenseSchema>;

export const listTransactionsQuerySchema = paginationQuerySchema.extend({
  month: monthSchema.optional(),
  year: yearSchema.optional(),
  accountId: idSchema.optional(),
  cardId: idSchema.optional(),
  categoryId: idSchema.optional(),
  type: transactionTypeSchema.optional(),
  status: transactionStatusSchema.optional(),
  search: z.string().max(120).optional(),
});
export type ListTransactionsQuery = z.infer<typeof listTransactionsQuerySchema>;

export const exportFormatSchema = z.enum(["csv", "ofx"]);
export type ExportFormat = z.infer<typeof exportFormatSchema>;

export const exportTransactionsQuerySchema = listTransactionsQuerySchema
  .omit({ page: true, pageSize: true })
  .extend({ format: exportFormatSchema.default("csv") });
export type ExportTransactionsQuery = z.infer<typeof exportTransactionsQuerySchema>;

export const transactionSchema = z.object({
  id: idSchema,
  type: transactionTypeSchema,
  description: z.string(),
  amountCents: centsSchema,
  date: dateStringSchema,
  status: transactionStatusSchema,
  accountId: idSchema.nullable(),
  accountName: z.string().nullable(),
  cardId: idSchema.nullable(),
  cardName: z.string().nullable(),
  categoryId: idSchema.nullable(),
  categoryName: z.string().nullable(),
  categoryIcon: z.string().nullable(),
  categoryColor: z.string().nullable(),
  transferGroupId: idSchema.nullable(),
  installmentNumber: z.number().int().nullable(),
  installmentTotal: z.number().int().nullable(),
  invoiceId: idSchema.nullable(),
  notes: z.string().nullable(),
});
export type Transaction = z.infer<typeof transactionSchema>;
