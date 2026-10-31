import { Injectable } from "@nestjs/common";
import { fromDateString, monthRange } from "../../common/date.util";
import { PrismaService } from "../../common/prisma.service";
import type { Category, Invoice, Transaction } from "../../generated/prisma/client";

export type InvoiceWithCardAndTotal = Invoice & {
  card: { name: string; closingDay: number; dueDay: number };
  totalCents: number;
};

@Injectable()
export class DashboardRepository {
  constructor(private readonly prisma: PrismaService) {}

  async totalBalanceCents(): Promise<number> {
    const [accounts, txSum] = await Promise.all([
      this.prisma.account.aggregate({
        where: { archived: false },
        _sum: { initialBalanceCents: true },
      }),
      this.prisma.transaction.aggregate({
        where: { status: "CONFIRMED", account: { is: { archived: false } } },
        _sum: { amountCents: true },
      }),
    ]);
    return (accounts._sum.initialBalanceCents ?? 0) + (txSum._sum.amountCents ?? 0);
  }

  async monthIncomeCents(year: number, month: number): Promise<number> {
    const agg = await this.prisma.transaction.aggregate({
      where: { type: "INCOME", status: "CONFIRMED", date: monthRange(year, month) },
      _sum: { amountCents: true },
    });
    return agg._sum.amountCents ?? 0;
  }

  async monthExpenseCents(year: number, month: number): Promise<number> {
    const agg = await this.prisma.transaction.aggregate({
      where: {
        type: { in: ["EXPENSE", "CARD_EXPENSE"] },
        status: "CONFIRMED",
        date: monthRange(year, month),
      },
      _sum: { amountCents: true },
    });
    return Math.abs(agg._sum.amountCents ?? 0);
  }

  async openInvoicesCents(): Promise<number> {
    const agg = await this.prisma.transaction.aggregate({
      where: { invoice: { is: { status: { not: "PAID" } } } },
      _sum: { amountCents: true },
    });
    return Math.abs(agg._sum.amountCents ?? 0);
  }

  pendingCount(): Promise<number> {
    return this.prisma.transaction.count({ where: { status: "PENDING" } });
  }

  /** Movimentos do mês para o gráfico de fluxo de caixa. */
  monthMovements(
    year: number,
    month: number,
  ): Promise<Pick<Transaction, "date" | "amountCents" | "type">[]> {
    return this.prisma.transaction.findMany({
      where: {
        type: { in: ["INCOME", "EXPENSE", "CARD_EXPENSE"] },
        status: "CONFIRMED",
        date: monthRange(year, month),
      },
      select: { date: true, amountCents: true, type: true },
      orderBy: { date: "asc" },
    });
  }

  async expensesByCategory(
    year: number,
    month: number,
  ): Promise<{ categoryId: string | null; totalCents: number }[]> {
    const groups = await this.prisma.transaction.groupBy({
      by: ["categoryId"],
      where: {
        type: { in: ["EXPENSE", "CARD_EXPENSE"] },
        status: "CONFIRMED",
        date: monthRange(year, month),
      },
      _sum: { amountCents: true },
    });
    return groups.map((g) => ({
      categoryId: g.categoryId,
      totalCents: Math.abs(g._sum.amountCents ?? 0),
    }));
  }

  findCategories(ids: string[]): Promise<Category[]> {
    return this.prisma.category.findMany({ where: { id: { in: ids } } });
  }

  pendingBetween(
    start: string,
    end: string,
  ): Promise<(Transaction & { category: { icon: string } | null })[]> {
    return this.prisma.transaction.findMany({
      where: {
        status: "PENDING",
        date: { gte: fromDateString(start), lte: fromDateString(end) },
      },
      include: { category: { select: { icon: true } } },
      orderBy: { date: "asc" },
    });
  }

  async unpaidInvoicesWithTotals(): Promise<InvoiceWithCardAndTotal[]> {
    const invoices = await this.prisma.invoice.findMany({
      where: { status: { not: "PAID" } },
      include: { card: { select: { name: true, closingDay: true, dueDay: true } } },
    });
    const totals = await this.prisma.transaction.groupBy({
      by: ["invoiceId"],
      where: { invoiceId: { in: invoices.map((i) => i.id) } },
      _sum: { amountCents: true },
    });
    const totalMap = new Map(totals.map((t) => [t.invoiceId, Math.abs(t._sum.amountCents ?? 0)]));
    return invoices.map((i) => ({ ...i, totalCents: totalMap.get(i.id) ?? 0 }));
  }
}
