import {
  Car,
  Coffee,
  CreditCard,
  Gift,
  HandCoins,
  House,
  Landmark,
  type LucideIcon,
  Palmtree,
  PiggyBank,
  RotateCcw,
  ShieldCheck,
  TrendingUp,
} from "lucide-react";
import { type ChangeEvent, type ReactNode, useState } from "react";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Field, Input, Select } from "../components/ui/field";
import { Badge, Segmented } from "../components/ui/segmented";
import { carVsApp } from "../lib/calculators/car-vs-app";
import { cdbVsSavings } from "../lib/calculators/cdb";
import { monthlyToAnnualPct, type RatePeriod, toMonthlyRate } from "../lib/calculators/common";
import { compoundInterest } from "../lib/calculators/compound";
import { cashVsInstallments, debtPayoff } from "../lib/calculators/debt";
import { type AmortizationPlan, priceSchedule, sacSchedule } from "../lib/calculators/financing";
import { emergencyFund, financialIndependence } from "../lib/calculators/goals";
import { rentVsBuy } from "../lib/calculators/rent-vs-buy";
import { type ExpenseFrequency, smallExpenses } from "../lib/calculators/small-expenses";
import {
  DEPENDENT_DEDUCTION_REFERENCE,
  INSS_REFERENCE,
  IRRF_REFERENCE,
  thirteenthSalary,
} from "../lib/calculators/thirteenth";
import { cn } from "../lib/cn";
import { formatBRLPlain as brl, centsToInput, formatPercent, parseBRLInput } from "../lib/format";

// ───────────────────────── infraestrutura dos formulários ─────────────────────────

type Bind = {
  value: string;
  onChange: (e: ChangeEvent<HTMLInputElement | HTMLSelectElement>) => void;
};

/** Campos como texto (igual aos formulários do app); a conversão acontece no cálculo. */
function useFields<T extends Record<string, string>>(initial: T) {
  const [values, setValues] = useState(initial);
  const bind = (key: keyof T & string): Bind => ({
    value: values[key] ?? "",
    onChange: (e) => setValues((v) => ({ ...v, [key]: e.target.value })),
  });
  return { values, bind };
}

/** "1.234,56" → centavos; vazio, zero ou inválido → 0. */
const money = (raw: string): number => parseBRLInput(raw) ?? 0;

/** "1,5" | "12" → número; inválido → 0. */
const num = (raw: string): number => {
  const n = Number(raw.trim().replace(",", "."));
  return Number.isFinite(n) ? n : 0;
};

const clampInt = (value: number, min: number, max: number): number =>
  Math.min(max, Math.max(min, Math.floor(value)));

const pctFmt = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 2 });
const pct2 = (value: number): string => `${pctFmt.format(value)}%`;

/** 27 → "2 anos e 3 meses". */
function formatDuration(months: number): string {
  if (months <= 0) return "já alcançado";
  const years = Math.floor(months / 12);
  const rest = months % 12;
  const y = years > 0 ? `${years} ${years === 1 ? "ano" : "anos"}` : "";
  const m = rest > 0 ? `${rest} ${rest === 1 ? "mês" : "meses"}` : "";
  return y && m ? `${y} e ${m}` : y || m;
}

const REFERENCE_NOTE = "valores de referência — confira a tabela vigente";

// ───────────────────────── blocos visuais ─────────────────────────

function Calc({
  id,
  icon: Icon,
  title,
  intro,
  children,
}: {
  id: string;
  icon: LucideIcon;
  title: string;
  intro: string;
  children: ReactNode;
}) {
  return (
    <Card
      id={id}
      data-testid={id}
      className="flex min-w-0 scroll-mt-6 flex-col gap-4 p-4 sm:p-5"
      aria-labelledby={`${id}-title`}
    >
      <header className="flex items-start gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-emerald-500/12 text-income">
          <Icon size={19} aria-hidden />
        </span>
        <div className="min-w-0">
          <h2 id={`${id}-title`} className="text-base font-semibold text-app-text">
            {title}
          </h2>
          <p className="mt-0.5 text-sm text-app-muted">{intro}</p>
        </div>
      </header>
      {children}
    </Card>
  );
}

function Fields({ children, cols = 3 }: { children: ReactNode; cols?: 2 | 3 }) {
  return (
    <div className={cn("grid grid-cols-2 gap-3", cols === 3 ? "sm:grid-cols-3" : "")}>
      {children}
    </div>
  );
}

function MoneyField({ label, bind }: { label: string; bind: Bind }) {
  return (
    <Field label={label} className="min-w-0">
      <Input inputMode="decimal" autoComplete="off" {...bind} />
    </Field>
  );
}

function NumberField({ label, bind }: { label: string; bind: Bind }) {
  return (
    <Field label={label} className="min-w-0">
      <Input inputMode="decimal" autoComplete="off" {...bind} />
    </Field>
  );
}

function Stat({
  label,
  value,
  hint,
  tone = "neutral",
  testId,
}: {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  tone?: "neutral" | "income" | "expense" | "warn";
  testId?: string;
}) {
  return (
    <div className="min-w-0 rounded-(--radius-control) bg-app-background/60 px-3 py-2.5">
      <dt className="text-xs text-app-muted">{label}</dt>
      <dd
        className={cn(
          "tnum mt-1 text-base font-semibold break-words",
          tone === "income" && "text-income",
          tone === "expense" && "text-expense",
          tone === "warn" && "text-warn",
          tone === "neutral" && "text-app-text",
        )}
        data-testid={testId}
      >
        {value}
      </dd>
      {hint ? <dd className="mt-0.5 text-[11px] leading-tight text-app-muted">{hint}</dd> : null}
    </div>
  );
}

function Stats({ children }: { children: ReactNode }) {
  return <dl className="grid grid-cols-2 gap-2 sm:grid-cols-3">{children}</dl>;
}

function Verdict({
  tone,
  children,
  testId,
}: {
  tone: "income" | "warn" | "expense" | "neutral";
  children: ReactNode;
  testId?: string;
}) {
  const box = {
    income: "border-emerald-500/30 bg-emerald-500/8",
    warn: "border-amber-400/40 bg-amber-400/8",
    expense: "border-rose-500/40 bg-rose-500/8",
    neutral: "border-app-border bg-app-background/60",
  }[tone];
  return (
    <p
      className={cn("rounded-(--radius-control) border p-3 text-sm text-app-text", box)}
      data-testid={testId}
      aria-live="polite"
    >
      {children}
    </p>
  );
}

function Assumptions({ items }: { items: ReactNode[] }) {
  return (
    <details className="group rounded-(--radius-control) border border-app-border/70 px-3">
      <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between text-sm font-medium text-app-text">
        Premissas do cálculo
        <span className="text-app-muted group-open:rotate-45" aria-hidden>
          +
        </span>
      </summary>
      <ul className="list-disc space-y-1 pb-3 pl-4 text-xs text-app-muted">
        {items.map((item, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: lista estática de textos
          <li key={i}>{item}</li>
        ))}
      </ul>
    </details>
  );
}

function Invalid({ children }: { children: ReactNode }) {
  return <p className="text-sm text-app-muted">{children}</p>;
}

const RATE_PERIODS = [
  ["MONTH", "% ao mês"],
  ["YEAR", "% ao ano"],
] as const;

// ───────────────────────── página ─────────────────────────

const CALCULATORS = [
  { id: "calc-rent-buy", label: "Alugar × comprar" },
  { id: "calc-car-app", label: "Carro × app" },
  { id: "calc-compound", label: "Juros compostos" },
  { id: "calc-small", label: "Pequenos gastos" },
  { id: "calc-13", label: "13º salário" },
  { id: "calc-emergency", label: "Reserva de emergência" },
  { id: "calc-fire", label: "Independência financeira" },
  { id: "calc-debt", label: "Quitar dívidas" },
  { id: "calc-price-sac", label: "Price × SAC" },
  { id: "calc-cash-installments", label: "À vista × parcelado" },
  { id: "calc-cdb", label: "CDB × poupança" },
] as const;

export function CalculatorsPage() {
  const goTo = (id: string) => (e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault();
    document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold tracking-tight">Calculadoras</h1>
        <p className="text-sm text-app-muted">
          Simule decisões antes de tomar: as contas rodam aqui mesmo, com premissas que você pode
          mudar
        </p>
      </header>

      <nav aria-label="Calculadoras disponíveis">
        <ul className="flex flex-wrap gap-2">
          {CALCULATORS.map((c) => (
            <li key={c.id}>
              <a
                href={`#${c.id}`}
                onClick={goTo(c.id)}
                className="inline-flex min-h-9 items-center rounded-full border border-app-border px-3 text-xs font-medium text-app-text transition-colors hover:border-emerald-500/50 hover:bg-emerald-500/8"
              >
                {c.label}
              </a>
            </li>
          ))}
        </ul>
      </nav>

      <div className="grid items-start gap-4 xl:grid-cols-2">
        <RentVsBuyCalc />
        <CarVsAppCalc />
        <CompoundCalc />
        <SmallExpensesCalc />
        <ThirteenthCalc />
        <EmergencyCalc />
        <FireCalc />
        <DebtCalc />
        <PriceSacCalc />
        <CashVsInstallmentsCalc />
        <CdbCalc />
      </div>
    </div>
  );
}

// ───────────────────────── 1. aluguel × compra ─────────────────────────

function RentVsBuyCalc() {
  const { values: v, bind } = useFields({
    property: "500.000,00",
    down: "100.000,00",
    closing: "4",
    finRate: "11,5",
    term: "30",
    rent: "2.000,00",
    rentInc: "5",
    appr: "5",
    invest: "10",
    maint: "0,5",
    horizon: "30",
  });
  const property = money(v.property);
  const down = money(v.down);
  const valid = property > 0 && down <= property && num(v.term) > 0 && num(v.horizon) > 0;
  const r = valid
    ? rentVsBuy({
        propertyCents: property,
        downPaymentCents: down,
        closingCostsPct: num(v.closing),
        financingYearPct: num(v.finRate),
        termYears: clampInt(num(v.term), 1, 35),
        rentCents: money(v.rent),
        rentIncreaseYearPct: num(v.rentInc),
        appreciationYearPct: num(v.appr),
        investmentYearPct: num(v.invest),
        maintenanceYearPct: num(v.maint),
        horizonYears: clampInt(num(v.horizon), 1, 50),
      })
    : null;
  const years = clampInt(num(v.horizon), 1, 50);

  return (
    <Calc
      id="calc-rent-buy"
      icon={House}
      title="Alugar ou comprar"
      intro="Financiar o imóvel ou morar de aluguel e investir a diferença: quem termina com mais patrimônio?"
    >
      <Fields>
        <MoneyField label="Valor do imóvel" bind={bind("property")} />
        <MoneyField label="Entrada" bind={bind("down")} />
        <NumberField label="Custos de compra (%)" bind={bind("closing")} />
        <NumberField label="Juros do financiamento (% a.a.)" bind={bind("finRate")} />
        <NumberField label="Prazo do financiamento (anos)" bind={bind("term")} />
        <MoneyField label="Aluguel mensal" bind={bind("rent")} />
        <NumberField label="Reajuste do aluguel (% a.a.)" bind={bind("rentInc")} />
        <NumberField label="Valorização do imóvel (% a.a.)" bind={bind("appr")} />
        <NumberField label="Rendimento dos investimentos (% a.a.)" bind={bind("invest")} />
        <NumberField label="Manutenção e IPTU (% a.a.)" bind={bind("maint")} />
        <NumberField label="Comparar ao longo de (anos)" bind={bind("horizon")} />
      </Fields>
      {r ? (
        <>
          <Stats>
            <Stat label="1ª parcela (Price)" value={brl(r.installmentCents)} />
            <Stat label="Juros pagos no período" value={brl(r.totalInterestCents)} />
            <Stat label="Imóvel no fim" value={brl(r.propertyValueCents)} />
            <Stat
              label="Patrimônio comprando"
              value={brl(r.buyerNetCents)}
              hint="imóvel − dívida + investimentos"
              testId="rent-buy-buyer"
            />
            <Stat
              label="Patrimônio alugando"
              value={brl(r.renterNetCents)}
              hint="entrada e diferenças investidas"
              testId="rent-buy-renter"
            />
          </Stats>
          <Verdict tone={r.winner === "TIE" ? "neutral" : "income"} testId="rent-buy-verdict">
            {r.winner === "TIE"
              ? `Em ${years} anos, as duas escolhas terminam praticamente empatadas.`
              : `Em ${years} anos, ${r.winner === "BUY" ? "comprar" : "alugar e investir a diferença"} deixa você com ${brl(r.differenceCents)} a mais.`}
          </Verdict>
        </>
      ) : (
        <Invalid>Informe o valor do imóvel e uma entrada menor que ele.</Invalid>
      )}
      <Assumptions
        items={[
          "Os dois cenários gastam o mesmo por mês: quem tem o custo menor investe a diferença.",
          "Quem aluga começa investindo a entrada e os custos de compra (ITBI, escritura, registro).",
          "Financiamento pela tabela Price; aluguel reajustado uma vez por ano.",
          "Condomínio fica de fora: é pago nos dois casos.",
          "Taxas anuais convertidas para mensais por juros compostos; sem impostos sobre os rendimentos.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 2. carro × app ─────────────────────────

function CarVsAppCalc() {
  const { values: v, bind } = useFields({
    price: "80.000,00",
    dep: "10",
    opp: "10",
    insurance: "3.000,00",
    ipva: "4",
    maint: "2.000,00",
    parking: "0",
    km: "800",
    kmpl: "12",
    fuel: "6,00",
    app: "2,50",
  });
  const r = carVsApp({
    carPriceCents: money(v.price),
    depreciationYearPct: num(v.dep),
    opportunityYearPct: num(v.opp),
    insuranceYearCents: money(v.insurance),
    ipvaPct: num(v.ipva),
    maintenanceYearCents: money(v.maint),
    parkingMonthCents: money(v.parking),
    kmPerMonth: Math.max(0, num(v.km)),
    kmPerLiter: num(v.kmpl),
    fuelPriceCents: money(v.fuel),
    appCostPerKmCents: money(v.app),
  });
  const carMore = r.differenceMonthlyCents > 0;

  return (
    <Calc
      id="calc-car-app"
      icon={Car}
      title="Carro próprio ou app de transporte"
      intro="Quanto custa ter um carro por mês, contando o que não aparece na bomba de combustível."
    >
      <Fields>
        <MoneyField label="Valor do carro" bind={bind("price")} />
        <NumberField label="Depreciação (% a.a.)" bind={bind("dep")} />
        <NumberField label="Rendimento do dinheiro (% a.a.)" bind={bind("opp")} />
        <MoneyField label="Seguro por ano" bind={bind("insurance")} />
        <NumberField label="IPVA (% do valor)" bind={bind("ipva")} />
        <MoneyField label="Manutenção por ano" bind={bind("maint")} />
        <MoneyField label="Estacionamento por mês" bind={bind("parking")} />
        <NumberField label="Km rodados por mês" bind={bind("km")} />
        <NumberField label="Consumo (km/l)" bind={bind("kmpl")} />
        <MoneyField label="Combustível (R$/l)" bind={bind("fuel")} />
        <MoneyField label="App de transporte (R$/km)" bind={bind("app")} />
      </Fields>
      <Stats>
        <Stat label="Carro por mês" value={brl(r.carMonthlyCents)} testId="car-monthly" />
        <Stat label="App por mês" value={brl(r.appMonthlyCents)} testId="app-monthly" />
        <Stat
          label="Empata em"
          value={r.breakEvenKm === null ? "—" : `${r.breakEvenKm.toLocaleString("pt-BR")} km/mês`}
          hint="acima disso o carro compensa"
        />
      </Stats>
      <Verdict tone={carMore ? "warn" : "income"}>
        {r.differenceMonthlyCents === 0
          ? "Os dois custam o mesmo por mês."
          : carMore
            ? `O carro custa ${brl(r.differenceMonthlyCents)} a mais por mês (${brl(r.differenceMonthlyCents * 12)} por ano).`
            : `O carro sai ${brl(-r.differenceMonthlyCents)} mais barato por mês que o app.`}
      </Verdict>
      <ul className="space-y-1 text-sm">
        {r.items.map((item) => (
          <li key={item.label} className="flex justify-between gap-3 text-app-muted">
            <span className="min-w-0">{item.label}</span>
            <span className="tnum shrink-0 text-app-text">{brl(item.monthlyCents)}</span>
          </li>
        ))}
      </ul>
      <Assumptions
        items={[
          "Custos do 1º ano, divididos por 12; depreciação e IPVA sobre o valor atual do carro.",
          "O dinheiro parado no carro deixaria de render: entra como custo de oportunidade simples.",
          "No app, o custo por km já inclui as tarifas médias das corridas.",
          "O ponto de empate considera que os custos fixos do carro existem mesmo sem rodar.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 3. juros compostos ─────────────────────────

function CompoundCalc() {
  const { values: v, bind } = useFields({
    initial: "1.000,00",
    monthly: "0",
    rate: "1",
    months: "12",
  });
  const [period, setPeriod] = useState<RatePeriod>("MONTH");
  const months = clampInt(num(v.months), 0, 1200);
  const r = compoundInterest({
    initialCents: money(v.initial),
    monthlyCents: money(v.monthly),
    ratePct: num(v.rate),
    ratePeriod: period,
    months,
  });
  const monthlyPct = toMonthlyRate(num(v.rate), period) * 100;

  return (
    <Calc
      id="calc-compound"
      icon={TrendingUp}
      title="Juros compostos"
      intro="Quanto um valor inicial e aportes mensais viram com o rendimento sobre rendimento."
    >
      <Segmented
        label="Período da taxa"
        value={period}
        onChange={setPeriod}
        options={RATE_PERIODS}
        className="sm:max-w-xs"
      />
      <Fields cols={2}>
        <MoneyField label="Valor inicial" bind={bind("initial")} />
        <MoneyField label="Aporte mensal" bind={bind("monthly")} />
        <NumberField
          label={period === "MONTH" ? "Taxa (% a.m.)" : "Taxa (% a.a.)"}
          bind={bind("rate")}
        />
        <NumberField label="Prazo (meses)" bind={bind("months")} />
      </Fields>
      <Stats>
        <Stat label="Valor final" value={brl(r.finalCents)} tone="income" testId="compound-final" />
        <Stat label="Total investido" value={brl(r.investedCents)} />
        <Stat label="Juros ganhos" value={brl(r.interestCents)} testId="compound-interest" />
      </Stats>
      {r.yearly.length > 0 ? (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[260px] text-sm">
            <thead>
              <tr className="text-left text-xs text-app-muted">
                <th scope="col" className="py-1.5 font-medium">
                  Mês
                </th>
                <th scope="col" className="py-1.5 text-right font-medium">
                  Investido
                </th>
                <th scope="col" className="py-1.5 text-right font-medium">
                  Saldo
                </th>
              </tr>
            </thead>
            <tbody className="tnum divide-y divide-app-border/60">
              {r.yearly.slice(0, 40).map((row) => (
                <tr key={row.month}>
                  <th scope="row" className="py-1.5 text-left font-normal text-app-muted">
                    {row.month}
                  </th>
                  <td className="py-1.5 text-right text-app-text">{brl(row.investedCents)}</td>
                  <td className="py-1.5 text-right font-medium text-app-text">
                    {brl(row.balanceCents)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
      <Assumptions
        items={[
          `Taxa equivalente: ${pct2(monthlyPct)} ao mês = ${pct2(monthlyToAnnualPct(monthlyPct))} ao ano.`,
          "Aportes no fim de cada mês; rendimento capitalizado mensalmente.",
          "Sem impostos, taxas nem inflação: use uma taxa já líquida para comparar com a realidade.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 4. pequenos gastos ─────────────────────────

const FREQUENCIES = [
  ["DAY", "por dia"],
  ["WEEK", "por semana"],
  ["MONTH", "por mês"],
] as const;

function SmallExpensesCalc() {
  const { values: v, bind } = useFields({ amount: "8,00", times: "5", invest: "10" });
  const [frequency, setFrequency] = useState<ExpenseFrequency>("WEEK");
  const r = smallExpenses({
    amountCents: money(v.amount),
    times: Math.max(0, num(v.times)),
    frequency,
    investmentYearPct: num(v.invest),
  });

  return (
    <Calc
      id="calc-small"
      icon={Coffee}
      title="Pequenos gastos"
      intro="O cafezinho, o lanche, a taxa de entrega: quanto o pouco vira em um ano e em dez."
    >
      <Fields>
        <MoneyField label="Valor de cada vez" bind={bind("amount")} />
        <NumberField label="Quantas vezes" bind={bind("times")} />
        <Field label="Frequência" className="min-w-0">
          <Select
            value={frequency}
            onChange={(e) => setFrequency(e.target.value as ExpenseFrequency)}
          >
            {FREQUENCIES.map(([key, label]) => (
              <option key={key} value={key}>
                {label}
              </option>
            ))}
          </Select>
        </Field>
        <NumberField label="Se investido (% a.a.)" bind={bind("invest")} />
      </Fields>
      <Stats>
        <Stat label="Por mês" value={brl(r.perMonthCents)} />
        <Stat label="Por ano" value={brl(r.perYearCents)} testId="small-year" />
        <Stat label="Em 10 anos" value={brl(r.tenYearsCents)} />
        <Stat
          label="Investido por 10 anos"
          value={brl(r.investedTenYearsCents)}
          tone="income"
          testId="small-invested"
        />
      </Stats>
      <Assumptions
        items={[
          "Ano com 365 dias, 52 semanas ou 12 meses, conforme a frequência.",
          "Investido: o mesmo valor aplicado todo mês por 120 meses, com rendimento composto.",
          "Valores nominais, sem inflação.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 5. 13º salário ─────────────────────────

const inssToForm = () =>
  INSS_REFERENCE.map((b) => ({ upTo: centsToInput(b.upToCents), rate: String(b.ratePct) }));
const irrfToForm = () =>
  IRRF_REFERENCE.map((b) => ({
    upTo: b.upToCents === null ? "" : centsToInput(b.upToCents),
    rate: String(b.ratePct).replace(".", ","),
    ded: centsToInput(b.deductionCents),
  }));

function ThirteenthCalc() {
  const { values: v, bind } = useFields({
    salary: "3.000,00",
    months: "12",
    extras: "0",
    dependents: "0",
  });
  const [inss, setInss] = useState(inssToForm);
  const [irrfRows, setIrrfRows] = useState(irrfToForm);
  const [depDed, setDepDed] = useState(centsToInput(DEPENDENT_DEDUCTION_REFERENCE));

  const reset = () => {
    setInss(inssToForm());
    setIrrfRows(irrfToForm());
    setDepDed(centsToInput(DEPENDENT_DEDUCTION_REFERENCE));
  };

  const salary = money(v.salary);
  const r =
    salary > 0
      ? thirteenthSalary({
          salaryCents: salary,
          monthsWorked: clampInt(num(v.months), 0, 12),
          extrasCents: money(v.extras),
          dependents: clampInt(num(v.dependents), 0, 20),
          inss: inss.map((row) => ({ upToCents: money(row.upTo), ratePct: num(row.rate) })),
          irrf: irrfRows.map((row) => ({
            upToCents: row.upTo.trim() === "" ? null : money(row.upTo),
            ratePct: num(row.rate),
            deductionCents: money(row.ded),
          })),
          dependentDeductionCents: money(depDed),
        })
      : null;

  return (
    <Calc
      id="calc-13"
      icon={Gift}
      title="13º salário"
      intro="Quanto cai em cada parcela, com INSS e Imposto de Renda descontados na 2ª."
    >
      <Fields cols={2}>
        <MoneyField label="Salário bruto mensal" bind={bind("salary")} />
        <NumberField label="Meses trabalhados no ano" bind={bind("months")} />
        <MoneyField label="Média de extras (opcional)" bind={bind("extras")} />
        <NumberField label="Dependentes" bind={bind("dependents")} />
      </Fields>
      {r ? (
        <Stats>
          <Stat label="13º bruto" value={brl(r.grossCents)} testId="thirteenth-gross" />
          <Stat label="INSS" value={brl(r.inssCents)} tone="expense" testId="thirteenth-inss" />
          <Stat
            label="Imposto de Renda"
            value={brl(r.irrfCents)}
            hint={`alíquota ${formatPercent(r.irrfRatePct)} sobre ${brl(r.irrfBaseCents)}`}
            tone="expense"
            testId="thirteenth-irrf"
          />
          <Stat label="13º líquido" value={brl(r.netCents)} tone="income" testId="thirteenth-net" />
          <Stat
            label="1ª parcela"
            value={brl(r.firstInstallmentCents)}
            hint="até 30 de novembro, sem descontos"
            testId="thirteenth-first"
          />
          <Stat
            label="2ª parcela"
            value={brl(r.secondInstallmentCents)}
            hint="até 20 de dezembro, com os descontos"
            testId="thirteenth-second"
          />
        </Stats>
      ) : (
        <Invalid>Informe o salário bruto para calcular.</Invalid>
      )}
      <p className="flex flex-wrap items-center gap-2 text-xs text-app-muted">
        <Badge tone="warn">Tabelas 2025</Badge>
        {REFERENCE_NOTE}
      </p>
      <details className="group rounded-(--radius-control) border border-app-border/70 px-3">
        <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between text-sm font-medium text-app-text">
          Editar tabelas de INSS e IRRF
          <span className="text-app-muted group-open:rotate-45" aria-hidden>
            +
          </span>
        </summary>
        <div className="space-y-4 pb-3">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[260px] text-sm">
              <caption className="pb-1 text-left text-xs font-medium text-app-muted">
                INSS (progressivo por faixa)
              </caption>
              <thead>
                <tr className="text-left text-xs text-app-muted">
                  <th scope="col" className="py-1 pr-2 font-medium">
                    Até (R$)
                  </th>
                  <th scope="col" className="py-1 font-medium">
                    Alíquota (%)
                  </th>
                </tr>
              </thead>
              <tbody>
                {inss.map((row, i) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: faixas fixas, editadas no lugar
                  <tr key={i}>
                    <td className="py-1 pr-2">
                      <Input
                        aria-label={`INSS faixa ${i + 1}: teto`}
                        inputMode="decimal"
                        className="h-10"
                        value={row.upTo}
                        onChange={(e) =>
                          setInss((rows) =>
                            rows.map((x, j) => (j === i ? { ...x, upTo: e.target.value } : x)),
                          )
                        }
                      />
                    </td>
                    <td className="py-1">
                      <Input
                        aria-label={`INSS faixa ${i + 1}: alíquota`}
                        inputMode="decimal"
                        className="h-10"
                        value={row.rate}
                        onChange={(e) =>
                          setInss((rows) =>
                            rows.map((x, j) => (j === i ? { ...x, rate: e.target.value } : x)),
                          )
                        }
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[300px] text-sm">
              <caption className="pb-1 text-left text-xs font-medium text-app-muted">
                IRRF (tributação exclusiva na fonte; última faixa sem teto)
              </caption>
              <thead>
                <tr className="text-left text-xs text-app-muted">
                  <th scope="col" className="py-1 pr-2 font-medium">
                    Até (R$)
                  </th>
                  <th scope="col" className="py-1 pr-2 font-medium">
                    Alíquota (%)
                  </th>
                  <th scope="col" className="py-1 font-medium">
                    Dedução (R$)
                  </th>
                </tr>
              </thead>
              <tbody>
                {irrfRows.map((row, i) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: faixas fixas, editadas no lugar
                  <tr key={i}>
                    {(["upTo", "rate", "ded"] as const).map((key) => (
                      <td key={key} className={cn("py-1", key !== "ded" && "pr-2")}>
                        <Input
                          aria-label={`IRRF faixa ${i + 1}: ${key === "upTo" ? "teto" : key === "rate" ? "alíquota" : "dedução"}`}
                          inputMode="decimal"
                          className="h-10"
                          placeholder={key === "upTo" ? "sem teto" : undefined}
                          value={row[key]}
                          onChange={(e) =>
                            setIrrfRows((rows) =>
                              rows.map((x, j) => (j === i ? { ...x, [key]: e.target.value } : x)),
                            )
                          }
                        />
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="flex flex-wrap items-end gap-3">
            <Field label="Dedução por dependente (R$)" className="min-w-0 flex-1">
              <Input
                inputMode="decimal"
                value={depDed}
                onChange={(e) => setDepDed(e.target.value)}
              />
            </Field>
            <Button variant="outline" onClick={reset}>
              <RotateCcw size={15} aria-hidden /> Restaurar referência
            </Button>
          </div>
        </div>
      </details>
      <Assumptions
        items={[
          "13º proporcional: (salário + média de extras) × meses trabalhados ÷ 12; 15 dias ou mais contam como mês.",
          "A 1ª parcela é metade do bruto, sem descontos; INSS e IR saem todos na 2ª.",
          "INSS progressivo: cada alíquota incide só sobre a parte do valor dentro da faixa, até o teto.",
          "IR do 13º é exclusivo na fonte, separado do salário: base = bruto − INSS − dependentes.",
          "Reduções e isenções criadas depois destas tabelas não estão incluídas: ajuste as faixas se mudaram.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 6. reserva de emergência ─────────────────────────

function EmergencyCalc() {
  const { values: v, bind } = useFields({
    cost: "4.000,00",
    months: "6",
    saved: "0",
    saving: "500,00",
    yield: "10",
  });
  const cost = money(v.cost);
  const r =
    cost > 0
      ? emergencyFund({
          monthlyCostCents: cost,
          months: clampInt(num(v.months), 1, 60),
          savedCents: money(v.saved),
          monthlySavingCents: money(v.saving),
          yieldYearPct: num(v.yield),
        })
      : null;

  return (
    <Calc
      id="calc-emergency"
      icon={ShieldCheck}
      title="Reserva de emergência"
      intro="O tamanho ideal da sua reserva e em quanto tempo você chega lá."
    >
      <Fields>
        <MoneyField label="Custo de vida mensal" bind={bind("cost")} />
        <NumberField label="Meses de proteção" bind={bind("months")} />
        <MoneyField label="Já guardado" bind={bind("saved")} />
        <MoneyField label="Quanto guarda por mês" bind={bind("saving")} />
        <NumberField label="Rendimento (% a.a.)" bind={bind("yield")} />
      </Fields>
      {r ? (
        <Stats>
          <Stat label="Reserva ideal" value={brl(r.targetCents)} testId="emergency-target" />
          <Stat label="Falta guardar" value={brl(r.missingCents)} />
          <Stat
            label="Prazo"
            value={r.monthsToTarget === null ? "não chega" : formatDuration(r.monthsToTarget)}
            tone={r.monthsToTarget === null ? "expense" : "income"}
            testId="emergency-months"
          />
          <Stat
            label="Cobre hoje"
            value={`${pctFmt.format(Math.floor(r.coveredMonths * 10) / 10)} meses`}
          />
        </Stats>
      ) : (
        <Invalid>Informe seu custo de vida mensal.</Invalid>
      )}
      <Assumptions
        items={[
          "Regra comum: 6 meses de custo de vida; 12 se você tem dependentes ou renda instável.",
          "Reserva em aplicação de liquidez diária; rendimento composto mensal sobre o saldo.",
          "Aportes no fim de cada mês; prazo máximo simulado de 50 anos.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 7. independência financeira ─────────────────────────

function FireCalc() {
  const { values: v, bind } = useFields({
    spend: "5.000,00",
    withdrawal: "4",
    current: "0",
    contrib: "2.000,00",
    real: "4",
  });
  const spend = money(v.spend);
  const withdrawal = num(v.withdrawal);
  const r =
    spend > 0 && withdrawal > 0
      ? financialIndependence({
          monthlySpendCents: spend,
          withdrawalPct: withdrawal,
          currentCents: money(v.current),
          monthlyContributionCents: money(v.contrib),
          realReturnYearPct: num(v.real),
        })
      : null;

  return (
    <Calc
      id="calc-fire"
      icon={Palmtree}
      title="Independência financeira"
      intro="Quanto patrimônio você precisa para viver de renda e quanto tempo falta."
    >
      <Fields>
        <MoneyField label="Gasto mensal desejado" bind={bind("spend")} />
        <NumberField label="Taxa de retirada (% a.a.)" bind={bind("withdrawal")} />
        <MoneyField label="Patrimônio atual" bind={bind("current")} />
        <MoneyField label="Aporte mensal" bind={bind("contrib")} />
        <NumberField label="Rendimento real (% a.a.)" bind={bind("real")} />
      </Fields>
      {r ? (
        <Stats>
          <Stat label="Patrimônio necessário" value={brl(r.targetCents)} testId="fire-target" />
          <Stat label="Você já tem" value={formatPercent(r.progressPct)} />
          <Stat
            label="Prazo"
            value={
              r.monthsToTarget === null ? "mais de 100 anos" : formatDuration(r.monthsToTarget)
            }
            tone={r.monthsToTarget === null ? "expense" : "income"}
          />
          <Stat
            label="Renda que já sustenta"
            value={`${brl(r.currentIncomeCents)}/mês`}
            hint="pela mesma regra de retirada"
          />
        </Stats>
      ) : (
        <Invalid>Informe o gasto mensal e uma taxa de retirada maior que zero.</Invalid>
      )}
      <Assumptions
        items={[
          "Patrimônio necessário = gasto anual ÷ taxa de retirada (regra dos 4% → 25 vezes o gasto anual).",
          "Use rendimento real (acima da inflação): os valores ficam em dinheiro de hoje.",
          "A taxa de retirada é uma referência histórica, não uma garantia; mais conservador = taxa menor.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 8. quitar dívidas ─────────────────────────

function DebtCalc() {
  const { values: v, bind } = useFields({
    balance: "10.000,00",
    rate: "3",
    payment: "500,00",
    extra: "200,00",
  });
  const balance = money(v.balance);
  const r = balance > 0 ? debtPayoff(balance, num(v.rate), money(v.payment), money(v.extra)) : null;

  return (
    <Calc
      id="calc-debt"
      icon={HandCoins}
      title="Quitar dívidas"
      intro="Em quanto tempo a dívida acaba e quanto um valor extra por mês economiza de juros."
    >
      <Fields cols={2}>
        <MoneyField label="Saldo devedor" bind={bind("balance")} />
        <NumberField label="Juros (% a.m.)" bind={bind("rate")} />
        <MoneyField label="Pagamento mensal" bind={bind("payment")} />
        <MoneyField label="Extra por mês" bind={bind("extra")} />
      </Fields>
      {!r ? (
        <Invalid>Informe o saldo devedor.</Invalid>
      ) : !r.base ? (
        <Verdict tone="expense">
          O pagamento mensal não cobre nem os juros do mês: a dívida só cresce. Aumente o pagamento
          ou renegocie a taxa.
        </Verdict>
      ) : (
        <>
          <Stats>
            <Stat
              label="Sem extra"
              value={formatDuration(r.base.months)}
              hint={`${brl(r.base.totalInterestCents)} de juros`}
              testId="debt-base"
            />
            <Stat
              label="Com extra"
              value={r.withExtra ? formatDuration(r.withExtra.months) : "—"}
              hint={r.withExtra ? `${brl(r.withExtra.totalInterestCents)} de juros` : undefined}
              testId="debt-extra"
            />
            <Stat
              label="Economia de juros"
              value={brl(r.interestSavedCents ?? 0)}
              tone="income"
              testId="debt-saved"
            />
          </Stats>
          {r.monthsSaved ? (
            <Verdict tone="income">
              Pagando {brl(money(v.extra))} a mais por mês, você termina{" "}
              {formatDuration(r.monthsSaved)} antes.
            </Verdict>
          ) : null}
        </>
      )}
      <Assumptions
        items={[
          "Juros compostos mensais sobre o saldo; o pagamento abate depois dos juros do mês.",
          "Com várias dívidas, pague o mínimo em todas e jogue o extra na de maior juros (avalanche).",
          "Para suas dívidas cadastradas, a página Dívidas compara as estratégias de quitação.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 9. Price × SAC ─────────────────────────

function PriceSacCalc() {
  const { values: v, bind } = useFields({ principal: "100.000,00", rate: "1", months: "12" });
  const [period, setPeriod] = useState<RatePeriod>("MONTH");
  const [schedule, setSchedule] = useState<"PRICE" | "SAC">("PRICE");
  const principal = money(v.principal);
  const months = clampInt(num(v.months), 0, 600);
  const rate = toMonthlyRate(num(v.rate), period);
  const valid = principal > 0 && months > 0;
  const price = valid ? priceSchedule(principal, rate, months) : null;
  const sac = valid ? sacSchedule(principal, rate, months) : null;
  const shown: AmortizationPlan | null = schedule === "PRICE" ? price : sac;

  return (
    <Calc
      id="calc-price-sac"
      icon={Landmark}
      title="Financiamento: Price ou SAC"
      intro="Parcelas fixas ou decrescentes: compare a primeira e a última parcela e os juros totais."
    >
      <Segmented
        label="Período da taxa"
        value={period}
        onChange={setPeriod}
        options={RATE_PERIODS}
        className="sm:max-w-xs"
      />
      <Fields>
        <MoneyField label="Valor financiado" bind={bind("principal")} />
        <NumberField
          label={period === "MONTH" ? "Juros (% a.m.)" : "Juros (% a.a.)"}
          bind={bind("rate")}
        />
        <NumberField label="Prazo (meses)" bind={bind("months")} />
      </Fields>
      {price && sac ? (
        <>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-2">
              <h3 className="text-sm font-medium text-app-text">Price (parcela fixa)</h3>
              <dl className="grid grid-cols-2 gap-2">
                <Stat
                  label="1ª parcela"
                  value={brl(price.firstInstallmentCents)}
                  testId="price-first"
                />
                <Stat
                  label="Última parcela"
                  value={brl(price.lastInstallmentCents)}
                  testId="price-last"
                />
                <Stat
                  label="Juros totais"
                  value={brl(price.totalInterestCents)}
                  tone="expense"
                  testId="price-interest"
                />
                <Stat label="Total pago" value={brl(price.totalPaidCents)} />
              </dl>
            </div>
            <div className="space-y-2">
              <h3 className="text-sm font-medium text-app-text">SAC (parcela decrescente)</h3>
              <dl className="grid grid-cols-2 gap-2">
                <Stat
                  label="1ª parcela"
                  value={brl(sac.firstInstallmentCents)}
                  testId="sac-first"
                />
                <Stat
                  label="Última parcela"
                  value={brl(sac.lastInstallmentCents)}
                  testId="sac-last"
                />
                <Stat
                  label="Juros totais"
                  value={brl(sac.totalInterestCents)}
                  tone="expense"
                  testId="sac-interest"
                />
                <Stat label="Total pago" value={brl(sac.totalPaidCents)} />
              </dl>
            </div>
          </div>
          <Verdict tone="neutral">
            {price.totalInterestCents > sac.totalInterestCents
              ? `No SAC você paga ${brl(price.totalInterestCents - sac.totalInterestCents)} a menos de juros, mas a 1ª parcela é ${brl(sac.firstInstallmentCents - price.firstInstallmentCents)} maior.`
              : "Sem juros, Price e SAC dão o mesmo resultado."}
          </Verdict>
          <details className="group rounded-(--radius-control) border border-app-border/70 px-3">
            <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between text-sm font-medium text-app-text">
              Ver tabela de amortização ({months} parcelas)
              <span className="text-app-muted group-open:rotate-45" aria-hidden>
                +
              </span>
            </summary>
            <div className="space-y-3 pb-3">
              <Segmented
                label="Sistema exibido"
                value={schedule}
                onChange={setSchedule}
                options={[
                  ["PRICE", "Price"],
                  ["SAC", "SAC"],
                ]}
                className="sm:max-w-xs"
              />
              <div className="max-h-96 overflow-auto">
                <table className="w-full min-w-[420px] text-sm" data-testid="amortization-table">
                  <thead className="sticky top-0 bg-app-surface">
                    <tr className="text-left text-xs text-app-muted">
                      <th scope="col" className="py-1.5 pr-2 font-medium">
                        Nº
                      </th>
                      <th scope="col" className="py-1.5 pr-2 text-right font-medium">
                        Parcela
                      </th>
                      <th scope="col" className="py-1.5 pr-2 text-right font-medium">
                        Juros
                      </th>
                      <th scope="col" className="py-1.5 pr-2 text-right font-medium">
                        Amortização
                      </th>
                      <th scope="col" className="py-1.5 text-right font-medium">
                        Saldo
                      </th>
                    </tr>
                  </thead>
                  <tbody className="tnum divide-y divide-app-border/60">
                    {shown?.rows.map((row) => (
                      <tr key={row.number}>
                        <th
                          scope="row"
                          className="py-1.5 pr-2 text-left font-normal text-app-muted"
                        >
                          {row.number}
                        </th>
                        <td className="py-1.5 pr-2 text-right text-app-text">
                          {brl(row.installmentCents)}
                        </td>
                        <td className="py-1.5 pr-2 text-right text-app-muted">
                          {brl(row.interestCents)}
                        </td>
                        <td className="py-1.5 pr-2 text-right text-app-muted">
                          {brl(row.amortizationCents)}
                        </td>
                        <td className="py-1.5 text-right text-app-text">{brl(row.balanceCents)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </details>
        </>
      ) : (
        <Invalid>Informe o valor financiado e o prazo.</Invalid>
      )}
      <Assumptions
        items={[
          "Price: parcela constante = P·i ÷ (1 − (1+i)^−n); no começo, quase tudo é juros.",
          "SAC: amortização constante (P ÷ n) mais juros sobre o saldo; parcelas caem todo mês.",
          "Taxa anual convertida para mensal por juros compostos. Seguros e tarifas ficam de fora.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 10. à vista × parcelado ─────────────────────────

function CashVsInstallmentsCalc() {
  const { values: v, bind } = useFields({
    cash: "950,00",
    installment: "100,00",
    n: "10",
    invest: "12",
  });
  const [firstNow, setFirstNow] = useState(false);
  const cash = money(v.cash);
  const installment = money(v.installment);
  const r =
    cash > 0 && installment > 0
      ? cashVsInstallments({
          cashPriceCents: cash,
          installmentCents: installment,
          installments: clampInt(num(v.n), 1, 120),
          firstAtPurchase: firstNow,
          investmentYearPct: num(v.invest),
        })
      : null;

  return (
    <Calc
      id="calc-cash-installments"
      icon={CreditCard}
      title="À vista ou parcelado"
      intro="Com desconto à vista ou parcelado deixando o dinheiro rendendo: o que sai mais barato?"
    >
      <Fields>
        <MoneyField label="Preço à vista" bind={bind("cash")} />
        <MoneyField label="Valor da parcela" bind={bind("installment")} />
        <NumberField label="Número de parcelas" bind={bind("n")} />
        <NumberField label="Rendimento do dinheiro (% a.a.)" bind={bind("invest")} />
      </Fields>
      <label className="flex min-h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text">
        <input
          type="checkbox"
          className="h-4 w-4 accent-emerald-500"
          checked={firstNow}
          onChange={(e) => setFirstNow(e.target.checked)}
        />
        1ª parcela no ato da compra
      </label>
      {r ? (
        <>
          <Stats>
            <Stat label="Total parcelado" value={brl(r.totalInstallmentsCents)} />
            <Stat
              label="Valor presente das parcelas"
              value={brl(r.presentValueCents)}
              hint="quanto aplicar hoje para pagá-las"
              testId="cash-pv"
            />
            <Stat
              label="Juros embutidos"
              value={
                r.implicitMonthlyRatePct === null
                  ? "sem juros"
                  : `${pct2(r.implicitMonthlyRatePct)} a.m.`
              }
            />
          </Stats>
          <Verdict tone={r.best === "TIE" ? "neutral" : "income"} testId="cash-verdict">
            {r.best === "TIE"
              ? "Dá praticamente no mesmo: escolha pelo que deixa seu caixa mais tranquilo."
              : r.best === "CASH"
                ? `À vista vale mais: você economiza ${brl(r.savingCents)} em valor de hoje.`
                : `Parcelar vale mais: deixando o dinheiro rendendo, você ganha ${brl(r.savingCents)} em valor de hoje.`}
          </Verdict>
        </>
      ) : (
        <Invalid>Informe o preço à vista e o valor da parcela.</Invalid>
      )}
      <Assumptions
        items={[
          "Compara o preço à vista com o valor presente das parcelas, descontadas pelo rendimento da aplicação.",
          "Sem a 1ª parcela no ato, ela vence em 30 dias.",
          "Rendimento líquido de impostos dá uma comparação mais justa.",
        ]}
      />
    </Calc>
  );
}

// ───────────────────────── 11. CDB × poupança ─────────────────────────

function CdbCalc() {
  const { values: v, bind } = useFields({
    amount: "10.000,00",
    days: "365",
    pct: "100",
    cdi: "14,9",
    selic: "15",
    tr: "0,1",
  });
  const amount = money(v.amount);
  const days = clampInt(num(v.days), 0, 3650);
  const r =
    amount > 0 && days > 0
      ? cdbVsSavings({
          amountCents: amount,
          days,
          cdbPctOfCdi: num(v.pct),
          cdiYearPct: num(v.cdi),
          selicYearPct: num(v.selic),
          trMonthPct: num(v.tr),
        })
      : null;

  return (
    <Calc
      id="calc-cdb"
      icon={PiggyBank}
      title="CDB ou poupança"
      intro="O CDB paga Imposto de Renda e a poupança não: quem rende mais no seu prazo?"
    >
      <Fields>
        <MoneyField label="Valor aplicado" bind={bind("amount")} />
        <NumberField label="Prazo (dias corridos)" bind={bind("days")} />
        <NumberField label="CDB (% do CDI)" bind={bind("pct")} />
        <NumberField label="CDI (% a.a.)" bind={bind("cdi")} />
        <NumberField label="Selic (% a.a.)" bind={bind("selic")} />
        <NumberField label="TR (% a.m.)" bind={bind("tr")} />
      </Fields>
      {r ? (
        <>
          <Stats>
            <Stat label="CDB bruto" value={brl(r.cdbGrossCents)} />
            <Stat
              label="Imposto de Renda"
              value={formatPercent(r.irPct)}
              hint={`${brl(r.irCents)} sobre o rendimento`}
              testId="cdb-ir-rate"
            />
            <Stat label="CDB líquido" value={brl(r.cdbNetCents)} tone="income" testId="cdb-net" />
            <Stat
              label="Poupança"
              value={brl(r.savingsGainCents)}
              hint={`${pct2(r.savingsMonthlyPct)} a.m. por ${r.savingsMonths} ${r.savingsMonths === 1 ? "mês" : "meses"}`}
              tone="income"
              testId="savings-gain"
            />
            <Stat
              label="Empate"
              value={
                r.breakEvenPctOfCdi === null ? "—" : `${pctFmt.format(r.breakEvenPctOfCdi)}% do CDI`
              }
              hint="abaixo disso a poupança ganha"
            />
          </Stats>
          <Verdict tone={r.winner === "TIE" ? "neutral" : "income"} testId="cdb-verdict">
            {r.winner === "TIE"
              ? "No seu prazo, os dois rendem praticamente o mesmo."
              : r.winner === "CDB"
                ? `O CDB rende ${brl(r.cdbNetCents - r.savingsGainCents)} a mais, já descontado o IR.`
                : `A poupança rende ${brl(r.savingsGainCents - r.cdbNetCents)} a mais no seu prazo.`}
          </Verdict>
        </>
      ) : (
        <Invalid>Informe o valor e o prazo em dias.</Invalid>
      )}
      <p className="text-xs text-app-muted">Selic, CDI e TR: {REFERENCE_NOTE}.</p>
      <Assumptions
        items={[
          "IR regressivo sobre o rendimento do CDB: 22,5% até 180 dias, 20% até 360, 17,5% até 720 e 15% acima.",
          "CDB: CDI diário (base 252 dias úteis) × % do CDI; dias úteis estimados como dias corridos × 252 ÷ 365.",
          "Poupança: 0,5% ao mês + TR com Selic acima de 8,5% a.a.; senão, 70% da Selic (mensalizada) + TR.",
          "A poupança só rende em meses completos (aniversário de 30 dias) e é isenta de IR.",
          "IOF (resgates com menos de 30 dias) e garantias não entram na conta.",
        ]}
      />
    </Calc>
  );
}
