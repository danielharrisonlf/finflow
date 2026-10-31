import { z } from "zod";
import { hexColorSchema, idSchema } from "./common";

export const categoryKindSchema = z.enum(["INCOME", "EXPENSE"]);
export type CategoryKind = z.infer<typeof categoryKindSchema>;

export const createCategorySchema = z.object({
  name: z.string().min(1).max(40),
  kind: categoryKindSchema,
  /** Nome de ícone Lucide em kebab-case, ex.: "utensils", "house", "car". */
  icon: z.string().min(1).max(40),
  color: hexColorSchema,
  parentId: idSchema.nullish(),
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
});
export type Category = z.infer<typeof categorySchema>;
