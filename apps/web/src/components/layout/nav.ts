import type { SpaceKind } from "@finflow/contracts";
import {
  ArrowDownUp,
  Briefcase,
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
  Users,
  Wallet,
} from "lucide-react";

export interface NavItem {
  to: string;
  label: string;
  /** Rótulo curto da barra inferior no mobile. */
  short: string;
  icon: LucideIcon;
  /** Tipos de espaço em que a página aparece (ausente = todos). */
  kinds?: readonly SpaceKind[];
  /** Rótulo alternativo por tipo de espaço. */
  labelFor?: Partial<Record<SpaceKind, string>>;
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
      {
        to: "/divisao",
        label: "Divisão de despesas",
        short: "Divisão",
        icon: Users,
        kinds: ["HOME"],
      },
      { to: "/categorias", label: "Categorias", short: "Categorias", icon: Tags },
      { to: "/orcamentos", label: "Orçamentos", short: "Orçamentos", icon: ChartPie },
      {
        to: "/metas",
        label: "Metas e desejos",
        short: "Metas",
        icon: PiggyBank,
        // empresa: sem desejos nem reserva de emergência (v3 fase 2 §6)
        labelFor: { BUSINESS: "Metas" },
      },
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
      {
        to: "/empresa",
        label: "Empresa",
        short: "Empresa",
        icon: Briefcase,
        kinds: ["BUSINESS"],
      },
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

/** Grupos com só as páginas relevantes para o tipo do espaço ativo (v3 fase 2 §6). */
export function navGroupsFor(kind: SpaceKind): NavGroup[] {
  return NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items
      .filter((item) => !item.kinds || item.kinds.includes(kind))
      .map((item) => ({ ...item, label: item.labelFor?.[kind] ?? item.label })),
  })).filter((group) => group.items.length > 0);
}

export function navItemsFor(kind: SpaceKind): NavItem[] {
  return navGroupsFor(kind).flatMap((g) => g.items);
}

export function isActivePath(pathname: string, to: string): boolean {
  return to === "/" ? pathname === "/" : pathname.startsWith(to);
}
