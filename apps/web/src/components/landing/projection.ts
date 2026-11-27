// Balance bands in cents (lower bounds), matching the app defaults.
export type Band = "dark-green" | "light-green" | "yellow" | "light-red" | "red";

export function bandOf(cents: number): Band {
  if (cents >= 200000) return "dark-green";
  if (cents >= 100000) return "light-green";
  if (cents >= 0) return "yellow";
  if (cents >= -50000) return "light-red";
  return "red";
}

export const bandNames: { band: Band; label: string }[] = [
  { band: "dark-green", label: "acima de R$ 2 mil" },
  { band: "light-green", label: "R$ 1 mil a 2 mil" },
  { band: "yellow", label: "até R$ 1 mil" },
  { band: "light-red", label: "até −R$ 500" },
  { band: "red", label: "abaixo de −R$ 500" },
];

// Thirty illustrative days: rent, card bill and an installment before payday.
const start = 248000;
const dailySpend = 5500;
const events: Record<number, { cents: number; label: string }> = {
  2: { cents: -12900, label: "Internet e celular" },
  4: { cents: -100000, label: "Aluguel" },
  7: { cents: -45000, label: "Fatura do cartão" },
  9: { cents: -25000, label: "Parcela do notebook" },
  13: { cents: 420000, label: "Salário" },
  20: { cents: -32000, label: "Energia e água" },
};

export const projectionDays = Array.from({ length: 30 }, (_, i) => i).reduce<
  { day: number; balance: number }[]
>((days, day) => {
  const previous = days.at(-1)?.balance ?? start;
  const balance = previous - dailySpend + (events[day]?.cents ?? 0);
  days.push({ day, balance });
  return days;
}, []);

export const projectionEvents = events;

export const lowestDay = projectionDays.reduce((low, d) => (d.balance < low.balance ? d : low));
