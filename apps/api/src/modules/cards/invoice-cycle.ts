import type { InvoiceStatus } from "@finflow/contracts";
import { fromDateString, toDateString } from "../../common/date.util";

export interface InvoicePeriod {
  year: number;
  month: number;
}

/**
 * Fatura à qual pertence uma compra: a primeira cuja data de fechamento
 * é posterior à compra. Compra no dia do fechamento já cai na próxima.
 */
export function invoicePeriodForPurchase(dateStr: string, closingDay: number): InvoicePeriod {
  const d = fromDateString(dateStr);
  let year = d.getUTCFullYear();
  let month = d.getUTCMonth() + 1;
  if (d.getUTCDate() >= closingDay) {
    month += 1;
    if (month > 12) {
      month = 1;
      year += 1;
    }
  }
  return { year, month };
}

export function addMonthsToPeriod(period: InvoicePeriod, delta: number): InvoicePeriod {
  const zeroBased = period.year * 12 + (period.month - 1) + delta;
  return { year: Math.floor(zeroBased / 12), month: (zeroBased % 12) + 1 };
}

export function invoiceClosingDate(period: InvoicePeriod, closingDay: number): string {
  const d = new Date(Date.UTC(period.year, period.month - 1, closingDay));
  return toDateString(d);
}

/** Vencimento: mesmo mês quando dueDay > closingDay; senão, mês seguinte. */
export function invoiceDueDate(period: InvoicePeriod, closingDay: number, dueDay: number): string {
  const duePeriod = dueDay > closingDay ? period : addMonthsToPeriod(period, 1);
  const d = new Date(Date.UTC(duePeriod.year, duePeriod.month - 1, dueDay));
  return toDateString(d);
}

/** PAID é persistido; OPEN/CLOSED derivam da data de fechamento. */
export function effectiveInvoiceStatus(
  paid: boolean,
  closingDate: string,
  today: string,
): InvoiceStatus {
  if (paid) return "PAID";
  return today >= closingDate ? "CLOSED" : "OPEN";
}
