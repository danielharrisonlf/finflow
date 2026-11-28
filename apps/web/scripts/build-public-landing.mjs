import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { copyFile, cp, mkdir, readdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { build } from "vite";

// Publish the approved portable landing at / and FinFlow in the browser at /app/
// (docs/browser-runtime-contract.md §5). The app is the `pnpm build:browser` output.
const web = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const workspace = resolve(web, "../..");
const publicRepo = resolve(workspace, "../finflow-landing");
const stage = join(web, "test-results/vercel-landing");
const source = join(web, "src/components/landing");
const browserApp = join(web, "dist-browser");
for (const required of ["index.html", "sw.js", "manifest.webmanifest"]) {
  if (!existsSync(join(browserApp, required))) {
    throw new Error(
      `apps/web/dist-browser/${required} not found: run \`pnpm build:browser\` first`,
    );
  }
}
if (!existsSync(join(browserApp, "runtime"))) {
  throw new Error(
    "apps/web/dist-browser/runtime not found: `pnpm build:browser` must run build:wasm",
  );
}
const project = JSON.parse(await readFile(join(publicRepo, ".vercel/project.json"), "utf8"));
if (project.projectName !== "finflow-landing") throw new Error("Unexpected Vercel project");
await mkdir(join(stage, "src/components/landing"), { recursive: true });
await mkdir(join(stage, "public/fonts"), { recursive: true });
await mkdir(join(stage, ".vercel/output"), { recursive: true });

const sourceHashes = {};
for (const name of await readdir(source)) {
  await copyFile(join(source, name), join(stage, "src/components/landing", name));
  sourceHashes[name] = createHash("sha256")
    .update(await readFile(join(source, name)))
    .digest("hex");
}
for (const name of ["geist-latin.woff2", "OFL.txt"]) {
  await copyFile(join(web, "public/fonts", name), join(stage, "public/fonts", name));
}
for (const name of ["robots.txt", "sitemap.xml"]) {
  await copyFile(join(publicRepo, "public", name), join(stage, "public", name));
}
let html = await readFile(join(web, "index.html"), "utf8");
html = html
  .replace(/\s*<link\b(?=[^>]*(?:fonts\.googleapis\.com|fonts\.gstatic\.com))[^>]*>/g, "")
  // O PWA é do app instalado no computador do usuário; o site público não publica manifest nem ícones.
  .replace(/\s*<link\b(?=[^>]*rel="(?:manifest|apple-touch-icon)")[^>]*>/g, "")
  .replace(
    /\s*<meta\b(?=[^>]*name="(?:mobile-web-app-capable|apple-mobile-web-app-title)")[^>]*>/g,
    "",
  )
  .replace(
    "<title>FinFlow</title>",
    "<title>FinFlow — Seu dinheiro em ordem, grátis, no navegador ou no computador</title>",
  )
  .replace(
    "</head>",
    `<link rel="canonical" href="https://finflow-landing-umber.vercel.app/" />
    <meta property="og:url" content="https://finflow-landing-umber.vercel.app/" />
    <link rel="preload" href="/fonts/geist-latin.woff2" as="font" type="font/woff2" crossorigin />
  </head>`,
  );
await writeFile(join(stage, "index.html"), html);
await writeFile(
  join(stage, "package.json"),
  JSON.stringify({ name: "finflow-public-landing", private: true, type: "module" }, null, 2),
);
await writeFile(
  join(stage, "src/main.tsx"),
  `import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { LandingPage } from "./components/landing/LandingPage";
import "./base.css";
const root = document.getElementById("root");
if (!root) throw new Error("Missing root");
createRoot(root).render(<StrictMode><LandingPage /></StrictMode>);
`,
);
await writeFile(
  join(stage, "src/base.css"),
  `@import "./components/landing/theme.css";
*, *::before, *::after { box-sizing: border-box; border-width: 0; border-style: solid; }
html { line-height: 1.5; -webkit-text-size-adjust: 100%; font-family: system-ui, sans-serif; scroll-behavior: smooth; background: var(--ff-bg); color: var(--ff-text); }
body { margin: 0; padding-bottom: env(safe-area-inset-bottom); background: var(--ff-bg); }
button, input, select, textarea { margin: 0; padding: 0; font: inherit; color: inherit; }
button, select { text-transform: none; }
button { appearance: button; }
fieldset { padding: 0; margin: 0; min-width: 0; }
svg { display: block; vertical-align: middle; }
@media (prefers-reduced-motion: reduce) { html { scroll-behavior: auto; } }
`,
);
await writeFile(join(stage, ".vercel/project.json"), JSON.stringify(project));
await build({
  root: stage,
  configFile: false,
  plugins: [react()],
  build: { outDir: ".vercel/output/static", emptyOutDir: true, sourcemap: false },
});
// The browser app keeps its own manifest and service worker under /app/.
await cp(browserApp, join(stage, ".vercel/output/static/app"), { recursive: true });
const swVersion = /const VERSION = "([^"]+)"/.exec(
  await readFile(join(browserApp, "sw.js"), "utf8"),
)?.[1];
await writeFile(
  join(stage, ".vercel/output/config.json"),
  JSON.stringify(
    {
      version: 3,
      routes: [
        {
          src: "/(.*)",
          headers: {
            "X-Content-Type-Options": "nosniff",
            "Referrer-Policy": "strict-origin-when-cross-origin",
            "Permissions-Policy": "camera=(), microphone=(), geolocation=()",
          },
          continue: true,
        },
        {
          src: "/assets/(.*)",
          headers: { "Cache-Control": "public, max-age=31536000, immutable" },
          continue: true,
        },
        // FinFlow in the browser. index.html, sw.js and the SPA routes always revalidate, so a
        // deploy reaches open apps through the service worker update prompt.
        { src: "/app", status: 308, headers: { Location: "/app/" } },
        { src: "/app/(.*)", headers: { "Cache-Control": "no-cache" }, continue: true },
        {
          src: "/app/assets/(.*)",
          headers: { "Cache-Control": "public, max-age=31536000, immutable" },
          continue: true,
        },
        {
          // runtime files keep stable names (versioned by query string): revalidate
          src: "/app/runtime/(.*)",
          headers: { "Cache-Control": "public, max-age=0, must-revalidate" },
          continue: true,
        },
        {
          src: "/app/(.*)\\.wasm",
          headers: { "Content-Type": "application/wasm" },
          continue: true,
        },
        {
          src: "/app/manifest\\.webmanifest",
          headers: { "Content-Type": "application/manifest+json" },
          continue: true,
        },
        {
          src: "/app/sw\\.js",
          headers: { "Service-Worker-Allowed": "/app/" },
          continue: true,
        },
        { handle: "filesystem" },
        { src: "/inicio", dest: "/index.html" },
        // SPA fallback for the app routes (/app/extrato, /app/configuracoes…); missing files
        // under assets/runtime/icons/fonts stay 404 instead of returning HTML.
        { src: "/app/(?!assets/|runtime/|icons/|fonts/).*", dest: "/app/index.html" },
      ],
    },
    null,
    2,
  ),
);
await writeFile(
  join(stage, "manifest.json"),
  JSON.stringify(
    {
      createdAt: new Date().toISOString(),
      source: "apps/web/src/components/landing",
      sourceHashes,
      project: project.projectName,
      productionUrl: "https://finflow-landing-umber.vercel.app/",
      app: { path: "/app/", source: "apps/web/dist-browser", serviceWorker: swVersion ?? null },
    },
    null,
    2,
  ),
);
console.log(
  JSON.stringify({
    stage,
    project: project.projectName,
    files: Object.keys(sourceHashes),
    app: { path: "/app/", serviceWorker: swVersion ?? null },
  }),
);
