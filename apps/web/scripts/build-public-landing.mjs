import { createHash } from "node:crypto";
import { copyFile, mkdir, readdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { build } from "vite";

// Publish only the approved portable landing, independent of the private app.
const web = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const workspace = resolve(web, "../..");
const publicRepo = resolve(workspace, "../finflow-landing");
const stage = join(web, "test-results/vercel-landing");
const source = join(web, "src/components/landing");
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
  .replace("<title>FinFlow</title>", "<title>FinFlow — Seu dinheiro em ordem. Em breve.</title>")
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
        { handle: "filesystem" },
        { src: "/inicio", dest: "/index.html" },
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
    },
    null,
    2,
  ),
);
console.log(
  JSON.stringify({ stage, project: project.projectName, files: Object.keys(sourceHashes) }),
);
