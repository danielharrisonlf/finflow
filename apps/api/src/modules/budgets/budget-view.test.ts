import { describe, expect, it } from "vitest";
import { computeBudgetView } from "./budget-view";

describe("computeBudgetView", () => {
  it("abaixo de 70% é OK", () => {
    expect(computeBudgetView(150_000, 100_000)).toEqual({ percent: 66.7, status: "OK" });
  });

  it("de 70% a 90% é WARNING", () => {
    expect(computeBudgetView(100_000, 70_000).status).toBe("WARNING");
    expect(computeBudgetView(100_000, 90_000).status).toBe("WARNING");
  });

  it("acima de 90% é DANGER", () => {
    expect(computeBudgetView(100_000, 90_001).status).toBe("DANGER");
  });

  it("orçamento estourado passa de 100%", () => {
    const view = computeBudgetView(100_000, 125_000);
    expect(view.percent).toBe(125);
    expect(view.status).toBe("DANGER");
  });

  it("orçamento zerado não divide por zero", () => {
    expect(computeBudgetView(0, 5_000)).toEqual({ percent: 0, status: "OK" });
  });
});
