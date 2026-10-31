import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountsService } from "../accounts/accounts.service";
import { CategoriesService } from "../categories/categories.service";
import { DashboardService } from "../dashboard/dashboard.service";
import { TelegramApiService } from "../notifications/telegram-api.service";
import { TransactionsService } from "../transactions/transactions.service";
import { TelegramService } from "./telegram.service";

const account = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "Nubank",
  type: "CHECKING" as const,
  initialBalanceCents: 0,
  balanceCents: 123_456,
  color: null,
  archived: false,
};

function buildService() {
  const telegramApi = {
    defaultChatId: "42",
    isConfigured: true,
    sendMessage: vi.fn().mockResolvedValue(true),
  };
  const accounts = { list: vi.fn().mockResolvedValue([account]) };
  const transactions = {
    create: vi.fn().mockResolvedValue({}),
    list: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 10 }),
  };
  const categories = {
    findByNameOrNull: vi.fn().mockResolvedValue({
      id: "22222222-2222-4222-8222-222222222222",
      name: "Alimentação",
      kind: "EXPENSE",
      icon: "utensils",
      color: "#f43f5e",
      parentId: null,
    }),
  };
  const dashboard = { summary: vi.fn(), categories: vi.fn() };

  const service = new TelegramService(
    telegramApi as unknown as TelegramApiService,
    accounts as unknown as AccountsService,
    transactions as unknown as TransactionsService,
    categories as unknown as CategoriesService,
    dashboard as unknown as DashboardService,
  );
  return { service, telegramApi, accounts, transactions, categories };
}

function update(text: string, chatId = 42) {
  return { update_id: 1, message: { message_id: 1, text, chat: { id: chatId } } };
}

describe("TelegramService", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("ignora mensagens de chats não vinculados", async () => {
    const { service, telegramApi, transactions } = buildService();
    await service.handleUpdate(update("/saldo", 999));
    expect(telegramApi.sendMessage).not.toHaveBeenCalled();
    expect(transactions.create).not.toHaveBeenCalled();
  });

  it("responde /saldo com o total das contas", async () => {
    const { service, telegramApi } = buildService();
    await service.handleUpdate(update("/saldo"));
    expect(telegramApi.sendMessage).toHaveBeenCalledOnce();
    const [reply, chatId] = telegramApi.sendMessage.mock.calls[0] ?? [];
    expect(String(reply)).toContain("Nubank");
    expect(String(reply)).toContain("R$");
    expect(chatId).toBe("42");
  });

  it("lança despesa rápida com categoria resolvida por hashtag", async () => {
    const { service, transactions, categories } = buildService();
    await service.handleUpdate(update("45,90 almoço #alimentacao"));
    expect(categories.findByNameOrNull).toHaveBeenCalledWith("alimentacao", "EXPENSE");
    expect(transactions.create).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "EXPENSE",
        amountCents: 4590,
        description: "almoço",
        accountId: account.id,
        categoryId: "22222222-2222-4222-8222-222222222222",
      }),
    );
  });

  it("responde fallback para mensagem não compreendida", async () => {
    const { service, telegramApi } = buildService();
    await service.handleUpdate(update("bom dia"));
    expect(String(telegramApi.sendMessage.mock.calls[0]?.[0])).toContain("/ajuda");
  });
});
