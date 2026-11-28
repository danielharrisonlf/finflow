import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { type BrowserContext, expect, type Page, test } from "@playwright/test";

// FinFlow no navegador de ponta a ponta, contra o build `dist-browser` com o runtime real
// (docs/browser-runtime-contract.md §6). Um único navegador e banco percorrem o caminho
// completo, por isso os cenários são em série e compartilham contexto e aba.

const DESCRIPTION = "Café do teste no navegador";

test.describe.configure({ mode: "serial" });

let context: BrowserContext;
let page: Page;
let backupPath = "";

test.beforeAll(async ({ browser }) => {
  context = await browser.newContext({ acceptDownloads: true });
  page = await context.newPage();
});

test.afterAll(async () => {
  await context?.close();
});

async function expectDashboardNumbers(target: Page) {
  const balance = target.getByTestId("hero-balance");
  await expect(balance).toBeVisible({ timeout: 90_000 });
  await expect(balance).toContainText(/R\$\s*[\d.]+,\d{2}/);
}

async function openTransactions(target: Page, search = DESCRIPTION) {
  await target.goto("/app/extrato");
  await expect(target.getByTestId("first-run")).toHaveCount(0);
  await target.getByPlaceholder("Buscar descrição…").fill(search);
  return target.getByTestId("transactions-list");
}

test("abertura: splash, primeira vez e dados de exemplo", async () => {
  await page.goto("/app/", { waitUntil: "commit" });
  await expect(page.getByTestId("runtime-splash")).toBeVisible();
  await expect(page.getByText("Carregando o FinFlow…")).toBeVisible();

  const firstRun = page.getByTestId("first-run");
  await expect(firstRun).toBeVisible({ timeout: 90_000 });
  await expect(firstRun.getByRole("button", { name: /Começar do zero/ })).toBeVisible();
  await expect(
    firstRun.getByRole("button", { name: /Restaurar um backup \(\.db\)/ }),
  ).toBeVisible();
  await firstRun.getByRole("button", { name: /Explorar com dados de exemplo/ }).click();

  await expectDashboardNumbers(page);
  await expect(page).toHaveURL(/\/app\/$/);
});

test("lança uma despesa e ela continua lá depois de recarregar", async () => {
  await page.getByRole("button", { name: "Nova transação" }).click();
  const form = page.getByTestId("form-expense");
  await expect(form).toBeVisible();
  await form.getByPlaceholder("0,00").fill("12,34");
  await form.getByPlaceholder("Mercado, almoço, uber…").fill(DESCRIPTION);
  for (const select of await form.locator("select").all()) {
    if ((await select.inputValue()) === "") await select.selectOption({ index: 1 });
  }
  await form.getByRole("button", { name: "Adicionar despesa" }).click();
  await expect(form).not.toBeVisible();

  await page.reload();
  await expectDashboardNumbers(page);
  const list = await openTransactions(page);
  await expect(list.getByText(DESCRIPTION)).toBeVisible();
});

test("exportar Excel baixa um .xlsx gerado no navegador", async () => {
  await openTransactions(page, "");
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("link", { name: /Excel \(\.xlsx\)/ }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toMatch(/\.xlsx$/);
  const path = await download.path();
  const bytes = await readFile(path);
  expect(bytes.subarray(0, 2).toString("latin1")).toBe("PK");
});

test("backup, apagar tudo e restaurar devolve os dados", async () => {
  await page.goto("/app/configuracoes");
  const section = page.getByTestId("browser-data");
  await expect(section).toBeVisible();
  await expect(section.getByTestId("browser-storage")).toBeVisible();
  await expect(section.getByText(/%AppData%\\finflow\\finflow\.db/)).toBeVisible();
  // itens do programa instalado não aparecem no navegador
  await expect(page.getByText("Celular na rede local")).toHaveCount(0);
  await expect(page.getByText("MCP no computador (stdio)")).toHaveCount(0);
  await expect(page.getByTestId("local-data")).toHaveCount(0);

  const downloadPromise = page.waitForEvent("download");
  await section.getByRole("button", { name: "Fazer backup (.db)" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toMatch(/^finflow-pessoal-\d{4}-\d{2}-\d{2}\.db$/);
  backupPath = test.info().outputPath("backup.db");
  await download.saveAs(backupPath);
  const header = (await readFile(backupPath)).subarray(0, 15).toString("latin1");
  expect(header).toBe("SQLite format 3");

  const erase = section.getByRole("button", { name: "Apagar todos os dados" });
  await expect(erase).toBeDisabled();
  await section.getByLabel("Digite apagar para confirmar").fill("apagar");
  await erase.click();

  const firstRun = page.getByTestId("first-run");
  await expect(firstRun).toBeVisible({ timeout: 90_000 });
  await firstRun.getByTestId("first-run-restore-input").setInputFiles(backupPath);
  await expectDashboardNumbers(page);
  const list = await openTransactions(page);
  await expect(list.getByText(DESCRIPTION)).toBeVisible();
});

test("backup .zip de todos os espaços recria o espaço Casa depois de apagar tudo", async () => {
  // Cria um espaço Casa (o app passa a usá-lo) e faz o backup de todos.
  await page.goto("/app/espacos");
  await page.getByRole("button", { name: "Novo espaço" }).click();
  await page.getByPlaceholder("Casa, Minha empresa…").fill("Casa");
  await page.getByRole("button", { name: "Criar espaço" }).click();
  // Espera o card do espaço novo: recarregar antes disso interromperia a criação no worker.
  await expect(page.getByRole("dialog")).toHaveCount(0, { timeout: 30_000 });
  await expect(page.getByRole("button", { name: /Abrir o espaço/ })).toBeVisible({
    timeout: 30_000,
  });

  await page.goto("/app/configuracoes");
  const section = page.getByTestId("browser-data");
  const downloadPromise = page.waitForEvent("download");
  await section.getByRole("button", { name: /Todos os espaços \(\.zip\)/ }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toMatch(/^finflow-backup-\d{4}-\d{2}-\d{2}\.zip$/);
  const zipPath = test.info().outputPath("backup.zip");
  await download.saveAs(zipPath);
  const zipText = (await readFile(zipPath)).toString("latin1");
  expect(zipText).toContain("spaces.json");
  expect(zipText).toContain("casa.db");

  await section.getByLabel("Digite apagar para confirmar").fill("apagar");
  await section.getByRole("button", { name: "Apagar todos os dados" }).click();
  const firstRun = page.getByTestId("first-run");
  await expect(firstRun).toBeVisible({ timeout: 90_000 });
  await firstRun.getByRole("button", { name: /Começar do zero/ }).click();

  await page.goto("/app/configuracoes");
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByTestId("restore-input").setInputFiles(zipPath);
  await expect(page.getByText("Backup restaurado.")).toBeVisible({ timeout: 30_000 });

  await page.goto("/app/espacos");
  await expect(page.getByText("Casa", { exact: true }).first()).toBeVisible();
  const list = await openTransactions(page);
  await expect(list.getByText(DESCRIPTION)).toBeVisible();
});

test("segunda aba mostra que o FinFlow está aberto em outra aba", async () => {
  const second = await context.newPage();
  await second.goto("/app/");
  const blocked = second.getByTestId("runtime-blocked");
  await expect(blocked).toBeVisible({ timeout: 60_000 });
  await expect(
    blocked.getByRole("heading", { name: "O FinFlow está aberto em outra aba" }),
  ).toBeVisible();
  await blocked.getByRole("button", { name: "Usar aqui" }).click();
  await expectDashboardNumbers(second);
  await second.close();
  // a primeira aba perdeu o banco para a segunda; fechada a segunda, ela abre de novo
  await expect(page.getByTestId("runtime-blocked")).toBeVisible();
  await page.goto("/app/");
  await expectDashboardNumbers(page);
});

test("recarregar offline depois do primeiro carregamento funciona", async () => {
  // espera o service worker instalar o cache e assumir a página
  await page.evaluate(async () => {
    await navigator.serviceWorker.ready;
  });
  if (!(await page.evaluate(() => Boolean(navigator.serviceWorker.controller)))) {
    await page.reload();
    await expectDashboardNumbers(page);
  }
  await context.setOffline(true);
  try {
    await page.reload();
    await expectDashboardNumbers(page);
    const list = await openTransactions(page);
    await expect(list.getByText(DESCRIPTION)).toBeVisible();
  } finally {
    await context.setOffline(false);
  }
});

test("nova versão do service worker mostra o aviso e recarrega quando pedido", async () => {
  await page.goto("/app/");
  await expectDashboardNumbers(page);
  // simula um deploy novo: o preview serve o sw.js do disco, então troca a versão nele
  const swFile = fileURLToPath(new URL("../dist-browser/sw.js", import.meta.url));
  const original = await readFile(swFile, "utf8");
  await writeFile(
    swFile,
    original.replace(/const VERSION = "[^"]+"/, 'const VERSION = "e2e-next"'),
  );
  try {
    await page.evaluate(async () => {
      const registration = await navigator.serviceWorker.getRegistration();
      await registration?.update();
    });
    const toast = page.getByRole("status").filter({ hasText: "Nova versão disponível" });
    await expect(toast).toBeVisible();
    await toast.getByRole("button", { name: "Recarregar" }).click();
    await page.waitForEvent("load");
    await expectDashboardNumbers(page);
    expect(await page.evaluate(() => caches.keys())).toEqual(["finflow-app-e2e-next"]);
  } finally {
    await writeFile(swFile, original);
  }
});
