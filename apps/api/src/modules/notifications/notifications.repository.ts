import { Injectable } from "@nestjs/common";
import { monthRange } from "../../common/date.util";
import { PrismaService } from "../../common/prisma.service";

export interface BudgetSnapshot {
  categoryName: string;
  budgetCents: number;
  spentCents: number;
}

@Injectable()
export class NotificationsRepository {
  constructor(private readonly prisma: PrismaService) {}

  async budgetSnapshot(
    categoryId: string,
    year: number,
    month: number,
  ): Promise<BudgetSnapshot | null> {
    const budget = await this.prisma.budget.findUnique({
      where: { categoryId },
      include: { category: { select: { name: true } } },
    });
    if (!budget) return null;
    const agg = await this.prisma.transaction.aggregate({
      where: {
        categoryId,
        type: { in: ["EXPENSE", "CARD_EXPENSE"] },
        date: monthRange(year, month),
      },
      _sum: { amountCents: true },
    });
    return {
      categoryName: budget.category.name,
      budgetCents: budget.amountCents,
      spentCents: Math.abs(agg._sum.amountCents ?? 0),
    };
  }
}
