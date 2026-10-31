import { describe, expect, it } from "vitest";
import { parseTelegramMessage } from "./parser";

describe("parseTelegramMessage", () => {
  it("reconhece comandos com e sem menção ao bot", () => {
    expect(parseTelegramMessage("/saldo")).toEqual({ kind: "balance" });
    expect(parseTelegramMessage("/saldo@finflow_bot")).toEqual({ kind: "balance" });
    expect(parseTelegramMessage("/resumo")).toEqual({ kind: "summary" });
    expect(parseTelegramMessage("/extrato")).toEqual({ kind: "statement" });
    expect(parseTelegramMessage("/ajuda")).toEqual({ kind: "help" });
    expect(parseTelegramMessage("/start")).toEqual({ kind: "help" });
  });

  it("lança despesa rápida com vírgula decimal", () => {
    expect(parseTelegramMessage("45,90 almoço no shopping")).toEqual({
      kind: "expense",
      amountCents: 4590,
      description: "almoço no shopping",
      categoryName: null,
    });
  });

  it("aceita milhar pt-BR e hashtag de categoria", () => {
    expect(parseTelegramMessage("1.250,00 passagem aérea #viagem")).toEqual({
      kind: "expense",
      amountCents: 125_000,
      description: "passagem aérea",
      categoryName: "viagem",
    });
  });

  it("aceita valor inteiro sem centavos", () => {
    expect(parseTelegramMessage("50 uber")).toEqual({
      kind: "expense",
      amountCents: 5000,
      description: "uber",
      categoryName: null,
    });
  });

  it("hashtag com underscore vira nome com espaço", () => {
    const parsed = parseTelegramMessage("30 pastel #comida_de_rua");
    expect(parsed).toMatchObject({ kind: "expense", categoryName: "comida de rua" });
  });

  it("rejeita comando desconhecido e texto sem valor", () => {
    expect(parseTelegramMessage("/naoexiste").kind).toBe("unknown");
    expect(parseTelegramMessage("comprei um lanche").kind).toBe("unknown");
    expect(parseTelegramMessage("0,00 nada").kind).toBe("unknown");
  });
});
