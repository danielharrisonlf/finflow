import { defineConfig, devices } from "@playwright/test";

// Runtime do navegador sem a interface (docs/browser-runtime-contract.md §6): runtime-smoke.html no
// Vite dev carrega src/runtime/client.ts, que sobe o worker com o SQLite WASM (OPFS) e a API Go em
// WebAssembly. `pnpm test:e2e:runtime` compila o finflow.wasm antes (scripts/build-wasm.mjs).
export default defineConfig({
  testDir: "./e2e-runtime",
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  timeout: 120_000,
  expect: { timeout: 30_000 },
  reporter: "list",
  use: {
    baseURL: "http://localhost:5275",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "pnpm exec vite --port 5275 --strictPort",
    url: "http://localhost:5275/runtime-smoke.html",
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
  outputDir: "./test-results/runtime",
});
