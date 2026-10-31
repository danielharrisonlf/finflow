#!/usr/bin/env node
import { FinFlowApiError, FinFlowClient, resolveByName } from "@finflow/api-client";
/**
 * Servidor MCP do FinFlow (stdio).
 *
 * Registre no seu agente apontando o comando para este binário, com a API
 * rodando (FINFLOW_API_URL opcional; padrão http://localhost:3333).
 * Valores monetários são SEMPRE em centavos inteiros (R$ 45,90 = 4590).
 */
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const client = new FinFlowClient();

const server = new McpServer({
  name: "finflow",
  version: "0.1.0",
});

type ToolResult = { content: { type: "text"; text: string }[]; isError?: boolean };

function ok(data: unknown): ToolResult {
  return {
    content: [
      { type: "text", text: typeof data === "string" ? data : JSON.stringify(data, null, 2) },
    ],
  };
}

function err(message: string): ToolResult {
  return { content: [{ type: "text", text: `erro: ${message}` }], isError: true };
}

function guard<A extends unknown[]>(fn: (...args: A) => Promise<ToolResult>) {
  return async (...args: A): Promise<ToolResult> => {
    try {
      return await fn(...args);
    } catch (error) {
      if (error instanceof FinFlowApiError) return err(error.message);
      return err((error as Error).message);
    }
  };
}

async function accountByRef(ref: string): Promise<{ id: string; name: string }> {
  const accounts = await client.accounts();
  const found = resolveByName(accounts, ref);
  if (!found) {
    throw new Error(
      `conta "${ref}" não encontrada ou ambígua. Contas: ${accounts.map((a) => a.name).join(", ")}`,
    );
  }
  return found;
}

async function cardByRef(ref: string): Promise<{ id: string; name: string }> {
  const cards = await client.cards();
  const found = resolveByName(cards, ref);
  if (!found) {
    throw new Error(
      `cartão "${ref}" não encontrado ou ambíguo. Cartões: ${cards.map((c) => c.name).join(", ")}`,
    );
  }
  return found;
}

async function categoryIdByRef(
  ref: string | undefined,
  kind: "INCOME" | "EXPENSE",
): Promise<string | null> {
  if (!ref) return null;
  const categories = await client.categories(kind);
  const found = resolveByName(categories, ref);
  if (!found) {
    throw new Error(
      `categoria "${ref}" não encontrada. Disponíveis: ${categories.map((c) => c.name).join(", ")}`,
    );
  }
  return found.id;
}

const period = {
  month: z.number().int().min(1).max(12).optional().describe("mês (1-12); padrão: mês atual"),
  year: z.number().int().optional().describe("ano; padrão: ano atual"),
};

const today = (): string => new Date().toISOString().slice(0, 10);

// ───────────────────────── leitura ─────────────────────────

server.tool(
  "get_summary",
  "Resumo financeiro: saldo consolidado, receitas/despesas/balanço do mês, faturas em aberto e pendências. Valores em centavos.",
  period,
  guard(async ({ month, year }) => ok(await client.dashboardSummary(month, year))),
);

server.tool(
  "get_cashflow",
  "Fluxo de caixa diário do mês (entradas vs. saídas), em centavos.",
  period,
  guard(async ({ month, year }) => ok(await client.cashflow(month, year))),
);

server.tool(
  "get_category_spending",
  "Despesas do mês agrupadas por categoria, com percentuais.",
  period,
  guard(async ({ month, year }) => ok(await client.categorySpending(month, year))),
);

server.tool(
  "get_upcoming_bills",
  "Contas pendentes e faturas de cartão vencendo nos próximos 7 dias.",
  {},
  guard(async () => ok(await client.upcoming())),
);

server.tool(
  "list_accounts",
  "Lista as contas com saldo atual (centavos).",
  {},
  guard(async () => ok(await client.accounts())),
);

server.tool(
  "list_cards",
  "Lista cartões de crédito com limite usado/disponível e fatura atual.",
  {},
  guard(async () => ok(await client.cards())),
);

server.tool(
  "list_card_invoices",
  "Lista as faturas de um cartão (status OPEN/CLOSED/PAID, totais e vencimentos).",
  { card: z.string().describe("nome (parcial) ou id do cartão") },
  guard(async ({ card }) => {
    const found = await cardByRef(card);
    return ok(await client.invoices(found.id));
  }),
);

server.tool(
  "list_transactions",
  "Extrato com filtros por período, tipo, status e busca textual.",
  {
    ...period,
    search: z.string().optional().describe("busca na descrição"),
    type: z.enum(["INCOME", "EXPENSE", "TRANSFER", "CARD_EXPENSE", "INVOICE_PAYMENT"]).optional(),
    status: z.enum(["PENDING", "CONFIRMED"]).optional(),
    page: z.number().int().min(1).optional(),
  },
  guard(async (filters) => ok(await client.transactions(filters))),
);

server.tool(
  "list_categories",
  "Lista categorias de receita e despesa.",
  { kind: z.enum(["INCOME", "EXPENSE"]).optional() },
  guard(async ({ kind }) => ok(await client.categories(kind))),
);

server.tool(
  "list_budgets",
  "Orçamentos por categoria com gasto do mês, percentual e status (OK/WARNING/DANGER).",
  period,
  guard(async ({ month, year }) => ok(await client.budgets(month, year))),
);

server.tool(
  "list_goals",
  "Metas financeiras com progresso e aporte mensal necessário.",
  {},
  guard(async () => ok(await client.goals())),
);

// ───────────────────────── escrita ─────────────────────────

const simpleExpenseShape = {
  amountCents: z.number().int().positive().describe("valor em centavos (R$ 45,90 = 4590)"),
  description: z.string().min(1),
  account: z.string().optional().describe("nome ou id da conta; padrão: primeira conta"),
  category: z.string().optional().describe("nome da categoria"),
  date: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/)
    .optional()
    .describe("YYYY-MM-DD; padrão: hoje"),
  pending: z.boolean().optional().describe("true = ainda não paga/recebida"),
};

async function createSimple(
  type: "INCOME" | "EXPENSE",
  input: {
    amountCents: number;
    description: string;
    account?: string;
    category?: string;
    date?: string;
    pending?: boolean;
  },
): Promise<ToolResult> {
  const accounts = await client.accounts();
  const account = input.account
    ? await accountByRef(input.account)
    : (accounts.find((a) => a.type === "CHECKING") ?? accounts[0]);
  if (!account) return err("nenhuma conta cadastrada");
  const categoryId = await categoryIdByRef(input.category, type);
  const created = await client.createTransaction({
    type,
    description: input.description,
    amountCents: input.amountCents,
    date: input.date ?? today(),
    status: input.pending ? "PENDING" : "CONFIRMED",
    accountId: account.id,
    categoryId,
    notes: "lançado via MCP",
  });
  return ok(created);
}

server.tool(
  "create_expense",
  "Lança uma despesa em uma conta. Valor em centavos.",
  simpleExpenseShape,
  guard(async (input) => createSimple("EXPENSE", input)),
);

server.tool(
  "create_income",
  "Lança uma receita em uma conta. Valor em centavos.",
  simpleExpenseShape,
  guard(async (input) => createSimple("INCOME", input)),
);

server.tool(
  "create_card_expense",
  "Compra no cartão de crédito, à vista ou parcelada; cada parcela cai na fatura do mês correto.",
  {
    card: z.string().describe("nome ou id do cartão"),
    description: z.string().min(1),
    totalCents: z.number().int().positive().describe("valor TOTAL da compra em centavos"),
    installments: z.number().int().min(1).max(48).optional().describe("parcelas; padrão 1"),
    category: z.string().optional(),
    date: z
      .string()
      .regex(/^\d{4}-\d{2}-\d{2}$/)
      .optional(),
  },
  guard(async ({ card, description, totalCents, installments, category, date }) => {
    const found = await cardByRef(card);
    const categoryId = await categoryIdByRef(category, "EXPENSE");
    const result = await client.addCardExpense({
      cardId: found.id,
      description,
      totalCents,
      date: date ?? today(),
      categoryId,
      installments: installments ?? 1,
      notes: "lançado via MCP",
    });
    return ok({ card: found.name, ...result });
  }),
);

server.tool(
  "transfer",
  "Transfere entre contas próprias (débito e crédito atômicos).",
  {
    from: z.string().describe("conta de origem (nome ou id)"),
    to: z.string().describe("conta de destino (nome ou id)"),
    amountCents: z.number().int().positive(),
    date: z
      .string()
      .regex(/^\d{4}-\d{2}-\d{2}$/)
      .optional(),
  },
  guard(async ({ from, to, amountCents, date }) => {
    const [origin, destination] = await Promise.all([accountByRef(from), accountByRef(to)]);
    const result = await client.transfer({
      fromAccountId: origin.id,
      toAccountId: destination.id,
      amountCents,
      date: date ?? today(),
    });
    return ok({ from: origin.name, to: destination.name, ...result });
  }),
);

server.tool(
  "pay_invoice",
  "Paga a fatura de um cartão debitando de uma conta.",
  {
    card: z.string().describe("nome ou id do cartão"),
    month: z.number().int().min(1).max(12).describe("mês da fatura"),
    year: z.number().int().describe("ano da fatura"),
    account: z.string().describe("conta para débito (nome ou id)"),
  },
  guard(async ({ card, month, year, account }) => {
    const found = await cardByRef(card);
    const invoices = await client.invoices(found.id);
    const invoice = invoices.find((i) => i.month === month && i.year === year);
    if (!invoice) {
      return err(
        `fatura ${month}/${year} não existe para ${found.name}. Faturas: ${invoices.map((i) => `${i.month}/${i.year}`).join(", ")}`,
      );
    }
    const payer = await accountByRef(account);
    return ok(await client.payInvoice(found.id, invoice.id, payer.id));
  }),
);

server.tool(
  "set_budget",
  "Define (ou atualiza) o teto mensal de uma categoria de despesa.",
  {
    category: z.string().describe("nome da categoria de despesa"),
    amountCents: z.number().int().positive(),
  },
  guard(async ({ category, amountCents }) => {
    const categoryId = await categoryIdByRef(category, "EXPENSE");
    if (!categoryId) return err(`categoria "${category}" não encontrada`);
    return ok(await client.upsertBudget({ categoryId, amountCents }));
  }),
);

server.tool(
  "contribute_goal",
  "Aporta (positivo) ou resgata (negativo) de uma meta financeira.",
  {
    goal: z.string().describe("nome ou id da meta"),
    amountCents: z.number().int().describe("centavos; negativo = resgate"),
    note: z.string().optional(),
  },
  guard(async ({ goal, amountCents, note }) => {
    const goals = await client.goals();
    const found = resolveByName(goals, goal);
    if (!found) {
      return err(`meta "${goal}" não encontrada. Metas: ${goals.map((g) => g.name).join(", ")}`);
    }
    return ok(await client.contributeGoal(found.id, { amountCents, note }));
  }),
);

server.tool(
  "export_transactions",
  "Exporta o extrato filtrado em CSV ou OFX e devolve o conteúdo do arquivo.",
  {
    format: z.enum(["csv", "ofx"]),
    ...period,
  },
  guard(async ({ format, month, year }) =>
    ok(await client.exportTransactions(format, { month, year })),
  ),
);

async function main(): Promise<void> {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  // stdout é do protocolo MCP; logs vão para stderr
  console.error(
    `finflow-mcp conectado (API: ${process.env.FINFLOW_API_URL ?? "http://localhost:3333"})`,
  );
}

void main();
