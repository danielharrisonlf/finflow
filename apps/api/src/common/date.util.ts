/** Datas de domínio são `YYYY-MM-DD` em UTC — nunca dependem do fuso do servidor. */

export function toDateString(date: Date): string {
  return date.toISOString().slice(0, 10);
}

export function fromDateString(value: string): Date {
  return new Date(`${value}T00:00:00.000Z`);
}

export function todayString(): string {
  return toDateString(new Date());
}

export function addDays(value: string, days: number): string {
  const d = fromDateString(value);
  d.setUTCDate(d.getUTCDate() + days);
  return toDateString(d);
}

/** Primeiro dia do mês como string ISO. */
export function monthStart(year: number, month: number): string {
  return `${year}-${String(month).padStart(2, "0")}-01`;
}

/** Intervalo [início, fim) do mês para filtros de data. */
export function monthRange(year: number, month: number): { gte: Date; lt: Date } {
  const next = month === 12 ? { year: year + 1, month: 1 } : { year, month: month + 1 };
  return {
    gte: fromDateString(monthStart(year, month)),
    lt: fromDateString(monthStart(next.year, next.month)),
  };
}

/** Soma meses mantendo o dia (clampado ao último dia do mês de destino). */
export function addMonthsToDateString(value: string, delta: number): string {
  const d = fromDateString(value);
  const target = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + delta, 1));
  const lastDay = new Date(
    Date.UTC(target.getUTCFullYear(), target.getUTCMonth() + 1, 0),
  ).getUTCDate();
  target.setUTCDate(Math.min(d.getUTCDate(), lastDay));
  return toDateString(target);
}

export function currentMonthYear(): { month: number; year: number } {
  const now = new Date();
  return { month: now.getUTCMonth() + 1, year: now.getUTCFullYear() };
}

/** Meses inteiros (arredondando para cima) entre hoje e o prazo; mínimo 1. */
export function monthsUntil(deadline: string, from = todayString()): number {
  const a = fromDateString(from);
  const b = fromDateString(deadline);
  const months =
    (b.getUTCFullYear() - a.getUTCFullYear()) * 12 + (b.getUTCMonth() - a.getUTCMonth());
  const adjusted = b.getUTCDate() >= a.getUTCDate() ? months : months - 1;
  return Math.max(1, adjusted);
}
