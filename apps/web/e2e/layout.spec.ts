import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

const ROUTES = [
  "/",
  "/compromissos",
  "/ano",
  "/projecao",
  "/planilhas",
  "/metas",
  "/categorias",
  "/configuracoes",
  "/extrato",
];

test("nenhuma página rola na horizontal em 320px", async ({ page }) => {
  await mockApi(page);
  await page.setViewportSize({ width: 320, height: 720 });
  for (const route of ROUTES) {
    await page.goto(route);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow, `scroll horizontal em ${route}`).toBeLessThanOrEqual(0);
  }
});
