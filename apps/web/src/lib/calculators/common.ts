/**
 * Utilitários das calculadoras (v3 §16). Dinheiro sempre em centavos; as contas
 * intermediárias usam ponto flutuante e arredondam para centavos só no resultado.
 */

export type RatePeriod = "MONTH" | "YEAR";

/** Taxa anual em % → taxa mensal equivalente (juros compostos), como fração. */
export function annualToMonthlyRate(annualPct: number): number {
  return (1 + annualPct / 100) ** (1 / 12) - 1;
}

/** Taxa mensal em % → taxa anual equivalente em %. */
export function monthlyToAnnualPct(monthlyPct: number): number {
  return ((1 + monthlyPct / 100) ** 12 - 1) * 100;
}

/** Taxa informada em % ao mês ou ao ano → fração mensal. */
export function toMonthlyRate(pct: number, period: RatePeriod): number {
  return period === "MONTH" ? pct / 100 : annualToMonthlyRate(pct);
}

/**
 * Valor futuro de um saldo inicial mais aportes mensais no fim de cada mês.
 * FV = PV·(1+i)^n + PMT·((1+i)^n − 1)/i.
 */
export function futureValueCents(
  presentCents: number,
  monthlyRate: number,
  months: number,
  monthlyContributionCents = 0,
): number {
  if (monthlyRate === 0) return Math.round(presentCents + monthlyContributionCents * months);
  const growth = (1 + monthlyRate) ** months;
  return Math.round(
    presentCents * growth + (monthlyContributionCents * (growth - 1)) / monthlyRate,
  );
}

/** Parcela fixa da tabela Price (sem arredondar): P·i / (1 − (1+i)^−n). */
export function pricePayment(principalCents: number, monthlyRate: number, months: number): number {
  if (months <= 0) return 0;
  if (monthlyRate === 0) return principalCents / months;
  return (principalCents * monthlyRate) / (1 - (1 + monthlyRate) ** -months);
}

/**
 * Meses até um saldo alcançar a meta com aportes mensais e rendimento.
 * Retorna 0 se já alcançou e null se não chega em `maxMonths`.
 */
export function monthsToReach(
  startCents: number,
  targetCents: number,
  monthlyContributionCents: number,
  monthlyRate: number,
  maxMonths = 1200,
): number | null {
  if (startCents >= targetCents) return 0;
  let balance = startCents;
  for (let month = 1; month <= maxMonths; month++) {
    balance = balance * (1 + monthlyRate) + monthlyContributionCents;
    if (balance >= targetCents) return month;
  }
  return null;
}

/** Busca por bisseção a raiz de uma função crescente ou decrescente em [lo, hi]. */
export function bisect(fn: (x: number) => number, lo: number, hi: number, steps = 100): number {
  let a = lo;
  let b = hi;
  const fa = fn(a);
  for (let i = 0; i < steps; i++) {
    const mid = (a + b) / 2;
    const fm = fn(mid);
    if (fm === 0) return mid;
    if (Math.sign(fm) === Math.sign(fa)) a = mid;
    else b = mid;
  }
  return (a + b) / 2;
}
