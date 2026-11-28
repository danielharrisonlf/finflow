import type { CreateSpaceInput, Space } from "@finflow/contracts";
import { useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  DatabaseBackup,
  Download,
  HardDrive,
  Laptop,
  ShieldCheck,
  Trash2,
  Upload,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, saveBlob } from "../../lib/api";
import { askPersistentStorage, resetBrowserFlags, useRuntimeInfo } from "../../lib/browser-runtime";
import { useSpaces } from "../../lib/queries";
import { DEFAULT_SPACE_SLUG, resetToDefaultSpace, useActiveSpaceSlug } from "../../lib/space";
import { showToast } from "../../lib/toast";
import { createZip, readZip } from "../../lib/zip";
import { exportDatabase, importDatabase, resetAllData } from "../../runtime/client";
import { Card } from "../ui/card";
import { Input, Select } from "../ui/field";
import { Badge } from "../ui/segmented";

// Configurações → "Dados neste navegador" (docs/browser-runtime-contract.md §4). Só no modo
// navegador: os dados moram no OPFS deste navegador, então backup e restauração ficam aqui.

const today = () => new Date().toISOString().slice(0, 10);

/** Catálogo gravado junto do backup .zip: sem ele, Casa e Empresa não voltam depois de "Apagar tudo". */
const SPACES_MANIFEST = "spaces.json";
type ManifestSpace = Pick<Space, "slug" | "name" | "kind" | "color">;

function readManifest(entries: { name: string; data: Uint8Array }[]): Map<string, ManifestSpace> {
  const entry = entries.find((e) => e.name.replace(/^.*\//, "") === SPACES_MANIFEST);
  const out = new Map<string, ManifestSpace>();
  if (!entry) return out;
  try {
    const parsed = JSON.parse(new TextDecoder().decode(entry.data)) as { spaces?: ManifestSpace[] };
    for (const s of parsed.spaces ?? []) {
      if (s && typeof s.slug === "string") out.set(s.slug, s);
    }
  } catch {
    // manifest corrompido: restaura só os espaços que já existem
  }
  return out;
}

type Os = "windows" | "darwin" | "linux";

function detectOs(): Os {
  const platform =
    typeof navigator === "undefined" ? "" : `${navigator.platform} ${navigator.userAgent}`;
  if (/Mac|iPhone|iPad/i.test(platform)) return "darwin";
  if (/Linux|X11|Android/i.test(platform)) return "linux";
  return "windows";
}

const INSTALLED_PATHS: { os: Os; label: string; path: string }[] = [
  { os: "windows", label: "Windows", path: "%AppData%\\finflow\\finflow.db" },
  { os: "darwin", label: "macOS", path: "~/Library/Application Support/finflow/finflow.db" },
  { os: "linux", label: "Linux", path: "~/.config/finflow/finflow.db" },
];

const actionCls =
  "inline-flex min-h-11 shrink-0 cursor-pointer items-center justify-center gap-2 rounded-(--radius-control) border border-app-border px-3 text-sm text-app-text transition-colors hover:border-app-muted hover:bg-app-surface disabled:cursor-wait disabled:opacity-60";

function SubHeading({ icon: Icon, children }: { icon: typeof HardDrive; children: string }) {
  return (
    <h3 className="flex items-center gap-2 text-sm font-semibold text-app-text">
      <Icon size={16} aria-hidden className="text-app-muted" />
      {children}
    </h3>
  );
}

function errorText(error: unknown): string {
  return error instanceof Error && error.message ? error.message : "erro desconhecido";
}

function useStorageStatus() {
  const [persisted, setPersisted] = useState<boolean | null>(null);
  const [usage, setUsage] = useState<number | null>(null);
  const refresh = useCallback(async () => {
    try {
      setPersisted((await navigator.storage?.persisted?.()) ?? false);
      const estimate = await navigator.storage?.estimate?.();
      setUsage(estimate?.usage ?? null);
    } catch {
      setPersisted(false);
    }
  }, []);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  return { persisted, usage, refresh };
}

export function BrowserDataSection() {
  const info = useRuntimeInfo();
  const spaces = useSpaces();
  const activeSlug = useActiveSpaceSlug();
  const queryClient = useQueryClient();
  const storage = useStorageStatus();
  const [backupSpace, setBackupSpace] = useState(activeSlug);
  const [busy, setBusy] = useState<string | null>(null);
  const [confirmText, setConfirmText] = useState("");
  const restoreInput = useRef<HTMLInputElement>(null);
  const installedInput = useRef<HTMLInputElement>(null);
  const os = detectOs();
  const list = spaces.data ?? [];
  const spaceName = (slug: string) => list.find((s) => s.slug === slug)?.name ?? slug;

  const run = async (key: string, action: () => Promise<void>) => {
    setBusy(key);
    try {
      await action();
    } finally {
      setBusy(null);
    }
  };

  const backupOne = () =>
    run("backup", async () => {
      try {
        const blob = await exportDatabase(backupSpace);
        saveBlob(blob, `finflow-${backupSpace}-${today()}.db`);
      } catch (e) {
        showToast({ tone: "error", message: `Backup falhou: ${errorText(e)}`, durationMs: 8000 });
      }
    });

  const backupAll = () =>
    run("backup-all", async () => {
      try {
        const slugs = list.length > 0 ? list.map((s) => s.slug) : [DEFAULT_SPACE_SLUG];
        const entries = [];
        for (const slug of slugs) {
          const blob = await exportDatabase(slug);
          entries.push({ name: `${slug}.db`, data: new Uint8Array(await blob.arrayBuffer()) });
        }
        const manifest: { version: 1; spaces: ManifestSpace[] } = {
          version: 1,
          spaces: list.map(({ slug, name, kind, color }) => ({ slug, name, kind, color })),
        };
        entries.push({
          name: SPACES_MANIFEST,
          data: new TextEncoder().encode(JSON.stringify(manifest, null, 2)),
        });
        saveBlob(createZip(entries), `finflow-backup-${today()}.zip`);
      } catch (e) {
        showToast({ tone: "error", message: `Backup falhou: ${errorText(e)}`, durationMs: 8000 });
      }
    });

  const afterRestore = async (message: string) => {
    await queryClient.invalidateQueries();
    showToast({ tone: "success", message, durationMs: 6000 });
  };

  const restore = (file: File, target: string) =>
    run("restore", async () => {
      try {
        if (file.name.toLowerCase().endsWith(".zip")) {
          const all = readZip(new Uint8Array(await file.arrayBuffer()));
          const manifest = readManifest(all);
          const entries = all.filter((e) => e.name.toLowerCase().endsWith(".db"));
          if (entries.length === 0) throw new Error("o .zip não tem nenhum arquivo .db");
          const names = entries.map((e) => e.name.replace(/\.db$/i, ""));
          if (
            !window.confirm(
              `Restaurar ${names.join(", ")}? Os dados atuais desses espaços serão substituídos.`,
            )
          ) {
            return;
          }
          const existing = new Set(
            (await api.get<Space[]>("/spaces")).map((sp) => sp.slug).concat(DEFAULT_SPACE_SLUG),
          );
          for (const entry of entries) {
            const slug = entry.name.replace(/^.*\//, "").replace(/\.db$/i, "");
            if (!existing.has(slug)) {
              // O runtime só importa para espaços do catálogo: recria o espaço antes.
              const meta = manifest.get(slug);
              const input: CreateSpaceInput = {
                slug,
                name: meta?.name ?? slug,
                kind: meta?.kind ?? "PERSONAL",
                ...(meta?.color ? { color: meta.color } : {}),
              };
              await api.post<Space>("/spaces", input);
              existing.add(slug);
            }
            await importDatabase(slug, new Blob([entry.data]));
          }
          await queryClient.invalidateQueries({ queryKey: ["spaces"] });
          await afterRestore("Backup restaurado.");
          return;
        }
        if (
          !window.confirm(
            `Restaurar "${file.name}" no espaço ${spaceName(target)}? Os dados atuais desse espaço serão substituídos.`,
          )
        ) {
          return;
        }
        await importDatabase(target, file);
        await afterRestore("Backup restaurado.");
      } catch (e) {
        showToast({
          tone: "error",
          message: `Não foi possível restaurar: ${errorText(e)}`,
          durationMs: 9000,
        });
      }
    });

  const eraseAll = () =>
    run("erase", async () => {
      try {
        await resetAllData();
        resetBrowserFlags();
        resetToDefaultSpace();
        try {
          for (const key of Object.keys(window.localStorage)) {
            if (key.startsWith("finflow_assistant_history")) window.localStorage.removeItem(key);
          }
        } catch {
          // storage indisponível
        }
        // volta ao início do app, que abre a primeira vez de novo
        window.location.assign(import.meta.env.BASE_URL);
      } catch (e) {
        showToast({ tone: "error", message: `Não foi possível apagar: ${errorText(e)}` });
      }
    });

  return (
    <Card
      className="grid gap-4 p-5 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] md:gap-8"
      data-testid="browser-data"
    >
      <div>
        <h2 className="text-sm font-semibold text-app-text">Dados neste navegador</h2>
        <p className="mt-1 text-sm text-app-muted">
          O FinFlow guarda tudo neste navegador, neste aparelho. Nada vai para um servidor. Faça
          backups para não perder nada e para levar os dados a outro aparelho ou ao app instalado.
        </p>
      </div>

      <div className="flex min-w-0 flex-col gap-6">
        <section className="flex flex-col gap-2" data-testid="browser-storage">
          <SubHeading icon={ShieldCheck}>Armazenamento</SubHeading>
          <div className="flex flex-wrap items-center gap-2">
            {info && !info.persistent ? (
              <Badge tone="expense">Temporário: os dados somem ao fechar</Badge>
            ) : storage.persisted ? (
              <Badge tone="income">Persistente</Badge>
            ) : (
              <Badge tone="warn">Pode ser apagado pelo navegador</Badge>
            )}
            {storage.usage !== null ? (
              <span className="tnum text-sm text-app-muted">
                {(storage.usage / 1024 / 1024).toFixed(1).replace(".", ",")} MB em uso
              </span>
            ) : null}
          </div>
          <p className="text-xs text-app-muted">
            {storage.persisted
              ? "O navegador só apaga estes dados se você pedir."
              : "Sem armazenamento persistente, o navegador pode apagar os dados quando faltar espaço. Instalar o FinFlow como app costuma liberar."}
          </p>
          {!storage.persisted && info?.persistent !== false ? (
            <button
              type="button"
              className={`${actionCls} self-start`}
              disabled={busy !== null}
              onClick={() =>
                void run("persist", async () => {
                  await askPersistentStorage();
                  await storage.refresh();
                })
              }
            >
              <ShieldCheck size={15} aria-hidden /> Pedir armazenamento persistente
            </button>
          ) : null}
        </section>

        <section className="flex flex-col gap-2">
          <SubHeading icon={Download}>Fazer backup</SubHeading>
          <p className="text-xs text-app-muted">
            Um arquivo .db por espaço (abre também no app instalado) ou um .zip com todos.
          </p>
          <div className="flex flex-wrap items-center gap-2">
            {list.length > 1 ? (
              <Select
                aria-label="Espaço do backup"
                value={backupSpace}
                onChange={(e) => setBackupSpace(e.target.value)}
                className="w-auto min-w-40"
              >
                {list.map((s) => (
                  <option key={s.slug} value={s.slug}>
                    {s.name}
                  </option>
                ))}
              </Select>
            ) : null}
            <button
              type="button"
              className={actionCls}
              disabled={busy !== null}
              onClick={() => void backupOne()}
            >
              <Download size={15} aria-hidden />
              {busy === "backup" ? "Gerando…" : "Fazer backup (.db)"}
            </button>
            <button
              type="button"
              className={actionCls}
              disabled={busy !== null}
              onClick={() => void backupAll()}
            >
              <DatabaseBackup size={15} aria-hidden />
              {busy === "backup-all" ? "Gerando…" : "Todos os espaços (.zip)"}
            </button>
          </div>
        </section>

        <section className="flex flex-col gap-2">
          <SubHeading icon={Upload}>Restaurar backup</SubHeading>
          <p className="text-xs text-app-muted">
            Um .db substitui os dados do espaço ativo ({spaceName(activeSlug)}); o .zip de todos os
            espaços restaura cada um no seu lugar.
          </p>
          <button
            type="button"
            className={`${actionCls} self-start`}
            disabled={busy !== null}
            onClick={() => restoreInput.current?.click()}
          >
            <Upload size={15} aria-hidden />
            {busy === "restore" ? "Restaurando…" : "Restaurar backup"}
          </button>
          <input
            ref={restoreInput}
            type="file"
            accept=".db,.sqlite,.zip,application/zip,application/x-sqlite3,application/vnd.sqlite3"
            className="sr-only"
            tabIndex={-1}
            aria-label="Arquivo de backup"
            data-testid="restore-input"
            onChange={(e) => {
              const file = e.target.files?.[0];
              e.target.value = "";
              if (file) void restore(file, activeSlug);
            }}
          />
        </section>

        <section className="flex flex-col gap-2" data-testid="installed-import">
          <SubHeading icon={Laptop}>Trazer dados do app instalado</SubHeading>
          <p className="text-xs text-app-muted">
            No computador com o FinFlow instalado, feche o programa e escolha o arquivo finflow.db.
            Ele substitui os dados do espaço Pessoal deste navegador.
          </p>
          <ul className="flex flex-col gap-1">
            {INSTALLED_PATHS.map((item) => (
              <li
                key={item.os}
                className={`flex flex-wrap items-baseline gap-x-2 text-sm ${item.os === os ? "text-app-text" : "text-app-muted"}`}
              >
                <span className="min-w-16 font-medium">{item.label}</span>
                <code className="text-xs break-all">{item.path}</code>
              </li>
            ))}
          </ul>
          <button
            type="button"
            className={`${actionCls} self-start`}
            disabled={busy !== null}
            onClick={() => installedInput.current?.click()}
          >
            <Laptop size={15} aria-hidden /> Escolher o finflow.db
          </button>
          <input
            ref={installedInput}
            type="file"
            accept=".db,.sqlite,application/x-sqlite3,application/vnd.sqlite3"
            className="sr-only"
            tabIndex={-1}
            aria-label="Arquivo finflow.db do app instalado"
            onChange={(e) => {
              const file = e.target.files?.[0];
              e.target.value = "";
              if (file) void restore(file, DEFAULT_SPACE_SLUG);
            }}
          />
        </section>

        <section className="flex flex-col gap-2" data-testid="erase-all">
          <SubHeading icon={Trash2}>Apagar todos os dados</SubHeading>
          <p className="text-xs text-app-muted">
            Remove todos os espaços deste navegador. Não dá para desfazer: faça um backup antes.
            Para confirmar, digite <strong className="text-app-text">apagar</strong>.
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              aria-label="Digite apagar para confirmar"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              autoComplete="off"
              className="w-40"
            />
            <button
              type="button"
              disabled={confirmText.trim().toLowerCase() !== "apagar" || busy !== null}
              onClick={() => void eraseAll()}
              className="inline-flex min-h-11 cursor-pointer items-center gap-2 rounded-(--radius-control) border border-rose-500/40 px-3 text-sm font-medium text-expense transition-colors hover:bg-rose-500/10 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Trash2 size={15} aria-hidden />
              {busy === "erase" ? "Apagando…" : "Apagar todos os dados"}
            </button>
          </div>
        </section>

        <p className="flex gap-2 rounded-(--radius-control) border border-amber-500/30 bg-amber-500/5 px-3.5 py-2.5 text-xs text-app-muted">
          <AlertTriangle size={15} aria-hidden className="mt-0.5 shrink-0 text-warn" />
          <span>
            Limpar os dados de sites do navegador (cookies e dados de sites, ou “limpar dados de
            navegação”) apaga o FinFlow deste aparelho. Mantenha um backup recente.
          </span>
        </p>
      </div>
    </Card>
  );
}
