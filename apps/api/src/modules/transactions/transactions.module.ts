import { Module } from "@nestjs/common";
import { AccountsModule } from "../accounts/accounts.module";
import { NotificationsModule } from "../notifications/notifications.module";
import { ExportService } from "./export.service";
import { TransactionsController } from "./transactions.controller";
import { TransactionsRepository } from "./transactions.repository";
import { TransactionsService } from "./transactions.service";

@Module({
  imports: [AccountsModule, NotificationsModule],
  controllers: [TransactionsController],
  providers: [TransactionsService, TransactionsRepository, ExportService],
  exports: [TransactionsService],
})
export class TransactionsModule {}
