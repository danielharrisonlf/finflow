import { expect, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

test("extrato mostra tags, filtra por tag e edita o conjunto", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/extrato");

  const lunch = page
    .getByTestId("transactions-list")
    .getByRole("listitem")
    .filter({ hasText: "Almoço" });
  await expect(lunch.getByTestId("transaction-tags")).toContainText("Viagem");

  const tagFilter = page.getByLabel("Filtrar por tag");
  const filtered = page.waitForRequest((r) => r.url().includes(`tagId=${ids.tagTrip}`));
  await tagFilter.selectOption({ label: "Viagem" });
  await filtered;

  await lunch.getByRole("button", { name: /Editar Almoço/ }).click();
  const form = page.getByTestId("form-edit-transaction");
  const picker = form.getByTestId("tag-picker");
  await expect(picker.getByRole("button", { name: "Viagem" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await picker.getByRole("button", { name: "Trabalho" }).click();
  await form.getByRole("button", { name: "Salvar alterações" }).click();

  await expect
    .poll(() => captured.find((r) => r.method === "PATCH" && r.url.includes("/transactions/")))
    .toBeDefined();
  const patch = captured.find((r) => r.method === "PATCH" && r.url.includes("/transactions/"));
  expect(patch?.body).toMatchObject({ tagIds: [ids.tagTrip, ids.tagWork] });
});

test("nova despesa leva as tags escolhidas", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/");
  const width = page.viewportSize()?.width ?? 1280;
  if (width < 768) await page.getByTestId("fab-new-transaction").click();
  else await page.getByRole("button", { name: "Nova transação" }).first().click();

  const form = page.getByTestId("form-expense");
  await form.getByPlaceholder("0,00").fill("45,90");
  await form.getByLabel("Descrição").fill("Táxi do aeroporto");
  await form.getByLabel("Conta").selectOption({ label: "Nubank" });
  await form.getByTestId("tag-picker").getByRole("button", { name: "Viagem" }).click();
  await form.getByRole("button", { name: "Adicionar despesa" }).click();

  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/transactions")))
    .toBeDefined();
  const post = captured.find((r) => r.method === "POST" && r.url.endsWith("/api/transactions"));
  expect(post?.body).toMatchObject({ amountCents: 4_590, tagIds: [ids.tagTrip] });
});

test("aba Tags em Categorias: lista, gasto por tag, criação e regra com tag", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/categorias");
  await page.getByRole("tab", { name: "Tags" }).click();

  await expect(page.getByTestId("tags-list")).toContainText("Viagem");
  await expect(page.getByTestId("tags-list")).toContainText("4 lançamentos");
  await expect(page.getByTestId("tag-spending")).toContainText("R$ 1.523,00");

  await page.getByRole("button", { name: "Nova tag" }).click();
  const tagForm = page.getByTestId("form-tag");
  await tagForm.getByLabel("Nome").fill("Casa");
  await tagForm.getByRole("button", { name: "Criar tag" }).click();
  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/tags")))
    .toBeDefined();
  expect(
    captured.find((r) => r.method === "POST" && r.url.endsWith("/api/tags"))?.body,
  ).toMatchObject({ name: "Casa", color: "#38bdf8" });

  await page.getByRole("tab", { name: "Automações" }).click();
  await page.getByRole("button", { name: "Nova regra" }).click();
  const ruleForm = page.getByTestId("form-rule");
  await ruleForm.getByLabel("Quando a descrição contém").fill("hotel");
  await ruleForm.getByLabel("Usar a categoria").selectOption({ label: "Moradia" });
  await ruleForm.getByLabel("Aplicar também a tag (opcional)").selectOption({ label: "Viagem" });
  await ruleForm.getByRole("button", { name: "Criar regra" }).click();
  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/rules")))
    .toBeDefined();
  expect(
    captured.find((r) => r.method === "POST" && r.url.endsWith("/api/rules"))?.body,
  ).toMatchObject({ pattern: "hotel", categoryId: ids.catHome, tagId: ids.tagTrip });
});
