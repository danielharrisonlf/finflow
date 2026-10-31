import type { Budget, ListBudgetsQuery, UpsertBudgetInput } from "@finflow/contracts";
import { BadRequestException, Injectable, NotFoundException } from "@nestjs/common";
import { currentMonthYear } from "../../common/date.util";
import { computeBudgetView } from "./budget-view";
import { BudgetsRepository, BudgetWithCategory } from "./budgets.repository";

function toContract(db: BudgetWithCategory, spentCents: number): Budget {
  const view = computeBudgetView(db.amountCents, spentCents);
  return {
    id: db.id,
    categoryId: db.categoryId,
    categoryName: db.category.name,
    categoryIcon: db.category.icon,
    categoryColor: db.category.color,
    amountCents: db.amountCents,
    spentCents,
    percent: view.percent,
    status: view.status,
  };
}

@Injectable()
export class BudgetsService {
  constructor(private readonly repository: BudgetsRepository) {}

  async list(query: ListBudgetsQuery): Promise<Budget[]> {
    const fallback = currentMonthYear();
    const year = query.year ?? fallback.year;
    const month = query.month ?? fallback.month;
    const [budgets, spent] = await Promise.all([
      this.repository.findAll(),
      this.repository.spentByCategory(year, month),
    ]);
    return budgets.map((b) => toContract(b, spent.get(b.categoryId) ?? 0));
  }

  async upsert(input: UpsertBudgetInput): Promise<Budget> {
    const category = await this.repository.findCategory(input.categoryId);
    if (!category) throw new NotFoundException("categoria não encontrada");
    if (category.kind !== "EXPENSE") {
      throw new BadRequestException("orçamentos só se aplicam a categorias de despesa");
    }
    const saved = await this.repository.upsert(input.categoryId, input.amountCents);
    const { month, year } = currentMonthYear();
    const spent = await this.repository.spentByCategory(year, month);
    return toContract(saved, spent.get(input.categoryId) ?? 0);
  }

  async delete(id: string): Promise<void> {
    const found = await this.repository.findById(id);
    if (!found) throw new NotFoundException("orçamento não encontrado");
    await this.repository.delete(id);
  }
}
