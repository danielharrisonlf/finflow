const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

export function formatBRL(cents: number): string {
  return brl.format(cents / 100);
}

/** Formata com sinal explícito para linhas de extrato: +R$ 12,00 / −R$ 8,50. */
export function formatSignedBRL(cents: number): string {
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
