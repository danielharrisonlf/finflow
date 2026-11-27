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
import { transactionSplitSchema } from "./member.contract";
import { transactionTagSchema } from "./tag.contract";

/** Conjunto de tags de um lançamento (substitui o atual quando enviado). */
const tagIdsSchema = z.array(idSchema).max(20);

/** Divisão entre membros (espaço Casa, v3 fase 2 §4): soma = 100; vazia = igual entre todos. */
const splitsSchema = z.array(transactionSplitSchema).max(50);

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
  tagIds: tagIdsSchema.optional(),
  paidByMemberId: idSchema.nullish(),
  splits: splitsSchema.optional(),
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
  tagIds: tagIdsSchema.optional(),
  paidByMemberId: idSchema.nullish(),
  splits: splitsSchema.optional(),
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
  /** Aplicadas a todas as parcelas. */
  tagIds: tagIdsSchema.optional(),
  paidByMemberId: idSchema.nullish(),
  splits: splitsSchema.optional(),
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
  tagId: idSchema.optional(),
});
export type ListTransactionsQuery = z.infer<typeof listTransactionsQuerySchema>;

export const exportFormatSchema = z.enum(["csv", "ofx", "xlsx"]);
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
  /** Recorrência (conta fixa / receita recorrente) que originou o lançamento. */
  recurringId: idSchema.nullable(),
  /** Empréstimo (v3 §11) cuja parcela originou o lançamento. */
  loanId: idSchema.nullish(),
  notes: z.string().nullable(),
  /** Sempre presente no v3; opcional para tolerar backend antigo. */
  tags: z.array(transactionTagSchema).optional(),
  /** Espaço Casa: quem pagou e a divisão (opcionais para tolerar outros espaços). */
  paidByMemberId: idSchema.nullish(),
  splits: z.array(transactionSplitSchema).optional(),
});
export type Transaction = z.infer<typeof transactionSchema>;

// ───────────────────────── transferências inteligentes (v3 §7) ─────────────────────────

export const transferCandidateSchema = z.object({
  expense: transactionSchema,
  income: transactionSchema,
  daysApart: z.number().int(),
});
export type TransferCandidate = z.infer<typeof transferCandidateSchema>;

export const mergeTransferSchema = z.object({
  expenseId: idSchema,
  incomeId: idSchema,
});
export type MergeTransferInput = z.infer<typeof mergeTransferSchema>;
