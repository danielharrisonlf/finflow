import { Injectable, Logger } from "@nestjs/common";

/**
 * Cliente fino da Bot API do Telegram. Sem TELEGRAM_BOT_TOKEN o serviço fica
 * inativo e todo envio vira no-op — nenhuma funcionalidade do app depende dele.
 */
@Injectable()
export class TelegramApiService {
  private readonly logger = new Logger(TelegramApiService.name);

  get isConfigured(): boolean {
    return Boolean(process.env.TELEGRAM_BOT_TOKEN);
  }

  get defaultChatId(): string | null {
    return process.env.TELEGRAM_CHAT_ID || null;
  }

  private get baseUrl(): string {
    const override = process.env.TELEGRAM_API_BASE;
    const token = process.env.TELEGRAM_BOT_TOKEN;
    return `${override ?? "https://api.telegram.org"}/bot${token}`;
  }

  async sendMessage(text: string, chatId?: string | number): Promise<boolean> {
    const target = chatId ?? this.defaultChatId;
    if (!this.isConfigured || !target) return false;
    try {
      const response = await fetch(`${this.baseUrl}/sendMessage`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ chat_id: target, text, parse_mode: "HTML" }),
      });
      if (!response.ok) {
        this.logger.warn(`Telegram sendMessage falhou: HTTP ${response.status}`);
        return false;
      }
      return true;
    } catch (error) {
      this.logger.warn(`Telegram indisponível: ${(error as Error).message}`);
      return false;
    }
  }
}
