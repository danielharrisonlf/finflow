import type { Account, DebtPlan, DebtStrategy, Loan, LoanKind } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import { Calculator, HandCoins, Landmark, Pencil, Plus, Trash2, Trophy } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card, CardHeader } from "../components/ui/card";
import { formatCategoryLabel } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Money } from "../components/ui/money";
import { Progress } from "../components/ui/progress";
import { Badge } from "../components/ui/segmented";
import { Sheet } from "../components/ui/sheet";
import { cn } from "../lib/cn";
import {
  centsToInput,
  formatBRL,
  formatDate,
  formatMonthYear,
  parseBRLInput,
  todayISO,
} from "../lib/format";
import {
  useAccounts,
  useCategories,
  useCreateLoan,
  useDebtStrategy,
  useDeleteLoan,
  useLoans,
  usePayLoan,
  useUpdateLoan,
} from "../lib/queries";

const LOAN_KINDS = [
  "PERSONAL",
  "PAYROLL",
  "VEHICLE",
  "MORTGAGE",
  "STUDENT",
  "CARD_DEBT",
  "OVERDRAFT",
  "OTHER",
] as const satisfies readonly LoanKind[];

const KIND_LABEL: Record<LoanKind, string> = {
  PERSONAL: "Pessoal",
  PAYROLL: "Consignado",
  VEHICLE: "Veículo",
  MORTGAGE: "Imobiliário",
  STUDENT: "Estudantil",
  CARD_DEBT: "Cartão (rotativo)",
  OVERDRAFT: "Cheque especial",
  OTHER: "Outro",
};

const rateFmt = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 2 });

/** "1,99" | "1.99" → 1.99; null quando inválido (0 a 100% a.m.). */
function parseRate(raw: string): number | null {
  const value = Number(raw.trim().replace("%", "").replace(",", "."));
  if (raw.trim() === "" || !Number.isFinite(value) || value < 0 || value > 100) return null;
  return value;
}

/** Parcela fixa da tabela Price: P × i ÷ (1 − (1 + i)^−n); sem juros, P ÷ n. */
function priceInstallmentCents(principalCents: number, ratePct: number, n: number): number {
  if (n <= 0) return 0;
  const i = ratePct / 100;
  if (i === 0) return Math.round(principalCents / n);
  return Math.round((principalCents * i) / (1 - (1 + i) ** -n));
}

function monthsLabel(months: number): string {
  return `${months} ${months === 1 ? "mês" : "meses"}`;
}

function monthYearOf(iso: string): string {
  return formatMonthYear(Number(iso.slice(5, 7)), Number(iso.slice(0, 4)));
}

const iconBtn =
  "flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-muted hover:bg-app-soft hover:text-app-text md:h-9 md:w-9";

export function LoansPage() {
  const loans = useLoans();
  const accounts = useAccounts();
  const remove = useDeleteLoan();
  const [sheet, setSheet] = useState<{ open: boolean; editing: Loan | null }>({
    open: false,
    editing: null,
  });
  const [paying, setPaying] = useState<Loan | null>(null);

  const all = loans.data ?? [];
  const active = all.filter((l) => l.active);
  const outstanding = active.reduce((acc, l) => acc + l.outstandingCents, 0);
  const monthly = active
    .filter((l) => l.remainingInstallments > 0)
    .reduce((acc, l) => acc + l.installmentCents, 0);
  const interest = active.reduce((acc, l) => acc + l.totalInterestRemainingCents, 0);

  const openCreate = (): void => setSheet({ open: true, editing: null });
  const onDelete = (loan: Loan): void => {
    if (
      window.confirm(
        `Excluir "${loan.name}"? Parcelas já pagas continuam no extrato, sem o vínculo com a dívida.`,
      )
    ) {
      remove.mutate(loan.id);
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Dívidas</h1>
          <p className="text-sm text-app-muted">
            Empréstimos, financiamentos e o melhor jeito de quitar
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={openCreate}>
          <Plus size={15} /> Nova dívida
        </Button>
      </header>

      <div className="grid gap-3 sm:grid-cols-3" data-testid="loans-totals">
        <Card className="p-4">
          <p className="text-sm text-app-muted">Saldo devedor</p>
          <p className="tnum mt-1 text-2xl font-semibold text-app-text">
            {loans.data ? formatBRL(outstanding) : "—"}
          </p>
          <p className="mt-1 text-xs text-app-muted">Quanto custaria quitar tudo hoje</p>
        </Card>
        <Card className="p-4">
          <p className="text-sm text-app-muted">Parcelas por mês</p>
          <p className="tnum mt-1 text-lg font-semibold text-app-text">
            {loans.data ? formatBRL(monthly) : "—"}
          </p>
        </Card>
        <Card className="p-4">
          <p className="text-sm text-app-muted">Juros ainda por pagar</p>
          <p className="tnum mt-1 text-lg font-semibold text-expense">
            {loans.data ? formatBRL(interest) : "—"}
          </p>
          <p className="mt-1 text-xs text-app-muted">Parcelas restantes − saldo devedor</p>
        </Card>
      </div>

      <Card>
        <CardHeader title="Suas dívidas" />
        {loans.isLoading ? (
          <LoadingRows rows={3} />
        ) : loans.isError ? (
          <Empty
            icon={Landmark}
            title="Não foi possível carregar as dívidas"
            hint={loans.error.message}
          />
        ) : all.length === 0 ? (
          <Empty
            icon={Landmark}
            title="Nenhuma dívida cadastrada"
            hint="Cadastre empréstimos e financiamentos para ver o saldo devedor, as próximas parcelas e a ordem mais barata de quitar."
            action={<Button onClick={openCreate}>Cadastrar dívida</Button>}
          />
        ) : (
          <ul className="divide-y divide-app-border/60" data-testid="loans-list">
            {all.map((loan) => (
              <LoanRow
                key={loan.id}
                loan={loan}
                onPay={() => setPaying(loan)}
                onEdit={() => setSheet({ open: true, editing: loan })}
                onDelete={() => onDelete(loan)}
              />
            ))}
          </ul>
        )}
      </Card>

      {remove.error ? (
        <p className="text-sm text-expense" role="alert">
          {remove.error.message}
        </p>
      ) : null}

      {active.length > 0 ? <StrategyCard loans={all} /> : null}

      <LoanSheet
        open={sheet.open}
        editing={sheet.editing}
        accounts={accounts.data ?? []}
        onOpenChange={(open) => setSheet((s) => ({ open, editing: open ? s.editing : null }))}
      />
      <PaySheet loan={paying} accounts={accounts.data ?? []} onClose={() => setPaying(null)} />
    </div>
  );
}

// ───────────────────────── linha ─────────────────────────

function LoanRow({
  loan,
  onPay,
  onEdit,
  onDelete,
}: {
  loan: Loan;
  onPay: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const percent =
    loan.totalInstallments > 0 ? (loan.paidInstallments / loan.totalInstallments) * 100 : 0;
  const done = loan.remainingInstallments <= 0;
  return (
    <li
      className={cn("flex flex-col gap-2.5 px-4 py-3.5 md:px-5", !loan.active && "opacity-70")}
      data-testid={`loan-${loan.id}`}
    >
      <div className="flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-app-soft/80 text-app-muted">
          <Landmark size={16} aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-app-text">{loan.name}</p>
          <p className="truncate text-xs text-app-muted">
            {KIND_LABEL[loan.kind] ?? loan.kind}
            {loan.lender ? ` · ${loan.lender}` : ""} ·{" "}
            <span className="tnum">{rateFmt.format(loan.monthlyRatePct)}% a.m.</span>
          </p>
        </div>
        <div className="flex shrink-0 flex-col items-end">
          <Money cents={loan.outstandingCents} neutral className="text-sm font-semibold" />
          <p className="text-[11px] text-app-muted">saldo devedor</p>
        </div>
      </div>
      <div className="pl-12">
        <div className="flex items-baseline justify-between gap-2 text-xs text-app-muted">
          <span className="tnum">
            {loan.paidInstallments} de {loan.totalInstallments} parcelas de{" "}
            {formatBRL(loan.installmentCents)}
          </span>
          <span className="tnum">{Math.round(percent)}%</span>
        </div>
        <Progress percent={percent} tone={done ? "OK" : "NEUTRAL"} className="mt-1.5" />
        <div className="mt-2 flex flex-wrap items-center gap-2">
          {!loan.active ? (
            <Badge>Inativa</Badge>
          ) : done ? (
            <Badge tone="income">Quitada</Badge>
          ) : loan.nextDueDate ? (
            <span className="text-xs text-app-muted">
              próxima {formatDate(loan.nextDueDate)} · juros restantes{" "}
              <span className="tnum text-expense">
                {formatBRL(loan.totalInterestRemainingCents)}
              </span>
            </span>
          ) : null}
          <div className="ml-auto flex items-center gap-1">
            {loan.active && !done ? (
              <Button
                variant="outline"
                size="sm"
                onClick={onPay}
                aria-label={`Pagar parcela de ${loan.name}`}
              >
                <HandCoins size={14} aria-hidden /> Pagar parcela
              </Button>
            ) : null}
            <button
              type="button"
              className={iconBtn}
              onClick={onEdit}
              aria-label={`Editar ${loan.name}`}
            >
              <Pencil size={14} />
            </button>
            <button
              type="button"
              className={cn(iconBtn, "hover:bg-rose-500/10 hover:text-expense")}
              onClick={onDelete}
              aria-label={`Excluir ${loan.name}`}
            >
              <Trash2 size={15} />
            </button>
          </div>
        </div>
      </div>
    </li>
  );
}

// ───────────────────────── estratégia de quitação ─────────────────────────

const PLANS = [
  {
    key: "avalanche",
    title: "Avalanche: maior juro primeiro",
    hint: "O dinheiro extra vai para a dívida com a maior taxa. Matematicamente, é a que paga menos juros.",
  },
  {
    key: "snowball",
    title: "Bola de neve: menor saldo primeiro",
    hint: "Quita primeiro a menor dívida. Paga um pouco mais de juros, mas dá vitórias rápidas.",
  },
  {
    key: "minimumOnly",
    title: "Só o mínimo",
    hint: "Paga apenas as parcelas combinadas, sem extra. Serve de referência.",
  },
] as const satisfies readonly { key: keyof DebtStrategy; title: string; hint: string }[];

function StrategyCard({ loans }: { loans: Loan[] }) {
  const [extra, setExtra] = useState("");
  const [extraCents, setExtraCents] = useState(0);
  const parsed = extra.trim() === "" ? 0 : parseBRLInput(extra);

  useEffect(() => {
    if (parsed === null) return;
    const timer = setTimeout(() => setExtraCents(parsed), 400);
    return () => clearTimeout(timer);
  }, [parsed]);

  const strategy = useDebtStrategy(extraCents, loans.length > 0);
  const data = strategy.data;
  const nameOf = (id: string): string => loans.find((l) => l.id === id)?.name ?? "Dívida";
  const best = data
    ? PLANS.reduce<keyof DebtStrategy>(
        (acc, p) => (data[p.key].totalInterestCents < data[acc].totalInterestCents ? p.key : acc),
        "avalanche",
      )
    : null;

  return (
    <Card className="p-5" data-testid="debt-strategy">
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div className="min-w-0 md:max-w-xl">
          <h2 className="text-sm font-semibold text-app-text">Como quitar mais rápido</h2>
          <p className="mt-0.5 text-sm text-app-muted">
            Todas as dívidas continuam com a parcela em dia. O valor extra por mês (e as parcelas
            que sobram quando uma dívida termina) vai para a próxima da fila.
          </p>
        </div>
        <Field
          label="Extra por mês"
          error={parsed === null ? "valor inválido — use 200,00" : undefined}
          className="md:w-48"
        >
          <Input
            inputMode="decimal"
            placeholder="0,00"
            value={extra}
            onChange={(e) => setExtra(e.target.value)}
          />
        </Field>
      </div>

      {strategy.isLoading ? (
        <LoadingRows rows={3} />
      ) : strategy.isError || !data ? (
        <p className="mt-4 text-sm text-app-muted">
          {strategy.error?.message ?? "Não foi possível simular a quitação agora."}
        </p>
      ) : (
        <div className="mt-4 grid gap-3 md:grid-cols-3">
          {PLANS.map((p) => (
            <PlanCard
              key={p.key}
              id={p.key}
              title={p.title}
              hint={p.hint}
              plan={data[p.key]}
              minimum={data.minimumOnly}
              best={best === p.key}
              nameOf={nameOf}
            />
          ))}
        </div>
      )}
    </Card>
  );
}

function PlanCard({
  id,
  title,
  hint,
  plan,
  minimum,
  best,
  nameOf,
}: {
  id: keyof DebtStrategy;
  title: string;
  hint: string;
  plan: DebtPlan;
  minimum: DebtPlan;
  best: boolean;
  nameOf: (id: string) => string;
}) {
  const savings = minimum.totalInterestCents - plan.totalInterestCents;
  const sooner = minimum.months - plan.months;
  return (
    <section
      className={cn(
        "flex min-w-0 flex-col gap-3 rounded-(--radius-card) border p-4",
        best
          ? "border-emerald-500/40 bg-emerald-500/6"
          : "border-app-border/80 bg-app-background/40",
      )}
      data-testid={`plan-${id}`}
      aria-label={title}
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="text-sm font-semibold text-app-text">{title}</h3>
        {best ? (
          <Badge tone="income">
            <Trophy size={12} aria-hidden /> Menos juros
          </Badge>
        ) : null}
      </div>
      <p className="text-xs text-app-muted">{hint}</p>
      <dl className="grid grid-cols-2 gap-x-3 gap-y-2 text-sm">
        <div>
          <dt className="text-xs text-app-muted">Quitação</dt>
          <dd className="font-medium text-app-text">{monthYearOf(plan.payoffDate)}</dd>
        </div>
        <div>
          <dt className="text-xs text-app-muted">Prazo</dt>
          <dd className="tnum font-medium text-app-text">{monthsLabel(plan.months)}</dd>
        </div>
        <div>
          <dt className="text-xs text-app-muted">Juros totais</dt>
          <dd className="tnum font-medium text-expense">{formatBRL(plan.totalInterestCents)}</dd>
        </div>
        {id !== "minimumOnly" ? (
          <div>
            <dt className="text-xs text-app-muted">Economia vs. mínimo</dt>
            <dd className={cn("tnum font-medium", savings > 0 ? "text-income" : "text-app-text")}>
              {formatBRL(Math.max(0, savings))}
              {sooner > 0 ? (
                <span className="block text-xs font-normal text-app-muted">
                  {monthsLabel(sooner)} antes
                </span>
              ) : null}
            </dd>
          </div>
        ) : null}
      </dl>
      {plan.order.length > 0 ? (
        <div>
          <p className="text-xs text-app-muted">Ordem de quitação</p>
          <ol className="mt-1 space-y-1">
            {plan.order.map((loanId, i) => {
              const per = plan.perLoan.find((p) => p.loanId === loanId);
              return (
                <li key={loanId} className="flex items-baseline gap-2 text-xs">
                  <span className="tnum flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-app-soft text-[11px] font-semibold text-app-text">
                    {i + 1}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-app-text">{nameOf(loanId)}</span>
                  {per ? (
                    <span className="shrink-0 text-app-muted">{monthYearOf(per.payoffDate)}</span>
                  ) : null}
                </li>
              );
            })}
          </ol>
        </div>
      ) : null}
    </section>
  );
}

// ───────────────────────── pagar parcela ─────────────────────────

function PaySheet({
  loan,
  accounts,
  onClose,
}: {
  loan: Loan | null;
  accounts: Account[];
  onClose: () => void;
}) {
  const pay = usePayLoan();
  const [amount, setAmount] = useState("");
  const [date, setDate] = useState("");
  const [accountId, setAccountId] = useState("");

  // biome-ignore lint/correctness/useExhaustiveDependencies: reinicia só ao trocar o alvo
  useEffect(() => {
    if (!loan) return;
    pay.reset();
    setAmount(centsToInput(loan.installmentCents));
    setDate(loan.nextDueDate ?? todayISO());
    setAccountId(loan.accountId ?? accounts[0]?.id ?? "");
  }, [loan]);

  const cents = parseBRLInput(amount);
  const submit = async (): Promise<void> => {
    if (!loan || !cents) return;
    await pay.mutateAsync({
      id: loan.id,
      date: date || undefined,
      amountCents: cents,
      accountId: accountId || undefined,
    });
    onClose();
  };

  return (
    <Sheet
      open={Boolean(loan)}
      onOpenChange={(open) => !open && onClose()}
      title={loan ? `Pagar parcela — ${loan.name}` : ""}
    >
      <form
        className="flex flex-col gap-4 pb-2"
        data-testid="form-pay-loan"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-app-muted">
          {loan
            ? `Parcela ${loan.paidInstallments + 1} de ${loan.totalInstallments}. Vira uma despesa confirmada no extrato, ligada a esta dívida.`
            : ""}
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
            <option value="">Conta padrão da dívida</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </Select>
        </Field>
        {pay.error ? <p className="text-sm text-expense">{pay.error.message}</p> : null}
        <Button type="submit" disabled={pay.isPending || !cents} className="h-12">
          {pay.isPending ? "Registrando…" : "Registrar pagamento"}
        </Button>
      </form>
    </Sheet>
  );
}

// ───────────────────────── cadastro / edição ─────────────────────────

const requiredMoney = z
  .string()
  .refine((v) => parseBRLInput(v) !== null, "valor inválido — use 1.000,00");

const intBetween = (min: number, max: number) => (v: string) => {
  const n = Number(v);
  return v.trim() !== "" && Number.isInteger(n) && n >= min && n <= max;
};

const loanFormSchema = z
  .object({
    name: z.string().trim().min(1, "dê um nome à dívida").max(80),
    lender: z.string().max(80),
    kind: z.enum(LOAN_KINDS),
    principal: requiredMoney,
    rate: z.string().refine((v) => parseRate(v) !== null, "taxa inválida — use 1,99"),
    installment: requiredMoney,
    totalInstallments: z.string().refine(intBetween(1, 600), "entre 1 e 600"),
    paidInstallments: z
      .string()
      .refine((v) => v.trim() === "" || intBetween(0, 600)(v), "número inteiro"),
    firstDueDate: z.string().min(10, "informe a data"),
    accountId: z.string(),
    categoryId: z.string(),
    notes: z.string().max(500),
    active: z.boolean(),
  })
  .refine((v) => Number(v.paidInstallments || 0) <= Number(v.totalInstallments || 0), {
    message: "mais pagas do que o total",
    path: ["paidInstallments"],
  });
type LoanForm = z.infer<typeof loanFormSchema>;

const emptyLoan = (): LoanForm => ({
  name: "",
  lender: "",
  kind: "PERSONAL",
  principal: "",
  rate: "",
  installment: "",
  totalInstallments: "",
  paidInstallments: "0",
  firstDueDate: todayISO(),
  accountId: "",
  categoryId: "",
  notes: "",
  active: true,
});

function LoanSheet({
  open,
  editing,
  accounts,
  onOpenChange,
}: {
  open: boolean;
  editing: Loan | null;
  accounts: Account[];
  onOpenChange: (open: boolean) => void;
}) {
  const create = useCreateLoan();
  const update = useUpdateLoan();
  const categories = useCategories("EXPENSE");
  const [calcHint, setCalcHint] = useState<string | null>(null);
  const form = useForm<LoanForm>({
    resolver: zodResolver(loanFormSchema),
    defaultValues: emptyLoan(),
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reinicia só ao abrir/trocar o alvo
  useEffect(() => {
    if (!open) return;
    create.reset();
    update.reset();
    setCalcHint(null);
    form.reset(
      editing
        ? {
            name: editing.name,
            lender: editing.lender ?? "",
            kind: editing.kind,
            principal: centsToInput(editing.principalCents),
            rate: String(editing.monthlyRatePct).replace(".", ","),
            installment: centsToInput(editing.installmentCents),
            totalInstallments: String(editing.totalInstallments),
            paidInstallments: String(editing.paidInstallments),
            firstDueDate: editing.firstDueDate,
            accountId: editing.accountId ?? "",
            categoryId: editing.categoryId ?? "",
            notes: editing.notes ?? "",
            active: editing.active,
          }
        : emptyLoan(),
    );
  }, [open, editing]);

  const fillInstallment = (): void => {
    const [principal, rate, total] = form.getValues(["principal", "rate", "totalInstallments"]);
    const p = parseBRLInput(principal);
    const r = parseRate(rate);
    const n = Number(total);
    if (!p || r === null || !Number.isInteger(n) || n < 1) {
      setCalcHint("Preencha valor emprestado, taxa e número de parcelas para calcular.");
      return;
    }
    setCalcHint(
      "Parcela fixa pela tabela Price. Confira com o contrato: tarifas e seguros mudam o valor.",
    );
    form.setValue("installment", centsToInput(priceInstallmentCents(p, r, n)), {
      shouldValidate: true,
    });
  };

  const submit = form.handleSubmit(async (v) => {
    const payload = {
      name: v.name.trim(),
      lender: v.lender.trim() || null,
      kind: v.kind,
      principalCents: parseBRLInput(v.principal) ?? 0,
      monthlyRatePct: parseRate(v.rate) ?? 0,
      installmentCents: parseBRLInput(v.installment) ?? 0,
      totalInstallments: Number(v.totalInstallments),
      paidInstallments: Number(v.paidInstallments || 0),
      firstDueDate: v.firstDueDate,
      accountId: v.accountId || null,
      categoryId: v.categoryId || null,
      notes: v.notes.trim() || null,
    };
    if (editing) await update.mutateAsync({ id: editing.id, ...payload, active: v.active });
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
      title={editing ? `Editar — ${editing.name}` : "Nova dívida"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-loan">
        <Field label="Nome" error={errors.name?.message}>
          <Input placeholder="Financiamento do carro, empréstimo…" {...form.register("name")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Tipo">
            <Select {...form.register("kind")}>
              {LOAN_KINDS.map((k) => (
                <option key={k} value={k}>
                  {KIND_LABEL[k]}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Credor (opcional)" error={errors.lender?.message}>
            <Input placeholder="Banco, financeira…" {...form.register("lender")} />
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Valor emprestado" error={errors.principal?.message}>
            <Input inputMode="decimal" placeholder="20.000,00" {...form.register("principal")} />
          </Field>
          <Field label="Taxa de juros (% ao mês)" error={errors.rate?.message}>
            <Input inputMode="decimal" placeholder="1,99" {...form.register("rate")} />
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Nº de parcelas" error={errors.totalInstallments?.message}>
            <Input inputMode="numeric" placeholder="48" {...form.register("totalInstallments")} />
          </Field>
          <Field label="Já pagas" error={errors.paidInstallments?.message}>
            <Input inputMode="numeric" placeholder="0" {...form.register("paidInstallments")} />
          </Field>
        </div>
        <div className="flex flex-col gap-2">
          <Field label="Valor da parcela" error={errors.installment?.message}>
            <Input inputMode="decimal" placeholder="650,00" {...form.register("installment")} />
          </Field>
          <Button variant="ghost" size="sm" className="self-start" onClick={fillInstallment}>
            <Calculator size={14} aria-hidden /> Calcular parcela (Price)
          </Button>
          {calcHint ? <p className="text-xs text-app-muted">{calcHint}</p> : null}
        </div>
        <Field label="Primeira parcela vence em" error={errors.firstDueDate?.message}>
          <Input type="date" {...form.register("firstDueDate")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Conta de pagamento">
            <Select {...form.register("accountId")}>
              <option value="">Escolher ao pagar</option>
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Categoria">
            <Select {...form.register("categoryId")}>
              <option value="">Empréstimos e financiamentos</option>
              {categories.data?.map((c) => (
                <option key={c.id} value={c.id}>
                  {formatCategoryLabel(c, categories.data)}
                </option>
              ))}
            </Select>
          </Field>
        </div>
        <Field label="Observações (opcional)">
          <Input placeholder="Contrato, carência…" {...form.register("notes")} />
        </Field>
        {editing ? (
          <label className="flex min-h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text">
            <input
              type="checkbox"
              className="h-4 w-4 accent-emerald-500"
              {...form.register("active")}
            />
            Ativa (entra na projeção, nos compromissos e no patrimônio)
          </label>
        ) : null}
        <p className="text-xs text-app-muted">
          As parcelas futuras entram na projeção e nos compromissos; o saldo devedor entra no
          patrimônio como dívida.
        </p>
        {error ? <p className="text-sm text-expense">{error}</p> : null}
        <Button type="submit" disabled={pending} className="h-12">
          {pending ? "Salvando…" : editing ? "Salvar alterações" : "Cadastrar dívida"}
        </Button>
      </form>
    </Sheet>
  );
}
