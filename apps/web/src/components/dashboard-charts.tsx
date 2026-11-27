import type { NetWorthPoint, Pace, SpendToday } from "@finflow/contracts";
import { Wallet } from "lucide-react";
import { cn } from "../lib/cn";
import { formatAxisBRL, formatBRL, formatMonthShort, formatSignedBRL } from "../lib/format";
import { useElementWidth } from "../lib/use-width";
import { Card, CardHeader } from "./ui/card";
import { Empty } from "./ui/empty";

// ───────────────────────── quanto posso gastar hoje (v3 §4) ─────────────────────────

const SPEND_TODAY_BASIS: Record<SpendToday["basis"], string> = {
  LIMIT: "pelo seu limite mensal",
  BALANCE: "sem deixar o saldo negativo até o fim do mês",
};

/** "Você pode gastar R$ X hoje": some quando o backend ainda não envia o campo. */
export function SpendTodayTile({ spendToday }: { spendToday: SpendToday | undefined }) {
  if (!spendToday) return null;
  const zero = spendToday.cents <= 0;
  return (
    <div
      className="mt-5 flex items-start gap-3 rounded-(--radius-control) border border-emerald-500/30 bg-emerald-500/8 px-4 py-3"
      data-testid="spend-today"
    >
      <Wallet size={18} aria-hidden className="mt-0.5 shrink-0 text-income" />
      <div className="min-w-0">
        <p className="text-sm text-app-text">
          {zero ? (
            "Hoje não sobra nada para gastar"
          ) : (
            <>
              Você pode gastar{" "}
              <span className="tnum text-lg font-semibold text-income">
                {formatBRL(spendToday.cents)}
              </span>{" "}
              hoje
            </>
          )}
        </p>
        <p className="text-xs text-app-muted">{SPEND_TODAY_BASIS[spendToday.basis]}</p>
      </div>
    </div>
  );
}

// ───────────────────────── ritmo acumulado ─────────────────────────

/** Gasto acumulado do mês vs. mês passado, com a projeção tracejada até o fim do mês. */
export function PaceChart({ pace }: { pace: Pace }) {
  const [ref, width] = useElementWidth<HTMLDivElement>(360);
  const series = pace.series ?? [];
  if (series.length === 0) return null;

  const height = 168;
  const padLeft = 40;
  const padRight = 10;
  const padTop = 10;
  const padBottom = 22;
  const plotW = Math.max(40, width - padLeft - padRight);
  const plotH = height - padTop - padBottom;
  const days = series.length;

  const current = series.filter((p) => p.currentCents !== null);
  const last = current[current.length - 1];
  const projected =
    pace.projectedMonthCents !== undefined && last && last.day < days
      ? pace.projectedMonthCents
      : null;

  const max = Math.max(
    10_000,
    ...series.map((p) => p.previousCents),
    ...current.map((p) => p.currentCents ?? 0),
    projected ?? 0,
  );
  const xAt = (day: number): number =>
    days <= 1 ? padLeft + plotW / 2 : padLeft + ((day - 1) / (days - 1)) * plotW;
  const yAt = (cents: number): number => padTop + plotH - (cents / max) * plotH;
  const line = (pts: { x: number; y: number }[]): string =>
    pts.map((p, i) => `${i === 0 ? "M" : "L"} ${p.x} ${p.y}`).join(" ");

  const prevPath = line(series.map((p) => ({ x: xAt(p.day), y: yAt(p.previousCents) })));
  const curPath = line(current.map((p) => ({ x: xAt(p.day), y: yAt(p.currentCents ?? 0) })));
  const curColor = pace.status === "ABOVE" ? "var(--ff-expense)" : "var(--ff-income)";
  const labelDays = [1, Math.ceil(days / 2), days];

  const summary = `Gasto acumulado até o dia ${last?.day ?? 0}: ${formatBRL(last?.currentCents ?? 0)}; mês passado no mesmo dia: ${formatBRL(series.find((p) => p.day === last?.day)?.previousCents ?? 0)}${projected !== null ? `; projeção para o mês: ${formatBRL(projected)}` : ""}.`;

  return (
    <div data-testid="pace-chart" className="mt-3">
      <ul className="flex flex-wrap gap-x-3 gap-y-1 pb-1 text-[11px] text-app-muted">
        <li className="flex items-center gap-1.5">
          <span className="h-0.5 w-3 rounded" style={{ backgroundColor: curColor }} aria-hidden />
          Este mês
        </li>
        <li className="flex items-center gap-1.5">
          <span className="h-0.5 w-3 rounded bg-(--ff-subtle)" aria-hidden />
          Mês passado
        </li>
        {projected !== null ? (
          <li className="flex items-center gap-1.5">
            <span
              className="w-3 border-t-2 border-dashed"
              style={{ borderColor: curColor }}
              aria-hidden
            />
            Projeção <span className="tnum text-app-text">{formatBRL(projected)}</span>
          </li>
        ) : null}
      </ul>
      <div ref={ref} className="w-full">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          height={height}
          className="block w-full"
          role="img"
          aria-label={summary}
        >
          {[0, 0.5, 1].map((t) => (
            <g key={t}>
              <line
                x1={padLeft}
                x2={padLeft + plotW}
                y1={yAt(max * t)}
                y2={yAt(max * t)}
                stroke="var(--ff-border)"
                strokeDasharray={t === 0 ? undefined : "3 3"}
              />
              <text
                x={padLeft - 6}
                y={yAt(max * t) + 4}
                textAnchor="end"
                fill="var(--ff-subtle)"
                fontSize={10}
              >
                {formatAxisBRL(max * t)}
              </text>
            </g>
          ))}
          <path
            d={prevPath}
            fill="none"
            stroke="var(--ff-subtle)"
            strokeWidth={1.5}
            strokeLinecap="round"
          />
          {curPath ? (
            <path
              d={curPath}
              fill="none"
              stroke={curColor}
              strokeWidth={2.25}
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          ) : null}
          {projected !== null && last ? (
            <path
              d={line([
                { x: xAt(last.day), y: yAt(last.currentCents ?? 0) },
                { x: xAt(days), y: yAt(projected) },
              ])}
              fill="none"
              stroke={curColor}
              strokeWidth={1.75}
              strokeDasharray="5 4"
              opacity={0.8}
            />
          ) : null}
          {last ? (
            <circle cx={xAt(last.day)} cy={yAt(last.currentCents ?? 0)} r={3.5} fill={curColor} />
          ) : null}
          {labelDays.map((day) => (
            <text
              key={day}
              x={xAt(day)}
              y={height - 6}
              textAnchor={
                day === labelDays[labelDays.length - 1]
                  ? "end"
                  : day === labelDays[0]
                    ? "start"
                    : "middle"
              }
              fill="var(--ff-subtle)"
              fontSize={10}
            >
              {`dia ${day}`}
            </text>
          ))}
        </svg>
      </div>
    </div>
  );
}

// ───────────────────────── evolução do patrimônio (v3 §12) ─────────────────────────

export function NetWorthChart({
  points,
  loading,
  className,
  style,
}: {
  points: NetWorthPoint[] | undefined;
  loading: boolean;
  className?: string;
  style?: React.CSSProperties;
}) {
  const [ref, width] = useElementWidth<HTMLDivElement>(360);
  const data = points ?? [];
  const first = data[0];
  const last = data[data.length - 1];
  const change = first && last ? last.netWorthCents - first.netWorthCents : 0;

  const height = 140;
  const padX = 8;
  const padTop = 12;
  const padBottom = 22;
  const plotW = Math.max(40, width - padX * 2);
  const plotH = height - padTop - padBottom;
  const values = data.map((p) => p.netWorthCents);
  const max = Math.max(0, ...values);
  const min = Math.min(0, ...values);
  const span = max - min || 1;
  const xAt = (i: number): number =>
    data.length <= 1 ? padX + plotW / 2 : padX + (i / (data.length - 1)) * plotW;
  const yAt = (cents: number): number => padTop + ((max - cents) / span) * plotH;
  const linePath = data
    .map((p, i) => `${i === 0 ? "M" : "L"} ${xAt(i)} ${yAt(p.netWorthCents)}`)
    .join(" ");
  const areaPath =
    data.length > 1
      ? `${linePath} L ${xAt(data.length - 1)} ${yAt(0)} L ${xAt(0)} ${yAt(0)} Z`
      : "";
  const up = change >= 0;

  return (
    <Card className={className} style={style} data-testid="networth-chart">
      <CardHeader title="Evolução do patrimônio" />
      {data.length === 0 ? (
        loading ? (
          <div className="h-36 animate-pulse px-5 pb-5" role="status" aria-label="Carregando">
            <div className="h-full rounded-(--radius-control) bg-app-soft" />
          </div>
        ) : (
          <Empty
            icon={Wallet}
            title="Sem histórico ainda"
            hint="O patrimônio de cada fim de mês (contas + investimentos − dívidas) aparece aqui."
          />
        )
      ) : (
        <div className="px-5 pb-4">
          <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
            <p className="tnum text-xl font-semibold text-app-text">
              {formatBRL(last?.netWorthCents ?? 0)}
            </p>
            <p className={cn("tnum text-xs font-medium", up ? "text-income" : "text-expense")}>
              {formatSignedBRL(change)} em {data.length} meses
            </p>
          </div>
          <div ref={ref} className="mt-2 w-full">
            <svg
              viewBox={`0 0 ${width} ${height}`}
              height={height}
              className="block w-full"
              role="img"
              aria-label={`Patrimônio líquido de ${formatBRL(first?.netWorthCents ?? 0)} para ${formatBRL(last?.netWorthCents ?? 0)} nos últimos ${data.length} meses.`}
            >
              <defs>
                <linearGradient id="gNetWorth" x1="0" y1="0" x2="0" y2="1">
                  <stop
                    offset="0%"
                    stopColor={up ? "var(--ff-income)" : "var(--ff-expense)"}
                    stopOpacity={0.25}
                  />
                  <stop
                    offset="100%"
                    stopColor={up ? "var(--ff-income)" : "var(--ff-expense)"}
                    stopOpacity={0}
                  />
                </linearGradient>
              </defs>
              {min < 0 ? (
                <line
                  x1={padX}
                  x2={padX + plotW}
                  y1={yAt(0)}
                  y2={yAt(0)}
                  stroke="var(--ff-border)"
                />
              ) : null}
              {areaPath ? <path d={areaPath} fill="url(#gNetWorth)" /> : null}
              <path
                d={linePath}
                fill="none"
                stroke={up ? "var(--ff-income)" : "var(--ff-expense)"}
                strokeWidth={2}
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              {data.map((p, i) => (
                <circle
                  key={`${p.year}-${p.month}`}
                  cx={xAt(i)}
                  cy={yAt(p.netWorthCents)}
                  r={i === data.length - 1 ? 3.5 : 2}
                  fill={up ? "var(--ff-income)" : "var(--ff-expense)"}
                >
                  <title>
                    {`${formatMonthShort(p.month)} ${p.year}: ${formatBRL(p.netWorthCents)} (contas ${formatBRL(p.cashCents)} + investimentos ${formatBRL(p.investmentsCents)} − dívidas ${formatBRL(p.debtsCents)})`}
                  </title>
                </circle>
              ))}
              {data.map((p, i) =>
                i === 0 || i === data.length - 1 || i === Math.floor((data.length - 1) / 2) ? (
                  <text
                    key={`l-${p.year}-${p.month}`}
                    x={xAt(i)}
                    y={height - 6}
                    textAnchor={i === 0 ? "start" : i === data.length - 1 ? "end" : "middle"}
                    fill="var(--ff-subtle)"
                    fontSize={10}
                  >
                    {`${formatMonthShort(p.month)}/${String(p.year).slice(2)}`}
                  </text>
                ) : null,
              )}
            </svg>
          </div>
          {last ? (
            <dl className="mt-2 grid grid-cols-3 gap-2 text-[11px]">
              <div className="min-w-0">
                <dt className="text-app-muted">Contas</dt>
                <dd className="tnum truncate text-app-text">{formatBRL(last.cashCents)}</dd>
              </div>
              <div className="min-w-0">
                <dt className="text-app-muted">Investimentos</dt>
                <dd className="tnum truncate text-app-text">{formatBRL(last.investmentsCents)}</dd>
              </div>
              <div className="min-w-0">
                <dt className="text-app-muted">Dívidas</dt>
                <dd className="tnum truncate text-expense">{formatBRL(last.debtsCents)}</dd>
              </div>
            </dl>
          ) : null}
        </div>
      )}
    </Card>
  );
}
