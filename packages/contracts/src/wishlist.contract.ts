import { z } from "zod";
import { centsSchema, dateStringSchema, idSchema, positiveCentsSchema } from "./common";

export const wishStatusSchema = z.enum(["LISTED", "BOUGHT", "ARCHIVED"]);
export type WishStatus = z.infer<typeof wishStatusSchema>;

export const createWishItemSchema = z.object({
  name: z.string().min(1).max(120),
  priceCents: positiveCentsSchema,
  url: z.string().max(500).nullish(),
  /** Imagem do produto (prévia do link, v3 §9). */
  imageUrl: z.string().max(1000).nullish(),
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
  /** Opcional para tolerar backend sem a coluna nova. */
  imageUrl: z.string().nullish(),
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

/** POST /api/wishlist/preview-link: lê og:title / preço / imagem do produto. */
export const previewLinkSchema = z.object({
  url: z.string().url().max(1000),
});
export type PreviewLinkInput = z.infer<typeof previewLinkSchema>;

export const linkPreviewSchema = z.object({
  title: z.string().nullable(),
  priceCents: centsSchema.nullable(),
  imageUrl: z.string().nullable(),
});
export type LinkPreview = z.infer<typeof linkPreviewSchema>;
