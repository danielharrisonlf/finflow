import type { Projection, ProjectionDay } from "@finflow/contracts";
import {
  AlertTriangle,
  CalendarClock,
  CircleCheck,
  Info,
  PiggyBank,
  ShoppingBag,
  TrendingUp,
} from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { CanAffordForm } from "../components/can-afford";
import { Card, CardHeader } from "../components/ui/card";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Money } from "../components/ui/money";
import { Segmented } from "../components/ui/segmented";
import { BAND_META, BAND_ORDER } from "../lib/bands";
import { cn } from "../lib/cn";
import {
  formatAxisBRL,
  formatBRL,
  formatDate,
  formatLongDate,
  formatMonthShort,
  formatSignedBRL,
  formatWeekdayDate,
} from "../lib/format";
import { useProjection, useSettings } from "../lib/queries";
import { useElementWidth } from "../lib/use-width";

const HORIZONS = [
  ["30", "30 dias"],
  ["90", "90 dias"],
  ["180", "6 meses"],
] as const;

type Horizon = (typeof HORIZONS)[number][0];

export function ProjectionPage() {
  const [horizon, setHorizon] = useState<Horizon>("90");
  const projection = useProjection(Number(horizon));
  const p = projection.data;

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Projeção</h1>
          <p className="text-sm text-app-muted">
            Seu saldo dia a dia, somando contas fixas, faturas e pendências
          </p>
        </div>
        <Segmented
          label="Horizonte da projeção"
          value={horizon}
          onChange={setHorizon}
          options={HORIZONS}
          className="w-full sm:w-auto"
        />
      </header>

      {projection.isLoading ? (
        <Card>
          <LoadingRows rows={5} />
        </Card>
      ) : !p ? (
        <Card>
          <Empty
            icon={TrendingUp}
            title="Não foi possível calcular a projeção"
            hint={projection.error?.message ?? "Tente novamente em instantes."}
          />
        </Card>
      ) : (
        <ProjectionContent p={p} />
      )}
    </div>
  );
}

function ProjectionContent({ p }: { p: Projection }) {
  const negative = p.lowest.balanceCents < 0;
  const lowestBand = p.days.find((d) => d.date === p.lowest.date)?.band;

  return (
    <>
      {/* alerta do menor saldo — a pergunta mais importante da página */}
      <section
        data-testid="lowest-alert"
        role={negative ? "alert" : undefined}
        className={cn(
          "flex gap-3 rounded-(--radius-card) border p-4",
          negative ? "border-rose-500/40 bg-rose-500/8" : "border-app-border/80 bg-app-surface/60",
        )}
      >
        {negative ? (
          <AlertTriangle size={20} aria-hidden className="mt-0.5 shrink-0 text-expense" />
        ) : (
          <CircleCheck size={20} aria-hidden className="mt-0.5 shrink-0 text-income" />
        )}
        <div className="min-w-0">
          <p className="text-sm font-semibold text-app-text">
            {negative
              ? "Seu saldo vai ficar negativo nos próximos 30 dias"
              : "Seu saldo fica positivo nos próximos 30 dias"}
          </p>
          <p className="mt-0.5 text-sm text-app-muted">
            Menor saldo previsto: <Money cents={p.lowest.balanceCents} className="font-semibold" />{" "}
            em {formatLongDate(p.lowest.date)}
            {lowestBand ? ` · faixa “${BAND_META[lowestBand].label}”` : ""}
          </p>
        </div>
      </section>

      {!p.limitConfigured ? <LimitNotice /> : null}

      <div className="grid gap-3 sm:grid-cols-3">
        <Card className="p-4" data-testid="can-save">
          <p className="flex items-center gap-1.5 text-sm text-app-muted">
            <PiggyBank size={15} aria-hidden /> Quanto posso guardar agora
          </p>
          <p className="tnum mt-1 text-2xl font-semibold text-income">
            {formatBRL(p.canSaveNowCents)}
          </p>
          <p className="mt-1 text-xs text-app-muted">
            {p.canSaveNowCents > 0
              ? "Guardando isso hoje, o saldo continua positivo até a próxima entrada."
              : "Por enquanto, segure o dinheiro: o saldo previsto não deixa folga até a próxima entrada."}
          </p>
        </Card>
        <Card className="p-4">
          <p className="flex items-center gap-1.5 text-sm text-app-muted">
            <CalendarClock size={15} aria-hidden /> Próxima entrada
          </p>
          <p className="mt-1 text-lg font-semibold text-app-text">
            {p.nextIncomeDate ? formatLongDate(p.nextIncomeDate) : "Nenhuma prevista"}
          </p>
          {!p.nextIncomeDate ? (
            <Link
              to="/compromissos"
              className="mt-1 inline-flex min-h-11 items-center text-xs text-income hover:underline"
            >
              Cadastre seu salário como receita recorrente
            </Link>
          ) : null}
        </Card>
        <Card className="p-4">
          <p className="text-sm text-app-muted">Saldo disponível hoje</p>
          <p className="tnum mt-1 text-lg font-semibold text-app-text">
            {formatBRL(p.startBalanceCents)}
          </p>
          <p className="mt-1 text-xs text-app-muted">
            Contas do dia a dia, sem poupança e investimentos
          </p>
        </Card>
      </div>

      <Card className="grid gap-4 p-5 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] md:gap-8">
        <div>
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-app-text">
            <ShoppingBag size={15} aria-hidden /> Posso comprar?
          </h2>
          <p className="mt-1 text-sm text-app-muted">
            Simule uma compra à vista ou parcelada no cartão e veja se o saldo continua positivo,
            quanto do limite mensal ela consome e a partir de quando cabe.
          </p>
        </div>
        <CanAffordForm id="posso-comprar" />
      </Card>

      <Card>
        <CardHeader title="Saldo projetado dia a dia" />
        <BandLegend />
        <div className="px-2 pb-3" data-testid="projection-chart">
          {p.days.length > 0 ? (
            <BalanceChart days={p.days} lowestDate={p.lowest.date} />
          ) : (
            <Empty icon={TrendingUp} title="Sem dias para projetar" />
          )}
        </div>
      </Card>

      {negative ? <ActionPlan /> : null}

      <MonthsTable p={p} />
      <DailyDetails days={p.days} />
    </>
  );
}

function LimitNotice() {
  return (
    <div
      className="flex gap-3 rounded-(--radius-card) border border-amber-400/40 bg-amber-400/8 p-4"
      data-testid="limit-notice"
    >
      <Info size={18} aria-hidden className="mt-0.5 shrink-0 text-warn" />
      <p className="text-sm text-app-text">
        Sem limite mensal configurado, a projeção considera só contas fixas, faturas e pendências e
        não estima os gastos do dia a dia, então o saldo real tende a ficar menor.{" "}
        <Link
          to="/configuracoes"
          className="font-medium text-income underline-offset-4 hover:underline"
        >
          Definir limite mensal
        </Link>
      </p>
    </div>
  );
}

function BandLegend() {
  const settings = useSettings();
  const b = settings.data?.balanceBands;
  const ranges: Record<string, string> = b
    ? {
        DARK_GREEN: `acima de ${formatBRL(b.darkGreen)}`,
        LIGHT_GREEN: `${formatBRL(b.lightGreen)} a ${formatBRL(b.darkGreen)}`,
        YELLOW: `${formatBRL(b.yellow)} a ${formatBRL(b.lightGreen)}`,
        LIGHT_RED: `${formatBRL(b.lightRed)} a ${formatBRL(b.yellow)}`,
        RED: `abaixo de ${formatBRL(b.lightRed)}`,
      }
    : {};
  return (
    <ul className="flex flex-wrap gap-x-4 gap-y-1.5 px-5 pt-1 pb-2" aria-label="Faixas de saldo">
      {BAND_ORDER.map((band) => (
        <li key={band} className="flex items-center gap-1.5 text-xs text-app-muted">
          <span
            className="h-2.5 w-2.5 rounded-sm"
            style={{ backgroundColor: BAND_META[band].color }}
            aria-hidden
          />
          <span className="font-medium text-app-text">{BAND_META[band].label}</span>
          {ranges[band] ? <span className="tnum">{ranges[band]}</span> : null}
        </li>
      ))}
    </ul>
  );
}

/** Barras diárias a partir do zero, coloridas pela faixa (SVG nativo, sem dependência). */
function BalanceChart({ days, lowestDate }: { days: ProjectionDay[]; lowestDate: string }) {
  const [ref, width] = useElementWidth<HTMLDivElement>(720);
  const height = width < 480 ? 200 : 260;
  const padLeft = 52;
  const padRight = 12;
  const padTop = 14;
  const padBottom = 26;
  const plotW = width - padLeft - padRight;
  const plotH = height - padTop - padBottom;

  const values = days.map((d) => d.balanceCents);
  const max = Math.max(10_000, ...values);
  const min = Math.min(0, ...values);
  const span = max - min || 1;
  const yAt = (cents: number): number => padTop + ((max - cents) / span) * plotH;
  const zeroY = yAt(0);
  const slot = plotW / days.length;
  const barW = Math.max(1.5, slot * 0.72);

  const ticks = Array.from(new Set([max, min < 0 ? min : max / 2, 0]));
  const labelEvery = Math.ceil(days.length / Math.max(3, Math.floor(plotW / 80)));
  const lowest = days.find((d) => d.date === lowestDate);
  const lowestIndex = lowest ? days.indexOf(lowest) : -1;

  return (
    <div ref={ref} className="w-full">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        height={height}
        className="block w-full"
        role="img"
        aria-label={`Saldo projetado de ${formatDate(days[0]!.date)} a ${formatDate(days[days.length - 1]!.date)}. Menor saldo ${formatBRL(lowest?.balanceCents ?? 0)} em ${formatDate(lowestDate)}.`}
      >
        {ticks.map((t) => (
          <g key={t}>
            <line
              x1={padLeft}
              x2={width - padRight}
              y1={yAt(t)}
              y2={yAt(t)}
              stroke="var(--ff-border)"
              strokeDasharray={t === 0 ? undefined : "3 3"}
            />
            <text
              x={padLeft - 8}
              y={yAt(t) + 4}
              textAnchor="end"
              fill="var(--ff-subtle)"
              fontSize={11}
            >
              {formatAxisBRL(t)}
            </text>
          </g>
        ))}
        {days.map((d, i) => {
          const x = padLeft + i * slot + (slot - barW) / 2;
          const y = Math.min(yAt(d.balanceCents), zeroY);
          const h = Math.max(1, Math.abs(yAt(d.balanceCents) - zeroY));
          return (
            <rect
              key={d.date}
              x={x}
              y={y}
              width={barW}
              height={h}
              rx={Math.min(2, barW / 2)}
              fill={BAND_META[d.band].color}
              data-band={d.band}
            >
              <title>{`${formatDate(d.date)}: ${formatBRL(d.balanceCents)} (${BAND_META[d.band].label})`}</title>
            </rect>
          );
        })}
        {lowestIndex >= 0 && lowest ? (
          <circle
            cx={padLeft + lowestIndex * slot + slot / 2}
            cy={yAt(lowest.balanceCents) + (lowest.balanceCents < 0 ? 6 : -6)}
            r={4}
            fill="var(--ff-surface)"
            stroke="var(--ff-text)"
            strokeWidth={1.5}
          />
        ) : null}
        {days.map((d, i) =>
          i % labelEvery === 0 ? (
            <text
              key={`l-${d.date}`}
              x={padLeft + i * slot + slot / 2}
              y={height - 8}
              textAnchor="middle"
              fill="var(--ff-subtle)"
              fontSize={11}
            >
              {formatDate(d.date)}
            </text>
          ) : null,
        )}
      </svg>
    </div>
  );
}

const ACTION_STEPS = [
  {
    title: "Encontre o buraco",
    text: "Veja na lista dia a dia abaixo em que data o saldo fica negativo e quanto falta.",
  },
  {
    title: "Corte despesas ou busque uma renda extra",
    text: "Comece pelo que é cortável (assinaturas, serviços) e adie compras que podem esperar.",
  },
  {
    title: "Use a reserva de emergência",
    text: "É para isso que ela existe. Depois, reponha com um pouco a mais (+10%).",
  },
  {
    title: "Priorize o essencial",
    text: "Moradia, contas básicas, alimentação, saúde e transporte vêm primeiro.",
  },
  {
    title: "Renegocie as dívidas mais caras primeiro",
    text: "Cartão rotativo e cheque especial têm os maiores juros: troque por crédito mais barato.",
  },
];

function ActionPlan() {
  return (
    <Card className="p-5" data-testid="action-plan">
      <h2 className="text-sm font-semibold text-app-text">Plano de ação</h2>
      <p className="mt-0.5 text-sm text-app-muted">Siga nesta ordem para não entrar no vermelho.</p>
      <ol className="mt-4 space-y-3">
        {ACTION_STEPS.map((step, i) => (
          <li key={step.title} className="flex gap-3">
            <span className="tnum flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-app-soft text-sm font-semibold text-app-text">
              {i + 1}
            </span>
            <div className="min-w-0">
              <p className="text-sm font-medium text-app-text">{step.title}</p>
              <p className="text-sm text-app-muted">{step.text}</p>
            </div>
          </li>
        ))}
      </ol>
    </Card>
  );
}

function MonthsTable({ p }: { p: Projection }) {
  return (
    <Card data-testid="months-table">
      <CardHeader title="Próximos 12 meses" />
      <p className="px-5 text-xs text-app-muted">
        Parcelas já estão dentro das faturas; a coluna mostra quanto delas é parcelamento.
      </p>
      <div className="mt-2 overflow-x-auto pb-2">
        <table className="w-full min-w-[640px] text-sm">
          <thead>
            <tr className="text-left text-xs text-app-muted">
              <th scope="col" className="px-5 py-2 font-medium">
                Mês
              </th>
              <th scope="col" className="px-3 py-2 text-right font-medium">
                Entradas
              </th>
              <th scope="col" className="px-3 py-2 text-right font-medium">
                Contas fixas
              </th>
              <th scope="col" className="px-3 py-2 text-right font-medium">
                Faturas
              </th>
              <th scope="col" className="px-3 py-2 text-right font-medium">
                Parcelas
              </th>
              <th scope="col" className="px-3 py-2 text-right font-medium">
                Dia a dia
              </th>
              <th scope="col" className="px-5 py-2 text-right font-medium">
                Saldo final
              </th>
            </tr>
          </thead>
          <tbody className="tnum divide-y divide-app-border/60">
            {p.months.map((m) => (
              <tr key={`${m.year}-${m.month}`}>
                <th scope="row" className="px-5 py-2.5 text-left font-medium text-app-text">
                  {formatMonthShort(m.month)} {m.year}
                </th>
                <td className="px-3 py-2.5 text-right text-income">{formatBRL(m.incomeCents)}</td>
                <td className="px-3 py-2.5 text-right text-app-text">
                  {formatBRL(m.recurringOutCents)}
                </td>
                <td className="px-3 py-2.5 text-right text-app-text">
                  {formatBRL(m.invoicesCents)}
                </td>
                <td className="px-3 py-2.5 text-right text-app-muted">
                  {formatBRL(m.installmentsCents)}
                </td>
                <td className="px-3 py-2.5 text-right text-app-text">
                  {formatBRL(m.dailySpendCents)}
                </td>
                <td className="px-5 py-2.5 text-right font-semibold">
                  <Money cents={m.endBalanceCents} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

function DailyDetails({ days }: { days: ProjectionDay[] }) {
  const withEvents = days.filter((d) => d.events.length > 0);
  return (
    <Card>
      <details className="group">
        <summary className="flex min-h-12 cursor-pointer list-none items-center justify-between px-5 text-sm font-medium text-app-text">
          Ver dia a dia ({withEvents.length} {withEvents.length === 1 ? "dia" : "dias"} com
          movimento)
          <span className="text-app-muted group-open:rotate-45" aria-hidden>
            +
          </span>
        </summary>
        <ul className="divide-y divide-app-border/60 px-5 pb-3">
          {withEvents.map((d) => (
            <li key={d.date} className="py-3">
              <div className="flex items-center justify-between gap-2">
                <p className="flex items-center gap-2 text-sm font-medium text-app-text">
                  <span
                    className="h-2.5 w-2.5 rounded-sm"
                    style={{ backgroundColor: BAND_META[d.band].color }}
                    aria-hidden
                  />
                  {formatWeekdayDate(d.date)}
                </p>
                <p className="tnum text-sm">
                  <span className="sr-only">Saldo </span>
                  <Money cents={d.balanceCents} className="font-semibold" />
                </p>
              </div>
              <ul className="mt-1 space-y-0.5 pl-4.5">
                {d.events.map((e) => (
                  <li
                    key={`${d.date}-${e.kind}-${e.description}-${e.amountCents}`}
                    className="flex justify-between gap-3 text-xs text-app-muted"
                  >
                    <span className="min-w-0 truncate">{e.description}</span>
                    <span
                      className={cn(
                        "tnum shrink-0",
                        e.amountCents < 0 ? "text-expense" : "text-income",
                      )}
                    >
                      {formatSignedBRL(e.amountCents)}
                    </span>
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
      </details>
    </Card>
  );
}
