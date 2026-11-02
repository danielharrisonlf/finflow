import { type TelegramStatus, type TelegramUpdate, telegramUpdateSchema } from "@finflow/contracts";
import { Body, Controller, ForbiddenException, Get, Headers, HttpCode, Post } from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import { ZodValidationPipe } from "../../common/zod-validation.pipe";
import { Public } from "../auth/public.decorator";
import { TelegramApiService } from "../notifications/telegram-api.service";
import { TelegramService } from "./telegram.service";

@ApiTags("telegram")
@Controller("telegram")
export class TelegramController {
  constructor(
    private readonly service: TelegramService,
    private readonly telegramApi: TelegramApiService,
  ) {}

  @Public()
  @Post("webhook")
  @HttpCode(200)
  @ApiOperation({ summary: "Webhook da Bot API do Telegram (protegido por secret token)" })
  async webhook(
    @Body(new ZodValidationPipe(telegramUpdateSchema)) update: TelegramUpdate,
    @Headers("x-telegram-bot-api-secret-token") secret?: string,
  ): Promise<{ ok: true }> {
    const expected = process.env.TELEGRAM_WEBHOOK_SECRET;
    if (expected && secret !== expected) {
      throw new ForbiddenException("secret token inválido");
    }
    await this.service.handleUpdate(update);
    return { ok: true };
  }

  @Get("status")
  @ApiOperation({ summary: "Status da integração com o Telegram" })
  status(): TelegramStatus {
    return {
      configured: this.telegramApi.isConfigured,
      chatLinked: Boolean(this.telegramApi.defaultChatId),
    };
  }
}
