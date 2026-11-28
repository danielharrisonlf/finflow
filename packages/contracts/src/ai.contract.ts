import { z } from "zod";

/**
 * GET /api/ai/status (v3 §15). O Assistente usa só a API do Claude com a chave do próprio usuário
 * (ANTHROPIC_API_KEY ou a chave salva em Configurações); sem chave, o recurso fica desativado.
 * Os campos além de `configured` são tolerantes (servidores anteriores devolviam `model` sempre
 * preenchido e não tinham `receiptSupported`). A chave nunca vem na resposta.
 */
export const aiStatusSchema = z.object({
  configured: z.boolean(),
  source: z.enum(["ENV", "SETTINGS"]).nullish(),
  model: z.string().nullish(),
  receiptSupported: z.boolean().nullish(),
});
export type AiStatus = z.infer<typeof aiStatusSchema>;

/** PUT /api/ai/key. `null` ou "" remove a chave salva. */
export const aiKeyInputSchema = z.object({
  apiKey: z.string().max(512).nullable(),
});
export type AiKeyInput = z.infer<typeof aiKeyInputSchema>;
