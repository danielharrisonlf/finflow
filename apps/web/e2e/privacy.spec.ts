import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

test("modo privacidade mascara valores em todo o app e persiste no navegador", async ({ page }) => {
  await mockApi(page);
  await page.goto("/");

  const hero = page.getByTestId("hero-balance");
  await expect(hero).toContainText("12.345,67");

  const toggle = page.locator('[data-testid="privacy-toggle"]:visible');
  await expect(toggle).toHaveAttribute("aria-pressed", "false");
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-pressed", "true");
  await expect(toggle).toHaveAccessibleName("Mostrar valores");

  await expect(hero).toContainText("R$ •••••");
  await expect(hero).not.toContainText("12.345,67");
  await expect(page.getByTestId("month-stats")).not.toContainText("9.850,00");
  await expect(page.getByTestId("networth")).not.toContainText("11.611,87");

  // vale em outras páginas e sobrevive ao recarregar
  await page.goto("/extrato");
  await expect(page.getByTestId("transactions-list")).toContainText("R$ •••••");
  await expect(page.getByTestId("transactions-list")).not.toContainText("129,00");
  await page.reload();
  await expect(page.getByTestId("transactions-list")).toContainText("R$ •••••");

  // textos da API com valores também são mascarados
  await page.locator('[data-testid="alerts-bell"]:visible').click();
  await expect(page.getByTestId("alerts-list")).not.toContainText("552,00");
  await page.keyboard.press("Escape");

  await page.locator('[data-testid="privacy-toggle"]:visible').click();
  await expect(page.getByTestId("transactions-list")).toContainText("129,00");
});
