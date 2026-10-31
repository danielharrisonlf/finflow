import type {
  Card,
  CreateCardExpenseInput,
  CreateCardInput,
  Invoice,
  PayInvoiceInput,
  Transaction,
  UpdateCardInput,
} from "@finflow/contracts";
import { ConflictException, Injectable, NotFoundException } from "@nestjs/common";
import { addMonthsToDateString, todayString } from "../../common/date.util";
import { formatBRL, splitInstallments } from "../../common/money.util";
import type { CreditCard as DbCard, Invoice as DbInvoice } from "../../generated/prisma/client";
import { NotificationsService } from "../notifications/notifications.service";
import { CardExpenseLeg, CardsRepository, InvoiceWithTotal } from "./cards.repository";
import {
  addMonthsToPeriod,
  effectiveInvoiceStatus,
  invoiceClosingDate,
  invoiceDueDate,
  invoicePeriodForPurchase,
} from "./invoice-cycle";

@Injectable()
export class CardsService {
  constructor(
    private readonly repository: CardsRepository,
    private readonly notifications: NotificationsService,
  ) {}

  private toInvoiceContract(db: InvoiceWithTotal, card: DbCard): Invoice {
    const period = { year: db.year, month: db.month };
    const closingDate = invoiceClosingDate(period, card.closingDay);
    return {
      id: db.id,
      cardId: db.cardId,
      month: db.month,
      year: db.year,
      status: effectiveInvoiceStatus(db.status === "PAID", closingDate, todayString()),
      totalCents: db.totalCents,
      closingDate,
      dueDate: invoiceDueDate(period, card.closingDay, card.dueDay),
      paidAt: db.paidAt ? db.paidAt.toISOString().slice(0, 10) : null,
    };
  }

  private async toCardContract(db: DbCard): Promise<Card> {
    const [usedLimitCents, invoices] = await Promise.all([
      this.repository.unpaidTotalCents(db.id),
      this.repository.findInvoicesWithTotals(db.id),
    ]);
    const current = invoicePeriodForPurchase(todayString(), db.closingDay);
    const currentDb = invoices.find((i) => i.year === current.year && i.month === current.month);
    return {
      id: db.id,
      name: db.name,
      brand: db.brand,
      limitCents: db.limitCents,
      closingDay: db.closingDay,
      dueDay: db.dueDay,
      color: db.color,
      archived: db.archived,
      usedLimitCents,
      availableLimitCents: db.limitCents - usedLimitCents,
      currentInvoice: currentDb ? this.toInvoiceContract(currentDb, db) : null,
    };
  }

  async list(includeArchived = false): Promise<Card[]> {
    const cards = await this.repository.findAll(includeArchived);
    return Promise.all(cards.map((c) => this.toCardContract(c)));
  }

  async findById(id: string): Promise<Card> {
    const card = await this.requireCard(id);
    return this.toCardContract(card);
  }

  async create(input: CreateCardInput): Promise<Card> {
    const created = await this.repository.create({
      name: input.name,
      brand: input.brand,
      limitCents: input.limitCents,
      closingDay: input.closingDay,
      dueDay: input.dueDay,
      color: input.color ?? null,
    });
    return this.toCardContract(created);
  }

  async update(id: string, input: UpdateCardInput): Promise<Card> {
    await this.requireCard(id);
    const updated = await this.repository.update(id, {
      name: input.name,
      brand: input.brand,
      limitCents: input.limitCents,
      closingDay: input.closingDay,
      dueDay: input.dueDay,
      color: input.color,
      archived: input.archived,
    });
    return this.toCardContract(updated);
  }

  async delete(id: string): Promise<void> {
    await this.requireCard(id);
    const txCount = await this.repository.countTransactions(id);
    if (txCount > 0) {
      throw new ConflictException(
        "cartão possui lançamentos — arquive-o em vez de excluir para preservar o histórico",
      );
    }
    await this.repository.delete(id);
  }

  /** Compra no cartão, à vista ou parcelada; cada parcela cai na fatura correta. */
  async addExpense(cardId: string, input: CreateCardExpenseInput): Promise<{ created: number }> {
    const card = await this.requireCard(cardId);
    const amounts = splitInstallments(input.totalCents, input.installments);
    const basePeriod = invoicePeriodForPurchase(input.date, card.closingDay);
    const many = input.installments > 1;

    const legs: CardExpenseLeg[] = amounts.map((amountCents, index) => ({
      description: many
        ? `${input.description} (${index + 1}/${input.installments})`
        : input.description,
      amountCents,
      // Cada parcela é datada no mês da sua fatura (afeta orçamentos e fluxo de caixa).
      date: addMonthsToDateString(input.date, index),
      categoryId: input.categoryId ?? null,
      notes: input.notes ?? null,
      installmentNumber: many ? index + 1 : null,
      installmentTotal: many ? input.installments : null,
      invoicePeriod: addMonthsToPeriod(basePeriod, index),
    }));

    const created = await this.repository.createCardExpense(cardId, legs);
    if (input.categoryId) {
      this.notifications
        .notifyBudgetThresholdCrossed(input.categoryId, amounts[0] ?? 0)
        .catch(() => undefined);
    }
    return { created: created.length };
  }

  async listInvoices(cardId: string): Promise<Invoice[]> {
    const card = await this.requireCard(cardId);
    const invoices = await this.repository.findInvoicesWithTotals(cardId);
    return invoices.map((i) => this.toInvoiceContract(i, card));
  }

  async listInvoiceItems(cardId: string, invoiceId: string): Promise<Transaction[]> {
    await this.requireInvoice(cardId, invoiceId);
    const items = await this.repository.findInvoiceItems(invoiceId);
    return items.map((t) => ({
      id: t.id,
      type: t.type,
      description: t.description,
      amountCents: t.amountCents,
      date: t.date.toISOString().slice(0, 10),
      status: t.status,
      accountId: null,
      accountName: null,
      cardId: t.cardId,
      cardName: t.card?.name ?? null,
      categoryId: t.categoryId,
      categoryName: t.category?.name ?? null,
      categoryIcon: t.category?.icon ?? null,
      categoryColor: t.category?.color ?? null,
      transferGroupId: null,
      installmentNumber: t.installmentNumber,
      installmentTotal: t.installmentTotal,
      invoiceId: t.invoiceId,
      notes: t.notes,
    }));
  }

  /** Pagamento de fatura: débito na conta + fatura PAID, atômico. */
  async payInvoice(cardId: string, invoiceId: string, input: PayInvoiceInput): Promise<Invoice> {
    const card = await this.requireCard(cardId);
    const invoice = await this.requireInvoice(cardId, invoiceId);
    if (invoice.status === "PAID") {
      throw new ConflictException("fatura já está paga");
    }
    const invoices = await this.repository.findInvoicesWithTotals(cardId);
    const withTotal = invoices.find((i) => i.id === invoiceId);
    const totalCents = withTotal?.totalCents ?? 0;
    if (totalCents <= 0) {
      throw new ConflictException("fatura sem valor a pagar");
    }
    await this.repository.payInvoice({
      invoiceId,
      accountId: input.accountId,
      date: input.date ?? todayString(),
      description: `Pagamento fatura ${card.name} ${String(invoice.month).padStart(2, "0")}/${invoice.year} — ${formatBRL(totalCents)}`,
      totalCents,
    });
    const refreshed = await this.repository.findInvoicesWithTotals(cardId);
    const updated = refreshed.find((i) => i.id === invoiceId);
    if (!updated) throw new NotFoundException("fatura não encontrada após pagamento");
    return this.toInvoiceContract(updated, card);
  }

  private async requireCard(id: string): Promise<DbCard> {
    const card = await this.repository.findById(id);
    if (!card) throw new NotFoundException("cartão não encontrado");
    return card;
  }

  private async requireInvoice(cardId: string, invoiceId: string): Promise<DbInvoice> {
    const invoice = await this.repository.findInvoice(invoiceId);
    if (!invoice || invoice.cardId !== cardId) {
      throw new NotFoundException("fatura não encontrada para este cartão");
    }
    return invoice;
  }
}
