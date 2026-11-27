import { annualToMonthlyRate, futureValueCents } from "./common";

export type ExpenseFrequency = "DAY" | "WEEK" | "MONTH";

const TIMES_PER_YEAR: Record<ExpenseFrequency, number> = { DAY: 365, WEEK: 52, MONTH: 12 };

export interface SmallExpenseInput {
  amountCents: number;
  /** Quantas vezes por período (ex.: 5 vezes por semana). */
  times: number;
  frequency: ExpenseFrequency;
  investmentYearPct: number;
}

export interface SmallExpenseResult {
  perMonthCents: number;
  perYearCents: number;
  tenYearsCents: number;
  /** Se o mesmo valor fosse investido todo mês por 10 anos. */
  investedTenYearsCents: number;
}

/** Simulador de pequenos gastos: valor × frequência → mês, ano, 10 anos e investido. */
export function smallExpenses(input: SmallExpenseInput): SmallExpenseResult {
  const perYear = input.amountCents * input.times * TIMES_PER_YEAR[input.frequency];
  const perMonth = perYear / 12;
  return {
    perMonthCents: Math.round(perMonth),
    perYearCents: Math.round(perYear),
    tenYearsCents: Math.round(perYear * 10),
    investedTenYearsCents: futureValueCents(
      0,
      annualToMonthlyRate(input.investmentYearPct),
      120,
      perMonth,
    ),
  };
}
