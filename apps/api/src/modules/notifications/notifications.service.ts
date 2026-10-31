import { Injectable } from "@nestjs/common";
import { currentMonthYear } from "../../common/date.util";
import { formatBRL } from "../../common/money.util";
import { crossedThreshold } from "./budget-threshold";
import { NotificationsRepository } from "./notifications.repository";
import { TelegramApiService } from "./telegram-api.service";

@Injectable()
export class NotificationsService {
  constructor(
    private readonly repository: NotificationsRepository,
    private readonly telegram: TelegramApiService,
  ) {}

  /**
   * Avisa no Telegram quando um novo gasto cruza 70% ou 90% do orçamento
   * da categoria no mês corrente. Chamado em fire-and-forget pelos serviços
   * de transações e cartões — falhas aqui nunca afetam o lançamento.
   */
  async notifyBudgetThresholdCrossed(categoryId: string, addedCents = 0): Promise<void> {
    if (!this.telegram.isConfigured || !this.telegram.defaultChatId) return;

    const { month, year } = currentMonthYear();
    const snapshot = await this.repository.budgetSnapshot(categoryId, year, month);
    if (!snapshot || snapshot.budgetCents <= 0) return;

    const toPercent = (cents: number): number => (cents / snapshot.budgetCents) * 100;
    const after = toPercent(snapshot.spentCents);
    const before = toPercent(Math.max(0, snapshot.spentCents - Math.abs(addedCents)));
    const threshold = crossedThreshold(before, after);
    if (!threshold) return;

    const emoji = threshold === 90 ? "🚨" : "⚠️";
    const headline = threshold === 90 ? "Orçamento quase estourado" : "Orçamento em atenção";
    await this.telegram.sendMessage(
      [
        `${emoji} <b>${headline}</b>`,
        `Categoria: <b>${snapshot.categoryName}</b>`,
        `Gasto no mês: <b>${formatBRL(snapshot.spentCents)}</b> de ${formatBRL(snapshot.budgetCents)} (${after.toFixed(0)}%)`,
      ].join("\n"),
    );
  }
}
