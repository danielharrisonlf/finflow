import type { Category, CategoryKind, CategoryRule, SpendType } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import * as Tabs from "@radix-ui/react-tabs";
import { Pencil, Plus, Tags, Trash2, Wand2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card, CardHeader } from "../components/ui/card";
import {
  CATEGORY_ICON_NAMES,
  CategoryIcon,
  formatCategoryLabel,
  iconByName,
} from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Badge } from "../components/ui/segmented";
import { Sheet } from "../components/ui/sheet";
import { SPEND_TYPE_META, SPEND_TYPE_ORDER } from "../lib/bands";
import { cn } from "../lib/cn";
import {
  useApplyRules,
  useCategories,
  useCreateCategory,
  useCreateRule,
  useDeleteCategory,
  useDeleteRule,
  useRules,
  useUpdateCategory,
  useUpdateRule,
} from "../lib/queries";

const tabCls =
  "min-h-10 flex-1 cursor-pointer rounded-(--radius-control) px-4 text-sm text-app-muted transition-colors data-[state=active]:bg-app-soft data-[state=active]:font-medium data-[state=active]:text-app-text sm:flex-none";

/** Tipo de gasto efetivo: o próprio ou, em subcategoria sem valor, o do pai. */
function effectiveSpendType(
  cat: Category,
  all: Category[],
): { type: SpendType; inherited: boolean } | null {
  if (cat.kind !== "EXPENSE") return null;
  if (cat.spendType) return { type: cat.spendType, inherited: false };
  const parent = cat.parentId ? all.find((p) => p.id === cat.parentId) : null;
  if (parent?.spendType) return { type: parent.spendType, inherited: true };
  return { type: "VARIABLE", inherited: Boolean(parent) };
}

const PRESET_COLORS = [
  "#10b981",
  "#34d399",
  "#22d3ee",
  "#38bdf8",
  "#60a5fa",
  "#a78bfa",
  "#f472b6",
  "#fb7185",
  "#fb923c",
  "#fbbf24",
  "#84cc16",
  "#94a3b8",
] as const;

export function CategoriesPage() {
  const categories = useCategories();
  const remove = useDeleteCategory();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState<Category | null>(null);
  const [tab, setTab] = useState("categories");
  const [ruleSheet, setRuleSheet] = useState<{ open: boolean; editing: CategoryRule | null }>({
    open: false,
    editing: null,
  });

  const all = categories.data ?? [];
  const expenseCategories = all.filter((c) => c.kind === "EXPENSE");
  const incomeCategories = all.filter((c) => c.kind === "INCOME");

  const openCreate = (): void => {
    setEditing(null);
    setSheetOpen(true);
  };

  const openEdit = (cat: Category): void => {
    setEditing(cat);
    setSheetOpen(true);
  };

  const onDelete = (cat: Category): void => {
    if (
      window.confirm(
        `Excluir a categoria "${cat.name}"? Os lançamentos existentes ficarão sem categoria.`,
      )
    ) {
      remove.mutate(cat.id);
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Categorias</h1>
          <p className="text-sm text-app-muted">
            {categories.data
              ? `${all.length} categorias (${expenseCategories.length} de despesa e ${incomeCategories.length} de receita)`
              : "Organize receitas, despesas e subcategorias"}
          </p>
        </div>
        {tab === "categories" ? (
          <Button variant="outline" size="sm" onClick={openCreate}>
            <Plus size={15} /> Nova categoria
          </Button>
        ) : (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setRuleSheet({ open: true, editing: null })}
          >
            <Plus size={15} /> Nova regra
          </Button>
        )}
      </header>

      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List
          className="flex gap-1 rounded-(--radius-control) bg-app-background/70 p-1 sm:inline-flex"
          aria-label="Seções"
        >
          <Tabs.Trigger value="categories" className={tabCls}>
            Categorias
          </Tabs.Trigger>
          <Tabs.Trigger value="rules" className={tabCls}>
            Automações
          </Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="categories" className="mt-4 outline-none">
          {categories.isLoading ? (
            <Card>
              <LoadingRows rows={6} />
            </Card>
          ) : all.length === 0 ? (
            <Card>
              <Empty
                icon={Tags}
                title="Nenhuma categoria cadastrada"
                hint="Crie categorias de despesa e receita para classificar lançamentos e definir orçamentos."
                action={<Button onClick={openCreate}>Criar primeira categoria</Button>}
              />
            </Card>
          ) : (
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2" data-testid="categories-grid">
              <CategorySection
                title="Despesas"
                items={expenseCategories}
                all={all}
                onEdit={openEdit}
                onDelete={onDelete}
              />
              <CategorySection
                title="Receitas"
                items={incomeCategories}
                all={all}
                onEdit={openEdit}
                onDelete={onDelete}
              />
            </div>
          )}
        </Tabs.Content>

        <Tabs.Content value="rules" className="mt-4 outline-none">
          <RulesPanel
            onCreate={() => setRuleSheet({ open: true, editing: null })}
            onEdit={(rule) => setRuleSheet({ open: true, editing: rule })}
          />
        </Tabs.Content>
      </Tabs.Root>

      <RuleSheet
        open={ruleSheet.open}
        editing={ruleSheet.editing}
        categories={all}
        onOpenChange={(open) => setRuleSheet((s) => ({ open, editing: open ? s.editing : null }))}
      />

      <CategorySheet
        open={sheetOpen}
        onOpenChange={(open) => {
          setSheetOpen(open);
          if (!open) setEditing(null);
        }}
        editing={editing}
        allCategories={all}
      />
    </div>
  );
}

function CategorySection({
  title,
  items,
  all,
  onEdit,
  onDelete,
}: {
  title: string;
  items: Category[];
  all: Category[];
  onEdit: (cat: Category) => void;
  onDelete: (cat: Category) => void;
}) {
  return (
    <Card>
      <CardHeader
        title={title}
        action={<span className="tnum text-xs text-app-muted">{items.length}</span>}
      />
      {items.length === 0 ? (
        <p className="px-5 pb-5 text-sm text-app-muted">Nenhuma categoria neste grupo.</p>
      ) : (
        <ul className="divide-y divide-app-border/60">
          {items.map((cat) => {
            const parent = cat.parentId ? all.find((p) => p.id === cat.parentId) : null;
            const spend = effectiveSpendType(cat, all);
            return (
              <li key={cat.id} className="group flex items-center gap-3 px-4 py-3 md:px-5">
                <CategoryIcon icon={cat.icon} color={cat.color} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-app-text">{cat.name}</p>
                  {parent ? (
                    <p className="truncate text-xs text-app-muted">Subcategoria de {parent.name}</p>
                  ) : null}
                </div>
                {spend ? (
                  <Badge
                    title={
                      spend.inherited
                        ? `${SPEND_TYPE_META[spend.type].label} (herdado da categoria principal)`
                        : SPEND_TYPE_META[spend.type].hint
                    }
                    data-testid="spend-type-badge"
                  >
                    <span
                      className="h-1.5 w-1.5 rounded-full"
                      style={{ backgroundColor: SPEND_TYPE_META[spend.type].color }}
                      aria-hidden
                    />
                    {SPEND_TYPE_META[spend.type].label}
                    {spend.inherited ? <span className="font-normal">· herdado</span> : null}
                  </Badge>
                ) : null}
                <button
                  type="button"
                  onClick={() => onEdit(cat)}
                  aria-label={`Editar ${cat.name}`}
                  className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-muted opacity-0 md:h-8 md:w-8 transition-opacity group-hover:opacity-100 hover:bg-app-soft hover:text-app-text focus-visible:opacity-100 max-md:opacity-100"
                >
                  <Pencil size={14} />
                </button>
                <button
                  type="button"
                  onClick={() => onDelete(cat)}
                  aria-label={`Excluir ${cat.name}`}
                  className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-subtle opacity-0 md:h-8 md:w-8 transition-opacity group-hover:opacity-100 hover:bg-rose-500/10 hover:text-expense focus-visible:opacity-100 max-md:opacity-100"
                >
                  <Trash2 size={15} />
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}

const categoryFormSchema = z.object({
  name: z.string().min(1, "informe o nome da categoria").max(40),
  kind: z.enum(["EXPENSE", "INCOME"]),
  icon: z.string().min(1),
  color: z.string().regex(/^#[0-9a-fA-F]{6}$/, "cor hexadecimal inválida"),
  parentId: z.string(),
  spendType: z.string(),
});
type CategoryFormValues = z.infer<typeof categoryFormSchema>;

function CategorySheet({
  open,
  onOpenChange,
  editing,
  allCategories,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editing: Category | null;
  allCategories: Category[];
}) {
  const create = useCreateCategory();
  const update = useUpdateCategory();

  const form = useForm<CategoryFormValues>({
    resolver: zodResolver(categoryFormSchema),
    defaultValues: {
      name: "",
      kind: "EXPENSE",
      icon: "tag",
      color: "#fb923c",
      parentId: "",
      spendType: "VARIABLE",
    },
  });

  useEffect(() => {
    if (editing) {
      form.reset({
        name: editing.name,
        kind: editing.kind,
        icon: editing.icon,
        color: editing.color,
        parentId: editing.parentId ?? "",
        spendType: editing.spendType ?? (editing.parentId ? "" : "VARIABLE"),
      });
    } else if (open) {
      form.reset({
        name: "",
        kind: "EXPENSE",
        icon: "tag",
        color: "#fb923c",
        parentId: "",
        spendType: "VARIABLE",
      });
    }
  }, [editing, open, form]);

  const selectedKind = form.watch("kind") as CategoryKind;
  const selectedIcon = form.watch("icon");
  const selectedColor = form.watch("color");
  const selectedParent = form.watch("parentId");
  const selectedSpend = form.watch("spendType");

  const parentCandidates = allCategories.filter(
    (c) => c.kind === selectedKind && !c.parentId && c.id !== editing?.id,
  );

  const submit = form.handleSubmit(async (values) => {
    const payload = {
      name: values.name.trim(),
      kind: values.kind,
      icon: values.icon,
      color: values.color,
      parentId: values.parentId || null,
      // subcategoria com "herdar" envia null; principal sem valor cai no padrão Variável
      spendType:
        values.kind === "INCOME"
          ? null
          : ((values.spendType || (values.parentId ? null : "VARIABLE")) as SpendType | null),
    };
    if (editing) {
      await update.mutateAsync({ id: editing.id, ...payload });
    } else {
      await create.mutateAsync(payload);
    }
    onOpenChange(false);
  });

  const isPending = create.isPending || update.isPending;
  const error = create.error?.message ?? update.error?.message;

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? `Editar — ${editing.name}` : "Nova categoria"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-category">
        <Field label="Nome" error={form.formState.errors.name?.message}>
          <Input placeholder="Ex.: Restaurantes, Pet, Dividendos…" {...form.register("name")} />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Tipo">
            <Select
              {...form.register("kind", {
                onChange: () => form.setValue("parentId", ""),
              })}
            >
              <option value="EXPENSE">Despesa</option>
              <option value="INCOME">Receita</option>
            </Select>
          </Field>
          <Field label="Subcategoria de (opcional)">
            <Select {...form.register("parentId")}>
              <option value="">Nenhuma (principal)</option>
              {parentCandidates.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </Field>
        </div>

        {selectedKind === "EXPENSE" ? (
          <Field label="Tipo de gasto">
            <Select {...form.register("spendType")}>
              {selectedParent ? <option value="">Herdar da categoria principal</option> : null}
              {SPEND_TYPE_ORDER.map((t) => (
                <option key={t} value={t}>
                  {SPEND_TYPE_META[t].label}
                </option>
              ))}
            </Select>
            <span className="text-xs text-app-muted">
              {selectedSpend
                ? SPEND_TYPE_META[selectedSpend as SpendType]?.hint
                : "Usa o mesmo tipo da categoria principal."}{" "}
              Fixo e Cortável entram nas Saídas; Variável, no Diário.
            </span>
          </Field>
        ) : null}

        <Field label="Ícone">
          <div className="grid grid-cols-7 gap-1.5 rounded-(--radius-control) border border-app-border bg-app-background p-2">
            {CATEGORY_ICON_NAMES.map((iconName) => {
              const Icon = iconByName(iconName);
              const active = selectedIcon === iconName;
              return (
                <button
                  key={iconName}
                  type="button"
                  onClick={() => form.setValue("icon", iconName)}
                  title={iconName}
                  aria-label={`Ícone ${iconName}`}
                  className={cn(
                    "flex h-9 cursor-pointer items-center justify-center rounded-lg transition-colors",
                    active
                      ? "bg-app-soft text-app-text ring-1 ring-emerald-400"
                      : "text-app-muted hover:bg-app-surface hover:text-app-text",
                  )}
                  style={active ? { color: selectedColor } : undefined}
                >
                  <Icon size={17} />
                </button>
              );
            })}
          </div>
        </Field>

        <Field label="Cor" error={form.formState.errors.color?.message}>
          <div className="flex items-center gap-2">
            <div className="flex flex-1 flex-wrap gap-1.5">
              {PRESET_COLORS.map((hex) => (
                <button
                  key={hex}
                  type="button"
                  onClick={() => form.setValue("color", hex)}
                  aria-label={`Cor ${hex}`}
                  className={cn(
                    "h-6 w-6 cursor-pointer rounded-full transition-transform",
                    selectedColor.toLowerCase() === hex.toLowerCase() &&
                      "scale-110 ring-2 ring-white ring-offset-2 ring-offset-zinc-950",
                  )}
                  style={{ backgroundColor: hex }}
                />
              ))}
            </div>
            <Input type="color" className="h-10 w-14 shrink-0 p-1" {...form.register("color")} />
          </div>
        </Field>

        {error ? <p className="text-sm text-expense">{error}</p> : null}
        <Button type="submit" disabled={isPending} className="h-12">
          {isPending ? "Salvando…" : editing ? "Salvar alterações" : "Criar categoria"}
        </Button>
      </form>
    </Sheet>
  );
}

// ───────────────────────── automações ─────────────────────────

function RulesPanel({
  onCreate,
  onEdit,
}: {
  onCreate: () => void;
  onEdit: (rule: CategoryRule) => void;
}) {
  const rules = useRules();
  const remove = useDeleteRule();
  const apply = useApplyRules();
  const sorted = [...(rules.data ?? [])].sort(
    (a, b) => b.priority - a.priority || b.pattern.length - a.pattern.length,
  );

  const onDelete = (rule: CategoryRule): void => {
    if (window.confirm(`Excluir a regra "${rule.pattern}"?`)) remove.mutate(rule.id);
  };

  return (
    <div className="space-y-3">
      <Card className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-app-muted">
          Quando a descrição contém o texto da regra, a transação recebe a categoria
          automaticamente: em lançamentos novos, compras no cartão, Telegram e importações.
          Maiúsculas e acentos não importam.
        </p>
        <Button
          variant="outline"
          className="shrink-0"
          disabled={apply.isPending || sorted.length === 0}
          onClick={() => apply.mutate({ onlyUncategorized: true })}
        >
          <Wand2 size={15} aria-hidden />
          {apply.isPending ? "Aplicando…" : "Aplicar às transações sem categoria"}
        </Button>
      </Card>
      <p role="status" className="text-sm" data-testid="rules-apply-result">
        {apply.data ? (
          <span className="text-income">
            {apply.data.updated === 0
              ? "Nenhuma transação sem categoria combinou com as regras."
              : `${apply.data.updated} ${apply.data.updated === 1 ? "transação categorizada" : "transações categorizadas"}.`}
          </span>
        ) : apply.error ? (
          <span className="text-expense">{apply.error.message}</span>
        ) : null}
      </p>

      <Card>
        {rules.isLoading ? (
          <LoadingRows rows={3} />
        ) : sorted.length === 0 ? (
          <Empty
            icon={Wand2}
            title="Nenhuma regra ainda"
            hint="Exemplo: descrição contém “ifood” → Delivery. Crie uma vez e nunca mais categorize à mão."
            action={<Button onClick={onCreate}>Criar primeira regra</Button>}
          />
        ) : (
          <ul className="divide-y divide-app-border/60" data-testid="rules-list">
            {sorted.map((rule) => (
              <li key={rule.id} className="flex items-center gap-3 px-4 py-3 md:px-5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm text-app-text">
                    contém <span className="font-semibold">“{rule.pattern}”</span>
                  </p>
                  <p className="truncate text-xs text-app-muted">
                    → {rule.categoryName}
                    {rule.priority !== 0 ? ` · prioridade ${rule.priority}` : ""}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => onEdit(rule)}
                  aria-label={`Editar regra ${rule.pattern}`}
                  className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-muted hover:bg-app-soft hover:text-app-text md:h-9 md:w-9"
                >
                  <Pencil size={14} />
                </button>
                <button
                  type="button"
                  onClick={() => onDelete(rule)}
                  aria-label={`Excluir regra ${rule.pattern}`}
                  className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-full text-app-subtle hover:bg-rose-500/10 hover:text-expense md:h-9 md:w-9"
                >
                  <Trash2 size={15} />
                </button>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}

const ruleFormSchema = z.object({
  pattern: z.string().trim().min(1, "informe o texto a procurar").max(120),
  categoryId: z.string().min(1, "escolha a categoria"),
  priority: z
    .string()
    .refine((v) => v.trim() === "" || Number.isInteger(Number(v)), "número inteiro"),
});
type RuleFormValues = z.infer<typeof ruleFormSchema>;

function RuleSheet({
  open,
  editing,
  categories,
  onOpenChange,
}: {
  open: boolean;
  editing: CategoryRule | null;
  categories: Category[];
  onOpenChange: (open: boolean) => void;
}) {
  const create = useCreateRule();
  const update = useUpdateRule();
  const form = useForm<RuleFormValues>({
    resolver: zodResolver(ruleFormSchema),
    defaultValues: { pattern: "", categoryId: "", priority: "" },
  });

  useEffect(() => {
    if (!open) return;
    form.reset(
      editing
        ? {
            pattern: editing.pattern,
            categoryId: editing.categoryId,
            priority: String(editing.priority),
          }
        : { pattern: "", categoryId: "", priority: "" },
    );
  }, [open, editing, form]);

  const submit = form.handleSubmit(async (v) => {
    const payload = {
      pattern: v.pattern.trim(),
      categoryId: v.categoryId,
      priority: v.priority.trim() === "" ? undefined : Number(v.priority),
    };
    if (editing) await update.mutateAsync({ id: editing.id, ...payload });
    else await create.mutateAsync(payload);
    onOpenChange(false);
  });

  const errors = form.formState.errors;
  const error = create.error?.message ?? update.error?.message;
  const pending = create.isPending || update.isPending;
  const groups = [
    { label: "Despesas", items: categories.filter((c) => c.kind === "EXPENSE") },
    { label: "Receitas", items: categories.filter((c) => c.kind === "INCOME") },
  ];

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? "Editar regra" : "Nova regra de categorização"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-rule">
        <Field label="Quando a descrição contém" error={errors.pattern?.message}>
          <Input placeholder="ifood, uber, netflix…" {...form.register("pattern")} />
        </Field>
        <Field label="Usar a categoria" error={errors.categoryId?.message}>
          <Select {...form.register("categoryId")}>
            <option value="" disabled>
              Escolher…
            </option>
            {groups.map((g) => (
              <optgroup key={g.label} label={g.label}>
                {g.items.map((c) => (
                  <option key={c.id} value={c.id}>
                    {formatCategoryLabel(c, categories)}
                  </option>
                ))}
              </optgroup>
            ))}
          </Select>
        </Field>
        <Field label="Prioridade (opcional)" error={errors.priority?.message}>
          <Input inputMode="numeric" placeholder="0" {...form.register("priority")} />
        </Field>
        <p className="text-xs text-app-muted">
          Se duas regras combinarem, vence a de maior prioridade; no empate, o texto mais longo. A
          regra só vale para transações do mesmo tipo da categoria (receita ou despesa).
        </p>
        {error ? <p className="text-sm text-expense">{error}</p> : null}
        <Button type="submit" disabled={pending} className="h-12">
          {pending ? "Salvando…" : editing ? "Salvar regra" : "Criar regra"}
        </Button>
      </form>
    </Sheet>
  );
}
