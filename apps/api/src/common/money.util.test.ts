import { describe, expect, it } from "vitest";
import { formatBRL, parseBRLToCents, splitInstallments } from "./money.util";

describe("splitInstallments", () => {
  it("divide igualmente quando não há resto", () => {
    expect(splitInstallments(350_000, 10)).toEqual(Array(10).fill(35_000));
  });

  it("distribui o resto nas primeiras parcelas, somando exatamente o total", () => {
    const parts = splitInstallments(100_00, 3); // R$ 100,00 em 3x
    expect(parts).toEqual([3334, 3333, 3333]);
    expect(parts.reduce((a, b) => a + b, 0)).toBe(100_00);
  });

  it("uma parcela devolve o total", () => {
    expect(splitInstallments(4590, 1)).toEqual([4590]);
  });

  it("rejeita número de parcelas inválido", () => {
    expect(() => splitInstallments(1000, 0)).toThrow();
  });
});

describe("parseBRLToCents", () => {
  it("entende vírgula decimal e milhar com ponto", () => {
    expect(parseBRLToCents("45,90")).toBe(4590);
    expect(parseBRLToCents("1.234,56")).toBe(123_456);
  });

  it("entende inteiro e prefixo R$", () => {
    expect(parseBRLToCents("50")).toBe(5000);
    expect(parseBRLToCents("R$ 12,30")).toBe(1230);
  });

  it("rejeita lixo e valores não positivos", () => {
    expect(parseBRLToCents("abc")).toBeNull();
    expect(parseBRLToCents("0")).toBeNull();
    expect(parseBRLToCents("")).toBeNull();
  });
});

describe("formatBRL", () => {
  it("formata centavos como moeda brasileira", () => {
    // Intl usa espaço não separável entre R$ e o número
    expect(formatBRL(123_456).replace(/ /g, " ")).toBe("R$ 1.234,56");
  });
});
