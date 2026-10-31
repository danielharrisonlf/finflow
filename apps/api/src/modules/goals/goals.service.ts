import type {
  ContributeGoalInput,
  CreateGoalInput,
  Goal,
  UpdateGoalInput,
} from "@finflow/contracts";
import { BadRequestException, Injectable, NotFoundException } from "@nestjs/common";
import { fromDateString, toDateString, todayString } from "../../common/date.util";
import { goalPercent, monthlyNeededCents } from "./goal-math";
import { GoalsRepository, GoalWithSaved } from "./goals.repository";

function toContract(db: GoalWithSaved): Goal {
  const deadline = db.deadline ? toDateString(db.deadline) : null;
  return {
    id: db.id,
    name: db.name,
    icon: db.icon,
    color: db.color,
    targetCents: db.targetCents,
    savedCents: db.savedCents,
    deadline,
    percent: goalPercent(db.targetCents, db.savedCents),
    monthlyNeededCents: monthlyNeededCents(db.targetCents, db.savedCents, deadline),
  };
}

@Injectable()
export class GoalsService {
  constructor(private readonly repository: GoalsRepository) {}

  async list(): Promise<Goal[]> {
    const goals = await this.repository.findAll();
    return goals.map(toContract);
  }

  async findById(id: string): Promise<Goal> {
    return toContract(await this.requireGoal(id));
  }

  async create(input: CreateGoalInput): Promise<Goal> {
    const created = await this.repository.create({
      name: input.name,
      icon: input.icon,
      color: input.color ?? null,
      targetCents: input.targetCents,
      deadline: input.deadline ? fromDateString(input.deadline) : null,
    });
    return toContract(created);
  }

  async update(id: string, input: UpdateGoalInput): Promise<Goal> {
    await this.requireGoal(id);
    const updated = await this.repository.update(id, {
      name: input.name,
      icon: input.icon,
      color: input.color,
      targetCents: input.targetCents,
      deadline:
        input.deadline === undefined
          ? undefined
          : input.deadline
            ? fromDateString(input.deadline)
            : null,
    });
    if (!updated) throw new NotFoundException("meta não encontrada");
    return toContract(updated);
  }

  async delete(id: string): Promise<void> {
    await this.requireGoal(id);
    await this.repository.delete(id);
  }

  /** Aporte (positivo) ou resgate (negativo); o guardado nunca fica negativo. */
  async contribute(id: string, input: ContributeGoalInput): Promise<Goal> {
    const goal = await this.requireGoal(id);
    if (goal.savedCents + input.amountCents < 0) {
      throw new BadRequestException("resgate maior que o valor guardado na meta");
    }
    await this.repository.addContribution({
      goalId: id,
      amountCents: input.amountCents,
      date: input.date ?? todayString(),
      note: input.note ?? null,
    });
    const refreshed = await this.requireGoal(id);
    return toContract(refreshed);
  }

  private async requireGoal(id: string): Promise<GoalWithSaved> {
    const goal = await this.repository.findById(id);
    if (!goal) throw new NotFoundException("meta não encontrada");
    return goal;
  }
}
