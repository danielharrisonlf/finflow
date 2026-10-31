import { describe, expect, it } from "vitest";
import { crossedThreshold } from "./budget-threshold";

describe("crossedThreshold", () => {
  it("detecta cruzamento do limiar de 70%", () => {
    expect(crossedThreshold(65, 72)).toBe(70);
  });

  it("detecta cruzamento do limiar de 90% (prioridade sobre o de 70%)", () => {
    expect(crossedThreshold(65, 95)).toBe(90);
    expect(crossedThreshold(85, 91)).toBe(90);
  });

  it("não repete alerta quando já estava acima do limiar", () => {
    expect(crossedThreshold(75, 80)).toBeNull();
    expect(crossedThreshold(92, 99)).toBeNull();
  });

  it("não alerta abaixo dos limiares", () => {
    expect(crossedThreshold(10, 50)).toBeNull();
  });
});
