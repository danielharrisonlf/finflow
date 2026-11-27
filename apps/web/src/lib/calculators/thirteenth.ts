/**
 * 13º salário: INSS progressivo por faixa e IRRF com tributação exclusiva na fonte.
 * As tabelas abaixo são valores de referência (2025) e ficam editáveis na tela:
 * confira sempre a tabela vigente.
 */

export interface InssBracket {
  /** Teto da faixa em centavos (a última é o teto de contribuição). */
  upToCents: number;
  ratePct: number;
}

export interface IrrfBracket {
  /** Teto da faixa; null = sem teto (última faixa). */
  upToCents: number | null;
  ratePct: number;
  deductionCents: number;
}

export const INSS_REFERENCE: InssBracket[] = [
  { upToCents: 151_800, ratePct: 7.5 },
  { upToCents: 279_388, ratePct: 9 },
  { upToCents: 419_083, ratePct: 12 },
  { upToCents: 815_741, ratePct: 14 },
];

export const IRRF_REFERENCE: IrrfBracket[] = [
  { upToCents: 242_880, ratePct: 0, deductionCents: 0 },
  { upToCents: 282_665, ratePct: 7.5, deductionCents: 18_216 },
  { upToCents: 375_105, ratePct: 15, deductionCents: 39_416 },
  { upToCents: 466_468, ratePct: 22.5, deductionCents: 67_549 },
  { upToCents: null, ratePct: 27.5, deductionCents: 90_873 },
];

export const DEPENDENT_DEDUCTION_REFERENCE = 18_959;

/** INSS progressivo: cada faixa incide só sobre a parte do salário dentro dela. */
export function inssProgressive(baseCents: number, table: InssBracket[]): number {
  let previous = 0;
  let total = 0;
  for (const bracket of table) {
    if (baseCents <= previous) break;
    total += ((Math.min(baseCents, bracket.upToCents) - previous) * bracket.ratePct) / 100;
    previous = bracket.upToCents;
  }
  return Math.round(total);
}

/** IRRF pela tabela progressiva: base × alíquota − parcela a deduzir (mínimo zero). */
export function irrf(
  baseCents: number,
  table: IrrfBracket[],
): { taxCents: number; ratePct: number } {
  if (baseCents <= 0) return { taxCents: 0, ratePct: 0 };
  const bracket =
    table.find((b) => b.upToCents === null || baseCents <= b.upToCents) ?? table[table.length - 1];
  if (!bracket) return { taxCents: 0, ratePct: 0 };
  const tax = Math.round((baseCents * bracket.ratePct) / 100 - bracket.deductionCents);
  return { taxCents: Math.max(0, tax), ratePct: bracket.ratePct };
}

export interface ThirteenthInput {
  salaryCents: number;
  /** Meses trabalhados no ano (fração de 15+ dias conta como mês). */
  monthsWorked: number;
  /** Média de horas extras, comissões e adicionais que integram o 13º. */
  extrasCents: number;
  dependents: number;
  inss: InssBracket[];
  irrf: IrrfBracket[];
  dependentDeductionCents: number;
}

export interface ThirteenthResult {
  grossCents: number;
  inssCents: number;
  irrfBaseCents: number;
  irrfRatePct: number;
  irrfCents: number;
  netCents: number;
  /** 1ª parcela (até 30/11): metade do bruto, sem descontos. */
  firstInstallmentCents: number;
  /** 2ª parcela (até 20/12): líquido − 1ª parcela. */
  secondInstallmentCents: number;
}

export function thirteenthSalary(input: ThirteenthInput): ThirteenthResult {
  const months = Math.min(12, Math.max(0, Math.floor(input.monthsWorked)));
  const gross = Math.round(((input.salaryCents + input.extrasCents) * months) / 12);
  const inss = inssProgressive(gross, input.inss);
  const base = Math.max(0, gross - inss - input.dependents * input.dependentDeductionCents);
  const tax = irrf(base, input.irrf);
  const net = gross - inss - tax.taxCents;
  const first = Math.round(gross / 2);
  return {
    grossCents: gross,
    inssCents: inss,
    irrfBaseCents: base,
    irrfRatePct: tax.ratePct,
    irrfCents: tax.taxCents,
    netCents: net,
    firstInstallmentCents: first,
    secondInstallmentCents: net - first,
  };
}
