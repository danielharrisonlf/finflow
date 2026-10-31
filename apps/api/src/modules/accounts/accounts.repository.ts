import { Injectable } from "@nestjs/common";
import { PrismaService } from "../../common/prisma.service";
import type { Account, Prisma } from "../../generated/prisma/client";

@Injectable()
export class AccountsRepository {
  constructor(private readonly prisma: PrismaService) {}

  findAll(includeArchived: boolean): Promise<Account[]> {
    return this.prisma.account.findMany({
      where: includeArchived ? undefined : { archived: false },
      orderBy: { createdAt: "asc" },
    });
  }

  findById(id: string): Promise<Account | null> {
    return this.prisma.account.findUnique({ where: { id } });
  }

  /** Soma assinada das transações confirmadas por conta (saldo = inicial + soma). */
  async confirmedSumsByAccount(): Promise<Map<string, number>> {
    const groups = await this.prisma.transaction.groupBy({
      by: ["accountId"],
      where: { status: "CONFIRMED", accountId: { not: null } },
      _sum: { amountCents: true },
    });
    const map = new Map<string, number>();
    for (const g of groups) {
      if (g.accountId) map.set(g.accountId, g._sum.amountCents ?? 0);
    }
    return map;
  }

  countTransactions(accountId: string): Promise<number> {
    return this.prisma.transaction.count({ where: { accountId } });
  }

  create(data: Prisma.AccountUncheckedCreateInput): Promise<Account> {
    return this.prisma.account.create({ data });
  }

  update(id: string, data: Prisma.AccountUncheckedUpdateInput): Promise<Account> {
    return this.prisma.account.update({ where: { id }, data });
  }

  delete(id: string): Promise<Account> {
    return this.prisma.account.delete({ where: { id } });
  }
}
