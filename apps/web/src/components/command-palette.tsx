import * as Dialog from "@radix-ui/react-dialog";
import {
  FileSpreadsheet,
  type LucideIcon,
  Plus,
  ReceiptText,
  Search,
  ShoppingBag,
} from "lucide-react";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { cn } from "../lib/cn";
import { formatDate, formatSignedBRL } from "../lib/format";
import { useAiEnabled, useTransactionSearch } from "../lib/queries";
import { useActiveSpace } from "../lib/use-active-space";
import { navItemsFor } from "./layout/nav";

interface PaletteItem {
  id: string;
  group: "Ações rápidas" | "Páginas" | "Lançamentos";
  label: string;
  hint?: string;
  icon: LucideIcon;
  run: () => void;
}

/** Sem acentos e minúsculo, para casar "orcamento" com "Orçamentos". */
function normalize(text: string): string {
  return text
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase();
}

function useDebounced<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = window.setTimeout(() => setDebounced(value), ms);
    return () => window.clearTimeout(id);
  }, [value, ms]);
  return debounced;
}

/**
 * Paleta de comandos (v3 §16): ⌘K / Ctrl+K abre; navega para qualquer página, busca
 * lançamentos em todo o histórico e dispara ações rápidas.
 */
export function CommandPalette({
  open,
  onOpenChange,
  onNewTransaction,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onNewTransaction: () => void;
}) {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const listId = useId();
  const listRef = useRef<HTMLDivElement>(null);
  const term = useDebounced(query.trim(), 200);
  const search = useTransactionSearch(open ? term : "");
  const { kind } = useActiveSpace();
  const aiEnabled = useAiEnabled();

  useEffect(() => {
    if (open) {
      setQuery("");
      setActive(0);
    }
  }, [open]);

  const items = useMemo<PaletteItem[]>(() => {
    const go = (to: string) => () => {
      onOpenChange(false);
      navigate(to);
    };
    const actions: PaletteItem[] = [
      {
        id: "action-new",
        group: "Ações rápidas",
        label: "Nova transação",
        hint: "despesa, receita, cartão ou transferência",
        icon: Plus,
        run: () => {
          onOpenChange(false);
          onNewTransaction();
        },
      },
      {
        id: "action-import",
        group: "Ações rápidas",
        label: "Importar planilha",
        hint: "CSV, OFX ou Excel",
        icon: FileSpreadsheet,
        run: go("/planilhas"),
      },
      {
        id: "action-afford",
        group: "Ações rápidas",
        label: "Posso comprar?",
        hint: "simular uma compra na projeção",
        icon: ShoppingBag,
        run: go("/projecao#posso-comprar"),
      },
    ];
    const pages: PaletteItem[] = navItemsFor(kind, aiEnabled).map((item) => ({
      id: `page-${item.to}`,
      group: "Páginas",
      label: item.label,
      icon: item.icon,
      run: go(item.to),
    }));
    const needle = normalize(query.trim());
    const matches = (item: PaletteItem) =>
      !needle || normalize(`${item.label} ${item.hint ?? ""}`).includes(needle);
    const found: PaletteItem[] =
      term.length >= 2
        ? (search.data ?? []).map((t) => {
            const [year, month] = t.date.split("-");
            const params = new URLSearchParams({
              search: t.description,
              month: String(Number(month)),
              year: year ?? "",
            });
            return {
              id: `tx-${t.id}`,
              group: "Lançamentos" as const,
              label: t.description,
              hint: `${formatDate(t.date)} · ${formatSignedBRL(t.amountCents)}`,
              icon: ReceiptText,
              run: go(`/extrato?${params.toString()}`),
            };
          })
        : [];
    return [...actions.filter(matches), ...pages.filter(matches), ...found];
  }, [query, term, search.data, kind, aiEnabled, navigate, onOpenChange, onNewTransaction]);

  useEffect(() => {
    setActive((i) => Math.min(i, Math.max(0, items.length - 1)));
  }, [items.length]);

  useEffect(() => {
    listRef.current
      ?.querySelector<HTMLElement>(`[data-index="${active}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>): void => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((i) => (items.length === 0 ? 0 : (i + 1) % items.length));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => (items.length === 0 ? 0 : (i - 1 + items.length) % items.length));
    } else if (e.key === "Enter") {
      e.preventDefault();
      items[active]?.run();
    }
  };

  const groups = ["Ações rápidas", "Páginas", "Lançamentos"] as const;
  let index = -1;

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="sheet-overlay fixed inset-0 z-40 bg-black/60 backdrop-blur-[2px]" />
        <Dialog.Content
          className="fixed inset-x-3 top-[8dvh] z-50 mx-auto flex max-h-[80dvh] max-w-xl flex-col overflow-hidden rounded-(--radius-card) border border-app-border bg-app-surface shadow-2xl outline-none"
          aria-describedby={undefined}
          data-testid="command-palette"
        >
          <Dialog.Title className="sr-only">Buscar e navegar</Dialog.Title>
          <div className="flex items-center gap-2 border-b border-app-border px-4">
            <Search size={17} className="shrink-0 text-app-muted" aria-hidden />
            <input
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setActive(0);
              }}
              onKeyDown={onKeyDown}
              placeholder="Buscar páginas, lançamentos ou ações…"
              aria-label="Buscar páginas, lançamentos ou ações"
              role="combobox"
              aria-expanded
              aria-controls={listId}
              aria-activedescendant={items[active] ? `${listId}-${active}` : undefined}
              autoComplete="off"
              className="h-13 min-w-0 flex-1 bg-transparent text-[15px] text-app-text placeholder:text-app-muted focus:outline-none"
            />
            <kbd className="hidden rounded border border-app-border px-1.5 py-0.5 text-[11px] text-app-muted sm:block">
              Esc
            </kbd>
          </div>
          <div
            ref={listRef}
            id={listId}
            role="listbox"
            aria-label="Resultados"
            className="min-h-0 flex-1 overflow-y-auto p-2"
          >
            {groups.map((group) => {
              const groupItems = items.filter((i) => i.group === group);
              if (groupItems.length === 0) return null;
              return (
                <div key={group} className="pb-1">
                  <p
                    aria-hidden
                    className="px-2 pt-2 pb-1 text-[11px] font-medium tracking-wide text-app-subtle uppercase"
                  >
                    {group}
                  </p>
                  {groupItems.map((item) => {
                    index += 1;
                    const i = index;
                    const Icon = item.icon;
                    return (
                      <div
                        key={item.id}
                        id={`${listId}-${i}`}
                        role="option"
                        aria-selected={i === active}
                        data-index={i}
                        tabIndex={-1}
                        onMouseMove={() => setActive(i)}
                        onClick={item.run}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") item.run();
                        }}
                        className={cn(
                          "flex min-h-11 cursor-pointer items-center gap-3 rounded-(--radius-control) px-2.5 text-sm text-app-text",
                          i === active && "bg-app-soft",
                        )}
                      >
                        <Icon size={16} className="shrink-0 text-app-muted" aria-hidden />
                        <span className="min-w-0 flex-1 truncate">{item.label}</span>
                        {item.hint ? (
                          <span className="tnum hidden max-w-[45%] truncate text-xs text-app-muted sm:block">
                            {item.hint}
                          </span>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
              );
            })}
            {items.length === 0 ? (
              <p className="px-3 py-8 text-center text-sm text-app-muted">
                {search.isFetching ? "Buscando…" : "Nada encontrado."}
              </p>
            ) : null}
            {term.length >= 2 && search.isFetching && items.length > 0 ? (
              <p className="px-3 py-2 text-xs text-app-muted">Buscando lançamentos…</p>
            ) : null}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
