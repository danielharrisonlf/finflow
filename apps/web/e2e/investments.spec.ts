import { expect, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

// "Hoje" fixo para o aviso de valor desatualizado (> 45 dias) ser determinístico.
test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-11-27T12:00:00Z"));
});

test("resumo, alocação, evolução e posições da carteira", async ({ page }) => {
  await mockApi(page);
  await page.goto("/investimentos");

  const summary = page.getByTestId("investments-summary");
  await expect(summary).toContainText("15.500,00");
  await expect(summary).toContainText("15.000,00");
  await expect(summary).toContainText("+R$ 500,00");
  await expect(summary).toContainText("3,3%");

  const allocation = page.getByTestId("investments-allocation");
  await expect(allocation).toContainText("Renda fixa");
  await expect(allocation).toContainText("Ações");
  await expect(allocation).toContainText("Corretora B");
  await expect(page.getByTestId("investments-history").getByRole("img")).toBeVisible();

  const list = page.getByTestId("investments-list");
  await expect(list).toContainText("CDB liquidez diária");
  await expect(list).toContainText("Carteira de ações");

  const stocks = page.getByTestId(`investment-${ids.investStocks}`);
  await expect(stocks.getByTestId("investment-stale")).toContainText(
    "valor não atualizado há 87 dias",
  );
  await expect(
    page.getByTestId(`investment-${ids.investCdb}`).getByTestId("investment-stale"),
  ).toHaveCount(0);
});

test("aporte com conta de origem envia o valor e a conta", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/investimentos");

  await page.getByRole("button", { name: "Aportar ou resgatar em CDB liquidez diária" }).click();
  const form = page.getByTestId("form-investment-move");
  await form.getByLabel("Valor").fill("250,00");
  await form.getByLabel("Tirar dinheiro da conta (opcional)").selectOption(ids.accountNubank);
  await form.getByRole("button", { name: "Registrar aporte" }).click();

  const find = () =>
    captured.find(
      (r) => r.method === "POST" && r.url.includes(`/investments/${ids.investCdb}/contribution`),
    );
  await expect.poll(find).toBeDefined();
  expect(find()?.body).toMatchObject({ amountCents: 25_000, fromAccountId: ids.accountNubank });
});

test("atualizar valor grava o novo saldo da posição", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/investimentos");

  await page.getByRole("button", { name: "Atualizar valor de Carteira de ações" }).click();
  const form = page.getByTestId("form-investment-value");
  await expect(form.getByLabel("Valor atual")).toHaveValue("4650,00");
  await form.getByLabel("Valor atual").fill("4.800,00");
  await form.getByRole("button", { name: "Salvar valor" }).click();

  const find = () =>
    captured.find(
      (r) => r.method === "POST" && r.url.includes(`/investments/${ids.investStocks}/value`),
    );
  await expect.poll(find).toBeDefined();
  expect(find()?.body).toMatchObject({ currentCents: 480_000, date: "2026-11-27" });
});
