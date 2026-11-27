import type { Alert, AlertSeverity } from "@finflow/contracts";
import {
  AlertOctagon,
  AlertTriangle,
  Bell,
  BellOff,
  ChevronRight,
  Info,
  type LucideIcon,
  X,
} from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { cn } from "../lib/cn";
import { formatDate } from "../lib/format";
import { maskMoneyText, usePrivacy } from "../lib/privacy";
import { useAlerts, useDismissAlert } from "../lib/queries";
import { Empty, LoadingRows } from "./ui/empty";
import { Sheet } from "./ui/sheet";

const SEVERITY: Record<AlertSeverity, { icon: LucideIcon; cls: string; label: string }> = {
  DANGER: { icon: AlertOctagon, cls: "text-expense bg-rose-500/10", label: "Urgente" },
  WARNING: { icon: AlertTriangle, cls: "text-warn bg-amber-400/10", label: "Atenção" },
  INFO: { icon: Info, cls: "text-app-muted bg-app-soft", label: "Dica" },
};

/** Sino do topo (v3 §3): contador + lista com atalho para agir e dispensar. */
export function AlertsBell({ className }: { className?: string }) {
  const [open, setOpen] = useState(false);
  const alerts = useAlerts();
  const list = alerts.data ?? [];
  const count = list.length;
  const urgent = list.some((a) => a.severity === "DANGER");

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label={count > 0 ? `Alertas (${count})` : "Alertas"}
        aria-haspopup="dialog"
        data-testid="alerts-bell"
        className={cn(
          "relative flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-(--radius-control) text-app-muted transition-colors hover:bg-app-soft hover:text-app-text",
          className,
        )}
      >
        <Bell size={18} aria-hidden />
        {count > 0 ? (
          <span
            className={cn(
              "tnum absolute top-1.5 right-1.5 flex h-4.5 min-w-4.5 items-center justify-center rounded-full px-1 text-[10px] leading-none font-semibold",
              urgent ? "bg-rose-500 text-white" : "bg-amber-400 text-amber-950",
            )}
            data-testid="alerts-count"
            aria-hidden
          >
            {count > 9 ? "9+" : count}
          </span>
        ) : null}
      </button>
      <Sheet open={open} onOpenChange={setOpen} title="Alertas e dicas">
        <AlertsList
          alerts={list}
          loading={alerts.isLoading}
          error={alerts.isError}
          onNavigate={() => setOpen(false)}
        />
      </Sheet>
    </>
  );
}

function AlertsList({
  alerts,
  loading,
  error,
  onNavigate,
}: {
  alerts: Alert[];
  loading: boolean;
  error: boolean;
  onNavigate: () => void;
}) {
  const dismiss = useDismissAlert();
  usePrivacy();

  if (loading) return <LoadingRows rows={3} />;
  if (error) {
    return (
      <Empty
        icon={BellOff}
        title="Alertas indisponíveis"
        hint="Não foi possível carregar os alertas agora."
      />
    );
  }
  if (alerts.length === 0) {
    return (
      <Empty
        icon={Bell}
        title="Tudo em dia"
        hint="Avisamos aqui quando um orçamento apertar, uma conta estiver para vencer ou uma assinatura ficar mais cara."
      />
    );
  }
  return (
    <ul className="flex flex-col gap-2 pb-3" data-testid="alerts-list">
      {alerts.map((alert) => {
        const meta = SEVERITY[alert.severity] ?? SEVERITY.INFO;
        const Icon = meta.icon;
        return (
          <li
            key={alert.id}
            className="flex gap-3 rounded-(--radius-card) border border-app-border/80 p-3"
            data-severity={alert.severity}
          >
            <span
              className={cn(
                "flex h-9 w-9 shrink-0 items-center justify-center rounded-full",
                meta.cls,
              )}
            >
              <Icon size={16} aria-hidden />
              <span className="sr-only">{meta.label}</span>
            </span>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-app-text">{maskMoneyText(alert.title)}</p>
              <p className="mt-0.5 text-sm text-app-muted">{maskMoneyText(alert.message)}</p>
              <div className="mt-1.5 flex flex-wrap items-center gap-x-3">
                <span className="text-xs text-app-subtle">{formatDate(alert.date)}</span>
                {alert.href ? (
                  <Link
                    to={alert.href}
                    onClick={onNavigate}
                    className="inline-flex min-h-9 items-center gap-0.5 text-sm font-medium text-income underline-offset-4 hover:underline"
                  >
                    Resolver <ChevronRight size={14} aria-hidden />
                  </Link>
                ) : null}
              </div>
            </div>
            <button
              type="button"
              onClick={() => dismiss.mutate(alert.id)}
              aria-label={`Dispensar alerta: ${alert.title}`}
              title="Dispensar"
              className="flex h-9 w-9 shrink-0 cursor-pointer items-center justify-center rounded-full text-app-subtle hover:bg-app-soft hover:text-app-text"
            >
              <X size={15} />
            </button>
          </li>
        );
      })}
    </ul>
  );
}
