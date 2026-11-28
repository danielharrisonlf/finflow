import { z } from "zod";

/** Provedores do Assistente (docs/local-agent-contract.md §1). */
export const aiProviderSchema = z.enum(["ollama", "openai_compatible", "claude"]);
export type AiProvider = z.infer<typeof aiProviderSchema>;

/**
 * GET /api/ai/status (§2). Os campos novos são opcionais: servidores anteriores ao assistente local
 * devolvem só `configured`, `source` e `model`. Chaves de API nunca vêm na resposta.
 */
export const aiStatusSchema = z.object({
  configured: z.boolean(),
  provider: aiProviderSchema.nullish(),
  model: z.string().nullish(),
  visionModel: z.string().nullish(),
  baseUrl: z.string().nullish(),
  /** AUTO: nada no ambiente nem salvo, e um Ollama com modelos respondeu em FINFLOW_OLLAMA_URL. */
  source: z.enum(["ENV", "SETTINGS", "AUTO"]).nullish(),
  /** Ping do servidor local; com nada configurado, do Ollama em http://localhost:11434. */
  reachable: z.boolean().nullish(),
  receiptSupported: z.boolean().nullish(),
});
export type AiStatus = z.infer<typeof aiStatusSchema>;

/** PUT /api/ai/config. `apiKey` omitido mantém a chave salva; `null` ou "" apaga. */
export const aiConfigInputSchema = z.object({
  provider: aiProviderSchema,
  baseUrl: z.string().max(512).nullish(),
  model: z.string().max(200).nullish(),
  visionModel: z.string().max(200).nullish(),
  apiKey: z.string().max(512).nullish(),
});
export type AiConfigInput = z.infer<typeof aiConfigInputSchema>;

/** Item de GET /api/ai/models; `tools` é null quando o servidor não informa capacidades. */
export const aiModelSchema = z.object({
  id: z.string(),
  label: z.string(),
  tools: z.boolean().nullish(),
});
export type AiModel = z.infer<typeof aiModelSchema>;

export const aiModelsResponseSchema = z.object({ models: z.array(aiModelSchema) });
export type AiModelsResponse = z.infer<typeof aiModelsResponseSchema>;
