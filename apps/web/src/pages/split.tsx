import type { Account, Member, SuggestedSettlement } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowRight, Handshake, House, Pencil, Plus, Trash2, Users } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { Link } from "react-router-dom";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card, CardHeader } from "../components/ui/card";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Money } from "../components/ui/money";
import { MonthSwitcher, usePeriodState } from "../components/ui/month-switcher";
import { Sheet } from "../components/ui/sheet";
import { cn } from "../lib/cn";
import { centsToInput, formatBRL, parseBRLInput, todayISO } from "../lib/format";
import {
  useAccounts,
  useCreateMember,
  useDeleteMember,
  useMemberBalances,
  useMembers,
  useSettleMembers,
  useUpdateMember,
} from "../lib/queries";
import { useActiveSpace } from "../lib/use-active-space";

const MEMBER_COLORS = ["#38bdf8", "#f472b6", "#a78bfa", "#fb923c", "#34d399", "#facc15"] as const;

const iconBtn =
  "flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-muted hover:bg-app-soft hover:text-app-text md:h-9 md:w-9";

/** Divisão de despesas do espaço Casa (v3 fase 2 §4). */
export function SplitPage() {
  const { kind } = useActiveSpace();
  if (kind !== "HOME") {
    return (
      <div className="space-y-4">
        <header>
          <h1 className="text-xl font-semibold tracking-tight">Divisão de despesas</h1>
          <p className="text-sm text-app-muted">Quem pagou, quem deve e como acertar</p>
        </header>
        <Card>
          <Empty
            icon={House}
            title="Disponível em espaços do tipo Casa"
            hint="Crie ou abra um espaço Casa para cadastrar as pessoas e dividir as despesas em comum."
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
  return <SplitContent />;
}

function SplitContent() {
  const [period, shift] = usePeriodState();
  const members = useMembers();
  const balances = useMemberBalances(period);
  const accounts = useAccounts();
  const remove = useDeleteMember();
  const [memberSheet, setMemberSheet] = useState<{ open: boolean; editing: Member | null }>({
    open: false,
    editing: null,
  });
  const [settle, setSettle] = useState<{ open: boolean; preset: SuggestedSettlement | null }>({
    open: false,
    preset: null,
  });

  const list = members.data ?? [];
  const nameOf = (id: string): string => list.find((m) => m.id === id)?.name ?? "Alguém";
  const colorOf = (id: string): string =>
    list.find((m) => m.id === id)?.color ?? "var(--ff-subtle)";
  const suggestions = balances.data?.settlements ?? [];

  const onDelete = (member: Member): void => {
    if (
      window.confirm(
        `Remover ${member.name}? As despesas continuam no extrato, sem a parte dessa pessoa.`,
      )
    ) {
      remove.mutate(member.id);
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Divisão de despesas</h1>
          <p className="text-sm text-app-muted">Quem pagou, quem deve e como acertar</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <MonthSwitcher period={period} onShift={shift} />
          <Button
            variant="outline"
            size="sm"
            disabled={list.length < 2}
            onClick={() => setSettle({ open: true, preset: null })}
          >
            <Handshake size={15} aria-hidden /> Registrar acerto
          </Button>
        </div>
      </header>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-12">
        <Card className="lg:col-span-5" data-testid="members-card">
          <CardHeader
            title="Pessoas da casa"
            action={
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setMemberSheet({ open: true, editing: null })}
              >
                <Plus size={15} aria-hidden /> Adicionar
              </Button>
            }
          />
          {members.isLoading ? (
            <LoadingRows rows={3} />
          ) : members.isError ? (
            <Empty
              icon={Users}
              title="Não foi possível carregar as pessoas"
              hint={members.error.message}
            />
          ) : list.length === 0 ? (
            <Empty
              icon={Users}
              title="Ninguém cadastrado ainda"
              hint="Cadastre quem divide as contas da casa. Despesas sem divisão definida ficam iguais para todos."
              action={
                <Button onClick={() => setMemberSheet({ open: true, editing: null })}>
                  Adicionar pessoa
                </Button>
              }
            />
          ) : (
            <ul className="divide-y divide-app-border/60 px-2 pb-2" data-testid="members-list">
              {list.map((m) => (
                <li key={m.id} className="flex items-center gap-3 px-2 py-1.5">
                  <MemberDot color={m.color} name={m.name} />
                  <span className="min-w-0 flex-1 truncate text-sm font-medium text-app-text">
                    {m.name}
                  </span>
                  <button
                    type="button"
                    className={iconBtn}
                    onClick={() => setMemberSheet({ open: true, editing: m })}
                    aria-label={`Editar ${m.name}`}
                  >
                    <Pencil size={14} />
                  </button>
                  <button
                    type="button"
                    className={cn(iconBtn, "hover:bg-rose-500/10 hover:text-expense")}
                    onClick={() => onDelete(m)}
                    aria-label={`Remover ${m.name}`}
                  >
                    <Trash2 size={15} />
                  </button>
                </li>
              ))}
            </ul>
          )}
          {remove.error ? (
            <p className="px-5 pb-4 text-sm text-expense" role="alert">
              {remove.error.message}
            </p>
          ) : null}
        </Card>

        <Card className="lg:col-span-7" data-testid="balances-card">
          <CardHeader title="Saldos do mês" />
          {balances.isLoading ? (
            <LoadingRows rows={3} />
          ) : balances.isError ? (
            <Empty
              icon={Handshake}
              title="Não foi possível calcular os saldos"
              hint={balances.error.message}
            />
          ) : (balances.data?.members ?? []).length === 0 ? (
            <Empty
              icon={Handshake}
              title="Nada para dividir neste mês"
              hint="Lance despesas da casa com quem pagou para ver quem tem a receber."
            />
          ) : (
            <ul
              className="divide-y divide-app-border/60 px-4 pb-2 md:px-5"
              data-testid="balances-list"
            >
              {(balances.data?.members ?? []).map((b) => (
                <li key={b.memberId} className="flex flex-col gap-1 py-3">
                  <div className="flex items-center gap-3">
                    <MemberDot color={colorOf(b.memberId)} name={b.name} />
                    <span className="min-w-0 flex-1 truncate text-sm font-medium text-app-text">
                      {b.name}
                    </span>
                    <div className="flex shrink-0 flex-col items-end">
                      <Money cents={b.balanceCents} signed className="text-sm font-semibold" />
                      <span className="text-[11px] text-app-muted">
                        {b.balanceCents > 0
                          ? "tem a receber"
                          : b.balanceCents < 0
                            ? "deve"
                            : "em dia"}
                      </span>
                    </div>
                  </div>
                  <p className="pl-11 text-xs text-app-muted">
                    pagou <span className="tnum text-app-text">{formatBRL(b.paidCents)}</span> · sua
                    parte <span className="tnum text-app-text">{formatBRL(b.owedCents)}</span>
                  </p>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>

      <Card data-testid="settlements-card">
        <CardHeader title="Acertos sugeridos" />
        {balances.isLoading ? (
          <LoadingRows rows={2} />
        ) : suggestions.length === 0 ? (
          <p className="px-5 pt-1 pb-5 text-sm text-app-muted">
            Ninguém deve nada a ninguém neste mês.
          </p>
        ) : (
          <ul className="flex flex-col gap-2 px-4 pt-1 pb-4 md:px-5" data-testid="settlements-list">
            {suggestions.map((s) => (
              <li
                key={`${s.fromMemberId}-${s.toMemberId}`}
                className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-(--radius-control) border border-app-border/80 bg-app-background/60 p-3"
              >
                <p className="flex min-w-0 flex-1 basis-52 flex-wrap items-center gap-x-1.5 text-sm text-app-text">
                  <span className="font-medium">{nameOf(s.fromMemberId)}</span>
                  <ArrowRight size={14} aria-label="paga para" className="text-app-muted" />
                  <span className="font-medium">{nameOf(s.toMemberId)}</span>
                  <Money cents={s.amountCents} neutral className="font-semibold" />
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setSettle({ open: true, preset: s })}
                  aria-label={`Registrar acerto de ${nameOf(s.fromMemberId)} para ${nameOf(s.toMemberId)}`}
                >
                  <Handshake size={14} aria-hidden /> Registrar acerto
                </Button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <p className="text-xs text-app-muted">
        Despesas sem divisão definida ficam iguais entre todos; os centavos que sobram ficam com
        quem pagou. Saldo positivo = tem a receber.
      </p>

      <MemberSheet
        open={memberSheet.open}
        editing={memberSheet.editing}
        onOpenChange={(open) => setMemberSheet((s) => ({ open, editing: open ? s.editing : null }))}
      />
      <SettleSheet
        open={settle.open}
        preset={settle.preset}
        members={list}
        accounts={accounts.data ?? []}
        onOpenChange={(open) => setSettle((s) => ({ open, preset: open ? s.preset : null }))}
      />
    </div>
  );
}

function MemberDot({ color, name }: { color: string; name: string }) {
  return (
    <span
      className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold text-white"
      style={{ backgroundColor: color }}
      aria-hidden
    >
      {name.trim().charAt(0).toUpperCase()}
    </span>
  );
}

// ───────────────────────── pessoa ─────────────────────────

const memberFormSchema = z.object({
  name: z.string().trim().min(1, "informe o nome").max(60),
  color: z.string().regex(/^#[0-9a-fA-F]{6}$/, "cor hexadecimal inválida"),
});
type MemberFormValues = z.infer<typeof memberFormSchema>;

function MemberSheet({
  open,
  editing,
  onOpenChange,
}: {
  open: boolean;
  editing: Member | null;
  onOpenChange: (open: boolean) => void;
}) {
  const create = useCreateMember();
  const update = useUpdateMember();
  const form = useForm<MemberFormValues>({
    resolver: zodResolver(memberFormSchema),
    defaultValues: { name: "", color: MEMBER_COLORS[0] },
  });
  const color = form.watch("color");

  useEffect(() => {
    if (!open) return;
    form.reset(
      editing
        ? { name: editing.name, color: editing.color }
        : { name: "", color: MEMBER_COLORS[0] },
    );
  }, [open, editing, form]);

  const submit = form.handleSubmit(async (values) => {
    const payload = { name: values.name.trim(), color: values.color };
    if (editing) await update.mutateAsync({ id: editing.id, ...payload });
    else await create.mutateAsync(payload);
    onOpenChange(false);
  });

  const pending = create.isPending || update.isPending;
  const error = create.error?.message ?? update.error?.message;

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? `Editar — ${editing.name}` : "Nova pessoa"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-member">
        <Field label="Nome" error={form.formState.errors.name?.message}>
          <Input placeholder="Ana, Bruno…" autoComplete="off" {...form.register("name")} />
        </Field>
        <Field label="Cor" error={form.formState.errors.color?.message}>
          <div className="flex flex-wrap gap-1">
            {MEMBER_COLORS.map((hex) => (
              <button
                key={hex}
                type="button"
                onClick={() => form.setValue("color", hex)}
                aria-label={`Cor ${hex}`}
                aria-pressed={color === hex}
                className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-full"
              >
                <span
                  className={cn(
                    "h-7 w-7 rounded-full transition-transform",
                    color === hex &&
                      "scale-110 ring-2 ring-app-text ring-offset-2 ring-offset-app-surface",
                  )}
                  style={{ backgroundColor: hex }}
                />
              </button>
            ))}
          </div>
        </Field>
        {error ? <p className="text-sm text-expense">{error}</p> : null}
        <Button type="submit" disabled={pending} className="h-12">
          {pending ? "Salvando…" : editing ? "Salvar alterações" : "Adicionar pessoa"}
        </Button>
      </form>
    </Sheet>
  );
}

// ───────────────────────── acerto ─────────────────────────

const settleFormSchema = z
  .object({
    fromMemberId: z.string().min(1, "quem paga?"),
    toMemberId: z.string().min(1, "quem recebe?"),
    amount: z.string().refine((v) => parseBRLInput(v) !== null, "valor inválido — use 45,90"),
    date: z.string().min(10, "informe a data"),
    fromAccountId: z.string(),
    toAccountId: z.string(),
  })
  .refine((v) => v.fromMemberId !== v.toMemberId, {
    message: "escolha duas pessoas diferentes",
    path: ["toMemberId"],
  })
  .refine((v) => !v.fromAccountId || !v.toAccountId || v.fromAccountId !== v.toAccountId, {
    message: "contas de origem e destino devem ser diferentes",
    path: ["toAccountId"],
  });
type SettleFormValues = z.infer<typeof settleFormSchema>;

function SettleSheet({
  open,
  preset,
  members,
  accounts,
  onOpenChange,
}: {
  open: boolean;
  preset: SuggestedSettlement | null;
  members: Member[];
  accounts: Account[];
  onOpenChange: (open: boolean) => void;
}) {
  const settle = useSettleMembers();
  const form = useForm<SettleFormValues>({
    resolver: zodResolver(settleFormSchema),
    defaultValues: {
      fromMemberId: "",
      toMemberId: "",
      amount: "",
      date: todayISO(),
      fromAccountId: "",
      toAccountId: "",
    },
  });
  const errors = form.formState.errors;
  const fromAccount = form.watch("fromAccountId");
  const toAccount = form.watch("toAccountId");

  useEffect(() => {
    if (!open) return;
    form.reset({
      fromMemberId: preset?.fromMemberId ?? "",
      toMemberId: preset?.toMemberId ?? "",
      amount: preset ? centsToInput(preset.amountCents) : "",
      date: todayISO(),
      fromAccountId: "",
      toAccountId: "",
    });
  }, [open, preset, form]);

  const submit = form.handleSubmit(async (values) => {
    await settle.mutateAsync({
      fromMemberId: values.fromMemberId,
      toMemberId: values.toMemberId,
      amountCents: parseBRLInput(values.amount) ?? 0,
      date: values.date,
      fromAccountId: values.fromAccountId || undefined,
      toAccountId: values.toAccountId || undefined,
    });
    onOpenChange(false);
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Registrar acerto">
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-settle">
        <div className="grid grid-cols-2 gap-3">
          <Field label="Quem paga" error={errors.fromMemberId?.message}>
            <Select {...form.register("fromMemberId")}>
              <option value="" disabled>
                Escolher…
              </option>
              {members.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Quem recebe" error={errors.toMemberId?.message}>
            <Select {...form.register("toMemberId")}>
              <option value="" disabled>
                Escolher…
              </option>
              {members.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
            </Select>
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Valor" error={errors.amount?.message}>
            <Input inputMode="decimal" placeholder="0,00" {...form.register("amount")} />
          </Field>
          <Field label="Data" error={errors.date?.message}>
            <Input type="date" {...form.register("date")} />
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Da conta (opcional)">
            <Select {...form.register("fromAccountId")}>
              <option value="">Sem conta</option>
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Para a conta (opcional)" error={errors.toAccountId?.message}>
            <Select {...form.register("toAccountId")}>
              <option value="">Sem conta</option>
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </Select>
          </Field>
        </div>
        <p className="text-xs text-app-muted">
          {fromAccount && toAccount
            ? "Vira uma transferência entre as duas contas e abate os saldos."
            : "Sem as duas contas, fica só o registro do acerto, que abate os saldos."}
        </p>
        {settle.error ? <p className="text-sm text-expense">{settle.error.message}</p> : null}
        <Button type="submit" disabled={settle.isPending} className="h-12">
          {settle.isPending ? "Registrando…" : "Registrar acerto"}
        </Button>
      </form>
    </Sheet>
  );
}
