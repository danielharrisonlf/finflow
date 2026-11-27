import type { Space, SpaceOverviewItem, SpaceTotals } from "@finflow/contracts";
import { LayoutGrid, Plus } from "lucide-react";
import { useState } from "react";
import { SpaceBadge, SpaceSheet, useSwitchSpace } from "../components/space-switcher";
import { Button } from "../components/ui/button";
import { Card, CardHeader } from "../components/ui/card";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Money } from "../components/ui/money";
import { Badge } from "../components/ui/segmented";
import { cn } from "../lib/cn";
import { useSpacesOverview } from "../lib/queries";
import { SPACE_KIND_LABEL, useActiveSpace } from "../lib/use-active-space";

/** Visão unificada (v3 fase 2 §3 e §6): um card por espaço e a soma de todos. */
export function SpacesPage() {
  const overview = useSpacesOverview();
  const { slug, spaces } = useActiveSpace();
  const switchTo = useSwitchSpace();
  const [creating, setCreating] = useState(false);
  const items = overview.data?.spaces ?? [];
  const totals = overview.data?.totals;

  const spaceOf = (item: SpaceOverviewItem): Space =>
    spaces.find((s) => s.slug === item.slug) ?? {
      slug: item.slug,
      name: item.name,
      kind: item.kind,
      color: "#34d399",
    };

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Visão unificada</h1>
          <p className="text-sm text-app-muted">
            Todos os seus espaços lado a lado, e a soma deles
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => setCreating(true)}>
          <Plus size={15} /> Novo espaço
        </Button>
      </header>

      {overview.isLoading ? (
        <Card>
          <LoadingRows rows={4} />
        </Card>
      ) : overview.isError ? (
        <Card>
          <Empty
            icon={LayoutGrid}
            title="Não foi possível carregar a visão unificada"
            hint={overview.error.message}
          />
        </Card>
      ) : (
        <>
          {totals ? (
            <Card data-testid="spaces-totals">
              <CardHeader title="Todos os espaços" />
              <TotalsGrid totals={totals} className="px-5 pt-2 pb-5" />
            </Card>
          ) : null}

          {items.length === 0 ? (
            <Card>
              <Empty
                icon={LayoutGrid}
                title="Só o espaço pessoal por enquanto"
                hint="Crie um espaço para a casa (despesas divididas) ou para a empresa (MRR, DRE e runway)."
                action={<Button onClick={() => setCreating(true)}>Criar espaço</Button>}
              />
            </Card>
          ) : (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3" data-testid="spaces-grid">
              {items.map((item) => {
                const space = spaceOf(item);
                const active = item.slug === slug;
                return (
                  <Card
                    key={item.slug}
                    className={cn(
                      "flex min-w-0 flex-col gap-3 p-4",
                      active && "border-emerald-500/40",
                    )}
                    data-testid={`space-card-${item.slug}`}
                  >
                    <div className="flex items-center gap-3">
                      <SpaceBadge space={space} size={36} />
                      <div className="min-w-0 flex-1">
                        <h2 className="truncate text-sm font-semibold text-app-text">
                          {item.name}
                        </h2>
                        <p className="text-xs text-app-muted">
                          {SPACE_KIND_LABEL[item.kind] ?? item.kind}
                        </p>
                      </div>
                      {active ? <Badge tone="income">Ativo</Badge> : null}
                    </div>
                    <TotalsGrid totals={item} />
                    {active ? null : (
                      <Button
                        variant="outline"
                        size="md"
                        onClick={() => switchTo(space)}
                        aria-label={`Abrir o espaço ${item.name}`}
                      >
                        Abrir espaço
                      </Button>
                    )}
                  </Card>
                );
              })}
            </div>
          )}
        </>
      )}

      <SpaceSheet open={creating} onOpenChange={setCreating} startCreating />
    </div>
  );
}

function TotalsGrid({ totals, className }: { totals: SpaceTotals; className?: string }) {
  return (
    <dl className={cn("grid grid-cols-2 gap-x-4 gap-y-2.5 text-sm", className)}>
      <Stat label="Saldo em contas">
        <Money cents={totals.balanceCents} neutral />
      </Stat>
      <Stat label="Patrimônio">
        <Money cents={totals.netWorthCents} neutral />
      </Stat>
      <Stat label="Entradas do mês">
        <Money cents={totals.monthIncomeCents} />
      </Stat>
      <Stat label="Saídas do mês">
        <Money cents={-Math.abs(totals.monthOutCents)} />
      </Stat>
      <Stat label="Resultado do mês" wide>
        <Money cents={totals.monthResultCents} signed className="font-semibold" />
      </Stat>
    </dl>
  );
}

function Stat({
  label,
  children,
  wide,
}: {
  label: string;
  children: React.ReactNode;
  wide?: boolean;
}) {
  return (
    <div className={cn("min-w-0", wide && "col-span-2")}>
      <dt className="text-xs text-app-muted">{label}</dt>
      <dd className="truncate">{children}</dd>
    </div>
  );
}
