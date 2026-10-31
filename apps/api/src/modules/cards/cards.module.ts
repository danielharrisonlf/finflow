import { Module } from "@nestjs/common";
import { NotificationsModule } from "../notifications/notifications.module";
import { CardsController } from "./cards.controller";
import { CardsRepository } from "./cards.repository";
import { CardsService } from "./cards.service";

@Module({
  imports: [NotificationsModule],
  controllers: [CardsController],
  providers: [CardsService, CardsRepository],
  exports: [CardsService, CardsRepository],
})
export class CardsModule {}
