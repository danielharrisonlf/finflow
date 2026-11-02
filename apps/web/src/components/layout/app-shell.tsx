import { useQueryClient } from "@tanstack/react-query";
import {
  ChartPie,
  LayoutDashboard,
  LogOut,
  type LucideIcon,
  PiggyBank,
  Plus,
  ReceiptText,
  Tags,
  Wallet,
} from "lucide-react";
import { useEffect, useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { clearAuthToken } from "../../lib/api";
import { cn } from "../../lib/cn";
import { useAuthStatus } from "../../lib/queries";
import { LoginPage } from "../../pages/login";
import { ThemeToggle } from "../landing/ThemeToggle";
import { NewTransactionSheet } from "../new-transaction-sheet";

interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
}

const NAV: NavItem[] = [
  { to: "/", label: "Início", icon: LayoutDashboard },
  { to: "/extrato", label: "Extrato", icon: ReceiptText },
  { to: "/contas", label: "Contas & Cartões", icon: Wallet },
  { to: "/orcamentos", label: "Orçamentos", icon: ChartPie },
  { to: "/metas", label: "Metas", icon: PiggyBank },
  { to: "/categorias", label: "Categorias", icon: Tags },
];

function Brand() {
  return (
    <div className="flex items-center gap-2.5 px-3">
      <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-500/15">
        <svg width="16" height="16" viewBox="0 0 32 32" fill="none" aria-hidden="true">
          <path d="M9 22V10h12M9 16h9" stroke="#34d399" strokeWidth="3.5" strokeLinecap="round" />
        </svg>
      </span>
      <span className="text-[17px] font-semibold tracking-tight text-app-text">FinFlow</span>
    </div>
  );
}

export function AppShell() {
  const [sheetOpen, setSheetOpen] = useState(false);
  const auth = useAuthStatus();
  const queryClient = useQueryClient();

  useEffect(() => {
    const onUnauthorized = () => {
      void queryClient.invalidateQueries({ queryKey: ["auth", "status"] });
    };
    window.addEventListener("finflow:unauthorized", onUnauthorized);
    return () => window.removeEventListener("finflow:unauthorized", onUnauthorized);
  }, [queryClient]);

  const onLogout = (): void => {
    clearAuthToken();
    void queryClient.invalidateQueries();
  };

  if (auth.data?.required && !auth.data.authenticated) {
    return <LoginPage />;
  }

  return (
    <div className="min-h-dvh md:flex">
      {/* sidebar — desktop */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 flex-col gap-6 border-r border-app-border bg-app-background py-6 md:flex">
        <Brand />
        <nav className="flex flex-1 flex-col gap-1 px-3" aria-label="Principal">
          {NAV.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              className={({ isActive }) =>
                cn(
                  "flex h-10 items-center gap-3 rounded-(--radius-control) px-3 text-sm transition-colors",
                  isActive
                    ? "bg-app-surface font-medium text-income"
                    : "text-app-muted hover:bg-app-surface/60 hover:text-app-text",
                )
              }
            >
              <Icon size={18} strokeWidth={2} />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="flex flex-col gap-2 px-3">
          <div className="flex items-center justify-between px-2 text-xs text-app-muted">
            <span>Aparência</span>
            <ThemeToggle />
          </div>
          <button
            type="button"
            onClick={() => setSheetOpen(true)}
            className="flex h-11 w-full cursor-pointer items-center justify-center gap-2 rounded-(--radius-control) bg-emerald-500 text-sm font-semibold text-emerald-950 transition-colors hover:bg-emerald-400"
          >
            <Plus size={18} strokeWidth={2.5} />
            Nova transação
          </button>
          {auth.data?.required ? (
            <button
              type="button"
              onClick={onLogout}
              className="flex h-9 w-full cursor-pointer items-center justify-center gap-2 rounded-(--radius-control) text-xs text-app-muted transition-colors hover:bg-app-surface hover:text-app-text"
            >
              <LogOut size={14} />
              Sair
            </button>
          ) : null}
        </div>
      </aside>

      {/* conteúdo */}
      <main className="w-full px-4 pt-5 pb-28 md:ml-60 md:px-8 md:pt-8 md:pb-12">
        <div className="mx-auto max-w-[1360px]">
          <div className="mb-4 flex items-center justify-between md:hidden">
            <Brand />
            <div className="flex items-center gap-2">
              <ThemeToggle />
              <NavLink
                to="/categorias"
                className={({ isActive }) =>
                  cn(
                    "flex h-9 items-center gap-1.5 rounded-(--radius-control) border px-2.5 text-xs transition-colors",
                    isActive
                      ? "border-emerald-500/40 bg-emerald-500/10 text-income"
                      : "border-app-border text-app-muted hover:border-app-border hover:text-app-text",
                  )
                }
              >
                <Tags size={14} />
                Categorias
              </NavLink>
              {auth.data?.required ? (
                <button
                  type="button"
                  onClick={onLogout}
                  aria-label="Sair"
                  className="flex h-9 w-9 cursor-pointer items-center justify-center rounded-(--radius-control) border border-app-border text-app-muted hover:text-app-text"
                >
                  <LogOut size={14} />
                </button>
              ) : null}
            </div>
          </div>
          <Outlet />
        </div>
      </main>

      {/* bottom nav — mobile */}
      <nav
        className="fixed inset-x-0 bottom-0 z-30 border-t border-app-border bg-app-background/90 backdrop-blur-lg md:hidden"
        style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
        aria-label="Principal"
      >
        <div className="grid h-16 grid-cols-6 items-center">
          {NAV.slice(0, 2).map((item) => (
            <MobileLink key={item.to} {...item} />
          ))}
          <div className="relative flex justify-center">
            <button
              type="button"
              onClick={() => setSheetOpen(true)}
              aria-label="Nova transação"
              data-testid="fab-new-transaction"
              className="absolute -top-9 flex h-14 w-14 cursor-pointer items-center justify-center rounded-full bg-emerald-500 text-emerald-950 shadow-lg shadow-emerald-500/25 transition-transform active:scale-95"
            >
              <Plus size={26} strokeWidth={2.5} />
            </button>
          </div>
          {NAV.slice(2, 5).map((item) => (
            <MobileLink key={item.to} {...item} />
          ))}
        </div>
      </nav>

      <NewTransactionSheet open={sheetOpen} onOpenChange={setSheetOpen} />
    </div>
  );
}

function MobileLink({ to, label, icon: Icon }: NavItem) {
  return (
    <NavLink
      to={to}
      end={to === "/"}
      className={({ isActive }) =>
        cn(
          "flex h-full min-w-11 flex-col items-center justify-center gap-1 text-[11px]",
          isActive ? "text-income" : "text-app-muted",
        )
      }
    >
      <Icon size={20} strokeWidth={2} />
      {label.split(" ")[0]}
    </NavLink>
  );
}
