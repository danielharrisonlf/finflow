import { mkdir, readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { join } from "node:path";
import { expect, type Page, test } from "@playwright/test";
import { mockApi } from "./mocks";

const screenshotsDir = join(process.cwd(), "test-results", "screenshots");
const themeButtonName = /Ativar tema (claro|escuro)/;

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
  const axePath = createRequire(import.meta.url).resolve("axe-core/axe.min.js");
  const axeScript = await readFile(axePath, "utf8");
  await page.addScriptTag({ content: axeScript });
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
    "O controle total do seu dinheiro, com a simplicidade que você sempre quis.",
  );
  await expect(
    page.getByRole("heading", { name: "Conecte seu agente de IA via CLI ou MCP" }),
  ).toBeVisible();
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
}) => {
  const jsErrors: string[] = [];
  page.on("pageerror", (error) => jsErrors.push(error.message));
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/inicio");
  await scrollFullPage(page);
  await page.evaluate(() => {
    for (const image of document.images) image.loading = "eager";
  });
  await page.waitForFunction(
    () => Array.from(document.images).every((image) => image.complete),
    null,
    { timeout: 15_000 },
  );

  for (const theme of ["light", "dark"] as const) {
    if (theme === "dark") await setTheme(page, "dark");
    for (const viewport of [
      { name: "mobile-375", width: 375, height: 812 },
      { name: "mobile-390", width: 390, height: 844 },
      { name: "tablet-768", width: 768, height: 1024 },
      { name: "desktop-1440", width: 1440, height: 900 },
    ]) {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
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
      if (viewport.width === 390 || viewport.width === 1440) {
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

test("landing demo interactions and app theme styling work across /inicio and /", async ({
  page,
}) => {
  await mockApi(page);
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);
  await page.goto("/inicio");

  const ai = page.locator("#assistente");
  const promptButtons = ai.getByRole("group", { name: "Exemplos de conversa" }).getByRole("button");
  await expect(promptButtons).toHaveCount(3);
  for (const button of await promptButtons.all()) {
    await button.click();
    await expect(button).toHaveAttribute("aria-pressed", "true");
  }

  const dashboard = page.locator("#demonstracao");
  await expect(dashboard.getByText("R$ 24.586,50")).toBeVisible();
  await dashboard.getByRole("button", { name: "Simular uma despesa" }).click();
  await expect(dashboard.getByText("R$ 24.562,00")).toBeVisible();
  await dashboard.getByRole("button", { name: "Desfazer despesa" }).click();
  await expect(dashboard.getByText("R$ 24.586,50")).toBeVisible();
  await dashboard.getByRole("combobox", { name: "Período do dashboard" }).selectOption("Outubro");
  await expect(dashboard.getByText("R$ 21.803,00")).toBeVisible();

  const calculator = page.locator(".ff-calculator");
  await calculator.getByLabel("Em quanto tempo?").selectOption("18");
  await expect(calculator.locator(".ff-calculator-result")).toContainText("R$ 1.333,34");
  const faq = page.locator("#duvidas details").first();
  await faq.locator("summary").click();
  await expect(faq).toHaveAttribute("open", "");

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

  for (const theme of ["light", "dark"] as const) {
    if (theme === "dark") await setTheme(page, "dark");
    await audit(page, `${theme} /inicio, dashboard, AI, goal calculator, and expanded FAQ`);
  }
});

test("notebook above-the-fold fit, mobile drawer interactions, and touch target sizes", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await clearThemeForFirstDocument(page);

  for (const notebook of [
    { name: "notebook-1366x768", width: 1366, height: 768 },
    { name: "notebook-1366x680", width: 1366, height: 680 },
    { name: "notebook-1280x720-150pct", width: 1280, height: 720 },
    { name: "notebook-1536x864-125pct", width: 1536, height: 864 },
  ]) {
    await page.setViewportSize({ width: notebook.width, height: notebook.height });
    await page.goto("/inicio");

    const metrics = await page.evaluate(() => {
      const h1 = document.querySelector(".ff-hero h1")?.getBoundingClientRect();
      const cta = document.querySelector(".ff-hero-actions .ff-button")?.getBoundingClientRect();
      const secondary = document
        .querySelector(".ff-hero-actions .ff-secondary-link")
        ?.getBoundingClientRect();
      const bezel = document.querySelector(".dashboard-bezel")?.getBoundingClientRect();
      const note = document.querySelector(".ff-preview-note")?.getBoundingClientRect();
      return {
        viewportHeight: window.innerHeight,
        h1Bottom: h1?.bottom ?? 0,
        ctaBottom: cta?.bottom ?? 0,
        secondaryBottom: secondary?.bottom ?? 0,
        bezelHeight: bezel?.height ?? 0,
        bezelBottom: bezel?.bottom ?? 0,
        noteBottom: note?.bottom ?? 0,
      };
    });

    expect(
      metrics.bezelHeight,
      `${notebook.name} DemoDashboard height should stay compact`,
    ).toBeLessThanOrEqual(440);
    expect(
      metrics.ctaBottom,
      `${notebook.name} primary CTA fits above the fold`,
    ).toBeLessThanOrEqual(metrics.viewportHeight);
    expect(
      metrics.secondaryBottom,
      `${notebook.name} secondary CTA fits above the fold`,
    ).toBeLessThanOrEqual(metrics.viewportHeight);
    expect(
      metrics.bezelBottom,
      `${notebook.name} DemoDashboard fits completely above the fold`,
    ).toBeLessThanOrEqual(metrics.viewportHeight);
    expect(
      metrics.noteBottom,
      `${notebook.name} preview footer note fits above the fold`,
    ).toBeLessThanOrEqual(metrics.viewportHeight);
  }

  // Test hamburger menu at 1024px and 375px
  await page.setViewportSize({ width: 1024, height: 768 });
  const menuBtn = page.getByRole("button", { name: "Abrir menu" });
  await expect(menuBtn).toBeVisible();
  await menuBtn.click();

  const closeBtn = page.getByRole("button", { name: "Fechar menu" });
  await expect(closeBtn).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator("#ff-mobile-nav")).toBeVisible();
  expect(await page.evaluate(() => document.body.style.overflow)).toBe("hidden");

  // Touch targets >= 44x44px inside mobile nav and menu button
  const touchSizes = await page.evaluate(() => {
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
  for (const item of touchSizes) {
    expect(item.width, `${item.label} touch width`).toBeGreaterThanOrEqual(44);
    expect(item.height, `${item.label} touch height`).toBeGreaterThanOrEqual(44);
  }

  // Close with Escape
  await page.keyboard.press("Escape");
  await expect(page.locator("#ff-mobile-nav")).toBeHidden();
  expect(await page.evaluate(() => document.body.style.overflow)).not.toBe("hidden");

  // Reopen and close by clicking anchor link
  await menuBtn.click();
  await expect(page.locator("#ff-mobile-nav")).toBeVisible();
  await page.locator("#ff-mobile-nav a", { hasText: "Recursos" }).click();
  await expect(page.locator("#ff-mobile-nav")).toBeHidden();
  expect(await page.evaluate(() => document.body.style.overflow)).not.toBe("hidden");

  // Mobile (< 768px) layout & zero horizontal overflow across 360px - 430px
  for (const width of [360, 375, 430]) {
    await page.setViewportSize({ width, height: 800 });
    const mobileState = await page.evaluate(() => {
      const cashflow = document.querySelector(".cashflow")?.getBoundingClientRect();
      const budgets = document.querySelector(".budget-panel")?.getBoundingClientRect();
      return {
        docWidth: document.documentElement.scrollWidth,
        bodyWidth: document.body.scrollWidth,
        cashflowVisible: (cashflow?.height ?? 0) > 0,
        budgetsVisible: (budgets?.height ?? 0) > 0,
        stackedCleanly: (budgets?.top ?? 0) >= (cashflow?.bottom ?? 0) - 2,
      };
    });
    expect(mobileState.docWidth, `mobile ${width}px docWidth`).toBeLessThanOrEqual(width);
    expect(mobileState.bodyWidth, `mobile ${width}px bodyWidth`).toBeLessThanOrEqual(width);
    expect(mobileState.cashflowVisible, `mobile ${width}px cashflow visible`).toBe(true);
    expect(mobileState.budgetsVisible, `mobile ${width}px budgets visible`).toBe(true);
    expect(mobileState.stackedCleanly, `mobile ${width}px cashflow and budgets stacked`).toBe(true);
  }
});
