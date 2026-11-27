// Gera os binários de release do FinFlow para 6 plataformas em dist/release/.
//
//   node scripts/release.mjs [versão] [--skip-web]
//
// - compila o web uma vez (pnpm build:web → internal/webui/dist, embutido via go:embed);
// - cross-compila com CGO_ENABLED=0 (SQLite é pure Go) e -trimpath -ldflags "-s -w -X main.version=<v>";
// - empacota cada binário com um LEIA-ME.txt em finflow-<os>-<arch>.zip (Windows) ou .tar.gz;
// - escreve checksums.txt (sha256), no formato do `sha256sum -c`.
//
// Os nomes dos arquivos não levam versão, para que
// https://github.com/danielharrisonlf/finflow/releases/latest/download/finflow-windows-amd64.zip
// sempre aponte para a última release. Os arquivos são escritos em Node puro (sem zip/tar externos).
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { crc32, deflateRawSync, gzipSync } from "node:zlib";

const targets = [
  ["windows", "amd64"],
  ["windows", "arm64"],
  ["darwin", "amd64"],
  ["darwin", "arm64"],
  ["linux", "amd64"],
  ["linux", "arm64"],
];

const root = path.resolve(import.meta.dirname, "..");
const outDir = path.join(root, "dist", "release");
const args = process.argv.slice(2);
const skipWeb = args.includes("--skip-web");
const version = resolveVersion(args.find((a) => !a.startsWith("--")));

function run(cmd, cmdArgs, env = {}) {
  try {
    execFileSync(cmd, cmdArgs, {
      cwd: root,
      stdio: "inherit",
      env: { ...process.env, ...env },
      shell: process.platform === "win32" && cmd === "pnpm",
    });
  } catch {
    console.error(`✖ falhou: ${cmd} ${cmdArgs.join(" ")}`);
    process.exit(1);
  }
}

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

// ───────────────────────── LEIA-ME ─────────────────────────

const dataDir = {
  windows: "%AppData%\\finflow\\finflow.db",
  darwin: "~/Library/Application Support/finflow/finflow.db",
  linux: "~/.config/finflow/finflow.db",
};

function readme(os) {
  const bin = os === "windows" ? "finflow.exe" : "./finflow";
  const start =
    os === "windows"
      ? [
          "1. Extraia este .zip numa pasta (por exemplo, Documentos\\FinFlow).",
          "2. Dê dois cliques em finflow.exe. Uma janela preta abre e o navegador",
          "   vai para http://localhost:3333.",
          '   Se o Windows mostrar "O Windows protegeu o computador", clique em',
          '   "Mais informações" > "Executar assim mesmo" (o binário não é assinado).',
        ]
      : [
          "1. Extraia o arquivo e abra um terminal na pasta.",
          os === "darwin"
            ? "2. Libere o binário baixado (não é assinado pela Apple):\n     xattr -d com.apple.quarantine finflow"
            : "2. Garanta a permissão de execução:\n     chmod +x finflow",
          "3. Rode ./finflow — o servidor sobe e o navegador abre http://localhost:3333.",
        ];
  const lines = [
    `FinFlow ${version} — controle financeiro pessoal, gratuito e de código aberto`,
    "",
    "COMO USAR",
    ...start,
    "",
    "Quer começar com dados de exemplo para conhecer o app?",
    `     ${bin} serve --seed`,
    "e abra http://localhost:3333 no navegador.",
    "",
    "ONDE FICAM OS SEUS DADOS",
    "Num arquivo SQLite no seu computador. Nada vai para a nuvem:",
    `     ${dataDir[os]}`,
    "Para fazer backup, copie esse arquivo com o FinFlow fechado.",
    "Outro local: defina a variável FINFLOW_DB_PATH (caminho do arquivo .db).",
    "",
    "COMO PARAR",
    os === "windows"
      ? "Feche a janela preta do FinFlow ou pressione Ctrl+C dentro dela."
      : "Pressione Ctrl+C no terminal onde o FinFlow está rodando.",
    "",
    "OUTROS COMANDOS",
    `     ${bin} --help        lista todos os comandos (saldo, resumo, extrato...)`,
    `     ${bin} serve --port 4000   usa outra porta`,
    `     ${bin} version       mostra a versão instalada`,
    "",
    "Código-fonte, novidades e ajuda: https://github.com/danielharrisonlf/finflow",
    "",
  ];
  const text = lines.join("\n");
  return os === "windows" ? text.replaceAll("\n", "\r\n") : text;
}

// ───────────────────────── arquivos (zip / tar.gz) ─────────────────────────

function dosDateTime(date) {
  const time = (date.getHours() << 11) | (date.getMinutes() << 5) | (date.getSeconds() >> 1);
  const day = ((date.getFullYear() - 1980) << 9) | ((date.getMonth() + 1) << 5) | date.getDate();
  return { time, day };
}

function zip(entries) {
  const { time, day } = dosDateTime(new Date());
  const locals = [];
  const centrals = [];
  let offset = 0;
  for (const { name, data, mode } of entries) {
    const nameBuf = Buffer.from(name, "utf8");
    const compressed = deflateRawSync(data, { level: 9 });
    const crc = crc32(data);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4); // versão necessária
    local.writeUInt16LE(0x0800, 6); // nomes em UTF-8
    local.writeUInt16LE(8, 8); // deflate
    local.writeUInt16LE(time, 10);
    local.writeUInt16LE(day, 12);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(compressed.length, 18);
    local.writeUInt32LE(data.length, 22);
    local.writeUInt16LE(nameBuf.length, 26);
    local.writeUInt16LE(0, 28);
    locals.push(local, nameBuf, compressed);

    const central = Buffer.alloc(46);
    central.writeUInt32LE(0x02014b50, 0);
    central.writeUInt16LE((3 << 8) | 20, 4); // criado em "Unix" para preservar o modo
    central.writeUInt16LE(20, 6);
    central.writeUInt16LE(0x0800, 8);
    central.writeUInt16LE(8, 10);
    central.writeUInt16LE(time, 12);
    central.writeUInt16LE(day, 14);
    central.writeUInt32LE(crc, 16);
    central.writeUInt32LE(compressed.length, 20);
    central.writeUInt32LE(data.length, 24);
    central.writeUInt16LE(nameBuf.length, 28);
    central.writeUInt32LE(((0o100000 | mode) << 16) >>> 0, 38);
    central.writeUInt32LE(offset, 42);
    centrals.push(central, nameBuf);
    offset += local.length + nameBuf.length + compressed.length;
  }
  const centralBuf = Buffer.concat(centrals);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(entries.length, 8);
  end.writeUInt16LE(entries.length, 10);
  end.writeUInt32LE(centralBuf.length, 12);
  end.writeUInt32LE(offset, 16);
  return Buffer.concat([...locals, centralBuf, end]);
}

function tarGz(entries) {
  const mtime = Math.floor(Date.now() / 1000);
  const blocks = [];
  for (const { name, data, mode } of entries) {
    const header = Buffer.alloc(512);
    const field = (value, offset, length) => header.write(value, offset, length, "utf8");
    const octal = (value, offset, length) =>
      field(`${value.toString(8).padStart(length - 1, "0")}\0`, offset, length);
    field(name, 0, 100);
    octal(mode, 100, 8);
    octal(0, 108, 8); // uid
    octal(0, 116, 8); // gid
    octal(data.length, 124, 12);
    octal(mtime, 136, 12);
    header.fill(" ", 148, 156); // checksum é calculado com o campo em branco
    field("0", 156, 1); // arquivo regular
    field("ustar\0", 257, 6);
    field("00", 263, 2);
    let sum = 0;
    for (const byte of header) sum += byte;
    field(`${sum.toString(8).padStart(6, "0")}\0 `, 148, 8);
    blocks.push(header, data, Buffer.alloc((512 - (data.length % 512)) % 512));
  }
  blocks.push(Buffer.alloc(1024));
  return gzipSync(Buffer.concat(blocks), { level: 9 });
}

// ───────────────────────── build ─────────────────────────

console.log(`▶ FinFlow ${version}: release para ${targets.length} plataformas`);
if (skipWeb) {
  if (!existsSync(path.join(root, "internal/webui/dist/index.html"))) {
    console.error("--skip-web exige internal/webui/dist/index.html (rode `pnpm build:web`).");
    process.exit(1);
  }
} else {
  run("pnpm", ["build:web"]);
}

rmSync(outDir, { recursive: true, force: true });
mkdirSync(outDir, { recursive: true });
const buildDir = path.join(outDir, ".build");
mkdirSync(buildDir, { recursive: true });

const checksums = [];
for (const [os, arch] of targets) {
  const binName = os === "windows" ? "finflow.exe" : "finflow";
  const binPath = path.join(buildDir, `${os}-${arch}`, binName);
  console.log(`▶ go build ${os}/${arch}`);
  run(
    "go",
    [
      "build",
      "-trimpath",
      "-ldflags",
      `-s -w -X main.version=${version}`,
      "-o",
      binPath,
      "./cmd/finflow",
    ],
    { CGO_ENABLED: "0", GOOS: os, GOARCH: arch },
  );
  const entries = [
    { name: binName, data: readFileSync(binPath), mode: 0o755 },
    { name: "LEIA-ME.txt", data: Buffer.from(readme(os), "utf8"), mode: 0o644 },
  ];
  const archiveName = `finflow-${os}-${arch}.${os === "windows" ? "zip" : "tar.gz"}`;
  const archive = os === "windows" ? zip(entries) : tarGz(entries);
  writeFileSync(path.join(outDir, archiveName), archive);
  checksums.push(`${createHash("sha256").update(archive).digest("hex")}  ${archiveName}`);
  console.log(`✔ ${archiveName} (${(archive.length / 1024 / 1024).toFixed(1)} MB)`);
}
rmSync(buildDir, { recursive: true, force: true });
writeFileSync(path.join(outDir, "checksums.txt"), `${checksums.join("\n")}\n`);
console.log(`✔ checksums.txt — artefatos em ${path.relative(root, outDir)}${path.sep}`);
