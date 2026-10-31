import type {
  CreateTransactionInput,
  ListTransactionsQuery,
  Paginated,
  Transaction,
  TransferInput,
  UpdateTransactionInput,
} from "@finflow/contracts";
import {
  BadRequestException,
  ConflictException,
  Injectable,
  NotFoundException,
} from "@nestjs/common";
import { fromDateString, toDateString } from "../../common/date.util";
import { AccountsRepository } from "../accounts/accounts.repository";
import { NotificationsService } from "../notifications/notifications.service";
import { TransactionsRepository, TransactionWithRefs } from "./transactions.repository";

export function toContract(db: TransactionWithRefs): Transaction {
  return {
    id: db.id,
    type: db.type,
    description: db.description,
    amountCents: db.amountCents,
    date: toDateString(db.date),
    status: db.status,
    accountId: db.accountId,
    accountName: db.account?.name ?? null,
    cardId: db.cardId,
    cardName: db.card?.name ?? null,
    categoryId: db.categoryId,
    categoryName: db.category?.name ?? null,
    categoryIcon: db.category?.icon ?? null,
    categoryColor: db.category?.color ?? null,
    transferGroupId: db.transferGroupId,
    installmentNumber: db.installmentNumber,
    installmentTotal: db.installmentTotal,
    invoiceId: db.invoiceId,
    notes: db.notes,
  };
}

/** Receita entra positiva; despesa, negativa. */
export function signedAmount(type: "INCOME" | "EXPENSE", amountCents: number): number {
  return type === "INCOME" ? Math.abs(amountCents) : -Math.abs(amountCents);
}

@Injectable()
export class TransactionsService {
  constructor(
    private readonly repository: TransactionsRepository,
    private readonly accounts: AccountsRepository,
    private readonly notifications: NotificationsService,
  ) {}

  async list(query: ListTransactionsQuery): Promise<Paginated<Transaction>> {
    const { items, total } = await this.repository.list(query);
    return {
      items: items.map(toContract),
      total,
      page: query.page,
      pageSize: query.pageSize,
    };
  }

  async findById(id: string): Promise<Transaction> {
    const found = await this.repository.findById(id);
    if (!found) throw new NotFoundException("transação não encontrada");
    return toContract(found);
  }

  async create(input: CreateTransactionInput): Promise<Transaction> {
    await this.requireAccount(input.accountId);
    const created = await this.repository.create({
      type: input.type,
      description: input.description,
      amountCents: signedAmount(input.type, input.amountCents),
      date: fromDateString(input.date),
      status: input.status,
      accountId: input.accountId,
      categoryId: input.categoryId ?? null,
      notes: input.notes ?? null,
    });
    if (input.type === "EXPENSE" && input.categoryId) {
      this.notifications
        .notifyBudgetThresholdCrossed(input.categoryId, input.amountCents)
        .catch(() => undefined);
    }
    return toContract(created);
  }

  async update(id: string, input: UpdateTransactionInput): Promise<Transaction> {
    const existing = await this.repository.findById(id);
    if (!existing) throw new NotFoundException("transação não encontrada");

    const isSimple = existing.type === "INCOME" || existing.type === "EXPENSE";
    if (!isSimple && (input.amountCents !== undefined || input.accountId !== undefined)) {
      throw new ConflictException(
        "valor e conta de transferências, compras no cartão e pagamentos de fatura não podem ser editados — exclua e lance novamente",
      );
    }
    if (input.accountId) await this.requireAccount(input.accountId);

    const updated = await this.repository.update(id, {
      description: input.description,
      amountCents:
        input.amountCents !== undefined && isSimple
          ? signedAmount(existing.type as "INCOME" | "EXPENSE", input.amountCents)
          : undefined,
      date: input.date ? fromDateString(input.date) : undefined,
      status: input.status,
      accountId: input.accountId,
      categoryId: input.categoryId,
      notes: input.notes,
    });
    return toContract(updated);
  }

  /** Marca uma transação pendente como confirmada (paga/recebida). */
  async confirm(id: string): Promise<Transaction> {
    const existing = await this.repository.findById(id);
    if (!existing) throw new NotFoundException("transação não encontrada");
    if (existing.status === "CONFIRMED") return toContract(existing);
    const updated = await this.repository.update(id, { status: "CONFIRMED" });
    return toContract(updated);
  }

  /**
   * Exclui a transação respeitando invariantes:
   * transferências caem em par; pagamento de fatura reabre a fatura;
   * parcelamentos aceitam escopo "single" ou "group".
   */
  async delete(id: string, scope: "single" | "group" = "single"): Promise<void> {
    const existing = await this.repository.findById(id);
    if (!existing) throw new NotFoundException("transação não encontrada");

    if (existing.transferGroupId) {
      await this.repository.deleteTransferGroup(existing.transferGroupId);
      return;
    }
    if (existing.type === "INVOICE_PAYMENT") {
      const invoice = await this.repository.findInvoicePaidBy(id);
      if (invoice) {
        await this.repository.deleteInvoicePayment(id, invoice.id);
        return;
      }
    }
    if (scope === "group" && existing.installmentGroupId) {
      await this.repository.deleteInstallmentGroup(existing.installmentGroupId);
      return;
    }
    await this.repository.delete(id);
  }

  /** Transferência entre contas próprias: débito e crédito atômicos. */
  async transfer(input: TransferInput): Promise<{ transferGroupId: string }> {
    const [from, to] = await Promise.all([
      this.accounts.findById(input.fromAccountId),
      this.accounts.findById(input.toAccountId),
    ]);
    if (!from) throw new NotFoundException("conta de origem não encontrada");
    if (!to) throw new NotFoundException("conta de destino não encontrada");
    if (from.archived || to.archived) {
      throw new BadRequestException("não é possível transferir usando contas arquivadas");
    }
    const base = input.description?.trim();
    return this.repository.createTransferPair({
      fromAccountId: input.fromAccountId,
      toAccountId: input.toAccountId,
      amountCents: input.amountCents,
      date: input.date,
      fromDescription: base ? `${base} → ${to.name}` : `Transferência para ${to.name}`,
      toDescription: base ? `${base} ← ${from.name}` : `Transferência de ${from.name}`,
    });
  }

  private async requireAccount(accountId: string): Promise<void> {
    const account = await this.accounts.findById(accountId);
    if (!account) throw new NotFoundException("conta não encontrada");
    if (account.archived) throw new BadRequestException("conta está arquivada");
  }
}
