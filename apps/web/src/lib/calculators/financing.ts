import { pricePayment } from "./common";

export interface AmortizationRow {
  number: number;
  installmentCents: number;
  interestCents: number;
  amortizationCents: number;
  balanceCents: number;
}

export interface AmortizationPlan {
  rows: AmortizationRow[];
  firstInstallmentCents: number;
  lastInstallmentCents: number;
  totalPaidCents: number;
  totalInterestCents: number;
}

function finish(rows: AmortizationRow[], principalCents: number, totalPaid: number) {
  const totalPaidCents = Math.round(totalPaid);
  return {
    rows,
    firstInstallmentCents: rows[0]?.installmentCents ?? 0,
    lastInstallmentCents: rows[rows.length - 1]?.installmentCents ?? 0,
    totalPaidCents,
    totalInterestCents: totalPaidCents - principalCents,
  };
}

/** Tabela Price: parcela constante; juros sobre o saldo, amortização = parcela − juros. */
export function priceSchedule(
  principalCents: number,
  monthlyRate: number,
  months: number,
): AmortizationPlan {
  const n = Math.max(0, Math.floor(months));
  const payment = pricePayment(principalCents, monthlyRate, n);
  const rows: AmortizationRow[] = [];
  let balance = principalCents;
  for (let k = 1; k <= n; k++) {
    const interest = balance * monthlyRate;
    const amortization = payment - interest;
    balance = Math.max(0, balance - amortization);
    rows.push({
      number: k,
      installmentCents: Math.round(payment),
      interestCents: Math.round(interest),
      amortizationCents: Math.round(amortization),
      balanceCents: Math.round(balance),
    });
  }
  return finish(rows, principalCents, payment * n);
}

/** SAC: amortização constante (P/n); juros sobre o saldo, parcelas decrescentes. */
export function sacSchedule(
  principalCents: number,
  monthlyRate: number,
  months: number,
): AmortizationPlan {
  const n = Math.max(0, Math.floor(months));
  const amortization = n > 0 ? principalCents / n : 0;
  const rows: AmortizationRow[] = [];
  let balance = principalCents;
  let totalPaid = 0;
  for (let k = 1; k <= n; k++) {
    const interest = balance * monthlyRate;
    const installment = amortization + interest;
    balance = Math.max(0, balance - amortization);
    totalPaid += installment;
    rows.push({
      number: k,
      installmentCents: Math.round(installment),
      interestCents: Math.round(interest),
      amortizationCents: Math.round(amortization),
      balanceCents: Math.round(balance),
    });
  }
  return finish(rows, principalCents, totalPaid);
}
