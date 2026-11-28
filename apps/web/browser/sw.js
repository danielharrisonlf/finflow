// Service worker do FinFlow no navegador (docs/browser-runtime-contract.md §4).
// Gerado no `pnpm build:browser` a partir de apps/web/browser/sw.js: o plugin browser-pwa troca
// a versão e a lista de arquivos. Guarda o shell e o runtime (/app/runtime/*) num cache por
// versão para o app abrir offline. Não há dados aqui: o banco fica no OPFS, fora deste cache.
const VERSION = "__FINFLOW_VERSION__";
const BASE = "__FINFLOW_BASE__";
const PRECACHE = /* __FINFLOW_PRECACHE__ */ [];
const CACHE = `finflow-app-${VERSION}`;
const INDEX = `${BASE}index.html`;

self.addEventListener("install", (event) => {
  // Não chama skipWaiting aqui: a página mostra "Nova versão disponível — recarregar" e só
  // então pede a troca (mensagem SKIP_WAITING), para não misturar versões numa aba aberta.
  event.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(PRECACHE.map((path) => `${BASE}${path}`))),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter((key) => key.startsWith("finflow-app-") && key !== CACHE)
            .map((key) => caches.delete(key)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("message", (event) => {
  if (event.data === "SKIP_WAITING") self.skipWaiting();
});

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin || !url.pathname.startsWith(BASE)) return;

  // navegação dentro do app: o shell desta versão (rotas do React Router)
  if (request.mode === "navigate") {
    event.respondWith(
      caches
        .match(INDEX, { cacheName: CACHE, ignoreVary: true })
        .then((cached) => cached || fetch(request))
        .catch(() => Response.error()),
    );
    return;
  }

  // arquivos do app: cache desta versão primeiro. O runtime pode vir com ?v=, então ignora a
  // query ali; ignoreVary porque o precache foi feito sem o cabeçalho Origin que os scripts de
  // módulo (crossorigin) enviam, e um `Vary: Origin` do servidor faria o cache nunca bater.
  event.respondWith(
    caches
      .open(CACHE)
      .then((cache) =>
        cache.match(request.url, {
          ignoreVary: true,
          ignoreSearch: url.pathname.startsWith(`${BASE}runtime/`),
        }),
      )
      .then((cached) => cached || fetch(request)),
  );
});
