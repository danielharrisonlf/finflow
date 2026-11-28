import { useSyncExternalStore } from "react";

/**
 * Avisos curtos no canto da tela (modo navegador: resultado do armazenamento persistente, nova
 * versão do app, falha num download). Store mínimo, sem dependência: quem mostra é o `Toaster`.
 */
export interface Toast {
  id: number;
  message: string;
  tone: "info" | "success" | "error";
  action?: { label: string; onClick: () => void };
  /** Sem duração: fica até a pessoa fechar ou agir. */
  durationMs?: number;
}

let toasts: Toast[] = [];
let nextId = 1;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

export function showToast(toast: Omit<Toast, "id">): number {
  const id = nextId++;
  toasts = [...toasts, { ...toast, id }];
  emit();
  if (toast.durationMs) window.setTimeout(() => dismissToast(id), toast.durationMs);
  return id;
}

export function dismissToast(id: number): void {
  const next = toasts.filter((t) => t.id !== id);
  if (next.length === toasts.length) return;
  toasts = next;
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useToasts(): Toast[] {
  return useSyncExternalStore(
    subscribe,
    () => toasts,
    () => toasts,
  );
}
