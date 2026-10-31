import { describe, expect, it } from "vitest";
import {
  addMonthsToPeriod,
  effectiveInvoiceStatus,
  invoiceClosingDate,
  invoiceDueDate,
  invoicePeriodForPurchase,
} from "./invoice-cycle";

describe("invoicePeriodForPurchase", () => {
  it("compra antes do fechamento cai na fatura do mês corrente", () => {
    expect(invoicePeriodForPurchase("2026-10-04", 5)).toEqual({ year: 2026, month: 10 });
  });

  it("compra no dia do fechamento cai na fatura seguinte", () => {
    expect(invoicePeriodForPurchase("2026-10-05", 5)).toEqual({ year: 2026, month: 11 });
  });

  it("compra após o fechamento em dezembro vira fatura de janeiro do ano seguinte", () => {
    expect(invoicePeriodForPurchase("2026-12-28", 20)).toEqual({ year: 2027, month: 1 });
  });
});

describe("addMonthsToPeriod", () => {
  it("avança parcelas atravessando a virada de ano", () => {
    expect(addMonthsToPeriod({ year: 2026, month: 11 }, 3)).toEqual({ year: 2027, month: 2 });
  });
});

describe("invoiceDueDate", () => {
  it("vence no mesmo mês quando o vencimento é após o fechamento", () => {
    expect(invoiceDueDate({ year: 2026, month: 10 }, 5, 12)).toBe("2026-10-12");
  });

  it("vence no mês seguinte quando o vencimento é antes do fechamento", () => {
    expect(invoiceDueDate({ year: 2026, month: 12 }, 25, 4)).toBe("2027-01-04");
  });
});

describe("effectiveInvoiceStatus", () => {
  const closing = invoiceClosingDate({ year: 2026, month: 10 }, 15);

  it("aberta antes do fechamento", () => {
    expect(effectiveInvoiceStatus(false, closing, "2026-10-14")).toBe("OPEN");
  });

  it("fechada a partir do dia do fechamento", () => {
    expect(effectiveInvoiceStatus(false, closing, "2026-10-15")).toBe("CLOSED");
  });

  it("paga quando houve pagamento, independentemente da data", () => {
    expect(effectiveInvoiceStatus(true, closing, "2026-10-01")).toBe("PAID");
  });
});
