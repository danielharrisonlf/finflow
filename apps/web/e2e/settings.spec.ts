import { expect, test } from "@playwright/test";
import { mockApi } from "./mocks";

test("salva limite mensal, dependentes e plano de gastos", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/configuracoes");

  const form = page.getByTestId("form-settings");
  await form.getByLabel("Limite mensal (deixe vazio para não usar)").fill("4.000,00");
  await form.getByLabel("Tenho dependentes (meta de 12 meses)").check();
  await form.getByLabel("Usar um plano de distribuição").check();
  await expect(form.getByText("Soma: 100%")).toBeVisible();
  await expect(page.getByTestId("local-data")).toContainText("SQLite");

  await form.getByRole("button", { name: "Salvar configurações" }).click();
  await expect(form.getByText("Configurações salvas")).toBeVisible();

  const patch = captured.find((r) => r.method === "PATCH" && r.url.includes("/settings"));
  expect(patch?.body).toMatchObject({
    monthlyLimitCents: 400_000,
    hasDependents: true,
    reserveMonths: 6,
    reserveRefillRatePct: 10,
    balanceBands: { darkGreen: 200_000, lightGreen: 100_000, yellow: 0, lightRed: -50_000 },
    spendPlan: { fixedPct: 50, cuttablePct: 20, variablePct: 30 },
  });
});

test("plano que não soma 100% bloqueia o envio", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/configuracoes");

  const form = page.getByTestId("form-settings");
  await form.getByLabel("Usar um plano de distribuição").check();
  await form.getByLabel("Fixo (%)").fill("70");
  await form.getByRole("button", { name: "Salvar configurações" }).click();

  await expect(form.getByText("a soma precisa dar 100%")).toBeVisible();
  expect(captured.some((r) => r.method === "PATCH")).toBe(false);
});
