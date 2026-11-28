import { expect, type Page, test } from "@playwright/test";

// Runtime do navegador de ponta a ponta, sem a interface: worker + SQLite WASM (opfs-sahpool) + API Go
// em WebAssembly, chamados por src/runtime/client.ts (exposto como window.finflowRuntime).

type ApiResult = { status: number; text: string; headers: Record<string, string> };

async function open(page: Page) {
  await page.goto("/runtime-smoke.html");
  await expect(page.locator("#status")).toHaveText("pronto");
  return page.evaluate(() => window.finflowRuntime.startRuntime());
}

async function api(page: Page, method: string, path: string, body?: unknown): Promise<ApiResult> {
  return page.evaluate(
    async ({ method, path, body }) => {
      const res = await window.finflowRuntime.runtimeFetch(path, {
        method,
        headers: body === undefined ? undefined : { "Content-Type": "application/json" },
        // string = JSON já pronto (ex.: inteiros acima de 2^53, que JSON.stringify arredondaria)
        body:
          body === undefined ? undefined : typeof body === "string" ? body : JSON.stringify(body),
      });
      return {
        status: res.status,
        text: await res.text(),
        headers: Object.fromEntries(res.headers.entries()),
      };
    },
    { method, path, body },
  );
}

async function json<T>(page: Page, method: string, path: string, body?: unknown): Promise<T> {
  const res = await api(page, method, path, body);
  expect(res.status, `${method} ${path}: ${res.text}`).toBeLessThan(400);
  return JSON.parse(res.text) as T;
}

type Account = { id: string; name: string };
type TxPage = { items: { description: string; amountCents: number }[]; total: number };

test("seed, lançamento persistente, backup, apagar e restaurar", async ({ page }) => {
  const info = await open(page);
  expect(info.persistent).toBe(true);
  expect(info.spaces).toContain("pessoal");

  const health = await api(page, "GET", "/api/health");
  expect(health.status).toBe(200);
  expect(JSON.parse(health.text)).toEqual({ ok: true });

  const sys = await json<{ runtime: string; dataPath: string | null; lanUrls: string[] }>(
    page,
    "GET",
    "/api/system/info",
  );
  expect(sys).toMatchObject({ runtime: "browser", dataPath: null, lanUrls: [] });

  expect((await api(page, "POST", "/api/system/seed")).status).toBe(201);
  expect((await api(page, "POST", "/api/system/seed")).status).toBe(409);
  const seeded = await json<TxPage>(page, "GET", "/api/transactions");
  expect(seeded.items.length).toBeGreaterThan(0);

  const [account] = await json<Account[]>(page, "GET", "/api/accounts");
  const today = new Date().toISOString().slice(0, 10);
  const created = await api(page, "POST", "/api/transactions", {
    type: "EXPENSE",
    description: "Teste de persistência",
    amountCents: 12345,
    date: today,
    status: "CONFIRMED",
    accountId: account!.id,
  });
  expect(created.status, created.text).toBe(201);

  // INTEGER de 64 bits volta exato (2^53 + 1 não cabe num double).
  const big = await api(
    page,
    "POST",
    "/api/accounts",
    '{"name":"Conta grande","type":"CHECKING","initialBalanceCents":9007199254740993}',
  );
  expect(big.status, big.text).toBe(201);
  expect((await api(page, "GET", "/api/accounts")).text).toContain("9007199254740993");

  // Recarregar: os dados continuam no OPFS.
  await page.reload();
  await open(page);
  const search = "/api/transactions?search=persist%C3%AAncia";
  const afterReload = await json<TxPage>(page, "GET", search);
  expect(afterReload.items.map((t) => t.description)).toContain("Teste de persistência");

  // Backup → apagar tudo → restaurar.
  const backup = await page.evaluate(async () => {
    const blob = await window.finflowRuntime.exportDatabase("pessoal");
    const bytes = new Uint8Array(await blob.arrayBuffer());
    return { size: bytes.byteLength, magic: new TextDecoder().decode(bytes.slice(0, 15)) };
  });
  expect(backup.magic).toBe("SQLite format 3");
  expect(backup.size).toBeGreaterThan(4096);

  await page.evaluate(async () => {
    const blob = await window.finflowRuntime.exportDatabase("pessoal");
    (window as unknown as { backup: Blob }).backup = blob;
    await window.finflowRuntime.resetAllData();
  });
  expect(await json<Account[]>(page, "GET", "/api/accounts")).toEqual([]);

  // Arquivo inválido: recusado, e os dados atuais (vazios) ficam.
  const invalid = await page.evaluate(async () => {
    try {
      await window.finflowRuntime.importDatabase("pessoal", new Blob(["não é um banco"]));
      return "aceito";
    } catch (error) {
      return (error as Error).message;
    }
  });
  expect(invalid).toContain("SQLite");

  await page.evaluate(() =>
    window.finflowRuntime.importDatabase("pessoal", (window as unknown as { backup: Blob }).backup),
  );
  const restored = await json<TxPage>(page, "GET", search);
  expect(restored.items.map((t) => t.description)).toContain("Teste de persistência");
  expect((await api(page, "GET", "/api/accounts")).text).toContain("9007199254740993");
});

test("upload multipart e resposta em stream (SSE do assistente)", async ({ page }) => {
  // A API do Claude é simulada: o SDK dentro do wasm chama api.anthropic.com pelo fetch do worker.
  const sse = (event: string, data: unknown) =>
    `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
  const turn =
    sse("message_start", {
      type: "message_start",
      message: {
        id: "msg",
        type: "message",
        role: "assistant",
        model: "claude",
        content: [],
        stop_reason: null,
        usage: { input_tokens: 1, output_tokens: 1 },
      },
    }) +
    sse("content_block_start", {
      type: "content_block_start",
      index: 0,
      content_block: { type: "text", text: "" },
    }) +
    sse("content_block_delta", {
      type: "content_block_delta",
      index: 0,
      delta: { type: "text_delta", text: "Olá, " },
    }) +
    sse("content_block_delta", {
      type: "content_block_delta",
      index: 0,
      delta: { type: "text_delta", text: "tudo certo." },
    }) +
    sse("content_block_stop", { type: "content_block_stop", index: 0 }) +
    sse("message_delta", {
      type: "message_delta",
      delta: { stop_reason: "end_turn" },
      usage: { output_tokens: 5 },
    }) +
    sse("message_stop", { type: "message_stop" });
  const browserAccess: string[] = [];
  const cors = {
    "access-control-allow-origin": "*",
    "access-control-allow-headers": "*",
    "access-control-allow-methods": "POST, OPTIONS",
  };
  await page.context().route("https://api.anthropic.com/**", async (route) => {
    const req = route.request();
    if (req.method() === "OPTIONS") return route.fulfill({ status: 204, headers: cors });
    browserAccess.push((await req.allHeaders())["anthropic-dangerous-direct-browser-access"] ?? "");
    return route.fulfill({
      status: 200,
      headers: { ...cors, "content-type": "text/event-stream" },
      body: turn,
    });
  });

  await open(page);
  const account = await json<Account>(page, "POST", "/api/accounts", {
    name: "Conta do extrato",
    type: "CHECKING",
    initialBalanceCents: 0,
  });

  // FormData: new Request serializa com o boundary; o Go lê com ParseMultipartForm.
  const preview = await page.evaluate(async (accountId) => {
    const csv =
      "Data;Histórico;Valor\n01/04/2026;Mercado do bairro;-150,30\n02/04/2026;Pix recebido;1.200,00\n";
    const form = new FormData();
    form.append("file", new File([csv], "extrato.csv", { type: "text/csv" }));
    form.append("accountId", accountId);
    const res = await window.finflowRuntime.runtimeFetch("/api/import/preview", {
      method: "POST",
      body: form,
    });
    return { status: res.status, body: await res.json() };
  }, account.id);
  expect(preview.status).toBe(200);
  expect(preview.body.format).toBe("CSV");
  expect(preview.body.rows).toHaveLength(2);

  expect(
    (await api(page, "PUT", "/api/ai/key", { apiKey: "sk-ant-api03-abcdefghijklmnopqrstuvwxyz" }))
      .status,
  ).toBe(200);
  const stream = await page.evaluate(async () => {
    const res = await window.finflowRuntime.runtimeFetch("/api/ai/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ messages: [{ role: "user", content: "oi" }] }),
    });
    const reader = res.body!.getReader();
    const decoder = new TextDecoder();
    let chunks = 0;
    let text = "";
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      chunks++;
      text += decoder.decode(value, { stream: true });
    }
    return { status: res.status, type: res.headers.get("content-type"), chunks, text };
  });
  expect(stream.status).toBe(200);
  expect(stream.type).toContain("text/event-stream");
  expect(stream.text).toContain('event: text\ndata: {"delta":"Olá, "}');
  expect(stream.text).toContain("event: done");
  // Cada evento SSE é um Flush no Go → um pedaço separado no Response.
  expect(stream.chunks).toBeGreaterThan(2);
  expect(browserAccess).toEqual(["true"]);

  // Abortar pela signal cancela a requisição.
  const aborted = await page.evaluate(async () => {
    const ctrl = new AbortController();
    const p = window.finflowRuntime.runtimeFetch("/api/health", { signal: ctrl.signal });
    ctrl.abort();
    try {
      await p;
      return "concluída";
    } catch (error) {
      return (error as Error).name;
    }
  });
  expect(aborted).toBe("AbortError");
});

test("segunda aba fica bloqueada e assume com Usar aqui", async ({ page, context }) => {
  await open(page);
  expect((await api(page, "POST", "/api/system/seed")).status).toBe(201);
  await page.evaluate(() => {
    const w = window as unknown as { lost: boolean };
    w.lost = false;
    window.finflowRuntime.onRuntimeLost(() => {
      w.lost = true;
    });
  });

  const second = await context.newPage();
  await second.goto("/runtime-smoke.html");
  await expect(second.locator("#status")).toHaveText("pronto");
  await second.evaluate(() => {
    const w = window as unknown as { blocked: boolean; started: Promise<unknown> };
    w.blocked = false;
    window.finflowRuntime.onRuntimeBlocked(() => {
      w.blocked = true;
    });
    w.started = window.finflowRuntime.startRuntime();
  });
  await expect
    .poll(() => second.evaluate(() => (window as unknown as { blocked: boolean }).blocked))
    .toBe(true);

  await second.evaluate(() => window.finflowRuntime.takeOver());
  await second.evaluate(() => (window as unknown as { started: Promise<unknown> }).started);
  await expect
    .poll(() => page.evaluate(() => (window as unknown as { lost: boolean }).lost))
    .toBe(true);
  const accounts = await json<Account[]>(second, "GET", "/api/accounts");
  expect(accounts.length).toBeGreaterThan(0);
});
