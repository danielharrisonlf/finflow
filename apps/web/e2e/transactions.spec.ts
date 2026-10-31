import { expect, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

test.beforeEach(async ({ page }) => {
  await mockApi(page);
});

test("extrato lista lançamentos com valores assinados e cores semânticas", async ({ page }) => {
  await page.goto("/extrato");
  const list = page.getByTestId("transactions-list");
  await expect(list.getByText("Almoço — rodízio japonês")).toBeVisible();
  await expect(list.getByText("−R$ 129,00")).toBeVisible();
  await expect(list.getByText("+R$ 9.850,00")).toBeVisible();
});

test("busca filtra a lista", async ({ page }) => {
  await page.goto("/extrato");
  await page.getByPlaceholder("Buscar descrição…").fill("almoço");
  const list = page.getByTestId("transactions-list");
  await expect(list.getByText("Almoço — rodízio japonês")).toBeVisible();
  await expect(list.getByText("Salário")).not.toBeVisible();
});

test("pendente pode ser confirmada com um clique", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/extrato");
  await page.getByRole("button", { name: "confirmar" }).click();
  await expect
    .poll(() =>
      captured.some(
        (r) => r.method === "PATCH" && r.url.includes(`/transactions/${ids.txPending}/confirm`),
      ),
    )
    .toBe(true);
});

test("exportação aponta para CSV e OFX com filtros aplicados", async ({ page }) => {
  await page.goto("/extrato");
  const csv = page.getByRole("link", { name: "CSV" });
  const ofx = page.getByRole("link", { name: "OFX" });
  await expect(csv).toHaveAttribute("href", /\/transactions\/export\?.*format=csv/);
  await expect(ofx).toHaveAttribute("href", /\/transactions\/export\?.*format=ofx/);
  await expect(csv).toHaveAttribute("href", /month=/);
});
