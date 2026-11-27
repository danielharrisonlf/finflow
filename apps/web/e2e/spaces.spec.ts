import { expect, type Page, test } from "@playwright/test";
import { ids, mockApi } from "./mocks";

// v3 fase 2: espaços (seletor, cabeçalho, casa e empresa).

function isMobile(page: Page): boolean {
  return (page.viewportSize()?.width ?? 1280) < 768;
}

/** Abre o espaço antes do carregamento, como se o usuário já tivesse escolhido. */
async function useSpace(page: Page, slug: string, kind: string): Promise<void> {
  await page.addInitScript(
    ([s, k]) => {
      localStorage.setItem("finflow_space", s as string);
      localStorage.setItem("finflow_space_kind", k as string);
    },
    [slug, kind],
  );
}

/** Seletor: na sidebar (desktop) ou dentro do "Mais" (mobile). */
async function openSwitcher(page: Page): Promise<void> {
  if (isMobile(page)) {
    await page.getByTestId("mobile-more").click();
    await page
      .getByRole("dialog", { name: "Todas as seções" })
      .getByTestId("space-switcher")
      .click();
  } else {
    await page.getByTestId("space-switcher").click();
  }
  await expect(page.getByRole("dialog", { name: "Espaços" })).toBeVisible();
}

async function openNewTransaction(page: Page): Promise<void> {
  if (isMobile(page)) await page.getByTestId("fab-new-transaction").click();
  else await page.getByRole("button", { name: "Nova transação" }).click();
}

test("trocar de espaço envia o cabeçalho e recarrega os dados", async ({ page }) => {
  await mockApi(page);
  await page.goto("/");
  const hero = page.getByTestId("hero-balance");
  await expect(hero).toContainText("12.345,67");

  const reload = page.waitForRequest(
    (r) => r.url().includes("/api/dashboard/summary") && r.headers()["x-finflow-space"] === "casa",
  );
  await openSwitcher(page);
  await page.getByTestId("space-option-casa").click();
  await reload;

  await expect(hero).toContainText("4.567,89");
  await expect(hero).not.toContainText("12.345,67");
  await expect(page.getByTestId("space-switcher").first()).toHaveAttribute("data-space", "casa");
  expect(await page.evaluate(() => localStorage.getItem("finflow_space"))).toBe("casa");

  // downloads levam o espaço na query string
  await page.goto("/extrato");
  await expect(page.getByRole("link", { name: /CSV/ })).toHaveAttribute("href", /space=casa/);
});

test("slug desconhecido volta para o espaço pessoal", async ({ page }) => {
  await useSpace(page, "sumiu", "HOME");
  await mockApi(page);
  await page.goto("/");
  await expect(page.getByTestId("hero-balance")).toContainText("12.345,67");
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem("finflow_space")))
    .toBe("pessoal");
});

test("cria um espaço Empresa e passa a usá-lo", async ({ page }) => {
  const captured = await mockApi(page);
  await page.goto("/");
  await openSwitcher(page);
  await page.getByRole("button", { name: "Novo espaço" }).click();

  const form = page.getByTestId("form-space");
  await form.getByLabel("Nome").fill("Loja");
  await form.locator("label", { hasText: "Contas da empresa" }).click();
  await form.getByRole("button", { name: "Criar espaço" }).click();

  const find = () => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/spaces"));
  await expect.poll(find).toBeDefined();
  expect(find()?.body).toMatchObject({ name: "Loja", kind: "BUSINESS", color: "#38bdf8" });

  await expect(form).not.toBeVisible();
  await expect(page.getByTestId("space-switcher").first()).toHaveAttribute("data-space", "loja");
});

test("navegação mostra só as páginas do tipo do espaço", async ({ page }) => {
  await mockApi(page);
  const navLinks = async (): Promise<string[]> => {
    if (isMobile(page)) {
      await page.getByTestId("mobile-more").click();
      const names = await page
        .getByRole("dialog", { name: "Todas as seções" })
        .getByRole("link")
        .allInnerTexts();
      await page.keyboard.press("Escape");
      return names.map((n) => n.trim());
    }
    const sidebar = page.getByRole("navigation", { name: "Principal" }).first();
    return (await sidebar.getByRole("link").allInnerTexts()).map((n) => n.trim());
  };

  await page.goto("/");
  let links = await navLinks();
  expect(links).toContain("Metas e desejos");
  expect(links).not.toContain("Empresa");
  expect(links).not.toContain("Divisão de despesas");

  await openSwitcher(page);
  await page.getByTestId("space-option-casa").click();
  await expect(page.getByTestId("space-switcher").first()).toHaveAttribute("data-kind", "HOME");
  links = await navLinks();
  expect(links).toContain("Divisão de despesas");
  expect(links).not.toContain("Empresa");

  await openSwitcher(page);
  await page.getByTestId("space-option-empresa").click();
  await expect(page.getByTestId("space-switcher").first()).toHaveAttribute("data-kind", "BUSINESS");
  links = await navLinks();
  expect(links).toContain("Empresa");
  expect(links).toContain("Metas");
  expect(links).not.toContain("Metas e desejos");
  expect(links).not.toContain("Divisão de despesas");
});

test("casa: quem pagou e divisão personalizada na nova despesa", async ({ page }) => {
  await useSpace(page, "casa", "HOME");
  const captured = await mockApi(page);
  await page.goto("/");
  await openNewTransaction(page);

  const form = page.getByTestId("form-expense");
  await form.getByPlaceholder("0,00").fill("100,00");
  await form.getByPlaceholder("Mercado, almoço, uber…").fill("Mercado do mês");
  await form.locator("select").first().selectOption(ids.accountNubank);

  const split = form.getByTestId("member-split");
  await split.getByLabel("Quem pagou").selectOption(ids.memberAna);
  await expect(split).toContainText("R$ 50,00 cada");
  await split.getByRole("button", { name: "Personalizada" }).click();
  await split.getByLabel("Parte de Ana (%)").fill("60");
  await split.getByLabel("Parte de Bruno (%)").fill("30");
  await form.getByRole("button", { name: "Adicionar despesa" }).click();
  await expect(split).toContainText("precisa dar 100%");

  await split.getByLabel("Parte de Ana (%)").fill("70");
  await form.getByRole("button", { name: "Adicionar despesa" }).click();
  await expect(form).not.toBeVisible();

  const post = captured.find((r) => r.method === "POST" && r.url.endsWith("/api/transactions"));
  expect(post?.space).toBe("casa");
  expect(post?.body).toMatchObject({
    amountCents: 10_000,
    paidByMemberId: ids.memberAna,
    splits: [
      { memberId: ids.memberAna, sharePct: 70 },
      { memberId: ids.memberBruno, sharePct: 30 },
    ],
  });
});

test("pessoal: formulário de despesa não mostra a divisão", async ({ page }) => {
  await mockApi(page);
  await page.goto("/");
  await openNewTransaction(page);
  await expect(page.getByTestId("form-expense")).toBeVisible();
  await expect(page.getByTestId("member-split")).toHaveCount(0);
});

test("casa: saldos, acerto sugerido e cadastro de pessoa", async ({ page }) => {
  await useSpace(page, "casa", "HOME");
  const captured = await mockApi(page);
  await page.goto("/divisao");

  await expect(page.getByTestId("members-list")).toContainText("Ana");
  await expect(page.getByTestId("members-list")).toContainText("Bruno");
  const balances = page.getByTestId("balances-list");
  await expect(balances).toContainText("+R$ 1.500,00");
  await expect(balances).toContainText("tem a receber");
  await expect(balances).toContainText("−R$ 1.500,00");
  await expect(balances).toContainText("deve");

  await page.getByRole("button", { name: "Registrar acerto de Bruno para Ana" }).click();
  const settle = page.getByTestId("form-settle");
  await expect(settle.getByLabel("Valor")).toHaveValue("1500,00");
  await settle.getByRole("button", { name: "Registrar acerto" }).click();
  const findSettle = () =>
    captured.find((r) => r.method === "POST" && r.url.endsWith("/api/members/settle"));
  await expect.poll(findSettle).toBeDefined();
  expect(findSettle()?.body).toMatchObject({
    fromMemberId: ids.memberBruno,
    toMemberId: ids.memberAna,
    amountCents: 150_000,
  });
  expect(findSettle()?.body).not.toHaveProperty("fromAccountId");

  await page.getByTestId("members-card").getByRole("button", { name: "Adicionar" }).click();
  const member = page.getByTestId("form-member");
  await member.getByLabel("Nome").fill("Carla");
  await member.getByRole("button", { name: "Adicionar pessoa" }).click();
  const findMember = () =>
    captured.find((r) => r.method === "POST" && r.url.endsWith("/api/members"));
  await expect.poll(findMember).toBeDefined();
  expect(findMember()?.body).toMatchObject({ name: "Carla" });
});

test("empresa: métricas, DRE e grupo do DRE nas categorias; privacidade mascara", async ({
  page,
}) => {
  await useSpace(page, "empresa", "BUSINESS");
  const captured = await mockApi(page);
  await page.goto("/empresa");

  await expect(page.getByTestId("metric-mrr")).toContainText("12.500,00");
  await expect(page.getByTestId("metric-arr")).toContainText("150.000,00");
  await expect(page.getByTestId("metric-churn")).toContainText("4,2%");
  await expect(page.getByTestId("metric-burn")).toContainText("3.200,00");
  await expect(page.getByTestId("metric-runway")).toContainText("9,5 meses");
  await expect(page.getByTestId("business-chart")).toBeVisible();
  const dre = page.getByTestId("dre-table");
  await expect(dre).toContainText("Receita bruta");
  await expect(dre).toContainText("Resultado líquido");
  await expect(dre).toContainText("10.000,00");

  await page.locator('[data-testid="privacy-toggle"]:visible').click();
  await expect(page.getByTestId("metric-mrr")).toContainText("R$ •••••");
  await expect(page.getByTestId("metric-mrr")).not.toContainText("12.500,00");
  await expect(dre).not.toContainText("10.000,00");
  await page.locator('[data-testid="privacy-toggle"]:visible').click();

  await page.goto("/categorias");
  await page
    .getByRole("button", { name: /Nova categoria/ })
    .first()
    .click();
  const form = page.getByTestId("form-category");
  await form.getByRole("textbox", { name: "Nome" }).fill("Hospedagem");
  await form.getByTestId("category-dre-group").selectOption("COSTS");
  await form.getByRole("button", { name: "Criar categoria" }).click();
  const find = () => captured.find((r) => r.method === "POST" && r.url.endsWith("/api/categories"));
  await expect.poll(find).toBeDefined();
  expect(find()?.body).toMatchObject({ name: "Hospedagem", dreGroup: "COSTS" });
});

test("visão unificada mostra os espaços e os totais", async ({ page }) => {
  await mockApi(page);
  await page.goto("/espacos");
  await expect(page.getByTestId("spaces-totals")).toContainText("47.313,56");
  await expect(page.getByTestId("space-card-casa")).toContainText("Casa");
  await expect(page.getByTestId("space-card-empresa")).toContainText("30.400,00");
  await page.getByRole("button", { name: "Abrir o espaço Casa" }).click();
  await expect(page.getByTestId("space-switcher").first()).toHaveAttribute("data-space", "casa");
});

test("novas páginas não rolam na horizontal em 320px", async ({ page }) => {
  await mockApi(page);
  await page.setViewportSize({ width: 320, height: 720 });
  const cases = [
    { slug: "pessoal", kind: "PERSONAL", route: "/espacos" },
    { slug: "casa", kind: "HOME", route: "/divisao" },
    { slug: "empresa", kind: "BUSINESS", route: "/empresa" },
  ];
  for (const c of cases) {
    await page.goto("/");
    await page.evaluate(
      ([s, k]) => {
        localStorage.setItem("finflow_space", s as string);
        localStorage.setItem("finflow_space_kind", k as string);
      },
      [c.slug, c.kind],
    );
    await page.goto(c.route);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    // espera o conteúdo real (não só o cabeçalho) antes de medir
    await page.waitForLoadState("networkidle");
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow, `scroll horizontal em ${c.route} (${c.slug})`).toBeLessThanOrEqual(0);
  }
});
