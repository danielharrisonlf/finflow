import { z } from "zod";
import { dateStringSchema } from "./common";

/** Feed de alertas e dicas (v3 §3). */
export const alertKindSchema = z.enum([
  "BUDGET_WARNING",
  "BUDGET_DANGER",
  "LIMIT_PACE",
  "PACE_ABOVE",
  "NEGATIVE_PROJECTION",
  "BILL_DUE",
  "INVOICE_DUE",
  "LOAN_DUE",
  "SUBSCRIPTION_PRICE_UP",
  "NEW_SUBSCRIPTION",
  "CATEGORY_ABOVE_AVERAGE",
  "RESERVE_LOW",
  "GOAL_BEHIND",
  "PENDING_OLD",
  "UNCATEGORIZED",
  "INVESTMENT_STALE",
]);
export type AlertKind = z.infer<typeof alertKindSchema>;

export const alertSeveritySchema = z.enum(["INFO", "WARNING", "DANGER"]);
export type AlertSeverity = z.infer<typeof alertSeveritySchema>;

export const alertSchema = z.object({
  /** Determinístico: "<kind>:<ref>:<YYYY-MM>" (dispensar vale até mudar). */
  id: z.string(),
  kind: alertKindSchema,
  severity: alertSeveritySchema,
  title: z.string(),
  message: z.string(),
  /** Rota do app para agir. */
  href: z.string(),
  date: dateStringSchema,
});
export type Alert = z.infer<typeof alertSchema>;
