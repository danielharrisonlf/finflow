import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

test.beforeEach(async ({ page }) => {
  await mockApi(page);
  await page.goto("/");
});

test("navegação entre as páginas principais", async ({ page }) => {
  const width = page.viewportSize()?.width ?? 1280;
  const mobile = width < 768;

  if (mobile) {
    // bottom nav com atalhos fixos; o resto fica no sheet "Mais"
    await page.getByRole("link", { name: "Agenda" }).click();
    await expect(page.getByRole("heading", { name: "Compromissos" })).toBeVisible();
    await page.getByTestId("mobile-more").click();
    const more = page.getByRole("dialog", { name: "Todas as seções" });
    await more.getByRole("link", { name: "Orçamentos" }).click();
    await expect(more).not.toBeVisible();
    await expect(page.getByTestId("budgets-grid")).toBeVisible();
    await page.getByTestId("mobile-more").click();
    await page.getByRole("dialog").getByRole("link", { name: "Metas e desejos" }).click();
    await expect(page.getByTestId("goals-grid")).toBeVisible();
  } else {
    const sidebar = page.getByRole("navigation", { name: "Principal" }).first();
    await expect(sidebar.getByText("Visão estratégica")).toBeVisible();
    await sidebar.getByRole("link", { name: "Contas e cartões" }).click();
    await expect(page.getByTestId("accounts-grid")).toBeVisible();
    await sidebar.getByRole("link", { name: "Orçamentos" }).click();
    await expect(page.getByTestId("budgets-grid")).toBeVisible();
    await sidebar.getByRole("link", { name: "Projeção" }).click();
    await expect(page.getByTestId("projection-chart")).toBeVisible();
  }
});

test("orçamento estourado aparece em vermelho com percentual", async ({ page }) => {
  await page.goto("/orcamentos");
  await expect(page.getByText("quase estourado")).toBeVisible();
  await expect(page.getByText("92%")).toBeVisible();
});

test("meta mostra progresso e aporte mensal necessário", async ({ page }) => {
  await page.goto("/metas");
  const grid = page.getByTestId("goals-grid");
  await expect(grid.getByText("Reserva de emergência")).toBeVisible();
  await expect(grid.getByText("35%")).toBeVisible();
  await expect(grid.getByText("R$ 1.500,00/mês")).toBeVisible();
});
