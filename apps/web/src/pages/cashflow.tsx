import type { CashflowBasis, CashflowMonth, CashflowWhere } from "@finflow/contracts";
import { ArrowDownUp, ChartPie, Info } from "lucide-react";
import { useState } from "react";
import { Card, CardHeader } from "../components/ui/card";
import { CategoryIcon } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Money } from "../components/ui/money";
import { Badge, Segmented } from "../components/ui/segmented";
import { cn } from "../lib/cn";
import { formatAxisBRL, formatBRL, formatMonthShort, formatPercent } from "../lib/format";
import { useCashflowReport } from "../lib/queries";
import { useElementWidth } from "../lib/use-width";

const BASES = [
  ["CASH", "Regime de caixa"],
  ["ACCRUAL", "Competência"],
] as const;

const RANGES = [
  ["3", "3 meses"],
  ["6", "6 meses"],
  ["12", "12 meses"],
] as const;

type Range = (typeof RANGES)[number][0];

const BASIS_HINT: Record<CashflowBasis, string> = {
  CASH: "Mostra o que de fato saiu da conta em cada mês: compras no cartão contam no mês do vencimento da fatura.",
  ACCRUAL:
    "Mostra quando você assumiu o gasto: compras no cartão contam na data da compra, mesmo que a fatura vença depois.",
};

const SERIES = [
  { key: "fixedBillsCents", label: "Contas fixas", color: "var(--spend-fixed)" },
  { key: "installmentsCents", label: "Parcelas", color: "var(--spend-cuttable)" },
  { key: "dailyCents", label: "Dia a dia", color: "var(--spend-variable)" },
] as const;

export function CashflowPage() {
  const [basis, setBasis] = useState<CashflowBasis>("CASH");
  const [range, setRange] = useState<Range>("3");
  const report = useCashflowReport(Number(range), basis);
  const months = report.data?.months ?? [];
  const where = report.data?.whereItWent ?? [];
  const lastMonth = months[months.length - 1];

  const totals = months.reduce(
    (acc, m) => ({
      income: acc.income + m.incomeCents,
      out: acc.out + m.totalOutCents,
      result: acc.result + m.resultCents,
    }),
    { income: 0, out: 0, result: 0 },
  );

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Fluxo de caixa</h1>
          <p className="text-sm text-app-muted">Quanto entrou, quanto saiu e para onde foi</p>
        </div>
        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
          <Segmented
            label="Regime do fluxo de caixa"
            value={basis}
            onChange={setBasis}
            options={BASES}
            className="w-full sm:w-auto"
          />
          <Segmented
            label="Período"
            value={range}
            onChange={setRange}
            options={RANGES}
            className="w-full sm:w-auto"
          />
        </div>
      </header>

      <div
        className="flex gap-3 rounded-(--radius-card) border border-app-border/80 bg-app-surface/60 p-4"
        data-testid="cashflow-basis"
        data-basis={basis}
      >
        <Info size={18} aria-hidden className="mt-0.5 shrink-0 text-app-muted" />
        <p className="text-sm text-app-text">
          <span className="font-medium">
            {basis === "CASH" ? "Regime de caixa: " : "Competência: "}
          </span>
          {BASIS_HINT[basis]} Pagamentos de fatura e transferências entre suas contas nunca contam.
        </p>
      </div>

      {report.isLoading ? (
        <Card>
          <LoadingRows rows={5} />
        </Card>
      ) : report.isError ? (
        <Card>
          <Empty
            icon={ArrowDownUp}
            title="Não foi possível carregar o fluxo de caixa"
            hint={report.error.message}
          />
        </Card>
      ) : months.length === 0 ? (
        <Card>
          <Empty
            icon={ArrowDownUp}
            title="Sem movimentos no período"
            hint="Lance receitas e despesas ou importe uma planilha para ver o fluxo de caixa."
          />
        </Card>
      ) : (
        <>
          <div className="grid grid-cols-3 gap-3">
            <TotalTile label="Entradas" cents={totals.income} />
            <TotalTile label="Saídas" cents={-totals.out} />
            <TotalTile label="Resultado" cents={totals.result} signed />
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-12">
            <Card className="lg:col-span-7" data-testid="cashflow-chart">
              <CardHeader title="Saídas por mês" />
              <MonthsBars months={months} />
            </Card>

            <Card className="lg:col-span-5" data-testid="cashflow-where">
              <CardHeader
                title={
                  lastMonth
                    ? `Para onde foi o dinheiro em ${formatMonthShort(lastMonth.month)}`
                    : "Para onde foi o dinheiro"
                }
              />
              <WhereItWent items={where} />
            </Card>
          </div>

          <Card data-testid="cashflow-months">
            <CardHeader title="Mês a mês" />
            <ul className="divide-y divide-app-border/60 px-4 pb-2 md:px-5">
              {[...months].reverse().map((m) => (
                <MonthRow key={`${m.year}-${m.month}`} m={m} />
              ))}
            </ul>
          </Card>
        </>
      )}
    </div>
  );
}

function TotalTile({ label, cents, signed }: { label: string; cents: number; signed?: boolean }) {
  return (
    <Card className="min-w-0 px-3 py-3 sm:px-4">
      <p className="text-xs text-app-muted">{label}</p>
      <p className="tnum mt-1 truncate text-sm font-semibold sm:text-lg">
        <Money cents={cents} signed={signed} />
      </p>
    </Card>
  );
}

function MonthRow({ m }: { m: CashflowMonth }) {
  const change = m.resultChangePct;
  return (
    <li className="py-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm font-medium text-app-text">
          {formatMonthShort(m.month)} {m.year}
        </p>
        <div className="flex items-center gap-2">
          {change !== null && change !== undefined ? (
            <Badge tone={change >= 0 ? "income" : "expense"} title="Resultado vs. mês anterior">
              {change >= 0 ? "+" : "−"}
              {formatPercent(Math.abs(change))} vs. mês anterior
            </Badge>
          ) : null}
          <Money cents={m.resultCents} signed className="text-sm font-semibold" />
        </div>
      </div>
      <dl className="mt-1.5 grid grid-cols-2 gap-x-4 gap-y-1 text-xs sm:grid-cols-5">
        <Stat label="Entradas" value={formatBRL(m.incomeCents)} tone="text-income" />
        <Stat label="Contas fixas" value={formatBRL(m.fixedBillsCents)} />
        <Stat label="Parcelas" value={formatBRL(m.installmentsCents)} />
        <Stat label="Dia a dia" value={formatBRL(m.dailyCents)} />
        <Stat label="Total de saídas" value={formatBRL(m.totalOutCents)} tone="text-expense" />
      </dl>
    </li>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="flex min-w-0 justify-between gap-2 sm:block">
      <dt className="text-app-muted">{label}</dt>
      <dd className={cn("tnum truncate", tone ?? "text-app-text")}>{value}</dd>
    </div>
  );
}

/** Barras empilhadas das saídas (fixas, parcelas, dia a dia) com a entrada como marcador. */
function MonthsBars({ months }: { months: CashflowMonth[] }) {
  const [ref, width] = useElementWidth<HTMLDivElement>(480);
  const height = 220;
  const padTop = 22;
  const padBottom = 24;
  const plotH = height - padTop - padBottom;
  const max = Math.max(10_000, ...months.map((m) => Math.max(m.totalOutCents, m.incomeCents)));
  const slot = width / Math.max(1, months.length);
  const barW = Math.min(44, slot * 0.56);
  const yAt = (cents: number): number => padTop + plotH - (cents / max) * plotH;

  return (
    <div className="px-4 pb-4">
      <ul className="flex flex-wrap gap-x-4 gap-y-1 pb-2 text-xs text-app-muted">
        {SERIES.map((s) => (
          <li key={s.key} className="flex items-center gap-1.5">
            <span
              className="h-2.5 w-2.5 rounded-sm"
              style={{ backgroundColor: s.color }}
              aria-hidden
            />
            {s.label}
          </li>
        ))}
        <li className="flex items-center gap-1.5">
          <span className="h-0.5 w-3 rounded bg-(--ff-income)" aria-hidden />
          Entradas
        </li>
      </ul>
      <div ref={ref} className="w-full">
        <svg
          height={height}
          viewBox={`0 0 ${width} ${height}`}
          className="block w-full"
          role="img"
          aria-label={months
            .map(
              (m) =>
                `${formatMonthShort(m.month)}: entradas ${formatBRL(m.incomeCents)}, saídas ${formatBRL(m.totalOutCents)}`,
            )
            .join("; ")}
        >
          <line
            x1={0}
            x2={width}
            y1={padTop + plotH}
            y2={padTop + plotH}
            stroke="var(--ff-border)"
          />
          {months.map((m, i) => {
            const x = i * slot + (slot - barW) / 2;
            let y = padTop + plotH;
            return (
              <g key={`${m.year}-${m.month}`}>
                {SERIES.map((s) => {
                  const h = (Math.max(0, m[s.key]) / max) * plotH;
                  y -= h;
                  return h > 0 ? (
                    <rect key={s.key} x={x} y={y} width={barW} height={h} fill={s.color} rx={3}>
                      <title>{`${s.label}: ${formatBRL(m[s.key])}`}</title>
                    </rect>
                  ) : null;
                })}
                {m.incomeCents > 0 ? (
                  <line
                    x1={x - 4}
                    x2={x + barW + 4}
                    y1={yAt(m.incomeCents)}
                    y2={yAt(m.incomeCents)}
                    stroke="var(--ff-income)"
                    strokeWidth={2.5}
                    strokeLinecap="round"
                  >
                    <title>{`Entradas: ${formatBRL(m.incomeCents)}`}</title>
                  </line>
                ) : null}
                <text
                  x={x + barW / 2}
                  y={Math.min(y, yAt(m.incomeCents)) - 6}
                  textAnchor="middle"
                  fill="var(--ff-muted)"
                  fontSize={11}
                  className="tnum"
                >
                  {m.totalOutCents ? formatAxisBRL(m.totalOutCents) : ""}
                </text>
                <text
                  x={x + barW / 2}
                  y={height - 6}
                  textAnchor="middle"
                  fill="var(--ff-subtle)"
                  fontSize={11}
                >
                  {formatMonthShort(m.month)}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    </div>
  );
}

function WhereItWent({ items }: { items: CashflowWhere[] }) {
  if (items.length === 0) {
    return (
      <Empty
        icon={ChartPie}
        title="Sem saídas no último mês"
        hint="As categorias com mais gastos aparecem aqui."
      />
    );
  }
  return (
    <ul className="space-y-3 px-5 pt-1 pb-5">
      {items.map((item) => (
        <li key={item.categoryId ?? item.name}>
          <div className="flex items-center gap-2.5">
            <CategoryIcon icon={item.icon} color={item.color} size={28} />
            <span className="min-w-0 flex-1 truncate text-sm text-app-text">{item.name}</span>
            <span className="tnum text-xs text-app-muted">{formatPercent(item.percent)}</span>
            <span className="tnum shrink-0 text-right text-sm font-medium text-app-text">
              {formatBRL(item.totalCents)}
            </span>
          </div>
          <div className="mt-1.5 ml-[38px] h-1.5 overflow-hidden rounded-full bg-app-soft">
            <div
              className="h-full rounded-full"
              style={{
                width: `${Math.min(100, Math.max(0, item.percent))}%`,
                backgroundColor: item.color ?? "var(--ff-subtle)",
              }}
            />
          </div>
        </li>
      ))}
    </ul>
  );
}
