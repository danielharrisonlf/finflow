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

test("Assistente: servidor compatível com teste de conexão, modelo e chave", async ({ page }) => {
  const captured = await mockApi(page);
  const modelQueries: string[] = [];
  await page.route("**/api/ai/models**", (route) => {
    const url = new URL(route.request().url());
    modelQueries.push(url.search);
    if (url.searchParams.get("provider") === "openai_compatible") {
      return route.fulfill({
        json: {
          models: [{ id: "qwen2.5-7b-instruct", label: "qwen2.5-7b-instruct", tools: null }],
        },
      });
    }
    return route.fulfill({
      status: 502,
      json: { message: "Não foi possível conectar ao Ollama em http://localhost:11434." },
    });
  });
  await page.goto("/configuracoes");

  const section = page.getByTestId("ai-settings");
  await expect(section.getByRole("heading", { name: "Assistente" })).toBeVisible();
  // Padrão: Ollama local, com instruções de instalação e sem campo de chave.
  await expect(section.getByLabel("Local (Ollama)")).toBeChecked();
  await expect(section.getByTestId("ai-help")).toContainText("ollama pull qwen2.5:7b");
  await expect(section.getByLabel("Chave da API (opcional)")).toHaveCount(0);
  await section.getByRole("button", { name: "Testar conexão" }).click();
  await expect(section.getByTestId("ai-test-result")).toContainText(
    "Não foi possível conectar ao Ollama",
  );

  await section.getByLabel("Servidor compatível (LM Studio, llama.cpp…)").check();
  await expect(section.getByTestId("ai-help")).toContainText("Start server");
  await section.getByLabel("URL do servidor").fill("http://localhost:1234/v1");
  await section.getByRole("button", { name: "Testar conexão" }).click();
  await expect(section.getByTestId("ai-test-result")).toHaveText("Conectado: 1 modelo encontrado.");
  await section.getByRole("combobox").first().selectOption("qwen2.5-7b-instruct");
  await section.getByLabel("Chave da API (opcional)").fill("lm-studio");
  await section.getByRole("button", { name: "Salvar assistente" }).click();
  await expect(section.getByText("Assistente salvo")).toBeVisible();

  const put = captured.find((r) => r.method === "PUT" && r.url.endsWith("/api/ai/config"));
  expect(put?.body).toEqual({
    provider: "openai_compatible",
    baseUrl: "http://localhost:1234/v1",
    model: "qwen2.5-7b-instruct",
    visionModel: null,
    apiKey: "lm-studio",
  });
  expect(modelQueries.some((q) => q.includes("baseUrl=http%3A%2F%2Flocalhost%3A1234%2Fv1"))).toBe(
    true,
  );

  // Claude: só a chave, sem URL nem modelo.
  await section.getByLabel("Claude (API)").check();
  await expect(section.getByLabel("URL do servidor")).toHaveCount(0);
  await expect(section.getByLabel("Chave da API do Claude")).toBeVisible();
  await expect(section.getByTestId("ai-help")).toContainText("console.anthropic.com");
});
