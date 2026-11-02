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
          <h1 className="text-xl font-semibold tracking-tight text-app-text">Visão geral</h1>
          <p className="text-sm text-app-muted">Suas finanças em um olhar</p>
        </div>
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => shiftPeriod(-1)}
            aria-label="Mês anterior"
            className="flex h-10 w-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-app-muted hover:bg-app-surface hover:text-app-text"
          >
            <ChevronLeft size={18} />
          </button>
          <span className="tnum min-w-32 text-center text-sm font-medium text-app-text">
            {formatMonthYear(period.month, period.year)}
          </span>
          <button
            type="button"
            onClick={() => shiftPeriod(1)}
            aria-label="Próximo mês"
            className="flex h-10 w-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-app-muted hover:bg-app-surface hover:text-app-text"
          >
            <ChevronRight size={18} />
          </button>
        </div>
      </header>

      <div className="grid grid-cols-12 gap-4">
        {/* herói: saldo consolidado */}
        <section
          className="rise relative col-span-12 overflow-hidden rounded-(--radius-hero) border border-app-border/80 bg-app-surface/60 p-6 lg:col-span-5"
          style={{ animationDelay: "60ms" }}
          data-testid="hero-balance"
        >
          <div
            aria-hidden
            className="pointer-events-none absolute -top-24 -right-20 h-64 w-64 rounded-full bg-emerald-500/10 blur-3xl"
          />
          <p className="text-sm text-app-muted">Saldo geral</p>
          <p className="tnum mt-2 text-[2.75rem] leading-none font-semibold tracking-tight text-app-text md:text-5xl">
            {s ? formatBRL(s.totalBalanceCents) : "—"}
          </p>
          <div className="mt-6 flex flex-wrap gap-x-8 gap-y-3">
            <div>
              <p className="text-xs text-app-muted">Faturas em aberto</p>
              <p className="tnum mt-0.5 text-[15px] font-medium text-app-text">
                {s ? formatBRL(s.openInvoicesCents) : "—"}
              </p>
            </div>
            <div>
              <p className="text-xs text-app-muted">Pendências</p>
              <p className="tnum mt-0.5 text-[15px] font-medium text-app-text">
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
            <ul className="divide-y divide-app-border/60 px-5 pb-2">
              {upcoming.data?.map((item) => (
                <li key={item.id} className="flex items-center gap-3 py-3">
                  <span className="flex h-9 w-9 items-center justify-center rounded-full bg-app-soft/80 text-app-muted">
                    {item.kind === "INVOICE" ? (
                      <CreditCard size={16} />
                    ) : (
                      <CalendarClock size={16} />
                    )}
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-app-text">{item.description}</p>
                    <p className="text-xs text-app-muted">vence {formatDate(item.dueDate)}</p>
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
          : "text-app-text";
  return (
    <Card className="flex flex-col justify-center px-4 py-3.5 lg:px-5">
      <p className="flex items-center gap-1.5 text-xs text-app-muted lg:text-sm">
        {icon}
        {label}
      </p>
      <p className={`tnum mt-1 text-base font-semibold lg:text-xl ${color}`}>
        {cents !== undefined ? formatBRL(cents) : "—"}
      </p>
    </Card>
  );
}

function buildSmoothPath(points: { x: number; y: number }[]): string {
  if (points.length === 0) return "";
  const first = points[0];
  if (!first) return "";
  if (points.length === 1) return `M ${first.x} ${first.y}`;
  let d = `M ${first.x} ${first.y}`;
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[i]!;
    const p1 = points[i + 1]!;
    const midX = (p0.x + p1.x) / 2;
    d += ` C ${midX} ${p0.y}, ${midX} ${p1.y}, ${p1.x} ${p1.y}`;
  }
  return d;
}

function CashflowChart({ data }: { data: CashflowPoint[] }) {
  const width = 640;
  const height = 220;
  const padLeft = 48;
  const padRight = 16;
  const padTop = 16;
  const padBottom = 28;
  const plotW = width - padLeft - padRight;
  const plotH = height - padTop - padBottom;
  const baseline = padTop + plotH;

  const maxCents = Math.max(10_000, ...data.map((p) => Math.max(p.incomeCents, p.expenseCents)));

  const xAt = (i: number): number =>
    data.length <= 1 ? padLeft + plotW / 2 : padLeft + (i / (data.length - 1)) * plotW;
  const yAt = (cents: number): number => baseline - (cents / maxCents) * plotH;

  const inPts = data.map((p, i) => ({ x: xAt(i), y: yAt(p.incomeCents) }));
  const outPts = data.map((p, i) => ({ x: xAt(i), y: yAt(p.expenseCents) }));

  const inLine = buildSmoothPath(inPts);
  const outLine = buildSmoothPath(outPts);

  const firstX = inPts[0]?.x ?? padLeft;
  const lastX = inPts[inPts.length - 1]?.x ?? padLeft + plotW;
  const inArea = inLine ? `${inLine} L ${lastX} ${baseline} L ${firstX} ${baseline} Z` : "";
  const outArea = outLine ? `${outLine} L ${lastX} ${baseline} L ${firstX} ${baseline} Z` : "";

  const yTicks = [0, 0.5, 1].map((t) => ({
    y: baseline - t * plotH,
    label: (() => {
      const reais = Math.round((maxCents * t) / 100);
      return reais >= 1000 ? `${Math.round(reais / 1000)}k` : String(reais);
    })(),
  }));

  return (
    <div className="relative h-full w-full">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="h-full w-full overflow-visible"
        role="img"
        aria-label="Fluxo de caixa do mês"
      >
        <defs>
          <linearGradient id="gIn" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--ff-income)" stopOpacity={0.28} />
            <stop offset="100%" stopColor="var(--ff-income)" stopOpacity={0} />
          </linearGradient>
          <linearGradient id="gOut" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--ff-expense)" stopOpacity={0.24} />
            <stop offset="100%" stopColor="var(--ff-expense)" stopOpacity={0} />
          </linearGradient>
        </defs>

        {yTicks.map((tick) => (
          <g key={tick.y}>
            <line
              x1={padLeft}
              y1={tick.y}
              x2={width - padRight}
              y2={tick.y}
              stroke="var(--ff-border)"
              strokeDasharray="3 3"
            />
            <text
              x={padLeft - 8}
              y={tick.y + 4}
              textAnchor="end"
              fill="var(--ff-subtle)"
              fontSize={11}
            >
              {tick.label}
            </text>
          </g>
        ))}

        {inArea ? <path d={inArea} fill="url(#gIn)" /> : null}
        {outArea ? <path d={outArea} fill="url(#gOut)" /> : null}
        {inLine ? (
          <path
            d={inLine}
            fill="none"
            stroke="var(--ff-income)"
            strokeWidth={2}
            strokeLinecap="round"
          />
        ) : null}
        {outLine ? (
          <path
            d={outLine}
            fill="none"
            stroke="var(--ff-expense)"
            strokeWidth={2}
            strokeLinecap="round"
          />
        ) : null}

        {data.map((point, i) => {
          const x = xAt(i);
          return (
            <g key={point.date}>
              <text x={x} y={height - 6} textAnchor="middle" fill="var(--ff-subtle)" fontSize={11}>
                {point.date.slice(8, 10)}
              </text>
              <circle cx={x} cy={yAt(point.incomeCents)} r={3} fill="var(--ff-income)" />
              <circle cx={x} cy={yAt(point.expenseCents)} r={3} fill="var(--ff-expense)" />
            </g>
          );
        })}
      </svg>
    </div>
  );
}

function CategoryDonut({ data }: { data: CategorySlice[] }) {
  const top = data.slice(0, 6);
  const total = data.reduce((acc, s) => acc + s.totalCents, 0);
  const radius = 70;
  const circumference = 2 * Math.PI * radius;

  let accumulated = 0;
  const segments = top.map((slice) => {
    const fraction = total > 0 ? slice.totalCents / total : 0;
    const dash = Math.max(0, fraction * circumference - 3);
    const offset = -accumulated * circumference;
    accumulated += fraction;
    return { slice, dash, offset };
  });

  return (
    <div className="flex flex-col gap-1 px-5 pb-5">
      <div className="relative mx-auto h-44 w-44" data-testid="category-donut">
        <svg
          viewBox="0 0 180 180"
          className="h-full w-full -rotate-90"
          role="img"
          aria-label="Gastos por categoria"
        >
          <circle
            cx="90"
            cy="90"
            r={radius}
            fill="none"
            stroke="var(--ff-border)"
            strokeWidth="22"
          />
          {segments.map(({ slice, dash, offset }) => (
            <circle
              key={slice.name}
              cx="90"
              cy="90"
              r={radius}
              fill="none"
              stroke={slice.color ?? "var(--ff-subtle)"}
              strokeWidth="22"
              strokeDasharray={`${dash} ${circumference}`}
              strokeDashoffset={offset}
            />
          ))}
        </svg>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-[11px] text-app-muted">Total</span>
          <span className="tnum text-sm font-semibold text-app-text">{formatBRL(total)}</span>
        </div>
      </div>
      <ul className="mt-2 space-y-2.5">
        {top.map((slice) => (
          <li key={slice.name} className="flex items-center gap-2.5">
            <CategoryIcon icon={slice.icon} color={slice.color} size={28} />
            <span className="min-w-0 flex-1 truncate text-sm text-app-text">{slice.name}</span>
            <span className="tnum text-xs text-app-muted">{slice.percent}%</span>
            <span className="tnum w-24 text-right text-sm font-medium text-app-text">
              {formatBRL(slice.totalCents)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
