import { Check, Plus } from "lucide-react";
import { useState } from "react";
import { cn } from "../../lib/cn";
import { useCreateTag, useTags } from "../../lib/queries";

/** Cores sugeridas para tags novas (mesma família das categorias). */
export const TAG_COLORS = [
  "#38bdf8",
  "#a78bfa",
  "#f472b6",
  "#fb923c",
  "#fbbf24",
  "#34d399",
  "#22d3ee",
  "#94a3b8",
] as const;

/** Etiqueta compacta tingida pela cor da tag (extrato, listas). */
export function TagChip({
  name,
  color,
  className,
}: {
  name: string;
  color: string | null | undefined;
  className?: string;
}) {
  const tint = color ?? "#94a3b8";
  return (
    <span
      className={cn(
        "inline-flex h-5 max-w-32 shrink-0 items-center gap-1 rounded-full border px-1.5 text-[11px] leading-none font-medium text-app-text",
        className,
      )}
      style={{ borderColor: `${tint}66`, backgroundColor: `${tint}1a` }}
    >
      <span
        className="h-1.5 w-1.5 shrink-0 rounded-full"
        style={{ backgroundColor: tint }}
        aria-hidden
      />
      <span className="truncate">{name}</span>
    </span>
  );
}

/**
 * Seleção múltipla de tags com criação rápida (formulários de lançamento).
 * `value` é a lista de ids; criar uma tag nova já a deixa selecionada.
 */
export function TagPicker({
  value,
  onChange,
  label = "Tags (opcional)",
}: {
  value: string[];
  onChange: (ids: string[]) => void;
  label?: string;
}) {
  const tags = useTags();
  const create = useCreateTag();
  const [draft, setDraft] = useState("");
  const all = tags.data ?? [];

  const toggle = (id: string): void =>
    onChange(value.includes(id) ? value.filter((v) => v !== id) : [...value, id]);

  const addDraft = async (): Promise<void> => {
    const name = draft.trim();
    if (!name) return;
    const existing = all.find((t) => t.name.toLowerCase() === name.toLowerCase());
    if (existing) {
      if (!value.includes(existing.id)) onChange([...value, existing.id]);
      setDraft("");
      return;
    }
    const created = await create.mutateAsync({
      name,
      color: TAG_COLORS[all.length % TAG_COLORS.length],
    });
    if (created?.id) onChange([...value, created.id]);
    setDraft("");
  };

  return (
    <fieldset className="flex flex-col gap-1.5" data-testid="tag-picker">
      <legend className="pb-1.5 text-sm text-app-muted">{label}</legend>
      {all.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {all.map((tag) => {
            const selected = value.includes(tag.id);
            return (
              <button
                key={tag.id}
                type="button"
                aria-pressed={selected}
                onClick={() => toggle(tag.id)}
                className={cn(
                  "inline-flex min-h-9 cursor-pointer items-center gap-1.5 rounded-full border px-3 text-sm transition-colors",
                  selected
                    ? "border-transparent font-medium text-app-text"
                    : "border-app-border text-app-muted hover:text-app-text",
                )}
                style={selected ? { backgroundColor: `${tag.color}33` } : undefined}
              >
                {selected ? (
                  <Check size={13} aria-hidden />
                ) : (
                  <span
                    className="h-2 w-2 rounded-full"
                    style={{ backgroundColor: tag.color }}
                    aria-hidden
                  />
                )}
                {tag.name}
              </button>
            );
          })}
        </div>
      ) : null}
      <div className="flex gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              void addDraft();
            }
          }}
          maxLength={40}
          placeholder="Nova tag: viagem, casa, trabalho…"
          aria-label="Nome da nova tag"
          className="h-10 min-w-0 flex-1 rounded-(--radius-control) border border-app-border/80 bg-app-surface px-3 text-sm text-app-text placeholder:text-app-muted focus:border-emerald-500 focus:outline-none"
        />
        <button
          type="button"
          onClick={() => void addDraft()}
          disabled={!draft.trim() || create.isPending}
          className="flex h-10 shrink-0 cursor-pointer items-center gap-1 rounded-(--radius-control) border border-app-border px-3 text-sm text-app-text hover:bg-app-soft disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Plus size={14} aria-hidden /> Criar
        </button>
      </div>
      {create.error ? <span className="text-xs text-expense">{create.error.message}</span> : null}
    </fieldset>
  );
}
