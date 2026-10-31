/**
 * Seed de desenvolvimento — dados realistas para o dashboard nascer vivo.
 * Idempotente: não roda duas vezes sobre o mesmo banco.
 */
import path from "node:path";
import { PrismaPg } from "@prisma/adapter-pg";
import { config } from "dotenv";
import { addMonthsToDateString, fromDateString, toDateString } from "../src/common/date.util";
import { splitInstallments } from "../src/common/money.util";
import { PrismaClient } from "../src/generated/prisma/client";
import { addMonthsToPeriod, invoicePeriodForPurchase } from "../src/modules/cards/invoice-cycle";

config({ path: path.resolve(__dirname, "../../../.env") });
config();

const prisma = new PrismaClient({
  adapter: new PrismaPg({ connectionString: process.env.DATABASE_URL ?? "" }),
});

const today = new Date();
const d = (daysOffset: number): Date => {
  const copy = new Date(today);
  copy.setUTCDate(copy.getUTCDate() + daysOffset);
  return copy;
};
/** Dia fixo do mês corrente deslocado em `monthOffset` meses. */
const monthDay = (monthOffset: number, day: number): Date =>
  new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth() + monthOffset, day));

async function main(): Promise<void> {
  if ((await prisma.account.count()) > 0) {
    console.log("Seed já aplicado (existem contas) — nada a fazer.");
    return;
  }

  // ───────────────────────── categorias ─────────────────────────
  const expenseCategories = [
    { name: "Alimentação", icon: "utensils", color: "#fb923c" },
    { name: "Mercado", icon: "shopping-cart", color: "#fbbf24" },
    { name: "Moradia", icon: "house", color: "#38bdf8" },
    { name: "Transporte", icon: "car", color: "#a78bfa" },
    { name: "Lazer", icon: "gamepad-2", color: "#f472b6" },
    { name: "Saúde", icon: "heart-pulse", color: "#34d399" },
    { name: "Educação", icon: "graduation-cap", color: "#60a5fa" },
    { name: "Assinaturas", icon: "repeat", color: "#94a3b8" },
  ] as const;
  const incomeCategories = [
    { name: "Salário", icon: "briefcase", color: "#10b981" },
    { name: "Freelance", icon: "laptop", color: "#22d3ee" },
    { name: "Rendimentos", icon: "trending-up", color: "#84cc16" },
  ] as const;

  const cat = new Map<string, string>();
  for (const c of expenseCategories) {
    const created = await prisma.category.create({ data: { ...c, kind: "EXPENSE" } });
    cat.set(c.name, created.id);
  }
  for (const c of incomeCategories) {
    const created = await prisma.category.create({ data: { ...c, kind: "INCOME" } });
    cat.set(c.name, created.id);
  }

  // ───────────────────────── contas ─────────────────────────
  const nubank = await prisma.account.create({
    data: { name: "Nubank", type: "CHECKING", initialBalanceCents: 412_550, color: "#8a05be" },
  });
  const itau = await prisma.account.create({
    data: { name: "Itaú Salário", type: "SALARY", initialBalanceCents: 125_000, color: "#ec7000" },
  });
  const carteira = await prisma.account.create({
    data: { name: "Carteira", type: "WALLET", initialBalanceCents: 18_000, color: "#64748b" },
  });
  const xp = await prisma.account.create({
    data: {
      name: "XP Investimentos",
      type: "INVESTMENT",
      initialBalanceCents: 2_850_000,
      color: "#facc15",
    },
  });

  // ───────────────────────── cartões ─────────────────────────
  const nuCard = await prisma.creditCard.create({
    data: {
      name: "Nubank Ultravioleta",
      brand: "MASTERCARD",
      limitCents: 1_200_000,
      closingDay: 3,
      dueDay: 10,
      color: "#7c3aed",
    },
  });
  const interCard = await prisma.creditCard.create({
    data: {
      name: "Inter Gold",
      brand: "VISA",
      limitCents: 600_000,
      closingDay: 20,
      dueDay: 27,
      color: "#f97316",
    },
  });

  // ─────────────────── receitas e despesas (3 meses) ───────────────────
  type Tx = {
    type: "INCOME" | "EXPENSE";
    description: string;
    amountCents: number;
    date: Date;
    accountId: string;
    category?: string;
    status?: "PENDING" | "CONFIRMED";
  };
  const txs: Tx[] = [];

  for (const m of [-2, -1, 0]) {
    txs.push(
      {
        type: "INCOME",
        description: "Salário",
        amountCents: 985_000,
        date: monthDay(m, 5),
        accountId: itau.id,
        category: "Salário",
      },
      {
        type: "EXPENSE",
        description: "Aluguel + condomínio",
        amountCents: 265_000,
        date: monthDay(m, 10),
        accountId: nubank.id,
        category: "Moradia",
      },
      {
        type: "EXPENSE",
        description: "Energia elétrica",
        amountCents: 21_500 + m * 1200,
        date: monthDay(m, 12),
        accountId: nubank.id,
        category: "Moradia",
      },
      {
        type: "EXPENSE",
        description: "Internet fibra",
        amountCents: 11_990,
        date: monthDay(m, 15),
        accountId: nubank.id,
        category: "Assinaturas",
      },
      {
        type: "EXPENSE",
        description: "Mercado — compra do mês",
        amountCents: 78_400 + m * 3500,
        date: monthDay(m, 7),
        accountId: nubank.id,
        category: "Mercado",
      },
      {
        type: "EXPENSE",
        description: "Academia",
        amountCents: 14_990,
        date: monthDay(m, 8),
        accountId: nubank.id,
        category: "Saúde",
      },
    );
  }

  txs.push(
    {
      type: "INCOME",
      description: "Freelance — app mobile",
      amountCents: 320_000,
      date: monthDay(-1, 18),
      accountId: nubank.id,
      category: "Freelance",
    },
    {
      type: "INCOME",
      description: "Dividendos FIIs",
      amountCents: 18_450,
      date: monthDay(0, 15),
      accountId: xp.id,
      category: "Rendimentos",
    },
    {
      type: "EXPENSE",
      description: "Uber — semana",
      amountCents: 8_740,
      date: d(-6),
      accountId: nubank.id,
      category: "Transporte",
    },
    {
      type: "EXPENSE",
      description: "Padaria",
      amountCents: 3_250,
      date: d(-3),
      accountId: carteira.id,
      category: "Alimentação",
    },
    {
      type: "EXPENSE",
      description: "Almoço — rodízio japonês",
      amountCents: 12_900,
      date: d(-2),
      accountId: nubank.id,
      category: "Alimentação",
    },
    {
      type: "EXPENSE",
      description: "Cinema IMAX",
      amountCents: 9_800,
      date: d(-1),
      accountId: nubank.id,
      category: "Lazer",
    },
    // próximos vencimentos (pendentes) — alimentam o card do dashboard
    {
      type: "EXPENSE",
      description: "Fatura internet (vence)",
      amountCents: 11_990,
      date: d(3),
      accountId: nubank.id,
      category: "Assinaturas",
      status: "PENDING",
    },
    {
      type: "EXPENSE",
      description: "IPVA parcela 2/3",
      amountCents: 48_700,
      date: d(5),
      accountId: nubank.id,
      category: "Transporte",
      status: "PENDING",
    },
    {
      type: "INCOME",
      description: "Reembolso plano de saúde",
      amountCents: 22_000,
      date: d(4),
      accountId: itau.id,
      category: "Saúde",
      status: "PENDING",
    },
  );

  for (const t of txs) {
    await prisma.transaction.create({
      data: {
        type: t.type,
        description: t.description,
        amountCents: t.type === "INCOME" ? t.amountCents : -t.amountCents,
        date: t.date,
        status: t.status ?? "CONFIRMED",
        accountId: t.accountId,
        categoryId: t.category ? cat.get(t.category) : undefined,
      },
    });
  }

  // ─────────────────── transferência (par atômico) ───────────────────
  const transferGroupId = crypto.randomUUID();
  await prisma.transaction.create({
    data: {
      type: "TRANSFER",
      description: "Aporte mensal → XP Investimentos",
      amountCents: -150_000,
      date: monthDay(0, 6),
      accountId: nubank.id,
      transferGroupId,
    },
  });
  await prisma.transaction.create({
    data: {
      type: "TRANSFER",
      description: "Aporte mensal ← Nubank",
      amountCents: 150_000,
      date: monthDay(0, 6),
      accountId: xp.id,
      transferGroupId,
    },
  });

  // ─────────────────── compras no cartão ───────────────────
  async function cardExpense(params: {
    cardId: string;
    closingDay: number;
    description: string;
    totalCents: number;
    date: Date;
    category: string;
    installments?: number;
  }): Promise<void> {
    const n = params.installments ?? 1;
    const amounts = splitInstallments(params.totalCents, n);
    const dateStr = toDateString(params.date);
    const basePeriod = invoicePeriodForPurchase(dateStr, params.closingDay);
    const groupId = n > 1 ? crypto.randomUUID() : null;
    for (let i = 0; i < n; i++) {
      const period = addMonthsToPeriod(basePeriod, i);
      const invoice = await prisma.invoice.upsert({
        where: {
          cardId_year_month: { cardId: params.cardId, year: period.year, month: period.month },
        },
        create: { cardId: params.cardId, year: period.year, month: period.month },
        update: {},
      });
      await prisma.transaction.create({
        data: {
          type: "CARD_EXPENSE",
          description: n > 1 ? `${params.description} (${i + 1}/${n})` : params.description,
          amountCents: -(amounts[i] ?? 0),
          date: fromDateString(addMonthsToDateString(dateStr, i)),
          status: "CONFIRMED",
          cardId: params.cardId,
          categoryId: cat.get(params.category),
          invoiceId: invoice.id,
          installmentGroupId: groupId,
          installmentNumber: n > 1 ? i + 1 : null,
          installmentTotal: n > 1 ? n : null,
        },
      });
    }
  }

  // Parcelado clássico: notebook em 10x iniciado há 2 meses
  await cardExpense({
    cardId: nuCard.id,
    closingDay: nuCard.closingDay,
    description: "Notebook Dell Inspiron",
    totalCents: 350_000, // 10x de R$ 350,00
    date: monthDay(-2, 14),
    category: "Educação",
    installments: 10,
  });
  await cardExpense({
    cardId: nuCard.id,
    closingDay: nuCard.closingDay,
    description: "iFood — jantar",
    totalCents: 6_890,
    date: d(-4),
    category: "Alimentação",
  });
  await cardExpense({
    cardId: nuCard.id,
    closingDay: nuCard.closingDay,
    description: "Spotify Família",
    totalCents: 3_490,
    date: d(-9),
    category: "Assinaturas",
  });
  await cardExpense({
    cardId: nuCard.id,
    closingDay: nuCard.closingDay,
    description: "Posto Shell — gasolina",
    totalCents: 28_000,
    date: d(-7),
    category: "Transporte",
  });
  await cardExpense({
    cardId: interCard.id,
    closingDay: interCard.closingDay,
    description: "Passagens GOL — feriado",
    totalCents: 184_000,
    date: monthDay(0, 2),
    category: "Lazer",
    installments: 4,
  });
  await cardExpense({
    cardId: interCard.id,
    closingDay: interCard.closingDay,
    description: "Farmácia Droga Raia",
    totalCents: 11_230,
    date: d(-5),
    category: "Saúde",
  });

  // ─────────────────── orçamentos ───────────────────
  const budgets: [string, number][] = [
    ["Mercado", 90_000],
    ["Alimentação", 60_000],
    ["Transporte", 70_000],
    ["Lazer", 60_000],
    ["Assinaturas", 20_000],
  ];
  for (const [name, amountCents] of budgets) {
    const categoryId = cat.get(name);
    if (categoryId) await prisma.budget.create({ data: { categoryId, amountCents } });
  }

  // ─────────────────── metas ───────────────────
  const reserva = await prisma.goal.create({
    data: {
      name: "Reserva de emergência",
      icon: "shield",
      color: "#10b981",
      targetCents: 3_000_000,
      deadline: monthDay(14, 1),
    },
  });
  const viagem = await prisma.goal.create({
    data: {
      name: "Eurotrip 2027",
      icon: "plane",
      color: "#38bdf8",
      targetCents: 1_800_000,
      deadline: monthDay(9, 1),
    },
  });
  const macbook = await prisma.goal.create({
    data: { name: "MacBook Pro", icon: "laptop", color: "#a78bfa", targetCents: 1_400_000 },
  });

  const contributions: [string, number, Date, string | null][] = [
    [reserva.id, 450_000, monthDay(-2, 6), "aporte inicial"],
    [reserva.id, 300_000, monthDay(-1, 6), null],
    [reserva.id, 300_000, monthDay(0, 6), null],
    [viagem.id, 200_000, monthDay(-1, 20), "bônus freelance"],
    [viagem.id, 120_000, monthDay(0, 20), null],
    [macbook.id, 150_000, monthDay(0, 12), null],
  ];
  for (const [goalId, amountCents, date, note] of contributions) {
    await prisma.goalContribution.create({ data: { goalId, amountCents, date, note } });
  }

  console.log(
    "Seed concluído: 4 contas, 2 cartões, 11 categorias, transações de 3 meses, 5 orçamentos e 3 metas.",
  );
}

main()
  .catch((error) => {
    console.error(error);
    process.exitCode = 1;
  })
  .finally(() => prisma.$disconnect());
