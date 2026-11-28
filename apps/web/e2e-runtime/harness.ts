// Expõe o cliente do runtime para o Playwright (runtime-smoke.html, servido pelo Vite dev).
import * as runtime from "../src/runtime/client";

declare global {
  interface Window {
    finflowRuntime: typeof runtime;
  }
}

window.finflowRuntime = runtime;
document.getElementById("status")!.textContent = "pronto";
