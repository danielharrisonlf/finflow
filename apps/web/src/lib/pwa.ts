import { isBrowserRuntime } from "../runtime/client";
import { showToast } from "./toast";

/**
 * Service worker (PWA). Modo servidor (v3 §14): `/sw.js`, só o shell estático, como antes.
 * Modo navegador (docs/browser-runtime-contract.md §4): `/app/sw.js` guarda o shell e o runtime
 * num cache por versão; quando há versão nova, avisa "Nova versão disponível" e recarrega só
 * quando a pessoa pedir.
 */
export function registerServiceWorker(): void {
  if (!import.meta.env.PROD || !("serviceWorker" in navigator)) return;
  window.addEventListener("load", () => {
    if (!isBrowserRuntime) {
      navigator.serviceWorker.register("/sw.js").catch(() => {
        // sem service worker o app segue funcionando normalmente
      });
      return;
    }
    registerBrowserServiceWorker();
  });
}

function registerBrowserServiceWorker(): void {
  const base = import.meta.env.BASE_URL;
  let reloadRequested = false;
  // a primeira instalação também troca o controlador (clients.claim): só recarrega se pedido
  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (reloadRequested) window.location.reload();
  });

  const announce = (worker: ServiceWorker) => {
    showToast({
      tone: "info",
      message: "Nova versão disponível",
      action: {
        label: "Recarregar",
        onClick: () => {
          reloadRequested = true;
          worker.postMessage("SKIP_WAITING");
        },
      },
    });
  };

  navigator.serviceWorker
    .register(`${base}sw.js`, { scope: base })
    .then((registration) => {
      if (registration.waiting && navigator.serviceWorker.controller) {
        announce(registration.waiting);
      }
      registration.addEventListener("updatefound", () => {
        const worker = registration.installing;
        worker?.addEventListener("statechange", () => {
          if (worker.state === "installed" && navigator.serviceWorker.controller) announce(worker);
        });
      });
      // app aberto por muito tempo (instalado): procura versão nova de hora em hora
      window.setInterval(() => void registration.update().catch(() => {}), 60 * 60 * 1000);
    })
    .catch(() => {
      // sem service worker o app funciona, só não abre offline
    });
}
