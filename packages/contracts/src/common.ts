import { z } from "zod";

/** Valores monetários sempre em centavos inteiros (BRL). */
export const centsSchema = z.number().int();
export const positiveCentsSchema = z.number().int().positive();

/** Datas trafegam como string ISO `YYYY-MM-DD` (sem fuso). */
export const dateStringSchema = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}$/, "data deve estar no formato YYYY-MM-DD");

export const idSchema = z.uuid();

export const monthSchema = z.coerce.number().int().min(1).max(12);
export const yearSchema = z.coerce.number().int().min(2000).max(2100);

export const hexColorSchema = z.string().regex(/^#[0-9a-fA-F]{6}$/, "cor deve ser hex #RRGGBB");

export const paginationQuerySchema = z.object({
  page: z.coerce.number().int().min(1).default(1),
  pageSize: z.coerce.number().int().min(1).max(100).default(50),
});

export type PaginationQuery = z.infer<typeof paginationQuerySchema>;

export interface Paginated<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}
