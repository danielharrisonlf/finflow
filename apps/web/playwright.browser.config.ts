import { defineConfig, devices } from "@playwright/test";

// FinFlow no navegador (docs/browser-runtime-contract.md §6): o build `dist-browser` servido
// localmente em /app/, com o runtime de verdade (Go wasm + SQLite no OPFS). Rode antes
// `pnpm build:browser`. Os cenários dividem o mesmo navegador e banco, então rodam em série.
export default defineConfig({
  testDir: "./e2e-browser",
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  timeout: 180_000,
  expect: { timeout: 30_000 },
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: "http://localhost:5274",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "pnpm preview:browser",
    url: "http://localhost:5274/app/",
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
  outputDir: "./test-results/browser-runtime",
});
