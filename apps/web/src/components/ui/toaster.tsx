import { X } from "lucide-react";
import { cn } from "../../lib/cn";
import { dismissToast, useToasts } from "../../lib/toast";

/** Pilha de avisos (lib/toast). Fica acima da navegação inferior no celular. */
export function Toaster() {
  const toasts = useToasts();
  if (toasts.length === 0) return null;
  return (
    <div
      className="pointer-events-none fixed inset-x-0 bottom-20 z-50 flex flex-col items-center gap-2 px-4 md:bottom-6 md:items-end md:px-6"
      data-testid="toaster"
    >
      {toasts.map((toast) => (
        <div
          key={toast.id}
          role={toast.tone === "error" ? "alert" : "status"}
          className={cn(
            "pointer-events-auto flex w-full max-w-sm items-center gap-3 rounded-(--radius-card) border bg-app-surface px-4 py-3 text-sm text-app-text shadow-lg",
            toast.tone === "error"
              ? "border-rose-500/40"
              : toast.tone === "success"
                ? "border-emerald-500/40"
                : "border-app-border",
          )}
        >
          <p className="min-w-0 flex-1">{toast.message}</p>
          {toast.action ? (
            <button
              type="button"
              onClick={() => {
                dismissToast(toast.id);
                toast.action?.onClick();
              }}
              className="min-h-11 shrink-0 cursor-pointer rounded-(--radius-control) px-3 font-semibold text-income hover:bg-app-soft"
            >
              {toast.action.label}
            </button>
          ) : null}
          <button
            type="button"
            onClick={() => dismissToast(toast.id)}
            aria-label="Fechar aviso"
            className="flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-(--radius-control) text-app-muted hover:bg-app-soft hover:text-app-text"
          >
            <X size={16} aria-hidden />
          </button>
        </div>
      ))}
    </div>
  );
}
