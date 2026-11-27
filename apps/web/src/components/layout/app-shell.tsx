import { useQueryClient } from "@tanstack/react-query";
import {
  CalendarRange,
  ChartPie,
  FileSpreadsheet,
  LayoutDashboard,
  LogOut,
  type LucideIcon,
  Menu,
  PiggyBank,
  Plus,
  ReceiptText,
  Repeat,
  Settings,
  Tags,
  TrendingUp,
  Wallet,
} from "lucide-react";
import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { clearAuthToken } from "../../lib/api";
import { cn } from "../../lib/cn";
import { useAuthStatus } from "../../lib/queries";
import { LoginPage } from "../../pages/login";
import { ThemeToggle } from "../landing/ThemeToggle";
import { NewTransactionSheet } from "../new-transaction-sheet";
import { Sheet } from "../ui/sheet";

interface NavItem {
  to: string;
  label: string;
  /** Rótulo curto da barra inferior no mobile. */
  short: string;
  icon: LucideIcon;
}

interface NavGroup {
  title: string | null;
  items: NavItem[];
}

const DASHBOARD: NavItem = {
  to: "/",
  label: "Visão geral",
  short: "Início",
  icon: LayoutDashboard,
};
const TRANSACTIONS: NavItem = {
  to: "/extrato",
  label: "Transações",
  short: "Extrato",
  icon: ReceiptText,
};
const COMMITMENTS: NavItem = {
  to: "/compromissos",
  label: "Compromissos",
  short: "Agenda",
  icon: Repeat,
};
const PROJECTION: NavItem = {
  to: "/projecao",
  label: "Projeção",
  short: "Projeção",
  icon: TrendingUp,
};

/** Navegação agrupada (contrato v2 §6). */
const NAV_GROUPS: NavGroup[] = [
  {
    title: "Organização",
    items: [
      DASHBOARD,
      TRANSACTIONS,
      COMMITMENTS,
      { to: "/ano", label: "Ano", short: "Ano", icon: CalendarRange },
    ],
  },
  {
    title: "Controle",
    items: [
      { to: "/contas", label: "Contas e cartões", short: "Contas", icon: Wallet },
      { to: "/categorias", label: "Categorias", short: "Categorias", icon: Tags },
      { to: "/orcamentos", label: "Orçamentos", short: "Orçamentos", icon: ChartPie },
      { to: "/metas", label: "Metas e desejos", short: "Metas", icon: PiggyBank },
    ],
  },
  {
    title: "Visão estratégica",
    items: [
      PROJECTION,
      { to: "/planilhas", label: "Planilhas", short: "Planilhas", icon: FileSpreadsheet },
    ],
  },
  {
    title: null,
    items: [{ to: "/configuracoes", label: "Configurações", short: "Ajustes", icon: Settings }],
  },
];

/** Atalhos fixos da barra inferior; o resto fica no "Mais". */
const MOBILE_PRIMARY: NavItem[] = [DASHBOARD, TRANSACTIONS, COMMITMENTS, PROJECTION];

function isActivePath(pathname: string, to: string): boolean {
  return to === "/" ? pathname === "/" : pathname.startsWith(to);
}

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
  const [moreOpen, setMoreOpen] = useState(false);
  const { pathname } = useLocation();
  const inMore = !MOBILE_PRIMARY.some((item) => isActivePath(pathname, item.to));
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
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 flex-col gap-5 border-r border-app-border bg-app-background py-6 md:flex">
        <Brand />
        <nav
          className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-3"
          aria-label="Principal"
        >
          {NAV_GROUPS.map((group) => (
            <div key={group.title ?? "geral"} className="flex flex-col gap-0.5">
              {group.title ? (
                <p className="px-3 pb-1 text-[11px] font-medium tracking-wide text-app-subtle uppercase">
                  {group.title}
                </p>
              ) : null}
              {group.items.map(({ to, label, icon: Icon }) => (
                <NavLink
                  key={to}
                  to={to}
                  end={to === "/"}
                  className={({ isActive }) =>
                    cn(
                      "flex h-10 shrink-0 items-center gap-3 rounded-(--radius-control) px-3 text-sm transition-colors",
                      isActive
                        ? "bg-app-surface font-medium text-income"
                        : "text-app-muted hover:bg-app-surface/60 hover:text-app-text",
                    )
                  }
                >
                  <Icon size={18} strokeWidth={2} aria-hidden />
                  {label}
                </NavLink>
              ))}
            </div>
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
      <main className="w-full min-w-0 px-4 pt-5 pb-28 md:ml-60 md:px-8 md:pt-8 md:pb-12">
        <div className="mx-auto max-w-[1360px]">
          <div className="mb-4 flex items-center justify-between md:hidden">
            <Brand />
            <div className="flex items-center gap-2">
              <ThemeToggle />
              <NavLink
                to="/configuracoes"
                aria-label="Configurações"
                className={({ isActive }) =>
                  cn(
                    "flex h-11 w-11 items-center justify-center rounded-(--radius-control) border transition-colors",
                    isActive
                      ? "border-emerald-500/40 bg-emerald-500/10 text-income"
                      : "border-app-border text-app-muted hover:text-app-text",
                  )
                }
              >
                <Settings size={16} aria-hidden />
              </NavLink>
              {auth.data?.required ? (
                <button
                  type="button"
                  onClick={onLogout}
                  aria-label="Sair"
                  className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-(--radius-control) border border-app-border text-app-muted hover:text-app-text"
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
          {MOBILE_PRIMARY.slice(0, 2).map((item) => (
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
          {MOBILE_PRIMARY.slice(2).map((item) => (
            <MobileLink key={item.to} {...item} />
          ))}
          <button
            type="button"
            onClick={() => setMoreOpen(true)}
            aria-haspopup="dialog"
            data-testid="mobile-more"
            className={cn(
              "flex h-full min-w-11 cursor-pointer flex-col items-center justify-center gap-1 text-[11px]",
              inMore ? "text-income" : "text-app-muted",
            )}
          >
            <Menu size={20} strokeWidth={2} aria-hidden />
            Mais
          </button>
        </div>
      </nav>

      <Sheet open={moreOpen} onOpenChange={setMoreOpen} title="Todas as seções">
        <nav aria-label="Todas as seções" className="flex flex-col gap-4 pb-2">
          {NAV_GROUPS.map((group) => (
            <div key={group.title ?? "geral"}>
              {group.title ? (
                <p className="pb-1.5 text-[11px] font-medium tracking-wide text-app-subtle uppercase">
                  {group.title}
                </p>
              ) : null}
              <div className="grid grid-cols-2 gap-2">
                {group.items.map(({ to, label, icon: Icon }) => (
                  <NavLink
                    key={to}
                    to={to}
                    end={to === "/"}
                    onClick={() => setMoreOpen(false)}
                    className={({ isActive }) =>
                      cn(
                        "flex min-h-12 items-center gap-2.5 rounded-(--radius-control) border px-3 text-sm transition-colors",
                        isActive
                          ? "border-emerald-500/40 bg-emerald-500/10 font-medium text-income"
                          : "border-app-border text-app-text hover:bg-app-soft",
                      )
                    }
                  >
                    <Icon size={18} aria-hidden className="shrink-0" />
                    <span className="min-w-0 leading-tight">{label}</span>
                  </NavLink>
                ))}
              </div>
            </div>
          ))}
        </nav>
      </Sheet>

      <NewTransactionSheet open={sheetOpen} onOpenChange={setSheetOpen} />
    </div>
  );
}

function MobileLink({ to, short, icon: Icon }: NavItem) {
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
      <Icon size={20} strokeWidth={2} aria-hidden />
      <span className="max-w-full truncate px-0.5">{short}</span>
    </NavLink>
  );
}
