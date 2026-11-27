import { isPrivacyOn, MASKED_BRL } from "./privacy";

const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

/** Valor em reais; no modo privacidade devolve a máscara (ver lib/privacy). */
export function formatBRL(cents: number): string {
  if (isPrivacyOn()) return MASKED_BRL;
  return brl.format(cents / 100);
}

/** Igual a `formatBRL`, mas ignora o modo privacidade (valores digitados pelo usuário). */
export function formatBRLPlain(cents: number): string {
  return brl.format(cents / 100);
}

/** Formata com sinal explícito para linhas de extrato: +R$ 12,00 / −R$ 8,50. */
export function formatSignedBRL(cents: number): string {
  if (isPrivacyOn()) return MASKED_BRL;
  const value = brl.format(Math.abs(cents) / 100);
  return cents >= 0 ? `+${value}` : `−${value}`;
}

const shortDate = new Intl.DateTimeFormat("pt-BR", {
  day: "2-digit",
  month: "short",
  timeZone: "UTC",
});

export function formatDate(iso: string): string {
  return shortDate
    .format(new Date(`${iso}T00:00:00Z`))
    .replace(". de", "")
    .replace(".", "");
}

export function formatMonthYear(month: number, year: number): string {
  const name = new Intl.DateTimeFormat("pt-BR", { month: "long", timeZone: "UTC" }).format(
    new Date(Date.UTC(year, month - 1, 1)),
  );
  return `${name[0]?.toUpperCase()}${name.slice(1)} ${year}`;
}

export function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

/** "1.234,56" | "45,9" | "50" → centavos; null quando inválido. */
export function parseBRLInput(raw: string): number | null {
  const cleaned = raw.trim().replace(/^R\$\s*/i, "");
  if (!cleaned || !/^[\d.,]+$/.test(cleaned)) return null;
  const normalized = cleaned.includes(",") ? cleaned.replace(/\./g, "").replace(",", ".") : cleaned;
  const value = Number(normalized);
  if (!Number.isFinite(value) || value <= 0) return null;
  return Math.round(value * 100);
}

/** "-500,00" | "0" | "2.000" → centavos (aceita zero e negativos); null quando inválido. */
export function parseSignedBRLInput(raw: string): number | null {
  const trimmed = raw.trim().replace(/^R\$\s*/i, "");
  const negative = /^[-−]/.test(trimmed);
  const cleaned = trimmed.replace(/^[-−]\s*/, "");
  if (!cleaned || !/^[\d.,]+$/.test(cleaned)) return null;
  const normalized = cleaned.includes(",") ? cleaned.replace(/\./g, "").replace(",", ".") : cleaned;
  const value = Number(normalized);
  if (!Number.isFinite(value)) return null;
  const cents = Math.round(value * 100);
  return negative ? -cents : cents;
}

/** Centavos → texto editável "1234,56" (sem milhar, com sinal quando negativo). */
export function centsToInput(cents: number): string {
  const text = (Math.abs(cents) / 100).toFixed(2).replace(".", ",");
  return cents < 0 ? `-${text}` : text;
}

const pct = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 });

/** 12.345 → "12,3%". */
export function formatPercent(value: number): string {
  return `${pct.format(value)}%`;
}

const longDate = new Intl.DateTimeFormat("pt-BR", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

/** "2026-12-05" → "5 de dezembro de 2026". */
export function formatLongDate(iso: string): string {
  return longDate.format(new Date(`${iso}T00:00:00Z`));
}

const weekdayDate = new Intl.DateTimeFormat("pt-BR", {
  weekday: "short",
  day: "2-digit",
  month: "short",
  timeZone: "UTC",
});

/** "2026-12-05" → "sáb., 05 de dez." */
export function formatWeekdayDate(iso: string): string {
  return weekdayDate.format(new Date(`${iso}T00:00:00Z`));
}

/** 3 → "Mar". */
export function formatMonthShort(month: number): string {
  const name = new Intl.DateTimeFormat("pt-BR", { month: "short", timeZone: "UTC" })
    .format(new Date(Date.UTC(2000, month - 1, 1)))
    .replace(".", "");
  return `${name[0]?.toUpperCase()}${name.slice(1)}`;
}

/** Valor compacto para eixos: 125000 → "1,3k", -50000 → "-500". */
export function formatAxisBRL(cents: number): string {
  if (isPrivacyOn()) return "•••";
  const reais = cents / 100;
  const abs = Math.abs(reais);
  if (abs >= 1000) return `${pct.format(reais / 1000)}k`;
  return String(Math.round(reais));
}
