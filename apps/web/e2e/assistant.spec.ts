import { expect, type Page, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

// Assistente (v3 §15): a API do Claude nunca é chamada — /api/ai/* é interceptado com SSE roteirizado.

const configured = { configured: true, source: "SETTINGS", model: "claude-opus-5-5" };

function sse(events: [string, unknown][]): string {
  return events.map(([name, data]) => `event: ${name}\ndata: ${JSON.stringify(data)}\n\n`).join("");
}

/** Registrado depois do mockApi, tem precedência sobre ele. */
async function mockAi(
  page: Page,
  opts: { status?: unknown; chat?: string; receipt?: unknown } = {},
): Promise<{ chatBodies: unknown[]; keyBodies: unknown[] }> {
  const chatBodies: unknown[] = [];
  const keyBodies: unknown[] = [];
  await page.route("**/api/ai/**", (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path.endsWith("/ai/status")) {
      return route.fulfill({ json: opts.status ?? configured });
    }
    if (path.endsWith("/ai/chat")) {
      chatBodies.push(request.postDataJSON());
      return route.fulfill({
        status: 200,
        headers: { "Content-Type": "text/event-stream" },
        body: opts.chat ?? sse([["done", { stopReason: "end_turn" }]]),
      });
    }
    if (path.endsWith("/ai/receipt")) {
      return route.fulfill({ json: opts.receipt ?? {} });
    }
    if (path.endsWith("/ai/key")) {
      keyBodies.push(request.postDataJSON());
      return route.fulfill({ json: configured });
    }
    return route.fulfill({ status: 404, json: { message: "rota" } });
  });
  return { chatBodies, keyBodies };
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    if (!sessionStorage.getItem("ff-e2e-keep-history")) {
      localStorage.removeItem("finflow_assistant_history");
    }
  });
});

test("sem chave: mostra configuração e o simulador Posso comprar?", async ({ page }) => {
  const captured = await mockApi(page);
  const { keyBodies } = await mockAi(page, {
    status: { configured: false, source: null, model: "claude-opus-5-5" },
  });
  await page.goto("/assistente");

  const setup = page.getByTestId("ai-setup");
  await expect(setup.getByRole("heading", { name: "Ative o Assistente" })).toBeVisible();
  await expect(setup.getByRole("link", { name: /console\.anthropic\.com/ })).toHaveAttribute(
    "href",
    "https://console.anthropic.com/",
  );

  const simulator = page.getByTestId("can-afford");
  await simulator.getByLabel("Valor total").fill("6.000,00");
  await simulator.getByRole("button", { name: "Simular" }).click();
  await expect(page.getByTestId("can-afford-result")).toContainText("Não cabe agora");
  const afford = captured.find((r) => r.url.includes("/insights/can-afford"));
  expect(afford?.body).toMatchObject({ amountCents: 600_000, installments: 1 });

  await setup.getByLabel("Chave da API do Claude").fill("sk-ant-api03-chave-de-teste-0000");
  await setup.getByRole("button", { name: "Salvar chave" }).click();
  await expect(page.getByText("Pergunte sobre o seu dinheiro")).toBeVisible();
  expect(keyBodies).toEqual([{ apiKey: "sk-ant-api03-chave-de-teste-0000" }]);
});

test("chat: ferramentas, resposta em markdown seguro e proposta confirmada", async ({ page }) => {
  const captured = await mockApi(page);
  const chat = sse([
    ["tool", { name: "get_category_spending", label: "Somando seus gastos por categoria" }],
    ["text", { delta: "Você gasta mais com **Alimentação**:\n" }],
    ["text", { delta: "- R$ 1.200,00 no mês\n- <img src=x onerror=alert(1)>" }],
    [
      "proposal",
      {
        id: "p1",
        kind: "CREATE_TRANSACTION",
        summary: "Despesa de R$ 45,90 — Mercado, em 20/11/2026 (Nubank)",
        payload: {
          type: "EXPENSE",
          description: "Mercado",
          amountCents: 4590,
          date: "2026-11-20",
          status: "CONFIRMED",
          accountId: ids.accountNubank,
        },
      },
    ],
    ["done", { stopReason: "end_turn" }],
  ]);
  const { chatBodies } = await mockAi(page, { chat });
  await page.goto("/assistente");

  for (const s of [
    "Posso jantar fora esse mês?",
    "Posso fazer uma viagem de R$ 3.000 em março?",
    "Posso comprar um celular de R$ 5.000 em 10x?",
  ]) {
    await expect(page.getByRole("button", { name: s })).toBeVisible();
  }
  await page.getByRole("button", { name: "Onde estou gastando mais?" }).click();

  const answer = page.getByTestId("ai-answer");
  await expect(answer.locator("strong")).toHaveText("Alimentação");
  await expect(answer.locator("li")).toHaveCount(2);
  await expect(answer).toContainText("<img src=x onerror=alert(1)>");
  await expect(answer.locator("img")).toHaveCount(0);
  await expect(page.getByTestId("ai-tool-chip")).toHaveText("Somando seus gastos por categoria");

  expect(chatBodies[0]).toEqual({
    messages: [{ role: "user", content: "Onde estou gastando mais?" }],
  });

  const proposal = page.getByTestId("ai-proposal");
  await expect(proposal).toContainText("Despesa de R$ 45,90");
  await proposal.getByRole("button", { name: "Confirmar" }).click();
  await expect(proposal).toContainText("Confirmado e salvo");
  const post = captured.find((r) => r.method === "POST" && r.url.endsWith("/api/transactions"));
  expect(post?.body).toMatchObject({ amountCents: 4590, accountId: ids.accountNubank });

  // Segunda pergunta leva o histórico (só texto).
  await page.getByLabel("Pergunta para o assistente").fill("E no mês passado?");
  await page.getByLabel("Pergunta para o assistente").press("Enter");
  await expect.poll(() => chatBodies.length).toBe(2);
  const second = chatBodies[1] as { messages: { role: string; content: string }[] };
  expect(second.messages.map((m) => m.role)).toEqual(["user", "assistant", "user"]);

  // Histórico sobrevive ao recarregar.
  await page.evaluate(() => sessionStorage.setItem("ff-e2e-keep-history", "1"));
  await page.reload();
  await expect(page.getByTestId("ai-message-user").first()).toHaveText("Onde estou gastando mais?");
  await expect(page.getByTestId("ai-proposal").first()).toContainText("Confirmado e salvo");
});

test("recusa: descarta a resposta parcial e mostra mensagem amigável", async ({ page }) => {
  await mockApi(page);
  await mockAi(page, {
    chat: sse([
      ["text", { delta: "Começando a responder" }],
      ["error", { message: "Não posso ajudar com esse pedido." }],
      ["done", { stopReason: "refusal" }],
    ]),
  });
  await page.goto("/assistente");
  await page.getByLabel("Pergunta para o assistente").fill("algo recusado");
  await page.getByRole("button", { name: "Enviar" }).click();
  await expect(page.getByRole("alert")).toHaveText("Não posso ajudar com esse pedido.");
  await expect(page.getByText("Começando a responder")).toHaveCount(0);
});

test("comprovante abre o lançamento preenchido; planilha vai para /planilhas", async ({ page }) => {
  await mockApi(page);
  await mockAi(page, {
    receipt: {
      description: "Supermercado Bom Preço",
      amountCents: 12345,
      date: "2026-11-25",
      kind: "EXPENSE",
      suggestedCategoryId: ids.catFood,
      merchant: "BOM PRECO LTDA",
      installments: null,
      confidence: "HIGH",
    },
  });
  await page.goto("/assistente");

  await page.getByTestId("ai-attach-input").setInputFiles({
    name: "nota.png",
    mimeType: "image/png",
    buffer: Buffer.from("89504e470d0a1a0a", "hex"),
  });
  const form = page.getByTestId("form-expense");
  await expect(form).toBeVisible();
  await expect(form.getByPlaceholder("0,00")).toHaveValue("123,45");
  await expect(form.getByLabel("Descrição")).toHaveValue("Supermercado Bom Preço");
  await expect(form.getByLabel("Data")).toHaveValue("2026-11-25");
  await expect(form.getByLabel("Categoria")).toHaveValue(ids.catFood);
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("ai-answer")).toContainText("Li o comprovante");

  await page.getByTestId("ai-attach-input").setInputFiles({
    name: "extrato.ofx",
    mimeType: "application/octet-stream",
    buffer: Buffer.from("OFXHEADER:100"),
  });
  await expect(page).toHaveURL(/\/planilhas$/);
});
