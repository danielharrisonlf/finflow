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
  tagTrip: uuid(40),
  tagWork: uuid(41),
  txTransferOut: uuid(42),
  txTransferIn: uuid(43),
  investCdb: uuid(44),
  investStocks: uuid(45),
  loanCar: uuid(46),
  loanPersonal: uuid(47),
  invoice: uuid(30),
  memberAna: uuid(60),
  memberBruno: uuid(61),
};

// ───────────────────────── v3 fase 2: espaços ─────────────────────────

export const spaceFixtures = {
  spaces: [
    {
      slug: "pessoal",
      name: "Pessoal",
      kind: "PERSONAL",
      color: "#34d399",
      createdAt: "2026-01-01T00:00:00Z",
      isDefault: true,
    },
    {
      slug: "casa",
      name: "Casa",
      kind: "HOME",
      color: "#38bdf8",
      createdAt: "2026-11-01T00:00:00Z",
      isDefault: false,
    },
    {
      slug: "empresa",
      name: "Minha empresa",
      kind: "BUSINESS",
      color: "#a78bfa",
      createdAt: "2026-11-02T00:00:00Z",
      isDefault: false,
    },
  ],
  /** Saldo geral do dashboard por espaço (prova que a troca refaz as consultas). */
  balanceBySpace: { pessoal: 1_234_567, casa: 456_789, empresa: 3_040_000 } as Record<
    string,
    number
  >,
  overview: {
    spaces: [
      {
        slug: "pessoal",
        name: "Pessoal",
        kind: "PERSONAL",
        balanceCents: 1_234_567,
        netWorthCents: 4_100_000,
        monthIncomeCents: 985_000,
        monthOutCents: 431_250,
        monthResultCents: 553_750,
      },
      {
        slug: "casa",
        name: "Casa",
        kind: "HOME",
        balanceCents: 456_789,
        netWorthCents: 456_789,
        monthIncomeCents: 0,
        monthOutCents: 300_000,
        monthResultCents: -300_000,
      },
      {
        slug: "empresa",
        name: "Minha empresa",
        kind: "BUSINESS",
        balanceCents: 3_040_000,
        netWorthCents: 3_040_000,
        monthIncomeCents: 1_400_000,
        monthOutCents: 1_720_000,
        monthResultCents: -320_000,
      },
    ],
    totals: {
      balanceCents: 4_731_356,
      netWorthCents: 7_596_789,
      monthIncomeCents: 2_385_000,
      monthOutCents: 2_451_250,
      monthResultCents: -66_250,
    },
  },
  members: [
    { id: ids.memberAna, name: "Ana", color: "#38bdf8" },
    { id: ids.memberBruno, name: "Bruno", color: "#f472b6" },
  ],
  balances: {
    members: [
      {
        memberId: ids.memberAna,
        name: "Ana",
        paidCents: 300_000,
        owedCents: 150_000,
        balanceCents: 150_000,
      },
      {
        memberId: ids.memberBruno,
        name: "Bruno",
        paidCents: 0,
        owedCents: 150_000,
        balanceCents: -150_000,
      },
    ],
    settlements: [
      { fromMemberId: ids.memberBruno, toMemberId: ids.memberAna, amountCents: 150_000 },
    ],
  },
  metrics: {
    mrrCents: 1_250_000,
    arrCents: 15_000_000,
    newMrrCents: 150_000,
    churnedMrrCents: 50_000,
    churnPct: 4.2,
    burnCents: 320_000,
    runwayMonths: 9.5,
    cashCents: 3_040_000,
  },
};

/** DRE determinístico: receita cresce 50 mil por mês; resultado negativo no começo do ano. */
export function businessDre() {
  const months = Array.from({ length: 12 }, (_, i) => {
    const revenueCents = 1_000_000 + i * 50_000;
    const deductionsCents = Math.round(revenueCents * 0.06);
    const netRevenueCents = revenueCents - deductionsCents;
    const costsCents = 300_000;
    const grossProfitCents = netRevenueCents - costsCents;
    const opexCents = 800_000;
    const operatingResultCents = grossProfitCents - opexCents;
    const financialCents = -10_000;
    const netResultCents = operatingResultCents + financialCents;
    return {
      month: i + 1,
      revenueCents,
      deductionsCents,
      netRevenueCents,
      costsCents,
      grossProfitCents,
      opexCents,
      operatingResultCents,
      financialCents,
      netResultCents,
      marginPct: Math.round((netResultCents / netRevenueCents) * 1000) / 10,
    };
  });
  const sum = (key: keyof (typeof months)[number]) =>
    months.reduce((acc, m) => acc + (m[key] as number), 0);
  const netRevenueCents = sum("netRevenueCents");
  const netResultCents = sum("netResultCents");
  return {
    months,
    totals: {
      revenueCents: sum("revenueCents"),
      deductionsCents: sum("deductionsCents"),
      netRevenueCents,
      costsCents: sum("costsCents"),
      grossProfitCents: sum("grossProfitCents"),
      opexCents: sum("opexCents"),
      operatingResultCents: sum("operatingResultCents"),
      financialCents: sum("financialCents"),
      netResultCents,
      marginPct: Math.round((netResultCents / netRevenueCents) * 1000) / 10,
    },
  };
}

/** "Minha Empresa" → "minha-empresa" (mesma ideia do servidor). */
function slugify(name: string): string {
  return name
    .toLowerCase()
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

/** Linha de extrato com os campos v3 preenchidos (tags, loanId). */
function tx(over: Record<string, unknown>) {
  return {
    type: "EXPENSE",
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
    recurringId: null,
    loanId: null,
    notes: null,
    tags: [],
    ...over,
  };
}

/** Estratégias de quitação: o extra encurta prazo e juros (valores determinísticos). */
export function debtStrategy(extraCents: number) {
  const cut = Math.min(10, Math.floor(extraCents / 20_000));
  const plan = (months: number, interest: number, payoff: string, order: string[]) => ({
    order,
    months,
    totalInterestCents: interest,
    payoffDate: payoff,
    perLoan: order.map((loanId, i) => ({
      loanId,
      payoffDate: i === 0 ? "2027-08-20" : payoff,
      interestCents: Math.round(interest / (i + 2)),
    })),
  });
  return {
    avalanche: plan(
      36 - cut,
      690_000 - cut * 20_000,
      extraCents > 0 ? "2029-01-15" : "2029-11-15",
      [ids.loanPersonal, ids.loanCar],
    ),
    snowball: plan(36 - cut, 712_000 - cut * 20_000, extraCents > 0 ? "2029-02-15" : "2029-11-15", [
      ids.loanPersonal,
      ids.loanCar,
    ]),
    minimumOnly: plan(36, 766_000, "2029-11-15", [ids.loanCar, ids.loanPersonal]),
  };
}

/** "Posso comprar?": até R$ 1.000 sim, até R$ 5.000 com cuidado, acima não. */
export function canAfford(amountCents: number) {
  if (amountCents > 500_000) {
    return {
      verdict: "NO",
      lowestBalanceAfterCents: -420_000,
      lowestDate: "2026-12-08",
      limit: null,
      reserveMonthsAfter: null,
      fitsOn: "2027-02-11",
      message: "Essa compra deixaria seu saldo negativo em 8 de dezembro.",
    };
  }
  if (amountCents > 100_000) {
    return {
      verdict: "CAREFUL",
      lowestBalanceAfterCents: 20_000,
      lowestDate: "2026-12-08",
      limit: { limitCents: 500_000, spentAfterCents: 581_250, percentAfter: 116.3 },
      reserveMonthsAfter: null,
      fitsOn: null,
      message: "Cabe, mas o saldo fica apertado e o limite do mês estoura.",
    };
  }
  return {
    verdict: "YES",
    lowestBalanceAfterCents: 180_000,
    lowestDate: "2026-12-08",
    limit: null,
    reserveMonthsAfter: null,
    fitsOn: null,
    message: "Pode comprar: o saldo continua confortável.",
  };
}

/** Série acumulada do ritmo: mês atual até o dia 27, anterior completo. */
function paceSeries() {
  return Array.from({ length: 30 }, (_, i) => {
    const day = i + 1;
    return {
      day,
      currentCents: day <= 27 ? Math.round((431_250 / 27) * day) : null,
      previousCents: Math.round((420_000 / 30) * day),
    };
  });
}

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
        tags: [{ id: ids.tagTrip, name: "Viagem", color: "#38bdf8" }],
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
      series: paceSeries(),
      projectedMonthCents: 479_166,
    },
    spendToday: { cents: 17_187, basis: "LIMIT" },
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
      loansCents: 89_000,
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
        kind: "LOAN",
        direction: "PAY",
        description: "Financiamento do carro",
        dueDate: "2026-11-15",
        amountCents: 89_000,
        recurringId: null,
        realized: false,
        installmentNumber: 13,
        installmentTotal: 48,
        cardName: null,
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
  // ───────────── v3 ─────────────
  tags: [
    { id: ids.tagTrip, name: "Viagem", color: "#38bdf8", usageCount: 4 },
    { id: ids.tagWork, name: "Trabalho", color: "#a78bfa", usageCount: 0 },
  ],
  tagSpending: [
    { tagId: ids.tagTrip, name: "Viagem", color: "#38bdf8", totalCents: 152_300, count: 4 },
  ],
  alerts: [
    {
      id: "NEGATIVE_PROJECTION:saldo:2026-11",
      kind: "NEGATIVE_PROJECTION",
      severity: "DANGER",
      title: "Saldo negativo à vista",
      message: "Seu saldo deve chegar a −R$ 800,00 em 8 de dezembro.",
      href: "/projecao",
      date: "2026-11-27",
    },
    {
      id: `BUDGET_DANGER:${ids.budget}:2026-11`,
      kind: "BUDGET_DANGER",
      severity: "WARNING",
      title: "Alimentação em 92% do orçamento",
      message: "Já foram R$ 552,00 de R$ 600,00 neste mês.",
      href: "/orcamentos",
      date: "2026-11-26",
    },
    {
      id: "SUBSCRIPTION_PRICE_UP:streaming:2026-11",
      kind: "SUBSCRIPTION_PRICE_UP",
      severity: "INFO",
      title: "Assinatura mais cara",
      message: "Streaming de vídeo subiu 12% (R$ 44,90 → R$ 49,90).",
      href: "/compromissos",
      date: "2026-11-20",
    },
  ],
  subscriptions: {
    items: [
      {
        key: "streaming video",
        description: "Streaming de vídeo",
        averageCents: 44_900,
        lastCents: 49_900,
        lastDate: "2026-11-12",
        occurrences: 8,
        cadence: "MONTHLY",
        nextExpectedDate: "2026-12-12",
        priceChangePct: 11.1,
        categoryId: null,
        accountId: null,
        cardId: ids.card,
        recurringId: null,
      },
      {
        key: "academia",
        description: "Academia",
        averageCents: 12_000,
        lastCents: 12_000,
        lastDate: "2026-11-05",
        occurrences: 12,
        cadence: "MONTHLY",
        nextExpectedDate: "2026-12-05",
        priceChangePct: null,
        categoryId: null,
        accountId: ids.accountNubank,
        cardId: null,
        recurringId: ids.recurringRent,
      },
    ],
    monthlyTotalCents: 61_900,
  },
  cashflowReport: {
    CASH: {
      basis: "CASH",
      months: [
        [9, 900_000, 250_000, 60_000, 180_000, null],
        [10, 985_000, 250_000, 40_000, 210_000, 7.5],
        [11, 985_000, 339_000, 73_380, 162_000, -12.3],
      ].map(([month, income, fixed, inst, daily, change]) => ({
        month: month as number,
        year: 2026,
        incomeCents: income as number,
        fixedBillsCents: fixed as number,
        installmentsCents: inst as number,
        dailyCents: daily as number,
        totalOutCents: (fixed as number) + (inst as number) + (daily as number),
        resultCents: (income as number) - (fixed as number) - (inst as number) - (daily as number),
        resultChangePct: change as number | null,
      })),
      whereItWent: [
        {
          categoryId: ids.catHome,
          name: "Moradia",
          icon: "house",
          color: "#38bdf8",
          totalCents: 250_000,
          percent: 43.6,
        },
        {
          categoryId: ids.catFood,
          name: "Alimentação",
          icon: "utensils",
          color: "#fb923c",
          totalCents: 162_000,
          percent: 28.2,
        },
      ],
    },
    ACCRUAL: {
      basis: "ACCRUAL",
      months: [9, 10, 11].map((month) => ({
        month,
        year: 2026,
        incomeCents: 985_000,
        fixedBillsCents: 250_000,
        installmentsCents: 40_000,
        dailyCents: 300_000,
        totalOutCents: 590_000,
        resultCents: 395_000,
        resultChangePct: month === 9 ? null : 0,
      })),
      whereItWent: [
        {
          categoryId: null,
          name: "Sem categoria",
          icon: null,
          color: null,
          totalCents: 300_000,
          percent: 50.8,
        },
      ],
    },
  },
  netWorth: Array.from({ length: 12 }, (_, i) => ({
    month: ((11 + i) % 12) + 1,
    year: 2025 + Math.floor((11 + i) / 12),
    cashCents: 900_000 + i * 30_000,
    investmentsCents: 1_500_000 + i * 45_000,
    debtsCents: 4_000_000 - i * 80_000,
    netWorthCents: 900_000 + i * 30_000 + 1_500_000 + i * 45_000 - (4_000_000 - i * 80_000),
  })),
  invoiceBreakdown: {
    purchasesCents: 80_380,
    creditsCents: 7_000,
    paymentsCents: 0,
    totalCents: 73_380,
    composition: { installmentsCents: 40_000, subscriptionsCents: 4_990, oneOffCents: 35_390 },
  },
  transferCandidates: [
    {
      expense: tx({
        id: ids.txTransferOut,
        description: "PIX enviado — Poupança",
        amountCents: -50_000,
        date: "2026-11-20",
      }),
      income: tx({
        id: ids.txTransferIn,
        type: "INCOME",
        description: "PIX recebido",
        amountCents: 50_000,
        date: "2026-11-21",
        accountId: ids.accountPoupanca,
        accountName: "Poupança",
      }),
      daysApart: 1,
    },
  ],
  linkPreview: {
    title: "Cafeteira espresso automática",
    priceCents: 129_900,
    imageUrl: "https://example.com/cafeteira.jpg",
  },
  investments: [
    {
      id: ids.investCdb,
      name: "CDB liquidez diária",
      kind: "FIXED_INCOME",
      institution: "Banco A",
      accountId: ids.accountPoupanca,
      accountName: "Poupança",
      investedCents: 1_000_000,
      currentCents: 1_085_000,
      returnCents: 85_000,
      returnPct: 8.5,
      valueUpdatedAt: "2026-11-20",
      notes: null,
      archived: false,
    },
    {
      id: ids.investStocks,
      name: "Carteira de ações",
      kind: "STOCKS",
      institution: "Corretora B",
      accountId: null,
      accountName: null,
      investedCents: 500_000,
      currentCents: 465_000,
      returnCents: -35_000,
      returnPct: -7,
      valueUpdatedAt: "2026-09-01",
      notes: null,
      archived: false,
    },
  ],
  investmentSummary: {
    totalInvestedCents: 1_500_000,
    totalCurrentCents: 1_550_000,
    returnCents: 50_000,
    returnPct: 3.33,
    byKind: [
      { kind: "FIXED_INCOME", currentCents: 1_085_000, percent: 70 },
      { kind: "STOCKS", currentCents: 465_000, percent: 30 },
    ],
    byInstitution: [
      { institution: "Banco A", currentCents: 1_085_000, percent: 70 },
      { institution: "Corretora B", currentCents: 465_000, percent: 30 },
    ],
    history: Array.from({ length: 12 }, (_, i) => ({
      month: ((11 + i) % 12) + 1,
      year: 2025 + Math.floor((11 + i) / 12),
      currentCents: 1_000_000 + i * 50_000,
      investedCents: 1_000_000 + i * 45_000,
    })),
  },
  loans: [
    {
      id: ids.loanCar,
      name: "Financiamento do carro",
      lender: "Banco C",
      kind: "VEHICLE",
      principalCents: 4_000_000,
      monthlyRatePct: 1.49,
      installmentCents: 89_000,
      totalInstallments: 48,
      paidInstallments: 12,
      remainingInstallments: 36,
      firstDueDate: "2025-11-15",
      nextDueDate: "2026-11-15",
      outstandingCents: 2_520_000,
      totalInterestRemainingCents: 684_000,
      accountId: ids.accountNubank,
      categoryId: null,
      active: true,
      notes: null,
    },
    {
      id: ids.loanPersonal,
      name: "Empréstimo pessoal",
      lender: null,
      kind: "PERSONAL",
      principalCents: 600_000,
      monthlyRatePct: 3.2,
      installmentCents: 41_000,
      totalInstallments: 18,
      paidInstallments: 6,
      remainingInstallments: 12,
      firstDueDate: "2026-05-20",
      nextDueDate: "2026-11-20",
      outstandingCents: 410_000,
      totalInterestRemainingCents: 82_000,
      accountId: null,
      categoryId: null,
      active: true,
      notes: null,
    },
  ],
  systemInfo: {
    version: "0.3.0",
    dbDialect: "sqlite",
    dataPath: "C:\\Users\\voce\\AppData\\Roaming\\finflow\\finflow.db",
    authRequired: true,
    lanEnabled: true,
    lanUrls: ["http://192.168.0.10:3333"],
    aiConfigured: false,
    mcpHttpPath: "/api/mcp",
  },
  aiStatus: { configured: false, source: null, model: "claude-opus-5-5" },
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
  /** Cabeçalho `X-FinFlow-Space` da escrita (undefined = ausente). */
  space?: string;
}

/** Intercepta a API inteira com fixtures determinísticas e captura escritas. */
export async function mockApi(
  page: Page,
  options: { requireAuth?: boolean } = {},
): Promise<CapturedRequest[]> {
  const captured: CapturedRequest[] = [];
  let authenticated = !options.requireAuth;
  // catálogo de espaços desta página (POST /spaces acrescenta)
  const spaces = spaceFixtures.spaces.map((s) => ({ ...s }));

  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

  await page.route("**/api/**", (route) => {
    const request = route.request();
    const method = request.method();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/api/, "");
    const space = request.headers()["x-finflow-space"];

    if (method !== "GET") {
      let body: unknown;
      try {
        body = request.postDataJSON();
      } catch {
        // multipart (upload de planilha): guarda o corpo bruto
        body = request.postData();
      }
      captured.push({ method, url: url.toString(), body, space });
    }

    // v3 fase 2: slug desconhecido = 404 (auth e system não dependem de espaço)
    const independent = path.startsWith("/auth/") || path.startsWith("/system/");
    if (!independent && space && !spaces.some((s) => s.slug === space)) {
      return json(route, { message: "espaço não encontrado" }, 404);
    }

    if (method === "GET") {
      if (path === "/auth/status")
        return json(route, { required: Boolean(options.requireAuth), authenticated });
      if (path === "/dashboard/summary")
        return json(route, {
          ...fixtures.summary,
          totalBalanceCents:
            spaceFixtures.balanceBySpace[space ?? "pessoal"] ?? fixtures.summary.totalBalanceCents,
        });
      if (path === "/spaces") return json(route, spaces);
      if (path === "/spaces/overview") return json(route, spaceFixtures.overview);
      if (path === "/members") return json(route, spaceFixtures.members);
      if (path === "/members/balances") return json(route, spaceFixtures.balances);
      if (path === "/business/metrics") return json(route, spaceFixtures.metrics);
      if (path === "/business/dre") return json(route, businessDre());
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
      // v3
      if (path.startsWith("/cards/") && path.endsWith("/breakdown"))
        return json(route, fixtures.invoiceBreakdown);
      if (path === "/tags") return json(route, fixtures.tags);
      if (path === "/insights/tags") return json(route, fixtures.tagSpending);
      if (path === "/alerts") return json(route, fixtures.alerts);
      if (path === "/insights/subscriptions") return json(route, fixtures.subscriptions);
      if (path === "/insights/cashflow") {
        const basis = url.searchParams.get("basis") === "ACCRUAL" ? "ACCRUAL" : "CASH";
        return json(route, fixtures.cashflowReport[basis]);
      }
      if (path === "/insights/net-worth") return json(route, fixtures.netWorth);
      if (path === "/transactions/transfer-candidates")
        return json(route, fixtures.transferCandidates);
      if (path === "/investments") return json(route, fixtures.investments);
      if (path === "/investments/summary") return json(route, fixtures.investmentSummary);
      if (path === "/loans") return json(route, fixtures.loans);
      if (path === "/loans/strategy")
        return json(route, debtStrategy(Number(url.searchParams.get("extraCents") ?? 0)));
      if (path === "/system/info") return json(route, fixtures.systemInfo);
      if (path === "/ai/status") return json(route, fixtures.aiStatus);
    }

    // v3 fase 2: escritas
    if (method === "POST" && path === "/spaces") {
      const input = request.postDataJSON() as { name: string; kind: string; color?: string };
      const created = {
        slug: slugify(input.name),
        name: input.name,
        kind: input.kind,
        color: input.color ?? "#34d399",
        createdAt: "2026-11-27T00:00:00Z",
        isDefault: false,
      };
      spaces.push(created);
      return json(route, created, 201);
    }
    if (method === "POST" && path === "/members")
      return json(
        route,
        { id: uuid(62), color: "#a78bfa", ...(request.postDataJSON() as object) },
        201,
      );
    if (method === "PATCH" && /^\/members\/[^/]+$/.test(path))
      return json(route, { ...spaceFixtures.members[0], ...(request.postDataJSON() as object) });
    if (method === "POST" && path === "/members/settle")
      return json(route, { id: uuid(63), ...(request.postDataJSON() as object) }, 201);

    // v3: escritas
    if (method === "POST" && path === "/tags")
      return json(route, { ...fixtures.tags[1], ...(request.postDataJSON() as object) }, 201);
    if (method === "PATCH" && /^\/tags\/[^/]+$/.test(path))
      return json(route, { ...fixtures.tags[0], ...(request.postDataJSON() as object) });
    if (method === "POST" && /^\/alerts\/.+\/dismiss$/.test(path))
      return route.fulfill({ status: 204 });
    if (method === "POST" && /^\/insights\/subscriptions\/.+\/track$/.test(path))
      return json(route, { ...fixtures.recurring[0], isSubscription: true }, 201);
    if (method === "POST" && /^\/insights\/subscriptions\/.+\/dismiss$/.test(path))
      return route.fulfill({ status: 204 });
    if (method === "POST" && path === "/transactions/merge-transfer")
      return json(route, { transferGroupId: "merged" });
    if (method === "POST" && path === "/insights/can-afford") {
      const { amountCents = 0 } = request.postDataJSON() as { amountCents?: number };
      return json(route, canAfford(amountCents));
    }
    if (method === "POST" && path === "/wishlist/preview-link")
      return json(route, fixtures.linkPreview);
    if (method === "POST" && path === "/investments")
      return json(route, fixtures.investments[0], 201);
    if (method === "PATCH" && /^\/investments\/[^/]+$/.test(path))
      return json(route, fixtures.investments[0]);
    if (method === "POST" && /^\/investments\/[^/]+\/(value|contribution|redeem)$/.test(path))
      return json(route, fixtures.investments[0]);
    if (method === "POST" && path === "/loans") return json(route, fixtures.loans[0], 201);
    if (method === "PATCH" && /^\/loans\/[^/]+$/.test(path)) return json(route, fixtures.loans[0]);
    if (method === "POST" && /^\/loans\/[^/]+\/pay$/.test(path))
      return json(route, { ...fixtures.transactions.items[0], id: "loan-paid" }, 201);

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
