import { z } from "zod";
import { centsSchema, dateStringSchema, idSchema, positiveCentsSchema } from "./common";

export const wishStatusSchema = z.enum(["LISTED", "BOUGHT", "ARCHIVED"]);
export type WishStatus = z.infer<typeof wishStatusSchema>;

export const createWishItemSchema = z.object({
  name: z.string().min(1).max(120),
  priceCents: positiveCentsSchema,
  url: z.string().max(500).nullish(),
  notes: z.string().max(500).nullish(),
  /** Menor = mais prioritário. Omitido → fim da lista. */
  priority: z.number().int().optional(),
});
export type CreateWishItemInput = z.infer<typeof createWishItemSchema>;

export const updateWishItemSchema = createWishItemSchema.partial().extend({
  status: wishStatusSchema.optional(),
});
export type UpdateWishItemInput = z.infer<typeof updateWishItemSchema>;

export const wishItemSchema = z.object({
  id: idSchema,
  name: z.string(),
  priceCents: centsSchema,
  url: z.string().nullable(),
  notes: z.string().nullable(),
  priority: z.number().int(),
  status: wishStatusSchema,
  /**
   * 1ª data em que o saldo projetado comporta este desejo (somado aos LISTED de maior
   * prioridade) e permanece >= 0 até o fim do horizonte de 365 dias; null se não couber.
   */
  affordableOn: dateStringSchema.nullable(),
});
export type WishItem = z.infer<typeof wishItemSchema>;
