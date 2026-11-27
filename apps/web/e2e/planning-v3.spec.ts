import { expect, test } from "@playwright/test";
import { fixtures, ids, mockApi } from "./mocks";

test("compromissos: assinaturas detectadas com alta de preço, acompanhar e ignorar", async ({
  page,
}) => {
  const captured = await mockApi(page);
  await page.goto("/compromissos");

  const card = page.getByTestId("subscriptions-card");
  await expect(card.getByTestId("subscriptions-total")).toContainText("R$ 619,00");
  const list = card.getByTestId("subscriptions-list");
  const streaming = list.getByRole("listitem").filter({ hasText: "Streaming de vídeo" });
  await expect(streaming.getByTestId("price-change")).toContainText("+11,1%");
  await expect(list.getByRole("listitem").filter({ hasText: "Academia" })).toContainText(
    "Acompanhada",
  );

  await streaming.getByRole("button", { name: "Acompanhar Streaming de vídeo" }).click();
  await expect.poll(() => captured.find((r) => r.url.includes("/track"))).toBeDefined();
  expect(captured.find((r) => r.url.includes("/track"))?.url).toContain(
    `/api/insights/subscriptions/${encodeURIComponent("streaming video")}/track`,
  );

  await streaming.getByRole("button", { name: "Ignorar Streaming de vídeo" }).click();
  await expect
    .poll(() =>
      captured.find((r) => r.url.includes("/subscriptions/") && r.url.endsWith("/dismiss")),
    )
    .toBeDefined();

  // parcela de empréstimo aparece nos compromissos
  await expect(page.getByTestId("commitment-items")).toContainText("Financiamento do carro");
  await expect(page.getByTestId("commitment-items")).toContainText("empréstimo · parcela 13/48");
});

test("extrato sugere transformar pares em transferência", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/extrato");

  const banner = page.getByTestId("transfer-suggestions");
  await expect(banner).toContainText("Isto parece uma transferência entre suas contas");
  await expect(banner).toContainText("de Nubank para Poupança");
  await banner.getByRole("button", { name: "Transformar em transferência" }).click();

  await expect
    .poll(() => captured.find((r) => r.url.endsWith("/api/transactions/merge-transfer")))
    .toBeDefined();
  expect(captured.find((r) => r.url.endsWith("/api/transactions/merge-transfer"))?.body).toEqual({
    expenseId: ids.txTransferOut,
    incomeId: ids.txTransferIn,
  });
});

test("posso comprar? na projeção: veredito, limite e quando cabe", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/projecao");

  const form = page.getByTestId("can-afford");
  await form.getByLabel("Valor da compra").fill("3.000,00");
  await form.getByLabel("Pagar com").selectOption({ label: "Cartão Nubank Ultravioleta" });
  await form.getByLabel("Parcelas").selectOption("10");
  await form.getByRole("button", { name: "Simular compra" }).click();

  const result = form.getByTestId("can-afford-result");
  await expect(result).toHaveAttribute("data-verdict", "CAREFUL");
  await expect(result).toContainText("Dá, mas com cuidado");
  await expect(result).toContainText("116,3%");
  const req = captured.find((r) => r.url.endsWith("/api/insights/can-afford"));
  expect(req?.body).toMatchObject({ amountCents: 300_000, cardId: ids.card, installments: 10 });

  await form.getByLabel("Valor da compra").fill("9.000,00");
  await form.getByRole("button", { name: "Simular compra" }).click();
  await expect(result).toHaveAttribute("data-verdict", "NO");
  await expect(result).toContainText("11 de fevereiro de 2027");
});

test("desejos: prévia do link preenche nome, preço e imagem; posso comprar no item", async ({
  page,
}) => {
  const captured = await mockApi(page);
  await page.goto("/metas");
  await page.getByRole("tab", { name: "Desejos" }).click();

  await page.getByRole("button", { name: "Novo desejo" }).click();
  const form = page.getByTestId("form-wish");
  await form.getByLabel("Link do produto (opcional)").fill("https://loja.example/cafeteira");
  await form.getByRole("button", { name: "Buscar" }).click();
  await expect(form.getByLabel("O que você quer")).toHaveValue(fixtures.linkPreview.title);
  await expect(form.getByLabel("Preço")).toHaveValue("1299,00");
  await expect(form.locator("img")).toHaveAttribute("src", fixtures.linkPreview.imageUrl);
  await form.getByRole("button", { name: "Adicionar à lista" }).click();

  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/wishlist")))
    .toBeDefined();
  expect(
    captured.find((r) => r.method === "POST" && r.url.endsWith("/api/wishlist"))?.body,
  ).toMatchObject({
    name: fixtures.linkPreview.title,
    priceCents: 129_900,
    url: "https://loja.example/cafeteira",
    imageUrl: fixtures.linkPreview.imageUrl,
  });

  await page.getByRole("button", { name: "Posso comprar Fone com cancelamento de ruído?" }).click();
  const sheet = page.getByRole("dialog", { name: /Posso comprar\? — Fone/ });
  await expect(sheet.getByLabel("Valor da compra")).toHaveValue("899,90");
  await sheet.getByRole("button", { name: "Simular compra" }).click();
  await expect(sheet.getByTestId("can-afford-result")).toHaveAttribute("data-verdict", "YES");
});

test("metas: modelo preenche nome, ícone e cor", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/metas");
  await page.getByRole("button", { name: "Nova meta" }).click();

  const form = page.getByTestId("form-goal");
  const templates = form.getByTestId("goal-templates");
  for (const name of [
    "Casa própria",
    "Filho",
    "Período sabático",
    "Viagem",
    "Veículo",
    "Casamento",
    "Reserva de emergência",
  ]) {
    await expect(templates.getByRole("button", { name })).toBeVisible();
  }
  await templates.getByRole("button", { name: "Período sabático" }).click();
  await expect(form.getByLabel("Nome")).toHaveValue("Período sabático");
  await form.getByLabel("Valor alvo").fill("60.000,00");
  await form.getByRole("button", { name: "Criar meta" }).click();

  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/goals")))
    .toBeDefined();
  const body = captured.find((r) => r.method === "POST" && r.url.endsWith("/api/goals"))?.body;
  expect(body).toMatchObject({
    name: "Período sabático",
    icon: "tree-palm",
    color: "#34d399",
    targetCents: 6_000_000,
  });
  expect(body).not.toHaveProperty("kind");
});
