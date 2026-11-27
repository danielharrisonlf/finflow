import type { Member, TransactionSplit } from "@finflow/contracts";
import { Users } from "lucide-react";
import { Link } from "react-router-dom";
import { z } from "zod";
import { cn } from "../lib/cn";
import { formatBRL } from "../lib/format";
import { useMembers } from "../lib/queries";
import { useActiveSpace } from "../lib/use-active-space";
import { Field, Input, Select } from "./ui/field";
import { Segmented } from "./ui/segmented";

/**
 * "Quem pagou" + divisão entre membros (espaço Casa, v3 fase 2 §4). Valores do formulário são
 * strings; `buildSplitPayload` converte e valida a soma (100%) no envio.
 */
export const splitFieldSchema = z.object({
  paidByMemberId: z.string(),
  mode: z.enum(["EQUAL", "CUSTOM"]),
  shares: z.record(z.string(), z.string()),
});
export type SplitValue = z.infer<typeof splitFieldSchema>;

export const EMPTY_SPLIT: SplitValue = { paidByMemberId: "", mode: "EQUAL", shares: {} };

const MODES = [
  ["EQUAL", "Igual entre todos"],
  ["CUSTOM", "Personalizada"],
] as const;

const pctFmt = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 2 });

/** "33,33" | "50" → 33.33; null quando inválido. */
function parsePct(raw: string): number | null {
  const trimmed = raw.trim().replace("%", "").replace(",", ".");
  if (trimmed === "") return 0;
  const value = Number(trimmed);
  return Number.isFinite(value) && value >= 0 && value <= 100 ? value : null;
}

/** 100% repartido em partes iguais (2 casas); o resto vai para o primeiro. */
export function equalShares(members: Member[]): Record<string, string> {
  if (members.length === 0) return {};
  const base = Math.floor(10_000 / members.length) / 100;
  const rest = Math.round((100 - base * members.length) * 100) / 100;
  return Object.fromEntries(
    members.map((m, i) => [m.id, pctFmt.format(i === 0 ? base + rest : base)]),
  );
}

/** Valor inicial a partir de um lançamento existente. */
export function splitFromTransaction(
  t: { paidByMemberId?: string | null; splits?: TransactionSplit[] } | null,
): SplitValue {
  if (!t) return EMPTY_SPLIT;
  const splits = t.splits ?? [];
  return {
    paidByMemberId: t.paidByMemberId ?? "",
    mode: splits.length > 0 ? "CUSTOM" : "EQUAL",
    shares: Object.fromEntries(splits.map((s) => [s.memberId, pctFmt.format(s.sharePct)])),
  };
}

function sumShares(value: SplitValue, members: Member[]): number | null {
  let total = 0;
  for (const m of members) {
    const pct = parsePct(value.shares[m.id] ?? "");
    if (pct === null) return null;
    total += pct;
  }
  return Math.round(total * 100) / 100;
}

/**
 * Converte para o corpo da API. Divisão igual não manda `splits` na criação (o servidor divide
 * entre todos) e manda `[]` na edição para limpar uma divisão personalizada anterior.
 */
export function buildSplitPayload(
  value: SplitValue,
  members: Member[],
  editing: boolean,
):
  | { ok: true; paidByMemberId: string | null | undefined; splits: TransactionSplit[] | undefined }
  | { ok: false; error: string } {
  const paidByMemberId = value.paidByMemberId || (editing ? null : undefined);
  if (value.mode === "EQUAL") return { ok: true, paidByMemberId, splits: editing ? [] : undefined };
  const total = sumShares(value, members);
  if (total === null) return { ok: false, error: "percentual inválido — use 0 a 100" };
  if (Math.abs(total - 100) > 0.01) {
    return { ok: false, error: `a divisão soma ${pctFmt.format(total)}% — precisa dar 100%` };
  }
  const splits = members
    .map((m) => ({ memberId: m.id, sharePct: parsePct(value.shares[m.id] ?? "") ?? 0 }))
    .filter((s) => s.sharePct > 0);
  return { ok: true, paidByMemberId, splits };
}

/** Membros do espaço quando ele é Casa; vazio nos outros tipos (os campos nem aparecem). */
export function useHomeMembers(): { isHome: boolean; members: Member[] } {
  const { kind } = useActiveSpace();
  const isHome = kind === "HOME";
  const members = useMembers(isHome);
  return { isHome, members: isHome ? (members.data ?? []) : [] };
}

export function MemberSplitFields({
  value,
  onChange,
  members,
  amountCents,
  error,
}: {
  value: SplitValue;
  onChange: (value: SplitValue) => void;
  members: Member[];
  /** Valor digitado, para mostrar quanto fica para cada um. */
  amountCents: number | null;
  error?: string;
}) {
  if (members.length === 0) {
    return (
      <p className="flex items-start gap-2 rounded-(--radius-control) border border-app-border/80 px-3 py-2.5 text-xs text-app-muted">
        <Users size={14} aria-hidden className="mt-0.5 shrink-0" />
        <span>
          Para dividir despesas,{" "}
          <Link
            to="/divisao"
            className="font-medium text-income underline-offset-2 hover:underline"
          >
            cadastre as pessoas da casa
          </Link>
          .
        </span>
      </p>
    );
  }

  const total = sumShares(value, members);
  const setMode = (mode: SplitValue["mode"]): void =>
    onChange({
      ...value,
      mode,
      // primeira vez no modo personalizado: começa pela divisão igual
      shares:
        mode === "CUSTOM" && Object.keys(value.shares).length === 0
          ? equalShares(members)
          : value.shares,
    });

  return (
    <div
      className="flex flex-col gap-3 rounded-(--radius-control) border border-app-border/80 p-3"
      data-testid="member-split"
    >
      <Field label="Quem pagou">
        <Select
          value={value.paidByMemberId}
          onChange={(e) => onChange({ ...value, paidByMemberId: e.target.value })}
        >
          <option value="">Ninguém em especial</option>
          {members.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name}
            </option>
          ))}
        </Select>
      </Field>
      <Segmented label="Divisão" value={value.mode} onChange={setMode} options={MODES} />
      {value.mode === "EQUAL" ? (
        <p className="text-xs text-app-muted">
          Dividida igualmente entre {members.length} {members.length === 1 ? "pessoa" : "pessoas"}
          {amountCents ? ` · ${formatBRL(Math.round(amountCents / members.length))} cada` : ""}.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {members.map((m) => {
            const pct = parsePct(value.shares[m.id] ?? "");
            return (
              <li key={m.id} className="flex items-center gap-2.5">
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: m.color }}
                  aria-hidden
                />
                <span className="min-w-0 flex-1 truncate text-sm text-app-text">{m.name}</span>
                {amountCents && pct !== null ? (
                  <span className="tnum hidden text-xs text-app-muted min-[360px]:inline">
                    {formatBRL(Math.round((amountCents * pct) / 100))}
                  </span>
                ) : null}
                <div className="relative w-24 shrink-0">
                  <Input
                    inputMode="decimal"
                    aria-label={`Parte de ${m.name} (%)`}
                    value={value.shares[m.id] ?? ""}
                    onChange={(e) =>
                      onChange({ ...value, shares: { ...value.shares, [m.id]: e.target.value } })
                    }
                    className="pr-7 text-right"
                  />
                  <span className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-sm text-app-muted">
                    %
                  </span>
                </div>
              </li>
            );
          })}
          <li
            className={cn(
              "tnum text-right text-xs",
              total !== null && Math.abs(total - 100) <= 0.01 ? "text-app-muted" : "text-warn",
            )}
            aria-live="polite"
          >
            Total: {total === null ? "—" : `${pctFmt.format(total)}%`}
          </li>
        </ul>
      )}
      {error ? <p className="text-xs text-expense">{error}</p> : null}
    </div>
  );
}
