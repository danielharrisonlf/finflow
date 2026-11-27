import { z } from "zod";
import { idSchema } from "./common";

/** Regra de categorização automática (Automações). */
export const createCategoryRuleSchema = z.object({
  /** Casa por substring, sem diferenciar maiúsculas nem acentos. */
  pattern: z.string().min(1).max(120),
  categoryId: idSchema,
  /** Maior prioridade vence; empate → padrão mais longo. */
  priority: z.number().int().optional(),
  /** Tag aplicada junto com a categoria (v3 §1). */
  tagId: idSchema.nullish(),
});
export type CreateCategoryRuleInput = z.infer<typeof createCategoryRuleSchema>;

export const updateCategoryRuleSchema = createCategoryRuleSchema.partial();
export type UpdateCategoryRuleInput = z.infer<typeof updateCategoryRuleSchema>;

export const categoryRuleSchema = z.object({
  id: idSchema,
  pattern: z.string(),
  categoryId: idSchema,
  categoryName: z.string(),
  priority: z.number().int(),
  tagId: idSchema.nullish(),
  tagName: z.string().nullish(),
});
export type CategoryRule = z.infer<typeof categoryRuleSchema>;

export const applyRulesSchema = z.object({
  onlyUncategorized: z.boolean().default(true),
});
export type ApplyRulesInput = z.input<typeof applyRulesSchema>;

export const applyRulesResultSchema = z.object({ updated: z.number().int() });
export type ApplyRulesResult = z.infer<typeof applyRulesResultSchema>;
