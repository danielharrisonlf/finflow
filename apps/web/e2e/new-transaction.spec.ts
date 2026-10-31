import { expect, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

function isMobile(width: number | undefined): boolean {
  return (width ?? 1280) < 768;
}

test("lança uma despesa pelo sheet (FAB no mobile, botão na sidebar no desktop)", async ({
  page,
}) => {
  const captured = await mockApi(page);
  await page.goto("/");

  if (isMobile(page.viewportSize()?.width)) {
    await page.getByTestId("fab-new-transaction").click();
  } else {
    await page.getByRole("button", { name: "Nova transação" }).click();
  }

  const form = page.getByTestId("form-expense");
  await expect(form).toBeVisible();

  await form.getByPlaceholder("0,00").fill("45,90");
  await form.getByPlaceholder("Mercado, almoço, uber…").fill("Padaria");
  await form.locator("select").first().selectOption(ids.accountNubank);
  await form.locator("select").nth(1).selectOption(ids.catFood);
  await form.getByRole("button", { name: "Adicionar despesa" }).click();

  await expect(form).not.toBeVisible();
  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.includes("/transactions")))
    .toBeDefined();

  const request = captured.find((r) => r.method === "POST" && r.url.includes("/transactions"));
  expect(request?.body).toMatchObject({
    type: "EXPENSE",
    description: "Padaria",
    amountCents: 4590,
    accountId: ids.accountNubank,
    categoryId: ids.catFood,
    status: "CONFIRMED",
  });
});

test("transferência valida origem e destino diferentes", async ({ page }) => {
  await mockApi(page);
  await page.goto("/");

  if (isMobile(page.viewportSize()?.width)) {
    await page.getByTestId("fab-new-transaction").click();
  } else {
    await page.getByRole("button", { name: "Nova transação" }).click();
  }

  await page.getByRole("tab", { name: "Transferir" }).click();
  const form = page.getByTestId("form-transfer");
  await form.getByPlaceholder("0,00").fill("100");
  await form.locator("select").first().selectOption(ids.accountNubank);
  await form.locator("select").nth(1).selectOption(ids.accountNubank);
  await form.getByRole("button", { name: "Transferir" }).click();

  await expect(form.getByText("origem e destino devem ser diferentes")).toBeVisible();
});
