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
    // bottom nav visível, sidebar não
    await expect(page.getByRole("navigation").last()).toBeVisible();
    await page.getByRole("link", { name: "Orçamentos" }).click();
    await expect(page.getByTestId("budgets-grid")).toBeVisible();
    await page.getByRole("link", { name: "Metas" }).click();
    await expect(page.getByTestId("goals-grid")).toBeVisible();
  } else {
    await page.getByRole("link", { name: "Contas & Cartões" }).click();
    await expect(page.getByTestId("accounts-grid")).toBeVisible();
    await page.getByRole("link", { name: "Orçamentos" }).click();
    await expect(page.getByTestId("budgets-grid")).toBeVisible();
  }
});

test("orçamento estourado aparece em vermelho com percentual", async ({ page }) => {
  await page.goto("/orcamentos");
  await expect(page.getByText("quase estourado")).toBeVisible();
  await expect(page.getByText("92%")).toBeVisible();
});

test("meta mostra progresso e aporte mensal necessário", async ({ page }) => {
  await page.goto("/metas");
  await expect(page.getByText("Reserva de emergência")).toBeVisible();
  await expect(page.getByText("35%")).toBeVisible();
  await expect(page.getByText("R$ 1.500,00/mês")).toBeVisible();
});
