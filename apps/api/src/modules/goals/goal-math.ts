import { monthsUntil } from "../../common/date.util";

/** Aporte mensal necessário para bater a meta até o prazo; null sem prazo ou concluída. */
export function monthlyNeededCents(
  targetCents: number,
  savedCents: number,
  deadline: string | null,
  today?: string,
): number | null {
  if (!deadline) return null;
  const remaining = targetCents - savedCents;
  if (remaining <= 0) return null;
  return Math.ceil(remaining / monthsUntil(deadline, today));
}

export function goalPercent(targetCents: number, savedCents: number): number {
  if (targetCents <= 0) return 0;
  return Math.min(100, Math.round((savedCents / targetCents) * 1000) / 10);
}
