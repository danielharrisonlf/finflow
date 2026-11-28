// Compila a API para o navegador (docs/browser-runtime-contract.md §2):
//   GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w -X main.version=<v>" ./cmd/finflow-wasm
// e grava em apps/web/public/runtime/: finflow.wasm, o wasm_exec.js da mesma versão do Go e
// runtime.json ({ version, hash }), que o worker usa para pedir finflow.wasm?v=<hash>.
// Uso: node scripts/build-wasm.mjs [versão]   (padrão: FINFLOW_VERSION, tag do CI ou git describe)
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const outDir = path.join(root, "apps/web/public/runtime");
const wasmPath = path.join(outDir, "finflow.wasm");

function resolveVersion(explicit) {
  if (explicit) return explicit;
  if (process.env.FINFLOW_VERSION) return process.env.FINFLOW_VERSION;
  if (process.env.GITHUB_REF_TYPE === "tag" && process.env.GITHUB_REF_NAME) {
    return process.env.GITHUB_REF_NAME;
  }
  try {
    return execFileSync("git", ["describe", "--tags", "--always", "--dirty"], {
      cwd: root,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return `v${JSON.parse(readFileSync(path.join(root, "package.json"), "utf8")).version}`;
  }
}

const version = resolveVersion(process.argv[2]);
mkdirSync(outDir, { recursive: true });

execFileSync(
  "go",
  [
    "build",
    "-trimpath",
    "-ldflags",
    `-s -w -X main.version=${version}`,
    "-o",
    wasmPath,
    "./cmd/finflow-wasm",
  ],
  {
    cwd: root,
    stdio: "inherit",
    env: { ...process.env, GOOS: "js", GOARCH: "wasm", CGO_ENABLED: "0" },
  },
);

const goroot = execFileSync("go", ["env", "GOROOT"], { cwd: root, encoding: "utf8" }).trim();
copyFileSync(path.join(goroot, "lib/wasm/wasm_exec.js"), path.join(outDir, "wasm_exec.js"));

const wasm = readFileSync(wasmPath);
const hash = createHash("sha256").update(wasm).digest("hex").slice(0, 12);
writeFileSync(path.join(outDir, "runtime.json"), `${JSON.stringify({ version, hash }, null, 2)}\n`);

const mb = (n) => `${(n / 1024 / 1024).toFixed(1)} MB`;
const raw = statSync(wasmPath).size;
const gz = gzipSync(wasm, { level: 9 }).length;
console.log(
  `✔ finflow.wasm ${version} (${hash}): ${mb(raw)} (gzip ${mb(gz)}) em apps/web/public/runtime/`,
);
