import type { SpaceKind } from "@finflow/contracts";
import { useSyncExternalStore } from "react";

/**
 * Espaço ativo (v3 fase 2 §2 e §6): slug guardado no localStorage, lido de forma síncrona
 * por lib/api (cabeçalho `X-FinFlow-Space` e `?space=` nos downloads) e observado pelos
 * componentes via `useActiveSpaceSlug`. O tipo fica em cache para a navegação não piscar
 * enquanto a lista de espaços carrega.
 */
export const DEFAULT_SPACE_SLUG = "pessoal";
const SLUG_KEY = "finflow_space";
const KIND_KEY = "finflow_space_kind";
export const SPACE_HEADER = "X-FinFlow-Space";

const SPACE_KINDS: readonly SpaceKind[] = ["PERSONAL", "HOME", "BUSINESS"];

function read(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // storage indisponível: vale só nesta aba
  }
}

function readKind(): SpaceKind {
  const stored = read(KIND_KEY);
  return SPACE_KINDS.includes(stored as SpaceKind) ? (stored as SpaceKind) : "PERSONAL";
}

let slug = (typeof window === "undefined" ? null : read(SLUG_KEY)) || DEFAULT_SPACE_SLUG;
let kindHint: SpaceKind = typeof window === "undefined" ? "PERSONAL" : readKind();
const listeners = new Set<() => void>();

export function getActiveSpace(): string {
  return slug;
}

export function getSpaceKindHint(): SpaceKind {
  return slug === DEFAULT_SPACE_SLUG ? "PERSONAL" : kindHint;
}

/** Troca o espaço ativo; quem assina (main.tsx) limpa o cache do TanStack Query. */
export function setActiveSpace(next: string, kind?: SpaceKind): void {
  if (kind && kind !== kindHint) {
    kindHint = kind;
    write(KIND_KEY, kind);
  }
  if (next === slug) return;
  slug = next;
  write(SLUG_KEY, next);
  for (const listener of listeners) listener();
}

/** Volta para o espaço padrão (slug desconhecido pela API). */
export function resetToDefaultSpace(): void {
  setActiveSpace(DEFAULT_SPACE_SLUG, "PERSONAL");
}

/** Atualiza só o tipo em cache (sem trocar de espaço nem notificar). */
export function rememberSpaceKind(kind: SpaceKind): void {
  if (kind === kindHint) return;
  kindHint = kind;
  write(KIND_KEY, kind);
}

export function subscribeSpace(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useActiveSpaceSlug(): string {
  return useSyncExternalStore(subscribeSpace, getActiveSpace, () => DEFAULT_SPACE_SLUG);
}

/** Cabeçalho do espaço para qualquer fetch da API. */
export function spaceHeaders(): Record<string, string> {
  return { [SPACE_HEADER]: slug };
}
