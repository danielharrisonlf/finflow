#!/usr/bin/env node
// Sugere a próxima versão do FinFlow a partir dos Conventional Commits desde a última tag estável,
// seguindo a política de ../SKILL.md (SemVer com a convenção pré-1.0 de Cargo/release-please).
// Uso (na raiz do repositório): node .claude/skills/finflow-release/scripts/suggest-version.mjs [--fetch] [--from X.Y.Z] [--to <ref>]
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join, resolve } from "node:path";

const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const option = (name) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
};
const root = resolve(option("--root") ?? process.cwd());
const to = option("--to") ?? "HEAD";

// Arquivos cuja mudança pode quebrar compatibilidade (SKILL.md §2) ou o contrato de distribuição (§5).
const SENSITIVE = [
  {
    pattern: /^internal\/db\/(schema|db|browser|sqlitejs)\.go$|^internal\/store\/defaults\.go$/,
    reason: "banco de dados",
  },
  { pattern: /^internal\/httpapi\/(server|handlers|[a-z_]+)\.go$/, reason: "rotas da API REST" },
  { pattern: /^internal\/mcp\/.+\.go$/, reason: "tools do MCP" },
  {
    pattern: /^internal\/cli\/cli\.go$|^internal\/cli\/[a-z_]+\.go$/,
    reason: "comandos e flags da CLI",
  },
  {
    pattern: /^internal\/config\/config\.go$|^\.env\.example$|^internal\/service\/settings\.go$/,
    reason: "configuração e dados salvos",
  },
  {
    pattern:
      /^apps\/web\/src\/runtime\/|^internal\/spaces\/(sqlcatalog|files_browser)\.go$|^apps\/web\/src\/components\/browser\/browser-data-section\.tsx$/,
    reason: "armazenamento do navegador / backups",
  },
  {
    pattern:
      /^scripts\/(release\.mjs|install\.sh|install\.ps1)$|^\.github\/workflows\/release\.yml$|^docker-compose\.yaml$|^Dockerfile$|^apps\/web\/scripts\/build-public-landing\.mjs$/,
    reason: "contrato de distribuição",
  },
];

const git = (...rest) => execFileSync("git", ["-C", root, ...rest], { encoding: "utf8" }).trim();
const compare = (a, b) => a[0] - b[0] || a[1] - b[1] || a[2] - b[2];

function classify(subject, body) {
  if (/^Merge (pull request|branch)/.test(subject) || /^chore\(release\)/.test(subject))
    return "ignored";
  const header = subject.match(/^(\w+)(\([^)]*\))?(!)?:/);
  if (header?.[3] || /^BREAKING[ -]CHANGE:/m.test(body)) return "breaking";
  const type = header?.[1]?.toLowerCase();
  if (type === "feat") return "feat";
  if (["fix", "perf", "revert"].includes(type)) return "fix";
  if (!type) return "unknown";
  return "none";
}

function next([major, minor, patch], effect) {
  if (effect === "breaking") return major === 0 ? [0, minor + 1, 0] : [major + 1, 0, 0];
  if (effect === "feat") return major === 0 ? [0, minor, patch + 1] : [major, minor + 1, 0];
  if (effect === "fix") return [major, minor, patch + 1];
  return null;
}

if (flag("--fetch")) git("fetch", "-q", "--tags", "origin");

const tags = git("tag", "--list")
  .split("\n")
  .map((name) => ({ name, match: name.match(/^v(\d+)\.(\d+)\.(\d+)$/) }))
  .filter((t) => t.match)
  .map((t) => ({ name: t.name, version: t.match.slice(1, 4).map(Number) }))
  .sort((a, b) => compare(b.version, a.version));
const from = option("--from");
const last = from ? tags.find((t) => t.version.join(".") === from) : tags[0];
if (!last) {
  console.log(from ? `tag da versão ${from} não encontrada` : "nenhuma tag de release encontrada");
  process.exit(1);
}

const versions = Object.fromEntries(
  ["package.json", "apps/web/package.json", "packages/contracts/package.json"].map((file) => [
    file,
    JSON.parse(readFileSync(join(root, file), "utf8")).version,
  ]),
);
const distinct = [...new Set(Object.values(versions))];
console.log(
  `última release: ${last.name}   versão nos arquivos: ${distinct.length === 1 ? distinct[0] : JSON.stringify(versions)}`,
);
if (distinct.length > 1) console.log("ATENÇÃO: os package.json divergem entre si.");

const raw = git("log", `${last.name}..${to}`, "--no-merges", "--format=%h%x1f%s%x1f%b%x1e");
const commits = raw
  .split("\x1e")
  .map((s) => s.trim())
  .filter(Boolean)
  .map((entry) => {
    const [hash, subject, body = ""] = entry.split("\x1f");
    return { hash, subject, effect: classify(subject, body) };
  });
const groups = { breaking: [], feat: [], fix: [], none: [], unknown: [] };
for (const c of commits) if (c.effect !== "ignored") groups[c.effect].push(c);
const titles = {
  breaking: "Incompatíveis",
  feat: "Funcionalidades",
  fix: "Correções",
  none: "Sem efeito de release",
  unknown: "Fora do padrão Conventional Commits (classifique à mão)",
};
for (const [key, list] of Object.entries(groups)) {
  if (!list.length) continue;
  console.log(`${titles[key]}:`);
  for (const c of list) console.log(`  ${c.hash} ${c.subject}`);
}

const changed = commits.length
  ? git("diff", "--name-only", `${last.name}..${to}`).split("\n").filter(Boolean)
  : [];
const warnings = new Map();
for (const file of changed)
  for (const s of SENSITIVE)
    if (s.pattern.test(file) && !/_test\.go$/.test(file)) warnings.set(file, s.reason);
if (warnings.size) {
  console.log("Revise o §2 da skill: arquivos sensíveis mudaram");
  for (const [file, reason] of warnings) console.log(`  ${file}  (${reason})`);
}

const effect = groups.breaking.length
  ? "breaking"
  : groups.feat.length
    ? "feat"
    : groups.fix.length
      ? "fix"
      : null;
const target = effect && next(last.version, effect);
if (!target) {
  console.log(
    `Sugestão: nada a lançar${groups.unknown.length ? " (confira os commits fora do padrão)" : ""}.`,
  );
  process.exit(0);
}
const pre = last.version[0] === 0;
const kind =
  effect === "breaking"
    ? pre
      ? "MINOR (incompatível, pré-1.0)"
      : "MAJOR (incompatível)"
    : effect === "feat"
      ? pre
        ? "PATCH (funcionalidade compatível, pré-1.0)"
        : "MINOR (funcionalidade compatível)"
      : "PATCH (correção)";
console.log(
  `Sugestão: ${last.version.join(".")} → ${target.join(".")}  [${kind}]   tag v${target.join(".")}`,
);
