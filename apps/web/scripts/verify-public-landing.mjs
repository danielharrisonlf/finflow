import assert from "node:assert/strict";
import { mkdir, readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium, webkit } from "@playwright/test";

const url = process.argv[2];
if (!url) throw new Error("Pass the local or production landing URL");
// FinFlow in the browser is published with the landing (docs/browser-runtime-contract.md §5).
const appUrl = new URL("/app/", url).href;
const artifacts = join(
  dirname(fileURLToPath(import.meta.url)),
  "../test-results/public-verification",
);
await mkdir(artifacts, { recursive: true });
const axe = await readFile(createRequire(import.meta.url).resolve("axe-core/axe.min.js"), "utf8");
for (const [engine, browserType] of [
  ["chromium", chromium],
  ["webkit", webkit],
]) {
  const browser = await browserType.launch();
  try {
    const page = await browser.newPage({
      viewport: { width: 1536, height: 718 },
      colorScheme: "light",
      reducedMotion: "reduce",
    });
    const errors = [];
    const api = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("request", (request) => {
      const requested = new URL(request.url());
      // The assistant example is scripted locally: neither the app API nor Anthropic is contacted.
      if (requested.pathname.startsWith("/api/") || requested.hostname.endsWith("anthropic.com")) {
        api.push(request.url());
      }
    });
    const response = await page.goto(url);
    assert.equal(response.status(), 200);
    await page.getByRole("heading", { level: 1 }).waitFor();
    await page.evaluate(() => document.fonts.ready);
    assert.equal(
      (await page.locator("h1").innerText()).replace(/\s+/g, " ").trim(),
      "Seu dinheiro em ordem. Mais espaço para viver.",
    );
    const dashboard = page.locator("#demonstracao");
    assert.equal(await dashboard.locator(".ff-preview-details").getAttribute("open"), null);
    assert.equal(
      await page.getByRole("link", { name: /demonstração|criar conta|cadastr/i }).count(),
      0,
    );
    assert.equal(await page.getByText(/disponível em breve|em preparação/i).count(), 0);
    assert.equal(await page.getByText(/Breno|Visor/i).count(), 0);
    assert.equal(await page.getByText(/ChatGPT|OpenAI|Gemini|Copilot/i).count(), 0);
    for (const theme of ["light", "dark"]) {
      if (theme === "dark")
        await page.getByRole("button", { name: "Ativar tema escuro", exact: true }).click();
      await page.addScriptTag({ content: axe });
      const violations = await page.evaluate(async () => {
        const result = await window.axe.run(document, {
          runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"] },
        });
        return result.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) }));
      });
      assert.deepEqual(violations, [], `${engine} ${theme}: accessibility`);
      for (const width of [320, 375, 390, 430, 768, 1024, 1536]) {
        await page.setViewportSize({ width, height: width >= 1024 ? 718 : 844 });
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          `${engine} ${theme} ${width}: overflow`,
        );
        if (width === 1536) {
          const metrics = await page.evaluate(() => ({
            panel: document.querySelector(".dashboard").getBoundingClientRect().height,
            noteBottom: document.querySelector(".ff-preview-note").getBoundingClientRect().bottom,
          }));
          assert.ok(metrics.panel >= 480 && metrics.panel <= 540);
          assert.ok(metrics.noteBottom <= 718);
          await page.screenshot({ path: join(artifacts, `${engine}-${theme}.png`) });
        }
      }
    }
    // The public page has no accounts: the header action opens FinFlow in the browser, never a
    // login. The hero offers the download first, the source code second and the browser app as a
    // third, text-only option (docs/local-agent-contract.md §4).
    assert.equal(await page.getByText("Acessar minha conta").count(), 0);
    assert.equal(await page.getByRole("dialog").count(), 0);
    const nav = page.locator(".ff-nav-actions");
    assert.equal(
      await nav.getByRole("link", { name: "Experimentar no navegador" }).getAttribute("href"),
      "/app/",
    );
    const hero = page.locator(".ff-hero-actions");
    assert.equal(await hero.getByRole("link").count(), 3);
    assert.equal(
      await hero.getByRole("link", { name: "Experimentar no navegador" }).getAttribute("href"),
      "/app/",
    );
    assert.equal(
      await hero.getByRole("link", { name: "Código no GitHub" }).getAttribute("href"),
      "https://github.com/danielharrisonlf/finflow",
    );
    await hero.getByRole("link", { name: "Baixar grátis" }).click();
    await page.waitForURL(/#baixar$/);
    const primary = page.locator("#baixar .ff-download-primary");
    assert.match(
      await primary.getAttribute("href"),
      /^https:\/\/github\.com\/danielharrisonlf\/finflow\/releases\/latest\/download\/finflow-(windows|darwin|linux)-(amd64|arm64)\.(zip|tar\.gz)$/,
    );
    assert.equal(await page.locator("#baixar .ff-download-others a").count(), 5);
    const tabs = page.locator("#baixar").getByRole("tab");
    assert.deepEqual(await tabs.allInnerTexts(), ["Executável", "Código-fonte", "Docker"]);
    await tabs.nth(1).click();
    assert.equal(
      (await page.locator("#baixar .ff-install-command code").first().innerText()).trim(),
      "git clone https://github.com/danielharrisonlf/finflow && cd finflow && pnpm install && pnpm start",
    );
    await tabs.nth(2).click();
    assert.match(await page.locator("#baixar").innerText(), /docker compose --profile ai up -d/);
    await tabs.nth(0).click();
    await dashboard.locator(".ff-preview-details > summary").click();
    assert.equal(await dashboard.locator(".balance strong").innerText(), "R$ 24.586,50");
    await dashboard.getByRole("button", { name: "Simular uma despesa", exact: true }).click();
    assert.equal(await dashboard.locator(".balance strong").innerText(), "R$ 24.562,00");
    await dashboard.getByRole("button", { name: "Desfazer despesa", exact: true }).click();
    const downloadPromise = page.waitForEvent("download");
    await dashboard.getByRole("button", { name: "Baixar exemplo CSV", exact: true }).click();
    const download = await downloadPromise;
    assert.equal(download.suggestedFilename(), "finflow-exemplo-novembro.csv");
    await dashboard.getByRole("combobox", { name: "Período do dashboard" }).selectOption("Outubro");
    assert.equal(await dashboard.locator(".balance strong").innerText(), "R$ 21.803,00");
    await page.getByLabel("Em quanto tempo?", { exact: true }).selectOption("18");
    assert.match(await page.locator(".ff-calculator-result").innerText(), /1\.333,34/);

    // v3: assistant example, net worth and debt strategy, calculators, mobile and FAQ.
    const ai = page.locator("#assistente");
    assert.equal(
      (await ai.getByRole("heading", { level: 2 }).innerText()).trim(),
      "Pergunte ao FinFlow.",
    );
    assert.match(await ai.innerText(), /modelo local \(Ollama ou LM Studio\)/);
    assert.match(await ai.innerText(), /sua própria chave da API do Claude/);
    const questions = ai.getByRole("group", { name: "Perguntas de exemplo" }).getByRole("button");
    assert.deepEqual(await questions.allInnerTexts(), [
      "Posso jantar fora esse mês?",
      "Posso fazer uma viagem?",
      "Posso comprar um celular em 10x?",
    ]);
    const verdict = ai.locator(".ff-assistant-verdict");
    assert.equal(await verdict.innerText(), "Cabe no mês");
    await questions.nth(1).click();
    assert.equal(await verdict.innerText(), "Com cuidado");
    await questions.nth(2).click();
    assert.equal(await verdict.innerText(), "Melhor esperar");
    const proposal = ai.locator(".ff-assistant-proposal");
    await proposal.getByRole("button", { name: "Confirmar", exact: true }).click();
    assert.match(await proposal.getByRole("status").innerText(), /Exemplo: nada foi salvo/);
    await ai.getByRole("button", { name: "Enviar foto do comprovante", exact: true }).click();
    assert.match(await ai.locator(".ff-assistant-attachment").innerText(), /comprovante\.jpg/);
    assert.match(await proposal.innerText(), /Nova despesa/);
    const wealth = page.locator("#patrimonio");
    assert.equal(await wealth.locator(".ff-wealth-chart rect").count(), 12);
    await wealth.getByRole("button", { name: "Bola de neve", exact: true }).click();
    assert.match(await wealth.locator(".ff-wealth-plan").innerText(), /Menor saldo primeiro/);
    assert.equal(
      await wealth.getByRole("list", { name: "Calculadoras incluídas" }).locator("li").count(),
      11,
    );
    assert.match(await page.locator("#mobile .ff-mobile-lan").innerText(), /finflow serve --lan/);
    for (const question of [
      "Preciso pagar pela IA?",
      "Funciona no celular?",
      "Meus dados vão para a nuvem?",
      "Onde ficam meus dados na versão do navegador?",
      "Posso levar meus dados para o app instalado?",
    ]) {
      const item = page.locator("#duvidas details", { hasText: question });
      await item.locator("summary").click();
      assert.equal(await item.getAttribute("open"), "");
    }
    const targets = await page.evaluate(
      () =>
        Array.from(document.querySelectorAll("#assistente button, #patrimonio button"))
          .filter((element) => element.getClientRects().length > 0)
          .map((element) => element.getBoundingClientRect())
          .filter((rect) => rect.width < 44 || rect.height < 44).length,
    );
    assert.equal(targets, 0, `${engine}: v3 touch targets`);
    assert.deepEqual(errors, []);
    assert.deepEqual(api, []);
    for (const asset of ["/robots.txt", "/sitemap.xml", "/fonts/geist-latin.woff2"]) {
      assert.equal((await page.request.get(new URL(asset, url).href)).status(), 200, asset);
    }
    console.log(
      JSON.stringify({
        engine,
        url,
        status: "passed",
        widths: [320, 375, 390, 430, 768, 1024, 1536],
        themes: 2,
        apiRequests: api.length,
      }),
    );
  } finally {
    await browser.close();
  }
}

// FinFlow in the browser at /app/: the runtime starts, demo data can be created, and the only
// requests are the site's own files (the assistant is never contacted here). Chromium is the
// reference engine; WebKit is reported but may lack the OPFS sync access the runtime needs.
for (const [engine, browserType] of [
  ["chromium", chromium],
  ["webkit", webkit],
]) {
  const browser = await browserType.launch();
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const origin = new URL(url).origin;
    const foreign = [];
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    context.on("request", (request) => {
      const requested = request.url();
      if (/^(data|blob):/.test(requested)) return;
      if (new URL(requested).origin !== origin) foreign.push(requested);
    });
    const response = await page.goto(appUrl);
    assert.equal(response.status(), 200, `${engine}: /app/ status`);
    await page
      .locator('[data-testid="first-run"], [data-testid="runtime-error"]')
      .first()
      .waitFor({ timeout: 90_000 });
    const outcome = (await page.getByTestId("runtime-error").count()) > 0 ? "error" : "first-run";
    if (outcome === "error") {
      const message = await page.getByTestId("runtime-error").innerText();
      assert.notEqual(engine, "chromium", `chromium: runtime failed to start: ${message}`);
      console.log(JSON.stringify({ engine, url: appUrl, status: "runtime-unsupported", message }));
      continue;
    }
    await page.getByRole("button", { name: /Explorar com dados de exemplo/ }).click();
    const balance = page.getByTestId("hero-balance");
    await balance.waitFor({ timeout: 60_000 });
    assert.match(await balance.innerText(), /R\$/, `${engine}: dashboard numbers`);
    assert.deepEqual(errors, [], `${engine}: app page errors`);
    assert.deepEqual(foreign, [], `${engine}: app requests outside the site`);

    // Hosting contract: SPA fallback, no-cache shell and service worker, wasm MIME type.
    const route = await page.request.get(new URL("/app/configuracoes", url).href);
    assert.equal(route.status(), 200, "SPA fallback for /app/*");
    assert.match(route.headers()["content-type"], /text\/html/);
    const sw = await page.request.get(new URL("/app/sw.js", url).href);
    assert.equal(sw.status(), 200, "/app/sw.js");
    const swSource = await sw.text();
    const precache = JSON.parse(/const PRECACHE = (\[[^\n]*\]);/.exec(swSource)?.[1] ?? "[]");
    const wasm = precache.find((file) => file.endsWith(".wasm"));
    assert.ok(wasm, "a .wasm file is precached");
    const wasmResponse = await page.request.get(new URL(`/app/${wasm}`, url).href);
    assert.equal(wasmResponse.status(), 200, wasm);
    assert.equal(wasmResponse.headers()["content-type"], "application/wasm");
    const manifest = await page.request.get(new URL("/app/manifest.webmanifest", url).href);
    const appManifest = await manifest.json();
    assert.equal(appManifest.start_url, "/app/");
    assert.equal(appManifest.scope, "/app/");
    console.log(
      JSON.stringify({
        engine,
        url: appUrl,
        status: "passed",
        precached: precache.length,
        foreignRequests: foreign.length,
        swCacheControl: sw.headers()["cache-control"] ?? null,
        indexCacheControl: route.headers()["cache-control"] ?? null,
      }),
    );
  } finally {
    await browser.close();
  }
}
