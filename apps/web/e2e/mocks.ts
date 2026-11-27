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
  recurringRent: uuid(11),
  recurringSalary: uuid(12),
  rule: uuid(13),
  wishA: uuid(14),
  wishB: uuid(15),
  catSalary: uuid(16),
};

/** Saldo projetado: 40 dias passando por todas as faixas e ficando negativo. */
function projectionDays() {
  const start = Date.UTC(2026, 10, 27);
  const balances = [
    250_000, 240_000, 180_000, 150_000, 120_000, 90_000, 60_000, 30_000, 10_000, -20_000, -60_000,
    -80_000, -40_000, 20_000, 400_000, 380_000, 360_000, 330_000, 300_000, 280_000,
  ];
  const band = (b: number) =>
    b >= 200_000
      ? "DARK_GREEN"
      : b >= 100_000
        ? "LIGHT_GREEN"
        : b >= 0
          ? "YELLOW"
          : b >= -50_000
            ? "LIGHT_RED"
            : "RED";
  return balances.map((balanceCents, i) => ({
    date: new Date(start + i * 86_400_000).toISOString().slice(0, 10),
    inCents: i === 14 ? 985_000 : 0,
    outCents: i === 14 ? 0 : 10_000,
    balanceCents,
    band: band(balanceCents),
    events:
      i === 14
        ? [{ kind: "RECURRING", description: "Salário", amountCents: 985_000 }]
        : i === 9
          ? [{ kind: "INVOICE", description: "Fatura Nubank Ultravioleta", amountCents: -73_380 }]
          : [],
  }));
}

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
      kind: "GOAL",
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
  overview: {
    month: 11,
    year: 2026,
    pace: {
      currentCents: 431_250,
      previousSamePeriodCents: 375_000,
      diffCents: 56_250,
      dayOfMonth: 27,
      status: "ABOVE",
    },
    monthlyLimit: {
      limitCents: 500_000,
      spentCents: 431_250,
      remainingCents: 68_750,
      dailyAllowanceCents: 17_187,
      percent: 86.3,
    },
    performance: {
      incomeCents: 985_000,
      fixedCents: 265_000,
      variableCents: 166_250,
      totalOutCents: 431_250,
      savedCents: 0,
      performanceCents: -12_000,
      savingsRatePct: 0,
      previousPerformanceCents: 120_000,
      verdict: "NEGATIVE",
      message: "As saídas passaram das entradas e nada foi guardado neste mês.",
    },
    spendTypes: [
      {
        spendType: "FIXED",
        totalCents: 265_000,
        percent: 61.4,
        categories: [
          {
            categoryId: ids.catHome,
            name: "Moradia",
            icon: "house",
            color: "#38bdf8",
            totalCents: 265_000,
          },
        ],
      },
      { spendType: "CUTTABLE", totalCents: 0, percent: 0, categories: [] },
      {
        spendType: "VARIABLE",
        totalCents: 166_250,
        percent: 38.6,
        categories: [
          {
            categoryId: ids.catFood,
            name: "Alimentação",
            icon: "utensils",
            color: "#fb923c",
            totalCents: 166_250,
          },
        ],
      },
    ],
    plan: { fixedPct: 50, cuttablePct: 20, variablePct: 30 },
    netWorthCents: 1_161_187,
    reserve: {
      goalId: null,
      avgMonthlyCostCents: 420_000,
      suggestedTargetCents: 2_520_000,
      targetCents: 2_520_000,
      savedCents: 0,
      monthsCovered: 0,
      status: "NONE",
    },
    cutSuggestions: [
      {
        kind: "ABOVE_AVERAGE",
        categoryId: ids.catFood,
        name: "Delivery da semana",
        currentCents: 166_250,
        averageCents: 120_000,
      },
    ],
  },
  projection: {
    startDate: "2026-11-27",
    startBalanceCents: 1_234_567,
    limitConfigured: false,
    days: projectionDays(),
    lowest: { date: "2026-12-08", balanceCents: -80_000 },
    nextIncomeDate: "2026-12-11",
    canSaveNowCents: 0,
    months: Array.from({ length: 12 }, (_, i) => ({
      month: ((10 + i) % 12) + 1,
      year: 2026 + Math.floor((10 + i) / 12),
      incomeCents: 985_000,
      recurringOutCents: 265_000,
      installmentsCents: 20_000,
      invoicesCents: 73_380,
      dailySpendCents: 0,
      endBalanceCents: 1_234_567 + i * 100_000,
    })),
  },
  year: {
    year: 2026,
    months: Array.from({ length: 12 }, (_, i) => ({
      month: i + 1,
      incomeCents: 985_000,
      fixedCents: 265_000,
      variableCents: 166_250,
      totalOutCents: 431_250,
      performanceCents: 553_750,
      savedCents: 300_000,
      savingsRatePct: 30.5,
      endBalanceCents: 1_000_000 + i * 50_000,
    })),
    totals: {
      incomeCents: 11_820_000,
      fixedCents: 3_180_000,
      variableCents: 1_995_000,
      totalOutCents: 5_175_000,
      performanceCents: 6_645_000,
      savedCents: 3_600_000,
      savingsRatePct: 30.5,
    },
  },
  commitments: {
    months: Array.from({ length: 6 }, (_, i) => ({
      month: ((10 + i) % 12) + 1,
      year: 2026 + Math.floor((10 + i) / 12),
      recurringExpenseCents: 250_000,
      recurringIncomeCents: 985_000,
      installmentsCents: i < 3 ? 40_000 : 0,
    })),
    items: [
      {
        kind: "RECURRING",
        direction: "PAY",
        description: "Aluguel",
        dueDate: "2026-11-10",
        amountCents: 250_000,
        recurringId: ids.recurringRent,
        realized: false,
        installmentNumber: null,
        installmentTotal: null,
        cardName: null,
      },
      {
        kind: "INSTALLMENT",
        direction: "PAY",
        description: "Notebook",
        dueDate: "2026-11-10",
        amountCents: 40_000,
        recurringId: null,
        realized: false,
        installmentNumber: 3,
        installmentTotal: 10,
        cardName: "Nubank Ultravioleta",
      },
      {
        kind: "RECURRING",
        direction: "RECEIVE",
        description: "Salário",
        dueDate: "2026-11-05",
        amountCents: 985_000,
        recurringId: ids.recurringSalary,
        realized: true,
        installmentNumber: null,
        installmentTotal: null,
        cardName: null,
      },
    ],
    totals: { payCents: 290_000, receiveCents: 0, balanceAfterCents: 944_567 },
  },
  recurring: [
    {
      id: ids.recurringRent,
      description: "Aluguel",
      kind: "EXPENSE",
      amountCents: 250_000,
      dayOfMonth: 10,
      accountId: ids.accountNubank,
      accountName: "Nubank",
      categoryId: ids.catHome,
      categoryName: "Moradia",
      categoryIcon: "house",
      categoryColor: "#38bdf8",
      startDate: "2026-01-10",
      endDate: null,
      active: true,
      nextDate: "2026-11-10",
    },
    {
      id: ids.recurringSalary,
      description: "Salário",
      kind: "INCOME",
      amountCents: 985_000,
      dayOfMonth: 5,
      accountId: ids.accountNubank,
      accountName: "Nubank",
      categoryId: null,
      categoryName: null,
      categoryIcon: null,
      categoryColor: null,
      startDate: "2026-01-05",
      endDate: null,
      active: true,
      nextDate: "2026-12-05",
    },
  ],
  settings: {
    monthlyLimitCents: null,
    reserveMonths: 6,
    hasDependents: false,
    reserveRefillRatePct: 10,
    balanceBands: { darkGreen: 200_000, lightGreen: 100_000, yellow: 0, lightRed: -50_000 },
    spendPlan: null,
  },
  rules: [
    {
      id: ids.rule,
      pattern: "ifood",
      categoryId: ids.catFood,
      categoryName: "Alimentação",
      priority: 0,
    },
  ],
  wishlist: [
    {
      id: ids.wishA,
      name: "Fone com cancelamento de ruído",
      priceCents: 89_990,
      url: null,
      notes: null,
      priority: 0,
      status: "LISTED",
      affordableOn: "2026-12-12",
    },
    {
      id: ids.wishB,
      name: "Bicicleta elétrica",
      priceCents: 1_200_000,
      url: null,
      notes: null,
      priority: 1,
      status: "LISTED",
      affordableOn: null,
    },
  ],
  importPreview: {
    format: "CSV",
    sheets: [],
    sheet: null,
    columns: ["Data", "Histórico", "Valor"],
    mapping: {
      date: 0,
      description: 1,
      amount: 2,
      debit: null,
      credit: null,
      type: null,
      category: null,
      account: null,
    },
    rows: [
      {
        index: 0,
        date: "2026-11-03",
        description: "IFOOD *RESTAURANTE",
        amountCents: -4_590,
        kind: "EXPENSE",
        categoryName: null,
        suggestedCategoryId: ids.catFood,
        accountName: null,
        duplicate: false,
        error: null,
      },
      {
        index: 1,
        date: "2026-11-05",
        description: "SALARIO EMPRESA",
        amountCents: 985_000,
        kind: "INCOME",
        categoryName: null,
        suggestedCategoryId: null,
        accountName: null,
        duplicate: false,
        error: null,
      },
      {
        index: 2,
        date: "2026-10-28",
        description: "Almoço — rodízio japonês",
        amountCents: -12_900,
        kind: "EXPENSE",
        categoryName: null,
        suggestedCategoryId: ids.catFood,
        accountName: null,
        duplicate: true,
        error: null,
      },
    ],
    warnings: ["1 linha parece já existir no extrato e foi desmarcada."],
  },
};

export interface CapturedRequest {
  method: string;
  url: string;
  body: unknown;
}

/** Intercepta a API inteira com fixtures determinísticas e captura escritas. */
export async function mockApi(
  page: Page,
  options: { requireAuth?: boolean } = {},
): Promise<CapturedRequest[]> {
  const captured: CapturedRequest[] = [];
  let authenticated = !options.requireAuth;

  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

  await page.route("**/api/**", (route) => {
    const request = route.request();
    const method = request.method();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/api/, "");

    if (method !== "GET") {
      let body: unknown;
      try {
        body = request.postDataJSON();
      } catch {
        // multipart (upload de planilha): guarda o corpo bruto
        body = request.postData();
      }
      captured.push({ method, url: url.toString(), body });
    }

    if (method === "GET") {
      if (path === "/auth/status")
        return json(route, { required: Boolean(options.requireAuth), authenticated });
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
      if (path === "/insights/overview") return json(route, fixtures.overview);
      if (path === "/insights/projection") return json(route, fixtures.projection);
      if (path === "/insights/year") return json(route, fixtures.year);
      if (path === "/insights/commitments") return json(route, fixtures.commitments);
      if (path === "/recurring") return json(route, fixtures.recurring);
      if (path === "/settings") return json(route, fixtures.settings);
      if (path === "/rules") return json(route, fixtures.rules);
      if (path === "/wishlist") return json(route, fixtures.wishlist);
      if (path.startsWith("/cards/") && path.endsWith("/invoices"))
        return json(route, [fixtures.cards[0]!.currentInvoice]);
    }

    if (method === "POST" && path === "/auth/login") {
      const payload = request.postDataJSON() as { password?: string };
      if (payload.password === "errada") {
        return json(route, { message: "senha incorreta" }, 401);
      }
      authenticated = true;
      return json(route, { token: "mock-jwt-token", expiresAt: "2027-01-01T00:00:00.000Z" }, 200);
    }
    if (method === "POST" && path === "/categories")
      return json(route, { ...fixtures.categories[0], id: "cat-new" }, 201);
    if (method === "PATCH" && /^\/categories\/[^/]+$/.test(path))
      return json(route, fixtures.categories[0]);
    if (method === "POST" && path === "/transactions")
      return json(route, { ...fixtures.transactions.items[0], id: "created" }, 201);
    if (method === "POST" && path === "/transactions/transfer")
      return json(route, { transferGroupId: "g" }, 201);
    if (method === "POST" && /\/cards\/.+\/expenses$/.test(path))
      return json(route, { created: 1 }, 201);
    if (method === "PATCH" && /\/transactions\/.+\/confirm$/.test(path))
      return json(route, { ...fixtures.transactions.items[2], status: "CONFIRMED" });
    if (method === "PATCH" && /^\/transactions\/[^/]+$/.test(path))
      return json(route, fixtures.transactions.items[0]);
    if (method === "POST" && path === "/budgets") return json(route, fixtures.budgets[0], 201);
    if (method === "POST" && path === "/goals") return json(route, fixtures.goals[0], 201);
    if (method === "POST" && /\/goals\/.+\/contributions$/.test(path))
      return json(route, fixtures.goals[0], 201);
    if (method === "POST" && /^\/recurring\/[^/]+\/post$/.test(path))
      return json(route, { ...fixtures.transactions.items[0], id: "posted" }, 201);
    if (method === "POST" && path === "/recurring") return json(route, fixtures.recurring[0], 201);
    if (method === "PATCH" && /^\/recurring\/[^/]+$/.test(path))
      return json(route, fixtures.recurring[0]);
    if (method === "PATCH" && path === "/settings") {
      return json(route, { ...fixtures.settings, ...(request.postDataJSON() as object) });
    }
    if (method === "POST" && path === "/rules") return json(route, fixtures.rules[0], 201);
    if (method === "POST" && path === "/rules/apply") return json(route, { updated: 4 });
    if (method === "POST" && path === "/wishlist") return json(route, fixtures.wishlist[0], 201);
    if (method === "PATCH" && /^\/wishlist\/[^/]+$/.test(path))
      return json(route, fixtures.wishlist[0]);
    if (method === "POST" && path === "/import/preview") return json(route, fixtures.importPreview);
    if (method === "POST" && path === "/import/commit") {
      const payload = request.postDataJSON() as { rows: unknown[] };
      return json(route, { created: payload.rows.length, skipped: 0 });
    }
    if (method === "DELETE") return route.fulfill({ status: 204 });

    return json(route, {}, 200);
  });

  return captured;
}
