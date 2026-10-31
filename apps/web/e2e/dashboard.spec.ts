import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

test.beforeEach(async ({ page }) => {
  await mockApi(page);
  await page.goto("/");
});

test("dashboard mostra saldo, métricas do mês e faturas", async ({ page }) => {
  const hero = page.getByTestId("hero-balance");
  await expect(hero).toContainText("Saldo geral");
  await expect(hero).toContainText("12.345,67");
  await expect(hero).toContainText("733,80"); // faturas em aberto
  await expect(hero).toContainText("2 lançamentos"); // pendências

  await expect(page.getByText("Receitas")).toBeVisible();
  await expect(page.getByText("9.850,00")).toBeVisible();
  await expect(page.getByText("-R$ 4.312,50")).toBeVisible();
  await expect(page.getByText("5.537,50")).toBeVisible();
});

test("gráficos e distribuição por categoria renderizam", async ({ page }) => {
  await expect(page.getByTestId("cashflow-chart").locator("svg").first()).toBeVisible();
  const donut = page.getByTestId("category-donut");
  await expect(donut.locator("svg").first()).toBeVisible();
  await expect(page.getByText("Moradia")).toBeVisible();
  await expect(page.getByText("61.4%")).toBeVisible();
});

test("próximos vencimentos listam pendências e fatura do cartão", async ({ page }) => {
  await expect(page.getByText("Fatura internet (vence)")).toBeVisible();
  await expect(page.getByText("Fatura Nubank Ultravioleta")).toBeVisible();
});

test("navegação muda de mês no cabeçalho", async ({ page }) => {
  const label = page.locator("header span.tnum").first();
  const before = await label.textContent();
  await page.getByRole("button", { name: "Mês anterior" }).click();
  await expect(label).not.toHaveText(before ?? "");
});
