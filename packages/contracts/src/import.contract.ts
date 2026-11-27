import { z } from "zod";
import { centsSchema, dateStringSchema, idSchema, positiveCentsSchema } from "./common";
import { transactionStatusSchema } from "./transaction.contract";

export const importFormatSchema = z.enum(["DAILY_SHEET", "TABLE", "CSV", "OFX"]);
export type ImportFormat = z.infer<typeof importFormatSchema>;

const columnIndex = z.number().int().min(0).nullable();

/** Índice da coluna de cada campo (TABLE/CSV); null = ausente. */
export const importMappingSchema = z.object({
  date: columnIndex,
  description: columnIndex,
  amount: columnIndex,
  debit: columnIndex,
  credit: columnIndex,
  type: columnIndex,
  category: columnIndex,
  account: columnIndex,
});
export type ImportMapping = z.infer<typeof importMappingSchema>;

export const importKindSchema = z.enum(["INCOME", "EXPENSE"]);
export type ImportKind = z.infer<typeof importKindSchema>;

export const importPreviewRowSchema = z.object({
  index: z.number().int(),
  date: z.string(),
  description: z.string(),
  amountCents: centsSchema,
  kind: importKindSchema,
  categoryName: z.string().nullable(),
  suggestedCategoryId: idSchema.nullable(),
  accountName: z.string().nullable(),
  duplicate: z.boolean(),
  error: z.string().nullable(),
});
export type ImportPreviewRow = z.infer<typeof importPreviewRowSchema>;

/** Resposta de POST /api/import/preview (multipart: file, mapping?, sheet?). */
export const importPreviewSchema = z.object({
  format: importFormatSchema,
  sheets: z.array(z.string()),
  sheet: z.string().nullable(),
  columns: z.array(z.string()),
  mapping: importMappingSchema,
  rows: z.array(importPreviewRowSchema),
  warnings: z.array(z.string()),
});
export type ImportPreview = z.infer<typeof importPreviewSchema>;

export const importCommitRowSchema = z.object({
  date: dateStringSchema,
  description: z.string().min(1).max(120),
  amountCents: positiveCentsSchema,
  kind: importKindSchema,
  categoryId: idSchema.nullish(),
  status: transactionStatusSchema.optional(),
});
export type ImportCommitRow = z.infer<typeof importCommitRowSchema>;

export const importCommitSchema = z.object({
  accountId: idSchema,
  skipDuplicates: z.boolean().default(true),
  rows: z.array(importCommitRowSchema),
});
export type ImportCommitInput = z.input<typeof importCommitSchema>;

export const importCommitResultSchema = z.object({
  created: z.number().int(),
  skipped: z.number().int(),
});
export type ImportCommitResult = z.infer<typeof importCommitResultSchema>;
