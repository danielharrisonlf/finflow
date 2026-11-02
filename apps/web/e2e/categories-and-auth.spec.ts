import { expect, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

test("cria subcategoria e edita categoria existente na tela de categorias", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/categorias");

  const grid = page.getByTestId("categories-grid");
  await expect(grid.getByText("Alimentação")).toBeVisible();

  await page.getByRole("button", { name: "Nova categoria" }).click();
  const form = page.getByTestId("form-category");
  await expect(form).toBeVisible();

  await form.getByPlaceholder("Ex.: Restaurantes, Pet, Dividendos…").fill("Restaurantes");
  await form.locator("select").nth(1).selectOption(ids.catFood);
  await form.getByRole("button", { name: "Ícone utensils" }).click();
  await form.getByRole("button", { name: "Criar categoria" }).click();

  await expect(form).not.toBeVisible();
  await expect
    .poll(() => captured.find((r) => r.method === "POST" && r.url.includes("/categories")))
    .toBeDefined();

  const postReq = captured.find((r) => r.method === "POST" && r.url.includes("/categories"));
  expect(postReq?.body).toMatchObject({
    name: "Restaurantes",
    kind: "EXPENSE",
    icon: "utensils",
    parentId: ids.catFood,
  });

  await page.getByRole("button", { name: "Editar Alimentação" }).click();
  await expect(form).toBeVisible();
  await form.getByPlaceholder("Ex.: Restaurantes, Pet, Dividendos…").fill("Alimentação & Feira");
  await form.getByRole("button", { name: "Salvar alterações" }).click();

  await expect
    .poll(() =>
      captured.find((r) => r.method === "PATCH" && r.url.includes(`/categories/${ids.catFood}`)),
    )
    .toBeDefined();
});

test("exibe tela de login quando autenticação é exigida e libera acesso após autenticar", async ({
  page,
}) => {
  await mockApi(page, { requireAuth: true });
  await page.goto("/");

  const loginPage = page.getByTestId("login-page");
  await expect(loginPage).toBeVisible();

  const form = page.getByTestId("form-login");
  await form.getByPlaceholder("Sua senha…").fill("errada");
  await form.getByRole("button", { name: "Entrar" }).click();
  await expect(form.getByText("senha incorreta")).toBeVisible();

  await form.getByPlaceholder("Sua senha…").fill("correta");
  await form.getByRole("button", { name: "Entrar" }).click();

  await expect(page.getByTestId("hero-balance")).toBeVisible();
});
