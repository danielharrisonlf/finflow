import type { Goal, WishItem, WishStatus } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import * as Tabs from "@radix-ui/react-tabs";
import {
  Archive,
  ArrowDown,
  ArrowUp,
  ExternalLink,
  Gift,
  Pencil,
  PiggyBank,
  Plus,
  RotateCcw,
  ShieldCheck,
  ShieldPlus,
  ShoppingBag,
  Trash2,
} from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { formatMonths } from "../components/insights";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { CategoryIcon } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input } from "../components/ui/field";
import { Progress } from "../components/ui/progress";
import { Badge, Segmented } from "../components/ui/segmented";
import { Sheet } from "../components/ui/sheet";
import { cn } from "../lib/cn";
import { centsToInput, formatBRL, formatDate, formatLongDate, parseBRLInput } from "../lib/format";
import {
  useContributeGoal,
  useCreateGoal,
  useCreateWish,
  useDeleteWish,
  useGoals,
  useInsightsOverview,
  useReorderWishes,
  useSettings,
  useUpdateWish,
  useWishlist,
} from "../lib/queries";

const tabCls =
  "min-h-10 flex-1 cursor-pointer rounded-(--radius-control) px-4 text-sm text-app-muted transition-colors data-[state=active]:bg-app-soft data-[state=active]:font-medium data-[state=active]:text-app-text sm:flex-none";

interface GoalPreset {
  kind: "RESERVE";
  targetCents: number;
}

export function GoalsPage() {
  const goals = useGoals();
  const [tab, setTab] = useState("goals");
  const [createOpen, setCreateOpen] = useState(false);
  const [preset, setPreset] = useState<GoalPreset | null>(null);
  const [contributing, setContributing] = useState<Goal | null>(null);
  const [wishSheet, setWishSheet] = useState<{ open: boolean; editing: WishItem | null }>({
    open: false,
    editing: null,
  });

  const regularGoals = (goals.data ?? []).filter((g) => g.kind !== "RESERVE");
  const reserveGoal = goals.data?.find((g) => g.kind === "RESERVE") ?? null;

  const openCreate = (p: GoalPreset | null = null): void => {
    setPreset(p);
    setCreateOpen(true);
  };

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Metas e desejos</h1>
          <p className="text-sm text-app-muted">
            Caixinhas, reserva de emergência e lista de compras
          </p>
        </div>
        {tab === "goals" ? (
          <Button variant="outline" size="sm" onClick={() => openCreate()}>
            <Plus size={15} /> Nova meta
          </Button>
        ) : (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setWishSheet({ open: true, editing: null })}
          >
            <Plus size={15} /> Novo desejo
          </Button>
        )}
      </header>

      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List
          className="flex gap-1 rounded-(--radius-control) bg-app-background/70 p-1 sm:inline-flex"
          aria-label="Seções"
        >
          <Tabs.Trigger value="goals" className={tabCls}>
            Metas
          </Tabs.Trigger>
          <Tabs.Trigger value="wishes" className={tabCls}>
            Desejos
          </Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="goals" className="mt-4 space-y-4 outline-none">
          <ReservePanel
            goal={reserveGoal}
            onCreate={(targetCents) => openCreate({ kind: "RESERVE", targetCents })}
            onContribute={setContributing}
          />

          {goals.isLoading ? (
            <Card>
              <LoadingRows rows={3} />
            </Card>
          ) : regularGoals.length === 0 ? (
            <Card>
              <Empty
                icon={PiggyBank}
                title="Nenhuma meta ainda"
                hint="Crie uma caixinha (viagem, carro, um sonho) e acompanhe quanto falta."
                action={<Button onClick={() => openCreate()}>Criar primeira meta</Button>}
              />
            </Card>
          ) : (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3" data-testid="goals-grid">
              {regularGoals.map((goal) => (
                <GoalCard key={goal.id} goal={goal} onContribute={() => setContributing(goal)} />
              ))}
            </div>
          )}
        </Tabs.Content>

        <Tabs.Content value="wishes" className="mt-4 outline-none">
          <WishlistPanel
            onCreate={() => setWishSheet({ open: true, editing: null })}
            onEdit={(item) => setWishSheet({ open: true, editing: item })}
          />
        </Tabs.Content>
      </Tabs.Root>

      <NewGoalSheet open={createOpen} onOpenChange={setCreateOpen} preset={preset} />
      <ContributeSheet goal={contributing} onClose={() => setContributing(null)} />
      <WishSheet
        open={wishSheet.open}
        editing={wishSheet.editing}
        onOpenChange={(open) => setWishSheet((s) => ({ open, editing: open ? s.editing : null }))}
      />
    </div>
  );
}

function GoalCard({ goal, onContribute }: { goal: Goal; onContribute: () => void }) {
  return (
    <Card className="flex flex-col gap-4 p-5">
      <div className="flex items-center gap-3">
        <CategoryIcon icon={goal.icon} color={goal.color} size={40} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-app-text">{goal.name}</p>
          <p className="text-xs text-app-muted">
            {goal.deadline ? `até ${formatDate(goal.deadline)}` : "sem prazo"}
          </p>
        </div>
        <span className="tnum text-sm font-semibold text-app-text">{goal.percent}%</span>
      </div>
      <div>
        <Progress
          percent={goal.percent}
          tone={goal.percent >= 100 ? "OK" : "NEUTRAL"}
          className="h-2"
        />
        <div className="mt-2 flex items-baseline justify-between">
          <p className="tnum text-lg font-semibold text-app-text">{formatBRL(goal.savedCents)}</p>
          <p className="tnum text-xs text-app-muted">de {formatBRL(goal.targetCents)}</p>
        </div>
        {goal.monthlyNeededCents !== null ? (
          <p className="mt-1 text-xs text-app-muted">
            guarde{" "}
            <span className="tnum font-medium text-income">
              {formatBRL(goal.monthlyNeededCents)}/mês
            </span>{" "}
            para chegar lá no prazo
          </p>
        ) : goal.percent >= 100 ? (
          <p className="mt-1 text-xs text-income">meta concluída 🎉</p>
        ) : null}
      </div>
      <Button variant="outline" size="sm" onClick={onContribute}>
        Guardar / resgatar
      </Button>
    </Card>
  );
}

// ───────────────────────── reserva de emergência ─────────────────────────

function ReservePanel({
  goal,
  onCreate,
  onContribute,
}: {
  goal: Goal | null;
  onCreate: (targetCents: number) => void;
  onContribute: (goal: Goal) => void;
}) {
  const overview = useInsightsOverview();
  const settings = useSettings();
  const r = overview.data?.reserve;
  const months = settings.data
    ? settings.data.hasDependents
      ? 12
      : settings.data.reserveMonths
    : null;
  const refill = settings.data?.reserveRefillRatePct ?? 10;

  return (
    <Card className="p-5" data-testid="reserve-panel">
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:gap-8">
        <div className="flex items-start gap-3 md:w-2/5">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-emerald-500/12 text-income">
            <ShieldCheck size={20} aria-hidden />
          </span>
          <div className="min-w-0">
            <h2 className="text-sm font-semibold text-app-text">Reserva de emergência</h2>
            <p className="mt-0.5 text-sm text-app-muted">
              Dinheiro guardado para imprevistos: {months ?? 6} meses do seu custo de vida
              {settings.data?.hasDependents ? " (você tem dependentes)" : " (12 com dependentes)"}.
            </p>
          </div>
        </div>

        <div className="min-w-0 flex-1">
          {goal ? (
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <p className="tnum text-sm text-app-muted">
                  <span className="text-2xl font-semibold text-app-text">
                    {formatBRL(goal.savedCents)}
                  </span>{" "}
                  de {formatBRL(goal.targetCents)}
                </p>
                {r?.status === "PROTECTED" ? (
                  <Badge tone="income">
                    <ShieldCheck size={12} aria-hidden /> Protegido
                  </Badge>
                ) : null}
              </div>
              <Progress
                percent={goal.percent}
                tone={goal.percent >= 100 ? "OK" : "NEUTRAL"}
                className="h-2"
              />
              <p className="text-sm text-app-text">
                {r ? (
                  <>
                    Cobre{" "}
                    <span className="tnum font-semibold">{formatMonths(r.monthsCovered)}</span> do
                    seu custo de vida ({formatBRL(r.avgMonthlyCostCents)}/mês em média)
                  </>
                ) : (
                  `${goal.percent}% da meta`
                )}
              </p>
              <p className="text-xs text-app-muted">
                Usou a reserva? Reponha com +{refill}%: para cada {formatBRL(10_000)} retirados,
                devolva {formatBRL(Math.round(10_000 * (1 + refill / 100)))}.
              </p>
              <Button
                variant="outline"
                size="sm"
                className="self-start"
                onClick={() => onContribute(goal)}
              >
                Guardar / resgatar
              </Button>
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              {r && r.suggestedTargetCents > 0 ? (
                <p className="text-sm text-app-text">
                  Meta sugerida:{" "}
                  <span className="tnum text-lg font-semibold">
                    {formatBRL(r.suggestedTargetCents)}
                  </span>
                  <span className="block text-xs text-app-muted">
                    {formatBRL(r.avgMonthlyCostCents)}/mês de custo médio (últimos 3 meses) ×{" "}
                    {months ?? 6} meses
                  </span>
                </p>
              ) : (
                <p className="text-sm text-app-muted">
                  Ainda não há meses completos de gastos para sugerir um valor. Você pode criar a
                  reserva com um valor próprio.
                </p>
              )}
              <Button className="self-start" onClick={() => onCreate(r?.suggestedTargetCents ?? 0)}>
                <ShieldPlus size={16} aria-hidden /> Criar reserva de emergência
              </Button>
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}

// ───────────────────────── desejos ─────────────────────────

const WISH_TABS = [
  ["LISTED", "Na lista"],
  ["BOUGHT", "Comprados"],
  ["ARCHIVED", "Arquivados"],
] as const;

const iconBtn =
  "flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-muted hover:bg-app-soft hover:text-app-text disabled:cursor-not-allowed disabled:opacity-30 md:h-9 md:w-9";

function WishlistPanel({
  onCreate,
  onEdit,
}: {
  onCreate: () => void;
  onEdit: (item: WishItem) => void;
}) {
  const wishlist = useWishlist();
  const update = useUpdateWish();
  const remove = useDeleteWish();
  const reorder = useReorderWishes();
  const [status, setStatus] = useState<WishStatus>("LISTED");

  const all = wishlist.data ?? [];
  const items = all.filter((w) => w.status === status).sort((a, b) => a.priority - b.priority);
  const listedTotal = all
    .filter((w) => w.status === "LISTED")
    .reduce((acc, w) => acc + w.priceCents, 0);

  const move = (index: number, delta: -1 | 1): void => {
    const next = [...items];
    const target = index + delta;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target]!, next[index]!];
    reorder.mutate(next);
  };

  const onDelete = (item: WishItem): void => {
    if (window.confirm(`Excluir "${item.name}" da lista?`)) remove.mutate(item.id);
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Segmented
          label="Situação dos desejos"
          value={status}
          onChange={setStatus}
          options={WISH_TABS.map(
            ([key, label]) =>
              [key, `${label} (${all.filter((w) => w.status === key).length})`] as const,
          )}
        />
        {status === "LISTED" && listedTotal > 0 ? (
          <p className="tnum text-sm text-app-muted">
            Total da lista:{" "}
            <span className="font-semibold text-app-text">{formatBRL(listedTotal)}</span>
          </p>
        ) : null}
      </div>

      <Card>
        {wishlist.isLoading ? (
          <LoadingRows rows={3} />
        ) : items.length === 0 ? (
          status === "LISTED" ? (
            <Empty
              icon={Gift}
              title="Sua lista de desejos está vazia"
              hint="Anote o que quer comprar, em ordem de prioridade. O FinFlow mostra quando cada item cabe no seu saldo sem te deixar no vermelho."
              action={<Button onClick={onCreate}>Adicionar desejo</Button>}
            />
          ) : (
            <Empty
              icon={status === "BOUGHT" ? ShoppingBag : Archive}
              title={status === "BOUGHT" ? "Nada comprado ainda" : "Nada arquivado"}
            />
          )
        ) : (
          <ol className="divide-y divide-app-border/60" data-testid="wish-list">
            {items.map((item, index) => (
              <li key={item.id} className="flex flex-wrap items-center gap-3 px-4 py-3 md:px-5">
                {status === "LISTED" ? (
                  <span className="tnum flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-app-soft text-xs font-semibold text-app-text">
                    {index + 1}
                  </span>
                ) : null}
                <div className="min-w-0 flex-1 basis-40">
                  <p className="flex items-center gap-1.5 text-sm font-medium text-app-text">
                    <span className="truncate">{item.name}</span>
                    {item.url ? (
                      <a
                        href={item.url}
                        target="_blank"
                        rel="noreferrer noopener"
                        aria-label={`Abrir link de ${item.name}`}
                        className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-app-muted hover:text-app-text"
                      >
                        <ExternalLink size={13} />
                      </a>
                    ) : null}
                  </p>
                  {status === "LISTED" ? (
                    <p
                      className={cn(
                        "text-xs",
                        item.affordableOn ? "text-income" : "text-app-muted",
                      )}
                      data-testid="wish-affordable"
                    >
                      {item.affordableOn
                        ? `Cabe no seu saldo em ${formatLongDate(item.affordableOn)}`
                        : "Ainda não cabe nos próximos 12 meses"}
                    </p>
                  ) : item.notes ? (
                    <p className="truncate text-xs text-app-muted">{item.notes}</p>
                  ) : null}
                </div>
                <span className="tnum text-sm font-semibold text-app-text">
                  {formatBRL(item.priceCents)}
                </span>
                <div className="flex items-center">
                  {status === "LISTED" ? (
                    <>
                      <button
                        type="button"
                        className={iconBtn}
                        disabled={index === 0 || reorder.isPending}
                        onClick={() => move(index, -1)}
                        aria-label={`Subir prioridade de ${item.name}`}
                      >
                        <ArrowUp size={15} />
                      </button>
                      <button
                        type="button"
                        className={iconBtn}
                        disabled={index === items.length - 1 || reorder.isPending}
                        onClick={() => move(index, 1)}
                        aria-label={`Descer prioridade de ${item.name}`}
                      >
                        <ArrowDown size={15} />
                      </button>
                      <button
                        type="button"
                        className={iconBtn}
                        onClick={() => update.mutate({ id: item.id, status: "BOUGHT" })}
                        aria-label={`Marcar ${item.name} como comprado`}
                        title="Comprei"
                      >
                        <ShoppingBag size={15} />
                      </button>
                      <button
                        type="button"
                        className={iconBtn}
                        onClick={() => update.mutate({ id: item.id, status: "ARCHIVED" })}
                        aria-label={`Arquivar ${item.name}`}
                        title="Arquivar"
                      >
                        <Archive size={15} />
                      </button>
                    </>
                  ) : (
                    <button
                      type="button"
                      className={iconBtn}
                      onClick={() => update.mutate({ id: item.id, status: "LISTED" })}
                      aria-label={`Voltar ${item.name} para a lista`}
                      title="Voltar para a lista"
                    >
                      <RotateCcw size={15} />
                    </button>
                  )}
                  <button
                    type="button"
                    className={iconBtn}
                    onClick={() => onEdit(item)}
                    aria-label={`Editar ${item.name}`}
                  >
                    <Pencil size={14} />
                  </button>
                  <button
                    type="button"
                    className={cn(iconBtn, "hover:bg-rose-500/10 hover:text-expense")}
                    onClick={() => onDelete(item)}
                    aria-label={`Excluir ${item.name}`}
                  >
                    <Trash2 size={15} />
                  </button>
                </div>
              </li>
            ))}
          </ol>
        )}
      </Card>
      {update.error || reorder.error || remove.error ? (
        <p className="text-sm text-expense" role="alert">
          {(update.error ?? reorder.error ?? remove.error)?.message}
        </p>
      ) : null}
    </div>
  );
}

const wishSchema = z.object({
  name: z.string().trim().min(1, "o que você quer comprar?").max(120),
  price: z.string().refine((v) => parseBRLInput(v) !== null, "valor inválido — use 899,90"),
  url: z
    .string()
    .refine((v) => v.trim() === "" || /^https?:\/\//i.test(v.trim()), "use um link http(s)"),
  notes: z.string().max(500),
});
type WishForm = z.infer<typeof wishSchema>;

function WishSheet({
  open,
  editing,
  onOpenChange,
}: {
  open: boolean;
  editing: WishItem | null;
  onOpenChange: (open: boolean) => void;
}) {
  const create = useCreateWish();
  const update = useUpdateWish();
  const form = useForm<WishForm>({
    resolver: zodResolver(wishSchema),
    defaultValues: { name: "", price: "", url: "", notes: "" },
  });

  useEffect(() => {
    if (!open) return;
    form.reset(
      editing
        ? {
            name: editing.name,
            price: centsToInput(editing.priceCents),
            url: editing.url ?? "",
            notes: editing.notes ?? "",
          }
        : { name: "", price: "", url: "", notes: "" },
    );
  }, [open, editing, form]);

  const submit = form.handleSubmit(async (v) => {
    const payload = {
      name: v.name.trim(),
      priceCents: parseBRLInput(v.price) ?? 0,
      url: v.url.trim() || null,
      notes: v.notes.trim() || null,
    };
    if (editing) await update.mutateAsync({ id: editing.id, ...payload });
    else await create.mutateAsync(payload);
    onOpenChange(false);
  });

  const error = create.error?.message ?? update.error?.message;
  const pending = create.isPending || update.isPending;
  const errors = form.formState.errors;

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? "Editar desejo" : "Novo desejo"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-wish">
        <Field label="O que você quer" error={errors.name?.message}>
          <Input placeholder="Fone com cancelamento de ruído…" {...form.register("name")} />
        </Field>
        <Field label="Preço" error={errors.price?.message}>
          <Input inputMode="decimal" placeholder="899,90" {...form.register("price")} />
        </Field>
        <Field label="Link (opcional)" error={errors.url?.message}>
          <Input type="url" inputMode="url" placeholder="https://…" {...form.register("url")} />
        </Field>
        <Field label="Observações (opcional)">
          <Input placeholder="Esperar a promoção…" {...form.register("notes")} />
        </Field>
        {error ? <p className="text-sm text-expense">{error}</p> : null}
        <Button type="submit" disabled={pending} className="h-12">
          {pending ? "Salvando…" : editing ? "Salvar alterações" : "Adicionar à lista"}
        </Button>
      </form>
    </Sheet>
  );
}

// ───────────────────────── metas ─────────────────────────

const goalSchema = z.object({
  name: z.string().min(1, "dê um nome à meta"),
  target: z.string().refine((v) => parseBRLInput(v) !== null, "valor inválido"),
  deadline: z.string(),
});

function NewGoalSheet({
  open,
  onOpenChange,
  preset,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  preset: GoalPreset | null;
}) {
  const create = useCreateGoal();
  const form = useForm<z.infer<typeof goalSchema>>({
    resolver: zodResolver(goalSchema),
    defaultValues: { name: "", target: "", deadline: "" },
  });
  const isReserve = preset?.kind === "RESERVE";

  // biome-ignore lint/correctness/useExhaustiveDependencies: reinicia só ao abrir
  useEffect(() => {
    if (!open) return;
    create.reset();
    form.reset(
      isReserve
        ? {
            name: "Reserva de emergência",
            target: preset.targetCents > 0 ? centsToInput(preset.targetCents) : "",
            deadline: "",
          }
        : { name: "", target: "", deadline: "" },
    );
  }, [open]);

  const submit = form.handleSubmit(async (values) => {
    await create.mutateAsync({
      name: values.name,
      icon: isReserve ? "shield" : "piggy-bank",
      targetCents: parseBRLInput(values.target) ?? 0,
      deadline: values.deadline || null,
      ...(isReserve ? { kind: "RESERVE" as const } : {}),
    });
    form.reset();
    onOpenChange(false);
  });

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={isReserve ? "Criar reserva de emergência" : "Nova meta"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-goal">
        <Field label="Nome" error={form.formState.errors.name?.message}>
          <Input placeholder="Viagem, carro novo…" {...form.register("name")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Valor alvo" error={form.formState.errors.target?.message}>
            <Input inputMode="decimal" placeholder="10.000,00" {...form.register("target")} />
          </Field>
          <Field label="Prazo (opcional)">
            <Input type="date" {...form.register("deadline")} />
          </Field>
        </div>
        {isReserve ? (
          <p className="text-xs text-app-muted">
            Valor sugerido a partir do seu custo de vida médio. Ajuste se quiser.
          </p>
        ) : null}
        {create.error ? <p className="text-sm text-expense">{create.error.message}</p> : null}
        <Button type="submit" disabled={create.isPending} className="h-12">
          {create.isPending ? "Criando…" : isReserve ? "Criar reserva" : "Criar meta"}
        </Button>
      </form>
    </Sheet>
  );
}

function ContributeSheet({ goal, onClose }: { goal: Goal | null; onClose: () => void }) {
  const contribute = useContributeGoal();
  const [amount, setAmount] = useState("");
  const [direction, setDirection] = useState<"in" | "out">("in");

  const submit = async (): Promise<void> => {
    if (!goal) return;
    const cents = parseBRLInput(amount);
    if (!cents) return;
    await contribute.mutateAsync({
      id: goal.id,
      amountCents: direction === "in" ? cents : -cents,
    });
    setAmount("");
    onClose();
  };

  return (
    <Sheet
      open={Boolean(goal)}
      onOpenChange={(open) => !open && onClose()}
      title={goal ? `${goal.name} — ${formatBRL(goal.savedCents)} guardados` : ""}
    >
      <div className="flex flex-col gap-4 pb-2">
        <div className="flex gap-1 rounded-(--radius-control) bg-app-background/70 p-1">
          <button
            type="button"
            onClick={() => setDirection("in")}
            className={`h-10 flex-1 cursor-pointer rounded-(--radius-control) text-sm transition-colors ${direction === "in" ? "bg-app-soft font-medium text-income" : "text-app-muted"}`}
          >
            Guardar
          </button>
          <button
            type="button"
            onClick={() => setDirection("out")}
            className={`h-10 flex-1 cursor-pointer rounded-(--radius-control) text-sm transition-colors ${direction === "out" ? "bg-app-soft font-medium text-expense" : "text-app-muted"}`}
          >
            Resgatar
          </button>
        </div>
        <Field label="Valor">
          <Input
            inputMode="decimal"
            placeholder="200,00"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>
        {contribute.error ? (
          <p className="text-sm text-expense">{contribute.error.message}</p>
        ) : null}
        <Button
          disabled={contribute.isPending || parseBRLInput(amount) === null}
          onClick={() => void submit()}
          className="h-12"
        >
          {contribute.isPending
            ? "Salvando…"
            : direction === "in"
              ? "Guardar na meta"
              : "Resgatar da meta"}
        </Button>
      </div>
    </Sheet>
  );
}
