/** Formata centavos como BRL (usado em mensagens do Telegram e exports). */
export function formatBRL(cents: number): string {
  return new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" }).format(cents / 100);
}

/**
 * Divide um total em N parcelas inteiras que somam exatamente o total.
 * O resto é distribuído 1 centavo por vez nas primeiras parcelas.
 */
export function splitInstallments(totalCents: number, installments: number): number[] {
  if (installments < 1 || !Number.isInteger(installments)) {
    throw new Error("número de parcelas inválido");
  }
  const base = Math.floor(totalCents / installments);
  const remainder = totalCents - base * installments;
  return Array.from({ length: installments }, (_, i) => base + (i < remainder ? 1 : 0));
}

/** Converte valor pt-BR ("1.234,56", "45,90", "50") para centavos; null se inválido. */
export function parseBRLToCents(raw: string): number | null {
  const cleaned = raw.trim().replace(/^r\$\s*/i, "");
  if (!/^[\d.,]+$/.test(cleaned)) return null;
  let normalized = cleaned;
  if (cleaned.includes(",")) {
    normalized = cleaned.replace(/\./g, "").replace(",", ".");
  }
  const value = Number(normalized);
  if (!Number.isFinite(value) || value <= 0) return null;
  return Math.round(value * 100);
}
