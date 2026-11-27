import { z } from "zod";
import { hexColorSchema, idSchema } from "./common";

export const categoryKindSchema = z.enum(["INCOME", "EXPENSE"]);
export type CategoryKind = z.infer<typeof categoryKindSchema>;

/**
 * Tipo de gasto (só despesas): Fixo / Cortável / Variável.
 * FIXED + CUTTABLE compõem as "Saídas" (custo de vida); VARIABLE é o "Diário".
 */
export const spendTypeSchema = z.enum(["FIXED", "CUTTABLE", "VARIABLE"]);
export type SpendType = z.infer<typeof spendTypeSchema>;

export const createCategorySchema = z.object({
  name: z.string().min(1).max(40),
  kind: categoryKindSchema,
  /** Nome de ícone Lucide em kebab-case, ex.: "utensils", "house", "car". */
  icon: z.string().min(1).max(40),
  color: hexColorSchema,
  parentId: idSchema.nullish(),
  /** Só em EXPENSE (default VARIABLE); null em subcategoria = herda do pai. */
  spendType: spendTypeSchema.nullish(),
});
export type CreateCategoryInput = z.infer<typeof createCategorySchema>;

export const updateCategorySchema = createCategorySchema.partial();
export type UpdateCategoryInput = z.infer<typeof updateCategorySchema>;

export const categorySchema = z.object({
  id: idSchema,
  name: z.string(),
  kind: categoryKindSchema,
  icon: z.string(),
  color: z.string(),
  parentId: idSchema.nullable(),
  spendType: spendTypeSchema.nullable(),
});
export type Category = z.infer<typeof categorySchema>;
