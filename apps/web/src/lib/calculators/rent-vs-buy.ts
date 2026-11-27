import { annualToMonthlyRate, pricePayment } from "./common";

export interface RentVsBuyInput {
  propertyCents: number;
  downPaymentCents: number;
  /** ITBI, escritura e registro, em % do valor do imóvel. */
  closingCostsPct: number;
  financingYearPct: number;
  termYears: number;
  rentCents: number;
  rentIncreaseYearPct: number;
  appreciationYearPct: number;
  investmentYearPct: number;
  /** Manutenção e IPTU do dono, em % do valor do imóvel por ano. */
  maintenanceYearPct: number;
  horizonYears: number;
}

export interface RentVsBuyResult {
  financedCents: number;
  installmentCents: number;
  totalInterestCents: number;
  propertyValueCents: number;
  remainingDebtCents: number;
  buyerInvestmentsCents: number;
  /** Patrimônio de quem comprou: imóvel − dívida restante + investimentos. */
  buyerNetCents: number;
  /** Patrimônio de quem alugou e investiu a diferença. */
  renterNetCents: number;
  winner: "BUY" | "RENT" | "TIE";
  differenceCents: number;
}

/**
 * Comprar financiado × alugar e investir a diferença. Os dois gastam o mesmo por mês:
 * quem tem o custo mensal menor investe a diferença. Quem aluga começa investindo a
 * entrada e as despesas de compra. Parcelas pela tabela Price; aluguel reajustado
 * uma vez por ano.
 */
export function rentVsBuy(input: RentVsBuyInput): RentVsBuyResult {
  const termMonths = Math.max(1, Math.round(input.termYears * 12));
  const horizon = Math.max(1, Math.round(input.horizonYears * 12));
  const financed = Math.max(0, input.propertyCents - input.downPaymentCents);
  const finRate = annualToMonthlyRate(input.financingYearPct);
  const invRate = annualToMonthlyRate(input.investmentYearPct);
  const apprRate = annualToMonthlyRate(input.appreciationYearPct);
  const payment = pricePayment(financed, finRate, termMonths);
  const closing = (input.propertyCents * input.closingCostsPct) / 100;

  let balance = financed;
  let renterInv = input.downPaymentCents + closing;
  let buyerInv = 0;
  let propertyValue = input.propertyCents;
  let interestPaid = 0;

  for (let m = 1; m <= horizon; m++) {
    let installment = 0;
    if (m <= termMonths && balance > 0) {
      const interest = balance * finRate;
      installment = Math.min(payment, balance + interest);
      interestPaid += interest;
      balance = Math.max(0, balance + interest - installment);
    }
    const maintenance = (propertyValue * input.maintenanceYearPct) / 100 / 12;
    const buyerOutlay = installment + maintenance;
    const rent =
      input.rentCents * (1 + input.rentIncreaseYearPct / 100) ** Math.floor((m - 1) / 12);
    const diff = buyerOutlay - rent;
    renterInv = renterInv * (1 + invRate) + Math.max(diff, 0);
    buyerInv = buyerInv * (1 + invRate) + Math.max(-diff, 0);
    propertyValue *= 1 + apprRate;
  }

  const buyerNet = propertyValue - balance + buyerInv;
  const difference = Math.round(Math.abs(buyerNet - renterInv));
  return {
    financedCents: financed,
    installmentCents: Math.round(payment),
    totalInterestCents: Math.round(interestPaid),
    propertyValueCents: Math.round(propertyValue),
    remainingDebtCents: Math.round(balance),
    buyerInvestmentsCents: Math.round(buyerInv),
    buyerNetCents: Math.round(buyerNet),
    renterNetCents: Math.round(renterInv),
    winner: difference < 100 ? "TIE" : buyerNet > renterInv ? "BUY" : "RENT",
    differenceCents: difference,
  };
}
