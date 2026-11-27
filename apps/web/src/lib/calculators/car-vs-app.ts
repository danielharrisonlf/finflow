export interface CarVsAppInput {
  carPriceCents: number;
  depreciationYearPct: number;
  /** Quanto o dinheiro do carro renderia aplicado (custo de oportunidade), % a.a. */
  opportunityYearPct: number;
  insuranceYearCents: number;
  ipvaPct: number;
  maintenanceYearCents: number;
  parkingMonthCents: number;
  kmPerMonth: number;
  kmPerLiter: number;
  fuelPriceCents: number;
  appCostPerKmCents: number;
}

export interface CarVsAppResult {
  items: { label: string; monthlyCents: number }[];
  carFixedMonthlyCents: number;
  carMonthlyCents: number;
  appMonthlyCents: number;
  /** Positivo: o carro custa mais por mês. */
  differenceMonthlyCents: number;
  /** Km por mês em que os dois custam igual (null se o app nunca empata). */
  breakEvenKm: number | null;
}

/**
 * Custo mensal de ter carro (1º ano, aproximação linear) × usar app de transporte
 * para a mesma quilometragem. Custo de oportunidade simples sobre o valor do carro.
 */
export function carVsApp(input: CarVsAppInput): CarVsAppResult {
  const price = input.carPriceCents;
  const fuelPerKm = input.kmPerLiter > 0 ? input.fuelPriceCents / input.kmPerLiter : 0;
  const items = [
    { label: "Depreciação", monthlyCents: (price * input.depreciationYearPct) / 100 / 12 },
    {
      label: "Rendimento que o dinheiro deixa de ter",
      monthlyCents: (price * input.opportunityYearPct) / 100 / 12,
    },
    { label: "Seguro", monthlyCents: input.insuranceYearCents / 12 },
    { label: "IPVA", monthlyCents: (price * input.ipvaPct) / 100 / 12 },
    { label: "Manutenção", monthlyCents: input.maintenanceYearCents / 12 },
    { label: "Estacionamento", monthlyCents: input.parkingMonthCents },
    { label: "Combustível", monthlyCents: fuelPerKm * input.kmPerMonth },
  ].map((i) => ({ label: i.label, monthlyCents: Math.round(i.monthlyCents) }));
  const carMonthly = items.reduce((acc, i) => acc + i.monthlyCents, 0);
  const fuelMonthly = items[items.length - 1]?.monthlyCents ?? 0;
  const fixed = carMonthly - fuelMonthly;
  const appMonthly = Math.round(input.appCostPerKmCents * input.kmPerMonth);
  const marginPerKm = input.appCostPerKmCents - fuelPerKm;
  return {
    items,
    carFixedMonthlyCents: fixed,
    carMonthlyCents: carMonthly,
    appMonthlyCents: appMonthly,
    differenceMonthlyCents: carMonthly - appMonthly,
    breakEvenKm: marginPerKm > 0 ? Math.round(fixed / marginPerKm) : null,
  };
}
