import { useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  DatabaseBackup,
  FilePlus2,
  MonitorSmartphone,
  Sparkles,
} from "lucide-react";
import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { api } from "../../lib/api";
import {
  askPersistentStorage,
  isOnboarded,
  markOnboarded,
  setRuntimeInfo,
} from "../../lib/browser-runtime";
import { cn } from "../../lib/cn";
import { DEFAULT_SPACE_SLUG } from "../../lib/space";
import { showToast } from "../../lib/toast";
import {
  importDatabase,
  onRuntimeBlocked,
  onRuntimeLost,
  startRuntime,
  takeOver,
} from "../../runtime/client";
import { Button } from "../ui/button";

// Abertura do FinFlow no navegador (docs/browser-runtime-contract.md §3–4): carrega o runtime,
// trata "aberto em outra aba" e, num banco vazio, oferece começar do zero, o exemplo ou um backup.

type GateState =
  | { step: "loading" }
  | { step: "blocked" }
  | { step: "error"; message: string }
  | { step: "first-run" }
  | { step: "ready" };

function BrandMark({ size = 44 }: { size?: number }) {
  return (
    <span
      className="flex items-center justify-center rounded-2xl bg-emerald-500/15"
      style={{ width: size, height: size }}
    >
      <svg width={size / 2} height={size / 2} viewBox="0 0 32 32" fill="none" aria-hidden="true">
        <path d="M9 22V10h12M9 16h9" stroke="#34d399" strokeWidth="3.5" strokeLinecap="round" />
      </svg>
    </span>
  );
}

function Screen({ children, testId }: { children: ReactNode; testId: string }) {
  return (
    <main
      className="flex min-h-dvh items-center justify-center bg-app-background px-4 py-10 text-app-text"
      data-testid={testId}
    >
      <div className="flex w-full max-w-md flex-col items-center gap-5 text-center">{children}</div>
    </main>
  );
}

function Splash() {
  return (
    <Screen testId="runtime-splash">
      <BrandMark />
      <div>
        <p className="text-lg font-semibold tracking-tight">FinFlow</p>
        <p className="mt-1 text-sm text-app-muted" role="status">
          Carregando o FinFlow…
        </p>
      </div>
      <div
        className="h-1 w-48 overflow-hidden rounded-full bg-app-soft"
        role="progressbar"
        aria-label="Carregando o FinFlow"
      >
        <div className="h-full w-1/3 animate-pulse rounded-full bg-emerald-500" />
      </div>
    </Screen>
  );
}

function ErrorScreen({ message }: { message: string }) {
  return (
    <Screen testId="runtime-error">
      <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-rose-500/10 text-expense">
        <AlertTriangle size={22} aria-hidden />
      </span>
      <div>
        <h1 className="text-lg font-semibold tracking-tight">Não foi possível abrir o FinFlow</h1>
        <p className="mt-2 text-sm text-app-muted">{message}</p>
      </div>
      <Button className="px-6" onClick={() => window.location.reload()}>
        Tentar de novo
      </Button>
      <p className="text-xs text-app-muted">
        O FinFlow guarda os dados no próprio navegador. Em janela anônima ou privada isso costuma
        ser bloqueado: abra numa janela normal. Use também um navegador atualizado (Chrome, Edge,
        Firefox ou Safari recentes) com os dados de sites permitidos.
      </p>
    </Screen>
  );
}

function BlockedScreen({ onUseHere }: { onUseHere: () => void }) {
  return (
    <Screen testId="runtime-blocked">
      <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-app-soft text-app-muted">
        <MonitorSmartphone size={22} aria-hidden />
      </span>
      <div>
        <h1 className="text-lg font-semibold tracking-tight">O FinFlow está aberto em outra aba</h1>
        <p className="mt-2 text-sm text-app-muted">
          Para proteger seus dados, o FinFlow funciona em uma aba por vez. Use a outra aba ou traga
          o FinFlow para esta.
        </p>
      </div>
      <Button className="px-6" onClick={onUseHere}>
        Usar aqui
      </Button>
    </Screen>
  );
}

function Choice({
  icon: Icon,
  title,
  text,
  onClick,
  disabled,
  primary,
}: {
  icon: typeof Sparkles;
  title: string;
  text: string;
  onClick: () => void;
  disabled?: boolean;
  primary?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "flex w-full cursor-pointer items-start gap-3 rounded-(--radius-card) border p-4 text-left transition-colors disabled:cursor-wait disabled:opacity-60",
        primary
          ? "border-emerald-500/50 bg-emerald-500/10 hover:bg-emerald-500/15"
          : "border-app-border bg-app-surface/60 hover:border-app-muted",
      )}
    >
      <Icon size={20} aria-hidden className="mt-0.5 shrink-0 text-income" />
      <span>
        <span className="block text-sm font-semibold text-app-text">{title}</span>
        <span className="mt-0.5 block text-sm text-app-muted">{text}</span>
      </span>
    </button>
  );
}

function FirstRun({ onDone }: { onDone: () => void }) {
  const [busy, setBusy] = useState<null | "seed" | "restore">(null);
  const [error, setError] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  const finish = (realData: boolean) => {
    markOnboarded();
    if (realData) void askPersistentStorage();
    onDone();
  };

  const seed = async () => {
    setBusy("seed");
    setError(null);
    try {
      await api.post("/system/seed");
      finish(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível criar os dados de exemplo.");
      setBusy(null);
    }
  };

  const restore = async (file: File) => {
    setBusy("restore");
    setError(null);
    try {
      await importDatabase(DEFAULT_SPACE_SLUG, file);
      showToast({ tone: "success", message: "Backup restaurado.", durationMs: 5000 });
      finish(true);
    } catch (e) {
      setError(
        e instanceof Error
          ? `Não foi possível restaurar esse arquivo: ${e.message}`
          : "Não foi possível restaurar esse arquivo.",
      );
      setBusy(null);
    }
  };

  return (
    <Screen testId="first-run">
      <BrandMark />
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Bem-vindo ao FinFlow</h1>
        <p className="mt-2 text-sm text-app-muted">
          Seus dados ficam guardados neste navegador, neste aparelho. Nada vai para um servidor.
        </p>
      </div>
      <div className="flex w-full flex-col gap-3">
        <Choice
          icon={FilePlus2}
          title="Começar do zero"
          text="Cadastre suas contas e lançamentos do seu jeito."
          onClick={() => finish(false)}
          disabled={busy !== null}
        />
        <Choice
          icon={Sparkles}
          title="Explorar com dados de exemplo"
          text={
            busy === "seed"
              ? "Criando contas, cartões e lançamentos de exemplo…"
              : "Três meses de contas, cartões e metas para conhecer o app."
          }
          onClick={() => void seed()}
          disabled={busy !== null}
          primary
        />
        <Choice
          icon={DatabaseBackup}
          title="Restaurar um backup (.db)"
          text={
            busy === "restore"
              ? "Restaurando o backup…"
              : "Um backup do FinFlow no navegador ou o finflow.db do app instalado."
          }
          onClick={() => fileInput.current?.click()}
          disabled={busy !== null}
        />
        <input
          ref={fileInput}
          type="file"
          accept=".db,.sqlite,application/x-sqlite3,application/vnd.sqlite3"
          className="sr-only"
          tabIndex={-1}
          aria-label="Arquivo de backup (.db)"
          data-testid="first-run-restore-input"
          onChange={(e) => {
            const file = e.target.files?.[0];
            e.target.value = "";
            if (file) void restore(file);
          }}
        />
      </div>
      {error ? (
        <p role="alert" className="text-sm text-expense">
          {error}
        </p>
      ) : null}
    </Screen>
  );
}

/** Banco vazio = nenhuma conta (o mesmo critério do `POST /api/system/seed`). */
async function needsFirstRun(): Promise<boolean> {
  if (isOnboarded()) return false;
  try {
    const accounts = await api.get<unknown[]>("/accounts");
    if (Array.isArray(accounts) && accounts.length > 0) {
      markOnboarded();
      return false;
    }
    return true;
  } catch {
    return false;
  }
}

export function RuntimeGate({ children }: { children: ReactNode }) {
  const [state, setState] = useState<GateState>({ step: "loading" });
  const queryClient = useQueryClient();
  const active = useRef(true);
  const notifiedTemporary = useRef(false);

  /** Espera o runtime (o que já está subindo ou um novo) e decide entre primeira vez e o app. */
  const boot = useCallback(() => {
    startRuntime()
      .then(async (info) => {
        setRuntimeInfo(info);
        if (!info.persistent && !notifiedTemporary.current) {
          notifiedTemporary.current = true;
          showToast({
            tone: "error",
            message:
              "Este navegador não permite guardar os dados do FinFlow: o que você fizer agora some ao fechar a aba. Abra numa janela normal de um navegador atualizado.",
          });
        }
        const firstRun = await needsFirstRun();
        if (active.current) setState({ step: firstRun ? "first-run" : "ready" });
      })
      .catch((error: unknown) => {
        if (!active.current) return;
        setState({
          step: "error",
          message:
            error instanceof Error && error.message
              ? error.message
              : "O navegador não conseguiu iniciar o FinFlow.",
        });
      });
  }, []);

  useEffect(() => {
    active.current = true;
    // outra aba com o banco aberto (ao abrir) ou que pediu "Usar aqui" (com o app já aberto)
    const toBlocked = () => {
      if (active.current) setState({ step: "blocked" });
    };
    const offBlocked = onRuntimeBlocked(toBlocked);
    const offLost = onRuntimeLost(toBlocked);
    boot();
    return () => {
      active.current = false;
      offBlocked();
      offLost();
    };
  }, [boot]);

  switch (state.step) {
    case "loading":
      return <Splash />;
    case "blocked":
      return (
        <BlockedScreen
          onUseHere={() => {
            setState({ step: "loading" });
            takeOver();
            // os dados podem ter mudado na outra aba
            void queryClient.invalidateQueries();
            boot();
          }}
        />
      );
    case "error":
      return <ErrorScreen message={state.message} />;
    case "first-run":
      return <FirstRun onDone={() => setState({ step: "ready" })} />;
    case "ready":
      return children;
  }
}
