import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { browserPwa, serverBuildWithoutRuntime } from "./browser/vite-plugin.ts";

// Dois builds (docs/browser-runtime-contract.md §4):
//  - servidor (padrão): embutido no binário (`pnpm build:web` → internal/webui), API em /api;
//  - navegador (`vite build --mode browser` ou VITE_FINFLOW_RUNTIME=browser): app em /app/, a API
//    roda num worker na própria página; saída em dist-browser/ para o pacote público.
export default defineConfig(({ mode }) => {
  const browser = mode === "browser" || process.env.VITE_FINFLOW_RUNTIME === "browser";
  if (browser) process.env.VITE_FINFLOW_RUNTIME = "browser";
  const port = browser ? 5274 : 5273;
  return {
    base: browser ? "/app/" : "/",
    plugins: [react(), tailwindcss(), browser ? browserPwa() : serverBuildWithoutRuntime()],
    build: browser ? { outDir: "dist-browser", emptyOutDir: true } : undefined,
    // Runtime do navegador (src/runtime/worker.ts): worker de módulo (ES) e o @sqlite.org/sqlite-wasm
    // fora do pre-bundle do dev, para o sqlite3.wasm ser resolvido via import.meta.url (README do pacote).
    worker: { format: "es" as const },
    optimizeDeps: { exclude: ["@sqlite.org/sqlite-wasm"] },
    server: {
      port,
      strictPort: true,
    },
    preview: {
      port,
      strictPort: true,
    },
  };
});
