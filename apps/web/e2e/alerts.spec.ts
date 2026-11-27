import { expect, test } from "@playwright/test";
import { fixtures, mockApi } from "./mocks";

test("sino mostra o contador, lista os alertas e dispensa", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/");

  const bell = page.locator('[data-testid="alerts-bell"]:visible');
  await expect(bell).toHaveAccessibleName("Alertas (3)");
  await expect(bell.getByTestId("alerts-count")).toHaveText("3");

  await bell.click();
  const dialog = page.getByRole("dialog", { name: "Alertas e dicas" });
  const list = dialog.getByTestId("alerts-list");
  await expect(list.getByRole("listitem")).toHaveCount(3);
  await expect(list).toContainText("Saldo negativo à vista");
  await expect(list).toContainText("R$ 552,00 de R$ 600,00");

  await dialog.getByRole("button", { name: /Dispensar alerta: Assinatura mais cara/ }).click();
  await expect(list.getByRole("listitem")).toHaveCount(2);
  const dismiss = captured.find((r) => r.method === "POST" && r.url.includes("/dismiss"));
  expect(dismiss?.url).toContain(
    `/api/alerts/${encodeURIComponent(fixtures.alerts[2]!.id)}/dismiss`,
  );

  await list.getByRole("listitem").first().getByRole("link", { name: "Resolver" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page).toHaveURL(/\/projecao$/);
});

test("sem alertas, o sino não mostra contador e explica o feed", async ({ page }) => {
  await mockApi(page);
  await page.route("**/api/alerts", (route) =>
    route.fulfill({ contentType: "application/json", body: "[]" }),
  );
  await page.goto("/");
  const bell = page.locator('[data-testid="alerts-bell"]:visible');
  await expect(bell).toHaveAccessibleName("Alertas");
  await bell.click();
  await expect(page.getByRole("dialog")).toContainText("Tudo em dia");
});

test("resposta inesperada da API de alertas não quebra o app", async ({ page }) => {
  await mockApi(page);
  await page.route("**/api/alerts", (route) =>
    route.fulfill({ contentType: "application/json", body: "{}" }),
  );
  await page.goto("/");
  await expect(page.getByTestId("hero-balance")).toBeVisible();
  await expect(page.locator('[data-testid="alerts-bell"]:visible')).toHaveAccessibleName("Alertas");
});
