import type {
  ImportCommitResult,
  ImportKind,
  ImportMapping,
  ImportPreview,
} from "@finflow/contracts";
import {
  AlertTriangle,
  CircleCheck,
  Download,
  FileSpreadsheet,
  FileUp,
  RotateCcw,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "../components/ui/button";
import { Card, CardHeader } from "../components/ui/card";
import { formatCategoryLabel } from "../components/ui/category-icon";
import { Field, Select } from "../components/ui/field";
import { Money } from "../components/ui/money";
import { MonthSwitcher, usePeriodState, YearSwitcher } from "../components/ui/month-switcher";
import { Badge } from "../components/ui/segmented";
import { downloadUrl } from "../lib/api";
import { cn } from "../lib/cn";
import { formatDate } from "../lib/format";
import { useAccounts, useCategories, useCommitImport, useImportPreview } from "../lib/queries";

export function SpreadsheetsPage() {
  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold tracking-tight">Planilhas</h1>
        <p className="text-sm text-app-muted">
          Traga seu histórico de planilhas e extratos, ou leve seus dados para o Excel
        </p>
      </header>
      <ImportWizard />
      <ExportSection />
    </div>
  );
}

// ───────────────────────── importação ─────────────────────────

const FORMAT_LABEL: Record<ImportPreview["format"], string> = {
  DAILY_SHEET: "Planilha diária (Entrada · Saída · Diário · Saldo)",
  TABLE: "Tabela do Excel",
  CSV: "CSV",
  OFX: "Extrato OFX",
};

const MAPPING_FIELDS: { key: keyof ImportMapping; label: string }[] = [
  { key: "date", label: "Data" },
  { key: "description", label: "Descrição" },
  { key: "amount", label: "Valor" },
  { key: "debit", label: "Débito" },
  { key: "credit", label: "Crédito" },
  { key: "type", label: "Tipo" },
  { key: "category", label: "Categoria" },
  { key: "account", label: "Conta" },
];

interface RowState {
  include: boolean;
  kind: ImportKind;
  categoryId: string;
}

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;
const PAGE = 200;

function ImportWizard() {
  const accounts = useAccounts();
  const categories = useCategories();
  const preview = useImportPreview();
  const commit = useCommitImport();

  const [file, setFile] = useState<File | null>(null);
  const [accountId, setAccountId] = useState("");
  const [sheet, setSheet] = useState<string | null>(null);
  const [mapping, setMapping] = useState<ImportMapping | null>(null);
  const [rows, setRows] = useState<Record<number, RowState>>({});
  const [visible, setVisible] = useState(PAGE);
  const [dragging, setDragging] = useState(false);
  const [result, setResult] = useState<ImportCommitResult | null>(null);

  const data = preview.data;

  useEffect(() => {
    if (!accountId && accounts.data?.[0]) setAccountId(accounts.data[0].id);
  }, [accounts.data, accountId]);

  // novo preview → estado editável das linhas (duplicadas e com erro já desmarcadas)
  useEffect(() => {
    if (!data) return;
    setSheet(data.sheet);
    setMapping(data.mapping);
    setVisible(PAGE);
    setRows(
      Object.fromEntries(
        data.rows.map((r) => [
          r.index,
          {
            include: !r.duplicate && !r.error,
            kind: r.kind,
            categoryId: r.suggestedCategoryId ?? "",
          },
        ]),
      ),
    );
  }, [data]);

  const runPreview = (
    next: { file?: File; sheet?: string | null; mapping?: ImportMapping; accountId?: string } = {},
  ): void => {
    const f = next.file ?? file;
    if (!f) return;
    setResult(null);
    commit.reset();
    preview.mutate({
      file: f,
      sheet: next.sheet === undefined ? sheet : next.sheet,
      mapping: next.mapping,
      accountId: next.accountId ?? (accountId || undefined),
    });
  };

  const pickFile = (f: File | undefined): void => {
    if (!f) return;
    setFile(f);
    setSheet(null);
    runPreview({ file: f, sheet: null });
  };

  const reset = (): void => {
    setFile(null);
    setSheet(null);
    setMapping(null);
    setRows({});
    setResult(null);
    preview.reset();
    commit.reset();
  };

  const selected = useMemo(
    () => (data?.rows ?? []).filter((r) => !r.error && rows[r.index]?.include),
    [data, rows],
  );
  const selectedTotal = selected.reduce(
    (acc, r) =>
      acc + (rows[r.index]?.kind === "INCOME" ? Math.abs(r.amountCents) : -Math.abs(r.amountCents)),
    0,
  );

  const patchRow = (index: number, patch: Partial<RowState>): void =>
    setRows((prev) => ({ ...prev, [index]: { ...(prev[index] as RowState), ...patch } }));

  const onCommit = async (): Promise<void> => {
    if (!accountId || selected.length === 0) return;
    const res = await commit.mutateAsync({
      accountId,
      // a escolha de quem importar já foi feita linha a linha (duplicadas vêm desmarcadas)
      skipDuplicates: false,
      rows: selected.map((r) => {
        const st = rows[r.index] as RowState;
        return {
          date: r.date,
          description: r.description,
          amountCents: Math.abs(r.amountCents),
          kind: st.kind,
          categoryId: st.categoryId || null,
        };
      }),
    });
    setResult(res);
  };

  if (result) {
    return (
      <Card
        className="flex flex-col items-start gap-3 p-6"
        data-testid="import-result"
        role="status"
      >
        <CircleCheck size={28} className="text-income" aria-hidden />
        <div>
          <p className="text-base font-semibold text-app-text">
            {result.created}{" "}
            {result.created === 1 ? "lançamento importado" : "lançamentos importados"}
          </p>
          <p className="text-sm text-app-muted">
            {result.skipped > 0
              ? `${result.skipped} ${result.skipped === 1 ? "linha ignorada" : "linhas ignoradas"} por já existir no extrato.`
              : "Nenhuma linha ignorada."}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link
            to="/extrato"
            className="inline-flex h-11 items-center rounded-(--radius-control) bg-emerald-500 px-4 text-sm font-semibold text-emerald-950 hover:bg-emerald-400"
          >
            Ver no extrato
          </Link>
          <Button variant="outline" onClick={reset}>
            Importar outro arquivo
          </Button>
        </div>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader
        title="Importar"
        action={
          file ? (
            <Button variant="ghost" size="sm" onClick={reset}>
              <RotateCcw size={14} /> Trocar arquivo
            </Button>
          ) : null
        }
      />
      <div className="flex flex-col gap-4 px-5 pt-2 pb-5">
        {!data ? (
          <label
            className={cn(
              "flex min-h-44 cursor-pointer flex-col items-center justify-center gap-2 rounded-(--radius-card) border-2 border-dashed px-6 py-8 text-center transition-colors focus-within:border-emerald-500",
              dragging
                ? "border-emerald-500 bg-emerald-500/8"
                : "border-app-border hover:border-app-muted",
            )}
            data-testid="import-dropzone"
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              pickFile(e.dataTransfer.files[0]);
            }}
          >
            <FileUp size={28} className="text-app-muted" aria-hidden />
            <span className="text-sm font-medium text-app-text">
              {preview.isPending
                ? "Lendo o arquivo…"
                : "Arraste o arquivo aqui ou toque para escolher"}
            </span>
            <span className="max-w-md text-xs text-app-muted">
              Aceita Excel (.xlsx) na planilha diária (Entrada · Saída · Diário · Saldo) ou em
              tabela, CSV de banco e extrato OFX. Até 10 MB. Nada é gravado antes de você confirmar.
            </span>
            <input
              type="file"
              accept=".xlsx,.csv,.ofx,text/csv"
              className="sr-only"
              data-testid="import-file"
              onChange={(e) => {
                pickFile(e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </label>
        ) : null}

        {preview.error ? (
          <p className="text-sm text-expense" role="alert">
            {preview.error.message}
          </p>
        ) : null}

        {data ? (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone="income" className="h-7 px-2.5 text-xs" data-testid="import-format">
                <FileSpreadsheet size={13} aria-hidden /> {FORMAT_LABEL[data.format]}
              </Badge>
              <span className="truncate text-sm text-app-muted">{file?.name}</span>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Importar para a conta">
                <Select
                  value={accountId}
                  onChange={(e) => {
                    setAccountId(e.target.value);
                    runPreview({ accountId: e.target.value, mapping: mapping ?? undefined });
                  }}
                  data-testid="import-account"
                >
                  {accounts.data?.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </Select>
              </Field>
              {data.sheets.length > 1 ? (
                <Field label="Aba da planilha">
                  <Select
                    value={sheet ?? ""}
                    onChange={(e) => {
                      setSheet(e.target.value);
                      runPreview({ sheet: e.target.value });
                    }}
                  >
                    {data.sheets.map((s) => (
                      <option key={s} value={s}>
                        {s}
                      </option>
                    ))}
                  </Select>
                </Field>
              ) : null}
            </div>

            {(data.format === "TABLE" || data.format === "CSV") && mapping ? (
              <fieldset className="rounded-(--radius-control) border border-app-border p-3">
                <legend className="px-1 text-sm text-app-muted">Colunas do arquivo</legend>
                <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
                  {MAPPING_FIELDS.map(({ key, label }) => (
                    <Field key={key} label={label}>
                      <Select
                        value={mapping[key] === null ? "" : String(mapping[key])}
                        onChange={(e) =>
                          setMapping({
                            ...mapping,
                            [key]: e.target.value === "" ? null : Number(e.target.value),
                          })
                        }
                      >
                        <option value="">—</option>
                        {data.columns.map((col, i) => (
                          <option key={`${col}-${String(i)}`} value={i}>
                            {col || `Coluna ${i + 1}`}
                          </option>
                        ))}
                      </Select>
                    </Field>
                  ))}
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-3"
                  disabled={preview.isPending}
                  onClick={() => runPreview({ mapping })}
                >
                  {preview.isPending ? "Relendo…" : "Reler com estas colunas"}
                </Button>
              </fieldset>
            ) : null}

            {data.warnings.length > 0 ? (
              <ul
                className="space-y-1 rounded-(--radius-control) border border-amber-400/40 bg-amber-400/8 p-3"
                data-testid="import-warnings"
              >
                {data.warnings.map((w) => (
                  <li key={w} className="flex gap-2 text-sm text-app-text">
                    <AlertTriangle size={15} className="mt-0.5 shrink-0 text-warn" aria-hidden />
                    {w}
                  </li>
                ))}
              </ul>
            ) : null}

            <PreviewTable
              data={data}
              rows={rows}
              visible={visible}
              categories={categories.data ?? []}
              onPatch={patchRow}
              onToggleAll={(include) =>
                setRows((prev) =>
                  Object.fromEntries(
                    data.rows.map((r) => [
                      r.index,
                      { ...(prev[r.index] as RowState), include: include && !r.error },
                    ]),
                  ),
                )
              }
            />
            {data.rows.length > visible ? (
              <Button variant="ghost" onClick={() => setVisible((v) => v + PAGE)}>
                Mostrar mais {Math.min(PAGE, data.rows.length - visible)} linhas (de{" "}
                {data.rows.length})
              </Button>
            ) : null}

            <div className="flex flex-col gap-3 border-t border-app-border/60 pt-4 sm:flex-row sm:items-center sm:justify-between">
              <p className="tnum text-sm text-app-muted" aria-live="polite">
                <span className="font-semibold text-app-text">{selected.length}</span> de{" "}
                {data.rows.length} linhas selecionadas · saldo{" "}
                <Money cents={selectedTotal} signed className="font-medium" />
              </p>
              <Button
                className="h-12"
                disabled={commit.isPending || selected.length === 0 || !accountId}
                onClick={() => void onCommit()}
              >
                {commit.isPending
                  ? "Importando…"
                  : `Importar ${selected.length} ${selected.length === 1 ? "lançamento" : "lançamentos"}`}
              </Button>
            </div>
            {commit.error ? (
              <p className="text-sm text-expense" role="alert">
                {commit.error.message}
              </p>
            ) : null}
          </>
        ) : null}
      </div>
    </Card>
  );
}

function PreviewTable({
  data,
  rows,
  visible,
  categories,
  onPatch,
  onToggleAll,
}: {
  data: ImportPreview;
  rows: Record<number, RowState>;
  visible: number;
  categories: { id: string; name: string; kind: string; parentId: string | null }[];
  onPatch: (index: number, patch: Partial<RowState>) => void;
  onToggleAll: (include: boolean) => void;
}) {
  const valid = data.rows.filter((r) => !r.error);
  const allOn = valid.length > 0 && valid.every((r) => rows[r.index]?.include);
  const byKind = {
    INCOME: categories.filter((c) => c.kind === "INCOME"),
    EXPENSE: categories.filter((c) => c.kind === "EXPENSE"),
  };

  if (data.rows.length === 0) {
    return <p className="text-sm text-app-muted">Nenhuma linha encontrada neste arquivo.</p>;
  }

  return (
    <div className="-mx-5 overflow-x-auto" data-testid="import-preview">
      <table className="w-full min-w-[720px] text-sm">
        <thead>
          <tr className="text-left text-xs text-app-muted">
            <th scope="col" className="w-12 py-2 pl-5">
              <input
                type="checkbox"
                className="h-4 w-4 accent-emerald-500"
                checked={allOn}
                onChange={(e) => onToggleAll(e.target.checked)}
                aria-label="Selecionar todas as linhas"
              />
            </th>
            <th scope="col" className="px-2 py-2 font-medium">
              Data
            </th>
            <th scope="col" className="px-2 py-2 font-medium">
              Descrição
            </th>
            <th scope="col" className="px-2 py-2 text-right font-medium">
              Valor
            </th>
            <th scope="col" className="px-2 py-2 font-medium">
              Tipo
            </th>
            <th scope="col" className="px-2 py-2 font-medium">
              Categoria
            </th>
            <th scope="col" className="py-2 pr-5 pl-2 font-medium">
              <span className="sr-only">Avisos</span>
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-app-border/60">
          {data.rows.slice(0, visible).map((r) => {
            const st = rows[r.index];
            if (!st) return null;
            const disabled = Boolean(r.error);
            return (
              <tr
                key={r.index}
                className={cn(!st.include && "opacity-60")}
                data-testid="import-row"
                data-duplicate={r.duplicate || undefined}
              >
                <td className="py-1.5 pl-5">
                  <input
                    type="checkbox"
                    className="h-4 w-4 accent-emerald-500"
                    checked={st.include}
                    disabled={disabled}
                    onChange={(e) => onPatch(r.index, { include: e.target.checked })}
                    aria-label={`Importar ${r.description || `linha ${r.index + 1}`}`}
                  />
                </td>
                <td className="tnum px-2 py-1.5 whitespace-nowrap text-app-text">
                  {ISO_DATE.test(r.date) ? formatDate(r.date) : r.date || "—"}
                </td>
                <td className="max-w-56 truncate px-2 py-1.5 text-app-text" title={r.description}>
                  {r.description || "—"}
                </td>
                <td className="px-2 py-1.5 text-right whitespace-nowrap">
                  <Money
                    cents={
                      st.kind === "INCOME" ? Math.abs(r.amountCents) : -Math.abs(r.amountCents)
                    }
                    signed
                  />
                </td>
                <td className="px-2 py-1.5">
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() =>
                      onPatch(r.index, {
                        kind: st.kind === "INCOME" ? "EXPENSE" : "INCOME",
                        categoryId: "",
                      })
                    }
                    aria-label={`Tipo: ${st.kind === "INCOME" ? "entrada" : "saída"}. Alternar`}
                    className={cn(
                      "h-9 min-w-20 cursor-pointer rounded-full border px-3 text-xs font-medium disabled:cursor-not-allowed",
                      st.kind === "INCOME"
                        ? "border-emerald-500/40 text-income"
                        : "border-rose-500/40 text-expense",
                    )}
                  >
                    {st.kind === "INCOME" ? "Entrada" : "Saída"}
                  </button>
                </td>
                <td className="px-2 py-1.5">
                  <Select
                    className="h-9 min-w-40 text-sm"
                    value={st.categoryId}
                    disabled={disabled}
                    onChange={(e) => onPatch(r.index, { categoryId: e.target.value })}
                    aria-label={`Categoria de ${r.description || `linha ${r.index + 1}`}`}
                  >
                    <option value="">Sem categoria</option>
                    {byKind[st.kind].map((c) => (
                      <option key={c.id} value={c.id}>
                        {formatCategoryLabel(c, categories)}
                      </option>
                    ))}
                  </Select>
                </td>
                <td className="py-1.5 pr-5 pl-2">
                  <div className="flex gap-1">
                    {r.duplicate ? <Badge tone="warn">Duplicado</Badge> : null}
                    {r.error ? (
                      <Badge tone="expense" className="max-w-48">
                        <span className="truncate" title={r.error}>
                          {r.error}
                        </span>
                      </Badge>
                    ) : null}
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ───────────────────────── exportação ─────────────────────────

const downloadCls =
  "inline-flex h-11 items-center justify-center gap-2 rounded-(--radius-control) border border-app-border px-4 text-sm font-medium text-app-text transition-colors hover:border-app-muted hover:bg-app-surface";

function ExportSection() {
  const [year, setYear] = useState(() => new Date().getFullYear());
  const [period, shiftPeriod] = usePeriodState();

  return (
    <section aria-labelledby="export-title" className="space-y-3">
      <h2 id="export-title" className="pt-2 text-base font-semibold text-app-text">
        Exportar
      </h2>
      <div className="grid gap-3 lg:grid-cols-3">
        <Card className="flex flex-col gap-3 p-5" data-testid="export-year">
          <div>
            <h3 className="text-sm font-semibold text-app-text">Planilha anual (.xlsx)</h3>
            <p className="mt-1 text-sm text-app-muted">
              Seu ano inteiro no formato da planilha diária, com fórmulas que continuam funcionando
              no Excel.
            </p>
          </div>
          <ul className="space-y-1 text-xs text-app-muted">
            <li>
              <span className="font-medium text-app-text">Aba do ano:</span> 12 blocos com Data ·
              Entrada · Saída · Diário · Saldo, saldo encadeado entre meses e cores por faixa.
            </li>
            <li>
              <span className="font-medium text-app-text">Economia:</span> quanto entrou e quanto
              você guardou em cada mês, com a porcentagem.
            </li>
            <li>
              <span className="font-medium text-app-text">Lançamentos:</span> todas as transações
              com filtro.
            </li>
            <li>
              <span className="font-medium text-app-text">Categorias:</span> gasto por categoria em
              cada mês.
            </li>
          </ul>
          <div className="mt-auto flex flex-wrap items-center justify-between gap-2">
            <YearSwitcher year={year} onChange={setYear} />
            <a
              href={downloadUrl(
                "/spreadsheet/export.xlsx",
                new URLSearchParams({ year: String(year) }),
              )}
              download
              className={downloadCls}
            >
              <Download size={15} aria-hidden /> Baixar {year}
            </a>
          </div>
        </Card>

        <Card className="flex flex-col gap-3 p-5" data-testid="export-template">
          <div>
            <h3 className="text-sm font-semibold text-app-text">Modelo de importação</h3>
            <p className="mt-1 text-sm text-app-muted">
              Uma planilha pronta para preencher e importar de volta.
            </p>
          </div>
          <ul className="space-y-1 text-xs text-app-muted">
            <li>
              <span className="font-medium text-app-text">Lançamentos:</span> Data · Descrição ·
              Valor · Tipo · Categoria · Conta, com 3 exemplos.
            </li>
            <li>
              <span className="font-medium text-app-text">Como usar:</span> o passo a passo para
              preencher.
            </li>
          </ul>
          <div className="mt-auto">
            <a href={downloadUrl("/spreadsheet/template.xlsx")} download className={downloadCls}>
              <Download size={15} aria-hidden /> Baixar modelo
            </a>
          </div>
        </Card>

        <Card className="flex flex-col gap-3 p-5" data-testid="export-transactions">
          <div>
            <h3 className="text-sm font-semibold text-app-text">Lançamentos do mês (.xlsx)</h3>
            <p className="mt-1 text-sm text-app-muted">
              As transações de um mês numa aba só, com categoria, tipo de gasto, conta e status. Com
              filtros, use a exportação do Extrato.
            </p>
          </div>
          <div className="mt-auto flex flex-wrap items-center justify-between gap-2">
            <MonthSwitcher period={period} onShift={shiftPeriod} />
            <a
              href={downloadUrl(
                "/transactions/export",
                new URLSearchParams({
                  format: "xlsx",
                  month: String(period.month),
                  year: String(period.year),
                }),
              )}
              download
              className={downloadCls}
            >
              <Download size={15} aria-hidden /> Baixar
            </a>
          </div>
        </Card>
      </div>
    </section>
  );
}
