import { expect, test } from "@playwright/test";
import { fixtures, mockApi } from "./mocks";

test("API e MCP mostra versão, snippets MCP e QR da rede local", async ({ page }) => {
  await mockApi(page);
  await page.goto("/configuracoes");

  const section = page.getByTestId("api-ai-section");
  await expect(section.getByRole("heading", { name: "API e MCP" })).toBeVisible();
  await expect(section.getByTestId("system-info")).toContainText(fixtures.systemInfo.version);
  await expect(section.getByTestId("system-info")).toContainText("SQLite");
  await expect(section).toContainText('"mcp"');
  await expect(section).toContainText("/api/mcp");
  await expect(section).toContainText("Authorization: Bearer <seu token>");

  const lan = section.getByTestId("lan-status");
  await expect(lan).toContainText("http://192.168.0.10:3333");
  await expect(lan.getByTestId("qr-code")).toBeVisible();
  await expect(lan.getByTestId("qr-code").locator("path")).toHaveAttribute("d", /^M/);

  // O Assistente tem a seção própria; sem chave, nada aqui aponta para ele.
  await expect(section.getByTestId("ai-status")).toHaveCount(0);
  await expect(section.getByRole("link", { name: /Assistente/ })).toHaveCount(0);
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
