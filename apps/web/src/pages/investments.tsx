import type { Account, Investment, InvestmentKind, InvestmentSummary } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowLeftRight, ChartLine, Clock, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card, CardHeader } from "../components/ui/card";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Money } from "../components/ui/money";
import { Badge, Segmented } from "../components/ui/segmented";
import { Sheet } from "../components/ui/sheet";
import { cn } from "../lib/cn";
import {
  centsToInput,
  formatAxisBRL,
  formatBRL,
  formatDate,
  formatMonthShort,
  formatPercent,
  parseBRLInput,
  parseSignedBRLInput,
  todayISO,
} from "../lib/format";
import {
  useAccounts,
  useContributeInvestment,
  useCreateInvestment,
  useDeleteInvestment,
  useInvestmentSummary,
  useInvestments,
  useRedeemInvestment,
  useUpdateInvestment,
  useUpdateInvestmentValue,
} from "../lib/queries";
import { useElementWidth } from "../lib/use-width";

const KINDS = [
  "FIXED_INCOME",
  "TREASURY",
  "STOCKS",
  "FII",
  "FUNDS",
  "CRYPTO",
  "PENSION",
  "OTHER",
] as const satisfies readonly InvestmentKind[];

const KIND_META: Record<InvestmentKind, { label: string; color: string }> = {
  FIXED_INCOME: { label: "Renda fixa", color: "#10b981" },
  TREASURY: { label: "Tesouro", color: "#38bdf8" },
  STOCKS: { label: "Ações", color: "#a78bfa" },
  FII: { label: "FIIs", color: "#fb923c" },
  FUNDS: { label: "Fundos", color: "#22d3ee" },
  CRYPTO: { label: "Cripto", color: "#fbbf24" },
  PENSION: { label: "Previdência", color: "#f472b6" },
  OTHER: { label: "Outros", color: "#94a3b8" },
};

const INSTITUTION_COLORS = ["#60a5fa", "#34d399", "#f472b6", "#fbbf24", "#a78bfa", "#94a3b8"];

/** Sem atualização há mais que isso, a posição pede um valor novo (v3 §3 INVESTMENT_STALE). */
const STALE_DAYS = 45;

function daysSince(iso: string): number {
  const today = Date.parse(`${todayISO()}T00:00:00Z`);
  const then = Date.parse(`${iso.slice(0, 10)}T00:00:00Z`);
  return Math.max(0, Math.round((today - then) / 86_400_000));
}

const iconBtn =
  "flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-muted hover:bg-app-soft hover:text-app-text md:h-9 md:w-9";

export function InvestmentsPage() {
  const investments = useInvestments();
  const summary = useInvestmentSummary();
  const accounts = useAccounts();
  const remove = useDeleteInvestment();
  const [editSheet, setEditSheet] = useState<{ open: boolean; editing: Investment | null }>({
    open: false,
    editing: null,
  });
  const [valueTarget, setValueTarget] = useState<Investment | null>(null);
  const [moveTarget, setMoveTarget] = useState<Investment | null>(null);

  const all = investments.data ?? [];
  const active = all.filter((i) => !i.archived);
  const archived = all.filter((i) => i.archived);

  const openCreate = (): void => setEditSheet({ open: true, editing: null });
  const onDelete = (item: Investment): void => {
    if (
      window.confirm(
        `Excluir "${item.name}"? O histórico de valores some junto; transferências já feitas continuam no extrato.`,
      )
    ) {
      remove.mutate(item.id);
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Investimentos</h1>
          <p className="text-sm text-app-muted">Posições, rentabilidade e alocação da carteira</p>
        </div>
        <Button variant="outline" size="sm" onClick={openCreate}>
          <Plus size={15} /> Novo investimento
        </Button>
      </header>

      <SummaryTiles summary={summary.data} />

      {summary.data && summary.data.totalCurrentCents > 0 ? (
        <div className="grid gap-4 lg:grid-cols-12">
          <Card className="lg:col-span-7" data-testid="investments-history">
            <CardHeader title="Evolução da carteira" />
            {trimEmptyMonths(summary.data.history).length > 1 ? (
              <HistoryChart history={trimEmptyMonths(summary.data.history)} />
            ) : (
              <p className="px-5 pb-5 text-sm text-app-muted">
                O gráfico aparece depois de algumas atualizações de valor.
              </p>
            )}
          </Card>
          <Card className="lg:col-span-5" data-testid="investments-allocation">
            <CardHeader title="Alocação" />
            <div className="space-y-5 px-5 pt-1 pb-5">
              <AllocationList
                title="Por tipo"
                rows={summary.data.byKind.map((k) => ({
                  key: k.kind,
                  label: KIND_META[k.kind]?.label ?? k.kind,
                  cents: k.currentCents,
                  percent: k.percent,
                  color: KIND_META[k.kind]?.color ?? "#94a3b8",
                }))}
              />
              <AllocationList
                title="Por instituição"
                rows={summary.data.byInstitution.map((r, i) => ({
                  key: r.institution,
                  label: r.institution || "Sem instituição",
                  cents: r.currentCents,
                  percent: r.percent,
                  color: INSTITUTION_COLORS[i % INSTITUTION_COLORS.length] ?? "#94a3b8",
                }))}
              />
            </div>
          </Card>
        </div>
      ) : null}

      <Card>
        <CardHeader title="Posições" />
        {investments.isLoading ? (
          <LoadingRows rows={3} />
        ) : investments.isError ? (
          <Empty
            icon={ChartLine}
            title="Não foi possível carregar os investimentos"
            hint={investments.error.message}
          />
        ) : active.length === 0 ? (
          <Empty
            icon={ChartLine}
            title="Nenhum investimento cadastrado"
            hint="CDB, Tesouro, ações, previdência: cadastre cada posição e atualize o valor de tempos em tempos para ver a rentabilidade."
            action={<Button onClick={openCreate}>Cadastrar primeiro</Button>}
          />
        ) : (
          <ul className="divide-y divide-app-border/60" data-testid="investments-list">
            {active.map((item) => (
              <InvestmentRow
                key={item.id}
                item={item}
                onValue={() => setValueTarget(item)}
                onMove={() => setMoveTarget(item)}
                onEdit={() => setEditSheet({ open: true, editing: item })}
                onDelete={() => onDelete(item)}
              />
            ))}
          </ul>
        )}
      </Card>

      {archived.length > 0 ? (
        <Card>
          <details className="group">
            <summary className="flex min-h-12 cursor-pointer list-none items-center justify-between px-5 text-sm font-medium text-app-text">
              Arquivados ({archived.length})
              <span className="text-app-muted group-open:rotate-45" aria-hidden>
                +
              </span>
            </summary>
            <ul className="divide-y divide-app-border/60 pb-2">
              {archived.map((item) => (
                <InvestmentRow
                  key={item.id}
                  item={item}
                  onValue={() => setValueTarget(item)}
                  onMove={() => setMoveTarget(item)}
                  onEdit={() => setEditSheet({ open: true, editing: item })}
                  onDelete={() => onDelete(item)}
                />
              ))}
            </ul>
          </details>
        </Card>
      ) : null}

      {remove.error ? (
        <p className="text-sm text-expense" role="alert">
          {remove.error.message}
        </p>
      ) : null}

      <InvestmentSheet
        open={editSheet.open}
        editing={editSheet.editing}
        accounts={accounts.data ?? []}
        onOpenChange={(open) => setEditSheet((s) => ({ open, editing: open ? s.editing : null }))}
      />
      <ValueSheet target={valueTarget} onClose={() => setValueTarget(null)} />
      <MoveSheet
        target={moveTarget}
        accounts={accounts.data ?? []}
        onClose={() => setMoveTarget(null)}
      />
    </div>
  );
}

// ───────────────────────── resumo ─────────────────────────

function SummaryTiles({ summary }: { summary: InvestmentSummary | undefined }) {
  return (
    <div className="grid gap-3 sm:grid-cols-3" data-testid="investments-summary">
      <Card className="p-4">
        <p className="text-sm text-app-muted">Valor atual</p>
        <p className="tnum mt-1 text-2xl font-semibold text-app-text">
          {summary ? formatBRL(summary.totalCurrentCents) : "—"}
        </p>
      </Card>
      <Card className="p-4">
        <p className="text-sm text-app-muted">Total investido</p>
        <p className="tnum mt-1 text-lg font-semibold text-app-text">
          {summary ? formatBRL(summary.totalInvestedCents) : "—"}
        </p>
        <p className="mt-1 text-xs text-app-muted">Aportes menos resgates</p>
      </Card>
      <Card className="p-4">
        <p className="text-sm text-app-muted">Rentabilidade</p>
        <p className="tnum mt-1 flex flex-wrap items-baseline gap-x-2 text-lg font-semibold">
          {summary ? (
            <>
              <Money cents={summary.returnCents} signed />
              {summary.returnPct !== null ? (
                <span
                  className={cn(
                    "text-sm font-medium",
                    summary.returnCents < 0 ? "text-expense" : "text-income",
                  )}
                >
                  {formatPercent(summary.returnPct)}
                </span>
              ) : null}
            </>
          ) : (
            "—"
          )}
        </p>
        <p className="mt-1 text-xs text-app-muted">Valor atual − investido</p>
      </Card>
    </div>
  );
}

function AllocationList({
  title,
  rows,
}: {
  title: string;
  rows: { key: string; label: string; cents: number; percent: number; color: string }[];
}) {
  if (rows.length === 0) return null;
  return (
    <div>
      <p className="text-xs font-medium tracking-wide text-app-subtle uppercase">{title}</p>
      <div className="mt-2 flex h-2.5 w-full overflow-hidden rounded-full bg-app-soft" aria-hidden>
        {rows.map((r) => (
          <span
            key={r.key}
            className="h-full"
            style={{ width: `${Math.max(0, r.percent)}%`, backgroundColor: r.color }}
          />
        ))}
      </div>
      <ul className="mt-2.5 space-y-2">
        {rows.map((r) => (
          <li key={r.key} className="flex items-center gap-2.5 text-sm">
            <span
              className="h-2.5 w-2.5 shrink-0 rounded-sm"
              style={{ backgroundColor: r.color }}
              aria-hidden
            />
            <span className="min-w-0 flex-1 truncate text-app-text">{r.label}</span>
            <span className="tnum text-xs text-app-muted">{formatPercent(r.percent)}</span>
            <span className="tnum w-24 text-right font-medium text-app-text">
              {formatBRL(r.cents)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/** Linhas do valor atual vs. investido, fim de cada mês (SVG nativo). */
/** Meses antes do primeiro registro de valor não são "zero investido": só não há dado ainda. */
function trimEmptyMonths(history: InvestmentSummary["history"]): InvestmentSummary["history"] {
  const first = history.findIndex((h) => h.currentCents !== 0 || h.investedCents !== 0);
  return first <= 0 ? history : history.slice(first);
}

function HistoryChart({ history }: { history: InvestmentSummary["history"] }) {
  const [ref, width] = useElementWidth<HTMLDivElement>(640);
  const height = 210;
  const padLeft = 48;
  const padRight = 14;
  const padTop = 14;
  const padBottom = 26;
  const plotW = Math.max(10, width - padLeft - padRight);
  const plotH = height - padTop - padBottom;
  const max = Math.max(10_000, ...history.flatMap((h) => [h.currentCents, h.investedCents]));
  const xAt = (i: number): number =>
    history.length <= 1 ? padLeft + plotW / 2 : padLeft + (i / (history.length - 1)) * plotW;
  const yAt = (cents: number): number => padTop + plotH - (cents / max) * plotH;
  const line = (key: "currentCents" | "investedCents"): string =>
    history.map((h, i) => `${i === 0 ? "M" : "L"} ${xAt(i)} ${yAt(h[key])}`).join(" ");
  const labelEvery = Math.ceil(history.length / Math.max(3, Math.floor(plotW / 56)));
  const last = history[history.length - 1];

  return (
    <div className="px-3 pb-4">
      <ul className="flex flex-wrap gap-x-4 gap-y-1 px-2 pb-2 text-xs text-app-muted">
        <li className="flex items-center gap-1.5">
          <span className="h-0.5 w-4 rounded bg-(--ff-income)" aria-hidden /> Valor atual
        </li>
        <li className="flex items-center gap-1.5">
          <span
            className="h-0.5 w-4 rounded border-t-2 border-dashed border-app-muted"
            aria-hidden
          />
          Investido
        </li>
      </ul>
      <div ref={ref} className="w-full">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          height={height}
          className="block w-full"
          role="img"
          aria-label={
            last
              ? `Evolução da carteira: ${formatBRL(last.currentCents)} atuais para ${formatBRL(last.investedCents)} investidos em ${formatMonthShort(last.month)} ${last.year}.`
              : "Evolução da carteira"
          }
        >
          {[0, 0.5, 1].map((t) => (
            <g key={t}>
              <line
                x1={padLeft}
                x2={width - padRight}
                y1={yAt(max * t)}
                y2={yAt(max * t)}
                stroke="var(--ff-border)"
                strokeDasharray={t === 0 ? undefined : "3 3"}
              />
              <text
                x={padLeft - 8}
                y={yAt(max * t) + 4}
                textAnchor="end"
                fill="var(--ff-subtle)"
                fontSize={11}
              >
                {formatAxisBRL(max * t)}
              </text>
            </g>
          ))}
          <path
            d={line("investedCents")}
            fill="none"
            stroke="var(--ff-muted)"
            strokeWidth={1.5}
            strokeDasharray="5 4"
          />
          <path
            d={line("currentCents")}
            fill="none"
            stroke="var(--ff-income)"
            strokeWidth={2}
            strokeLinejoin="round"
            strokeLinecap="round"
          />
          {history.map((h, i) => (
            <g key={`${h.year}-${h.month}`}>
              <circle cx={xAt(i)} cy={yAt(h.currentCents)} r={2.5} fill="var(--ff-income)">
                <title>{`${formatMonthShort(h.month)} ${h.year}: ${formatBRL(h.currentCents)} (investido ${formatBRL(h.investedCents)})`}</title>
              </circle>
              {i % labelEvery === 0 || i === history.length - 1 ? (
                <text
                  x={xAt(i)}
                  y={height - 8}
                  textAnchor="middle"
                  fill="var(--ff-subtle)"
                  fontSize={11}
                >
                  {formatMonthShort(h.month)}
                </text>
              ) : null}
            </g>
          ))}
        </svg>
      </div>
    </div>
  );
}

// ───────────────────────── posição ─────────────────────────

function InvestmentRow({
  item,
  onValue,
  onMove,
  onEdit,
  onDelete,
}: {
  item: Investment;
  onValue: () => void;
  onMove: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const meta = KIND_META[item.kind] ?? KIND_META.OTHER;
  const age = item.valueUpdatedAt ? daysSince(item.valueUpdatedAt) : null;
  const stale = age === null || age > STALE_DAYS;
  return (
    <li
      className={cn("flex flex-col gap-2.5 px-4 py-3.5 md:px-5", item.archived && "opacity-70")}
      data-testid={`investment-${item.id}`}
    >
      <div className="flex items-start gap-3">
        <span
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full"
          style={{ backgroundColor: `${meta.color}1f`, color: meta.color }}
          aria-hidden
        >
          <ChartLine size={17} />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-app-text">{item.name}</p>
          <p className="truncate text-xs text-app-muted">
            {meta.label}
            {item.institution ? ` · ${item.institution}` : ""}
            {item.accountName ? ` · conta ${item.accountName}` : " · sem conta vinculada"}
          </p>
        </div>
        <div className="flex shrink-0 flex-col items-end">
          <Money cents={item.currentCents} neutral className="text-sm font-semibold" />
          <p
            className={cn(
              "tnum text-xs",
              item.returnCents < 0
                ? "text-expense"
                : item.returnCents > 0
                  ? "text-income"
                  : "text-app-muted",
            )}
          >
            <Money cents={item.returnCents} signed />
            {item.returnPct !== null ? ` · ${formatPercent(item.returnPct)}` : ""}
          </p>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2 sm:pl-12">
        {stale ? (
          <span
            className="inline-flex min-h-6 items-center gap-1 rounded-full border border-amber-400/50 px-2 py-0.5 text-[11px] font-medium text-warn"
            data-testid="investment-stale"
          >
            <Clock size={12} aria-hidden className="shrink-0" />
            {age === null
              ? "valor nunca atualizado — atualize"
              : `valor não atualizado há ${age} dias — atualize`}
          </span>
        ) : item.valueUpdatedAt ? (
          <span className="text-xs text-app-muted">
            atualizado em {formatDate(item.valueUpdatedAt.slice(0, 10))}
          </span>
        ) : null}
        {item.archived ? <Badge>Arquivado</Badge> : null}
        <div className="ml-auto flex flex-wrap items-center justify-end gap-1">
          <Button
            variant="outline"
            size="sm"
            onClick={onValue}
            aria-label={`Atualizar valor de ${item.name}`}
          >
            <RefreshCw size={14} aria-hidden /> Atualizar valor
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={onMove}
            aria-label={`Aportar ou resgatar em ${item.name}`}
          >
            <ArrowLeftRight size={14} aria-hidden /> Aportar / resgatar
          </Button>
          <button
            type="button"
            className={iconBtn}
            onClick={onEdit}
            aria-label={`Editar ${item.name}`}
          >
            <Pencil size={14} />
          </button>
          <button
            type="button"
            className={cn(iconBtn, "hover:bg-rose-500/10 hover:text-expense")}
            onClick={onDelete}
            aria-label={`Excluir ${item.name}`}
          >
            <Trash2 size={15} />
          </button>
        </div>
      </div>
    </li>
  );
}

// ───────────────────────── cadastro / edição ─────────────────────────

const optionalMoney = z
  .string()
  .refine(
    (v) => v.trim() === "" || (parseSignedBRLInput(v) ?? -1) >= 0,
    "valor inválido — use 1.000,00",
  );

const investmentFormSchema = z.object({
  name: z.string().trim().min(1, "dê um nome ao investimento").max(80),
  kind: z.enum(KINDS),
  institution: z.string().max(80),
  accountId: z.string(),
  invested: optionalMoney,
  current: optionalMoney,
  notes: z.string().max(500),
  archived: z.boolean(),
});
type InvestmentForm = z.infer<typeof investmentFormSchema>;

const emptyInvestment = (): InvestmentForm => ({
  name: "",
  kind: "FIXED_INCOME",
  institution: "",
  accountId: "",
  invested: "",
  current: "",
  notes: "",
  archived: false,
});

function InvestmentSheet({
  open,
  editing,
  accounts,
  onOpenChange,
}: {
  open: boolean;
  editing: Investment | null;
  accounts: Account[];
  onOpenChange: (open: boolean) => void;
}) {
  const create = useCreateInvestment();
  const update = useUpdateInvestment();
  const form = useForm<InvestmentForm>({
    resolver: zodResolver(investmentFormSchema),
    defaultValues: emptyInvestment(),
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reinicia só ao abrir/trocar o alvo
  useEffect(() => {
    if (!open) return;
    create.reset();
    update.reset();
    form.reset(
      editing
        ? {
            name: editing.name,
            kind: editing.kind,
            institution: editing.institution ?? "",
            accountId: editing.accountId ?? "",
            invested: centsToInput(editing.investedCents),
            current: centsToInput(editing.currentCents),
            notes: editing.notes ?? "",
            archived: editing.archived,
          }
        : emptyInvestment(),
    );
  }, [open, editing]);

  const submit = form.handleSubmit(async (v) => {
    const investedCents = v.invested.trim() ? (parseSignedBRLInput(v.invested) ?? 0) : 0;
    const currentCents = v.current.trim()
      ? (parseSignedBRLInput(v.current) ?? investedCents)
      : investedCents;
    const payload = {
      name: v.name.trim(),
      kind: v.kind,
      institution: v.institution.trim() || null,
      accountId: v.accountId || null,
      investedCents,
      currentCents,
      notes: v.notes.trim() || null,
    };
    if (editing) await update.mutateAsync({ id: editing.id, ...payload, archived: v.archived });
    else await create.mutateAsync(payload);
    onOpenChange(false);
  });

  const errors = form.formState.errors;
  const error = create.error?.message ?? update.error?.message;
  const pending = create.isPending || update.isPending;

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? `Editar — ${editing.name}` : "Novo investimento"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-investment">
        <Field label="Nome" error={errors.name?.message}>
          <Input placeholder="CDB 110% do CDI, Tesouro IPCA+…" {...form.register("name")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Tipo">
            <Select {...form.register("kind")}>
              {KINDS.map((k) => (
                <option key={k} value={k}>
                  {KIND_META[k].label}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Instituição (opcional)" error={errors.institution?.message}>
            <Input placeholder="Banco, corretora…" {...form.register("institution")} />
          </Field>
        </div>
        <Field label="Conta vinculada (opcional)">
          <Select {...form.register("accountId")}>
            <option value="">Nenhuma</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
                {a.type === "INVESTMENT" ? " (investimentos)" : ""}
              </option>
            ))}
          </Select>
          <span className="text-xs text-app-muted">
            Use uma conta do tipo Investimentos: aportes e resgates viram transferências dela, e o
            patrimônio passa a usar o valor das posições no lugar do saldo da conta.
          </span>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Total investido" error={errors.invested?.message}>
            <Input inputMode="decimal" placeholder="10.000,00" {...form.register("invested")} />
          </Field>
          <Field label="Valor atual" error={errors.current?.message}>
            <Input
              inputMode="decimal"
              placeholder="igual ao investido"
              {...form.register("current")}
            />
          </Field>
        </div>
        <Field label="Observações (opcional)">
          <Input placeholder="Vencimento, liquidez…" {...form.register("notes")} />
        </Field>
        {editing ? (
          <label className="flex min-h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text">
            <input
              type="checkbox"
              className="h-4 w-4 accent-emerald-500"
              {...form.register("archived")}
            />
            Arquivado (resgatado por completo; sai da carteira)
          </label>
        ) : null}
        {error ? <p className="text-sm text-expense">{error}</p> : null}
        <Button type="submit" disabled={pending} className="h-12">
          {pending ? "Salvando…" : editing ? "Salvar alterações" : "Cadastrar investimento"}
        </Button>
      </form>
    </Sheet>
  );
}

// ───────────────────────── atualizar valor ─────────────────────────

function ValueSheet({ target, onClose }: { target: Investment | null; onClose: () => void }) {
  const updateValue = useUpdateInvestmentValue();
  const [amount, setAmount] = useState("");
  const [date, setDate] = useState(todayISO());

  // biome-ignore lint/correctness/useExhaustiveDependencies: reinicia só ao trocar o alvo
  useEffect(() => {
    if (!target) return;
    updateValue.reset();
    setAmount(centsToInput(target.currentCents));
    setDate(todayISO());
  }, [target]);

  const cents = parseSignedBRLInput(amount);
  const valid = cents !== null && cents >= 0;

  const submit = async (): Promise<void> => {
    if (!target || !valid) return;
    await updateValue.mutateAsync({ id: target.id, currentCents: cents, date: date || undefined });
    onClose();
  };

  return (
    <Sheet
      open={Boolean(target)}
      onOpenChange={(open) => !open && onClose()}
      title={target ? `Atualizar valor — ${target.name}` : ""}
    >
      <form
        className="flex flex-col gap-4 pb-2"
        data-testid="form-investment-value"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-app-muted">
          Copie o saldo bruto que aparece no app da instituição. O FinFlow guarda o histórico para
          calcular a rentabilidade e desenhar a evolução.
        </p>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Valor atual" error={amount && !valid ? "valor inválido" : undefined}>
            <Input inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} />
          </Field>
          <Field label="Data">
            <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
          </Field>
        </div>
        {updateValue.error ? (
          <p className="text-sm text-expense">{updateValue.error.message}</p>
        ) : null}
        <Button type="submit" disabled={updateValue.isPending || !valid} className="h-12">
          {updateValue.isPending ? "Salvando…" : "Salvar valor"}
        </Button>
      </form>
    </Sheet>
  );
}

// ───────────────────────── aporte / resgate ─────────────────────────

function MoveSheet({
  target,
  accounts,
  onClose,
}: {
  target: Investment | null;
  accounts: Account[];
  onClose: () => void;
}) {
  const contribute = useContributeInvestment();
  const redeem = useRedeemInvestment();
  const [direction, setDirection] = useState<"in" | "out">("in");
  const [amount, setAmount] = useState("");
  const [date, setDate] = useState(todayISO());
  const [accountId, setAccountId] = useState("");

  // biome-ignore lint/correctness/useExhaustiveDependencies: reinicia só ao trocar o alvo
  useEffect(() => {
    if (!target) return;
    contribute.reset();
    redeem.reset();
    setDirection("in");
    setAmount("");
    setDate(todayISO());
    setAccountId("");
  }, [target]);

  const cents = parseBRLInput(amount);
  const mutation = direction === "in" ? contribute : redeem;
  const others = accounts.filter((a) => a.id !== target?.accountId);

  const submit = async (): Promise<void> => {
    if (!target || !cents) return;
    if (direction === "in") {
      await contribute.mutateAsync({
        id: target.id,
        amountCents: cents,
        date: date || undefined,
        fromAccountId: accountId || undefined,
      });
    } else {
      await redeem.mutateAsync({
        id: target.id,
        amountCents: cents,
        date: date || undefined,
        toAccountId: accountId || undefined,
      });
    }
    onClose();
  };

  return (
    <Sheet
      open={Boolean(target)}
      onOpenChange={(open) => !open && onClose()}
      title={target ? `${target.name} — ${formatBRL(target.currentCents)}` : ""}
    >
      <form
        className="flex flex-col gap-4 pb-2"
        data-testid="form-investment-move"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <Segmented
          label="Movimento"
          value={direction}
          onChange={(v) => {
            setDirection(v);
            contribute.reset();
            redeem.reset();
          }}
          options={[
            ["in", "Aportar"],
            ["out", "Resgatar"],
          ]}
        />
        <div className="grid grid-cols-2 gap-3">
          <Field label="Valor" error={amount && !cents ? "valor inválido" : undefined}>
            <Input
              inputMode="decimal"
              placeholder="500,00"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
            />
          </Field>
          <Field label="Data">
            <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
          </Field>
        </div>
        <Field
          label={
            direction === "in"
              ? "Tirar dinheiro da conta (opcional)"
              : "Mandar para a conta (opcional)"
          }
        >
          <Select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
            <option value="">Nenhuma — só ajustar a posição</option>
            {others.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name} ({formatBRL(a.balanceCents)})
              </option>
            ))}
          </Select>
        </Field>
        <p className="text-xs text-app-muted">
          {accountId
            ? target?.accountName
              ? direction === "in"
                ? `Cria uma transferência dessa conta para ${target.accountName} (conta da posição).`
                : `Cria uma transferência de ${target.accountName} (conta da posição) para essa conta.`
              : "Esta posição não tem conta vinculada: vincule uma conta de investimentos em Editar para movimentar dinheiro."
            : "Sem conta, só o investido e o valor atual da posição mudam; o saldo das contas fica igual."}
        </p>
        {mutation.error ? <p className="text-sm text-expense">{mutation.error.message}</p> : null}
        <Button type="submit" disabled={mutation.isPending || !cents} className="h-12">
          {mutation.isPending
            ? "Salvando…"
            : direction === "in"
              ? "Registrar aporte"
              : "Registrar resgate"}
        </Button>
      </form>
    </Sheet>
  );
}
