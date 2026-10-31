import { z } from "zod";
import {
  centsSchema,
  dateStringSchema,
  hexColorSchema,
  idSchema,
  positiveCentsSchema,
} from "./common";

export const cardBrandSchema = z.enum(["VISA", "MASTERCARD", "ELO", "AMEX", "HIPERCARD", "OTHER"]);
export type CardBrand = z.infer<typeof cardBrandSchema>;

export const invoiceStatusSchema = z.enum(["OPEN", "CLOSED", "PAID"]);
export type InvoiceStatus = z.infer<typeof invoiceStatusSchema>;

export const createCardSchema = z.object({
  name: z.string().min(1).max(60),
  brand: cardBrandSchema,
  limitCents: positiveCentsSchema,
  /** Dia do fechamento da fatura (1-28 para existir em todo mês). */
  closingDay: z.number().int().min(1).max(28),
  /** Dia do vencimento da fatura (1-28). */
  dueDay: z.number().int().min(1).max(28),
  color: hexColorSchema.optional(),
});
export type CreateCardInput = z.infer<typeof createCardSchema>;

export const updateCardSchema = createCardSchema.partial().extend({
  archived: z.boolean().optional(),
});
export type UpdateCardInput = z.infer<typeof updateCardSchema>;

export const invoiceSchema = z.object({
  id: idSchema,
  cardId: idSchema,
  month: z.number().int().min(1).max(12),
  year: z.number().int(),
  status: invoiceStatusSchema,
  totalCents: centsSchema,
  closingDate: dateStringSchema,
  dueDate: dateStringSchema,
  paidAt: dateStringSchema.nullable(),
});
export type Invoice = z.infer<typeof invoiceSchema>;

export const cardSchema = z.object({
  id: idSchema,
  name: z.string(),
  brand: cardBrandSchema,
  limitCents: centsSchema,
  closingDay: z.number().int(),
  dueDay: z.number().int(),
  color: z.string().nullable(),
  archived: z.boolean(),
  /** Soma das faturas não pagas (compromete o limite). */
  usedLimitCents: centsSchema,
  availableLimitCents: centsSchema,
  currentInvoice: invoiceSchema.nullable(),
});
export type Card = z.infer<typeof cardSchema>;

export const payInvoiceSchema = z.object({
  accountId: idSchema,
  date: dateStringSchema.optional(),
});
export type PayInvoiceInput = z.infer<typeof payInvoiceSchema>;
