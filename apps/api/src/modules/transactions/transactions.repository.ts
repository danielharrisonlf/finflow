import { randomUUID } from "node:crypto";
import type { ListTransactionsQuery } from "@finflow/contracts";
import { Injectable } from "@nestjs/common";
import { fromDateString, monthRange } from "../../common/date.util";
import { PrismaService } from "../../common/prisma.service";
import type { Prisma, Transaction } from "../../generated/prisma/client";

export type TransactionWithRefs = Transaction & {
  account: { name: string } | null;
  card: { name: string } | null;
  category: { name: string; icon: string; color: string } | null;
};

const includeRefs = {
  account: { select: { name: true } },
  card: { select: { name: true } },
  category: { select: { name: true, icon: true, color: true } },
} satisfies Prisma.TransactionInclude;

export function buildWhere(
  query: Omit<ListTransactionsQuery, "page" | "pageSize">,
): Prisma.TransactionWhereInput {
  const where: Prisma.TransactionWhereInput = {};
  if (query.year || query.month) {
    const year = query.year ?? new Date().getUTCFullYear();
    where.date = query.month
      ? monthRange(year, query.month)
      : { gte: monthRange(year, 1).gte, lt: monthRange(year + 1, 1).gte };
  }
  if (query.accountId) where.accountId = query.accountId;
  if (query.cardId) where.cardId = query.cardId;
  if (query.categoryId) where.categoryId = query.categoryId;
  if (query.type) where.type = query.type;
  if (query.status) where.status = query.status;
  if (query.search) where.description = { contains: query.search, mode: "insensitive" };
  return where;
}

@Injectable()
export class TransactionsRepository {
  constructor(private readonly prisma: PrismaService) {}

  async list(
    query: ListTransactionsQuery,
  ): Promise<{ items: TransactionWithRefs[]; total: number }> {
    const where = buildWhere(query);
    const [items, total] = await Promise.all([
      this.prisma.transaction.findMany({
        where,
        include: includeRefs,
        orderBy: [{ date: "desc" }, { createdAt: "desc" }],
        skip: (query.page - 1) * query.pageSize,
        take: query.pageSize,
      }),
      this.prisma.transaction.count({ where }),
    ]);
    return { items, total };
  }

  listAll(query: Omit<ListTransactionsQuery, "page" | "pageSize">): Promise<TransactionWithRefs[]> {
    return this.prisma.transaction.findMany({
      where: buildWhere(query),
      include: includeRefs,
      orderBy: [{ date: "asc" }, { createdAt: "asc" }],
    });
  }

  findById(id: string): Promise<TransactionWithRefs | null> {
    return this.prisma.transaction.findUnique({ where: { id }, include: includeRefs });
  }

  create(data: Prisma.TransactionUncheckedCreateInput): Promise<TransactionWithRefs> {
    return this.prisma.transaction.create({ data, include: includeRefs });
  }

  update(id: string, data: Prisma.TransactionUncheckedUpdateInput): Promise<TransactionWithRefs> {
    return this.prisma.transaction.update({ where: { id }, data, include: includeRefs });
  }

  /** Cria as duas pernas da transferência com o mesmo grupo, atomicamente. */
  async createTransferPair(params: {
    fromAccountId: string;
    toAccountId: string;
    amountCents: number;
    date: string;
    fromDescription: string;
    toDescription: string;
  }): Promise<{ transferGroupId: string }> {
    const transferGroupId = randomUUID();
    const date = fromDateString(params.date);
    await this.prisma.$transaction([
      this.prisma.transaction.create({
        data: {
          type: "TRANSFER",
          description: params.fromDescription,
          amountCents: -params.amountCents,
          date,
          status: "CONFIRMED",
          accountId: params.fromAccountId,
          transferGroupId,
        },
      }),
      this.prisma.transaction.create({
        data: {
          type: "TRANSFER",
          description: params.toDescription,
          amountCents: params.amountCents,
          date,
          status: "CONFIRMED",
          accountId: params.toAccountId,
          transferGroupId,
        },
      }),
    ]);
    return { transferGroupId };
  }

  /** Apaga as duas pernas de uma transferência. */
  deleteTransferGroup(transferGroupId: string): Promise<{ count: number }> {
    return this.prisma.transaction.deleteMany({ where: { transferGroupId } });
  }

  /** Apaga todas as parcelas de uma compra parcelada. */
  deleteInstallmentGroup(installmentGroupId: string): Promise<{ count: number }> {
    return this.prisma.transaction.deleteMany({ where: { installmentGroupId } });
  }

  /** Apaga pagamento de fatura reabrindo a fatura, atomicamente. */
  async deleteInvoicePayment(transactionId: string, invoiceId: string): Promise<void> {
    await this.prisma.$transaction([
      this.prisma.invoice.update({
        where: { id: invoiceId },
        data: { status: "OPEN", paidAt: null, paymentTransactionId: null },
      }),
      this.prisma.transaction.delete({ where: { id: transactionId } }),
    ]);
  }

  /** Fatura que esta transação paga, se for um pagamento de fatura. */
  findInvoicePaidBy(transactionId: string): Promise<{ id: string } | null> {
    return this.prisma.invoice.findUnique({
      where: { paymentTransactionId: transactionId },
      select: { id: true },
    });
  }

  delete(id: string): Promise<Transaction> {
    return this.prisma.transaction.delete({ where: { id } });
  }
}
