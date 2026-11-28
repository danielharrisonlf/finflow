import { useSyncExternalStore } from "react";
import { type RuntimeInfo, requestPersistentStorage } from "../runtime/client";
import { showToast } from "./toast";

/**
 * Estado do FinFlow no navegador (docs/browser-runtime-contract.md §4), compartilhado entre a tela
 * de abertura (RuntimeGate) e Configurações → "Dados neste navegador". Só faz sentido no modo
 * navegador; no modo servidor nada aqui é chamado.
 */
let info: RuntimeInfo | null = null;
const listeners = new Set<() => void>();

export function setRuntimeInfo(next: RuntimeInfo): void {
  info = next;
  for (const listener of listeners) listener();
}

export function getRuntimeInfo(): RuntimeInfo | null {
  return info;
}

export function useRuntimeInfo(): RuntimeInfo | null {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => info,
    () => null,
  );
}

const ONBOARDED_KEY = "finflow_browser_onboarded";
const PERSIST_ASKED_KEY = "finflow_browser_persist_asked";

function read(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string | null): void {
  try {
    if (value === null) window.localStorage.removeItem(key);
    else window.localStorage.setItem(key, value);
  } catch {
    // storage indisponível: a pergunta pode voltar na próxima abertura
  }
}

/** A pessoa já passou pela primeira abertura (começou do zero, usou o exemplo ou restaurou). */
export function isOnboarded(): boolean {
  return read(ONBOARDED_KEY) === "1";
}

export function markOnboarded(): void {
  write(ONBOARDED_KEY, "1");
}

/** Depois de "Apagar todos os dados": a próxima abertura volta a ser a primeira. */
export function resetBrowserFlags(): void {
  write(ONBOARDED_KEY, null);
  write(PERSIST_ASKED_KEY, null);
}

/** Pede armazenamento persistente e mostra o resultado. Devolve se ficou persistente. */
export async function askPersistentStorage(): Promise<boolean> {
  write(PERSIST_ASKED_KEY, "1");
  let granted = false;
  try {
    granted = await requestPersistentStorage();
  } catch {
    granted = false;
  }
  showToast(
    granted
      ? {
          tone: "success",
          message: "Armazenamento persistente ativado: o navegador não apaga seus dados sozinho.",
          durationMs: 6000,
        }
      : {
          tone: "info",
          message:
            "O navegador não garantiu armazenamento persistente. Faça backups em Configurações → Dados neste navegador.",
          durationMs: 9000,
        },
  );
  return granted;
}

/** Na primeira gravação de dados reais, pede o armazenamento persistente uma única vez. */
export function askPersistentStorageOnce(): void {
  if (read(PERSIST_ASKED_KEY) === "1") return;
  void askPersistentStorage();
}

/** O FinFlow no navegador, publicado junto com a landing (docs/browser-runtime-contract.md §5). */
export const HOSTED_APP_URL = "https://finflow-landing-umber.vercel.app/app/";
