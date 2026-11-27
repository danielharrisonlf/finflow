import type { BusinessDre, BusinessMetrics, DreLine, DreMonth } from "@finflow/contracts";
import { Briefcase, ChartColumn } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { Card, CardHeader } from "../components/ui/card";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Money } from "../components/ui/money";
import { MonthSwitcher, usePeriodState, YearSwitcher } from "../components/ui/month-switcher";
import { cn } from "../lib/cn";
import { formatAxisBRL, formatBRL, formatMonthShort, formatPercent } from "../lib/format";
import { useBusinessDre, useBusinessMetrics } from "../lib/queries";
import { useActiveSpace } from "../lib/use-active-space";
import { useElementWidth } from "../lib/use-width";

/** Linhas do DRE na ordem de leitura; `strong` = subtotal. */
const DRE_ROWS = [
  { key: "revenueCents", label: "Receita bruta" },
  { key: "deductionsCents", label: "(−) Deduções", minus: true },
  { key: "netRevenueCents", label: "Receita líquida", strong: true },
  { key: "costsCents", label: "(−) Custos", minus: true },
  { key: "grossProfitCents", label: "Lucro bruto", strong: true },
  { key: "opexCents", label: "(−) Despesas operacionais", minus: true },
  { key: "operatingResultCents", label: "Resultado operacional", strong: true },
  { key: "financialCents", label: "(±) Resultado financeiro" },
  { key: "netResultCents", label: "Resultado líquido", strong: true },
] as const satisfies readonly {
  key: keyof DreLine;
  label: string;
  minus?: boolean;
  strong?: boolean;
}[];

const monthsFmt = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 });

/** Métricas da empresa (v3 fase 2 §5). */
export function BusinessPage() {
  const { kind } = useActiveSpace();
  if (kind !== "BUSINESS") {
    return (
      <div className="space-y-4">
        <header>
          <h1 className="text-xl font-semibold tracking-tight">Empresa</h1>
          <p className="text-sm text-app-muted">MRR, churn, runway e DRE</p>
        </header>
        <Card>
          <Empty
            icon={Briefcase}
            title="Disponível em espaços do tipo Empresa"
            hint="Crie ou abra um espaço Empresa para acompanhar receita recorrente, queima de caixa e o DRE."
            action={
              <Link
                to="/espacos"
                className="inline-flex h-11 items-center rounded-(--radius-control) border border-app-border px-4 text-sm text-app-text hover:bg-app-soft"
              >
                Ver espaços
              </Link>
            }
          />
        </Card>
      </div>
    );
  }
  return <BusinessContent />;
}

function BusinessContent() {
  const [period, shift] = usePeriodState();
  const [year, setYear] = useState(() => new Date().getFullYear());
  const metrics = useBusinessMetrics(period);
  const dre = useBusinessDre(year);

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Empresa</h1>
          <p className="text-sm text-app-muted">Receita recorrente, queima de caixa e DRE</p>
        </div>
        <MonthSwitcher period={period} onShift={shift} />
      </header>

      {metrics.isLoading ? (
        <Card>
          <LoadingRows rows={3} />
        </Card>
      ) : metrics.isError || !metrics.data ? (
        <Card>
          <Empty
            icon={Briefcase}
            title="Não foi possível calcular as métricas"
            hint={metrics.error?.message ?? "Tente novamente em instantes."}
          />
        </Card>
      ) : (
        <MetricsGrid m={metrics.data} />
      )}

      <section className="space-y-3" aria-labelledby="dre-title">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="dre-title" className="text-base font-semibold tracking-tight">
            DRE de {year}
          </h2>
          <YearSwitcher year={year} onChange={setYear} />
        </div>
        {dre.isLoading ? (
          <Card>
            <LoadingRows rows={6} />
          </Card>
        ) : !dre.data ? (
          <Card>
            <Empty
              icon={ChartColumn}
              title="Não foi possível montar o DRE"
              hint={dre.error?.message ?? "Defina o grupo do DRE nas categorias e lance receitas."}
            />
          </Card>
        ) : (
          <>
            <Card data-testid="business-chart">
              <CardHeader title="Receita líquida e resultado, mês a mês" />
              <DreBars months={dre.data.months} />
            </Card>
            <DreTable dre={dre.data} year={year} />
          </>
        )}
        <p className="text-xs text-app-muted">
          O grupo de cada categoria (receita, deduções, custos, despesas operacionais, financeiro) é
          definido em{" "}
          <Link to="/categorias" className="text-income underline-offset-2 hover:underline">
            Categorias
          </Link>
          .
        </p>
      </section>
    </div>
  );
}

function MetricsGrid({ m }: { m: BusinessMetrics }) {
  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="business-metrics">
      <Tile label="MRR" hint="Receita recorrente mensal" testId="metric-mrr">
        <Money cents={m.mrrCents} neutral />
      </Tile>
      <Tile label="ARR" hint="12 × MRR" testId="metric-arr">
        <Money cents={m.arrCents} neutral />
      </Tile>
      <Tile
        label="Churn"
        hint={`${formatBRL(m.churnedMrrCents)} de MRR perdido`}
        testId="metric-churn"
      >
        <span className={cn(m.churnPct && m.churnPct > 0 ? "text-expense" : "text-app-text")}>
          {m.churnPct === null ? "—" : formatPercent(m.churnPct)}
        </span>
      </Tile>
      <Tile label="Novo MRR" hint="Recorrências que começaram no mês" testId="metric-new-mrr">
        <Money cents={m.newMrrCents} />
      </Tile>
      <Tile label="Caixa" hint="Saldo disponível nas contas" testId="metric-cash">
        <Money cents={m.cashCents} neutral />
      </Tile>
      <Tile
        label="Burn mensal"
        hint="Média das saídas − entradas dos 3 meses anteriores"
        testId="metric-burn"
      >
        <span className={m.burnCents > 0 ? "text-expense" : "text-app-text"}>
          {formatBRL(m.burnCents)}
        </span>
      </Tile>
      <Tile
        label="Runway"
        hint={m.runwayMonths === null ? "Sem queima de caixa" : "Meses de caixa no ritmo atual"}
        testId="metric-runway"
        className="col-span-2"
      >
        <span
          className={cn(
            m.runwayMonths !== null && m.runwayMonths < 6 ? "text-warn" : "text-app-text",
          )}
        >
          {m.runwayMonths === null
            ? "Indefinido"
            : `${monthsFmt.format(m.runwayMonths)} ${m.runwayMonths === 1 ? "mês" : "meses"}`}
        </span>
      </Tile>
    </div>
  );
}

function Tile({
  label,
  hint,
  children,
  testId,
  className,
}: {
  label: string;
  hint: string;
  children: React.ReactNode;
  testId: string;
  className?: string;
}) {
  return (
    <Card className={cn("min-w-0 px-4 py-3.5", className)} data-testid={testId}>
      <p className="text-xs text-app-muted">{label}</p>
      <p className="tnum mt-1 truncate text-lg font-semibold">{children}</p>
      <p className="mt-0.5 text-[11px] leading-snug text-app-muted">{hint}</p>
    </Card>
  );
}

/** Barras agrupadas: receita líquida e resultado líquido (abaixo do zero quando negativo). */
function DreBars({ months }: { months: DreMonth[] }) {
  const [ref, width] = useElementWidth<HTMLDivElement>(480);
  const height = 220;
  const padTop = 16;
  const padBottom = 24;
  const plotH = height - padTop - padBottom;
  const max = Math.max(10_000, ...months.map((m) => Math.max(m.netRevenueCents, m.netResultCents)));
  const min = Math.min(0, ...months.map((m) => Math.min(m.netRevenueCents, m.netResultCents)));
  const range = max - min || 1;
  const yAt = (cents: number): number => padTop + ((max - cents) / range) * plotH;
  const zeroY = yAt(0);
  const slot = width / Math.max(1, months.length);
  const barW = Math.max(3, Math.min(16, slot * 0.32));

  if (months.length === 0) {
    return <Empty icon={ChartColumn} title="Sem lançamentos no ano" />;
  }

  return (
    <div className="px-4 pb-4">
      <ul className="flex flex-wrap gap-x-4 gap-y-1 pb-2 text-xs text-app-muted">
        <li className="flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-sm bg-(--ff-income) opacity-45" aria-hidden />
          Receita líquida
        </li>
        <li className="flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-sm bg-(--ff-income)" aria-hidden />
          Resultado (negativo em vermelho)
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
                `${formatMonthShort(m.month)}: receita líquida ${formatBRL(m.netRevenueCents)}, resultado ${formatBRL(m.netResultCents)}`,
            )
            .join("; ")}
        >
          <line x1={0} x2={width} y1={zeroY} y2={zeroY} stroke="var(--ff-border)" />
          {months.map((m, i) => {
            const cx = i * slot + slot / 2;
            const bar = (cents: number, x: number, fill: string, opacity = 1, title = "") => {
              const y = yAt(Math.max(0, cents));
              const h = Math.abs(yAt(cents) - zeroY);
              return h > 0 ? (
                <rect x={x} y={y} width={barW} height={h} rx={2} fill={fill} opacity={opacity}>
                  <title>{title}</title>
                </rect>
              ) : null;
            };
            return (
              <g key={m.month}>
                {bar(
                  m.netRevenueCents,
                  cx - barW - 1,
                  "var(--ff-income)",
                  0.45,
                  `Receita líquida: ${formatBRL(m.netRevenueCents)}`,
                )}
                {bar(
                  m.netResultCents,
                  cx + 1,
                  m.netResultCents >= 0 ? "var(--ff-income)" : "var(--ff-expense)",
                  1,
                  `Resultado: ${formatBRL(m.netResultCents)}`,
                )}
                <text
                  x={cx}
                  y={height - 6}
                  textAnchor="middle"
                  fill="var(--ff-subtle)"
                  fontSize={slot < 28 ? 9 : 11}
                >
                  {slot < 22 ? formatMonthShort(m.month).charAt(0) : formatMonthShort(m.month)}
                </text>
              </g>
            );
          })}
          <text x={2} y={padTop - 4} fill="var(--ff-muted)" fontSize={10} className="tnum">
            {formatAxisBRL(max)}
          </text>
        </svg>
      </div>
    </div>
  );
}

function DreTable({ dre, year }: { dre: BusinessDre; year: number }) {
  const value = (line: DreLine, key: keyof DreLine): number => Number(line[key] ?? 0);
  return (
    <Card data-testid="dre-table">
      {/* rolagem horizontal só dentro da tabela, nunca na página */}
      <div className="overflow-x-auto">
        <table className="w-full min-w-[1100px] text-sm">
          <caption className="sr-only">DRE mensal de {year}</caption>
          <thead>
            <tr className="text-left text-xs text-app-muted">
              <th scope="col" className="sticky left-0 z-10 bg-app-surface px-5 py-3 font-medium">
                Linha
              </th>
              {dre.months.map((m) => (
                <th key={m.month} scope="col" className="px-3 py-3 text-right font-medium">
                  {formatMonthShort(m.month)}
                </th>
              ))}
              <th scope="col" className="px-5 py-3 text-right font-medium">
                Total
              </th>
            </tr>
          </thead>
          <tbody className="tnum divide-y divide-app-border/60">
            {DRE_ROWS.map((row) => {
              const strong = "strong" in row && row.strong;
              return (
                <tr key={row.key} className={cn(strong && "font-semibold")}>
                  <th
                    scope="row"
                    className={cn(
                      "sticky left-0 z-10 bg-app-surface px-5 py-2.5 text-left whitespace-nowrap",
                      strong ? "text-app-text" : "font-normal text-app-muted",
                    )}
                  >
                    {row.label}
                  </th>
                  {dre.months.map((m) => (
                    <td key={m.month} className="px-3 py-2.5 text-right whitespace-nowrap">
                      <DreCell cents={value(m, row.key)} strong={strong} />
                    </td>
                  ))}
                  <td className="px-5 py-2.5 text-right whitespace-nowrap">
                    <DreCell cents={value(dre.totals, row.key)} strong />
                  </td>
                </tr>
              );
            })}
            <tr>
              <th
                scope="row"
                className="sticky left-0 z-10 bg-app-surface px-5 py-2.5 text-left font-normal whitespace-nowrap text-app-muted"
              >
                Margem líquida
              </th>
              {dre.months.map((m) => (
                <td key={m.month} className="px-3 py-2.5 text-right text-app-muted">
                  {m.marginPct === null ? "—" : formatPercent(m.marginPct)}
                </td>
              ))}
              <td className="px-5 py-2.5 text-right font-semibold text-app-text">
                {dre.totals.marginPct === null ? "—" : formatPercent(dre.totals.marginPct)}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </Card>
  );
}

function DreCell({ cents, strong }: { cents: number; strong?: boolean }) {
  if (strong) return <Money cents={cents} />;
  return <span className="text-app-text">{formatBRL(cents)}</span>;
}
