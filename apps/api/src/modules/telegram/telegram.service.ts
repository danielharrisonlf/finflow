import type { TelegramUpdate } from "@finflow/contracts";
import { Injectable, Logger } from "@nestjs/common";
import { todayString } from "../../common/date.util";
import { formatBRL } from "../../common/money.util";
import { AccountsService } from "../accounts/accounts.service";
import { CategoriesService } from "../categories/categories.service";
import { DashboardService } from "../dashboard/dashboard.service";
import { TelegramApiService } from "../notifications/telegram-api.service";
import { TransactionsService } from "../transactions/transactions.service";
import { parseTelegramMessage } from "./parser";

const HELP_TEXT = [
  "<b>FinFlow Bot</b> — seu controle financeiro no Telegram",
  "",
  "<b>Comandos</b>",
  "/saldo — saldo das contas",
  "/resumo — receitas, despesas e balanço do mês",
  "/extrato — últimos lançamentos",
  "/ajuda — esta mensagem",
  "",
  "<b>Lançamento rápido de despesa</b>",
  "Envie: <code>45,90 almoço #alimentacao</code>",
  "(valor + descrição + categoria opcional via #hashtag)",
].join("\n");

@Injectable()
export class TelegramService {
  private readonly logger = new Logger(TelegramService.name);

  constructor(
    private readonly telegramApi: TelegramApiService,
    private readonly accounts: AccountsService,
    private readonly transactions: TransactionsService,
    private readonly categories: CategoriesService,
    private readonly dashboard: DashboardService,
  ) {}

  async handleUpdate(update: TelegramUpdate): Promise<void> {
    const message = update.message;
    if (!message?.text) return;

    const chatId = String(message.chat.id);
    const linkedChatId = this.telegramApi.defaultChatId;

    // Sem chat vinculado: responde com o id para facilitar a configuração.
    if (!linkedChatId) {
      await this.telegramApi.sendMessage(
        `Chat ainda não vinculado. Defina <code>TELEGRAM_CHAT_ID=${chatId}</code> no servidor para ativar o bot.`,
        chatId,
      );
      return;
    }
    // Ignora silenciosamente mensagens de qualquer outro chat.
    if (chatId !== linkedChatId) {
      this.logger.warn(`update ignorado de chat não vinculado: ${chatId}`);
      return;
    }

    const reply = await this.buildReply(message.text);
    await this.telegramApi.sendMessage(reply, chatId);
  }

  /** Resolve o texto de resposta para uma mensagem recebida (puro de I/O do Telegram). */
  async buildReply(text: string): Promise<string> {
    const parsed = parseTelegramMessage(text);
    switch (parsed.kind) {
      case "help":
        return HELP_TEXT;
      case "balance":
        return this.balanceReply();
      case "summary":
        return this.summaryReply();
      case "statement":
        return this.statementReply();
      case "expense":
        return this.expenseReply(parsed.amountCents, parsed.description, parsed.categoryName);
      default:
        return "Não entendi 🤔 — envie /ajuda para ver os comandos e o formato de lançamento rápido.";
    }
  }

  private async balanceReply(): Promise<string> {
    const accounts = await this.accounts.list();
    if (accounts.length === 0) return "Nenhuma conta cadastrada ainda.";
    const total = accounts.reduce((acc, a) => acc + a.balanceCents, 0);
    const lines = accounts.map((a) => `• ${a.name}: <b>${formatBRL(a.balanceCents)}</b>`);
    return ["<b>💰 Saldo das contas</b>", ...lines, "", `Total: <b>${formatBRL(total)}</b>`].join(
      "\n",
    );
  }

  private async summaryReply(): Promise<string> {
    const [summary, categories] = await Promise.all([
      this.dashboard.summary({}),
      this.dashboard.categories({}),
    ]);
    const month = String(summary.month).padStart(2, "0");
    const top = categories
      .slice(0, 3)
      .map((c) => `• ${c.name}: ${formatBRL(c.totalCents)} (${c.percent}%)`);
    return [
      `<b>📊 Resumo ${month}/${summary.year}</b>`,
      `Receitas: <b>${formatBRL(summary.monthIncomeCents)}</b>`,
      `Despesas: <b>${formatBRL(summary.monthExpenseCents)}</b>`,
      `Balanço: <b>${formatBRL(summary.monthNetCents)}</b>`,
      `Faturas em aberto: ${formatBRL(summary.openInvoicesCents)}`,
      ...(top.length > 0 ? ["", "<b>Maiores gastos</b>", ...top] : []),
    ].join("\n");
  }

  private async statementReply(): Promise<string> {
    const { items } = await this.transactions.list({ page: 1, pageSize: 10 });
    if (items.length === 0) return "Nenhum lançamento ainda.";
    const lines = items.map((t) => {
      const arrow = t.amountCents >= 0 ? "🟢" : "🔴";
      return `${arrow} ${t.date.slice(8, 10)}/${t.date.slice(5, 7)} ${t.description} — <b>${formatBRL(t.amountCents)}</b>`;
    });
    return ["<b>🧾 Últimos lançamentos</b>", ...lines].join("\n");
  }

  private async expenseReply(
    amountCents: number,
    description: string,
    categoryName: string | null,
  ): Promise<string> {
    const accounts = await this.accounts.list();
    const target = accounts.find((a) => a.type === "CHECKING") ?? accounts[0];
    if (!target) {
      return "Nenhuma conta cadastrada — crie uma conta no app antes de lançar despesas.";
    }

    let categoryId: string | null = null;
    let categoryNote = "";
    if (categoryName) {
      const category = await this.categories.findByNameOrNull(categoryName, "EXPENSE");
      if (category) {
        categoryId = category.id;
        categoryNote = ` em <b>${category.name}</b>`;
      } else {
        categoryNote = ` (categoria "${categoryName}" não encontrada — ficou sem categoria)`;
      }
    }

    await this.transactions.create({
      type: "EXPENSE",
      description,
      amountCents,
      date: todayString(),
      status: "CONFIRMED",
      accountId: target.id,
      categoryId,
      notes: "lançado via Telegram",
    });

    return `✅ Despesa de <b>${formatBRL(amountCents)}</b> (${description})${categoryNote} lançada na conta <b>${target.name}</b>.`;
  }
}
