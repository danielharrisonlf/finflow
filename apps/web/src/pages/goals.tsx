import type { Goal } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import { PiggyBank, Plus } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { CategoryIcon } from "../components/ui/category-icon";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input } from "../components/ui/field";
import { Progress } from "../components/ui/progress";
import { Sheet } from "../components/ui/sheet";
import { formatBRL, formatDate, parseBRLInput } from "../lib/format";
import { useContributeGoal, useCreateGoal, useGoals } from "../lib/queries";

export function GoalsPage() {
  const goals = useGoals();
  const [createOpen, setCreateOpen] = useState(false);
  const [contributing, setContributing] = useState<Goal | null>(null);

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Metas</h1>
          <p className="text-sm text-app-muted">Caixinhas para os seus objetivos</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => setCreateOpen(true)}>
          <Plus size={15} /> Nova meta
        </Button>
      </header>

      {goals.isLoading ? (
        <Card>
          <LoadingRows rows={3} />
        </Card>
      ) : (goals.data?.length ?? 0) === 0 ? (
        <Card>
          <Empty
            icon={PiggyBank}
            title="Nenhuma meta ainda"
            hint="Crie uma caixinha — reserva de emergência, viagem, um sonho — e acompanhe quanto falta."
            action={<Button onClick={() => setCreateOpen(true)}>Criar primeira meta</Button>}
          />
        </Card>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3" data-testid="goals-grid">
          {goals.data?.map((goal) => (
            <Card key={goal.id} className="flex flex-col gap-4 p-5">
              <div className="flex items-center gap-3">
                <CategoryIcon icon={goal.icon} color={goal.color} size={40} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-app-text">{goal.name}</p>
                  <p className="text-xs text-app-muted">
                    {goal.deadline ? `até ${formatDate(goal.deadline)}` : "sem prazo"}
                  </p>
                </div>
                <span className="tnum text-sm font-semibold text-app-text">{goal.percent}%</span>
              </div>
              <div>
                <Progress
                  percent={goal.percent}
                  tone={goal.percent >= 100 ? "OK" : "NEUTRAL"}
                  className="h-2"
                />
                <div className="mt-2 flex items-baseline justify-between">
                  <p className="tnum text-lg font-semibold text-app-text">
                    {formatBRL(goal.savedCents)}
                  </p>
                  <p className="tnum text-xs text-app-muted">de {formatBRL(goal.targetCents)}</p>
                </div>
                {goal.monthlyNeededCents !== null ? (
                  <p className="mt-1 text-xs text-app-muted">
                    guarde{" "}
                    <span className="tnum font-medium text-income">
                      {formatBRL(goal.monthlyNeededCents)}/mês
                    </span>{" "}
                    para chegar lá no prazo
                  </p>
                ) : goal.percent >= 100 ? (
                  <p className="mt-1 text-xs text-income">meta concluída 🎉</p>
                ) : null}
              </div>
              <Button variant="outline" size="sm" onClick={() => setContributing(goal)}>
                Guardar / resgatar
              </Button>
            </Card>
          ))}
        </div>
      )}

      <NewGoalSheet open={createOpen} onOpenChange={setCreateOpen} />
      <ContributeSheet goal={contributing} onClose={() => setContributing(null)} />
    </div>
  );
}

const goalSchema = z.object({
  name: z.string().min(1, "dê um nome à meta"),
  target: z.string().refine((v) => parseBRLInput(v) !== null, "valor inválido"),
  deadline: z.string(),
});

function NewGoalSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const create = useCreateGoal();
  const form = useForm<z.infer<typeof goalSchema>>({
    resolver: zodResolver(goalSchema),
    defaultValues: { name: "", target: "", deadline: "" },
  });

  const submit = form.handleSubmit(async (values) => {
    await create.mutateAsync({
      name: values.name,
      icon: "piggy-bank",
      targetCents: parseBRLInput(values.target) ?? 0,
      deadline: values.deadline || null,
    });
    form.reset();
    onOpenChange(false);
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Nova meta">
      <form onSubmit={submit} className="flex flex-col gap-4 pb-2" data-testid="form-goal">
        <Field label="Nome" error={form.formState.errors.name?.message}>
          <Input placeholder="Reserva de emergência, viagem…" {...form.register("name")} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Valor alvo" error={form.formState.errors.target?.message}>
            <Input inputMode="decimal" placeholder="10.000,00" {...form.register("target")} />
          </Field>
          <Field label="Prazo (opcional)">
            <Input type="date" {...form.register("deadline")} />
          </Field>
        </div>
        {create.error ? <p className="text-sm text-expense">{create.error.message}</p> : null}
        <Button type="submit" disabled={create.isPending} className="h-12">
          {create.isPending ? "Criando…" : "Criar meta"}
        </Button>
      </form>
    </Sheet>
  );
}

function ContributeSheet({ goal, onClose }: { goal: Goal | null; onClose: () => void }) {
  const contribute = useContributeGoal();
  const [amount, setAmount] = useState("");
  const [direction, setDirection] = useState<"in" | "out">("in");

  const submit = async (): Promise<void> => {
    if (!goal) return;
    const cents = parseBRLInput(amount);
    if (!cents) return;
    await contribute.mutateAsync({
      id: goal.id,
      amountCents: direction === "in" ? cents : -cents,
    });
    setAmount("");
    onClose();
  };

  return (
    <Sheet
      open={Boolean(goal)}
      onOpenChange={(open) => !open && onClose()}
      title={goal ? `${goal.name} — ${formatBRL(goal.savedCents)} guardados` : ""}
    >
      <div className="flex flex-col gap-4 pb-2">
        <div className="flex gap-1 rounded-(--radius-control) bg-app-background/70 p-1">
          <button
            type="button"
            onClick={() => setDirection("in")}
            className={`h-10 flex-1 cursor-pointer rounded-(--radius-control) text-sm transition-colors ${direction === "in" ? "bg-app-soft font-medium text-income" : "text-app-muted"}`}
          >
            Guardar
          </button>
          <button
            type="button"
            onClick={() => setDirection("out")}
            className={`h-10 flex-1 cursor-pointer rounded-(--radius-control) text-sm transition-colors ${direction === "out" ? "bg-app-soft font-medium text-expense" : "text-app-muted"}`}
          >
            Resgatar
          </button>
        </div>
        <Field label="Valor">
          <Input
            inputMode="decimal"
            placeholder="200,00"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>
        {contribute.error ? (
          <p className="text-sm text-expense">{contribute.error.message}</p>
        ) : null}
        <Button
          disabled={contribute.isPending || parseBRLInput(amount) === null}
          onClick={() => void submit()}
          className="h-12"
        >
          {contribute.isPending
            ? "Salvando…"
            : direction === "in"
              ? "Guardar na meta"
              : "Resgatar da meta"}
        </Button>
      </div>
    </Sheet>
  );
}
