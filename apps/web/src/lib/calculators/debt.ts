import { annualToMonthlyRate, bisect } from "./common";

// ───────────────────────── quitar dívidas ─────────────────────────

export interface PayoffSimulation {
  months: number;
  totalInterestCents: number;
  totalPaidCents: number;
}

/**
 * Mês a mês: o saldo rende juros e a parcela abate. null quando a parcela não cobre
 * nem os juros do 1º mês (a dívida nunca acaba) ou passa de 100 anos.
 */
export function simulatePayoff(
  balanceCents: number,
  monthlyRatePct: number,
  paymentCents: number,
): PayoffSimulation | null {
  const rate = monthlyRatePct / 100;
  if (balanceCents <= 0) return { months: 0, totalInterestCents: 0, totalPaidCents: 0 };
  if (paymentCents <= balanceCents * rate) return null;
  let balance = balanceCents;
  let interest = 0;
  let paid = 0;
  let months = 0;
  while (balance > 0.5 && months < 1200) {
    const monthInterest = balance * rate;
    balance += monthInterest;
    interest += monthInterest;
    const pay = Math.min(paymentCents, balance);
    balance -= pay;
    paid += pay;
    months++;
  }
  if (balance > 0.5) return null;
  return { months, totalInterestCents: Math.round(interest), totalPaidCents: Math.round(paid) };
}

export interface DebtPayoffResult {
  base: PayoffSimulation | null;
  withExtra: PayoffSimulation | null;
  monthsSaved: number | null;
  interestSavedCents: number | null;
}

export function debtPayoff(
  balanceCents: number,
  monthlyRatePct: number,
  paymentCents: number,
  extraCents: number,
): DebtPayoffResult {
  const base = simulatePayoff(balanceCents, monthlyRatePct, paymentCents);
  const withExtra = simulatePayoff(balanceCents, monthlyRatePct, paymentCents + extraCents);
  return {
    base,
    withExtra,
    monthsSaved: base && withExtra ? base.months - withExtra.months : null,
    interestSavedCents:
      base && withExtra ? base.totalInterestCents - withExtra.totalInterestCents : null,
  };
}

// ───────────────────────── à vista × parcelado ─────────────────────────

export interface CashVsInstallmentsInput {
  cashPriceCents: number;
  installmentCents: number;
  installments: number;
  /** 1ª parcela no ato da compra (senão, 30 dias depois). */
  firstAtPurchase: boolean;
  /** Rendimento do dinheiro enquanto as parcelas não vencem, % a.a. */
  investmentYearPct: number;
}

export interface CashVsInstallmentsResult {
  totalInstallmentsCents: number;
  /** Valor presente das parcelas descontado pelo rendimento. */
  presentValueCents: number;
  best: "CASH" | "INSTALLMENTS" | "TIE";
  savingCents: number;
  /** Juros mensais embutidos no parcelamento (null se não há juros). */
  implicitMonthlyRatePct: number | null;
}

function presentValue(installment: number, n: number, rate: number, offset: number): number {
  let pv = 0;
  for (let k = 0; k < n; k++) pv += installment / (1 + rate) ** (k + offset);
  return pv;
}

/**
 * Parcelar vale a pena quando o valor presente das parcelas (o quanto você precisa
 * aplicar hoje para pagá-las) é menor que o preço à vista.
 */
export function cashVsInstallments(input: CashVsInstallmentsInput): CashVsInstallmentsResult {
  const n = Math.max(1, Math.floor(input.installments));
  const offset = input.firstAtPurchase ? 0 : 1;
  const rate = annualToMonthlyRate(input.investmentYearPct);
  const total = input.installmentCents * n;
  const pv = Math.round(presentValue(input.installmentCents, n, rate, offset));
  const diff = input.cashPriceCents - pv;
  const implicit =
    total > input.cashPriceCents && input.cashPriceCents > 0
      ? bisect(
          (r) => presentValue(input.installmentCents, n, r, offset) - input.cashPriceCents,
          0,
          1,
        ) * 100
      : null;
  return {
    totalInstallmentsCents: total,
    presentValueCents: pv,
    best: Math.abs(diff) < 100 ? "TIE" : diff > 0 ? "INSTALLMENTS" : "CASH",
    savingCents: Math.abs(diff),
    implicitMonthlyRatePct: implicit,
  };
}
