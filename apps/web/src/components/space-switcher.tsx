import type { Space, SpaceKind } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  ArrowLeft,
  Briefcase,
  Check,
  ChevronsUpDown,
  House,
  LayoutGrid,
  type LucideIcon,
  Plus,
  User,
} from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { z } from "zod";
import { cn } from "../lib/cn";
import { useCreateSpace } from "../lib/queries";
import { setActiveSpace } from "../lib/space";
import { SPACE_KIND_LABEL, useActiveSpace } from "../lib/use-active-space";
import { ALL_NAV_ITEMS, isActivePath } from "./layout/nav";
import { Button } from "./ui/button";
import { Field, Input } from "./ui/field";
import { Sheet } from "./ui/sheet";

export const SPACE_KIND_ICON: Record<SpaceKind, LucideIcon> = {
  PERSONAL: User,
  HOME: House,
  BUSINESS: Briefcase,
};

const SPACE_KIND_HINT: Record<SpaceKind, string> = {
  PERSONAL: "Suas finanças individuais",
  HOME: "Despesas em comum, com quem pagou e a divisão",
  BUSINESS: "Contas da empresa, MRR, DRE e runway",
};

const SPACE_COLORS = ["#34d399", "#38bdf8", "#a78bfa", "#f472b6", "#fb923c", "#facc15"] as const;

/** Bolinha com a cor do espaço e o ícone do tipo. */
export function SpaceBadge({
  space,
  size = 32,
}: {
  space: Pick<Space, "kind" | "color">;
  size?: number;
}) {
  const Icon = SPACE_KIND_ICON[space.kind] ?? User;
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-lg"
      style={{
        width: size,
        height: size,
        backgroundColor: `${space.color}26`,
        color: space.color,
      }}
      aria-hidden
    >
      <Icon size={Math.round(size / 2)} strokeWidth={2.2} />
    </span>
  );
}

/** Botão com o espaço ativo; abre o seletor. */
export function SpaceSwitcherButton({
  onClick,
  className,
}: {
  onClick: () => void;
  className?: string;
}) {
  const { slug, space, kind } = useActiveSpace();
  const current = space ?? { slug, name: slug, kind, color: "#34d399" };
  return (
    <button
      type="button"
      onClick={onClick}
      aria-haspopup="dialog"
      aria-label={`Espaço: ${current.name} (${SPACE_KIND_LABEL[current.kind]}). Trocar de espaço`}
      data-testid="space-switcher"
      data-space={slug}
      data-kind={current.kind}
      className={cn(
        "flex min-h-11 w-full cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border px-2.5 py-1.5 text-left transition-colors hover:border-app-muted",
        className,
      )}
    >
      <SpaceBadge space={current} size={30} />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium text-app-text">{current.name}</span>
        <span className="block text-[11px] text-app-muted">{SPACE_KIND_LABEL[current.kind]}</span>
      </span>
      <ChevronsUpDown size={15} className="shrink-0 text-app-subtle" aria-hidden />
    </button>
  );
}

/** Versão só com o ícone para o topo mobile; aparece quando existe mais de um espaço. */
export function SpaceSwitcherCompact({ onClick }: { onClick: () => void }) {
  const { slug, space, kind, spaces } = useActiveSpace();
  if (spaces.length < 2) return null;
  const current = space ?? { slug, name: slug, kind, color: "#34d399" };
  return (
    <button
      type="button"
      onClick={onClick}
      aria-haspopup="dialog"
      aria-label={`Espaço: ${current.name} (${SPACE_KIND_LABEL[current.kind]}). Trocar de espaço`}
      data-testid="space-switcher-compact"
      className="flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-(--radius-control) hover:bg-app-soft"
    >
      <SpaceBadge space={current} size={28} />
    </button>
  );
}

/** Troca o espaço; se a página atual não existe no novo tipo, volta para a visão geral. */
export function useSwitchSpace(): (space: Space) => void {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  return (space) => {
    const item = ALL_NAV_ITEMS.find((i) => isActivePath(pathname, i.to) && i.to !== "/");
    if (item?.kinds && !item.kinds.includes(space.kind)) navigate("/");
    setActiveSpace(space.slug, space.kind);
  };
}

export function SpaceSheet({
  open,
  onOpenChange,
  startCreating = false,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Abre direto no formulário de "Novo espaço". */
  startCreating?: boolean;
}) {
  const { slug, spaces } = useActiveSpace();
  const [creating, setCreating] = useState(startCreating);
  const switchTo = useSwitchSpace();

  useEffect(() => {
    if (open) setCreating(startCreating);
  }, [open, startCreating]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title={creating ? "Novo espaço" : "Espaços"}>
      {creating ? (
        <NewSpaceForm
          onCancel={() => setCreating(false)}
          onCreated={(space) => {
            onOpenChange(false);
            switchTo(space);
          }}
        />
      ) : (
        <div className="flex flex-col gap-3 pb-2">
          <ul className="flex flex-col gap-1.5" aria-label="Seus espaços">
            {spaces.map((s) => {
              const active = s.slug === slug;
              return (
                <li key={s.slug}>
                  <button
                    type="button"
                    aria-pressed={active}
                    data-testid={`space-option-${s.slug}`}
                    onClick={() => {
                      onOpenChange(false);
                      if (!active) switchTo(s);
                    }}
                    className={cn(
                      "flex min-h-12 w-full cursor-pointer items-center gap-3 rounded-(--radius-control) border px-3 py-2 text-left transition-colors",
                      active
                        ? "border-emerald-500/40 bg-emerald-500/10"
                        : "border-app-border hover:bg-app-soft",
                    )}
                  >
                    <SpaceBadge space={s} />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-app-text">
                        {s.name}
                      </span>
                      <span className="block text-xs text-app-muted">
                        {SPACE_KIND_LABEL[s.kind] ?? s.kind}
                      </span>
                    </span>
                    {active ? (
                      <Check size={16} className="shrink-0 text-income" aria-hidden />
                    ) : null}
                  </button>
                </li>
              );
            })}
          </ul>
          <div className="grid grid-cols-2 gap-2">
            <Link
              to="/espacos"
              onClick={() => onOpenChange(false)}
              className="flex min-h-11 items-center justify-center gap-2 rounded-(--radius-control) border border-app-border px-3 text-sm text-app-text hover:bg-app-soft"
            >
              <LayoutGrid size={16} aria-hidden />
              Visão unificada
            </Link>
            <Button variant="outline" onClick={() => setCreating(true)} className="min-h-11">
              <Plus size={16} aria-hidden />
              Novo espaço
            </Button>
          </div>
          <p className="text-xs text-app-muted">
            Cada espaço tem contas, categorias e lançamentos próprios, guardados num banco separado
            neste computador.
          </p>
        </div>
      )}
    </Sheet>
  );
}

const newSpaceSchema = z.object({
  name: z.string().trim().min(1, "dê um nome ao espaço").max(60),
  kind: z.enum(["PERSONAL", "HOME", "BUSINESS"]),
  color: z.string().regex(/^#[0-9a-fA-F]{6}$/, "cor hexadecimal inválida"),
});
type NewSpaceValues = z.infer<typeof newSpaceSchema>;

const KIND_OPTIONS: readonly SpaceKind[] = ["HOME", "BUSINESS", "PERSONAL"];

function NewSpaceForm({
  onCancel,
  onCreated,
}: {
  onCancel: () => void;
  onCreated: (space: Space) => void;
}) {
  const create = useCreateSpace();
  const form = useForm<NewSpaceValues>({
    resolver: zodResolver(newSpaceSchema),
    defaultValues: { name: "", kind: "HOME", color: SPACE_COLORS[1] },
  });
  const kind = form.watch("kind");
  const color = form.watch("color");

  const submit = form.handleSubmit(async (values) => {
    const created = await create.mutateAsync({
      name: values.name.trim(),
      kind: values.kind,
      color: values.color,
    });
    // resposta sem slug (backend antigo): só volta para a lista
    if (!created?.slug) {
      onCancel();
      return;
    }
    onCreated({
      slug: created.slug,
      name: created.name ?? values.name.trim(),
      kind: created.kind ?? values.kind,
      color: created.color ?? values.color,
    });
  });

  return (
    <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-space">
      <Field label="Nome" error={form.formState.errors.name?.message}>
        <Input placeholder="Casa, Minha empresa…" autoComplete="off" {...form.register("name")} />
      </Field>
      <fieldset className="flex flex-col gap-1.5">
        <legend className="pb-1.5 text-sm text-app-muted">Tipo</legend>
        {KIND_OPTIONS.map((k) => {
          const Icon = SPACE_KIND_ICON[k];
          return (
            <label
              key={k}
              className={cn(
                "flex min-h-12 cursor-pointer items-center gap-3 rounded-(--radius-control) border px-3 py-2 transition-colors",
                kind === k
                  ? "border-emerald-500/40 bg-emerald-500/10"
                  : "border-app-border hover:bg-app-soft",
              )}
            >
              <input type="radio" value={k} className="sr-only" {...form.register("kind")} />
              <Icon size={18} aria-hidden className="shrink-0 text-app-muted" />
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium text-app-text">
                  {SPACE_KIND_LABEL[k]}
                </span>
                <span className="block text-xs text-app-muted">{SPACE_KIND_HINT[k]}</span>
              </span>
            </label>
          );
        })}
      </fieldset>
      <Field label="Cor" error={form.formState.errors.color?.message}>
        <div className="flex flex-wrap gap-1">
          {SPACE_COLORS.map((hex) => (
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
      {create.error ? <p className="text-sm text-expense">{create.error.message}</p> : null}
      <div className="grid grid-cols-[auto_1fr] gap-2">
        <Button variant="ghost" onClick={onCancel} aria-label="Voltar para a lista de espaços">
          <ArrowLeft size={16} aria-hidden />
        </Button>
        <Button type="submit" disabled={create.isPending} className="h-12">
          {create.isPending ? "Criando…" : "Criar espaço"}
        </Button>
      </div>
    </form>
  );
}
