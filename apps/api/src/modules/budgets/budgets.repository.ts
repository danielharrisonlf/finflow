import { Injectable } from "@nestjs/common";
import { monthRange } from "../../common/date.util";
import { PrismaService } from "../../common/prisma.service";
import type { Budget, Category } from "../../generated/prisma/client";

export type BudgetWithCategory = Budget & { category: Category };

@Injectable()
export class BudgetsRepository {
  constructor(private readonly prisma: PrismaService) {}

  findAll(): Promise<BudgetWithCategory[]> {
    return this.prisma.budget.findMany({
      include: { category: true },
      orderBy: { category: { name: "asc" } },
    });
  }

  findById(id: string): Promise<BudgetWithCategory | null> {
    return this.prisma.budget.findUnique({ where: { id }, include: { category: true } });
  }

  /** Gasto do mês por categoria (despesas em conta e no cartão, pendentes incluídas). */
  async spentByCategory(year: number, month: number): Promise<Map<string, number>> {
    const groups = await this.prisma.transaction.groupBy({
      by: ["categoryId"],
      where: {
        type: { in: ["EXPENSE", "CARD_EXPENSE"] },
        categoryId: { not: null },
        date: monthRange(year, month),
      },
      _sum: { amountCents: true },
    });
    const map = new Map<string, number>();
    for (const g of groups) {
      if (g.categoryId) map.set(g.categoryId, Math.abs(g._sum.amountCents ?? 0));
    }
    return map;
  }

  findCategory(categoryId: string): Promise<Category | null> {
    return this.prisma.category.findUnique({ where: { id: categoryId } });
  }

  upsert(categoryId: string, amountCents: number): Promise<BudgetWithCategory> {
    return this.prisma.budget.upsert({
      where: { categoryId },
      create: { categoryId, amountCents },
      update: { amountCents },
      include: { category: true },
    });
  }

  delete(id: string): Promise<Budget> {
    return this.prisma.budget.delete({ where: { id } });
  }
}
