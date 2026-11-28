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

test("Assistente (IA): explica o recurso, pede a chave e linka o console da Anthropic", async ({
  page,
}) => {
  const captured = await mockApi(page);
  await page.goto("/configuracoes");

  const section = page.getByTestId("ai-settings");
  await expect(section.getByRole("heading", { name: "Assistente (IA)" })).toBeVisible();
  await expect(section).toContainText("usa a API do Claude com a sua chave");
  await expect(section).toContainText("cobrado pela Anthropic");
  await expect(section).toContainText("Todo o resto do FinFlow funciona sem ele.");
  await expect(section.getByTestId("ai-settings-status")).toContainText("Desativado");
  await expect(section.getByLabel("Chave da API do Claude")).toHaveAttribute("type", "password");
  await expect(section.getByRole("button", { name: "Remover chave" })).toHaveCount(0);
  await expect(section.getByRole("link", { name: /console\.anthropic\.com/ })).toHaveAttribute(
    "href",
    "https://console.anthropic.com/",
  );

  // Sem chave digitada: aviso, nenhuma requisição.
  await section.getByRole("button", { name: "Salvar" }).click();
  await expect(section.getByText("Cole a chave da API do Claude")).toBeVisible();
  expect(captured.some((r) => r.url.endsWith("/api/ai/key"))).toBe(false);
});

test("Assistente (IA): chave da variável de ambiente não pode ser removida pela interface", async ({
  page,
}) => {
  await mockApi(page);
  await page.route("**/api/ai/status", (route) =>
    route.fulfill({
      json: { configured: true, source: "ENV", model: "claude-opus-5-5", receiptSupported: true },
    }),
  );
  await page.goto("/configuracoes");

  const section = page.getByTestId("ai-settings");
  await expect(section.getByTestId("ai-settings-status")).toContainText("Ativo");
  await expect(section.getByTestId("ai-settings-status")).toContainText(
    "chave da variável de ambiente",
  );
  await expect(section.getByTestId("ai-env-note")).toContainText("ANTHROPIC_API_KEY");
  await expect(section.getByTestId("ai-env-note")).toContainText("não pode ser removida por aqui");
  await expect(section.getByLabel("Chave da API do Claude")).toHaveCount(0);
  await expect(section.getByRole("button", { name: "Remover chave" })).toHaveCount(0);
});
