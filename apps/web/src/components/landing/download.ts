// Release assets keep stable names, so /releases/latest/download/<asset> always serves the newest build.
export const repoUrl = "https://github.com/danielharrisonlf/finflow";
const latest = `${repoUrl}/releases/latest/download`;
const rawScripts = "https://raw.githubusercontent.com/danielharrisonlf/finflow/main/scripts";

export type PlatformId =
  | "windows-amd64"
  | "windows-arm64"
  | "darwin-arm64"
  | "darwin-amd64"
  | "linux-amd64"
  | "linux-arm64";

export interface Platform {
  id: PlatformId;
  os: "windows" | "darwin" | "linux";
  label: string;
  detail: string;
  href: string;
}

const asset = (id: PlatformId) =>
  `${latest}/finflow-${id}.${id.startsWith("windows") ? "zip" : "tar.gz"}`;

export const platforms: Platform[] = [
  {
    id: "windows-amd64",
    os: "windows",
    label: "Windows",
    detail: "64 bits · .zip",
    href: asset("windows-amd64"),
  },
  {
    id: "darwin-arm64",
    os: "darwin",
    label: "macOS (Apple Silicon)",
    detail: "M1 ou mais novo · .tar.gz",
    href: asset("darwin-arm64"),
  },
  {
    id: "darwin-amd64",
    os: "darwin",
    label: "macOS (Intel)",
    detail: "Intel · .tar.gz",
    href: asset("darwin-amd64"),
  },
  {
    id: "linux-amd64",
    os: "linux",
    label: "Linux",
    detail: "x64 · .tar.gz",
    href: asset("linux-amd64"),
  },
  {
    id: "windows-arm64",
    os: "windows",
    label: "Windows (ARM)",
    detail: "ARM64 · .zip",
    href: asset("windows-arm64"),
  },
  {
    id: "linux-arm64",
    os: "linux",
    label: "Linux (ARM)",
    detail: "ARM64 · .tar.gz",
    href: asset("linux-arm64"),
  },
];

export const installCommands = {
  unix: `curl -fsSL ${rawScripts}/install.sh | sh`,
  windows: `irm ${rawScripts}/install.ps1 | iex`,
};

// Rodar a partir do código (docs/local-agent-contract.md §4): `pnpm start` ou Docker Compose.
export const sourceCommands = {
  clone: `git clone ${repoUrl} && cd finflow && pnpm install && pnpm start`,
  seed: "pnpm start -- --seed",
  docker: `git clone ${repoUrl} && cd finflow && docker compose up -d`,
  // Assistente opcional: a chave da API do Claude passa pelo ambiente (sem chave, fica desligado).
  dockerAssistant: "ANTHROPIC_API_KEY=sk-ant-... docker compose up -d",
};

export function platformById(id: PlatformId): Platform {
  return platforms.find((p) => p.id === id) ?? (platforms[0] as Platform);
}

interface UADataLike {
  platform?: string;
  getHighEntropyValues?: (hints: string[]) => Promise<{ architecture?: string }>;
}

export interface Detection {
  platform: PlatformId;
  mobile: boolean;
}

// Browsers cannot always tell Apple Silicon from Intel: Chromium exposes the architecture,
// otherwise Apple Silicon is the default and Intel stays one click away.
export async function detectPlatform(): Promise<Detection> {
  const ua = navigator.userAgent;
  const mobile = /Android|iPhone|iPad|iPod|Mobile/i.test(ua);
  const uaData = (navigator as Navigator & { userAgentData?: UADataLike }).userAgentData;
  const os: Platform["os"] = /Windows/i.test(ua)
    ? "windows"
    : /Mac OS X|Macintosh/i.test(ua) && !mobile
      ? "darwin"
      : /Linux|X11|CrOS/i.test(ua) && !/Android/i.test(ua)
        ? "linux"
        : "windows";

  let arch: "arm64" | "amd64" | null = /aarch64|arm64/i.test(ua) ? "arm64" : null;
  const uaPlatform = uaData?.platform?.toLowerCase() ?? "";
  const agrees =
    (os === "windows" && uaPlatform === "windows") ||
    (os === "darwin" && uaPlatform === "macos") ||
    (os === "linux" && uaPlatform === "linux");
  if (!arch && agrees && uaData?.getHighEntropyValues) {
    try {
      const values = await uaData.getHighEntropyValues(["architecture"]);
      if (values.architecture === "arm") arch = "arm64";
      if (values.architecture === "x86") arch = "amd64";
    } catch {
      // Keep the default for this operating system.
    }
  }
  const resolved = arch ?? (os === "darwin" ? "arm64" : "amd64");
  return { platform: `${os}-${resolved}` as PlatformId, mobile };
}
