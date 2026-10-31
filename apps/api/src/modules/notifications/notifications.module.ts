import { Module } from "@nestjs/common";
import { NotificationsRepository } from "./notifications.repository";
import { NotificationsService } from "./notifications.service";
import { TelegramApiService } from "./telegram-api.service";

@Module({
  providers: [NotificationsService, NotificationsRepository, TelegramApiService],
  exports: [NotificationsService, TelegramApiService],
})
export class NotificationsModule {}
