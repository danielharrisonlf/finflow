import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

test("Ctrl+K abre a paleta e navega para uma página", async ({ page }) => {
  await mockApi(page);
  await page.goto("/");
  await expect(page.getByTestId("hero-balance")).toBeVisible();

  await page.keyboard.press("Control+k");
  const palette = page.getByTestId("command-palette");
  await expect(palette).toBeVisible();
  const input = palette.getByRole("combobox");
  await expect(input).toBeFocused();

  await input.fill("investi");
  await expect(palette.getByRole("option", { name: "Investimentos" })).toBeVisible();
  await input.press("Enter");
  await expect(palette).not.toBeVisible();
  await expect(page).toHaveURL(/\/investimentos$/);
});

test("busca lançamentos em todo o histórico e abre no extrato", async ({ page }) => {
  await mockApi(page);
  await page.goto("/");
  await page.locator('[aria-label="Buscar"]:visible, [data-testid="open-palette"]:visible').click();

  const palette = page.getByTestId("command-palette");
  const searched = page.waitForRequest(
    (r) => r.url().includes("/api/transactions") && r.url().includes("search=almo"),
  );
  await palette.getByRole("combobox").fill("almo");
  await searched;
  const option = palette.getByRole("option", { name: /Almoço — rodízio japonês/ });
  await expect(option).toBeVisible();
  await option.click();

  await expect(page).toHaveURL(/\/extrato\?search=/);
  await expect(page.getByPlaceholder("Buscar descrição…")).toHaveValue("Almoço — rodízio japonês");
  await expect(page.getByText("Outubro 2026")).toBeVisible();
});

test("ações rápidas: nova transação e posso comprar", async ({ page }) => {
  await mockApi(page);
  await page.goto("/extrato");

  await page.keyboard.press("Control+k");
  const palette = page.getByTestId("command-palette");
  await palette.getByRole("combobox").fill("nova");
  await palette.getByRole("option", { name: "Nova transação" }).click();
  await expect(page.getByRole("dialog", { name: "Novo lançamento" })).toBeVisible();
  await page.keyboard.press("Escape");

  await page.keyboard.press("Control+k");
  await palette.getByRole("combobox").fill("posso");
  await palette.getByRole("combobox").press("Enter");
  await expect(page).toHaveURL(/\/projecao#posso-comprar$/);
  await expect(page.getByTestId("can-afford").getByLabel("Valor da compra")).toBeFocused();
});
