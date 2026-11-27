import { cloneElement, isValidElement, type ReactNode, useSyncExternalStore } from "react";

/**
 * Modo privacidade (v3 §16): mascara valores monetários formatados por `formatBRL` e
 * companhia. Estado único por navegador (localStorage), lido de forma síncrona pelos
 * formatadores e observado pelos componentes via `usePrivacy`.
 */
const STORAGE_KEY = "finflow-privacy";
export const MASKED_BRL = "R$ •••••";

function readStored(): boolean {
  try {
    return window.localStorage.getItem(STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

let hidden = typeof window === "undefined" ? false : readStored();
const listeners = new Set<() => void>();

export function isPrivacyOn(): boolean {
  return hidden;
}

export function setPrivacy(next: boolean): void {
  if (next === hidden) return;
  hidden = next;
  try {
    window.localStorage.setItem(STORAGE_KEY, next ? "1" : "0");
  } catch {
    // storage indisponível: vale só nesta aba
  }
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** [oculto, alternar] — re-renderiza quem chama quando o modo muda. */
export function usePrivacy(): [boolean, () => void] {
  const value = useSyncExternalStore(subscribe, isPrivacyOn, () => false);
  return [value, () => setPrivacy(!hidden)];
}

/** Mascara valores "R$ 1.234,56" dentro de textos vindos da API (alertas, mensagens). */
export function maskMoneyText(text: string): string {
  return hidden ? text.replace(/[-−+]?\s?R\$\s?[-−]?[\d.]+(,\d{1,2})?/g, MASKED_BRL) : text;
}

/**
 * Força a página a re-renderizar quando o modo muda: os elementos das rotas são criados
 * uma vez no roteador, então clonamos para gerar novas props e furar o bailout do React.
 */
export function PrivacyRerender({ children }: { children: ReactNode }) {
  usePrivacy();
  return isValidElement(children) ? cloneElement(children) : children;
}
