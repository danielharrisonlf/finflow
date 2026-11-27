import type {
  CommitmentItem,
  CommitmentMonth,
  DetectedSubscription,
  Recurring,
  SubscriptionCadence,
} from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  CalendarClock,
  Check,
  CreditCard,
  HandCoins,
  Pencil,
  Plus,
  Repeat,
  Trash2,
  TrendingDown,
  TrendingUp,
  Tv,
} from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { Link } from "react-router-dom";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card, CardHeader } from "../components/ui/card";
import { CategoryIcon, formatCategoryLabel } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Money } from "../components/ui/money";
import { MonthSwitcher, usePeriodState } from "../components/ui/month-switcher";
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
  todayISO,
} from "../lib/format";
import {
  useAccounts,
  useCategories,
  useCommitments,
  useCreateRecurring,
  useDeleteRecurring,
  useDismissSubscription,
  usePostRecurring,
  useRecurring,
  useSubscriptions,
  useTrackSubscription,
  useUpdateRecurring,
} from "../lib/queries";
import { useElementWidth } from "../lib/use-width";

type Direction = "PAY" | "RECEIVE";

const DIRECTIONS = [
  ["PAY", "A pagar"],
  ["RECEIVE", "A receber"],
] as const;

export function CommitmentsPage() {
  const [period, shiftPeriod] = usePeriodState();
  const [direction, setDirection] = useState<Direction>("PAY");
  const commitments = useCommitments(period);
  const recurring = useRecurring();
  const [posting, setPosting] = useState<{ item: CommitmentItem; rec: Recurring | null } | null>(
    null,
  );
  const [editing, setEditing] = useState<Recurring | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const remove = useDeleteRecurring();

  const data = commitments.data;
  const items = (data?.items ?? []).filter((i) => i.direction === direction);

  const openCreate = (): void => {
    setEditing(null);
    setSheetOpen(true);
  };
  const openEdit = (rec: Recurring): void => {
    setEditing(rec);
    setSheetOpen(true);
  };
  const onDelete = (rec: Recurring): void => {
    if (
      window.confirm(
        `Excluir "${rec.description}"? Lançamentos já feitos continuam no extrato, sem o vínculo.`,
      )
    ) {
      remove.mutate(rec.id);
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Compromissos</h1>
          <p className="text-sm text-app-muted">Contas fixas, receitas recorrentes e parcelas</p>
        </div>
        <MonthSwitcher period={period} onShift={shiftPeriod} />
      </header>

      <div className="grid grid-cols-3 gap-3">
        <TotalTile label="A pagar" cents={data ? -data.totals.payCents : undefined} />
        <TotalTile label="A receber" cents={data?.totals.receiveCents} />
        <TotalTile
          label="Saldo depois"
          cents={data?.totals.balanceAfterCents}
          hint="saldo disponível + a receber − a pagar"
        />
      </div>

      <Segmented
        label="Direção dos compromissos"
        value={direction}
        onChange={setDirection}
        options={DIRECTIONS}
        className="sm:max-w-xs"
      />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-12">
        <Card className="lg:col-span-5" data-testid="commitments-chart">
          <CardHeader title="Próximos 6 meses" />
          {data ? (
            <MonthsChart months={data.months} direction={direction} />
          ) : (
            <LoadingRows rows={3} />
          )}
        </Card>

        <Card className="lg:col-span-7">
          <CardHeader
            title={direction === "PAY" ? "Contas e parcelas do mês" : "Entradas previstas do mês"}
          />
          {commitments.isLoading ? (
            <LoadingRows rows={4} />
          ) : commitments.isError ? (
            <Empty
              icon={CalendarClock}
              title="Não foi possível carregar os compromissos"
              hint={commitments.error.message}
            />
          ) : items.length === 0 ? (
            <Empty
              icon={CalendarClock}
              title={direction === "PAY" ? "Nada a pagar neste mês" : "Nada a receber neste mês"}
              hint="Cadastre contas fixas e receitas recorrentes para enxergar o mês antes dele chegar."
            />
          ) : (
            <ul
              className="divide-y divide-app-border/60 px-4 pb-2 md:px-5"
              data-testid="commitment-items"
            >
              {items.map((item) => {
                const rec = item.recurringId
                  ? (recurring.data?.find((r) => r.id === item.recurringId) ?? null)
                  : null;
                return (
                  <li
                    key={`${item.kind}-${item.recurringId ?? item.description}-${item.dueDate}-${item.installmentNumber ?? 0}-${item.cardName ?? ""}`}
                    className="flex items-center gap-3 py-3"
                  >
                    {rec ? (
                      <CategoryIcon icon={rec.categoryIcon ?? "repeat"} color={rec.categoryColor} />
                    ) : (
                      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-app-soft/80 text-app-muted">
                        {item.kind === "INSTALLMENT" ? (
                          <CreditCard size={16} />
                        ) : item.kind === "LOAN" ? (
                          <HandCoins size={16} />
                        ) : (
                          <Repeat size={16} />
                        )}
                      </span>
                    )}
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm text-app-text">{item.description}</p>
                      <p className="truncate text-xs text-app-muted">
                        {formatDate(item.dueDate)}
                        {item.kind === "INSTALLMENT"
                          ? ` · parcela ${item.installmentNumber ?? "?"}/${item.installmentTotal ?? "?"}${item.cardName ? ` · ${item.cardName}` : ""}`
                          : item.kind === "LOAN"
                            ? ` · empréstimo${item.installmentNumber ? ` · parcela ${item.installmentNumber}/${item.installmentTotal ?? "?"}` : ""}`
                            : " · recorrente"}
                      </p>
                    </div>
                    <Money
                      cents={item.direction === "PAY" ? -item.amountCents : item.amountCents}
                      signed
                      className="text-sm font-medium"
                    />
                    {item.realized ? (
                      <Badge tone="income">
                        <Check size={12} aria-hidden /> Lançado
                      </Badge>
                    ) : item.kind === "RECURRING" && item.recurringId ? (
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-11 md:h-9"
                        onClick={() => setPosting({ item, rec })}
                        aria-label={`Lançar ${item.description}`}
                      >
                        Lançar
                      </Button>
                    ) : item.kind === "LOAN" ? (
                      <Link
                        to="/dividas"
                        className="inline-flex h-11 shrink-0 items-center rounded-(--radius-control) border border-app-border px-3 text-sm text-app-text hover:bg-app-surface md:h-9"
                        aria-label={`Pagar ${item.description} em Dívidas`}
                      >
                        Pagar
                      </Link>
                    ) : null}
                  </li>
                );
              })}
            </ul>
          )}
        </Card>
      </div>

      <SubscriptionsCard />

      <Card>
        <CardHeader
          title="Contas fixas e receitas recorrentes"
          action={
            <Button variant="outline" size="sm" onClick={openCreate}>
              <Plus size={15} /> Nova recorrência
            </Button>
          }
        />
        {recurring.isLoading ? (
          <LoadingRows rows={3} />
        ) : (recurring.data?.length ?? 0) === 0 ? (
          <Empty
            icon={Repeat}
            title="Nenhuma recorrência cadastrada"
            hint="Aluguel, internet, salário: cadastre uma vez e o FinFlow projeta todos os meses."
            action={<Button onClick={openCreate}>Cadastrar primeira</Button>}
          />
        ) : (
          <ul className="divide-y divide-app-border/60" data-testid="recurring-list">
            {recurring.data?.map((rec) => (
              <li key={rec.id} className="group flex items-center gap-3 px-4 py-3 md:px-5">
                <CategoryIcon icon={rec.categoryIcon ?? "repeat"} color={rec.categoryColor} />
                <div className="min-w-0 flex-1">
                  <p
                    className={cn(
                      "truncate text-sm",
                      rec.active ? "text-app-text" : "text-app-muted line-through",
                    )}
                  >
                    {rec.description}
                  </p>
                  <p className="truncate text-xs text-app-muted">
                    {rec.isSubscription ? "assinatura · " : ""}
                    todo dia {rec.dayOfMonth}
                    {rec.accountName ? ` · ${rec.accountName}` : " · sem conta"}
                    {rec.categoryName ? ` · ${rec.categoryName}` : ""}
                    {!rec.active
                      ? " · pausada"
                      : rec.nextDate
                        ? ` · próxima ${formatDate(rec.nextDate)}`
                        : ""}
                  </p>
                </div>
                <Money
                  cents={rec.kind === "EXPENSE" ? -rec.amountCents : rec.amountCents}
                  signed
                  className="text-sm font-medium"
                />
                <button
                  type="button"
                  onClick={() => openEdit(rec)}
                  aria-label={`Editar ${rec.description}`}
                  className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-muted hover:bg-app-soft hover:text-app-text md:h-9 md:w-9"
                >
                  <Pencil size={14} />
                </button>
                <button
                  type="button"
                  onClick={() => onDelete(rec)}
                  aria-label={`Excluir ${rec.description}`}
                  className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-subtle hover:bg-rose-500/10 hover:text-expense md:h-9 md:w-9"
                >
                  <Trash2 size={15} />
                </button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <PostRecurringSheet target={posting} onClose={() => setPosting(null)} />
      <RecurringSheet
        open={sheetOpen}
        onOpenChange={(open) => {
          setSheetOpen(open);
          if (!open) setEditing(null);
        }}
        editing={editing}
      />
    </div>
  );
}

function TotalTile({
  label,
  cents,
  hint,
}: {
  label: string;
  cents: number | undefined;
  hint?: string;
}) {
  return (
    <Card className="px-3 py-3 sm:px-4">
      <p className="text-xs text-app-muted">{label}</p>
      <p className="tnum mt-1 truncate text-sm font-semibold sm:text-lg">
        {cents === undefined ? "—" : <Money cents={cents} />}
      </p>
      {hint ? <p className="mt-0.5 hidden text-[11px] text-app-muted sm:block">{hint}</p> : null}
    </Card>
  );
}

/** Barras empilhadas: contas fixas + parcelas (a pagar) ou receitas recorrentes (a receber). */
function MonthsChart({ months, direction }: { months: CommitmentMonth[]; direction: Direction }) {
  const [ref, width] = useElementWidth<HTMLDivElement>(420);
  const height = 200;
  const padTop = 22;
  const padBottom = 24;
  const plotH = height - padTop - padBottom;
  const hasLoans = months.some((m) => (m.loansCents ?? 0) > 0);
  const series =
    direction === "PAY"
      ? [
          {
            key: "recurringExpenseCents" as const,
            label: "Contas fixas",
            color: "var(--spend-fixed)",
          },
          { key: "installmentsCents" as const, label: "Parcelas", color: "var(--spend-cuttable)" },
          ...(hasLoans
            ? [
                {
                  key: "loansCents" as const,
                  label: "Empréstimos",
                  color: "var(--spend-variable)",
                },
              ]
            : []),
        ]
      : [
          {
            key: "recurringIncomeCents" as const,
            label: "Receitas recorrentes",
            color: "var(--ff-income)",
          },
        ];
  const totals = months.map((m) => series.reduce((acc, s) => acc + (m[s.key] ?? 0), 0));
  const max = Math.max(10_000, ...totals);
  const slot = width / Math.max(1, months.length);
  const barW = Math.min(44, slot * 0.56);

  return (
    <div className="px-4 pb-4">
      <ul className="flex flex-wrap gap-x-4 gap-y-1 pb-2 text-xs text-app-muted">
        {series.map((s) => (
          <li key={s.key} className="flex items-center gap-1.5">
            <span
              className="h-2.5 w-2.5 rounded-sm"
              style={{ backgroundColor: s.color }}
              aria-hidden
            />
            {s.label}
          </li>
        ))}
      </ul>
      <div ref={ref} className="w-full">
        <svg
          height={height}
          viewBox={`0 0 ${width} ${height}`}
          className="block w-full"
          role="img"
          aria-label={months
            .map((m, i) => `${formatMonthShort(m.month)}: ${formatBRL(totals[i] ?? 0)}`)
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
                {series.map((s) => {
                  const h = ((m[s.key] ?? 0) / max) * plotH;
                  y -= h;
                  return h > 0 ? (
                    <rect key={s.key} x={x} y={y} width={barW} height={h} fill={s.color} rx={3}>
                      <title>{`${s.label}: ${formatBRL(m[s.key] ?? 0)}`}</title>
                    </rect>
                  ) : null;
                })}
                <text
                  x={x + barW / 2}
                  y={y - 6}
                  textAnchor="middle"
                  fill="var(--ff-muted)"
                  fontSize={11}
                  className="tnum"
                >
                  {totals[i] ? formatAxisBRL(totals[i] ?? 0) : ""}
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

// ───────────────────────── assinaturas detectadas (v3 §2) ─────────────────────────

const CADENCE: Record<SubscriptionCadence, string> = {
  WEEKLY: "semanal",
  MONTHLY: "mensal",
  YEARLY: "anual",
};

function SubscriptionsCard() {
  const subscriptions = useSubscriptions();
  const track = useTrackSubscription();
  const dismiss = useDismissSubscription();
  const items = subscriptions.data?.items ?? [];
  const busyKey = track.isPending ? track.variables : dismiss.isPending ? dismiss.variables : null;

  return (
    <Card data-testid="subscriptions-card">
      <CardHeader
        title={
          <span className="flex items-center gap-1.5">
            <Tv size={14} aria-hidden /> Assinaturas detectadas
          </span>
        }
        action={
          items.length > 0 ? (
            <span className="tnum text-xs text-app-muted" data-testid="subscriptions-total">
              ≈ <Money cents={subscriptions.data?.monthlyTotalCents ?? 0} neutral />
              /mês
            </span>
          ) : null
        }
      />
      {subscriptions.isLoading ? (
        <LoadingRows rows={2} />
      ) : items.length === 0 ? (
        <p className="px-5 pb-5 text-sm text-app-muted">
          Nenhuma cobrança repetida encontrada no seu histórico. O FinFlow procura gastos com a
          mesma descrição e valor parecido toda semana, mês ou ano.
        </p>
      ) : (
        <>
          <p className="px-4 text-xs text-app-muted md:px-5">
            Cobranças que se repetem no seu histórico. Acompanhe para entrarem nos compromissos e na
            projeção, ou ignore o que não for assinatura.
          </p>
          <ul
            className="mt-1 divide-y divide-app-border/60 px-4 pb-2 md:px-5"
            data-testid="subscriptions-list"
          >
            {items.map((sub) => (
              <SubscriptionRow
                key={sub.key}
                sub={sub}
                busy={busyKey === sub.key}
                onTrack={() => track.mutate(sub.key)}
                onDismiss={() => dismiss.mutate(sub.key)}
              />
            ))}
          </ul>
        </>
      )}
      {track.error || dismiss.error ? (
        <p className="px-5 pb-4 text-sm text-expense" role="alert">
          {(track.error ?? dismiss.error)?.message}
        </p>
      ) : null}
    </Card>
  );
}

function SubscriptionRow({
  sub,
  busy,
  onTrack,
  onDismiss,
}: {
  sub: DetectedSubscription;
  busy: boolean;
  onTrack: () => void;
  onDismiss: () => void;
}) {
  const change = sub.priceChangePct;
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3">
      <div className="min-w-0 flex-1 basis-48">
        <p className="flex items-center gap-2 text-sm text-app-text">
          <span className="truncate">{sub.description}</span>
          {change !== null && Math.abs(change) >= 5 ? (
            <Badge tone={change > 0 ? "warn" : "income"} data-testid="price-change">
              {change > 0 ? (
                <TrendingUp size={12} aria-hidden />
              ) : (
                <TrendingDown size={12} aria-hidden />
              )}
              {change > 0 ? "+" : "−"}
              {formatPercent(Math.abs(change))}
            </Badge>
          ) : null}
        </p>
        <p className="truncate text-xs text-app-muted">
          {CADENCE[sub.cadence] ?? sub.cadence} · {sub.occurrences} cobranças · próxima{" "}
          {formatDate(sub.nextExpectedDate)}
          {change !== null && Math.abs(change) >= 5
            ? ` · média ${formatBRL(sub.averageCents)}`
            : ""}
        </p>
      </div>
      <Money cents={-Math.abs(sub.lastCents)} signed className="text-sm font-medium" />
      {sub.recurringId ? (
        <Badge tone="income">
          <Check size={12} aria-hidden /> Acompanhada
        </Badge>
      ) : (
        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            className="h-11 md:h-9"
            disabled={busy}
            onClick={onTrack}
            aria-label={`Acompanhar ${sub.description}`}
          >
            Acompanhar
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-11 text-app-muted md:h-9"
            disabled={busy}
            onClick={onDismiss}
            aria-label={`Ignorar ${sub.description}`}
          >
            Ignorar
          </Button>
        </div>
      )}
    </li>
  );
}

// ───────────────────────── lançar ocorrência ─────────────────────────

function PostRecurringSheet({
  target,
  onClose,
}: {
  target: { item: CommitmentItem; rec: Recurring | null } | null;
  onClose: () => void;
}) {
  const accounts = useAccounts();
  const post = usePostRecurring();
  const [amount, setAmount] = useState("");
  const [date, setDate] = useState("");
  const [accountId, setAccountId] = useState("");

  // biome-ignore lint/correctness/useExhaustiveDependencies: reinicia só ao trocar o alvo
  useEffect(() => {
    if (!target) return;
    post.reset();
    setAmount(centsToInput(target.item.amountCents));
    setDate(target.item.dueDate);
    setAccountId(target.rec?.accountId ?? accounts.data?.[0]?.id ?? "");
  }, [target]);

  const cents = parseBRLInput(amount);
  const submit = async (): Promise<void> => {
    if (!target?.item.recurringId || !cents || !accountId) return;
    await post.mutateAsync({
      id: target.item.recurringId,
      date: date || undefined,
      amountCents: cents,
      accountId,
    });
    onClose();
  };

  return (
    <Sheet
      open={Boolean(target)}
      onOpenChange={(open) => !open && onClose()}
      title={target ? `Lançar ${target.item.description}` : ""}
    >
      <form
        className="flex flex-col gap-4 pb-2"
        data-testid="form-post-recurring"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-app-muted">
          Vira uma transação confirmada no extrato. Ajuste o valor se a conta veio diferente este
          mês.
        </p>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Valor" error={amount && !cents ? "valor inválido" : undefined}>
            <Input inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} />
          </Field>
          <Field label="Data">
            <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
          </Field>
        </div>
        <Field label="Conta">
          <Select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
            <option value="" disabled>
              Escolher…
            </option>
            {accounts.data?.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </Select>
        </Field>
        {post.error ? <p className="text-sm text-expense">{post.error.message}</p> : null}
        <Button type="submit" disabled={post.isPending || !cents || !accountId} className="h-12">
          {post.isPending ? "Lançando…" : "Lançar transação"}
        </Button>
      </form>
    </Sheet>
  );
}

// ───────────────────────── cadastro de recorrência ─────────────────────────

const recurringFormSchema = z
  .object({
    description: z.string().trim().min(1, "descreva a recorrência").max(120),
    kind: z.enum(["EXPENSE", "INCOME"]),
    amount: z.string().refine((v) => parseBRLInput(v) !== null, "valor inválido — use 120,00"),
    dayOfMonth: z.string().refine((v) => {
      const n = Number(v);
      return Number.isInteger(n) && n >= 1 && n <= 31;
    }, "dia entre 1 e 31"),
    accountId: z.string(),
    categoryId: z.string(),
    startDate: z.string(),
    endDate: z.string(),
    active: z.boolean(),
  })
  .refine((v) => !v.endDate || !v.startDate || v.endDate >= v.startDate, {
    message: "o fim deve ser depois do início",
    path: ["endDate"],
  });
type RecurringForm = z.infer<typeof recurringFormSchema>;

const emptyRecurring = (): RecurringForm => ({
  description: "",
  kind: "EXPENSE",
  amount: "",
  dayOfMonth: String(new Date().getDate()),
  accountId: "",
  categoryId: "",
  startDate: todayISO(),
  endDate: "",
  active: true,
});

function RecurringSheet({
  open,
  onOpenChange,
  editing,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editing: Recurring | null;
}) {
  const accounts = useAccounts();
  const create = useCreateRecurring();
  const update = useUpdateRecurring();
  const form = useForm<RecurringForm>({
    resolver: zodResolver(recurringFormSchema),
    defaultValues: emptyRecurring(),
  });
  const kind = form.watch("kind");
  const categories = useCategories(kind);

  useEffect(() => {
    if (editing) {
      form.reset({
        description: editing.description,
        kind: editing.kind,
        amount: centsToInput(editing.amountCents),
        dayOfMonth: String(editing.dayOfMonth),
        accountId: editing.accountId ?? "",
        categoryId: editing.categoryId ?? "",
        startDate: editing.startDate,
        endDate: editing.endDate ?? "",
        active: editing.active,
      });
    } else if (open) {
      form.reset(emptyRecurring());
    }
  }, [editing, open, form]);

  const submit = form.handleSubmit(async (values) => {
    const payload = {
      description: values.description.trim(),
      kind: values.kind,
      amountCents: parseBRLInput(values.amount) ?? 0,
      dayOfMonth: Number(values.dayOfMonth),
      accountId: values.accountId || null,
      categoryId: values.categoryId || null,
      startDate: values.startDate || undefined,
      endDate: values.endDate || null,
    };
    if (editing) {
      await update.mutateAsync({ id: editing.id, ...payload, active: values.active });
    } else {
      await create.mutateAsync(payload);
    }
    onOpenChange(false);
  });

  const isPending = create.isPending || update.isPending;
  const error = create.error?.message ?? update.error?.message;
  const errors = form.formState.errors;

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? `Editar — ${editing.description}` : "Nova recorrência"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-recurring">
        <Segmented
          label="Tipo"
          value={kind}
          onChange={(v) => {
            form.setValue("kind", v);
            form.setValue("categoryId", "");
          }}
          options={[
            ["EXPENSE", "Conta fixa"],
            ["INCOME", "Receita recorrente"],
          ]}
        />
        <Field label="Descrição" error={errors.description?.message}>
          <Input placeholder="Aluguel, internet, salário…" {...form.register("description")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Valor" error={errors.amount?.message}>
            <Input inputMode="decimal" placeholder="1.200,00" {...form.register("amount")} />
          </Field>
          <Field label="Dia do mês" error={errors.dayOfMonth?.message}>
            <Input inputMode="numeric" placeholder="10" {...form.register("dayOfMonth")} />
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Conta">
            <Select {...form.register("accountId")}>
              <option value="">Escolher ao lançar</option>
              {accounts.data?.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Categoria">
            <Select {...form.register("categoryId")}>
              <option value="">Sem categoria</option>
              {categories.data?.map((c) => (
                <option key={c.id} value={c.id}>
                  {formatCategoryLabel(c, categories.data)}
                </option>
              ))}
            </Select>
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Começa em">
            <Input type="date" {...form.register("startDate")} />
          </Field>
          <Field label="Termina em (opcional)" error={errors.endDate?.message}>
            <Input type="date" {...form.register("endDate")} />
          </Field>
        </div>
        {editing ? (
          <label className="flex min-h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text">
            <input
              type="checkbox"
              className="h-4 w-4 accent-emerald-500"
              {...form.register("active")}
            />
            Ativa (entra na projeção e nos compromissos)
          </label>
        ) : null}
        <p className="text-xs text-app-muted">
          Em meses mais curtos, o lançamento cai no último dia.
        </p>
        {error ? <p className="text-sm text-expense">{error}</p> : null}
        <Button type="submit" disabled={isPending} className="h-12">
          {isPending ? "Salvando…" : editing ? "Salvar alterações" : "Cadastrar recorrência"}
        </Button>
      </form>
    </Sheet>
  );
}
