import { z } from "zod";
import { centsSchema, dateStringSchema, idSchema, monthSchema, yearSchema } from "./common";

export const dashboardQuerySchema = z.object({
  month: monthSchema.optional(),
  year: yearSchema.optional(),
});
export type DashboardQuery = z.infer<typeof dashboardQuerySchema>;

export const dashboardSummarySchema = z.object({
  month: z.number().int(),
  year: z.number().int(),
  totalBalanceCents: centsSchema,
  monthIncomeCents: centsSchema,
  monthExpenseCents: centsSchema,
  monthNetCents: centsSchema,
  openInvoicesCents: centsSchema,
  pendingCount: z.number().int(),
});
export type DashboardSummary = z.infer<typeof dashboardSummarySchema>;

export const cashflowPointSchema = z.object({
  date: dateStringSchema,
  incomeCents: centsSchema,
  expenseCents: centsSchema,
});
export type CashflowPoint = z.infer<typeof cashflowPointSchema>;

export const categorySliceSchema = z.object({
  categoryId: idSchema.nullable(),
  name: z.string(),
  icon: z.string().nullable(),
  color: z.string().nullable(),
  totalCents: centsSchema,
  percent: z.number(),
});
export type CategorySlice = z.infer<typeof categorySliceSchema>;

export const upcomingItemSchema = z.object({
  id: z.string(),
  kind: z.enum(["TRANSACTION", "INVOICE"]),
  description: z.string(),
  dueDate: dateStringSchema,
  amountCents: centsSchema,
  categoryIcon: z.string().nullable(),
});
export type UpcomingItem = z.infer<typeof upcomingItemSchema>;
