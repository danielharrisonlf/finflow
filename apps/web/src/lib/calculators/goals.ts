import { annualToMonthlyRate, monthsToReach } from "./common";

// ───────────────────────── reserva de emergência ─────────────────────────

export interface EmergencyInput {
  monthlyCostCents: number;
  months: number;
  savedCents: number;
  monthlySavingCents: number;
  yieldYearPct: number;
}

export interface EmergencyResult {
  targetCents: number;
  missingCents: number;
  /** Meses para completar guardando o valor mensal com rendimento; null se não chega. */
  monthsToTarget: number | null;
  coveredMonths: number;
}

/** Reserva = custo de vida mensal × meses de proteção; prazo com aportes e rendimento. */
export function emergencyFund(input: EmergencyInput): EmergencyResult {
  const target = Math.round(input.monthlyCostCents * input.months);
  return {
    targetCents: target,
    missingCents: Math.max(0, target - input.savedCents),
    monthsToTarget: monthsToReach(
      input.savedCents,
      target,
      input.monthlySavingCents,
      annualToMonthlyRate(input.yieldYearPct),
      600,
    ),
    coveredMonths: input.monthlyCostCents > 0 ? input.savedCents / input.monthlyCostCents : 0,
  };
}

// ───────────────────────── independência financeira ─────────────────────────

export interface FireInput {
  monthlySpendCents: number;
  /** Regra de retirada anual (padrão 4% a.a.). */
  withdrawalPct: number;
  currentCents: number;
  monthlyContributionCents: number;
  /** Rendimento real (acima da inflação), % a.a. */
  realReturnYearPct: number;
}

export interface FireResult {
  /** Patrimônio necessário: gasto anual ÷ taxa de retirada. */
  targetCents: number;
  progressPct: number;
  monthsToTarget: number | null;
  /** Renda mensal que o patrimônio atual já sustenta pela regra. */
  currentIncomeCents: number;
}

export function financialIndependence(input: FireInput): FireResult {
  const rate = input.withdrawalPct / 100;
  const target = rate > 0 ? Math.round((input.monthlySpendCents * 12) / rate) : 0;
  return {
    targetCents: target,
    progressPct: target > 0 ? Math.min(100, (input.currentCents / target) * 100) : 0,
    monthsToTarget:
      target > 0
        ? monthsToReach(
            input.currentCents,
            target,
            input.monthlyContributionCents,
            annualToMonthlyRate(input.realReturnYearPct),
          )
        : null,
    currentIncomeCents: Math.round((input.currentCents * rate) / 12),
  };
}
