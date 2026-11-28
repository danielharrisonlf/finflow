import { expect, type Page, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

// Assistente (v3 §15): só a API do Claude com a chave do usuário; sem chave o recurso fica desativado
// e fora da navegação. Nenhum modelo é chamado — /api/ai/* é interceptado com SSE roteirizado.

const configured = {
  configured: true,
  source: "SETTINGS",
  model: "claude-opus-5-5",
  receiptSupported: true,
};

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

const isMobile = (page: Page) => (page.viewportSize()?.width ?? 1280) < 768;

/** Links de navegação visíveis: barra lateral (desktop) ou sheet "Mais" (mobile). */
async function openNav(page: Page) {
  if (isMobile(page)) {
    await page.getByTestId("mobile-more").click();
    return page.getByRole("dialog", { name: "Todas as seções" });
  }
  return page.getByRole("navigation", { name: "Principal" }).first();
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    if (!sessionStorage.getItem("ff-e2e-keep-history")) {
      localStorage.removeItem("finflow_assistant_history");
    }
  });
});

test("sem chave: Assistente fora da navegação e da paleta; /assistente aponta para Configurações", async ({
  page,
}) => {
  const captured = await mockApi(page);
  await page.goto("/");
  await expect(page.getByTestId("hero-balance")).toBeVisible();

  const nav = await openNav(page);
  await expect(nav.getByRole("link", { name: "Projeção" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Assistente" })).toHaveCount(0);
  if (isMobile(page)) await page.keyboard.press("Escape");
  // Nenhuma promoção do Assistente nas páginas.
  await expect(page.getByRole("link", { name: /Assistente/ })).toHaveCount(0);

  await page.keyboard.press("Control+k");
  const palette = page.getByTestId("command-palette");
  await palette.getByRole("combobox").fill("assist");
  await expect(palette.getByRole("option", { name: "Assistente" })).toHaveCount(0);
  await page.keyboard.press("Escape");

  await page.goto("/assistente");
  const disabled = page.getByTestId("ai-disabled");
  await expect(
    disabled.getByRole("heading", { name: "O Assistente está desativado" }),
  ).toBeVisible();
  await expect(disabled).toContainText("Configurações → Assistente (IA)");
  // Sem chat, sem anexo de comprovante e sem o "Posso comprar?" (que fica em Projeção/Desejos).
  await expect(page.getByLabel("Pergunta para o assistente")).toHaveCount(0);
  await expect(page.getByTestId("ai-attach-input")).toHaveCount(0);
  await expect(page.getByTestId("can-afford")).toHaveCount(0);

  await disabled.getByRole("link", { name: "Abrir Configurações" }).click();
  await expect(page).toHaveURL(/\/configuracoes#assistente$/);
  await expect(page.getByTestId("ai-settings")).toBeInViewport();
  expect(captured.some((r) => r.url.includes("/api/ai/"))).toBe(false);
});

test("salvar a chave em Configurações ativa o Assistente; remover desativa de novo", async ({
  page,
}) => {
  const captured = await mockApi(page);
  await page.goto("/configuracoes");

  const section = page.getByTestId("ai-settings");
  await expect(section.getByRole("heading", { name: "Assistente (IA)" })).toBeVisible();
  await expect(section.getByTestId("ai-settings-status")).toContainText("Desativado");
  let nav = await openNav(page);
  await expect(nav.getByRole("link", { name: "Assistente" })).toHaveCount(0);
  if (isMobile(page)) await page.keyboard.press("Escape");

  await section.getByLabel("Chave da API do Claude").fill("sk-ant-api03-chave-de-teste-0000");
  await section.getByRole("button", { name: "Salvar" }).click();
  await expect(section.getByTestId("ai-settings-status")).toContainText("Ativo");
  await expect(section.getByText("Chave salva. O Assistente está ativo.")).toBeVisible();
  await expect(section.getByLabel("Chave da API do Claude")).toHaveValue("");
  const put = captured.find((r) => r.method === "PUT" && r.url.endsWith("/api/ai/key"));
  expect(put?.body).toEqual({ apiKey: "sk-ant-api03-chave-de-teste-0000" });

  nav = await openNav(page);
  await nav.getByRole("link", { name: "Assistente" }).click();
  await expect(page).toHaveURL(/\/assistente$/);
  await expect(page.getByText("Pergunte sobre o seu dinheiro")).toBeVisible();
  await expect(page.getByTestId("ai-attach-input")).toHaveCount(1);

  await page.goto("/configuracoes");
  await section.getByRole("button", { name: "Remover chave" }).click();
  await expect(section.getByTestId("ai-settings-status")).toContainText("Desativado");
  await expect(section.getByText("Chave removida. O Assistente foi desativado.")).toBeVisible();
  const removed = captured.filter((r) => r.method === "PUT" && r.url.endsWith("/api/ai/key"));
  expect(removed.at(-1)?.body).toEqual({ apiKey: null });
  nav = await openNav(page);
  await expect(nav.getByRole("link", { name: "Assistente" })).toHaveCount(0);
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
