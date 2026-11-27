import assert from "node:assert/strict";
import { mkdir, readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium, webkit } from "@playwright/test";

const url = process.argv[2];
if (!url) throw new Error("Pass the local or production landing URL");
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
      if (new URL(request.url()).pathname.startsWith("/api/")) api.push(request.url());
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
    assert.equal(await page.getByRole("link", { name: /demonstração|grátis/i }).count(), 0);
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
    await page.getByRole("button", { name: "Acessar minha conta", exact: true }).click();
    const dialog = page.getByRole("dialog");
    assert.equal(await dialog.isVisible(), true);
    assert.match(await dialog.innerText(), /em breve/);
    await dialog.getByRole("button", { name: "Fechar acesso à conta", exact: true }).click();
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
