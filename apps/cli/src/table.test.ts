import pc from "picocolors";
import { describe, expect, it } from "vitest";
import { progressBar, renderTable, visibleWidth } from "./table";

describe("visibleWidth", () => {
  it("ignora códigos ANSI de cor", () => {
    expect(visibleWidth(pc.green("abc"))).toBe(3);
    expect(visibleWidth("abc")).toBe(3);
  });
});

describe("renderTable", () => {
  it("alinha colunas à esquerda e à direita", () => {
    const out = renderTable(
      [{ header: "Nome" }, { header: "Valor", align: "right" }],
      [
        ["Nubank", "R$ 1,00"],
        ["XP", "R$ 10,00"],
      ],
    );
    const lines = out.split("\n");
    expect(lines).toHaveLength(3);
    expect(lines[1]).toContain("Nubank");
    // valores alinhados pela direita: ambos terminam na mesma coluna
    expect(visibleWidth(lines[1] ?? "")).toBe(visibleWidth(lines[2] ?? ""));
  });
});

describe("progressBar", () => {
  it("preenche proporcionalmente e clampa em 100%", () => {
    expect(visibleWidth(progressBar(50, 10))).toBe(10);
    expect(visibleWidth(progressBar(250, 10))).toBe(10);
  });
});
