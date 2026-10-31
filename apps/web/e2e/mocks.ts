import type { Page, Route } from "@playwright/test";

const uuid = (n: number): string => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;

export const ids = {
  accountNubank: uuid(1),
  accountPoupanca: uuid(2),
  card: uuid(3),
  catFood: uuid(4),
  catHome: uuid(5),
  budget: uuid(6),
  goal: uuid(7),
  txLunch: uuid(8),
  txSalary: uuid(9),
  txPending: uuid(10),
};

export const fixtures = {
  summary: {
    month: 10,
    year: 2026,
    totalBalanceCents: 1_234_567,
    monthIncomeCents: 985_000,
    monthExpenseCents: 431_250,
    monthNetCents: 553_750,
    openInvoicesCents: 73_380,
    pendingCount: 2,
  },
  cashflow: [
    { date: "2026-10-05", incomeCents: 985_000, expenseCents: 0 },
    { date: "2026-10-10", incomeCents: 0, expenseCents: 265_000 },
    { date: "2026-10-18", incomeCents: 0, expenseCents: 86_250 },
    { date: "2026-10-26", incomeCents: 0, expenseCents: 80_000 },
  ],
  categorySlices: [
    {
      categoryId: ids.catHome,
      name: "Moradia",
      icon: "house",
      color: "#38bdf8",
      totalCents: 265_000,
      percent: 61.4,
    },
    {
      categoryId: ids.catFood,
      name: "Alimentação",
      icon: "utensils",
      color: "#fb923c",
      totalCents: 166_250,
      percent: 38.6,
    },
  ],
  upcoming: [
    {
      id: uuid(20),
      kind: "TRANSACTION",
      description: "Fatura internet (vence)",
      dueDate: "2026-11-02",
      amountCents: -11_990,
      categoryIcon: "repeat",
    },
    {
      id: uuid(21),
      kind: "INVOICE",
      description: "Fatura Nubank Ultravioleta",
      dueDate: "2026-11-03",
      amountCents: -73_380,
      categoryIcon: "credit-card",
    },
  ],
  accounts: [
    {
      id: ids.accountNubank,
      name: "Nubank",
      type: "CHECKING",
      initialBalanceCents: 0,
      balanceCents: 834_567,
      color: "#8a05be",
      archived: false,
    },
    {
      id: ids.accountPoupanca,
      name: "Poupança",
      type: "SAVINGS",
      initialBalanceCents: 0,
      balanceCents: 400_000,
      color: "#10b981",
      archived: false,
    },
  ],
  cards: [
    {
      id: ids.card,
      name: "Nubank Ultravioleta",
      brand: "MASTERCARD",
      limitCents: 1_200_000,
      closingDay: 3,
      dueDay: 10,
      color: "#7c3aed",
      archived: false,
      usedLimitCents: 388_380,
      availableLimitCents: 811_620,
      currentInvoice: {
        id: uuid(30),
        cardId: ids.card,
        month: 11,
        year: 2026,
        status: "OPEN",
        totalCents: 73_380,
        closingDate: "2026-11-03",
        dueDate: "2026-11-10",
        paidAt: null,
      },
    },
  ],
  categories: [
    {
      id: ids.catFood,
      name: "Alimentação",
      kind: "EXPENSE",
      icon: "utensils",
      color: "#fb923c",
      parentId: null,
    },
    {
      id: ids.catHome,
      name: "Moradia",
      kind: "EXPENSE",
      icon: "house",
      color: "#38bdf8",
      parentId: null,
    },
  ],
  transactions: {
    items: [
      {
        id: ids.txLunch,
        type: "EXPENSE",
        description: "Almoço — rodízio japonês",
        amountCents: -12_900,
        date: "2026-10-28",
        status: "CONFIRMED",
        accountId: ids.accountNubank,
        accountName: "Nubank",
        cardId: null,
        cardName: null,
        categoryId: ids.catFood,
        categoryName: "Alimentação",
        categoryIcon: "utensils",
        categoryColor: "#fb923c",
        transferGroupId: null,
        installmentNumber: null,
        installmentTotal: null,
        invoiceId: null,
        notes: null,
      },
      {
        id: ids.txSalary,
        type: "INCOME",
        description: "Salário",
        amountCents: 985_000,
        date: "2026-10-05",
        status: "CONFIRMED",
        accountId: ids.accountNubank,
        accountName: "Nubank",
        cardId: null,
        cardName: null,
        categoryId: null,
        categoryName: null,
        categoryIcon: null,
        categoryColor: null,
        transferGroupId: null,
        installmentNumber: null,
        installmentTotal: null,
        invoiceId: null,
        notes: null,
      },
      {
        id: ids.txPending,
        type: "EXPENSE",
        description: "IPVA parcela 2/3",
        amountCents: -48_700,
        date: "2026-11-04",
        status: "PENDING",
        accountId: ids.accountNubank,
        accountName: "Nubank",
        cardId: null,
        cardName: null,
        categoryId: null,
        categoryName: null,
        categoryIcon: null,
        categoryColor: null,
        transferGroupId: null,
        installmentNumber: null,
        installmentTotal: null,
        invoiceId: null,
        notes: null,
      },
    ],
    total: 3,
    page: 1,
    pageSize: 50,
  },
  budgets: [
    {
      id: ids.budget,
      categoryId: ids.catFood,
      categoryName: "Alimentação",
      categoryIcon: "utensils",
      categoryColor: "#fb923c",
      amountCents: 60_000,
      spentCents: 55_200,
      percent: 92,
      status: "DANGER",
    },
  ],
  goals: [
    {
      id: ids.goal,
      name: "Reserva de emergência",
      icon: "shield",
      color: "#10b981",
      targetCents: 3_000_000,
      savedCents: 1_050_000,
      deadline: "2027-12-01",
      percent: 35,
      monthlyNeededCents: 150_000,
    },
  ],
};

export interface CapturedRequest {
  method: string;
  url: string;
  body: unknown;
}

/** Intercepta a API inteira com fixtures determinísticas e captura escritas. */
export async function mockApi(page: Page): Promise<CapturedRequest[]> {
  const captured: CapturedRequest[] = [];

  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

  await page.route("**/api/**", (route) => {
    const request = route.request();
    const method = request.method();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/api/, "");

    if (method !== "GET") {
      captured.push({ method, url: url.toString(), body: request.postDataJSON() });
    }

    if (method === "GET") {
      if (path === "/dashboard/summary") return json(route, fixtures.summary);
      if (path === "/dashboard/cashflow") return json(route, fixtures.cashflow);
      if (path === "/dashboard/categories") return json(route, fixtures.categorySlices);
      if (path === "/dashboard/upcoming") return json(route, fixtures.upcoming);
      if (path === "/accounts") return json(route, fixtures.accounts);
      if (path === "/cards") return json(route, fixtures.cards);
      if (path === "/categories") return json(route, fixtures.categories);
      if (path === "/transactions") {
        const search = url.searchParams.get("search")?.toLowerCase();
        const items = search
          ? fixtures.transactions.items.filter((t) => t.description.toLowerCase().includes(search))
          : fixtures.transactions.items;
        return json(route, { ...fixtures.transactions, items, total: items.length });
      }
      if (path === "/budgets") return json(route, fixtures.budgets);
      if (path === "/goals") return json(route, fixtures.goals);
      if (path.startsWith("/cards/") && path.endsWith("/invoices"))
        return json(route, [fixtures.cards[0]!.currentInvoice]);
    }

    if (method === "POST" && path === "/transactions")
      return json(route, { ...fixtures.transactions.items[0], id: "created" }, 201);
    if (method === "POST" && path === "/transactions/transfer")
      return json(route, { transferGroupId: "g" }, 201);
    if (method === "POST" && /\/cards\/.+\/expenses$/.test(path))
      return json(route, { created: 1 }, 201);
    if (method === "PATCH" && /\/transactions\/.+\/confirm$/.test(path))
      return json(route, { ...fixtures.transactions.items[2], status: "CONFIRMED" });
    if (method === "POST" && path === "/budgets") return json(route, fixtures.budgets[0], 201);
    if (method === "POST" && path === "/goals") return json(route, fixtures.goals[0], 201);
    if (method === "POST" && /\/goals\/.+\/contributions$/.test(path))
      return json(route, fixtures.goals[0], 201);
    if (method === "DELETE") return route.fulfill({ status: 204 });

    return json(route, {}, 200);
  });

  return captured;
}
