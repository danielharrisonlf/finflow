import { Module } from "@nestjs/common";
import { PrismaModule } from "./common/prisma.service";
import { AccountsModule } from "./modules/accounts/accounts.module";
import { BudgetsModule } from "./modules/budgets/budgets.module";
import { CardsModule } from "./modules/cards/cards.module";
import { CategoriesModule } from "./modules/categories/categories.module";
import { DashboardModule } from "./modules/dashboard/dashboard.module";
import { GoalsModule } from "./modules/goals/goals.module";
import { NotificationsModule } from "./modules/notifications/notifications.module";
import { TelegramModule } from "./modules/telegram/telegram.module";
import { TransactionsModule } from "./modules/transactions/transactions.module";

@Module({
  imports: [
    PrismaModule,
    CategoriesModule,
    AccountsModule,
    CardsModule,
    TransactionsModule,
    BudgetsModule,
    GoalsModule,
    DashboardModule,
    NotificationsModule,
    TelegramModule,
  ],
})
export class AppModule {}
