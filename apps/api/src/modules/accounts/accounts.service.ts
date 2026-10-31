import type { Account, CreateAccountInput, UpdateAccountInput } from "@finflow/contracts";
import { ConflictException, Injectable, NotFoundException } from "@nestjs/common";
import type { Account as DbAccount } from "../../generated/prisma/client";
import { AccountsRepository } from "./accounts.repository";

function toContract(db: DbAccount, confirmedSumCents: number): Account {
  return {
    id: db.id,
    name: db.name,
    type: db.type,
    initialBalanceCents: db.initialBalanceCents,
    balanceCents: db.initialBalanceCents + confirmedSumCents,
    color: db.color,
    archived: db.archived,
  };
}

@Injectable()
export class AccountsService {
  constructor(private readonly repository: AccountsRepository) {}

  async list(includeArchived = false): Promise<Account[]> {
    const [accounts, sums] = await Promise.all([
      this.repository.findAll(includeArchived),
      this.repository.confirmedSumsByAccount(),
    ]);
    return accounts.map((a) => toContract(a, sums.get(a.id) ?? 0));
  }

  async findById(id: string): Promise<Account> {
    const account = await this.repository.findById(id);
    if (!account) throw new NotFoundException("conta não encontrada");
    const sums = await this.repository.confirmedSumsByAccount();
    return toContract(account, sums.get(id) ?? 0);
  }

  async create(input: CreateAccountInput): Promise<Account> {
    const created = await this.repository.create({
      name: input.name,
      type: input.type,
      initialBalanceCents: input.initialBalanceCents,
      color: input.color ?? null,
    });
    return toContract(created, 0);
  }

  async update(id: string, input: UpdateAccountInput): Promise<Account> {
    await this.ensureExists(id);
    const updated = await this.repository.update(id, {
      name: input.name,
      type: input.type,
      initialBalanceCents: input.initialBalanceCents,
      color: input.color,
      archived: input.archived,
    });
    const sums = await this.repository.confirmedSumsByAccount();
    return toContract(updated, sums.get(id) ?? 0);
  }

  /** Remove conta sem movimentações; com movimentações, exige arquivamento. */
  async delete(id: string): Promise<void> {
    await this.ensureExists(id);
    const txCount = await this.repository.countTransactions(id);
    if (txCount > 0) {
      throw new ConflictException(
        "conta possui transações — arquive-a em vez de excluir para preservar o histórico",
      );
    }
    await this.repository.delete(id);
  }

  private async ensureExists(id: string): Promise<void> {
    const found = await this.repository.findById(id);
    if (!found) throw new NotFoundException("conta não encontrada");
  }
}
