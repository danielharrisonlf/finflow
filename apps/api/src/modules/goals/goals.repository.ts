import { Injectable } from "@nestjs/common";
import { fromDateString } from "../../common/date.util";
import { PrismaService } from "../../common/prisma.service";
import type { Goal, GoalContribution, Prisma } from "../../generated/prisma/client";

export type GoalWithSaved = Goal & { savedCents: number };

@Injectable()
export class GoalsRepository {
  constructor(private readonly prisma: PrismaService) {}

  private async withSaved(goals: Goal[]): Promise<GoalWithSaved[]> {
    const sums = await this.prisma.goalContribution.groupBy({
      by: ["goalId"],
      where: { goalId: { in: goals.map((g) => g.id) } },
      _sum: { amountCents: true },
    });
    const map = new Map(sums.map((s) => [s.goalId, s._sum.amountCents ?? 0]));
    return goals.map((g) => ({ ...g, savedCents: map.get(g.id) ?? 0 }));
  }

  async findAll(): Promise<GoalWithSaved[]> {
    const goals = await this.prisma.goal.findMany({
      where: { archived: false },
      orderBy: { createdAt: "asc" },
    });
    return this.withSaved(goals);
  }

  async findById(id: string): Promise<GoalWithSaved | null> {
    const goal = await this.prisma.goal.findUnique({ where: { id } });
    if (!goal) return null;
    const [withSaved] = await this.withSaved([goal]);
    return withSaved ?? null;
  }

  async create(data: Prisma.GoalUncheckedCreateInput): Promise<GoalWithSaved> {
    const goal = await this.prisma.goal.create({ data });
    return { ...goal, savedCents: 0 };
  }

  async update(id: string, data: Prisma.GoalUncheckedUpdateInput): Promise<GoalWithSaved | null> {
    await this.prisma.goal.update({ where: { id }, data });
    return this.findById(id);
  }

  delete(id: string): Promise<Goal> {
    return this.prisma.goal.delete({ where: { id } });
  }

  addContribution(params: {
    goalId: string;
    amountCents: number;
    date: string;
    note: string | null;
  }): Promise<GoalContribution> {
    return this.prisma.goalContribution.create({
      data: {
        goalId: params.goalId,
        amountCents: params.amountCents,
        date: fromDateString(params.date),
        note: params.note,
      },
    });
  }

  listContributions(goalId: string): Promise<GoalContribution[]> {
    return this.prisma.goalContribution.findMany({
      where: { goalId },
      orderBy: [{ date: "desc" }, { createdAt: "desc" }],
    });
  }
}
