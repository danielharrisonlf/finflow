import type {
  CashflowPoint,
  CategorySlice,
  DashboardQuery,
  DashboardSummary,
  UpcomingItem,
} from "@finflow/contracts";
import { Injectable } from "@nestjs/common";
import { addDays, currentMonthYear, toDateString, todayString } from "../../common/date.util";
import { invoiceDueDate } from "../cards/invoice-cycle";
import { DashboardRepository } from "./dashboard.repository";

function resolvePeriod(query: DashboardQuery): { month: number; year: number } {
  const fallback = currentMonthYear();
  return { month: query.month ?? fallback.month, year: query.year ?? fallback.year };
}

@Injectable()
export class DashboardService {
  constructor(private readonly repository: DashboardRepository) {}

  async summary(query: DashboardQuery): Promise<DashboardSummary> {
    const { month, year } = resolvePeriod(query);
    const [
      totalBalanceCents,
      monthIncomeCents,
      monthExpenseCents,
      openInvoicesCents,
      pendingCount,
    ] = await Promise.all([
      this.repository.totalBalanceCents(),
      this.repository.monthIncomeCents(year, month),
      this.repository.monthExpenseCents(year, month),
      this.repository.openInvoicesCents(),
      this.repository.pendingCount(),
    ]);
    return {
      month,
      year,
      totalBalanceCents,
      monthIncomeCents,
      monthExpenseCents,
      monthNetCents: monthIncomeCents - monthExpenseCents,
      openInvoicesCents,
      pendingCount,
    };
  }

  /** Série diária de entradas vs. saídas do mês. */
  async cashflow(query: DashboardQuery): Promise<CashflowPoint[]> {
    const { month, year } = resolvePeriod(query);
    const movements = await this.repository.monthMovements(year, month);
    const byDay = new Map<string, { incomeCents: number; expenseCents: number }>();
    for (const m of movements) {
      const day = toDateString(m.date);
      const bucket = byDay.get(day) ?? { incomeCents: 0, expenseCents: 0 };
      if (m.type === "INCOME") bucket.incomeCents += m.amountCents;
      else bucket.expenseCents += Math.abs(m.amountCents);
      byDay.set(day, bucket);
    }
    return [...byDay.entries()]
      .sort(([a], [b]) => (a < b ? -1 : 1))
      .map(([date, v]) => ({ date, ...v }));
  }

  /** Distribuição de despesas por categoria (gráfico donut). */
  async categories(query: DashboardQuery): Promise<CategorySlice[]> {
    const { month, year } = resolvePeriod(query);
    const groups = await this.repository.expensesByCategory(year, month);
    const total = groups.reduce((acc, g) => acc + g.totalCents, 0);
    const ids = groups.map((g) => g.categoryId).filter((id): id is string => id !== null);
    const categories = await this.repository.findCategories(ids);
    const meta = new Map(categories.map((c) => [c.id, c]));
    return groups
      .map((g) => {
        const category = g.categoryId ? meta.get(g.categoryId) : undefined;
        return {
          categoryId: g.categoryId,
          name: category?.name ?? "Sem categoria",
          icon: category?.icon ?? null,
          color: category?.color ?? null,
          totalCents: g.totalCents,
          percent: total > 0 ? Math.round((g.totalCents / total) * 1000) / 10 : 0,
        };
      })
      .sort((a, b) => b.totalCents - a.totalCents);
  }

  /** Contas pendentes e faturas vencendo nos próximos 7 dias. */
  async upcoming(): Promise<UpcomingItem[]> {
    const today = todayString();
    const limit = addDays(today, 7);
    const [pending, invoices] = await Promise.all([
      this.repository.pendingBetween(today, limit),
      this.repository.unpaidInvoicesWithTotals(),
    ]);

    const items: UpcomingItem[] = pending.map((t) => ({
      id: t.id,
      kind: "TRANSACTION" as const,
      description: t.description,
      dueDate: toDateString(t.date),
      amountCents: t.amountCents,
      categoryIcon: t.category?.icon ?? null,
    }));

    for (const invoice of invoices) {
      if (invoice.totalCents <= 0) continue;
      const dueDate = invoiceDueDate(
        { year: invoice.year, month: invoice.month },
        invoice.card.closingDay,
        invoice.card.dueDay,
      );
      if (dueDate >= today && dueDate <= limit) {
        items.push({
          id: invoice.id,
          kind: "INVOICE" as const,
          description: `Fatura ${invoice.card.name}`,
          dueDate,
          amountCents: -invoice.totalCents,
          categoryIcon: "credit-card",
        });
      }
    }

    return items.sort((a, b) => (a.dueDate < b.dueDate ? -1 : 1));
  }
}
