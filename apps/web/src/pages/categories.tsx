import type { Category, CategoryKind } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import { Pencil, Plus, Tags, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card, CardHeader } from "../components/ui/card";
import { CATEGORY_ICON_NAMES, CategoryIcon, iconByName } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Sheet } from "../components/ui/sheet";
import { cn } from "../lib/cn";
import {
  useCategories,
  useCreateCategory,
  useDeleteCategory,
  useUpdateCategory,
} from "../lib/queries";

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
        <Button variant="outline" size="sm" onClick={openCreate}>
          <Plus size={15} /> Nova categoria
        </Button>
      </header>

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
        <div className="grid gap-4 lg:grid-cols-2" data-testid="categories-grid">
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
            return (
              <li key={cat.id} className="group flex items-center gap-3 px-4 py-3 md:px-5">
                <CategoryIcon icon={cat.icon} color={cat.color} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-app-text">{cat.name}</p>
                  {parent ? (
                    <p className="truncate text-xs text-app-muted">Subcategoria de {parent.name}</p>
                  ) : null}
                </div>
                <button
                  type="button"
                  onClick={() => onEdit(cat)}
                  aria-label={`Editar ${cat.name}`}
                  className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full text-app-muted opacity-0 transition-opacity group-hover:opacity-100 hover:bg-app-soft hover:text-app-text focus-visible:opacity-100 max-md:opacity-100"
                >
                  <Pencil size={14} />
                </button>
                <button
                  type="button"
                  onClick={() => onDelete(cat)}
                  aria-label={`Excluir ${cat.name}`}
                  className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full text-app-subtle opacity-0 transition-opacity group-hover:opacity-100 hover:bg-rose-500/10 hover:text-expense focus-visible:opacity-100 max-md:opacity-100"
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
      });
    } else if (open) {
      form.reset({
        name: "",
        kind: "EXPENSE",
        icon: "tag",
        color: "#fb923c",
        parentId: "",
      });
    }
  }, [editing, open, form]);

  const selectedKind = form.watch("kind") as CategoryKind;
  const selectedIcon = form.watch("icon");
  const selectedColor = form.watch("color");

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
