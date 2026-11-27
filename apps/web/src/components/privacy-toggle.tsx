import { Eye, EyeOff } from "lucide-react";
import { cn } from "../lib/cn";
import { usePrivacy } from "../lib/privacy";

/** Olho do topo (v3 §16): oculta/mostra valores em todo o app; lembra por navegador. */
export function PrivacyToggle({ className }: { className?: string }) {
  const [hidden, toggle] = usePrivacy();
  const label = hidden ? "Mostrar valores" : "Ocultar valores";
  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={label}
      aria-pressed={hidden}
      title={label}
      data-testid="privacy-toggle"
      className={cn(
        "flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-(--radius-control) transition-colors hover:bg-app-soft hover:text-app-text",
        hidden ? "text-income" : "text-app-muted",
        className,
      )}
    >
      {hidden ? <EyeOff size={18} aria-hidden /> : <Eye size={18} aria-hidden />}
    </button>
  );
}
