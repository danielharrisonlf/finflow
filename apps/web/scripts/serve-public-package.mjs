import { existsSync, statSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { dirname, extname, join, normalize, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

// Serve the staged Vercel package locally, applying the same Build Output API routes from
// .vercel/output/config.json (headers, redirects, filesystem, rewrites), so verify-public-landing
// can check the landing and /app/ before a deploy:
//   node apps/web/scripts/build-public-landing.mjs
//   node apps/web/scripts/serve-public-package.mjs 4180
//   node apps/web/scripts/verify-public-landing.mjs http://localhost:4180/
const web = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const output = join(web, "test-results/vercel-landing/.vercel/output");
const staticDir = join(output, "static");
const port = Number(process.argv[2] ?? 4180);
const { routes } = JSON.parse(await readFile(join(output, "config.json"), "utf8"));

const types = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json",
  ".webmanifest": "application/manifest+json",
  ".wasm": "application/wasm",
  ".png": "image/png",
  ".svg": "image/svg+xml",
  ".ico": "image/x-icon",
  ".woff2": "font/woff2",
  ".txt": "text/plain; charset=utf-8",
  ".xml": "application/xml",
};

/** Arquivo estático para o caminho (diretório → index.html), ou null. */
function staticFile(pathname) {
  const target = normalize(join(staticDir, decodeURIComponent(pathname)));
  if (target !== staticDir && !target.startsWith(staticDir + sep)) return null;
  if (existsSync(target) && statSync(target).isFile()) return target;
  const index = join(target, "index.html");
  return existsSync(index) ? index : null;
}

async function send(res, file, headers, method) {
  const body = await readFile(file);
  res.writeHead(200, {
    "Content-Type": types[extname(file)] ?? "application/octet-stream",
    "Content-Length": body.length,
    ...headers,
  });
  res.end(method === "HEAD" ? undefined : body);
}

createServer(async (req, res) => {
  try {
    const url = new URL(req.url ?? "/", `http://localhost:${port}`);
    let pathname = url.pathname;
    const headers = {};
    let afterFilesystem = false;
    for (const route of routes) {
      if (route.handle === "filesystem") {
        const file = staticFile(pathname);
        if (file) return await send(res, file, headers, req.method);
        afterFilesystem = true;
        continue;
      }
      const match = new RegExp(`^${route.src}$`).exec(pathname);
      if (!match) continue;
      Object.assign(headers, route.headers ?? {});
      if (route.status && route.headers?.Location) {
        res.writeHead(route.status, headers);
        return res.end();
      }
      if (route.dest) {
        pathname = route.dest;
        if (afterFilesystem) {
          const file = staticFile(pathname);
          if (file) return await send(res, file, headers, req.method);
        }
      }
      if (!route.continue && !route.dest) break;
    }
    res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8", ...headers });
    res.end("404");
  } catch (error) {
    res.writeHead(500, { "Content-Type": "text/plain; charset=utf-8" });
    res.end(String(error));
  }
}).listen(port, () => {
  console.log(`public package on http://localhost:${port}/ (app at /app/)`);
});
