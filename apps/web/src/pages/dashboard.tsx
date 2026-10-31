import type { CashflowPoint, CategorySlice } from "@finflow/contracts";
import {
  ArrowDownRight,
  ArrowUpRight,
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  CreditCard,
} from "lucide-react";
import { useState } from "react";
import {
  Area,
  AreaChart,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card, CardHeader } from "../components/ui/card";
import { CategoryIcon } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Money } from "../components/ui/money";
import { formatBRL, formatDate, formatMonthYear } from "../lib/format";
import {
  type Period,
  useCashflow,
  useCategorySlices,
  useSummary,
  useUpcoming,
} from "../lib/queries";

function usePeriodState(): [Period, (delta: number) => void] {
  const now = new Date();
  const [period, setPeriod] = useState<Period>({
    month: now.getMonth() + 1,
    year: now.getFullYear(),
  });
  const shift = (delta: number): void => {
    setPeriod((p) => {
      const zero = p.year * 12 + (p.month - 1) + delta;
      return { year: Math.floor(zero / 12), month: (zero % 12) + 1 };
    });
  };
  return [period, shift];
}

export function DashboardPage() {
  const [period, shiftPeriod] = usePeriodState();
  const summary = useSummary(period);
  const cashflow = useCashflow(period);
  const slices = useCategorySlices(period);
  const upcoming = useUpcoming();

  const s = summary.data;

  return (
    <div className="space-y-5">
      <header className="rise flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-zinc-100">Visão geral</h1>
          <p className="text-sm text-zinc-500">Suas finanças em um olhar</p>
        </div>
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => shiftPeriod(-1)}
            aria-label="Mês anterior"
            className="flex h-10 w-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100"
          >
            <ChevronLeft size={18} />
          </button>
          <span className="tnum min-w-32 text-center text-sm font-medium text-zinc-300">
            {formatMonthYear(period.month, period.year)}
          </span>
          <button
            type="button"
            onClick={() => shiftPeriod(1)}
            aria-label="Próximo mês"
            className="flex h-10 w-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100"
          >
            <ChevronRight size={18} />
          </button>
        </div>
      </header>

      <div className="grid grid-cols-12 gap-4">
        {/* herói: saldo consolidado */}
        <section
          className="rise relative col-span-12 overflow-hidden rounded-(--radius-hero) border border-zinc-800/80 bg-zinc-900/60 p-6 lg:col-span-5"
          style={{ animationDelay: "60ms" }}
          data-testid="hero-balance"
        >
          <div
            aria-hidden
            className="pointer-events-none absolute -top-24 -right-20 h-64 w-64 rounded-full bg-emerald-500/10 blur-3xl"
          />
          <p className="text-sm text-zinc-400">Saldo geral</p>
          <p className="tnum mt-2 text-[2.75rem] leading-none font-semibold tracking-tight text-zinc-50 md:text-5xl">
            {s ? formatBRL(s.totalBalanceCents) : "—"}
          </p>
          <div className="mt-6 flex flex-wrap gap-x-8 gap-y-3">
            <div>
              <p className="text-xs text-zinc-500">Faturas em aberto</p>
              <p className="tnum mt-0.5 text-[15px] font-medium text-zinc-200">
                {s ? formatBRL(s.openInvoicesCents) : "—"}
              </p>
            </div>
            <div>
              <p className="text-xs text-zinc-500">Pendências</p>
              <p className="tnum mt-0.5 text-[15px] font-medium text-zinc-200">
                {s ? `${s.pendingCount} lançamento${s.pendingCount === 1 ? "" : "s"}` : "—"}
              </p>
            </div>
          </div>
        </section>

        {/* trio de métricas do mês */}
        <section
          className="rise col-span-12 grid grid-cols-3 gap-4 lg:col-span-3 lg:grid-cols-1"
          style={{ animationDelay: "120ms" }}
        >
          <StatTile
            label="Receitas"
            cents={s?.monthIncomeCents}
            icon={<ArrowUpRight size={15} className="text-(--color-income)" />}
            tone="income"
          />
          <StatTile
            label="Despesas"
            cents={s?.monthExpenseCents === undefined ? undefined : -s.monthExpenseCents}
            icon={<ArrowDownRight size={15} className="text-(--color-expense)" />}
            tone="expense"
          />
          <StatTile label="Balanço" cents={s?.monthNetCents} tone="net" />
        </section>

        {/* próximos 7 dias */}
        <Card className="rise col-span-12 lg:col-span-4" style={{ animationDelay: "180ms" }}>
          <CardHeader title="Próximos 7 dias" />
          {upcoming.isLoading ? (
            <LoadingRows rows={3} />
          ) : (upcoming.data?.length ?? 0) === 0 ? (
            <Empty
              icon={CalendarClock}
              title="Nada vencendo"
              hint="Contas pendentes e faturas aparecem aqui quando estiverem perto do vencimento."
            />
          ) : (
            <ul className="divide-y divide-zinc-800/60 px-5 pb-2">
              {upcoming.data?.map((item) => (
                <li key={item.id} className="flex items-center gap-3 py-3">
                  <span className="flex h-9 w-9 items-center justify-center rounded-full bg-zinc-800/80 text-zinc-400">
                    {item.kind === "INVOICE" ? (
                      <CreditCard size={16} />
                    ) : (
                      <CalendarClock size={16} />
                    )}
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-zinc-200">{item.description}</p>
                    <p className="text-xs text-zinc-500">vence {formatDate(item.dueDate)}</p>
                  </div>
                  <Money cents={item.amountCents} signed className="text-sm font-medium" />
                </li>
              ))}
            </ul>
          )}
        </Card>

        {/* fluxo de caixa */}
        <Card className="rise col-span-12 lg:col-span-8" style={{ animationDelay: "240ms" }}>
          <CardHeader title="Fluxo de caixa do mês" />
          <div className="h-64 px-2 pb-3" data-testid="cashflow-chart">
            {cashflow.data && cashflow.data.length > 0 ? (
              <CashflowChart data={cashflow.data} />
            ) : (
              <Empty
                icon={ArrowUpRight}
                title="Sem movimentos neste mês"
                hint="Lance receitas e despesas para ver o fluxo diário."
              />
            )}
          </div>
        </Card>

        {/* distribuição por categoria */}
        <Card className="rise col-span-12 lg:col-span-4" style={{ animationDelay: "300ms" }}>
          <CardHeader title="Gastos por categoria" />
          {slices.data && slices.data.length > 0 ? (
            <CategoryDonut data={slices.data} />
          ) : (
            <Empty
              icon={ArrowDownRight}
              title="Sem despesas no período"
              hint="Suas categorias mais pesadas aparecem aqui."
            />
          )}
        </Card>
      </div>
    </div>
  );
}

function StatTile({
  label,
  cents,
  icon,
  tone,
}: {
  label: string;
  cents: number | undefined;
  icon?: React.ReactNode;
  tone: "income" | "expense" | "net";
}) {
  const color =
    tone === "income"
      ? "text-(--color-income)"
      : tone === "expense"
        ? "text-(--color-expense)"
        : cents !== undefined && cents < 0
          ? "text-(--color-expense)"
          : "text-zinc-100";
  return (
    <Card className="flex flex-col justify-center px-4 py-3.5 lg:px-5">
      <p className="flex items-center gap-1.5 text-xs text-zinc-500 lg:text-sm">
        {icon}
        {label}
      </p>
      <p className={`tnum mt-1 text-base font-semibold lg:text-xl ${color}`}>
        {cents !== undefined ? formatBRL(cents) : "—"}
      </p>
    </Card>
  );
}

const tooltipStyle = {
  backgroundColor: "#18181b",
  border: "1px solid #3f3f46",
  borderRadius: 10,
  fontSize: 13,
  color: "#e4e4e7",
} as const;

function CashflowChart({ data }: { data: CashflowPoint[] }) {
  const chartData = data.map((p) => ({
    day: p.date.slice(8, 10),
    Entradas: p.incomeCents / 100,
    Saídas: p.expenseCents / 100,
  }));
  return (
    <ResponsiveContainer width="100%" height="100%">
      <AreaChart data={chartData} margin={{ top: 12, right: 12, bottom: 0, left: 0 }}>
        <defs>
          <linearGradient id="gIn" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#34d399" stopOpacity={0.25} />
            <stop offset="100%" stopColor="#34d399" stopOpacity={0} />
          </linearGradient>
          <linearGradient id="gOut" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#fb7185" stopOpacity={0.22} />
            <stop offset="100%" stopColor="#fb7185" stopOpacity={0} />
          </linearGradient>
        </defs>
        <XAxis
          dataKey="day"
          tickLine={false}
          axisLine={false}
          tick={{ fill: "#71717a", fontSize: 11 }}
          dy={6}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          width={52}
          tick={{ fill: "#71717a", fontSize: 11 }}
          tickFormatter={(v: number) => (v >= 1000 ? `${Math.round(v / 1000)}k` : String(v))}
        />
        <Tooltip
          contentStyle={tooltipStyle}
          formatter={(value: number) => formatBRL(Math.round(value * 100))}
          labelFormatter={(day) => `Dia ${day}`}
        />
        <Area
          type="monotone"
          dataKey="Entradas"
          stroke="#34d399"
          strokeWidth={2}
          fill="url(#gIn)"
        />
        <Area type="monotone" dataKey="Saídas" stroke="#fb7185" strokeWidth={2} fill="url(#gOut)" />
      </AreaChart>
    </ResponsiveContainer>
  );
}

function CategoryDonut({ data }: { data: CategorySlice[] }) {
  const top = data.slice(0, 6);
  const total = data.reduce((acc, s) => acc + s.totalCents, 0);
  return (
    <div className="flex flex-col gap-1 px-5 pb-5">
      <div className="relative mx-auto h-44 w-44" data-testid="category-donut">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={top}
              dataKey="totalCents"
              nameKey="name"
              innerRadius={58}
              outerRadius={82}
              paddingAngle={3}
              strokeWidth={0}
            >
              {top.map((slice) => (
                <Cell key={slice.name} fill={slice.color ?? "#71717a"} />
              ))}
            </Pie>
            <Tooltip contentStyle={tooltipStyle} formatter={(value: number) => formatBRL(value)} />
          </PieChart>
        </ResponsiveContainer>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-[11px] text-zinc-500">Total</span>
          <span className="tnum text-sm font-semibold text-zinc-100">{formatBRL(total)}</span>
        </div>
      </div>
      <ul className="mt-2 space-y-2.5">
        {top.map((slice) => (
          <li key={slice.name} className="flex items-center gap-2.5">
            <CategoryIcon icon={slice.icon} color={slice.color} size={28} />
            <span className="min-w-0 flex-1 truncate text-sm text-zinc-300">{slice.name}</span>
            <span className="tnum text-xs text-zinc-500">{slice.percent}%</span>
            <span className="tnum w-24 text-right text-sm font-medium text-zinc-200">
              {formatBRL(slice.totalCents)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
