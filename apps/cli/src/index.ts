#!/usr/bin/env node
import { writeFileSync } from "node:fs";
import path from "node:path";
import {
  FinFlowApiError,
  FinFlowClient,
  formatBRL,
  parseBRLToCents,
  resolveByName,
} from "@finflow/api-client";
import type { Transaction } from "@finflow/contracts";
import { Command } from "commander";
import pc from "picocolors";
import { progressBar, renderTable } from "./table";

const program = new Command();

program
  .name("finflow")
  .description("FinFlow — controle financeiro pessoal no terminal")
  .version("0.1.0")
  .option("--api <url>", "URL da API (padrão: FINFLOW_API_URL ou http://localhost:3333)")
  .option("--token <token>", "Token ou senha da API (padrão: FINFLOW_TOKEN ou FINFLOW_PASSWORD)");

function client(): FinFlowClient {
  const opts = program.opts<{ api?: string; token?: string }>();
  return new FinFlowClient(opts.api, opts.token);
}

function fail(message: string): never {
  console.error(pc.red(`✖ ${message}`));
  process.exit(1);
}

function money(cents: number, opts: { signed?: boolean } = {}): string {
  const text = opts.signed && cents > 0 ? `+${formatBRL(cents)}` : formatBRL(cents);
  return cents > 0 ? pc.green(text) : cents < 0 ? pc.red(text) : pc.dim(text);
}

function requireCents(raw: string): number {
  const cents = parseBRLToCents(raw);
  if (cents === null) fail(`valor inválido: "${raw}" — use formatos como 45,90 ou 1.234,56`);
  return cents;
}

const TYPE_LABEL: Record<Transaction["type"], string> = {
  INCOME: "receita",
  EXPENSE: "despesa",
  TRANSFER: "transferência",
  CARD_EXPENSE: "cartão",
  INVOICE_PAYMENT: "pgto fatura",
};

async function resolveAccount(
  ref: string | undefined,
  label = "conta",
): Promise<{ id: string; name: string }> {
  const accounts = await client().accounts();
  if (accounts.length === 0) fail("nenhuma conta cadastrada — crie uma no app ou com a API");
  if (!ref) {
    const first = accounts[0];
    if (!first) fail("nenhuma conta cadastrada");
    return first;
  }
  const found = resolveByName(accounts, ref);
  if (!found) {
    fail(
      `${label} "${ref}" não encontrada (ou ambígua). Contas: ${accounts.map((a) => a.name).join(", ")}`,
    );
  }
  return found;
}

async function resolveCategory(
  ref: string | undefined,
  kind: "INCOME" | "EXPENSE",
): Promise<string | null> {
  if (!ref) return null;
  const categories = await client().categories(kind);
  const found = resolveByName(categories, ref);
  if (!found) {
    fail(
      `categoria "${ref}" não encontrada. Disponíveis: ${categories.map((c) => c.name).join(", ")}`,
    );
  }
  return found.id;
}

// ───────────────────────── consultas ─────────────────────────

program
  .command("saldo")
  .description("Saldo de todas as contas e total consolidado")
  .action(async () => {
    const accounts = await client().accounts();
    const rows = accounts.map((a) => [a.name, pc.dim(a.type.toLowerCase()), money(a.balanceCents)]);
    console.log(
      renderTable(
        [{ header: "Conta" }, { header: "Tipo" }, { header: "Saldo", align: "right" }],
        rows,
      ),
    );
    const total = accounts.reduce((acc, a) => acc + a.balanceCents, 0);
    console.log(`\n${pc.bold("Total:")} ${pc.bold(money(total))}`);
  });

program
  .command("resumo")
  .description("Resumo do mês: receitas, despesas, balanço e maiores gastos")
  .option("--mes <n>", "mês (1-12)")
  .option("--ano <n>", "ano")
  .action(async (opts: { mes?: string; ano?: string }) => {
    const month = opts.mes ? Number(opts.mes) : undefined;
    const year = opts.ano ? Number(opts.ano) : undefined;
    const c = client();
    const [summary, slices] = await Promise.all([
      c.dashboardSummary(month, year),
      c.categorySpending(month, year),
    ]);
    console.log(pc.bold(`Resumo ${String(summary.month).padStart(2, "0")}/${summary.year}`));
    console.log(`  Saldo geral        ${money(summary.totalBalanceCents)}`);
    console.log(`  Receitas           ${money(summary.monthIncomeCents, { signed: true })}`);
    console.log(`  Despesas           ${money(-summary.monthExpenseCents)}`);
    console.log(`  Balanço            ${money(summary.monthNetCents, { signed: true })}`);
    console.log(`  Faturas em aberto  ${pc.yellow(formatBRL(summary.openInvoicesCents))}`);
    console.log(`  Pendências         ${summary.pendingCount}`);
    if (slices.length > 0) {
      console.log(`\n${pc.bold("Maiores gastos")}`);
      for (const slice of slices.slice(0, 5)) {
        console.log(
          `  ${progressBar(slice.percent, 12)} ${slice.name.padEnd(16)} ${formatBRL(slice.totalCents)} ${pc.dim(`(${slice.percent}%)`)}`,
        );
      }
    }
  });

program
  .command("extrato")
  .description("Extrato com filtros")
  .option("--mes <n>", "mês (1-12)")
  .option("--ano <n>", "ano")
  .option("--busca <texto>", "busca na descrição")
  .option("--tipo <tipo>", "INCOME|EXPENSE|TRANSFER|CARD_EXPENSE|INVOICE_PAYMENT")
  .option("--status <status>", "PENDING|CONFIRMED")
  .option("--pagina <n>", "página", "1")
  .action(
    async (opts: {
      mes?: string;
      ano?: string;
      busca?: string;
      tipo?: string;
      status?: string;
      pagina: string;
    }) => {
      const result = await client().transactions({
        month: opts.mes ? Number(opts.mes) : undefined,
        year: opts.ano ? Number(opts.ano) : undefined,
        search: opts.busca,
        type: opts.tipo as Transaction["type"] | undefined,
        status: opts.status as Transaction["status"] | undefined,
        page: Number(opts.pagina),
        pageSize: 25,
      });
      if (result.items.length === 0) {
        console.log(pc.dim("nenhum lançamento para esses filtros"));
        return;
      }
      const rows = result.items.map((t) => [
        pc.dim(t.date),
        t.description + (t.status === "PENDING" ? pc.yellow(" (pendente)") : ""),
        pc.dim(t.categoryName ?? TYPE_LABEL[t.type]),
        pc.dim(t.accountName ?? t.cardName ?? ""),
        money(t.amountCents, { signed: true }),
      ]);
      console.log(
        renderTable(
          [
            { header: "Data" },
            { header: "Descrição" },
            { header: "Categoria" },
            { header: "Conta/Cartão" },
            { header: "Valor", align: "right" },
          ],
          rows,
        ),
      );
      const pages = Math.max(1, Math.ceil(result.total / result.pageSize));
      console.log(pc.dim(`\npágina ${result.page}/${pages} — ${result.total} lançamentos`));
    },
  );

// ───────────────────────── lançamentos ─────────────────────────

function registerSimple(name: "despesa" | "receita", type: "EXPENSE" | "INCOME"): void {
  program
    .command(`${name} <valor> <descricao...>`)
    .description(`Lança uma ${name} (ex.: finflow ${name} 45,90 almoço --categoria alimentacao)`)
    .option("--conta <nome>", "conta (padrão: primeira conta)")
    .option("--categoria <nome>", "categoria")
    .option("--data <YYYY-MM-DD>", "data (padrão: hoje)")
    .option("--pendente", `marca como ${type === "EXPENSE" ? "a pagar" : "a receber"}`)
    .action(
      async (
        valor: string,
        descricao: string[],
        opts: { conta?: string; categoria?: string; data?: string; pendente?: boolean },
      ) => {
        const amountCents = requireCents(valor);
        const account = await resolveAccount(opts.conta);
        const categoryId = await resolveCategory(opts.categoria, type);
        const created = await client().createTransaction({
          type,
          description: descricao.join(" "),
          amountCents,
          date: opts.data ?? new Date().toISOString().slice(0, 10),
          status: opts.pendente ? "PENDING" : "CONFIRMED",
          accountId: account.id,
          categoryId,
          notes: "lançado via CLI",
        });
        console.log(
          pc.green(
            `✔ ${name} de ${formatBRL(Math.abs(created.amountCents))} lançada em ${account.name}`,
          ),
        );
      },
    );
}
registerSimple("despesa", "EXPENSE");
registerSimple("receita", "INCOME");

program
  .command("transferir <valor>")
  .description("Transfere entre contas (ex.: finflow transferir 500 --de Nubank --para XP)")
  .requiredOption("--de <conta>", "conta de origem")
  .requiredOption("--para <conta>", "conta de destino")
  .option("--data <YYYY-MM-DD>", "data (padrão: hoje)")
  .action(async (valor: string, opts: { de: string; para: string; data?: string }) => {
    const amountCents = requireCents(valor);
    const from = await resolveAccount(opts.de, "conta de origem");
    const to = await resolveAccount(opts.para, "conta de destino");
    await client().transfer({
      fromAccountId: from.id,
      toAccountId: to.id,
      amountCents,
      date: opts.data ?? new Date().toISOString().slice(0, 10),
    });
    console.log(
      pc.green(`✔ ${formatBRL(amountCents)} transferidos de ${from.name} para ${to.name}`),
    );
  });

program
  .command("compra <valor> <descricao...>")
  .description("Compra no cartão, à vista ou parcelada (--parcelas N)")
  .requiredOption("--cartao <nome>", "cartão")
  .option("--parcelas <n>", "número de parcelas", "1")
  .option("--categoria <nome>", "categoria")
  .option("--data <YYYY-MM-DD>", "data da compra (padrão: hoje)")
  .action(
    async (
      valor: string,
      descricao: string[],
      opts: { cartao: string; parcelas: string; categoria?: string; data?: string },
    ) => {
      const totalCents = requireCents(valor);
      const cards = await client().cards();
      const card = resolveByName(cards, opts.cartao);
      if (!card)
        fail(
          `cartão "${opts.cartao}" não encontrado. Cartões: ${cards.map((c) => c.name).join(", ")}`,
        );
      const categoryId = await resolveCategory(opts.categoria, "EXPENSE");
      const installments = Number(opts.parcelas);
      await client().addCardExpense({
        cardId: card.id,
        description: descricao.join(" "),
        totalCents,
        date: opts.data ?? new Date().toISOString().slice(0, 10),
        categoryId,
        installments,
        notes: "lançado via CLI",
      });
      console.log(
        pc.green(
          `✔ compra de ${formatBRL(totalCents)} no ${card.name}${installments > 1 ? ` em ${installments}x` : ""}`,
        ),
      );
    },
  );

// ───────────────────────── cartões e faturas ─────────────────────────

program
  .command("cartoes")
  .description("Cartões com fatura atual e limite disponível")
  .action(async () => {
    const cards = await client().cards();
    if (cards.length === 0) {
      console.log(pc.dim("nenhum cartão cadastrado"));
      return;
    }
    const rows = cards.map((c) => [
      c.name,
      pc.dim(c.brand),
      c.currentInvoice ? formatBRL(c.currentInvoice.totalCents) : pc.dim("—"),
      c.currentInvoice ? c.currentInvoice.status : pc.dim("—"),
      c.currentInvoice ? pc.dim(c.currentInvoice.dueDate) : "",
      money(c.availableLimitCents),
    ]);
    console.log(
      renderTable(
        [
          { header: "Cartão" },
          { header: "Bandeira" },
          { header: "Fatura atual", align: "right" },
          { header: "Status" },
          { header: "Vencimento" },
          { header: "Limite disp.", align: "right" },
        ],
        rows,
      ),
    );
  });

program
  .command("faturas <cartao>")
  .description("Faturas de um cartão; pague com --pagar MM/AAAA --conta <nome>")
  .option("--pagar <MM/AAAA>", "paga a fatura do período")
  .option("--conta <nome>", "conta para débito do pagamento")
  .action(async (cartao: string, opts: { pagar?: string; conta?: string }) => {
    const c = client();
    const cards = await c.cards();
    const card = resolveByName(cards, cartao);
    if (!card)
      fail(`cartão "${cartao}" não encontrado. Cartões: ${cards.map((x) => x.name).join(", ")}`);
    const invoices = await c.invoices(card.id);

    if (opts.pagar) {
      const match = opts.pagar.match(/^(\d{1,2})\/(\d{4})$/);
      if (!match) fail("período inválido — use o formato MM/AAAA, ex.: --pagar 11/2026");
      const invoice = invoices.find(
        (i) => i.month === Number(match[1]) && i.year === Number(match[2]),
      );
      if (!invoice) fail(`fatura ${opts.pagar} não encontrada para ${card.name}`);
      const account = await resolveAccount(opts.conta, "conta de pagamento");
      const paid = await c.payInvoice(card.id, invoice.id, account.id);
      console.log(
        pc.green(
          `✔ fatura ${opts.pagar} do ${card.name} paga (${formatBRL(paid.totalCents)}) via ${account.name}`,
        ),
      );
      return;
    }

    const rows = invoices.map((i) => [
      `${String(i.month).padStart(2, "0")}/${i.year}`,
      i.status === "PAID"
        ? pc.dim("paga")
        : i.status === "CLOSED"
          ? pc.yellow("fechada")
          : pc.green("aberta"),
      pc.dim(i.dueDate),
      formatBRL(i.totalCents),
    ]);
    console.log(
      renderTable(
        [
          { header: "Período" },
          { header: "Status" },
          { header: "Vencimento" },
          { header: "Total", align: "right" },
        ],
        rows,
      ),
    );
  });

// ───────────────────────── orçamentos e metas ─────────────────────────

program
  .command("orcamentos")
  .description("Orçamentos do mês com progresso")
  .action(async () => {
    const budgets = await client().budgets();
    if (budgets.length === 0) {
      console.log(pc.dim("nenhum orçamento definido"));
      return;
    }
    for (const b of budgets) {
      console.log(
        `${progressBar(b.percent)} ${b.categoryName.padEnd(16)} ${formatBRL(b.spentCents)} / ${formatBRL(b.amountCents)} ${pc.dim(`(${b.percent}%)`)}`,
      );
    }
  });

program
  .command("metas")
  .description("Metas com progresso e aporte mensal necessário")
  .action(async () => {
    const goals = await client().goals();
    if (goals.length === 0) {
      console.log(pc.dim("nenhuma meta criada"));
      return;
    }
    for (const g of goals) {
      const monthly =
        g.monthlyNeededCents !== null
          ? pc.dim(` — guarde ${formatBRL(g.monthlyNeededCents)}/mês`)
          : "";
      console.log(
        `${progressBar(g.percent)} ${g.name.padEnd(24)} ${formatBRL(g.savedCents)} / ${formatBRL(g.targetCents)} ${pc.dim(`(${g.percent}%)`)}${monthly}`,
      );
    }
  });

program
  .command("guardar <valor>")
  .description("Aporta em uma meta (ex.: finflow guardar 200 --meta reserva)")
  .requiredOption("--meta <nome>", "meta")
  .action(async (valor: string, opts: { meta: string }) => {
    const cents = requireCents(valor);
    const goals = await client().goals();
    const goal = resolveByName(goals, opts.meta);
    if (!goal)
      fail(`meta "${opts.meta}" não encontrada. Metas: ${goals.map((g) => g.name).join(", ")}`);
    const updated = await client().contributeGoal(goal.id, { amountCents: cents });
    console.log(
      pc.green(`✔ ${formatBRL(cents)} guardados em "${goal.name}" (${updated.percent}%)`),
    );
  });

// ───────────────────────── exportação e status ─────────────────────────

program
  .command("exportar")
  .description("Exporta o extrato em CSV ou OFX")
  .option("--formato <fmt>", "csv|ofx", "csv")
  .option("--mes <n>", "mês (1-12)")
  .option("--ano <n>", "ano")
  .option("--saida <arquivo>", "arquivo de saída")
  .action(async (opts: { formato: string; mes?: string; ano?: string; saida?: string }) => {
    if (opts.formato !== "csv" && opts.formato !== "ofx") fail("formato deve ser csv ou ofx");
    const content = await client().exportTransactions(opts.formato, {
      month: opts.mes ? Number(opts.mes) : undefined,
      year: opts.ano ? Number(opts.ano) : undefined,
    });
    const file = path.resolve(opts.saida ?? `finflow-extrato.${opts.formato}`);
    writeFileSync(file, content, "utf-8");
    console.log(pc.green(`✔ extrato exportado para ${file}`));
  });

program
  .command("status")
  .description("Verifica a API, a autenticação e a integração com o Telegram")
  .action(async () => {
    const c = client();
    const [summary, telegram, auth] = await Promise.all([
      c.dashboardSummary(),
      c.telegramStatus(),
      c.authStatus(),
    ]);
    console.log(pc.green("✔ API respondendo"));
    console.log(`  Saldo consolidado: ${money(summary.totalBalanceCents)}`);
    console.log(
      `  Autenticação: ${auth.required ? pc.green("protegida por senha") : pc.dim("modo aberto (defina FINFLOW_PASSWORD para proteger)")}`,
    );
    console.log(
      `  Telegram: ${telegram.configured ? pc.green("bot configurado") : pc.dim("bot não configurado")}${telegram.chatLinked ? pc.green(" + chat vinculado") : ""}`,
    );
  });

program.parseAsync().catch((error: unknown) => {
  if (error instanceof FinFlowApiError) {
    fail(
      error.issues
        ? `${error.message} (${error.issues.map((i) => `${i.path}: ${i.message}`).join("; ")})`
        : error.message,
    );
  }
  fail((error as Error).message);
});
