import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import request from "supertest";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { AppModule } from "../src/app.module";
import { toDateString } from "../src/common/date.util";
import { PrismaService } from "../src/common/prisma.service";

const today = toDateString(new Date());

describe("FinFlow API (integração, banco *_test)", () => {
  let app: INestApplication;
  let prisma: PrismaService;
  let http: ReturnType<typeof request>;

  beforeAll(async () => {
    process.env.TELEGRAM_BOT_TOKEN = "test-token";
    process.env.TELEGRAM_CHAT_ID = "4242";
    process.env.TELEGRAM_WEBHOOK_SECRET = "test-secret";

    const moduleRef = await Test.createTestingModule({ imports: [AppModule] }).compile();
    app = moduleRef.createNestApplication();
    app.setGlobalPrefix("api");
    await app.init();

    prisma = app.get(PrismaService);
    // limpa o banco de teste (ordem respeita FKs via cascade)
    await prisma.transaction.deleteMany();
    await prisma.invoice.deleteMany();
    await prisma.goalContribution.deleteMany();
    await prisma.goal.deleteMany();
    await prisma.budget.deleteMany();
    await prisma.creditCard.deleteMany();
    await prisma.account.deleteMany();
    await prisma.category.deleteMany();

    http = request(app.getHttpServer());
  });

  afterAll(async () => {
    await app?.close();
  });

  let accountA: string;
  let accountB: string;
  let foodCategory: string;

  it("cria contas e categorias", async () => {
    const a = await http
      .post("/api/accounts")
      .send({ name: "Conta Corrente", type: "CHECKING", initialBalanceCents: 100_000 })
      .expect(201);
    accountA = a.body.id;
    expect(a.body.balanceCents).toBe(100_000);

    const b = await http
      .post("/api/accounts")
      .send({ name: "Poupança", type: "SAVINGS", initialBalanceCents: 50_000 })
      .expect(201);
    accountB = b.body.id;

    const cat = await http
      .post("/api/categories")
      .send({ name: "Alimentação", kind: "EXPENSE", icon: "utensils", color: "#fb923c" })
      .expect(201);
    foodCategory = cat.body.id;
  });

  it("valida payload inválido com 400 e issues do Zod", async () => {
    const res = await http.post("/api/accounts").send({ name: "", type: "NAO_EXISTE" }).expect(400);
    expect(res.body.issues?.length).toBeGreaterThan(0);
  });

  it("lança despesa e receita refletindo no saldo", async () => {
    await http
      .post("/api/transactions")
      .send({
        type: "EXPENSE",
        description: "Mercado",
        amountCents: 25_000,
        date: today,
        accountId: accountA,
        categoryId: foodCategory,
      })
      .expect(201);

    await http
      .post("/api/transactions")
      .send({
        type: "INCOME",
        description: "Salário",
        amountCents: 300_000,
        date: today,
        accountId: accountA,
      })
      .expect(201);

    const account = await http.get(`/api/accounts/${accountA}`).expect(200);
    expect(account.body.balanceCents).toBe(100_000 - 25_000 + 300_000);
  });

  it("transfere entre contas com débito e crédito atômicos", async () => {
    await http
      .post("/api/transactions/transfer")
      .send({ fromAccountId: accountA, toAccountId: accountB, amountCents: 40_000, date: today })
      .expect(201);

    const [a, b] = await Promise.all([
      http.get(`/api/accounts/${accountA}`),
      http.get(`/api/accounts/${accountB}`),
    ]);
    expect(a.body.balanceCents).toBe(375_000 - 40_000);
    expect(b.body.balanceCents).toBe(50_000 + 40_000);

    // excluir uma perna remove o par inteiro
    const list = await http.get("/api/transactions?type=TRANSFER").expect(200);
    expect(list.body.items).toHaveLength(2);
    await http.delete(`/api/transactions/${list.body.items[0].id}`).expect(204);
    const after = await http.get("/api/transactions?type=TRANSFER").expect(200);
    expect(after.body.items).toHaveLength(0);
  });

  describe("cartão de crédito", () => {
    let cardId: string;

    it("cadastra cartão e lança compra parcelada em 3x", async () => {
      const card = await http
        .post("/api/cards")
        .send({
          name: "Cartão Teste",
          brand: "VISA",
          limitCents: 500_000,
          closingDay: 10,
          dueDay: 17,
        })
        .expect(201);
      cardId = card.body.id;

      await http
        .post(`/api/cards/${cardId}/expenses`)
        .send({
          cardId,
          description: "Fone de ouvido",
          totalCents: 100_000,
          date: today,
          installments: 3,
          categoryId: foodCategory,
        })
        .expect(201);

      const invoices = await http.get(`/api/cards/${cardId}/invoices`).expect(200);
      expect(invoices.body).toHaveLength(3);
      const total = invoices.body.reduce(
        (acc: number, i: { totalCents: number }) => acc + i.totalCents,
        0,
      );
      expect(total).toBe(100_000);

      const detail = await http.get(`/api/cards/${cardId}`).expect(200);
      expect(detail.body.usedLimitCents).toBe(100_000);
      expect(detail.body.availableLimitCents).toBe(400_000);
    });

    it("paga fatura debitando da conta e liberando limite", async () => {
      const invoices = await http.get(`/api/cards/${cardId}/invoices`).expect(200);
      const oldest = invoices.body[invoices.body.length - 1];

      const before = await http.get(`/api/accounts/${accountA}`).expect(200);
      const paid = await http
        .post(`/api/cards/${cardId}/invoices/${oldest.id}/pay`)
        .send({ accountId: accountA })
        .expect(201);
      expect(paid.body.status).toBe("PAID");

      const after = await http.get(`/api/accounts/${accountA}`).expect(200);
      expect(after.body.balanceCents).toBe(before.body.balanceCents - oldest.totalCents);

      const card = await http.get(`/api/cards/${cardId}`).expect(200);
      expect(card.body.usedLimitCents).toBe(100_000 - oldest.totalCents);

      // pagar duas vezes é rejeitado
      await http
        .post(`/api/cards/${cardId}/invoices/${oldest.id}/pay`)
        .send({ accountId: accountA })
        .expect(409);
    });
  });

  it("orçamento reporta gasto e status", async () => {
    const budget = await http
      .post("/api/budgets")
      .send({ categoryId: foodCategory, amountCents: 50_000 })
      .expect(201);
    // despesa de 25.000 + 1ª parcela do cartão no mês corrente
    expect(budget.body.spentCents).toBeGreaterThanOrEqual(25_000);
    expect(["OK", "WARNING", "DANGER"]).toContain(budget.body.status);
  });

  it("meta calcula progresso e aporte mensal; resgate maior que o saldo é rejeitado", async () => {
    const goal = await http
      .post("/api/goals")
      .send({ name: "Reserva", targetCents: 120_000, icon: "shield" })
      .expect(201);

    const after = await http
      .post(`/api/goals/${goal.body.id}/contributions`)
      .send({ amountCents: 30_000 })
      .expect(201);
    expect(after.body.savedCents).toBe(30_000);
    expect(after.body.percent).toBe(25);

    await http
      .post(`/api/goals/${goal.body.id}/contributions`)
      .send({ amountCents: -50_000 })
      .expect(400);
  });

  it("dashboard resume o mês e exporta extrato em CSV e OFX", async () => {
    const summary = await http.get("/api/dashboard/summary").expect(200);
    expect(summary.body.monthIncomeCents).toBeGreaterThan(0);
    expect(summary.body.monthExpenseCents).toBeGreaterThan(0);

    const csv = await http.get("/api/transactions/export?format=csv").expect(200);
    expect(csv.headers["content-disposition"]).toContain(".csv");
    expect(csv.text).toContain("Data;Descrição");

    const ofx = await http.get("/api/transactions/export?format=ofx").expect(200);
    expect(ofx.text).toContain("<OFX>");
    expect(ofx.text).toContain("<STMTTRN>");
  });

  describe("webhook do Telegram", () => {
    it("rejeita secret inválido", async () => {
      await http
        .post("/api/telegram/webhook")
        .set("x-telegram-bot-api-secret-token", "errado")
        .send({ update_id: 1, message: { message_id: 1, text: "/saldo", chat: { id: 4242 } } })
        .expect(403);
    });

    it("lança despesa rápida vinda do chat vinculado (Bot API mockada)", async () => {
      const fetchMock = vi.fn(
        async (..._args: Parameters<typeof fetch>) => new Response(JSON.stringify({ ok: true })),
      );
      vi.stubGlobal("fetch", fetchMock);
      try {
        await http
          .post("/api/telegram/webhook")
          .set("x-telegram-bot-api-secret-token", "test-secret")
          .send({
            update_id: 2,
            message: { message_id: 2, text: "19,90 pizza", chat: { id: 4242 } },
          })
          .expect(200);

        expect(fetchMock).toHaveBeenCalledOnce();
        const url = String(fetchMock.mock.calls[0]?.[0]);
        expect(url).toContain("/bottest-token/sendMessage");

        const list = await http.get("/api/transactions?search=pizza").expect(200);
        expect(list.body.items).toHaveLength(1);
        expect(list.body.items[0].amountCents).toBe(-1990);
        expect(list.body.items[0].notes).toBe("lançado via Telegram");
      } finally {
        vi.unstubAllGlobals();
      }
    });
  });

  it("cria e edita subcategoria e edita transação existente", async () => {
    const sub = await http
      .post("/api/categories")
      .send({
        name: "Restaurantes",
        kind: "EXPENSE",
        icon: "utensils",
        color: "#f97316",
        parentId: foodCategory,
      })
      .expect(201);
    expect(sub.body.parentId).toBe(foodCategory);

    const updatedSub = await http
      .patch(`/api/categories/${sub.body.id}`)
      .send({ name: "Restaurantes & Delivery" })
      .expect(200);
    expect(updatedSub.body.name).toBe("Restaurantes & Delivery");

    const list = await http.get("/api/transactions?search=Mercado").expect(200);
    const txId = list.body.items[0]?.id as string;
    const editedTx = await http
      .patch(`/api/transactions/${txId}`)
      .send({
        description: "Supermercado mensal",
        amountCents: 28_000,
        categoryId: sub.body.id,
        notes: "compra ajustada",
      })
      .expect(200);
    expect(editedTx.body.description).toBe("Supermercado mensal");
    expect(editedTx.body.amountCents).toBe(-28_000);
    expect(editedTx.body.categoryName).toBe("Restaurantes & Delivery");
  });

  describe("autenticação (FINFLOW_PASSWORD)", () => {
    it("bloqueia rotas sem token e libera com Bearer token do login", async () => {
      process.env.FINFLOW_PASSWORD = "senha-integração-123";
      try {
        const statusAnon = await http.get("/api/auth/status").expect(200);
        expect(statusAnon.body).toEqual({ required: true, authenticated: false });

        await http.get("/api/accounts").expect(401);
        await http.post("/api/auth/login").send({ password: "errada" }).expect(401);

        const login = await http
          .post("/api/auth/login")
          .send({ password: "senha-integração-123" })
          .expect(200);
        const token = login.body.token as string;
        expect(token).toBeTruthy();

        const statusAuth = await http
          .get("/api/auth/status")
          .set("Authorization", `Bearer ${token}`)
          .expect(200);
        expect(statusAuth.body).toEqual({ required: true, authenticated: true });

        await http.get("/api/accounts").set("Authorization", `Bearer ${token}`).expect(200);

        await http
          .get(`/api/transactions/export?format=csv&token=${encodeURIComponent(token)}`)
          .expect(200);
      } finally {
        delete process.env.FINFLOW_PASSWORD;
      }
    });
  });
});
