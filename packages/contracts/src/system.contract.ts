import { z } from "zod";

/** GET /api/system/info (v3 §13): base da página "API e IA" em Configurações. */
export const systemInfoSchema = z.object({
  version: z.string(),
  dbDialect: z.enum(["sqlite", "postgres"]),
  dataPath: z.string().nullable(),
  authRequired: z.boolean(),
  lanEnabled: z.boolean(),
  lanUrls: z.array(z.string()),
  aiConfigured: z.boolean(),
  mcpHttpPath: z.string(),
});
export type SystemInfo = z.infer<typeof systemInfoSchema>;
