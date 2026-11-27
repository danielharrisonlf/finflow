import { expect, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

test("compromissos: lança uma conta fixa do mês", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/compromissos");

  await expect(page.getByTestId("commitments-chart").locator("svg")).toBeVisible();
  const items = page.getByTestId("commitment-items");
  await expect(items.getByText("parcela 3/10")).toBeVisible();

  await items.getByRole("button", { name: "Lançar Aluguel" }).click();
  const form = page.getByTestId("form-post-recurring");
  await form.getByRole("button", { name: "Lançar transação" }).click();
  await expect(form).not.toBeVisible();

  const post = captured.find((r) => r.url.includes(`/recurring/${ids.recurringRent}/post`));
  expect(post?.body).toMatchObject({
    amountCents: 250_000,
    accountId: ids.accountNubank,
    date: "2026-11-10",
  });

  await page.getByRole("button", { name: "A receber" }).click();
  await expect(items.getByText("Lançado")).toBeVisible();
});

test("ano: grade de 12 meses com totais", async ({ page }) => {
  await mockApi(page);
  await page.goto("/ano");
  const table = page.getByTestId("year-table");
  await expect(table.locator("tbody tr")).toHaveCount(12);
  await expect(table.locator("tfoot")).toContainText("118.200,00");
});

test("metas: reserva sugerida e desejos com prioridade e data em que cabe", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/metas");

  const reserve = page.getByTestId("reserve-panel");
  await expect(reserve).toContainText("25.200,00");
  await reserve.getByRole("button", { name: "Criar reserva de emergência" }).click();
  const goalForm = page.getByTestId("form-goal");
  await expect(goalForm.getByPlaceholder("10.000,00")).toHaveValue("25200,00");
  await goalForm.getByRole("button", { name: "Criar reserva" }).click();
  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/goals"))?.body)
    .toMatchObject({ kind: "RESERVE", targetCents: 2_520_000, name: "Reserva de emergência" });

  await page.getByRole("tab", { name: "Desejos" }).click();
  const list = page.getByTestId("wish-list");
  await expect(list).toContainText("Cabe no seu saldo em 12 de dezembro de 2026");
  await expect(list).toContainText("Ainda não cabe nos próximos 12 meses");

  await list
    .getByRole("button", { name: "Descer prioridade de Fone com cancelamento de ruído" })
    .click();
  await expect
    .poll(() =>
      captured
        .filter((r) => r.method === "PATCH" && r.url.includes("/wishlist/"))
        .map((r) => ({ url: r.url.split("/").pop(), body: r.body })),
    )
    .toEqual([
      { url: ids.wishB, body: { priority: 0 } },
      { url: ids.wishA, body: { priority: 1 } },
    ]);
});

test("categorias: tipo de gasto e automações", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/categorias");

  await expect(page.getByTestId("spend-type-badge").first()).toContainText("Variável");

  await page.getByRole("tab", { name: "Automações" }).click();
  await expect(page.getByTestId("rules-list")).toContainText("ifood");
  await page.getByRole("button", { name: "Aplicar às transações sem categoria" }).click();
  await expect(page.getByTestId("rules-apply-result")).toContainText("4 transações categorizadas");
  expect(captured.find((r) => r.url.includes("/rules/apply"))?.body).toEqual({
    onlyUncategorized: true,
  });
});
