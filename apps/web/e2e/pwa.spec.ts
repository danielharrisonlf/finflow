import { expect, test } from "@playwright/test";

test("manifest instalável com ícones 192/512 (inclusive maskable)", async ({ request }) => {
  const res = await request.get("/manifest.webmanifest");
  expect(res.ok()).toBe(true);
  const manifest = (await res.json()) as {
    display: string;
    start_url: string;
    icons: { src: string; sizes: string; purpose?: string; type: string }[];
  };
  expect(manifest.display).toBe("standalone");
  expect(manifest.start_url).toBe("/");
  for (const size of ["192x192", "512x512"]) {
    for (const purpose of ["any", "maskable"]) {
      const icon = manifest.icons.find((i) => i.sizes === size && i.purpose === purpose);
      expect(icon, `${size} ${purpose}`).toBeDefined();
      const png = await request.get(icon!.src);
      expect(png.ok()).toBe(true);
      const bytes = await png.body();
      expect(bytes.subarray(1, 4).toString("ascii")).toBe("PNG");
    }
  }
});

test("service worker nunca intercepta a API", async ({ request }) => {
  const res = await request.get("/sw.js");
  expect(res.ok()).toBe(true);
  const source = await res.text();
  expect(source).toContain('startsWith("/api/")');
  expect(source).toContain("skipWaiting");
});

test("index.html aponta para o manifest", async ({ request }) => {
  const html = await (await request.get("/")).text();
  expect(html).toContain('<link rel="manifest" href="/manifest.webmanifest" />');
  expect(html).toContain('rel="apple-touch-icon"');
});
