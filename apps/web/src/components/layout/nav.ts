import {
  ArrowDownUp,
  Calculator,
  CalendarRange,
  ChartCandlestick,
  ChartPie,
  FileSpreadsheet,
  HandCoins,
  LayoutDashboard,
  type LucideIcon,
  PiggyBank,
  ReceiptText,
  Repeat,
  Settings,
  Sparkles,
  Tags,
  TrendingUp,
  Wallet,
} from "lucide-react";

export interface NavItem {
  to: string;
  label: string;
  /** Rótulo curto da barra inferior no mobile. */
  short: string;
  icon: LucideIcon;
}

export interface NavGroup {
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

/** Navegação agrupada (contrato v2 §6 + v3 §16). */
export const NAV_GROUPS: NavGroup[] = [
  {
    title: "Organização",
    items: [
      DASHBOARD,
      TRANSACTIONS,
      COMMITMENTS,
      { to: "/ano", label: "Ano", short: "Ano", icon: CalendarRange },
      { to: "/fluxo", label: "Fluxo de caixa", short: "Fluxo", icon: ArrowDownUp },
    ],
  },
  {
    title: "Controle",
    items: [
      { to: "/contas", label: "Contas e cartões", short: "Contas", icon: Wallet },
      { to: "/categorias", label: "Categorias", short: "Categorias", icon: Tags },
      { to: "/orcamentos", label: "Orçamentos", short: "Orçamentos", icon: ChartPie },
      { to: "/metas", label: "Metas e desejos", short: "Metas", icon: PiggyBank },
      {
        to: "/investimentos",
        label: "Investimentos",
        short: "Investimentos",
        icon: ChartCandlestick,
      },
      { to: "/dividas", label: "Dívidas", short: "Dívidas", icon: HandCoins },
    ],
  },
  {
    title: "Visão estratégica",
    items: [
      PROJECTION,
      { to: "/planilhas", label: "Planilhas", short: "Planilhas", icon: FileSpreadsheet },
      { to: "/assistente", label: "Assistente", short: "Assistente", icon: Sparkles },
      { to: "/calculadoras", label: "Calculadoras", short: "Calculadoras", icon: Calculator },
    ],
  },
  {
    title: null,
    items: [{ to: "/configuracoes", label: "Configurações", short: "Ajustes", icon: Settings }],
  },
];

/** Atalhos fixos da barra inferior; o resto fica no "Mais". */
export const MOBILE_PRIMARY: NavItem[] = [DASHBOARD, TRANSACTIONS, COMMITMENTS, PROJECTION];

export const ALL_NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items);

export function isActivePath(pathname: string, to: string): boolean {
  return to === "/" ? pathname === "/" : pathname.startsWith(to);
}
