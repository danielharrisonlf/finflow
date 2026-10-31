import { randomUUID } from "node:crypto";
import { Injectable } from "@nestjs/common";
import { fromDateString } from "../../common/date.util";
import { PrismaService } from "../../common/prisma.service";
import type { CreditCard, Invoice, Prisma, Transaction } from "../../generated/prisma/client";

export type InvoiceWithTotal = Invoice & { totalCents: number };

export interface CardExpenseLeg {
  description: string;
  amountCents: number;
  date: string;
  categoryId: string | null;
  notes: string | null;
  installmentNumber: number | null;
  installmentTotal: number | null;
  invoicePeriod: { year: number; month: number };
}

@Injectable()
export class CardsRepository {
  constructor(private readonly prisma: PrismaService) {}

  findAll(includeArchived: boolean): Promise<CreditCard[]> {
    return this.prisma.creditCard.findMany({
      where: includeArchived ? undefined : { archived: false },
      orderBy: { createdAt: "asc" },
    });
  }

  findById(id: string): Promise<CreditCard | null> {
    return this.prisma.creditCard.findUnique({ where: { id } });
  }

  create(data: Prisma.CreditCardUncheckedCreateInput): Promise<CreditCard> {
    return this.prisma.creditCard.create({ data });
  }

  update(id: string, data: Prisma.CreditCardUncheckedUpdateInput): Promise<CreditCard> {
    return this.prisma.creditCard.update({ where: { id }, data });
  }

  countTransactions(cardId: string): Promise<number> {
    return this.prisma.transaction.count({ where: { cardId } });
  }

  delete(id: string): Promise<CreditCard> {
    return this.prisma.creditCard.delete({ where: { id } });
  }

  findInvoice(invoiceId: string): Promise<Invoice | null> {
    return this.prisma.invoice.findUnique({ where: { id: invoiceId } });
  }

  async findInvoicesWithTotals(cardId: string): Promise<InvoiceWithTotal[]> {
    const invoices = await this.prisma.invoice.findMany({
      where: { cardId },
      orderBy: [{ year: "desc" }, { month: "desc" }],
    });
    const totals = await this.prisma.transaction.groupBy({
      by: ["invoiceId"],
      where: { invoiceId: { in: invoices.map((i) => i.id) } },
      _sum: { amountCents: true },
    });
    const totalMap = new Map(totals.map((t) => [t.invoiceId, t._sum.amountCents ?? 0]));
    return invoices.map((i) => ({ ...i, totalCents: Math.abs(totalMap.get(i.id) ?? 0) }));
  }

  findInvoiceItems(invoiceId: string): Promise<
    (Transaction & {
      category: { name: string; icon: string; color: string } | null;
      card: { name: string } | null;
    })[]
  > {
    return this.prisma.transaction.findMany({
      where: { invoiceId },
      include: {
        category: { select: { name: true, icon: true, color: true } },
        card: { select: { name: true } },
      },
      orderBy: [{ date: "asc" }, { createdAt: "asc" }],
    });
  }

  /** Total comprometido do limite: itens de faturas ainda não pagas. */
  async unpaidTotalCents(cardId: string): Promise<number> {
    const agg = await this.prisma.transaction.aggregate({
      where: { invoice: { cardId, status: { not: "PAID" } } },
      _sum: { amountCents: true },
    });
    return Math.abs(agg._sum.amountCents ?? 0);
  }

  /** Cria as parcelas da compra e as faturas que ainda não existem, atomicamente. */
  async createCardExpense(cardId: string, legs: CardExpenseLeg[]): Promise<Transaction[]> {
    const installmentGroupId = legs.length > 1 ? randomUUID() : null;
    return this.prisma.$transaction(async (tx) => {
      const created: Transaction[] = [];
      for (const leg of legs) {
        const invoice = await tx.invoice.upsert({
          where: {
            cardId_year_month: {
              cardId,
              year: leg.invoicePeriod.year,
              month: leg.invoicePeriod.month,
            },
          },
          create: { cardId, year: leg.invoicePeriod.year, month: leg.invoicePeriod.month },
          update: {},
        });
        created.push(
          await tx.transaction.create({
            data: {
              type: "CARD_EXPENSE",
              description: leg.description,
              amountCents: -Math.abs(leg.amountCents),
              date: fromDateString(leg.date),
              status: "CONFIRMED",
              cardId,
              categoryId: leg.categoryId,
              invoiceId: invoice.id,
              installmentGroupId,
              installmentNumber: leg.installmentNumber,
              installmentTotal: leg.installmentTotal,
              notes: leg.notes,
            },
          }),
        );
      }
      return created;
    });
  }

  /** Paga a fatura: transação de pagamento na conta + marca PAID, atomicamente. */
  async payInvoice(params: {
    invoiceId: string;
    accountId: string;
    date: string;
    description: string;
    totalCents: number;
  }): Promise<Transaction> {
    return this.prisma.$transaction(async (tx) => {
      const payment = await tx.transaction.create({
        data: {
          type: "INVOICE_PAYMENT",
          description: params.description,
          amountCents: -Math.abs(params.totalCents),
          date: fromDateString(params.date),
          status: "CONFIRMED",
          accountId: params.accountId,
        },
      });
      await tx.invoice.update({
        where: { id: params.invoiceId },
        data: {
          status: "PAID",
          paidAt: fromDateString(params.date),
          paymentTransactionId: payment.id,
        },
      });
      return payment;
    });
  }
}
