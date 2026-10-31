import type { Budget } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import { ChartPie, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { CategoryIcon } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input, Select } from "../components/ui/field";
import { Progress } from "../components/ui/progress";
import { Sheet } from "../components/ui/sheet";
import { formatBRL, parseBRLInput } from "../lib/format";
import { useBudgets, useCategories, useDeleteBudget, useUpsertBudget } from "../lib/queries";

const STATUS_HINT = {
  OK: { label: "dentro do planejado", cls: "text-emerald-300" },
  WARNING: { label: "atenção ao ritmo", cls: "text-amber-300" },
  DANGER: { label: "quase estourado", cls: "text-rose-300" },
} as const;

export function BudgetsPage() {
  const budgets = useBudgets();
  const remove = useDeleteBudget();
  const [sheetOpen, setSheetOpen] = useState(false);

  const totalBudget = budgets.data?.reduce((acc, b) => acc + b.amountCents, 0) ?? 0;
  const totalSpent = budgets.data?.reduce((acc, b) => acc + b.spentCents, 0) ?? 0;

  const onDelete = (budget: Budget): void => {
    if (window.confirm(`Remover o orçamento de ${budget.categoryName}?`)) {
      remove.mutate(budget.id);
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Orçamentos</h1>
          <p className="text-sm text-zinc-500">
            {budgets.data && budgets.data.length > 0 ? (
              <>
                <span className="tnum text-zinc-300">{formatBRL(totalSpent)}</span> gastos de{" "}
                <span className="tnum text-zinc-300">{formatBRL(totalBudget)}</span> planejados
              </>
            ) : (
              "Tetos mensais por categoria"
            )}
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => setSheetOpen(true)}>
          <Plus size={15} /> Definir orçamento
        </Button>
      </header>

      {budgets.isLoading ? (
        <Card>
          <LoadingRows rows={4} />
        </Card>
      ) : (budgets.data?.length ?? 0) === 0 ? (
        <Card>
          <Empty
            icon={ChartPie}
            title="Nenhum orçamento definido"
            hint="Defina um teto mensal por categoria e acompanhe o progresso: verde até 70%, amarelo até 90%, vermelho acima."
            action={<Button onClick={() => setSheetOpen(true)}>Definir primeiro orçamento</Button>}
          />
        </Card>
      ) : (
        <div className="grid gap-3 md:grid-cols-2" data-testid="budgets-grid">
          {budgets.data?.map((budget) => {
            const hint = STATUS_HINT[budget.status];
            return (
              <Card key={budget.id} className="group p-4">
                <div className="flex items-center gap-3">
                  <CategoryIcon icon={budget.categoryIcon} color={budget.categoryColor} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-zinc-200">
                      {budget.categoryName}
                    </p>
                    <p className={`text-xs ${hint.cls}`}>{hint.label}</p>
                  </div>
                  <p className="tnum text-sm text-zinc-400">
                    <span className="font-semibold text-zinc-100">
                      {formatBRL(budget.spentCents)}
                    </span>{" "}
                    / {formatBRL(budget.amountCents)}
                  </p>
                  <button
                    type="button"
                    onClick={() => onDelete(budget)}
                    aria-label={`Remover orçamento de ${budget.categoryName}`}
                    className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full text-zinc-600 opacity-0 transition-opacity group-hover:opacity-100 hover:bg-rose-500/10 hover:text-rose-400 focus-visible:opacity-100 max-md:opacity-100"
                  >
                    <Trash2 size={15} />
                  </button>
                </div>
                <div className="mt-3 flex items-center gap-3">
                  <Progress percent={budget.percent} tone={budget.status} className="flex-1" />
                  <span className="tnum w-12 text-right text-xs text-zinc-400">
                    {budget.percent}%
                  </span>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      <BudgetSheet open={sheetOpen} onOpenChange={setSheetOpen} />
    </div>
  );
}

const budgetSchema = z.object({
  categoryId: z.string().min(1, "escolha a categoria"),
  amount: z.string().refine((v) => parseBRLInput(v) !== null, "valor inválido — use 1.500,00"),
});

function BudgetSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const categories = useCategories("EXPENSE");
  const upsert = useUpsertBudget();
  const form = useForm<z.infer<typeof budgetSchema>>({
    resolver: zodResolver(budgetSchema),
    defaultValues: { categoryId: "", amount: "" },
  });

  const submit = form.handleSubmit(async (values) => {
    await upsert.mutateAsync({
      categoryId: values.categoryId,
      amountCents: parseBRLInput(values.amount) ?? 0,
    });
    form.reset();
    onOpenChange(false);
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Teto mensal por categoria">
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-budget">
        <Field label="Categoria" error={form.formState.errors.categoryId?.message}>
          <Select {...form.register("categoryId")} defaultValue="">
            <option value="" disabled>
              Escolher…
            </option>
            {categories.data?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Teto mensal" error={form.formState.errors.amount?.message}>
          <Input inputMode="decimal" placeholder="1.500,00" {...form.register("amount")} />
        </Field>
        {upsert.error ? <p className="text-sm text-rose-400">{upsert.error.message}</p> : null}
        <Button type="submit" disabled={upsert.isPending} className="h-12">
          {upsert.isPending ? "Salvando…" : "Salvar orçamento"}
        </Button>
      </form>
    </Sheet>
  );
}
