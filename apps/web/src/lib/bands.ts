import type { BalanceBand, SpendType } from "@finflow/contracts";

/** Faixas de saldo projetado (método da planilha diária). Cores em styles.css. */
export const BAND_META: Record<BalanceBand, { label: string; color: string }> = {
  DARK_GREEN: { label: "Folga", color: "var(--band-dark-green)" },
  LIGHT_GREEN: { label: "Confortável", color: "var(--band-light-green)" },
  YELLOW: { label: "Atenção", color: "var(--band-yellow)" },
  LIGHT_RED: { label: "Apertado", color: "var(--band-light-red)" },
  RED: { label: "Negativo", color: "var(--band-red)" },
};

export const BAND_ORDER: BalanceBand[] = [
  "DARK_GREEN",
  "LIGHT_GREEN",
  "YELLOW",
  "LIGHT_RED",
  "RED",
];

export const SPEND_TYPE_META: Record<SpendType, { label: string; hint: string; color: string }> = {
  FIXED: {
    label: "Fixo",
    hint: "Contas que se repetem e são difíceis de mudar: aluguel, escola, plano de saúde.",
    color: "var(--spend-fixed)",
  },
  CUTTABLE: {
    label: "Cortável",
    hint: "Assinaturas e serviços que você pode cancelar ou trocar sem sofrer.",
    color: "var(--spend-cuttable)",
  },
  VARIABLE: {
    label: "Variável",
    hint: "O dia a dia: mercado, delivery, transporte. Muda conforme suas escolhas.",
    color: "var(--spend-variable)",
  },
};

export const SPEND_TYPE_ORDER: SpendType[] = ["FIXED", "CUTTABLE", "VARIABLE"];
