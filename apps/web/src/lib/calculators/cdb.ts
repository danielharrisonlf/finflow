import { bisect } from "./common";

/** IR regressivo de renda fixa pelo prazo em dias corridos. */
export function fixedIncomeTaxPct(days: number): number {
  if (days <= 180) return 22.5;
  if (days <= 360) return 20;
  if (days <= 720) return 17.5;
  return 15;
}

/**
 * Rendimento mensal da poupança: 0,5% a.m. + TR quando a Selic passa de 8,5% a.a.;
 * senão 70% da Selic (convertida para mensal) + TR. Retorna fração mensal.
 */
export function savingsMonthlyRate(selicYearPct: number, trMonthPct: number): number {
  const base = selicYearPct > 8.5 ? 0.005 : (1 + (selicYearPct * 0.7) / 100) ** (1 / 12) - 1;
  return base + trMonthPct / 100;
}

export interface CdbVsSavingsInput {
  amountCents: number;
  days: number;
  cdbPctOfCdi: number;
  cdiYearPct: number;
  selicYearPct: number;
  trMonthPct: number;
}

export interface CdbVsSavingsResult {
  businessDays: number;
  cdbGrossCents: number;
  irPct: number;
  irCents: number;
  cdbNetCents: number;
  /** Meses completos: a poupança só rende no aniversário. */
  savingsMonths: number;
  savingsMonthlyPct: number;
  savingsGainCents: number;
  winner: "CDB" | "SAVINGS" | "TIE";
  /** % do CDI que o CDB precisa pagar para empatar com a poupança (após IR). */
  breakEvenPctOfCdi: number | null;
}

/** Ganho bruto do CDB: CDI diário (base 252) × % do CDI, por dias úteis estimados. */
function cdbGross(amount: number, cdiYearPct: number, pct: number, businessDays: number): number {
  const daily = (1 + cdiYearPct / 100) ** (1 / 252) - 1;
  return amount * ((1 + daily * (pct / 100)) ** businessDays - 1);
}

export function cdbVsSavings(input: CdbVsSavingsInput): CdbVsSavingsResult {
  const days = Math.max(1, Math.floor(input.days));
  const businessDays = Math.round((days * 252) / 365);
  const irPct = fixedIncomeTaxPct(days);
  const gross = cdbGross(input.amountCents, input.cdiYearPct, input.cdbPctOfCdi, businessDays);
  const grossCents = Math.round(gross);
  const irCents = Math.round((gross * irPct) / 100);
  const netCents = grossCents - irCents;

  const months = Math.floor(days / 30);
  const monthly = savingsMonthlyRate(input.selicYearPct, input.trMonthPct);
  const savingsGain = Math.round(input.amountCents * ((1 + monthly) ** months - 1));

  const netFor = (pct: number) =>
    cdbGross(input.amountCents, input.cdiYearPct, pct, businessDays) * (1 - irPct / 100);
  const breakEven =
    savingsGain > 0 && input.cdiYearPct > 0
      ? bisect((pct) => netFor(pct) - savingsGain, 0, 500)
      : null;

  return {
    businessDays,
    cdbGrossCents: grossCents,
    irPct,
    irCents,
    cdbNetCents: netCents,
    savingsMonths: months,
    savingsMonthlyPct: monthly * 100,
    savingsGainCents: savingsGain,
    winner:
      Math.abs(netCents - savingsGain) < 100 ? "TIE" : netCents > savingsGain ? "CDB" : "SAVINGS",
    breakEvenPctOfCdi: breakEven,
  };
}
