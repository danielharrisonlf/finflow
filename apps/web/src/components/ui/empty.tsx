import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

export function Empty({
  icon: Icon,
  title,
  hint,
  action,
}: {
  icon: LucideIcon;
  title: string;
  hint?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-12 text-center">
      <Icon size={28} className="text-app-subtle" aria-hidden />
      <p className="text-sm font-medium text-app-text">{title}</p>
      {hint ? <p className="max-w-xs text-sm text-app-muted">{hint}</p> : null}
      {action ? <div className="mt-3">{action}</div> : null}
    </div>
  );
}

export function LoadingRows({ rows = 4 }: { rows?: number }) {
  return (
    <div className="animate-pulse space-y-3 p-5" role="status" aria-label="Carregando">
      {Array.from({ length: rows }, (_, i) => (
        <div key={String(i)} className="flex items-center gap-3">
          <div className="h-9 w-9 rounded-full bg-app-soft" />
          <div className="h-3 flex-1 rounded bg-app-soft" />
          <div className="h-3 w-16 rounded bg-app-soft" />
        </div>
      ))}
    </div>
  );
}
