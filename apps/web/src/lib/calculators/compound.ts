import { futureValueCents, type RatePeriod, toMonthlyRate } from "./common";

export interface CompoundInput {
  initialCents: number;
  monthlyCents: number;
  ratePct: number;
  ratePeriod: RatePeriod;
  months: number;
}

export interface CompoundResult {
  finalCents: number;
  investedCents: number;
  interestCents: number;
  /** Saldo ao fim de cada ano (e do último mês, se não fechar o ano). */
  yearly: { month: number; balanceCents: number; investedCents: number }[];
}

/** Juros compostos com aporte mensal no fim de cada mês. */
export function compoundInterest(input: CompoundInput): CompoundResult {
  const months = Math.max(0, Math.floor(input.months));
  const rate = toMonthlyRate(input.ratePct, input.ratePeriod);
  const finalCents = futureValueCents(input.initialCents, rate, months, input.monthlyCents);
  const investedCents = input.initialCents + input.monthlyCents * months;
  const yearly: CompoundResult["yearly"] = [];
  for (let m = 12; m <= months; m += 12) {
    yearly.push({
      month: m,
      balanceCents: futureValueCents(input.initialCents, rate, m, input.monthlyCents),
      investedCents: input.initialCents + input.monthlyCents * m,
    });
  }
  if (months % 12 !== 0 && months > 0) {
    yearly.push({ month: months, balanceCents: finalCents, investedCents });
  }
  return { finalCents, investedCents, interestCents: finalCents - investedCents, yearly };
}
