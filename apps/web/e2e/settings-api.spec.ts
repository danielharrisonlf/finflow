import { expect, test } from "@playwright/test";
import { fixtures, mockApi } from "./mocks";

test("API e IA mostra versão, snippets MCP, QR da rede local e status do assistente", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/configuracoes");

  const section = page.getByTestId("api-ai-section");
  await expect(section.getByRole("heading", { name: "API e IA" })).toBeVisible();
  await expect(section.getByTestId("system-info")).toContainText(fixtures.systemInfo.version);
  await expect(section.getByTestId("system-info")).toContainText("SQLite");
  await expect(section).toContainText('"mcp"');
  await expect(section).toContainText("/api/mcp");
  await expect(section).toContainText("Authorization: Bearer <seu token>");

  const lan = section.getByTestId("lan-status");
  await expect(lan).toContainText("http://192.168.0.10:3333");
  await expect(lan.getByTestId("qr-code")).toBeVisible();
  await expect(lan.getByTestId("qr-code").locator("path")).toHaveAttribute("d", /^M/);

  const ai = section.getByTestId("ai-status");
  await expect(ai).toContainText("não configurada");
  await expect(ai.getByRole("link", { name: "Abrir o Assistente" })).toHaveAttribute(
    "href",
    "/assistente",
  );
});

test("sem rede local explica como ativar e sem endpoint não quebra", async ({ page }) => {
  await mockApi(page);
  await page.route("**/api/system/info", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ ...fixtures.systemInfo, lanEnabled: false, lanUrls: [] }),
    }),
  );
  await page.goto("/configuracoes");
  const lan = page.getByTestId("lan-status");
  await expect(lan).toContainText("finflow serve --lan");
  await expect(lan.getByTestId("qr-code")).toHaveCount(0);

  await page.unroute("**/api/system/info");
  await page.route("**/api/system/info", (route) => route.fulfill({ status: 404, body: "" }));
  await page.route("**/api/ai/status", (route) => route.fulfill({ status: 404, body: "" }));
  await page.reload();
  const section = page.getByTestId("api-ai-section");
  await expect(section).toContainText("indisponíveis", { timeout: 10_000 });
  await expect(page.getByTestId("form-settings")).toBeVisible();
});

test("copiar o snippet MCP mostra confirmação", async ({ page, context, browserName }) => {
  test.skip(browserName !== "chromium", "permissão de clipboard só no Chromium");
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await mockApi(page);
  await page.goto("/configuracoes");
  await page.getByRole("button", { name: "Copiar configuração MCP stdio" }).click();
  await expect(page.getByRole("button", { name: "Copiar configuração MCP stdio" })).toContainText(
    "Copiado",
  );
});

test("Assistente detectado automaticamente aparece nas configurações", async ({ page }) => {
  await mockApi(page);
  await page.route("**/api/ai/status", (route) =>
    route.fulfill({
      json: {
        configured: true,
        provider: "ollama",
        model: "qwen2.5:7b",
        visionModel: null,
        baseUrl: "http://ollama:11434",
        source: "AUTO",
        reachable: true,
        receiptSupported: false,
      },
    }),
  );
  await page.goto("/configuracoes");
  await expect(page.getByTestId("ai-status")).toContainText(
    "Detectado automaticamente: Ollama · qwen2.5:7b",
  );
  const auto = page.getByTestId("ai-auto");
  await expect(auto).toContainText("http://ollama:11434");
  await expect(page.getByTestId("ai-settings").getByLabel("Local (Ollama)")).toBeChecked();
});
