import type {
  Account,
  AccountType,
  CardBrand,
  Card as CardModel,
  Invoice,
} from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import { Landmark, Plus, Wallet } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Money } from "../components/ui/money";
import { Progress } from "../components/ui/progress";
import { Sheet } from "../components/ui/sheet";
import { cn } from "../lib/cn";
import { formatBRL, formatDate, parseBRLInput } from "../lib/format";
import {
  useAccounts,
  useCards,
  useCreateAccount,
  useCreateCard,
  useInvoices,
  usePayInvoice,
} from "../lib/queries";

const ACCOUNT_TYPE_LABEL: Record<AccountType, string> = {
  CHECKING: "Conta corrente",
  SAVINGS: "Poupança",
  WALLET: "Carteira",
  SALARY: "Conta salário",
  INVESTMENT: "Investimentos",
};

const BRAND_LABEL: Record<CardBrand, string> = {
  VISA: "Visa",
  MASTERCARD: "Mastercard",
  ELO: "Elo",
  AMEX: "Amex",
  HIPERCARD: "Hipercard",
  OTHER: "Outro",
};

const INVOICE_STATUS = {
  OPEN: { label: "Aberta", cls: "text-income border-emerald-400/40" },
  CLOSED: { label: "Fechada", cls: "text-warn border-amber-400/40" },
  PAID: { label: "Paga", cls: "text-app-muted border-app-border" },
} as const;

export function AccountsPage() {
  const accounts = useAccounts();
  const cards = useCards();
  const [accountSheet, setAccountSheet] = useState(false);
  const [cardSheet, setCardSheet] = useState(false);
  const [openCard, setOpenCard] = useState<CardModel | null>(null);

  const totalCents = accounts.data?.reduce((acc, a) => acc + a.balanceCents, 0) ?? 0;

  return (
    <div className="space-y-6">
      <section className="space-y-3">
        <header className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold tracking-tight">Contas</h1>
            <p className="text-sm text-app-muted">
              Total: <span className="tnum font-medium text-app-text">{formatBRL(totalCents)}</span>
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={() => setAccountSheet(true)}>
            <Plus size={15} /> Nova conta
          </Button>
        </header>

        {accounts.isLoading ? (
          <Card>
            <LoadingRows rows={3} />
          </Card>
        ) : (accounts.data?.length ?? 0) === 0 ? (
          <Card>
            <Empty
              icon={Landmark}
              title="Nenhuma conta ainda"
              hint="Crie sua primeira conta para começar a lançar."
              action={<Button onClick={() => setAccountSheet(true)}>Criar conta</Button>}
            />
          </Card>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" data-testid="accounts-grid">
            {accounts.data?.map((account) => (
              <AccountTile key={account.id} account={account} />
            ))}
          </div>
        )}
      </section>

      <section className="space-y-3">
        <header className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-semibold tracking-tight">Cartões de crédito</h2>
            <p className="text-sm text-app-muted">Limite, fatura atual e parcelas futuras</p>
          </div>
          <Button variant="outline" size="sm" onClick={() => setCardSheet(true)}>
            <Plus size={15} /> Novo cartão
          </Button>
        </header>

        {cards.isLoading ? (
          <Card>
            <LoadingRows rows={2} />
          </Card>
        ) : (cards.data?.length ?? 0) === 0 ? (
          <Card>
            <Empty
              icon={Wallet}
              title="Nenhum cartão cadastrado"
              hint="Cadastre um cartão para controlar faturas e compras parceladas."
              action={<Button onClick={() => setCardSheet(true)}>Cadastrar cartão</Button>}
            />
          </Card>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {cards.data?.map((card) => (
              <CreditCardTile key={card.id} card={card} onOpen={() => setOpenCard(card)} />
            ))}
          </div>
        )}
      </section>

      <NewAccountSheet open={accountSheet} onOpenChange={setAccountSheet} />
      <NewCardSheet open={cardSheet} onOpenChange={setCardSheet} />
      <InvoicesSheet
        card={openCard}
        onClose={() => setOpenCard(null)}
        accounts={accounts.data ?? []}
      />
    </div>
  );
}

function AccountTile({ account }: { account: Account }) {
  return (
    <Card className="p-4">
      <div className="flex items-center gap-2.5">
        <span
          aria-hidden
          className="h-2.5 w-2.5 rounded-full"
          style={{ backgroundColor: account.color ?? "#71717a" }}
        />
        <p className="min-w-0 flex-1 truncate text-sm font-medium text-app-text">{account.name}</p>
      </div>
      <p className="mt-0.5 text-xs text-app-muted">{ACCOUNT_TYPE_LABEL[account.type]}</p>
      <Money
        cents={account.balanceCents}
        neutral={account.balanceCents >= 0}
        className="mt-3 block text-xl font-semibold"
      />
    </Card>
  );
}

function CreditCardTile({ card, onOpen }: { card: CardModel; onOpen: () => void }) {
  const usedPercent = card.limitCents > 0 ? (card.usedLimitCents / card.limitCents) * 100 : 0;
  const invoice = card.currentInvoice;
  const tint = card.color ?? "#52525b";
  return (
    <button
      type="button"
      onClick={onOpen}
      className="cursor-pointer rounded-(--radius-card) border border-app-border/80 bg-app-surface/60 p-5 text-left transition-colors hover:border-app-border"
      data-testid={`card-${card.name}`}
    >
      <div
        className="relative overflow-hidden rounded-xl border border-white/5 p-4"
        style={{ background: `linear-gradient(135deg, ${tint}33, ${tint}0d 70%)` }}
      >
        <div className="flex items-start justify-between">
          <p className="text-sm font-medium text-app-text">{card.name}</p>
          <span className="text-xs font-semibold tracking-wide text-app-text">
            {BRAND_LABEL[card.brand]}
          </span>
        </div>
        <p className="tnum mt-5 text-lg font-semibold text-app-text">
          {invoice ? formatBRL(invoice.totalCents) : formatBRL(0)}
        </p>
        <p className="text-xs text-app-muted">
          {invoice
            ? `fatura ${INVOICE_STATUS[invoice.status].label.toLowerCase()} — vence ${formatDate(invoice.dueDate)}`
            : "sem fatura em aberto"}
        </p>
      </div>
      <div className="mt-4 space-y-1.5">
        <div className="flex justify-between text-xs text-app-muted">
          <span>
            Limite usado:{" "}
            <span className="tnum text-app-text">{formatBRL(card.usedLimitCents)}</span>
          </span>
          <span className="tnum">disponível {formatBRL(card.availableLimitCents)}</span>
        </div>
        <Progress
          percent={usedPercent}
          tone={usedPercent > 90 ? "DANGER" : usedPercent >= 70 ? "WARNING" : "NEUTRAL"}
        />
      </div>
    </button>
  );
}

function InvoicesSheet({
  card,
  onClose,
  accounts,
}: {
  card: CardModel | null;
  onClose: () => void;
  accounts: Account[];
}) {
  const invoices = useInvoices(card?.id ?? null);
  const pay = usePayInvoice();
  const [payingId, setPayingId] = useState<string | null>(null);
  const [accountId, setAccountId] = useState("");

  const onPay = async (invoice: Invoice): Promise<void> => {
    if (!card || !accountId) return;
    await pay.mutateAsync({ cardId: card.id, invoiceId: invoice.id, accountId });
    setPayingId(null);
  };

  return (
    <Sheet
      open={Boolean(card)}
      onOpenChange={(open) => !open && onClose()}
      title={card ? `Faturas — ${card.name}` : "Faturas"}
    >
      {invoices.isLoading ? (
        <LoadingRows rows={3} />
      ) : (
        <ul className="space-y-2 pb-3">
          {invoices.data?.map((invoice) => {
            const status = INVOICE_STATUS[invoice.status];
            const payable = invoice.status !== "PAID" && invoice.totalCents > 0;
            return (
              <li key={invoice.id} className="rounded-(--radius-card) border border-app-border p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="tnum text-sm font-medium text-app-text">
                      {String(invoice.month).padStart(2, "0")}/{invoice.year}
                    </p>
                    <p className="text-xs text-app-muted">vence {formatDate(invoice.dueDate)}</p>
                  </div>
                  <span className={cn("rounded-full border px-2.5 py-0.5 text-xs", status.cls)}>
                    {status.label}
                  </span>
                  <Money
                    cents={-invoice.totalCents}
                    neutral={invoice.status === "PAID"}
                    className="text-sm font-semibold"
                  />
                </div>
                {payable ? (
                  payingId === invoice.id ? (
                    <div className="mt-3 flex items-end gap-2">
                      <Field label="Debitar de" className="flex-1">
                        <Select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
                          <option value="" disabled>
                            Escolher conta…
                          </option>
                          {accounts.map((a) => (
                            <option key={a.id} value={a.id}>
                              {a.name} ({formatBRL(a.balanceCents)})
                            </option>
                          ))}
                        </Select>
                      </Field>
                      <Button
                        size="md"
                        disabled={!accountId || pay.isPending}
                        onClick={() => void onPay(invoice)}
                      >
                        {pay.isPending ? "Pagando…" : "Confirmar"}
                      </Button>
                    </div>
                  ) : (
                    <Button
                      variant="outline"
                      size="sm"
                      className="mt-3"
                      onClick={() => setPayingId(invoice.id)}
                    >
                      Pagar fatura
                    </Button>
                  )
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </Sheet>
  );
}

const newAccountSchema = z.object({
  name: z.string().min(1, "dê um nome à conta"),
  type: z.enum(["CHECKING", "SAVINGS", "WALLET", "SALARY", "INVESTMENT"]),
  initialBalance: z.string(),
  color: z.string(),
});

function NewAccountSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const create = useCreateAccount();
  const form = useForm<z.infer<typeof newAccountSchema>>({
    resolver: zodResolver(newAccountSchema),
    defaultValues: { name: "", type: "CHECKING", initialBalance: "", color: "#10b981" },
  });

  const submit = form.handleSubmit(async (values) => {
    await create.mutateAsync({
      name: values.name,
      type: values.type,
      initialBalanceCents: values.initialBalance ? (parseBRLInput(values.initialBalance) ?? 0) : 0,
      color: values.color,
    });
    form.reset();
    onOpenChange(false);
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Nova conta">
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-account">
        <Field label="Nome" error={form.formState.errors.name?.message}>
          <Input placeholder="Nubank, Carteira, Poupança…" {...form.register("name")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Tipo">
            <Select {...form.register("type")}>
              {Object.entries(ACCOUNT_TYPE_LABEL).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Saldo inicial">
            <Input inputMode="decimal" placeholder="0,00" {...form.register("initialBalance")} />
          </Field>
        </div>
        <Field label="Cor">
          <Input type="color" className="h-11 p-1" {...form.register("color")} />
        </Field>
        {create.error ? <p className="text-sm text-expense">{create.error.message}</p> : null}
        <Button type="submit" disabled={create.isPending} className="h-12">
          {create.isPending ? "Criando…" : "Criar conta"}
        </Button>
      </form>
    </Sheet>
  );
}

const newCardSchema = z.object({
  name: z.string().min(1, "dê um nome ao cartão"),
  brand: z.enum(["VISA", "MASTERCARD", "ELO", "AMEX", "HIPERCARD", "OTHER"]),
  limit: z.string().refine((v) => parseBRLInput(v) !== null, "limite inválido"),
  closingDay: z.string().refine(isDayOfMonth, "dia entre 1 e 28"),
  dueDay: z.string().refine(isDayOfMonth, "dia entre 1 e 28"),
  color: z.string(),
});

function isDayOfMonth(value: string): boolean {
  const n = Number(value);
  return Number.isInteger(n) && n >= 1 && n <= 28;
}

function NewCardSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const create = useCreateCard();
  const form = useForm<z.infer<typeof newCardSchema>>({
    resolver: zodResolver(newCardSchema),
    defaultValues: {
      name: "",
      brand: "VISA",
      limit: "",
      closingDay: "3",
      dueDay: "10",
      color: "#7c3aed",
    },
  });

  const submit = form.handleSubmit(async (values) => {
    await create.mutateAsync({
      name: values.name,
      brand: values.brand,
      limitCents: parseBRLInput(values.limit) ?? 0,
      closingDay: Number(values.closingDay),
      dueDay: Number(values.dueDay),
      color: values.color,
    });
    form.reset();
    onOpenChange(false);
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Novo cartão de crédito">
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-new-card">
        <Field label="Nome" error={form.formState.errors.name?.message}>
          <Input placeholder="Nubank Ultravioleta, Inter Gold…" {...form.register("name")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Bandeira">
            <Select {...form.register("brand")}>
              {Object.entries(BRAND_LABEL).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Limite" error={form.formState.errors.limit?.message}>
            <Input inputMode="decimal" placeholder="5.000,00" {...form.register("limit")} />
          </Field>
        </div>
        <div className="grid grid-cols-3 gap-3">
          <Field label="Fecha dia">
            <Input type="number" min={1} max={28} {...form.register("closingDay")} />
          </Field>
          <Field label="Vence dia">
            <Input type="number" min={1} max={28} {...form.register("dueDay")} />
          </Field>
          <Field label="Cor">
            <Input type="color" className="h-11 p-1" {...form.register("color")} />
          </Field>
        </div>
        {create.error ? <p className="text-sm text-expense">{create.error.message}</p> : null}
        <Button type="submit" disabled={create.isPending} className="h-12">
          {create.isPending ? "Salvando…" : "Cadastrar cartão"}
        </Button>
      </form>
    </Sheet>
  );
}
