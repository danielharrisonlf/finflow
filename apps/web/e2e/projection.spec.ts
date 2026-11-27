import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

test.beforeEach(async ({ page }) => {
  await mockApi(page);
  await page.goto("/projecao");
});

test("projeção desenha o saldo diário com as cinco faixas e alerta o menor saldo", async ({
  page,
}) => {
  const chart = page.getByTestId("projection-chart");
  await expect(chart.locator("svg")).toBeVisible();
  for (const band of ["DARK_GREEN", "LIGHT_GREEN", "YELLOW", "LIGHT_RED", "RED"]) {
    await expect(chart.locator(`rect[data-band="${band}"]`).first()).toBeAttached();
  }

  const alert = page.getByTestId("lowest-alert");
  await expect(alert).toHaveAttribute("role", "alert");
  await expect(alert).toContainText("negativo");
  await expect(alert).toContainText("800,00");

  await expect(page.getByTestId("can-save")).toContainText("R$ 0,00");
  await expect(page.getByText("11 de dezembro de 2026")).toBeVisible();
});

test("saldo negativo mostra o plano de ação e o aviso de limite não configurado", async ({
  page,
}) => {
  const plan = page.getByTestId("action-plan");
  await expect(plan.locator("li")).toHaveCount(5);
  await expect(plan).toContainText("Renegocie as dívidas mais caras primeiro");

  const notice = page.getByTestId("limit-notice");
  await expect(notice.getByRole("link", { name: "Definir limite mensal" })).toHaveAttribute(
    "href",
    "/configuracoes",
  );

  await expect(page.getByTestId("months-table").locator("tbody tr")).toHaveCount(12);
});

test("página não rola na horizontal em 320px", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 720 });
  await page.reload();
  await expect(page.getByTestId("projection-chart").locator("svg")).toBeVisible();
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
});
