import type { BudgetStatus } from "@finflow/contracts";

export interface BudgetView {
  percent: number;
  status: BudgetStatus;
}

/** Verde < 70%, amarelo 70–90%, vermelho > 90% (ou estourado). */
export function computeBudgetView(amountCents: number, spentCents: number): BudgetView {
  const ratio = amountCents > 0 ? (spentCents / amountCents) * 100 : 0;
  const status: BudgetStatus = ratio > 90 ? "DANGER" : ratio >= 70 ? "WARNING" : "OK";
  return { percent: Math.round(ratio * 10) / 10, status };
}
