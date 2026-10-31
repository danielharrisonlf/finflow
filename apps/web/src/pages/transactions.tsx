import type { Transaction } from "@finflow/contracts";
import {
  ArrowLeftRight,
  Check,
  ChevronLeft,
  ChevronRight,
  CreditCard,
  Download,
  ReceiptText,
  Search,
  Trash2,
} from "lucide-react";
import { useState } from "react";
import { Card } from "../components/ui/card";
import { CategoryIcon } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Input, Select } from "../components/ui/field";
import { Money } from "../components/ui/money";
import { exportUrl } from "../lib/api";
import { formatDate, formatMonthYear } from "../lib/format";
import {
  filtersToParams,
  type TransactionFilters,
  useAccounts,
  useCategories,
  useConfirmTransaction,
  useDeleteTransaction,
  useTransactions,
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
          <p className="text-sm text-zinc-500">
            {query.data ? `${query.data.total} lançamentos no período` : "Carregando…"}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <a
            href={exportUrl("csv", exportParams)}
            download
            className="flex h-10 items-center gap-2 rounded-(--radius-control) border border-zinc-700 px-3 text-sm text-zinc-300 hover:border-zinc-500"
          >
            <Download size={15} /> CSV
          </a>
          <a
            href={exportUrl("ofx", exportParams)}
            download
            className="flex h-10 items-center gap-2 rounded-(--radius-control) border border-zinc-700 px-3 text-sm text-zinc-300 hover:border-zinc-500"
          >
            <Download size={15} /> OFX
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
            className="flex h-10 w-9 cursor-pointer items-center justify-center rounded-(--radius-control) text-zinc-400 hover:bg-zinc-800"
          >
            <ChevronLeft size={16} />
          </button>
          <span className="tnum min-w-30 text-center text-sm text-zinc-200">
            {formatMonthYear(filters.month ?? 1, filters.year ?? now.getFullYear())}
          </span>
          <button
            type="button"
            onClick={() => shiftMonth(1)}
            aria-label="Próximo mês"
            className="flex h-10 w-9 cursor-pointer items-center justify-center rounded-(--radius-control) text-zinc-400 hover:bg-zinc-800"
          >
            <ChevronRight size={16} />
          </button>
        </div>
        <div className="relative min-w-40 flex-1">
          <Search size={15} className="absolute top-1/2 left-3 -translate-y-1/2 text-zinc-500" />
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
              {c.name}
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
          <ul className="divide-y divide-zinc-800/60" data-testid="transactions-list">
            {query.data?.items.map((t) => (
              <li key={t.id} className="group flex items-center gap-3 px-4 py-3 md:px-5">
                {t.type === "TRANSFER" ? (
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-zinc-800/80 text-zinc-400">
                    <ArrowLeftRight size={16} />
                  </span>
                ) : t.type === "INVOICE_PAYMENT" ? (
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-zinc-800/80 text-zinc-400">
                    <CreditCard size={16} />
                  </span>
                ) : (
                  <CategoryIcon icon={t.categoryIcon} color={t.categoryColor} />
                )}
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm text-zinc-100">{t.description}</p>
                  <p className="truncate text-xs text-zinc-500">
                    {formatDate(t.date)}
                    {t.accountName ? ` — ${t.accountName}` : t.cardName ? ` — ${t.cardName}` : ""}
                    {t.categoryName ? ` — ${t.categoryName}` : ""}
                  </p>
                </div>
                {t.status === "PENDING" ? (
                  <button
                    type="button"
                    onClick={() => confirm.mutate(t.id)}
                    className="flex h-8 cursor-pointer items-center gap-1.5 rounded-full border border-amber-400/40 px-2.5 text-xs text-amber-300 hover:bg-amber-400/10"
                    title="Marcar como paga/recebida"
                  >
                    <Check size={13} /> confirmar
                  </button>
                ) : null}
                <Money cents={t.amountCents} signed className="text-sm font-medium" />
                <button
                  type="button"
                  onClick={() => onDelete(t)}
                  aria-label={`Excluir ${t.description}`}
                  className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full text-zinc-600 opacity-0 transition-opacity group-hover:opacity-100 hover:bg-rose-500/10 hover:text-rose-400 focus-visible:opacity-100 max-md:opacity-100"
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
        <div className="flex items-center justify-center gap-3 text-sm text-zinc-400">
          <button
            type="button"
            disabled={(filters.page ?? 1) <= 1}
            onClick={() => set({ page: (filters.page ?? 1) - 1 })}
            className="h-10 cursor-pointer rounded-(--radius-control) px-3 hover:bg-zinc-900 disabled:cursor-default disabled:opacity-40"
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
            className="h-10 cursor-pointer rounded-(--radius-control) px-3 hover:bg-zinc-900 disabled:cursor-default disabled:opacity-40"
          >
            Próxima
          </button>
        </div>
      ) : null}
    </div>
  );
}
