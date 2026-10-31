import { z } from "zod";

export const telegramStatusSchema = z.object({
  /** Token configurado no servidor (sem expor o valor). */
  configured: z.boolean(),
  /** Chat padrão vinculado para notificações. */
  chatLinked: z.boolean(),
});
export type TelegramStatus = z.infer<typeof telegramStatusSchema>;

/**
 * Subconjunto do Update da Bot API do Telegram que o webhook consome.
 * Campos desconhecidos são ignorados (payload real tem dezenas de campos).
 */
export const telegramUpdateSchema = z.looseObject({
  update_id: z.number(),
  message: z
    .looseObject({
      message_id: z.number(),
      text: z.string().optional(),
      chat: z.looseObject({ id: z.number() }),
    })
    .optional(),
});
export type TelegramUpdate = z.infer<typeof telegramUpdateSchema>;
