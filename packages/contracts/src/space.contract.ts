import { z } from "zod";
import { centsSchema, hexColorSchema } from "./common";

/** Espaços (v3 fase 2 §1–§3): cada um é um banco separado; `pessoal` é o padrão. */
export const spaceKindSchema = z.enum(["PERSONAL", "HOME", "BUSINESS"]);
export type SpaceKind = z.infer<typeof spaceKindSchema>;

export const spaceSlugSchema = z.string().regex(/^[a-z0-9][a-z0-9-]{1,30}$/, "slug inválido");

export const DEFAULT_SPACE_SLUG = "pessoal";

export const createSpaceSchema = z.object({
  name: z.string().trim().min(1).max(60),
  kind: spaceKindSchema,
  color: hexColorSchema.optional(),
  /** Derivado do nome quando ausente. */
  slug: spaceSlugSchema.optional(),
});
export type CreateSpaceInput = z.infer<typeof createSpaceSchema>;

export const updateSpaceSchema = z.object({
  name: z.string().trim().min(1).max(60).optional(),
  color: hexColorSchema.optional(),
});
export type UpdateSpaceInput = z.infer<typeof updateSpaceSchema>;

export const spaceSchema = z.object({
  slug: z.string(),
  name: z.string(),
  kind: spaceKindSchema,
  color: z.string(),
  createdAt: z.string().optional(),
  isDefault: z.boolean().optional(),
});
export type Space = z.infer<typeof spaceSchema>;

const spaceTotalsSchema = z.object({
  balanceCents: centsSchema,
  netWorthCents: centsSchema,
  monthIncomeCents: centsSchema,
  monthOutCents: centsSchema,
  monthResultCents: centsSchema,
});
export type SpaceTotals = z.infer<typeof spaceTotalsSchema>;

/** GET /api/spaces/overview: visão unificada. */
export const spacesOverviewSchema = z.object({
  spaces: z.array(
    spaceTotalsSchema.extend({ slug: z.string(), name: z.string(), kind: spaceKindSchema }),
  ),
  totals: spaceTotalsSchema,
});
export type SpacesOverview = z.infer<typeof spacesOverviewSchema>;
export type SpaceOverviewItem = SpacesOverview["spaces"][number];
