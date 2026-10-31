import { describe, expect, it } from "vitest";
import { addDays, addMonthsToDateString, monthRange, monthsUntil, toDateString } from "./date.util";

describe("monthRange", () => {
  it("cobre [primeiro dia, primeiro dia do mês seguinte)", () => {
    const range = monthRange(2026, 10);
    expect(toDateString(range.gte)).toBe("2026-10-01");
    expect(toDateString(range.lt)).toBe("2026-11-01");
  });

  it("vira o ano em dezembro", () => {
    expect(toDateString(monthRange(2026, 12).lt)).toBe("2027-01-01");
  });
});

describe("addMonthsToDateString", () => {
  it("mantém o dia quando possível", () => {
    expect(addMonthsToDateString("2026-01-15", 2)).toBe("2026-03-15");
  });

  it("clampa para o último dia do mês destino", () => {
    expect(addMonthsToDateString("2026-01-31", 1)).toBe("2026-02-28");
  });
});

describe("addDays", () => {
  it("atravessa o fim do mês", () => {
    expect(addDays("2026-10-30", 7)).toBe("2026-11-06");
  });
});

describe("monthsUntil", () => {
  it("conta meses cheios até o prazo", () => {
    expect(monthsUntil("2027-10-30", "2026-10-30")).toBe(12);
  });

  it("prazo passado retorna no mínimo 1", () => {
    expect(monthsUntil("2026-01-01", "2026-10-30")).toBe(1);
  });
});
