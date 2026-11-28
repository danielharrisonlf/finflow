import { mkdir, readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { join } from "node:path";
import { expect, type Page, test } from "@playwright/test";
import { mockApi } from "./mocks";

const screenshotsDir = join(process.cwd(), "test-results", "screenshots");
const themeButtonName = /Ativar tema (claro|escuro)/;
// Inside the installed app, /inicio points "Experimentar no navegador" at the hosted browser app;
// the public package points it at its own /app/ (verify-public-landing.mjs).
const hostedApp = "https://finflow-landing-umber.vercel.app/app/";
const repoUrl = "https://github.com/danielharrisonlf/finflow";

interface AxeNodeResult {
  target: string[];
  failureSummary: string;
}

interface AxeViolationResult {
  id: string;
  impact: string | null;
  nodes: AxeNodeResult[];
}

interface AxeRunner {
  run(
    document: Document,
    options: { runOnly: { type: "tag"; values: string[] } },
  ): Promise<{ violations: AxeViolationResult[] }>;
}

async function clearThemeForFirstDocument(page: Page) {
  await page.addInitScript(() => {
    try {
      if (sessionStorage.getItem("__finflow-theme-test-cleaned") !== "1") {
        localStorage.removeItem("finflow-theme");
        sessionStorage.setItem("__finflow-theme-test-cleaned", "1");
      }
    } catch {
      // Storage denial is covered separately; let the application exercise its fallback.
    }
  });
}

async function setTheme(page: Page, theme: "light" | "dark") {
  const root = page.locator("html");
  if ((await root.getAttribute("data-theme")) !== theme) {
    await page.getByRole("button", { name: themeButtonName }).first().click();
  }
  await expect(root).toHaveAttribute("data-theme", theme);
  await expect(root).toHaveClass(new RegExp(`\\b${theme}\\b`));
}

async function scrollFullPage(page: Page) {
  await page.evaluate(async () => {
    document.documentElement.style.scrollBehavior = "auto";
    const step = Math.max(400, Math.floor(window.innerHeight * 0.6));
    for (let y = 0; y < document.documentElement.scrollHeight; y += step) {
      window.scrollTo(0, y);
      await new Promise((resolve) => window.setTimeout(resolve, 60));
    }
    window.scrollTo(0, document.documentElement.scrollHeight);
    await new Promise((resolve) => window.setTimeout(resolve, 250));
  });
}

async function saveScreenshot(page: Page, name: string) {
  await mkdir(screenshotsDir, { recursive: true });
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: join(screenshotsDir, `${name}.png`),
    fullPage: true,
    animations: "disabled",
  });
}

async function audit(page: Page, state: string) {
  // This workspace tests the standalone canonical landing and its exact monorepo copy.
  // Reuse the canonical repo's axe-core install without adding app dependencies/config.
  const axeLoaded = await page.evaluate(() =>
    Boolean((window as unknown as { axe?: AxeRunner }).axe),
  );
  if (!axeLoaded) {
    const axePath = createRequire(import.meta.url).resolve("axe-core/axe.min.js");
    const axeScript = await readFile(axePath, "utf8");
    await page.addScriptTag({ content: axeScript });
  }
  await page.waitForFunction(
    () =>
      document
        .getAnimations()
        .every(
          (animation) =>
            animation.playState !== "running" ||
            animation.effect?.getTiming().iterations === Infinity,
        ),
    null,
    { timeout: 5000 },
  );
  const stableColors = await page.evaluate(() => {
    const brand = document.querySelector<HTMLElement>("nav .ff-brand");
    const cardText = document.querySelector<HTMLElement>("#cartoes .ff-feature-copy p");
    const card = cardText?.closest<HTMLElement>(".ff-feature-card");
    return {
      brand: brand ? getComputedStyle(brand).color : null,
      cardText: cardText ? getComputedStyle(cardText).color : null,
      cardBackground: card ? getComputedStyle(card).backgroundColor : null,
    };
  });
  console.log(`stable colors ${state}: ${JSON.stringify(stableColors)}`);
  const violations = await page.evaluate(async () => {
    const axe = (window as unknown as { axe: AxeRunner }).axe;
    const results = await axe.run(document, {
      runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"] },
    });
    return results.violations.map((violation) => ({
      id: violation.id,
      impact: violation.impact,
      count: violation.nodes.length,
      examples: violation.nodes.slice(0, 3).map((node) => ({
        target: node.target,
        summary: node.failureSummary,
      })),
    }));
  });
  console.log(`axe ${state}: ${JSON.stringify(violations)}`);
  expect(violations, `axe WCAG AA violations in ${state}`).toEqual([]);
}

test("/inicio is public, isolated from APIs, and links into the existing auth gate", async ({
  page,
}) => {
  const apiRequests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname.startsWith("/api/")) apiRequests.push(request.url());
  });
  await mockApi(page, { requireAuth: true });
  await page.goto("/inicio");

  await expect(page.getByRole("heading", { level: 1 })).toHaveText(
    "Seu dinheiro em ordem. Mais espaço para viver.",
  );
  await expect(page.getByText("Código aberto (MIT) · roda no seu computador")).toBeVisible();
  await expect(
    page.getByText(/disponível em breve|em preparação|ainda não foi lançado/i),
  ).toHaveCount(0);
  await expect(page.getByText(/Breno|Visor/i)).toHaveCount(0);
  // The AI integration may name Claude/Anthropic; no other AI vendor is named.
  await expect(page.getByText(/ChatGPT|OpenAI|Gemini|Copilot/i)).toHaveCount(0);
  await expect(page.locator(".ff-hero-copy > p")).toHaveText(
    "Contas, cartões, metas e planilhas num programa que roda no seu computador. Sem conta e sem nuvem: seus dados ficam num arquivo SQLite seu.",
  );
  await expect(page.locator(".ff-hero-proof")).toContainText("Assistente local (Ollama) ou Claude");
  const heroActions = page.locator(".ff-hero-actions");
  // Primary CTA: the download (docs/local-agent-contract.md §4). Secondary: the source code.
  // Third, as a text link: try it in the browser.
  await expect(heroActions.getByRole("link")).toHaveCount(3);
  await expect(heroActions.locator(".ff-button")).toHaveText("Baixar grátis");
  await expect(heroActions.getByRole("link", { name: "Baixar grátis" })).toHaveAttribute(
    "href",
    "#baixar",
  );
  await expect(heroActions.getByRole("link", { name: "Código no GitHub" })).toHaveAttribute(
    "href",
    repoUrl,
  );
  await expect(
    heroActions.getByRole("link", { name: "Experimentar no navegador" }),
  ).toHaveAttribute("href", hostedApp);
  // There is no sign-up flow, so nothing may promise a demo account or registration.
  await expect(
    page.getByRole("link", { name: /demo|demonstração|criar conta|cadastro|cadastre-se/i }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", {
      name: /demo|demonstração|criar conta|cadastro|cadastre-se/i,
    }),
  ).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Pergunte ao FinFlow." })).toBeVisible();
  expect(apiRequests, "/inicio should not contact the authenticated app API").toEqual([]);

  if (await page.getByRole("button", { name: "Abrir menu" }).isVisible()) {
    await page.getByRole("button", { name: "Abrir menu" }).click();
  }
  await expect(page.getByRole("link", { name: "Acessar minha conta" })).toHaveAttribute(
    "href",
    "/",
  );
  await page.getByRole("link", { name: "Acessar minha conta" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByTestId("login-page")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Entrar no FinFlow" })).toBeVisible();
  await expect(page.getByLabel("Senha")).toBeVisible();
  await page.getByLabel("Senha").fill("errada");
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page.getByTestId("login-page")).toBeVisible();
  await expect(page.getByText("senha incorreta")).toBeVisible();
});

async function emulateDesktop(page: Page, userAgent: string) {
  await page.addInitScript((ua) => {
    Object.defineProperty(navigator, "userAgent", { configurable: true, get: () => ua });
    Object.defineProperty(navigator, "userAgentData", { configurable: true, get: () => undefined });
  }, userAgent);
}

const latestDownload = "https://github.com/danielharrisonlf/finflow/releases/latest/download/";

test("download section offers the right release asset, other systems, and install commands", async ({
  page,
}) => {
  await emulateDesktop(
    page,
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36",
  );
  await page.goto("/inicio");
  const download = page.locator("#baixar");
  await expect(download.getByRole("heading", { name: "Rode no seu computador." })).toBeVisible();
  const primary = download.locator(".ff-download-primary");
  await expect(primary).toHaveText("Baixar para Windows");
  await expect(primary).toHaveAttribute("href", `${latestDownload}finflow-windows-amd64.zip`);
  const others = download.locator(".ff-download-others a");
  await expect(others).toHaveCount(5);
  const hrefs = await others.evaluateAll((links) => links.map((a) => a.getAttribute("href")));
  expect(hrefs.sort()).toEqual(
    [
      "finflow-darwin-amd64.tar.gz",
      "finflow-darwin-arm64.tar.gz",
      "finflow-linux-amd64.tar.gz",
      "finflow-linux-arm64.tar.gz",
      "finflow-windows-arm64.zip",
    ].map((name) => `${latestDownload}${name}`),
  );
  await expect(download.locator(".ff-download-steps li")).toHaveCount(3);
  await expect(download.locator(".ff-download-steps")).toContainText("localhost:3333");
  await expect(download.getByText(/arquivo SQLite no seu computador/)).toBeVisible();
  await expect(download.getByText(/Nada vai para a nuvem/)).toBeVisible();
  await expect(download.locator(".ff-install-command code")).toHaveText([
    "curl -fsSL https://raw.githubusercontent.com/danielharrisonlf/finflow/main/scripts/install.sh | sh",
    "irm https://raw.githubusercontent.com/danielharrisonlf/finflow/main/scripts/install.ps1 | iex",
  ]);
  const copy = download.getByRole("button", { name: "Copiar comando para macOS e Linux" });
  await copy.click();
  await expect(copy).toContainText(/Copiado|Selecionado/);

  // Three ways to run it locally: executable (default), source code with pnpm start, Docker.
  const tabs = download.getByRole("tablist", { name: "Como rodar o FinFlow" }).getByRole("tab");
  await expect(tabs).toHaveText(["Executável", "Código-fonte", "Docker"]);
  await expect(tabs.first()).toHaveAttribute("aria-selected", "true");
  await tabs.nth(1).click();
  await expect(tabs.nth(1)).toHaveAttribute("aria-selected", "true");
  await expect(download.locator(".ff-download-primary")).toHaveCount(0);
  await expect(download.locator(".ff-install-command code")).toHaveText([
    "git clone https://github.com/danielharrisonlf/finflow && cd finflow && pnpm install && pnpm start",
    "pnpm start -- --seed",
  ]);
  await expect(download.getByRole("list", { name: "Requisitos" })).toContainText(
    /Go 1\.26[\s\S]*Node 22\.12[\s\S]*Corepack/,
  );
  await expect(download.getByRole("link", { name: "Código no GitHub" })).toHaveAttribute(
    "href",
    repoUrl,
  );
  // Arrow keys move between tabs (WAI-ARIA tabs pattern).
  await tabs.nth(1).press("ArrowRight");
  await expect(tabs.nth(2)).toBeFocused();
  await expect(tabs.nth(2)).toHaveAttribute("aria-selected", "true");
  await expect(download.locator(".ff-install-command code")).toHaveText([
    "git clone https://github.com/danielharrisonlf/finflow && cd finflow && docker compose up -d",
    "docker compose --profile ai up -d",
  ]);
  await expect(download.getByRole("tabpanel")).toContainText("Ollama");
  await tabs.first().click();
  await expect(download.locator(".ff-download-primary")).toHaveText("Baixar para Windows");

  // Every "Baixar grátis" CTA leads to this section; every "Experimentar no navegador" CTA
  // (hero, final call, footer, download intro) opens the browser app; "Código no GitHub" opens
  // the repository.
  const downloadCtas = await page.getByRole("link", { name: "Baixar grátis" }).all();
  expect(downloadCtas.length).toBeGreaterThanOrEqual(3);
  for (const cta of downloadCtas) await expect(cta).toHaveAttribute("href", "#baixar");
  const appCtas = await page.getByRole("link", { name: /^Experimentar no navegador/ }).all();
  expect(appCtas.length).toBeGreaterThanOrEqual(4);
  for (const cta of appCtas) await expect(cta).toHaveAttribute("href", hostedApp);
  const repoCtas = await page.getByRole("link", { name: "Código no GitHub" }).all();
  expect(repoCtas.length).toBeGreaterThanOrEqual(2);
  for (const cta of repoCtas) await expect(cta).toHaveAttribute("href", repoUrl);

  for (const [ua, label, asset] of [
    [
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
      "Baixar para macOS (Apple Silicon)",
      "finflow-darwin-arm64.tar.gz",
    ],
    [
      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36",
      "Baixar para Linux",
      "finflow-linux-amd64.tar.gz",
    ],
  ] as const) {
    const other = await page.context().newPage();
    await emulateDesktop(other, ua);
    await other.goto("/inicio");
    const button = other.locator("#baixar .ff-download-primary");
    await expect(button).toHaveText(label);
    await expect(button).toHaveAttribute("href", `${latestDownload}${asset}`);
    await other.close();
  }
});

test("spreadsheet and projection previews expose readable data", async ({ page }) => {
  await page.goto("/inicio");
  const sheets = page.locator("#planilhas");
  await expect(sheets.getByRole("heading", { level: 2 })).toContainText("Sua planilha entra.");
  await expect(sheets).toContainText("Entrada · Saída · Diário · Saldo");
  await expect(sheets).toContainText("OFX");
  await expect(sheets).toContainText(".xlsx");
  await expect(sheets).toContainText("Economia");
  const table = sheets.getByRole("table");
  await expect(table.getByRole("columnheader")).toHaveText([
    "Data",
    "Entrada",
    "Saída",
    "Diário",
    "Saldo",
  ]);
  await expect(table.locator("tbody tr")).toHaveCount(7);

  const projection = page.locator("#projecao");
  await expect(projection.getByRole("img")).toHaveAttribute(
    "aria-label",
    /Saldo projetado dia a dia nos próximos 30 dias/,
  );
  await expect(projection.locator(".ff-projection-chart rect")).toHaveCount(30);
  await expect(
    projection.getByText("Menor saldo nos próximos 30 dias", { exact: true }),
  ).toBeVisible();
  await expect(
    projection.getByRole("list", { name: "Faixas de cor do saldo" }).locator("li"),
  ).toHaveCount(5);
  // v3: "Ritmo do mês" moved into the spend-today card; subscriptions, alerts, tags and cash flow
  // joined the list.
  await expect(page.locator(".ff-feature-list li")).toHaveCount(9);
  for (const title of [
    "Compromissos",
    "Assinaturas detectadas",
    "Alertas e dicas",
    "Tags",
    "Fluxo de caixa",
    "Onde dá para cortar",
    "Reserva de emergência",
    "Desejos",
    "Automações",
  ]) {
    await expect(
      page.locator(".ff-feature-list").getByRole("heading", { name: title }),
    ).toBeVisible();
  }
  const budgets = page.locator(".ff-budgets-feature");
  await expect(budgets.getByRole("heading", { level: 3 })).toHaveText(
    "Quanto dá para gastar hoje.",
  );
  await expect(budgets.locator(".ff-spend-today")).toContainText("Você pode gastar hoje");
  await expect(budgets.locator(".ff-spend-today strong")).toHaveText(/R\$\s+86,00/);
  await expect(budgets.locator(".ff-spend-today")).toContainText("Ritmo");
});

test("assistant example answers locally and always asks before writing", async ({ page }) => {
  const apiRequests: string[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.pathname.startsWith("/api/") || url.hostname.endsWith("anthropic.com")) {
      apiRequests.push(request.url());
    }
  });
  await page.goto("/inicio");
  const ai = page.locator("#assistente");
  await expect(ai.getByRole("heading", { level: 2 })).toHaveText("Pergunte ao FinFlow.");
  await expect(ai.locator(".ff-assistant-badge")).toHaveText("Exemplo");
  await expect(ai).toContainText("modelo local (Ollama ou LM Studio)");
  await expect(ai).toContainText("sua própria chave da API do Claude");
  await expect(ai).toContainText("depois que você confirma");
  await expect(ai).toContainText("foto do comprovante");

  const questions = ai.getByRole("group", { name: "Perguntas de exemplo" }).getByRole("button");
  await expect(questions).toHaveText([
    "Posso jantar fora esse mês?",
    "Posso fazer uma viagem?",
    "Posso comprar um celular em 10x?",
  ]);
  const thread = ai.locator(".ff-assistant-thread");
  await expect(questions.first()).toHaveAttribute("aria-pressed", "true");
  await expect(thread.locator(".ff-chat-outgoing")).toHaveText("Posso jantar fora esse mês?");
  await expect(thread.locator(".ff-assistant-verdict")).toHaveText("Cabe no mês");

  await questions.nth(1).click();
  await expect(questions.nth(1)).toHaveAttribute("aria-pressed", "true");
  await expect(questions.first()).toHaveAttribute("aria-pressed", "false");
  await expect(thread.locator(".ff-assistant-verdict")).toHaveText("Com cuidado");
  await expect(thread).toContainText("Consultou sua projeção de saldo");
  await expect(thread.getByRole("button", { name: "Confirmar" })).toHaveCount(0);

  // The installment question proposes a write; nothing happens until the visitor confirms.
  await questions.nth(2).click();
  await expect(thread.locator(".ff-assistant-verdict")).toHaveText("Melhor esperar");
  const proposal = thread.locator(".ff-assistant-proposal");
  await expect(proposal).toContainText("Precisa da sua confirmação");
  await expect(proposal).toContainText("Adicionar aos desejos");
  await proposal.getByRole("button", { name: "Agora não" }).click();
  await expect(proposal.getByRole("status")).toHaveText("Tudo bem. Nada foi alterado.");
  await questions.nth(1).click();
  await questions.nth(2).click();
  await proposal.getByRole("button", { name: "Confirmar" }).click();
  await expect(proposal.getByRole("status")).toContainText("Exemplo: nada foi salvo.");

  const receipt = ai.getByRole("button", { name: "Enviar foto do comprovante" });
  await receipt.click();
  await expect(receipt).toHaveAttribute("aria-pressed", "true");
  await expect(thread.locator(".ff-assistant-attachment")).toContainText("comprovante.jpg");
  await expect(thread).toContainText("Leu a foto do comprovante");
  await expect(proposal).toContainText("Nova despesa");
  await expect(proposal.getByRole("button", { name: "Confirmar" })).toBeVisible();

  await expect(ai.getByRole("heading", { name: "Use com o Claude ou outra IA" })).toBeVisible();
  await expect(ai).toContainText("/api/mcp");
  await expect(ai.getByRole("heading", { name: "Sua API pessoal" })).toBeVisible();
  expect(apiRequests, "the assistant example never contacts an API").toEqual([]);
});

test("planning, mobile and FAQ sections describe v3 features", async ({ page }) => {
  await page.goto("/inicio");
  const wealth = page.locator("#patrimonio");
  await expect(wealth.getByRole("heading", { level: 2 })).toContainText("patrimônio de amanhã");
  for (const title of ["Investimentos", "Dívidas e financiamentos", "Evolução do patrimônio"]) {
    await expect(wealth.locator(".ff-sheets-list")).toContainText(title);
  }
  await expect(wealth.locator(".ff-wealth-chart")).toHaveAttribute(
    "aria-label",
    /Evolução do patrimônio líquido em 12 meses/,
  );
  await expect(wealth.locator(".ff-wealth-chart rect")).toHaveCount(12);
  const strategy = wealth.getByRole("group", { name: "Estratégia de quitação" });
  const plan = wealth.locator(".ff-wealth-plan");
  await expect(plan).toContainText("Maior juro primeiro");
  await expect(plan).toContainText("31 meses");
  await strategy.getByRole("button", { name: "Bola de neve" }).click();
  await expect(strategy.getByRole("button", { name: "Bola de neve" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(plan).toContainText("Menor saldo primeiro");
  await expect(plan).toContainText("33 meses");
  await expect(
    plan.getByRole("list", { name: "Ordem de quitação" }).locator("li").first(),
  ).toHaveText("Empréstimo pessoal");
  const calculators = wealth.getByRole("list", { name: "Calculadoras incluídas" }).locator("li");
  await expect(calculators).toHaveCount(11);
  await expect(wealth.locator(".ff-calculators h3")).toHaveText(
    "11 calculadoras para decidir com calma",
  );

  const lan = page.locator("#mobile .ff-mobile-lan");
  await expect(lan).toContainText("pela rede de casa");
  await expect(lan).toContainText("finflow serve --lan");
  await expect(lan).toContainText("instale como app");
  await expect(lan).toContainText("sem nuvem");

  const faq = page.locator("#duvidas");
  for (const [question, answer] of [
    [
      "Preciso pagar pela IA?",
      /^Não\. [\s\S]*modelo local, pelo Ollama ou pelo LM Studio, sem chave e sem custo[\s\S]*sua própria chave da API do Claude, cobrada pela Anthropic/,
    ],
    ["Funciona no celular?", /navegador do celular[\s\S]*instale como app[\s\S]*rede de casa/],
    [
      "Meus dados vão para a nuvem?",
      /assistente local, nem as perguntas saem dele[\s\S]*escolher o Claude[\s\S]*API do Claude/,
    ],
    [
      "Onde ficam meus dados na versão do navegador?",
      /No próprio navegador[\s\S]*Dados neste navegador[\s\S]*apaga o FinFlow/,
    ],
    [
      "Posso levar meus dados para o app instalado?",
      /backup \.db feito no navegador abre no app instalado[\s\S]*finflow\.db/,
    ],
  ] as const) {
    const item = faq.locator("details", { has: page.getByText(question, { exact: true }) });
    await item.locator("summary").click();
    await expect(item).toHaveAttribute("open", "");
    await expect(item.locator("p")).toHaveText(answer);
  }
});

test("v3 sections render cleanly at 390 and 1440 in both themes", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop", "screenshots are captured once, on desktop");
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await clearThemeForFirstDocument(page);
  await page.goto("/inicio");
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  await mkdir(screenshotsDir, { recursive: true });
  await page
    .locator("#assistente")
    .getByRole("button", { name: "Posso comprar um celular em 10x?" })
    .click();
  for (const question of [
    "Preciso pagar pela IA?",
    "Funciona no celular?",
    "Meus dados vão para a nuvem?",
  ]) {
    await page.locator("#duvidas summary", { hasText: question }).click();
  }
  for (const theme of ["light", "dark"] as const) {
    await setTheme(page, theme);
    for (const width of [390, 1440]) {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        `${theme} ${width}: no horizontal scroll`,
      ).toBe(true);
      for (const section of ["recursos", "assistente", "patrimonio", "mobile", "duvidas"]) {
        await page.locator(`#${section}`).screenshot({
          path: join(screenshotsDir, `v3-${theme}-${width}-${section}.png`),
          animations: "disabled",
          // The sticky header would otherwise be stamped over the captured section.
          style: ".ff-header, .ff-skip-link { visibility: hidden !important; }",
        });
      }
    }
  }
});

test("theme follows OS, persists explicit choice, resets to system, and syncs across tabs", async ({
  page,
  context,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);
  await page.goto("/inicio");
  const root = page.locator("html");
  await expect(page.getByRole("button", { name: "Ativar tema escuro" }).first()).toBeVisible();
  await expect(root).toHaveAttribute("data-theme", "light");
  await expect(root).toHaveAttribute("data-theme-preference", "system");

  await page.emulateMedia({ colorScheme: "dark" });
  await expect(root).toHaveAttribute("data-theme", "dark");
  await page.getByRole("button", { name: "Ativar tema claro" }).click();
  await expect(root).toHaveAttribute("data-theme", "light");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("finflow-theme"))).toBe("light");
  await page.reload();
  await expect(root).toHaveAttribute("data-theme", "light");
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(root).toHaveAttribute("data-theme", "light");
  await page.getByRole("button", { name: "Usar tema do sistema" }).click();
  await expect(root).toHaveAttribute("data-theme-preference", "system");
  await expect(root).toHaveAttribute("data-theme", "dark");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("finflow-theme"))).toBeNull();

  const secondTab = await context.newPage();
  await secondTab.emulateMedia({ colorScheme: "dark" });
  await secondTab.goto("/inicio");
  await secondTab.getByRole("button", { name: "Ativar tema claro" }).click();
  await expect(root).toHaveAttribute("data-theme", "light");
  await expect(root).toHaveAttribute("data-theme-preference", "light");
});

test("theme remains usable when storage is denied and bootstraps before React", async ({
  page,
  context,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.addInitScript(() => {
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      get() {
        throw new DOMException("Storage access denied", "SecurityError");
      },
    });
  });
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/inicio");
  await page.getByRole("button", { name: "Ativar tema escuro" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  expect(errors).toEqual([]);

  const bootstrapPage = await context.newPage();
  await bootstrapPage.addInitScript(() => localStorage.setItem("finflow-theme", "dark"));
  await bootstrapPage.route("**/src/main.tsx", (route) => route.abort());
  await bootstrapPage.goto("/inicio", { waitUntil: "domcontentloaded" });
  await expect(bootstrapPage.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(bootstrapPage.locator("#root")).toBeEmpty();
});

test("/inicio stays within mobile, tablet, and desktop widths in light and dark themes", async ({
  page,
}, testInfo) => {
  const jsErrors: string[] = [];
  page.on("pageerror", (error) => jsErrors.push(error.message));
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);
  await page.setViewportSize({ width: 320, height: 844 });
  await page.goto("/inicio");
  await page.evaluate(() => {
    for (const image of document.images) image.loading = "eager";
  });
  await page.waitForFunction(
    () => Array.from(document.images).every((image) => image.complete),
    null,
    { timeout: 15_000 },
  );
  await scrollFullPage(page);
  await page.evaluate(() => window.scrollTo(0, 0));

  for (const theme of ["light", "dark"] as const) {
    if (theme === "dark") await setTheme(page, "dark");
    for (const viewport of [
      { name: "mobile-320", width: 320, height: 844 },
      { name: "mobile-375", width: 375, height: 812 },
      { name: "mobile-390", width: 390, height: 844 },
      { name: "mobile-430", width: 430, height: 900 },
      { name: "tablet-768", width: 768, height: 1024 },
      { name: "laptop-1024", width: 1024, height: 768 },
      { name: "desktop-1440", width: 1440, height: 900 },
    ]) {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await page.evaluate(() => window.scrollTo(0, 0));
      const layout = await page.evaluate(() => ({
        viewport: window.innerWidth,
        document: document.documentElement.scrollWidth,
        body: document.body.scrollWidth,
      }));
      expect(layout.document, `${theme} ${viewport.name} document width`).toBeLessThanOrEqual(
        viewport.width,
      );
      expect(layout.body, `${theme} ${viewport.name} body width`).toBeLessThanOrEqual(
        viewport.width,
      );
      if (testInfo.project.name === "desktop") {
        await saveScreenshot(page, `${theme}-${viewport.name}`);
      }
    }
  }

  const brokenImages = await page.locator("img").evaluateAll((images) =>
    images
      .map((element) => element as HTMLImageElement)
      .filter((image) => !image.complete || image.naturalWidth === 0)
      .map((image) => ({ src: image.currentSrc || image.src, alt: image.alt })),
  );
  expect(brokenImages, "all landing illustrations load").toEqual([]);
  expect(jsErrors, "the landing has no uncaught JavaScript errors").toEqual([]);
});

test("compact landing preview fits the intended desktop fold", async ({ page }, testInfo) => {
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);

  for (const viewport of [
    { name: "1366x768", width: 1366, height: 768 },
    { name: "1536x864", width: 1536, height: 864 },
    { name: "1920x900", width: 1920, height: 900 },
    { name: "1536x718", width: 1536, height: 718 },
  ]) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    await page.goto("/inicio");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.locator(".dashboard")).toBeVisible();
    await page.evaluate(async () => {
      await document.fonts.ready;
    });
    await page.evaluate(() => window.scrollTo(0, 0));

    const panelBox = await page.locator(".dashboard").boundingBox();
    const primaryBox = await page.locator(".ff-hero-actions .ff-button").boundingBox();
    const secondaryBox = await page.locator(".ff-hero-actions .ff-secondary-link").boundingBox();
    const tertiaryBox = await page.locator(".ff-hero-actions .ff-try-link").boundingBox();
    const noteBox = await page.locator(".ff-preview-note").boundingBox();
    if (!panelBox || !primaryBox || !secondaryBox || !tertiaryBox || !noteBox) {
      throw new Error(
        `${viewport.name}: hero, preview panel, and note must render before measuring`,
      );
    }
    const headingLines = await page.locator(".ff-hero h1").evaluate((element) => {
      const rect = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      const fontSize = Number.parseFloat(style.fontSize);
      const lineHeightValue = Number.parseFloat(style.lineHeight);
      const lineHeight = Number.isFinite(lineHeightValue) ? lineHeightValue : fontSize * 1.15;
      return Math.ceil(rect.height / lineHeight - 0.05);
    });
    const viewportHeight = await page.evaluate(() => window.innerHeight);

    const dashboard = page.locator("#demonstracao");
    await expect(dashboard.locator(".ff-preview-details")).not.toHaveAttribute("open", "");
    await expect(dashboard.getByRole("button", { name: "Entradas e saídas" })).toHaveCount(0);
    await expect(dashboard.getByRole("button", { name: "Simular uma despesa" })).toHaveCount(0);
    expect(headingLines, `${viewport.name} hero heading line count`).toBeLessThanOrEqual(3);
    expect(panelBox.height, `${viewport.name} default preview height`).toBeGreaterThanOrEqual(480);
    expect(panelBox.height, `${viewport.name} default preview height`).toBeLessThanOrEqual(540);
    expect(
      primaryBox.y + primaryBox.height,
      `${viewport.name} primary CTA fits`,
    ).toBeLessThanOrEqual(viewportHeight);
    expect(
      secondaryBox.y + secondaryBox.height,
      `${viewport.name} secondary CTA fits`,
    ).toBeLessThanOrEqual(viewportHeight);
    expect(
      tertiaryBox.y + tertiaryBox.height,
      `${viewport.name} browser link fits`,
    ).toBeLessThanOrEqual(viewportHeight);
    expect(panelBox.y + panelBox.height, `${viewport.name} default panel fits`).toBeLessThanOrEqual(
      viewportHeight,
    );
    expect(noteBox.y + noteBox.height, `${viewport.name} preview note fits`).toBeLessThanOrEqual(
      viewportHeight,
    );

    if (testInfo.project.name === "desktop") {
      await mkdir(screenshotsDir, { recursive: true });
      await page.screenshot({
        path: join(screenshotsDir, `fold-${viewport.name}.png`),
        animations: "disabled",
      });
    }
  }
});

test("landing preview interactions and app theme styling work across /inicio and /", async ({
  page,
}) => {
  await mockApi(page);
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);
  await page.goto("/inicio");

  const ai = page.locator("#assistente");
  const promptButtons = ai.getByRole("group", { name: "Perguntas de exemplo" }).getByRole("button");
  await expect(promptButtons).toHaveCount(3);
  for (const button of await promptButtons.all()) {
    await button.click();
    await expect(button).toHaveAttribute("aria-pressed", "true");
  }

  const mobilePreview = page.locator("#mobile");
  const phoneSteps = mobilePreview.getByRole("group", { name: "Telas do aplicativo" });
  await expect(phoneSteps.getByRole("button")).toHaveCount(3);
  const addExpenseStep = phoneSteps.getByRole("button", {
    name: /O pequeno gasto também conta/,
  });
  await addExpenseStep.click();
  await expect(addExpenseStep).toHaveAttribute("aria-pressed", "true");
  await expect(mobilePreview.locator(".ff-phone-view")).toContainText("Nova despesa");
  await mobilePreview.getByRole("button", { name: "Registrar despesa", exact: true }).click();
  await expect(mobilePreview.locator(".ff-phone-toast")).toContainText(
    "Despesa de R$ 32,00 registrada",
  );
  await mobilePreview.getByRole("button", { name: "Registrada. Toque para desfazer" }).click();
  await phoneSteps.getByRole("button", { name: /Seu mês num relance/ }).click();
  await expect(mobilePreview.locator(".ff-phone-balance strong")).toHaveText("R$ 6.420,00");
  const phoneTabs = mobilePreview.getByRole("group", { name: "Navegação do aplicativo" });
  const statementTab = phoneTabs.getByRole("button", { name: "Extrato" });
  await statementTab.click();
  await expect(statementTab).toHaveAttribute("aria-pressed", "true");
  await expect(mobilePreview.locator(".ff-phone-view")).toContainText("Extrato");
  const goalsTab = phoneTabs.getByRole("button", { name: "Metas" });
  await goalsTab.click();
  await expect(goalsTab).toHaveAttribute("aria-pressed", "true");
  await expect(mobilePreview.locator(".ff-phone-view")).toContainText("Minhas metas");

  const telegramPreview = mobilePreview.locator(".ff-telegram");
  await telegramPreview.getByRole("button", { name: "Ver exemplo de consulta" }).click();
  await expect(telegramPreview.getByRole("status")).toContainText(/R\$\s+24\.586,50/);
  await telegramPreview.getByRole("button", { name: "Reiniciar exemplo" }).click();
  await expect(telegramPreview.getByRole("status")).toHaveCount(0);

  const dashboard = page.locator("#demonstracao");
  const previewDetails = dashboard.locator(".ff-preview-details");
  const previewSummary = dashboard.locator(".ff-preview-details > summary");
  await expect(dashboard.getByRole("heading", { name: "Visão geral" })).toBeVisible();
  await expect(previewDetails).not.toHaveAttribute("open", "");
  const balance = dashboard.locator(".balance strong");
  await expect(balance).toHaveText("R$ 24.586,50");
  await expect(dashboard.locator(".flows .ff-income")).toHaveText("R$ 8.500,00");
  await expect(dashboard.locator(".flows .ff-expense")).toHaveText("R$ 5.716,50");
  const chart = dashboard.getByRole("img");
  await expect(chart).toHaveAttribute("aria-label", /Evolução do saldo de Novembro/i);
  await expect(chart).toHaveAttribute("aria-label", /Dia 30: saldo R\$\s+24\.586,50/);
  await expect(dashboard.locator(".ff-chart-axis")).toBeVisible();
  await expect(dashboard.locator(".ff-chart-plot polyline[points]")).toHaveAttribute(
    "points",
    /.+/,
  );
  const staticDates = dashboard.locator(".ff-chart-dates");
  await expect(staticDates).toContainText("07 nov");
  await expect(staticDates).toContainText("30 nov");
  await expect(dashboard.getByRole("button", { name: "Entradas e saídas" })).toHaveCount(0);
  await expect(dashboard.getByRole("button", { name: "Simular uma despesa" })).toHaveCount(0);

  await previewSummary.click();
  await expect(previewDetails).toHaveAttribute("open", "");
  await expect(previewSummary).toHaveText("Explorar exemplo+");
  await expect(dashboard.locator(".ff-chart-readout")).toContainText(/R\$\s+24\.586,50/);
  const chartDates = dashboard.locator(".chart-months button");
  await expect(chartDates).toHaveCount(4);
  await expect(chartDates.first()).toHaveAccessibleName("07 nov");
  await chartDates.first().click();
  await expect(dashboard.locator(".ff-chart-readout")).toContainText("Saldo em 07 nov");
  await expect(dashboard.locator(".ff-chart-readout")).toContainText(/R\$\s+27\.019,00/);
  await chartDates.last().click();
  await expect(dashboard.locator(".ff-chart-readout")).toContainText(/R\$\s+24\.586,50/);

  const chartViews = dashboard.getByRole("group", { name: "Visualização do gráfico" });
  await chartViews.getByRole("button", { name: "Entradas e saídas" }).click();
  await expect(chart).toHaveAttribute("aria-label", /Entradas e saídas de Novembro/i);
  await expect(dashboard.locator(".ff-chart-readout")).toContainText("Entradas");
  await expect(dashboard.locator(".ff-chart-readout")).toContainText("Saídas");
  await chartViews.getByRole("button", { name: "Saldo ao longo do mês" }).click();
  await expect(chart).toHaveAttribute("aria-label", /Evolução do saldo de Novembro/i);

  const novemberLabels = await dashboard.locator(".chart-months").innerText();
  expect(novemberLabels).toMatch(/07[\s\S]*14[\s\S]*21[\s\S]*30/i);
  expect(novemberLabels).toMatch(/nov/i);
  const novemberSeries = await chart
    .locator("polyline[points]")
    .evaluateAll((lines) => lines.map((line) => line.getAttribute("points")));
  expect(novemberSeries.length, "cashflow chart exposes plotted series").toBeGreaterThan(0);

  const periodControl = dashboard.getByRole("combobox", { name: "Período do dashboard" });
  await periodControl.selectOption("Outubro");
  await expect(balance).toHaveText("R$ 21.803,00");
  await expect(chart).toHaveAttribute("aria-label", /Evolução do saldo de Outubro/i);
  await expect(chart).toHaveAttribute("aria-label", /Dia 30: saldo R\$\s+21\.803,00/);
  await expect(dashboard.locator(".ff-chart-readout")).toContainText(/R\$\s+21\.803,00/);
  const octoberLabels = await dashboard.locator(".chart-months").innerText();
  expect(octoberLabels).toMatch(/07[\s\S]*14[\s\S]*21[\s\S]*30/i);
  expect(octoberLabels).toMatch(/out/i);
  expect(octoberLabels).not.toBe(novemberLabels);
  const octoberSeries = await chart
    .locator("polyline[points]")
    .evaluateAll((lines) => lines.map((line) => line.getAttribute("points")));
  expect(octoberSeries, "changing period updates the plotted cashflow").not.toEqual(novemberSeries);

  await periodControl.selectOption("Novembro");
  const beforeExpenseSeries = await chart
    .locator("polyline[points]")
    .evaluateAll((lines) => lines.map((line) => line.getAttribute("points")));
  await dashboard.getByRole("button", { name: "Simular uma despesa" }).click();
  await expect(balance).toHaveText("R$ 24.562,00");
  await expect(chart).toHaveAttribute("aria-label", /Dia 30: saldo R\$\s+24\.562,00/);
  await expect(dashboard.locator(".ff-chart-readout")).toContainText(/R\$\s+24\.562,00/);
  await expect(dashboard.getByText("Café com a equipe")).toBeVisible();
  await expect(dashboard.locator(".flows .ff-expense")).toHaveText("R$ 5.741,00");
  const afterExpenseSeries = await chart
    .locator("polyline[points]")
    .evaluateAll((lines) => lines.map((line) => line.getAttribute("points")));
  expect(afterExpenseSeries, "the chart reflects the added expense").not.toEqual(
    beforeExpenseSeries,
  );

  const downloadPromise = page.waitForEvent("download");
  await dashboard.getByRole("button", { name: "Baixar exemplo CSV" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe("finflow-exemplo-novembro.csv");
  const csvPath = await download.path();
  expect(csvPath).not.toBeNull();
  if (!csvPath) throw new Error("CSV download did not produce a local file");
  const csv = await readFile(csvPath, "utf8");
  expect(csv).toContain("24,50");

  await dashboard.getByRole("button", { name: "Desfazer despesa" }).click();
  await expect(balance).toHaveText("R$ 24.586,50");

  const calculator = page.locator(".ff-calculator");
  await calculator.getByLabel("Em quanto tempo?").selectOption("18");
  await expect(calculator.locator(".ff-calculator-result")).toContainText("R$ 1.333,34");
  const faq = page.locator("#duvidas details").first();
  await faq.locator("summary").click();
  await expect(faq).toHaveAttribute("open", "");

  await page.evaluate(() => window.scrollTo(0, 0));
  await page.getByRole("button", { name: "Ativar tema escuro" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const landingBg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  expect(landingBg).toBe("rgb(9, 9, 11)");

  if (await page.getByRole("button", { name: "Abrir menu" }).isVisible()) {
    await page.getByRole("button", { name: "Abrir menu" }).click();
  }
  await page.getByRole("link", { name: "Acessar minha conta" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.locator('[data-testid="hero-balance"]')).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const appBg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  expect(appBg).toBe("rgb(9, 9, 11)");

  await page.getByRole("button", { name: "Ativar tema claro" }).first().click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await expect
    .poll(() => page.evaluate(() => getComputedStyle(document.body).backgroundColor))
    .toBe("rgb(250, 250, 250)");
});

test("axe WCAG AA passes on /inicio in both themes with dashboard, AI, calculator, and FAQ", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);
  await page.goto("/inicio");
  await page.locator("#duvidas details").first().locator("summary").click();
  await page.locator("#duvidas summary", { hasText: "Preciso pagar pela IA?" }).click();
  // Audit the assistant with a verdict, a pending confirmation and the debt toggle in use.
  await page
    .locator("#assistente")
    .getByRole("button", { name: "Posso comprar um celular em 10x?" })
    .click();
  await page
    .getByRole("group", { name: "Estratégia de quitação" })
    .getByRole("button", { name: "Bola de neve" })
    .click();

  const dashboard = page.locator("#demonstracao");
  await dashboard.locator(".ff-preview-details > summary").click();
  await dashboard.locator(".ff-chart-data summary").click();
  await dashboard.locator(".budget-panel summary").click();
  const chartViews = page.getByRole("group", { name: "Visualização do gráfico" });
  for (const theme of ["light", "dark"] as const) {
    if (theme === "dark") await setTheme(page, "dark");
    await audit(page, `${theme} /inicio, balance chart, dashboard, AI, calculator, and FAQ`);
    await chartViews.getByRole("button", { name: "Entradas e saídas" }).click();
    await audit(page, `${theme} /inicio, comparison chart, dashboard, AI, calculator, and FAQ`);
    await chartViews.getByRole("button", { name: "Saldo ao longo do mês" }).click();
  }
});

test("dashboard data and budgets expand cleanly at 320px", async ({ page }, testInfo) => {
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);
  await page.setViewportSize({ width: 320, height: 900 });
  await page.goto("/inicio");

  const dashboard = page.locator("#demonstracao");
  await dashboard.locator(".ff-preview-details > summary").click();
  const chartData = dashboard.locator(".ff-chart-data");
  await chartData.locator("summary").click();
  await expect(chartData).toHaveAttribute("open", "");
  await expect(chartData.getByRole("row")).toHaveCount(5);

  const budgets = dashboard.locator(".budget-panel");
  await budgets.locator("summary").click();
  await expect(budgets).toHaveAttribute("open", "");
  await expect(budgets.getByRole("meter")).toHaveCount(3);

  const layout = await page.evaluate(() => ({
    document: document.documentElement.scrollWidth,
    body: document.body.scrollWidth,
  }));
  expect(layout.document, "expanded dashboard document width at 320px").toBeLessThanOrEqual(320);
  expect(layout.body, "expanded dashboard body width at 320px").toBeLessThanOrEqual(320);

  const targets = await dashboard.evaluate((root) =>
    Array.from(root.querySelectorAll<HTMLElement>("button, select, summary"))
      .filter((element) => element.getClientRects().length > 0)
      .map((element) => {
        const rect = element.getBoundingClientRect();
        return {
          label:
            element.textContent?.trim() || element.getAttribute("aria-label") || element.tagName,
          width: Math.round(rect.width),
          height: Math.round(rect.height),
        };
      }),
  );
  for (const target of targets) {
    expect(target.width, `${target.label} width at 320px`).toBeGreaterThanOrEqual(44);
    expect(target.height, `${target.label} height at 320px`).toBeGreaterThanOrEqual(44);
  }

  if (testInfo.project.name === "desktop") {
    await saveScreenshot(page, "light-dashboard-expanded-320");
  }
  await setTheme(page, "dark");
  if (testInfo.project.name === "desktop") {
    await saveScreenshot(page, "dark-dashboard-expanded-320");
  }
});

test("mobile menu supports keyboard use and landing controls meet touch target size", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto("/inicio");
  const menuBtn = page.getByRole("button", { name: "Abrir menu" });
  await expect(menuBtn).toBeVisible();
  await menuBtn.focus();
  await page.keyboard.press("Enter");

  const closeBtn = page.getByRole("button", { name: "Fechar menu" });
  await expect(closeBtn).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator("#ff-mobile-nav")).toBeVisible();
  expect(await page.evaluate(() => document.body.style.overflow)).toBe("hidden");
  const mobileNavTargets = await page.evaluate(() => {
    const elements = [
      document.querySelector(".ff-menu-button"),
      ...Array.from(document.querySelectorAll("#ff-mobile-nav a, #ff-mobile-nav button")),
    ].filter((el): el is HTMLElement => el !== null);
    return elements.map((el) => {
      const rect = el.getBoundingClientRect();
      return {
        label: el.textContent?.trim() || el.getAttribute("aria-label") || el.tagName,
        width: Math.round(rect.width),
        height: Math.round(rect.height),
      };
    });
  });
  for (const target of mobileNavTargets) {
    expect(target.width, `${target.label} touch width`).toBeGreaterThanOrEqual(44);
    expect(target.height, `${target.label} touch height`).toBeGreaterThanOrEqual(44);
  }

  await page.keyboard.press("Escape");
  await expect(page.locator("#ff-mobile-nav")).toBeHidden();
  await expect(menuBtn).toBeFocused();
  expect(await page.evaluate(() => document.body.style.overflow)).not.toBe("hidden");

  await page.keyboard.press("Enter");
  await expect(page.locator("#ff-mobile-nav")).toBeVisible();
  await page.keyboard.press("Tab");
  const resourcesLink = page.locator('#ff-mobile-nav a[href="#recursos"]');
  await expect(resourcesLink).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("#ff-mobile-nav")).toBeHidden();
  await expect(page).toHaveURL(/#recursos$/);
  expect(await page.evaluate(() => document.body.style.overflow)).not.toBe("hidden");

  await page.setViewportSize({ width: 375, height: 812 });
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.locator("#demonstracao .ff-preview-details > summary").click();
  // Show the confirmation buttons so their size is measured too.
  await page
    .locator("#assistente")
    .getByRole("button", { name: "Posso comprar um celular em 10x?" })
    .click();
  const landingControlTargets = await page.evaluate(() => {
    const elements = Array.from(
      document.querySelectorAll<HTMLElement>(
        ".ff-hero-actions a, .ff-theme-control button, #demonstracao button, #demonstracao select, #demonstracao summary, #baixar a, #baixar button, #assistente button, #patrimonio button, #duvidas summary",
      ),
    ).filter((element) => element.getClientRects().length > 0);
    return elements.map((element) => {
      const rect = element.getBoundingClientRect();
      return {
        label: element.textContent?.trim() || element.getAttribute("aria-label") || element.tagName,
        width: Math.round(rect.width),
        height: Math.round(rect.height),
      };
    });
  });
  for (const target of landingControlTargets) {
    expect(target.width, `${target.label} touch width`).toBeGreaterThanOrEqual(44);
    expect(target.height, `${target.label} touch height`).toBeGreaterThanOrEqual(44);
  }
});
