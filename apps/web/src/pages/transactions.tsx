import type { Transaction } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  ArrowLeftRight,
  Check,
  ChevronLeft,
  ChevronRight,
  CreditCard,
  Download,
  Pencil,
  ReceiptText,
  Search,
  Trash2,
} from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { CategoryIcon, formatCategoryLabel } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Money } from "../components/ui/money";
import { Sheet } from "../components/ui/sheet";
import { exportUrl } from "../lib/api";
import { formatBRL, formatDate, formatMonthYear, parseBRLInput } from "../lib/format";
import {
  filtersToParams,
  type TransactionFilters,
  useAccounts,
  useCategories,
  useConfirmTransaction,
  useDeleteTransaction,
  useTransactions,
  useUpdateTransaction,
} from "../lib/queries";

const TYPE_OPTIONS = [
  ["", "Todos os tipos"],
  ["INCOME", "Receitas"],
  ["EXPENSE", "Despesas"],
  ["CARD_EXPENSE", "Cartão"],
  ["TRANSFER", "Transferências"],
  ["INVOICE_PAYMENT", "Pagamentos de fatura"],
] as const;

export function TransactionsPage() {
  const now = new Date();
  const [filters, setFilters] = useState<TransactionFilters>({
    month: now.getMonth() + 1,
    year: now.getFullYear(),
    page: 1,
  });
  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<Transaction | null>(null);

  const accounts = useAccounts();
  const categories = useCategories();
  const query = useTransactions({ ...filters, search: search || undefined });
  const confirm = useConfirmTransaction();
  const remove = useDeleteTransaction();

  const set = (patch: Partial<TransactionFilters>): void =>
    setFilters((f) => ({ ...f, ...patch, page: patch.page ?? 1 }));

  const shiftMonth = (delta: number): void => {
    const zero = (filters.year ?? now.getFullYear()) * 12 + ((filters.month ?? 1) - 1) + delta;
    set({ year: Math.floor(zero / 12), month: (zero % 12) + 1 });
  };

  const totalPages = query.data
    ? Math.max(1, Math.ceil(query.data.total / query.data.pageSize))
    : 1;
  const exportParams = filtersToParams({
    ...filters,
    search: search || undefined,
    page: undefined,
  });

  const onDelete = (t: Transaction): void => {
    const isInstallment = Boolean(t.installmentTotal);
    const message = t.transferGroupId
      ? "Excluir esta transferência? As duas pernas (débito e crédito) serão removidas."
      : isInstallment
        ? "Excluir todas as parcelas desta compra? (OK = todas, Cancelar = nada)"
        : `Excluir "${t.description}"?`;
    if (window.confirm(message)) {
      remove.mutate({ id: t.id, scope: isInstallment ? "group" : "single" });
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Extrato</h1>
          <p className="text-sm text-app-muted">
            {query.data ? `${query.data.total} lançamentos no período` : "Carregando…"}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <a
            href={exportUrl("csv", exportParams)}
            download
            className="flex h-10 items-center gap-2 rounded-(--radius-control) border border-app-border px-3 text-sm text-app-text hover:border-app-muted"
          >
            <Download size={15} /> CSV
          </a>
          <a
            href={exportUrl("ofx", exportParams)}
            download
            className="flex h-10 items-center gap-2 rounded-(--radius-control) border border-app-border px-3 text-sm text-app-text hover:border-app-muted"
          >
            <Download size={15} /> OFX
          </a>
          <a
            href={exportUrl("xlsx", exportParams)}
            download
            className="flex h-10 items-center gap-2 rounded-(--radius-control) border border-app-border px-3 text-sm text-app-text hover:border-app-muted"
          >
            <Download size={15} /> <span className="hidden sm:inline">Exportar </span>Excel (.xlsx)
          </a>
        </div>
      </header>

      {/* filtros */}
      <Card className="flex flex-wrap items-center gap-2 p-3">
        <div className="flex items-center">
          <button
            type="button"
            onClick={() => shiftMonth(-1)}
            aria-label="Mês anterior"
            className="flex h-10 w-9 cursor-pointer items-center justify-center rounded-(--radius-control) text-app-muted hover:bg-app-soft"
          >
            <ChevronLeft size={16} />
          </button>
          <span className="tnum min-w-30 text-center text-sm text-app-text">
            {formatMonthYear(filters.month ?? 1, filters.year ?? now.getFullYear())}
          </span>
          <button
            type="button"
            onClick={() => shiftMonth(1)}
            aria-label="Próximo mês"
            className="flex h-10 w-9 cursor-pointer items-center justify-center rounded-(--radius-control) text-app-muted hover:bg-app-soft"
          >
            <ChevronRight size={16} />
          </button>
        </div>
        <div className="relative min-w-40 flex-1">
          <Search size={15} className="absolute top-1/2 left-3 -translate-y-1/2 text-app-muted" />
          <Input
            placeholder="Buscar descrição…"
            className="h-10 pl-9"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <Select
          className="h-10 w-auto"
          value={filters.type ?? ""}
          onChange={(e) =>
            set({ type: (e.target.value || undefined) as TransactionFilters["type"] })
          }
        >
          {TYPE_OPTIONS.map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </Select>
        <Select
          className="h-10 w-auto"
          value={filters.status ?? ""}
          onChange={(e) =>
            set({ status: (e.target.value || undefined) as TransactionFilters["status"] })
          }
        >
          <option value="">Todos os status</option>
          <option value="CONFIRMED">Confirmadas</option>
          <option value="PENDING">Pendentes</option>
        </Select>
        <Select
          className="h-10 w-auto max-w-44"
          value={filters.accountId ?? ""}
          onChange={(e) => set({ accountId: e.target.value || undefined })}
        >
          <option value="">Todas as contas</option>
          {accounts.data?.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
            </option>
          ))}
        </Select>
        <Select
          className="h-10 w-auto max-w-44"
          value={filters.categoryId ?? ""}
          onChange={(e) => set({ categoryId: e.target.value || undefined })}
        >
          <option value="">Todas as categorias</option>
          {categories.data?.map((c) => (
            <option key={c.id} value={c.id}>
              {formatCategoryLabel(c, categories.data)}
            </option>
          ))}
        </Select>
      </Card>

      {/* lista */}
      <Card>
        {query.isLoading ? (
          <LoadingRows rows={8} />
        ) : (query.data?.items.length ?? 0) === 0 ? (
          <Empty
            icon={ReceiptText}
            title="Nenhum lançamento encontrado"
            hint="Ajuste os filtros ou lance sua primeira transação pelo botão +."
          />
        ) : (
          <ul className="divide-y divide-app-border/60" data-testid="transactions-list">
            {query.data?.items.map((t) => (
              <li key={t.id} className="group flex items-center gap-3 px-4 py-3 md:px-5">
                {t.type === "TRANSFER" ? (
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-app-soft/80 text-app-muted">
                    <ArrowLeftRight size={16} />
                  </span>
                ) : t.type === "INVOICE_PAYMENT" ? (
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-app-soft/80 text-app-muted">
                    <CreditCard size={16} />
                  </span>
                ) : (
                  <CategoryIcon icon={t.categoryIcon} color={t.categoryColor} />
                )}
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm text-app-text">{t.description}</p>
                  <p className="truncate text-xs text-app-muted">
                    {formatDate(t.date)}
                    {t.accountName ? ` — ${t.accountName}` : t.cardName ? ` — ${t.cardName}` : ""}
                    {t.categoryName ? ` — ${t.categoryName}` : ""}
                  </p>
                </div>
                {t.status === "PENDING" ? (
                  <button
                    type="button"
                    onClick={() => confirm.mutate(t.id)}
                    className="flex h-8 cursor-pointer items-center gap-1.5 rounded-full border border-amber-400/40 px-2.5 text-xs text-warn hover:bg-amber-400/10"
                    title="Marcar como paga/recebida"
                  >
                    <Check size={13} /> <span className="max-sm:sr-only">confirmar</span>
                  </button>
                ) : null}
                <Money cents={t.amountCents} signed className="text-sm font-medium" />
                <button
                  type="button"
                  onClick={() => setEditing(t)}
                  aria-label={`Editar ${t.description}`}
                  className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full text-app-muted opacity-0 transition-opacity group-hover:opacity-100 hover:bg-app-soft hover:text-app-text focus-visible:opacity-100 max-md:opacity-100"
                >
                  <Pencil size={14} />
                </button>
                <button
                  type="button"
                  onClick={() => onDelete(t)}
                  aria-label={`Excluir ${t.description}`}
                  className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full text-app-subtle opacity-0 transition-opacity group-hover:opacity-100 hover:bg-rose-500/10 hover:text-expense focus-visible:opacity-100 max-md:opacity-100"
                >
                  <Trash2 size={15} />
                </button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      {/* paginação */}
      {totalPages > 1 ? (
        <div className="flex items-center justify-center gap-3 text-sm text-app-muted">
          <button
            type="button"
            disabled={(filters.page ?? 1) <= 1}
            onClick={() => set({ page: (filters.page ?? 1) - 1 })}
            className="h-10 cursor-pointer rounded-(--radius-control) px-3 hover:bg-app-surface disabled:cursor-default disabled:opacity-40"
          >
            Anterior
          </button>
          <span className="tnum">
            {filters.page ?? 1} / {totalPages}
          </span>
          <button
            type="button"
            disabled={(filters.page ?? 1) >= totalPages}
            onClick={() => set({ page: (filters.page ?? 1) + 1 })}
            className="h-10 cursor-pointer rounded-(--radius-control) px-3 hover:bg-app-surface disabled:cursor-default disabled:opacity-40"
          >
            Próxima
          </button>
        </div>
      ) : null}

      <EditTransactionSheet transaction={editing} onClose={() => setEditing(null)} />
    </div>
  );
}

const editTransactionSchema = z.object({
  description: z.string().min(1, "descreva o lançamento").max(120),
  amount: z.string().refine((v) => parseBRLInput(v) !== null, "valor inválido — use 45,90"),
  date: z.string().min(10, "informe a data"),
  accountId: z.string(),
  categoryId: z.string(),
  pending: z.boolean(),
  notes: z.string().max(500),
});
type EditTransactionForm = z.infer<typeof editTransactionSchema>;

function centsToInput(cents: number): string {
  return (Math.abs(cents) / 100).toFixed(2).replace(".", ",");
}

function EditTransactionSheet({
  transaction,
  onClose,
}: {
  transaction: Transaction | null;
  onClose: () => void;
}) {
  const isSimple = transaction?.type === "INCOME" || transaction?.type === "EXPENSE";
  const categoryKind = transaction?.type === "INCOME" ? "INCOME" : "EXPENSE";

  const accounts = useAccounts();
  const categories = useCategories(categoryKind);
  const update = useUpdateTransaction();

  const form = useForm<EditTransactionForm>({
    resolver: zodResolver(editTransactionSchema),
    defaultValues: {
      description: "",
      amount: "0,00",
      date: "",
      accountId: "",
      categoryId: "",
      pending: false,
      notes: "",
    },
  });

  useEffect(() => {
    if (transaction) {
      form.reset({
        description: transaction.description,
        amount: centsToInput(transaction.amountCents),
        date: transaction.date,
        accountId: transaction.accountId ?? "",
        categoryId: transaction.categoryId ?? "",
        pending: transaction.status === "PENDING",
        notes: transaction.notes ?? "",
      });
    }
  }, [transaction, form]);

  const submit = form.handleSubmit(async (values) => {
    if (!transaction) return;
    await update.mutateAsync({
      id: transaction.id,
      description: values.description.trim(),
      amountCents: isSimple ? (parseBRLInput(values.amount) ?? undefined) : undefined,
      accountId: isSimple && values.accountId ? values.accountId : undefined,
      date: values.date,
      status: values.pending ? "PENDING" : "CONFIRMED",
      categoryId: values.categoryId || null,
      notes: values.notes.trim() || null,
    });
    onClose();
  });

  return (
    <Sheet
      open={Boolean(transaction)}
      onOpenChange={(open) => !open && onClose()}
      title="Editar lançamento"
    >
      {transaction ? (
        <form
          onSubmit={submit}
          className="flex flex-col gap-4 pb-2"
          data-testid="form-edit-transaction"
        >
          {isSimple ? (
            <Field label="Valor" error={form.formState.errors.amount?.message}>
              <Input inputMode="decimal" placeholder="0,00" {...form.register("amount")} />
            </Field>
          ) : (
            <div className="rounded-(--radius-control) border border-app-border bg-app-background/60 px-3.5 py-2.5 text-xs text-app-muted">
              Valor (
              <span className="tnum font-medium text-app-text">
                {formatBRL(Math.abs(transaction.amountCents))}
              </span>
              ) e vínculo de cartão/transferência são fixos; para alterar o valor, exclua e lance
              novamente.
            </div>
          )}

          <Field label="Descrição" error={form.formState.errors.description?.message}>
            <Input placeholder="Descrição do lançamento…" {...form.register("description")} />
          </Field>

          <div className="grid grid-cols-2 gap-3">
            {isSimple ? (
              <Field label="Conta" error={form.formState.errors.accountId?.message}>
                <Select {...form.register("accountId")}>
                  {accounts.data?.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </Select>
              </Field>
            ) : (
              <Field label="Origem">
                <Input
                  value={transaction.cardName ?? transaction.accountName ?? "—"}
                  disabled
                  className="opacity-60"
                />
              </Field>
            )}
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

          <div className="grid grid-cols-2 items-end gap-3">
            <Field label="Data" error={form.formState.errors.date?.message}>
              <Input type="date" {...form.register("date")} />
            </Field>
            <label className="flex h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text">
              <input
                type="checkbox"
                className="h-4 w-4 accent-amber-400"
                {...form.register("pending")}
              />
              {transaction.type === "INCOME" ? "A receber" : "A pagar"}
            </label>
          </div>

          <Field label="Observações (opcional)">
            <Input placeholder="Notas adicionais…" {...form.register("notes")} />
          </Field>

          {update.error ? <p className="text-sm text-expense">{update.error.message}</p> : null}
          <Button type="submit" disabled={update.isPending} className="h-12">
            {update.isPending ? "Salvando…" : "Salvar alterações"}
          </Button>
        </form>
      ) : null}
    </Sheet>
  );
}
