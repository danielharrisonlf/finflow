import { expect, test } from "@playwright/test";
import { fixtures, mockApi } from "./mocks";

test("dashboard mostra quanto pode gastar hoje, ritmo acumulado e evolução do patrimônio", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/");

  const spendToday = page.getByTestId("spend-today");
  await expect(spendToday).toContainText("R$ 171,87");
  await expect(spendToday).toContainText("pelo seu limite mensal");

  const pace = page.getByTestId("pace-chart");
  await expect(pace.locator("svg")).toBeVisible();
  await expect(pace).toContainText("Mês passado");
  await expect(pace).toContainText("Projeção");

  const netWorth = page.getByTestId("networth-chart");
  await expect(netWorth.locator("svg")).toBeVisible();
  await expect(netWorth).toContainText("Evolução do patrimônio");
});

test("dashboard continua funcionando sem os campos novos do v3", async ({ page }) => {
  await mockApi(page);
  const { series: _series, projectedMonthCents: _projected, ...pace } = fixtures.overview.pace;
  const { spendToday: _spendToday, ...overview } = fixtures.overview;
  await page.route("**/api/insights/overview*", (route) =>
    route.fulfill({ contentType: "application/json", body: JSON.stringify({ ...overview, pace }) }),
  );
  await page.route("**/api/insights/net-worth*", (route) =>
    route.fulfill({ contentType: "application/json", body: "{}" }),
  );
  await page.goto("/");

  await expect(page.getByTestId("pace-card")).toContainText("15% a mais");
  await expect(page.getByTestId("spend-today")).toHaveCount(0);
  await expect(page.getByTestId("pace-chart")).toHaveCount(0);
  await expect(page.getByTestId("networth-chart")).toContainText("Sem histórico ainda");
});

test("fatura explica parcelas, assinaturas e compras avulsas", async ({ page }) => {
  await mockApi(page);
  await page.goto("/contas");

  await page.getByTestId("card-Nubank Ultravioleta").click();
  await page.getByRole("button", { name: "Entender minha fatura" }).click();

  const panel = page.getByTestId("invoice-breakdown");
  await expect(panel).toContainText("Parcelas");
  await expect(panel).toContainText("R$ 400,00");
  await expect(panel).toContainText("Assinaturas");
  await expect(panel).toContainText("R$ 49,90");
  await expect(panel).toContainText("Compras avulsas");
  await expect(panel).toContainText("R$ 353,90");
});
