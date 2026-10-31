import { describe, expect, it } from "vitest";
import { goalPercent, monthlyNeededCents } from "./goal-math";

describe("monthlyNeededCents", () => {
  it("divide o restante pelos meses até o prazo, arredondando para cima", () => {
    // faltam R$ 6.000 em 12 meses → R$ 500/mês
    expect(monthlyNeededCents(1_000_000, 400_000, "2027-10-30", "2026-10-30")).toBe(50_000);
  });

  it("prazo no mês seguinte exige o restante inteiro", () => {
    expect(monthlyNeededCents(100_000, 40_000, "2026-11-30", "2026-10-30")).toBe(60_000);
  });

  it("sem prazo retorna null", () => {
    expect(monthlyNeededCents(100_000, 0, null)).toBeNull();
  });

  it("meta concluída retorna null", () => {
    expect(monthlyNeededCents(100_000, 100_000, "2027-01-01", "2026-10-30")).toBeNull();
  });

  it("prazo já vencido usa no mínimo 1 mês", () => {
    expect(monthlyNeededCents(100_000, 50_000, "2026-10-01", "2026-10-30")).toBe(50_000);
  });
});

describe("goalPercent", () => {
  it("calcula percentual com uma casa decimal", () => {
    expect(goalPercent(300_000, 100_000)).toBe(33.3);
  });

  it("não passa de 100%", () => {
    expect(goalPercent(100_000, 150_000)).toBe(100);
  });
});
