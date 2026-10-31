import { z } from "zod";
import {
  centsSchema,
  dateStringSchema,
  hexColorSchema,
  idSchema,
  positiveCentsSchema,
} from "./common";

export const createGoalSchema = z.object({
  name: z.string().min(1).max(60),
  icon: z.string().min(1).max(40).default("piggy-bank"),
  color: hexColorSchema.optional(),
  targetCents: positiveCentsSchema,
  deadline: dateStringSchema.nullish(),
});
export type CreateGoalInput = z.infer<typeof createGoalSchema>;

export const updateGoalSchema = createGoalSchema.partial();
export type UpdateGoalInput = z.infer<typeof updateGoalSchema>;

export const contributeGoalSchema = z.object({
  /** Positivo = aporte; negativo = resgate. */
  amountCents: centsSchema.refine((v) => v !== 0, "valor não pode ser zero"),
  date: dateStringSchema.optional(),
  note: z.string().max(120).optional(),
});
export type ContributeGoalInput = z.infer<typeof contributeGoalSchema>;

export const goalSchema = z.object({
  id: idSchema,
  name: z.string(),
  icon: z.string(),
  color: z.string().nullable(),
  targetCents: centsSchema,
  savedCents: centsSchema,
  deadline: dateStringSchema.nullable(),
  percent: z.number(),
  /** Aporte mensal necessário para bater a meta até o prazo (null sem prazo ou concluída). */
  monthlyNeededCents: centsSchema.nullable(),
});
export type Goal = z.infer<typeof goalSchema>;
