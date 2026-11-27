import type { YearOverview } from "@finflow/contracts";
import { CalendarRange } from "lucide-react";
import { useState } from "react";
import { Card } from "../components/ui/card";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Money } from "../components/ui/money";
import { YearSwitcher } from "../components/ui/month-switcher";
import { formatBRL, formatMonthShort, formatPercent, formatSignedBRL } from "../lib/format";
import { useYearOverview } from "../lib/queries";

export function YearPage() {
  const [year, setYear] = useState(() => new Date().getFullYear());
  const overview = useYearOverview(year);
  const data = overview.data;

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Ano</h1>
          <p className="text-sm text-app-muted">
            O ano mês a mês, no formato da planilha diária (Entrada · Saída · Diário · Saldo)
          </p>
        </div>
        <YearSwitcher year={year} onChange={setYear} />
      </header>

      {overview.isLoading ? (
        <Card>
          <LoadingRows rows={6} />
        </Card>
      ) : !data ? (
        <Card>
          <Empty
            icon={CalendarRange}
            title="Não foi possível carregar o ano"
            hint={overview.error?.message ?? "Tente novamente em instantes."}
          />
        </Card>
      ) : (
        <YearContent data={data} />
      )}
    </div>
  );
}

function SummaryTile({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <Card className="px-4 py-3.5">
      <p className="text-xs text-app-muted">{label}</p>
      <p className="tnum mt-1 text-lg font-semibold text-app-text">{children}</p>
    </Card>
  );
}

function YearContent({ data }: { data: YearOverview }) {
  const t = data.totals;
  return (
    <>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <SummaryTile label="Entradas no ano">
          <span className="text-income">{formatBRL(t.incomeCents)}</span>
        </SummaryTile>
        <SummaryTile label="Saída total (Saídas + Diário)">
          {formatBRL(t.totalOutCents)}
        </SummaryTile>
        <SummaryTile label="Performance">
          <span className={t.performanceCents < 0 ? "text-expense" : "text-income"}>
            {formatSignedBRL(t.performanceCents)}
          </span>
        </SummaryTile>
        <SummaryTile label="Economia · taxa">
          {formatBRL(t.savedCents)}{" "}
          <span className="text-sm font-normal text-app-muted">
            {t.savingsRatePct === null ? "" : formatPercent(t.savingsRatePct)}
          </span>
        </SummaryTile>
      </div>

      <Card data-testid="year-table">
        {/* rolagem horizontal só dentro da tabela, nunca na página */}
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-sm">
            <caption className="sr-only">Resumo mensal de {data.year}</caption>
            <thead>
              <tr className="text-left text-xs text-app-muted">
                <th scope="col" className="sticky left-0 bg-app-surface px-5 py-3 font-medium">
                  Mês
                </th>
                <th scope="col" className="px-3 py-3 text-right font-medium">
                  Entradas
                </th>
                <th scope="col" className="px-3 py-3 text-right font-medium">
                  Saídas
                </th>
                <th scope="col" className="px-3 py-3 text-right font-medium">
                  Diário
                </th>
                <th scope="col" className="px-3 py-3 text-right font-medium">
                  Performance
                </th>
                <th scope="col" className="px-3 py-3 text-right font-medium">
                  Economia
                </th>
                <th scope="col" className="px-3 py-3 text-right font-medium">
                  %
                </th>
                <th scope="col" className="px-5 py-3 text-right font-medium">
                  Saldo final
                </th>
              </tr>
            </thead>
            <tbody className="tnum divide-y divide-app-border/60">
              {data.months.map((m) => (
                <tr key={m.month}>
                  <th
                    scope="row"
                    className="sticky left-0 bg-app-surface px-5 py-2.5 text-left font-medium text-app-text"
                  >
                    {formatMonthShort(m.month)}
                  </th>
                  <td className="px-3 py-2.5 text-right text-income">{formatBRL(m.incomeCents)}</td>
                  <td className="px-3 py-2.5 text-right text-app-text">
                    {formatBRL(m.fixedCents)}
                  </td>
                  <td className="px-3 py-2.5 text-right text-app-text">
                    {formatBRL(m.variableCents)}
                  </td>
                  <td className="px-3 py-2.5 text-right font-medium">
                    <Money cents={m.performanceCents} signed />
                  </td>
                  <td className="px-3 py-2.5 text-right text-app-text">
                    {formatBRL(m.savedCents)}
                  </td>
                  <td className="px-3 py-2.5 text-right text-app-muted">
                    {m.savingsRatePct === null ? "—" : formatPercent(m.savingsRatePct)}
                  </td>
                  <td className="px-5 py-2.5 text-right font-semibold">
                    <Money cents={m.endBalanceCents} />
                  </td>
                </tr>
              ))}
            </tbody>
            <tfoot className="tnum border-t-2 border-app-border">
              <tr className="font-semibold">
                <th
                  scope="row"
                  className="sticky left-0 bg-app-surface px-5 py-3 text-left text-app-text"
                >
                  Total
                </th>
                <td className="px-3 py-3 text-right text-income">{formatBRL(t.incomeCents)}</td>
                <td className="px-3 py-3 text-right text-app-text">{formatBRL(t.fixedCents)}</td>
                <td className="px-3 py-3 text-right text-app-text">{formatBRL(t.variableCents)}</td>
                <td className="px-3 py-3 text-right">
                  <Money cents={t.performanceCents} signed />
                </td>
                <td className="px-3 py-3 text-right text-app-text">{formatBRL(t.savedCents)}</td>
                <td className="px-3 py-3 text-right text-app-muted">
                  {t.savingsRatePct === null ? "—" : formatPercent(t.savingsRatePct)}
                </td>
                <td className="px-5 py-3 text-right text-app-muted">—</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </Card>
      <p className="text-xs text-app-muted">
        Saídas = custo de vida fixo (categorias Fixo e Cortável) · Diário = gastos variáveis ·
        Performance = Entradas − (Saídas + Diário) · Economia = o que foi para poupança,
        investimentos e metas.
      </p>
    </>
  );
}
