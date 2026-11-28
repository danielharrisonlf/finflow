import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { readdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import { join, relative, resolve, sep } from "node:path";
import type { Plugin } from "vite";

// Plugins do build do FinFlow (docs/browser-runtime-contract.md §4).
//  - modo navegador (`pnpm build:browser`, base /app/): manifest e service worker próprios do /app/,
//    sem fontes externas no index.html (o app não faz requisição além dos próprios arquivos);
//  - modo servidor (`pnpm build:web`): o runtime do navegador (public/runtime) não entra no
//    binário, que continua com o mesmo conteúdo de antes.

const webRoot = resolve(import.meta.dirname, "..");

async function listFiles(dir: string): Promise<string[]> {
  const out: string[] = [];
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...(await listFiles(full)));
    else out.push(full);
  }
  return out;
}

const runtimeClient = join(webRoot, "src/runtime/client.ts");
const serverClient = join(webRoot, "browser/server-client.ts").split(sep).join("/");

/**
 * Modo servidor: `src/runtime/client` vira `browser/server-client.ts` (sem worker nem SQLite do
 * navegador, que o Vite emitiria como arquivos sem uso) e o runtime de public/runtime sai da
 * saída do build embutido no binário.
 */
export function serverBuildWithoutRuntime(): Plugin {
  let outDir = "";
  let isBuild = false;
  return {
    name: "finflow-server-without-runtime",
    enforce: "pre",
    // Só no build do binário: o servidor de dev precisa do cliente real para o harness do runtime
    // (e2e-runtime/), e no dev o worker nunca é iniciado fora do modo navegador.
    apply: "build",
    configResolved(config) {
      outDir = resolve(config.root, config.build.outDir);
      isBuild = config.command === "build";
    },
    async resolveId(source, importer, options) {
      if (!importer || !/runtime\/client(\.ts)?$/.test(source)) return null;
      const resolved = await this.resolve(source, importer, { ...options, skipSelf: true });
      return resolved && resolve(resolved.id) === runtimeClient ? serverClient : null;
    },
    async closeBundle() {
      if (!isBuild) return;
      const runtime = join(outDir, "runtime");
      if (existsSync(runtime)) await rm(runtime, { recursive: true, force: true });
    },
  };
}

export function browserPwa(): Plugin {
  let outDir = "";
  let base = "/app/";
  return {
    name: "finflow-browser-pwa",
    apply: "build",
    configResolved(config) {
      outDir = resolve(config.root, config.build.outDir);
      base = config.base;
    },
    transformIndexHtml(html) {
      return (
        html
          // sem Google Fonts: a fonte cai para a do sistema e nada sai do site
          .replace(/\s*<link\b(?=[^>]*(?:fonts\.googleapis\.com|fonts\.gstatic\.com))[^>]*>/g, "")
          .replace(
            /<meta name="description" content="[^"]*" \/>/,
            '<meta name="description" content="FinFlow no navegador: contas, cartões, metas e planilhas, com os dados guardados neste navegador." />',
          )
      );
    },
    async closeBundle() {
      // manifest do /app/ (o do binário continua em public/manifest.webmanifest)
      const manifest = JSON.parse(
        await readFile(join(webRoot, "public/manifest.webmanifest"), "utf8"),
      ) as { icons: { src: string }[] } & Record<string, unknown>;
      const browserManifest = {
        ...manifest,
        description:
          "Controle financeiro pessoal no navegador: contas, cartões, projeção e metas, com os dados guardados neste aparelho.",
        id: base,
        start_url: base,
        scope: base,
        icons: manifest.icons.map((icon) => ({ ...icon, src: `${base}${icon.src.slice(1)}` })),
      };
      await writeFile(
        join(outDir, "manifest.webmanifest"),
        `${JSON.stringify(browserManifest, null, 2)}\n`,
      );

      // service worker: lista tudo o que o build gerou (shell, assets com hash, runtime, ícones)
      const files = (await listFiles(outDir))
        .map((file) => relative(outDir, file).split(sep).join("/"))
        .filter((path) => path !== "sw.js" && !path.endsWith(".map"))
        .sort();
      const template = await readFile(join(webRoot, "browser/sw.js"), "utf8");
      // a versão muda com qualquer arquivo do app e com o próprio service worker
      const hash = createHash("sha256").update(template);
      let bytes = 0;
      for (const path of files) {
        const content = await readFile(join(outDir, path));
        bytes += (await stat(join(outDir, path))).size;
        hash.update(path).update("\0").update(content);
      }
      const version = hash.digest("hex").slice(0, 12);
      const sw = template
        .replace("__FINFLOW_VERSION__", version)
        .replace("__FINFLOW_BASE__", base)
        .replace("/* __FINFLOW_PRECACHE__ */ []", JSON.stringify(files));
      await writeFile(join(outDir, "sw.js"), sw);
      console.log(
        `\nfinflow-browser-pwa: sw.js ${version} · ${files.length} arquivos · ${(bytes / 1024 / 1024).toFixed(1)} MB em cache`,
      );
    },
  };
}
