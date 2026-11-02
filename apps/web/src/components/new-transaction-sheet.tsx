import { zodResolver } from "@hookform/resolvers/zod";
import * as Tabs from "@radix-ui/react-tabs";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { cn } from "../lib/cn";
import { parseBRLInput, todayISO } from "../lib/format";
import {
  useAccounts,
  useCards,
  useCategories,
  useCreateCardExpense,
  useCreateTransaction,
  useTransfer,
} from "../lib/queries";
import { Button } from "./ui/button";
import { formatCategoryLabel } from "./ui/category-icon";
import { Field, Input, Select } from "./ui/field";
import { Sheet } from "./ui/sheet";

const amountField = z
  .string()
  .refine((v) => parseBRLInput(v) !== null, "valor inválido — use 45,90");

const simpleSchema = z.object({
  amount: amountField,
  description: z.string().min(1, "descreva o lançamento"),
  accountId: z.string().min(1, "escolha a conta"),
  categoryId: z.string(),
  date: z.string().min(10),
  pending: z.boolean(),
});
type SimpleForm = z.infer<typeof simpleSchema>;

const cardSchema = z.object({
  amount: amountField,
  description: z.string().min(1, "descreva a compra"),
  cardId: z.string().min(1, "escolha o cartão"),
  categoryId: z.string(),
  installments: z.string(),
  date: z.string().min(10),
});
type CardForm = z.infer<typeof cardSchema>;

const transferSchema = z
  .object({
    amount: amountField,
    fromAccountId: z.string().min(1, "escolha a origem"),
    toAccountId: z.string().min(1, "escolha o destino"),
    date: z.string().min(10),
    description: z.string(),
  })
  .refine((v) => v.fromAccountId !== v.toAccountId, {
    message: "origem e destino devem ser diferentes",
    path: ["toAccountId"],
  });
type TransferForm = z.infer<typeof transferSchema>;

const tabCls =
  "h-10 flex-1 rounded-(--radius-control) text-sm text-app-muted transition-colors data-[state=active]:bg-app-soft data-[state=active]:font-medium data-[state=active]:text-app-text";

export function NewTransactionSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Novo lançamento">
      <Tabs.Root defaultValue="expense">
        <Tabs.List
          className="mb-4 flex gap-1 rounded-(--radius-control) bg-app-background/70 p-1"
          aria-label="Tipo de lançamento"
        >
          <Tabs.Trigger value="expense" className={tabCls}>
            Despesa
          </Tabs.Trigger>
          <Tabs.Trigger value="income" className={tabCls}>
            Receita
          </Tabs.Trigger>
          <Tabs.Trigger value="card" className={tabCls}>
            Cartão
          </Tabs.Trigger>
          <Tabs.Trigger value="transfer" className={tabCls}>
            Transferir
          </Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="expense">
          <SimpleTransactionForm type="EXPENSE" onDone={() => onOpenChange(false)} />
        </Tabs.Content>
        <Tabs.Content value="income">
          <SimpleTransactionForm type="INCOME" onDone={() => onOpenChange(false)} />
        </Tabs.Content>
        <Tabs.Content value="card">
          <CardExpenseForm onDone={() => onOpenChange(false)} />
        </Tabs.Content>
        <Tabs.Content value="transfer">
          <TransferFormView onDone={() => onOpenChange(false)} />
        </Tabs.Content>
      </Tabs.Root>
    </Sheet>
  );
}

function AmountInput({
  error,
  tone,
  ...register
}: Record<string, unknown> & { error?: string; tone: "income" | "expense" | "neutral" }) {
  return (
    <Field label="Valor" error={error}>
      <div className="relative">
        <span className="absolute top-1/2 left-4 -translate-y-1/2 text-lg text-app-muted">R$</span>
        <input
          inputMode="decimal"
          placeholder="0,00"
          autoComplete="off"
          className={cn(
            "tnum h-14 w-full rounded-(--radius-control) border border-app-border/80 bg-app-background pr-4 pl-12 text-2xl font-semibold placeholder:text-app-subtle focus:border-emerald-500 focus:outline-none",
            tone === "income" && "text-(--color-income)",
            tone === "expense" && "text-(--color-expense)",
            tone === "neutral" && "text-app-text",
          )}
          {...register}
        />
      </div>
    </Field>
  );
}

function SimpleTransactionForm({
  type,
  onDone,
}: {
  type: "INCOME" | "EXPENSE";
  onDone: () => void;
}) {
  const accounts = useAccounts();
  const categories = useCategories(type);
  const mutation = useCreateTransaction();

  const form = useForm<SimpleForm>({
    resolver: zodResolver(simpleSchema),
    defaultValues: {
      amount: "",
      description: "",
      accountId: "",
      categoryId: "",
      date: todayISO(),
      pending: false,
    },
  });

  const firstAccount = accounts.data?.[0]?.id ?? "";

  const submit = form.handleSubmit(async (values) => {
    await mutation.mutateAsync({
      type,
      description: values.description,
      amountCents: parseBRLInput(values.amount) ?? 0,
      date: values.date,
      status: values.pending ? "PENDING" : "CONFIRMED",
      accountId: values.accountId || firstAccount,
      categoryId: values.categoryId || null,
      notes: null,
    });
    form.reset();
    onDone();
  });

  return (
    <form
      onSubmit={submit}
      className="flex flex-col gap-4 pb-2"
      data-testid={`form-${type.toLowerCase()}`}
    >
      <AmountInput
        tone={type === "INCOME" ? "income" : "expense"}
        error={form.formState.errors.amount?.message}
        {...form.register("amount")}
      />
      <Field label="Descrição" error={form.formState.errors.description?.message}>
        <Input
          placeholder={type === "INCOME" ? "Salário, venda, reembolso…" : "Mercado, almoço, uber…"}
          {...form.register("description")}
        />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Conta" error={form.formState.errors.accountId?.message}>
          <Select {...form.register("accountId")} defaultValue="">
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
        <Field label="Categoria">
          <Select {...form.register("categoryId")} defaultValue="">
            <option value="">Sem categoria</option>
            {categories.data?.map((c) => (
              <option key={c.id} value={c.id}>
                {formatCategoryLabel(c, categories.data)}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      <div className="grid grid-cols-2 items-end gap-3">
        <Field label="Data">
          <Input type="date" {...form.register("date")} />
        </Field>
        <label className="flex h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text">
          <input
            type="checkbox"
            className="h-4 w-4 accent-amber-400"
            {...form.register("pending")}
          />
          {type === "INCOME" ? "A receber" : "A pagar"}
        </label>
      </div>
      <SubmitRow
        loading={mutation.isPending}
        error={mutation.error?.message}
        label={type === "INCOME" ? "Adicionar receita" : "Adicionar despesa"}
      />
    </form>
  );
}

function CardExpenseForm({ onDone }: { onDone: () => void }) {
  const cards = useCards();
  const categories = useCategories("EXPENSE");
  const mutation = useCreateCardExpense();

  const form = useForm<CardForm>({
    resolver: zodResolver(cardSchema),
    defaultValues: {
      amount: "",
      description: "",
      cardId: "",
      categoryId: "",
      installments: "1",
      date: todayISO(),
    },
  });

  const submit = form.handleSubmit(async (values) => {
    await mutation.mutateAsync({
      cardId: values.cardId,
      description: values.description,
      totalCents: parseBRLInput(values.amount) ?? 0,
      date: values.date,
      categoryId: values.categoryId || null,
      installments: Number(values.installments),
      notes: null,
    });
    form.reset();
    onDone();
  });

  return (
    <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-card">
      <AmountInput
        tone="expense"
        error={form.formState.errors.amount?.message}
        {...form.register("amount")}
      />
      <Field label="Descrição" error={form.formState.errors.description?.message}>
        <Input placeholder="Notebook, mercado, assinatura…" {...form.register("description")} />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Cartão" error={form.formState.errors.cardId?.message}>
          <Select {...form.register("cardId")} defaultValue="">
            <option value="" disabled>
              Escolher…
            </option>
            {cards.data?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Parcelas">
          <Select {...form.register("installments")}>
            {Array.from({ length: 24 }, (_, i) => i + 1).map((n) => (
              <option key={n} value={n}>
                {n}x
              </option>
            ))}
          </Select>
        </Field>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Categoria">
          <Select {...form.register("categoryId")} defaultValue="">
            <option value="">Sem categoria</option>
            {categories.data?.map((c) => (
              <option key={c.id} value={c.id}>
                {formatCategoryLabel(c, categories.data)}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Data da compra">
          <Input type="date" {...form.register("date")} />
        </Field>
      </div>
      <SubmitRow
        loading={mutation.isPending}
        error={mutation.error?.message}
        label="Lançar no cartão"
      />
    </form>
  );
}

function TransferFormView({ onDone }: { onDone: () => void }) {
  const accounts = useAccounts();
  const mutation = useTransfer();

  const form = useForm<TransferForm>({
    resolver: zodResolver(transferSchema),
    defaultValues: {
      amount: "",
      fromAccountId: "",
      toAccountId: "",
      date: todayISO(),
      description: "",
    },
  });

  const submit = form.handleSubmit(async (values) => {
    await mutation.mutateAsync({
      fromAccountId: values.fromAccountId,
      toAccountId: values.toAccountId,
      amountCents: parseBRLInput(values.amount) ?? 0,
      date: values.date,
      description: values.description || undefined,
    });
    form.reset();
    onDone();
  });

  return (
    <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-transfer">
      <AmountInput
        tone="neutral"
        error={form.formState.errors.amount?.message}
        {...form.register("amount")}
      />
      <div className="grid grid-cols-2 gap-3">
        <Field label="De" error={form.formState.errors.fromAccountId?.message}>
          <Select {...form.register("fromAccountId")} defaultValue="">
            <option value="" disabled>
              Origem…
            </option>
            {accounts.data?.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Para" error={form.formState.errors.toAccountId?.message}>
          <Select {...form.register("toAccountId")} defaultValue="">
            <option value="" disabled>
              Destino…
            </option>
            {accounts.data?.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Data">
          <Input type="date" {...form.register("date")} />
        </Field>
        <Field label="Descrição (opcional)">
          <Input placeholder="Aporte, reserva…" {...form.register("description")} />
        </Field>
      </div>
      <SubmitRow loading={mutation.isPending} error={mutation.error?.message} label="Transferir" />
    </form>
  );
}

function SubmitRow({ loading, error, label }: { loading: boolean; error?: string; label: string }) {
  return (
    <div className="flex flex-col gap-2">
      {error ? <p className="text-sm text-expense">{error}</p> : null}
      <Button type="submit" disabled={loading} className="h-12 text-[15px]">
        {loading ? "Salvando…" : label}
      </Button>
    </div>
  );
}
