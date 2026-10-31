import {
  ChartPie,
  LayoutDashboard,
  type LucideIcon,
  PiggyBank,
  Plus,
  ReceiptText,
  Wallet,
} from "lucide-react";
import { useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { cn } from "../../lib/cn";
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
];

function Brand() {
  return (
    <div className="flex items-center gap-2.5 px-3">
      <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-500/15">
        <svg width="16" height="16" viewBox="0 0 32 32" fill="none" aria-hidden="true">
          <path d="M9 22V10h12M9 16h9" stroke="#34d399" strokeWidth="3.5" strokeLinecap="round" />
        </svg>
      </span>
      <span className="text-[17px] font-semibold tracking-tight text-zinc-100">FinFlow</span>
    </div>
  );
}

export function AppShell() {
  const [sheetOpen, setSheetOpen] = useState(false);

  return (
    <div className="min-h-dvh md:flex">
      {/* sidebar — desktop */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 flex-col gap-6 border-r border-zinc-900 bg-zinc-950 py-6 md:flex">
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
                    ? "bg-zinc-900 font-medium text-emerald-400"
                    : "text-zinc-400 hover:bg-zinc-900/60 hover:text-zinc-200",
                )
              }
            >
              <Icon size={18} strokeWidth={2} />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="px-3">
          <button
            type="button"
            onClick={() => setSheetOpen(true)}
            className="flex h-11 w-full cursor-pointer items-center justify-center gap-2 rounded-(--radius-control) bg-emerald-500 text-sm font-semibold text-emerald-950 transition-colors hover:bg-emerald-400"
          >
            <Plus size={18} strokeWidth={2.5} />
            Nova transação
          </button>
        </div>
      </aside>

      {/* conteúdo */}
      <main className="w-full px-4 pt-5 pb-28 md:ml-60 md:px-8 md:pt-8 md:pb-12">
        <div className="mx-auto max-w-[1360px]">
          <Outlet />
        </div>
      </main>

      {/* bottom nav — mobile */}
      <nav
        className="fixed inset-x-0 bottom-0 z-30 border-t border-zinc-900 bg-zinc-950/90 backdrop-blur-lg md:hidden"
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
          isActive ? "text-emerald-400" : "text-zinc-500",
        )
      }
    >
      <Icon size={20} strokeWidth={2} />
      {label.split(" ")[0]}
    </NavLink>
  );
}
