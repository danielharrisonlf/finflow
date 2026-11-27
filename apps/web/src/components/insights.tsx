import type {
  CutSuggestion,
  InsightsOverview,
  MonthlyLimit,
  Pace,
  Performance,
  PerformanceVerdict,
  ReserveInsight,
  SpendPlan,
  SpendTypeBreakdown,
} from "@finflow/contracts";
import {
  AlertTriangle,
  ArrowDownRight,
  ArrowUpRight,
  Equal,
  Gauge,
  type LucideIcon,
  PartyPopper,
  Scissors,
  ShieldCheck,
  ShieldPlus,
  ThumbsUp,
  TrendingUp,
} from "lucide-react";
import { Link } from "react-router-dom";
import { SPEND_TYPE_META, SPEND_TYPE_ORDER } from "../lib/bands";
import { cn } from "../lib/cn";
import { formatBRL, formatPercent, formatSignedBRL } from "../lib/format";
import { useCreateGoal } from "../lib/queries";
import { Button } from "./ui/button";
import { Card, CardHeader } from "./ui/card";
import { Progress } from "./ui/progress";
import { Badge } from "./ui/segmented";

const linkCls =
  "inline-flex min-h-11 items-center text-sm font-medium text-income underline-offset-4 hover:underline";

// ───────────────────────── ritmo do mês ─────────────────────────

function paceSentence(pace: Pace): { text: string; tone: "income" | "expense" | "neutral" } {
  if (pace.previousSamePeriodCents === 0) {
    return pace.currentCents === 0
      ? { text: "Nenhum gasto até agora neste mês.", tone: "neutral" }
      : {
          text: "Ainda não há gastos do mês passado para comparar o ritmo.",
          tone: "neutral",
        };
  }
  const pct = Math.round((Math.abs(pace.diffCents) / pace.previousSamePeriodCents) * 100);
  if (pace.status === "SAME") {
    return { text: "Você está gastando no mesmo ritmo do mês passado.", tone: "neutral" };
  }
  return pace.status === "ABOVE"
    ? {
        text: `Você está gastando ${pct}% a mais que no mesmo período do mês passado.`,
        tone: "expense",
      }
    : {
        text: `Você está gastando ${pct}% a menos que no mesmo período do mês passado.`,
        tone: "income",
      };
}

export function PaceCard({ pace }: { pace: Pace }) {
  const { text, tone } = paceSentence(pace);
  const Icon =
    pace.status === "ABOVE" ? ArrowUpRight : pace.status === "BELOW" ? ArrowDownRight : Equal;
  return (
    <div data-testid="pace-card">
      <p className="flex items-center gap-1.5 text-sm text-app-muted">
        <Gauge size={15} aria-hidden /> Ritmo do mês
      </p>
      <p className="mt-2 flex items-start gap-2 text-[15px] leading-snug text-app-text">
        <Icon
          size={18}
          aria-hidden
          className={cn(
            "mt-0.5 shrink-0",
            tone === "expense"
              ? "text-expense"
              : tone === "income"
                ? "text-income"
                : "text-app-muted",
          )}
        />
        <span>{text}</span>
      </p>
      <p className="tnum mt-1.5 text-xs text-app-muted">
        Até o dia {pace.dayOfMonth}: {formatBRL(pace.currentCents)} agora vs.{" "}
        {formatBRL(pace.previousSamePeriodCents)} no mês passado
      </p>
    </div>
  );
}

// ───────────────────────── limite mensal ─────────────────────────

export function LimitCard({ limit }: { limit: MonthlyLimit }) {
  if (limit.limitCents === null) {
    return (
      <div data-testid="limit-card">
        <p className="text-sm text-app-muted">Limite mensal de gastos</p>
        <p className="mt-2 text-sm text-app-text">
          Defina quanto quer gastar por mês e o FinFlow calcula quanto ainda pode sair por dia.
        </p>
        <Link to="/configuracoes" className={linkCls}>
          Definir limite mensal
        </Link>
      </div>
    );
  }
  const percent = limit.percent ?? 0;
  const tone = percent >= 100 ? "DANGER" : percent >= 80 ? "WARNING" : "OK";
  const over = (limit.remainingCents ?? 0) < 0;
  return (
    <div data-testid="limit-card">
      <div className="flex items-baseline justify-between gap-2">
        <p className="text-sm text-app-muted">Limite mensal de gastos</p>
        <p className="tnum text-xs text-app-muted">{formatPercent(percent)}</p>
      </div>
      <p className="tnum mt-1.5 text-sm text-app-muted">
        <span className="text-lg font-semibold text-app-text">{formatBRL(limit.spentCents)}</span>{" "}
        de {formatBRL(limit.limitCents)}
      </p>
      <Progress percent={percent} tone={tone} className="mt-2 h-2" />
      <p className={cn("mt-2 text-sm", over ? "text-expense" : "text-app-text")}>
        {over ? (
          <>
            Limite estourado em{" "}
            <span className="tnum font-medium">
              {formatBRL(Math.abs(limit.remainingCents ?? 0))}
            </span>
          </>
        ) : limit.dailyAllowanceCents !== null ? (
          <>
            Você pode gastar{" "}
            <span className="tnum font-semibold text-income">
              {formatBRL(limit.dailyAllowanceCents)} por dia
            </span>{" "}
            até o fim do mês
          </>
        ) : (
          <>
            Sobrou <span className="tnum font-medium">{formatBRL(limit.remainingCents ?? 0)}</span>{" "}
            do limite
          </>
        )}
      </p>
    </div>
  );
}

// ───────────────────────── performance (método da planilha) ─────────────────────────

const VERDICT: Record<
  PerformanceVerdict,
  { title: string; icon: LucideIcon; tone: "income" | "warn" | "expense" }
> = {
  SAVING: { title: "Mês saudável: sobrou e você guardou", icon: PartyPopper, tone: "income" },
  POSITIVE: { title: "Mês positivo: entrou mais do que saiu", icon: ThumbsUp, tone: "income" },
  TIGHT: { title: "Mês apertado: sobrou pouco", icon: AlertTriangle, tone: "warn" },
  NEGATIVE_SAVED: {
    title: "Tudo certo: a performance ficou negativa porque você guardou dinheiro",
    icon: ShieldCheck,
    tone: "income",
  },
  NEGATIVE: {
    title: "Atenção: você gastou mais do que entrou e não guardou nada",
    icon: AlertTriangle,
    tone: "expense",
  },
};

const verdictBox = {
  income: "border-emerald-500/30 bg-emerald-500/8",
  warn: "border-amber-400/40 bg-amber-400/8",
  expense: "border-rose-500/40 bg-rose-500/8",
} as const;

const verdictText = { income: "text-income", warn: "text-warn", expense: "text-expense" } as const;

function Metric({
  label,
  hint,
  value,
  className,
}: {
  label: string;
  hint: string;
  value: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "min-w-0 rounded-(--radius-control) bg-app-background/60 px-3 py-2.5",
        className,
      )}
    >
      <p className="text-xs font-medium text-app-text">{label}</p>
      <p className="text-[11px] leading-tight text-app-muted">{hint}</p>
      <p className="tnum mt-1.5 truncate text-base font-semibold">{value}</p>
    </div>
  );
}

export function PerformanceCard({ performance: p }: { performance: Performance }) {
  const verdict = VERDICT[p.verdict];
  const Icon = verdict.icon;
  const prevDiff = p.performanceCents - p.previousPerformanceCents;
  return (
    <Card className="flex flex-col gap-4 p-5" data-testid="performance-card">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-sm font-medium text-app-muted">Performance do mês</h2>
        <p className="text-xs text-app-muted">Entradas − (Saídas + Diário)</p>
      </div>

      <div
        className={cn(
          "flex gap-3 rounded-(--radius-control) border p-3.5",
          verdictBox[verdict.tone],
        )}
        data-testid="performance-verdict"
        data-verdict={p.verdict}
        role={p.verdict === "NEGATIVE" ? "alert" : undefined}
      >
        <Icon size={20} aria-hidden className={cn("mt-0.5 shrink-0", verdictText[verdict.tone])} />
        <div className="min-w-0">
          <p className={cn("text-sm font-semibold", verdictText[verdict.tone])}>{verdict.title}</p>
          {p.message ? <p className="mt-0.5 text-sm text-app-text">{p.message}</p> : null}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Metric
          label="Entradas"
          hint="receitas do mês"
          value={<span className="text-income">{formatBRL(p.incomeCents)}</span>}
        />
        <Metric
          label="Saídas"
          hint="custo de vida fixo"
          value={<span className="text-app-text">{formatBRL(p.fixedCents)}</span>}
        />
        <Metric
          label="Diário"
          hint="gastos variáveis"
          value={<span className="text-app-text">{formatBRL(p.variableCents)}</span>}
        />
        <Metric
          label="Performance"
          hint="o que sobrou"
          value={
            <span className={p.performanceCents < 0 ? "text-expense" : "text-income"}>
              {formatSignedBRL(p.performanceCents)}
            </span>
          }
        />
      </div>

      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 border-t border-app-border/60 pt-3 text-sm">
        <p className="text-app-muted">
          Economia{" "}
          <span className="tnum font-semibold text-app-text">{formatBRL(p.savedCents)}</span>
        </p>
        <p className="text-app-muted">
          Taxa de economia{" "}
          <span className="tnum font-semibold text-app-text">
            {p.savingsRatePct === null ? "—" : formatPercent(p.savingsRatePct)}
          </span>
        </p>
        <p className="text-app-muted">
          vs. mês passado{" "}
          <span className={cn("tnum font-medium", prevDiff < 0 ? "text-expense" : "text-income")}>
            {formatSignedBRL(prevDiff)}
          </span>
        </p>
      </div>
    </Card>
  );
}

// ───────────────────────── tipo de gasto ─────────────────────────

export function SpendTypeCard({
  spendTypes,
  plan,
}: {
  spendTypes: SpendTypeBreakdown[];
  plan: SpendPlan | null;
}) {
  const total = spendTypes.reduce((acc, s) => acc + s.totalCents, 0);
  const planPct = (type: SpendTypeBreakdown["spendType"]): number | null =>
    plan
      ? type === "FIXED"
        ? plan.fixedPct
        : type === "CUTTABLE"
          ? plan.cuttablePct
          : plan.variablePct
      : null;

  return (
    <Card data-testid="spendtype-card">
      <CardHeader
        title="Fixo, cortável e variável"
        action={
          plan ? null : (
            <Link to="/configuracoes" className="text-xs text-income hover:underline">
              Definir meu plano
            </Link>
          )
        }
      />
      {total === 0 ? (
        <p className="px-5 pb-5 text-sm text-app-muted">Sem despesas no período.</p>
      ) : (
        <ul className="space-y-4 px-5 pt-2 pb-5">
          {SPEND_TYPE_ORDER.map((type) => {
            const row = spendTypes.find((s) => s.spendType === type);
            const meta = SPEND_TYPE_META[type];
            const percent = row?.percent ?? 0;
            const target = planPct(type);
            const delta = target === null ? null : percent - target;
            return (
              <li key={type}>
                <div className="flex items-baseline justify-between gap-2">
                  <p className="flex items-center gap-2 text-sm font-medium text-app-text">
                    <span
                      className="h-2.5 w-2.5 rounded-full"
                      style={{ backgroundColor: meta.color }}
                      aria-hidden
                    />
                    {meta.label}
                  </p>
                  <p className="tnum text-sm text-app-text">
                    {formatBRL(row?.totalCents ?? 0)}{" "}
                    <span className="text-xs text-app-muted">{formatPercent(percent)}</span>
                  </p>
                </div>
                <div className="relative mt-1.5 h-2 w-full overflow-hidden rounded-full bg-app-soft">
                  <div
                    className="h-full rounded-full"
                    style={{ width: `${Math.min(100, percent)}%`, backgroundColor: meta.color }}
                  />
                  {target !== null ? (
                    <span
                      className="absolute inset-y-0 w-0.5 bg-app-text"
                      style={{ left: `calc(${Math.min(100, target)}% - 1px)` }}
                      aria-hidden
                    />
                  ) : null}
                </div>
                <p className="mt-1 text-xs text-app-muted">
                  {delta === null ? (
                    meta.hint
                  ) : Math.abs(delta) < 1 ? (
                    <>No plano ({formatPercent(target ?? 0)})</>
                  ) : (
                    <>
                      Plano {formatPercent(target ?? 0)} ·{" "}
                      <span className={delta > 0 ? "text-expense" : "text-income"}>
                        {delta > 0
                          ? `${formatPercent(delta)} acima`
                          : `${formatPercent(-delta)} abaixo`}
                      </span>
                    </>
                  )}
                </p>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}

// ───────────────────────── onde dá para cortar ─────────────────────────

const CUT_META = {
  CANCEL: { label: "Dá para cancelar", tone: "warn" },
  REDUCE: { label: "Dá para reduzir", tone: "neutral" },
  ABOVE_AVERAGE: { label: "Acima da média", tone: "expense" },
} as const;

export function CutSuggestionsCard({ items }: { items: CutSuggestion[] }) {
  return (
    <Card data-testid="cut-card">
      <CardHeader
        title={
          <span className="flex items-center gap-1.5">
            <Scissors size={14} aria-hidden /> Onde dá para cortar
          </span>
        }
      />
      {items.length === 0 ? (
        <p className="px-5 pb-5 text-sm text-app-muted">
          Nada fora do comum este mês. Marque assinaturas como “Cortável” em Categorias para vê-las
          aqui.
        </p>
      ) : (
        <ul className="divide-y divide-app-border/60 px-5 pb-2">
          {items.map((item) => {
            const meta = CUT_META[item.kind];
            return (
              <li
                key={`${item.kind}-${item.categoryId ?? item.name}`}
                className="flex items-center gap-3 py-3"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm text-app-text">{item.name}</p>
                  <p className="tnum truncate text-xs text-app-muted">
                    média {formatBRL(item.averageCents)}/mês
                  </p>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1">
                  <span className="tnum text-sm font-medium text-app-text">
                    {formatBRL(item.currentCents)}
                  </span>
                  <Badge tone={meta.tone}>{meta.label}</Badge>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}

// ───────────────────────── reserva de emergência ─────────────────────────

const monthsFmt = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 });

export function formatMonths(value: number): string {
  return `${monthsFmt.format(value)} ${value >= 1 && value < 2 ? "mês" : "meses"}`;
}

export function ReserveCard({ reserve }: { reserve: ReserveInsight }) {
  const create = useCreateGoal();
  const percent = reserve.targetCents > 0 ? (reserve.savedCents / reserve.targetCents) * 100 : 0;

  return (
    <Card className="flex flex-col gap-3 p-5" data-testid="reserve-card">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-medium text-app-muted">Reserva de emergência</h2>
        {reserve.status === "PROTECTED" ? (
          <Badge tone="income">
            <ShieldCheck size={12} aria-hidden /> Protegido
          </Badge>
        ) : null}
      </div>

      {reserve.status === "NONE" ? (
        <>
          <p className="text-sm text-app-text">
            Você ainda não tem uma reserva. O ideal é guardar o equivalente a alguns meses do seu
            custo de vida para imprevistos.
          </p>
          {reserve.suggestedTargetCents > 0 ? (
            <p className="text-sm text-app-muted">
              Meta sugerida:{" "}
              <span className="tnum font-semibold text-app-text">
                {formatBRL(reserve.suggestedTargetCents)}
              </span>{" "}
              (custo médio de {formatBRL(reserve.avgMonthlyCostCents)}/mês)
            </p>
          ) : null}
          {create.error ? <p className="text-sm text-expense">{create.error.message}</p> : null}
          {reserve.suggestedTargetCents > 0 ? (
            <Button
              variant="outline"
              disabled={create.isPending || create.isSuccess}
              onClick={() =>
                create.mutate({
                  name: "Reserva de emergência",
                  icon: "shield",
                  targetCents: reserve.suggestedTargetCents,
                  kind: "RESERVE",
                })
              }
            >
              <ShieldPlus size={16} aria-hidden />
              {create.isPending ? "Criando…" : "Criar minha reserva"}
            </Button>
          ) : (
            <Link to="/metas" className={linkCls}>
              Criar reserva em Metas
            </Link>
          )}
        </>
      ) : (
        <>
          <p className="tnum text-sm text-app-muted">
            <span className="text-2xl font-semibold text-app-text">
              {formatBRL(reserve.savedCents)}
            </span>{" "}
            de {formatBRL(reserve.targetCents)}
          </p>
          <Progress
            percent={percent}
            tone={reserve.status === "PROTECTED" ? "OK" : "NEUTRAL"}
            className="h-2"
          />
          <p className="text-sm text-app-text">
            Cobre <span className="tnum font-semibold">{formatMonths(reserve.monthsCovered)}</span>{" "}
            do seu custo de vida
          </p>
          <Link to="/metas" className={linkCls}>
            Ver reserva
          </Link>
        </>
      )}
    </Card>
  );
}

// ───────────────────────── patrimônio ─────────────────────────

export function NetWorth({ overview }: { overview: InsightsOverview | undefined }) {
  return (
    <div data-testid="networth">
      <p className="flex items-center gap-1 text-xs text-app-muted">
        <TrendingUp size={12} aria-hidden /> Patrimônio líquido
      </p>
      <p className="tnum mt-0.5 text-[15px] font-medium text-app-text">
        {overview ? formatBRL(overview.netWorthCents) : "—"}
      </p>
    </div>
  );
}
