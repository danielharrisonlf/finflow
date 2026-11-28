// Roda o FinFlow a partir do código-fonte (docs/local-agent-contract.md §4).
//
//   pnpm install && pnpm start                → compila o que mudou e abre o FinFlow (modo duplo clique)
//   pnpm start -- --seed                      → cria dados de exemplo (se o banco estiver vazio) e abre
//   pnpm start -- --port 4000 --lan           → outras flags vão para `finflow serve` (sem abrir o navegador)
//   pnpm start -- resumo                      → um comando vira `finflow <comando>` (CLI)
//
// - confere Go, Node e pnpm com mensagens em português e links de instalação;
// - roda `pnpm install` se faltar node_modules;
// - recompila o web (pnpm build:web) e o binário (go build) só quando os fontes forem mais novos;
// - o binário fica na raiz (./finflow ou finflow.exe), igual ao `pnpm build`.
//
// Variáveis úteis: FINFLOW_DB_PATH (arquivo do banco), API_PORT (porta inicial), FINFLOW_NO_BROWSER=1,
// FINFLOW_AI_* (assistente local). Força recompilar tudo com FINFLOW_REBUILD=1.
import { spawn, spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
const isWindows = process.platform === "win32";
const binary = path.join(root, isWindows ? "finflow.exe" : "finflow");
const MIN_NODE = [22, 12];

// ───────────────────────── pré-requisitos ─────────────────────────

function fail(lines) {
  console.error(`\n✖ ${lines.join("\n  ")}\n`);
  process.exit(1);
}

function parseVersion(text) {
  const m = /(\d+)\.(\d+)(?:\.(\d+))?/.exec(text ?? "");
  return m ? [Number(m[1]), Number(m[2]), Number(m[3] ?? 0)] : null;
}

function atLeast(have, want) {
  for (let i = 0; i < want.length; i++) {
    if ((have[i] ?? 0) !== want[i]) return (have[i] ?? 0) > want[i];
  }
  return true;
}

// No Windows, `go`/`pnpm` podem ser .cmd/.bat e exigem shell; o comando vai como uma string só
// (os argumentos aqui são fixos, sem espaços nem entrada do usuário).
function spawnArgs(cmd, args, shell) {
  return shell ? [[cmd, ...args].join(" "), []] : [cmd, args];
}

function capture(cmd, args) {
  const res = spawnSync(...spawnArgs(cmd, args, isWindows), {
    cwd: root,
    encoding: "utf8",
    shell: isWindows,
  });
  return res.status === 0 ? res.stdout.trim() : null;
}

function checkNode() {
  const have = parseVersion(process.versions.node);
  if (!atLeast(have, MIN_NODE)) {
    fail([
      `Node ${MIN_NODE.join(".")} ou mais novo é necessário (encontrado: ${process.versions.node}).`,
      "Instale a versão LTS em https://nodejs.org/pt/download e abra um terminal novo.",
    ]);
  }
}

// O go.mod define a versão mínima (ex.: `go 1.26.1`); exigimos a mesma série (1.26+).
function checkGo() {
  const goMod = readFileSync(path.join(root, "go.mod"), "utf8");
  const want = parseVersion(/^go\s+(\S+)/m.exec(goMod)?.[1]).slice(0, 2);
  const out = capture("go", ["version"]);
  if (!out) {
    fail([
      `Go ${want.join(".")} ou mais novo não foi encontrado (o comando \`go\` não está no PATH).`,
      "Instale em https://go.dev/dl/ e abra um terminal novo.",
      "Sem Go nem Node? Use o executável pronto ou o Docker (veja o README).",
    ]);
  }
  const have = parseVersion(/go(\d+\.\d+(?:\.\d+)?)/.exec(out)?.[1]);
  if (!have || !atLeast(have, want)) {
    fail([
      `Go ${want.join(".")} ou mais novo é necessário (encontrado: ${out}).`,
      "Atualize em https://go.dev/dl/ e abra um terminal novo.",
    ]);
  }
}

// Rodando via `pnpm start`, o pnpm já existe (npm_execpath aponta para ele).
function pnpmCommand() {
  const execPath = process.env.npm_execpath;
  if (execPath && /pnpm/i.test(path.basename(execPath)) && execPath.endsWith(".cjs")) {
    return { cmd: process.execPath, prefix: [execPath], shell: false };
  }
  if (capture("pnpm", ["--version"])) return { cmd: "pnpm", prefix: [], shell: isWindows };
  const pinned = JSON.parse(readFileSync(path.join(root, "package.json"), "utf8")).packageManager;
  fail([
    "pnpm não foi encontrado.",
    "Ative pelo Corepack, que vem com o Node:",
    "  corepack enable",
    `  corepack prepare ${pinned ?? "pnpm@latest"} --activate`,
    "Se o comando `corepack` não existir (Node 25+): npm install -g corepack",
    "Mais em https://pnpm.io/pt/installation",
  ]);
}

// ───────────────────────── execução ─────────────────────────

function run(cmd, args, { shell = false } = {}) {
  console.log(`\n› ${[path.basename(cmd), ...args].join(" ")}`);
  const res = spawnSync(...spawnArgs(cmd, args, shell), { cwd: root, stdio: "inherit", shell });
  if (res.status !== 0) {
    fail([`Falhou: ${[path.basename(cmd), ...args].join(" ")}`]);
  }
}

function pnpm(pm, args) {
  run(pm.cmd, [...pm.prefix, ...args], { shell: pm.shell });
}

// Pastas de saída/cache que não contam como fonte.
const IGNORED = new Set([
  "node_modules",
  "dist",
  "dist-browser",
  "test-results",
  "playwright-report",
  ".turbo",
]);
// Gerados por `pnpm build:wasm` (apps/web/public/runtime): não são fonte do web embutido.
const IGNORED_PATHS = [path.join(root, "apps", "web", "public", "runtime")];

// Maior mtime (ms) entre os arquivos dos caminhos dados; 0 se nenhum existir.
function newest(paths, { skip = IGNORED } = {}) {
  let max = 0;
  const walk = (p) => {
    if (IGNORED_PATHS.includes(p)) return;
    let st;
    try {
      st = statSync(p);
    } catch {
      return;
    }
    if (st.isDirectory()) {
      for (const entry of readdirSync(p)) {
        if (!skip.has(entry)) walk(path.join(p, entry));
      }
    } else if (st.mtimeMs > max) {
      max = st.mtimeMs;
    }
  };
  for (const p of paths) walk(path.join(root, p));
  return max;
}

function mtime(p) {
  try {
    return statSync(path.join(root, p)).mtimeMs;
  } catch {
    return 0;
  }
}

const force = Boolean(process.env.FINFLOW_REBUILD);

function webIsStale() {
  const built = mtime("internal/webui/dist/index.html");
  if (force || built === 0) return true;
  return (
    newest([
      "apps/web/src",
      "apps/web/public",
      "apps/web/index.html",
      "apps/web/package.json",
      "apps/web/vite.config.ts",
      "apps/web/browser",
      "packages/contracts/src",
      "packages/contracts/package.json",
      "pnpm-lock.yaml",
    ]) > built
  );
}

function binaryIsStale() {
  const built = mtime(path.basename(binary));
  if (force || built === 0) return true;
  // internal/ inclui o web embutido (internal/webui/dist), por isso não ignoramos "dist" aqui.
  const goSkip = new Set(["node_modules", "testdata"]);
  return newest(["cmd", "internal", "go.mod", "go.sum"], { skip: goSkip }) > built;
}

// pnpm start -- <args>: ver o mapeamento no topo do arquivo.
function binaryArgs(extra) {
  const args = extra[0] === "--" ? extra.slice(1) : extra;
  const seed = args.includes("--seed");
  const rest = args.filter((a) => a !== "--seed");
  if (rest.length === 0) return { seed, args: [] }; // modo duplo clique
  if (rest[0].startsWith("-")) return { seed: false, args: ["serve", ...args] };
  return { seed: false, args };
}

function launch(args) {
  console.log(`\n› ${[path.basename(binary), ...args].join(" ")}\n`);
  const child = spawn(binary, args, { cwd: root, stdio: "inherit" });
  // Ctrl+C chega ao processo filho pelo terminal; aqui só esperamos ele encerrar.
  for (const sig of ["SIGINT", "SIGTERM"]) {
    process.on(sig, () => {
      if (!isWindows) child.kill(sig);
    });
  }
  child.on("exit", (code, signal) => process.exit(code ?? (signal ? 130 : 0)));
  child.on("error", (err) => fail([`Não foi possível iniciar ${binary}: ${err.message}`]));
}

checkNode();
checkGo();
const pm = pnpmCommand();

if (!existsSync(path.join(root, "node_modules"))) {
  pnpm(pm, ["install", "--frozen-lockfile"]);
}

if (webIsStale()) {
  pnpm(pm, ["build:web"]);
} else {
  console.log("✔ Web já compilado (sem mudanças em apps/web ou packages/contracts)");
}

if (binaryIsStale()) {
  run(process.execPath, [path.join(root, "scripts", "build-go.mjs")]);
} else {
  console.log(`✔ Binário em dia: ${path.basename(binary)}`);
}

const { seed, args } = binaryArgs(process.argv.slice(2));
if (seed) {
  run(binary, ["seed"]);
}
launch(args);
