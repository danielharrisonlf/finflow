import { Calculator, ChartNoAxesCombined, HandCoins, PiggyBank } from "lucide-react";
import { useState } from "react";
import { money } from "./money";

// Twelve month-end net worth values (cents), December to November. Illustrative only.
const months = ["dez", "jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov"];
const netWorth = [
  4_278_000, 4_315_000, 4_261_000, 4_390_000, 4_452_000, 4_421_000, 4_538_000, 4_605_000, 4_570_000,
  4_710_000, 4_802_000, 4_892_000,
];
const parts = [
  { label: "Contas", cents: 1_240_000 },
  { label: "Investimentos", cents: 6_132_000 },
  { label: "Dívidas", cents: -2_480_000 },
];

type Strategy = "avalanche" | "snowball";
const strategies: Record<
  Strategy,
  { label: string; rule: string; order: string[]; months: number; interest: number }
> = {
  avalanche: {
    label: "Avalanche",
    rule: "Maior juro primeiro",
    order: ["Cartão parcelado", "Empréstimo pessoal", "Financiamento do carro"],
    months: 31,
    interest: 418_000,
  },
  snowball: {
    label: "Bola de neve",
    rule: "Menor saldo primeiro",
    order: ["Empréstimo pessoal", "Cartão parcelado", "Financiamento do carro"],
    months: 33,
    interest: 496_000,
  },
};

const width = 300;
const height = 96;
const slot = width / netWorth.length;
const top = Math.max(...netWorth);
const base = Math.min(...netWorth) * 0.97;
const barY = (cents: number) => 4 + ((top - cents) / (top - base)) * (height - 4);

function WealthPreview() {
  const [strategy, setStrategy] = useState<Strategy>("avalanche");
  const plan = strategies[strategy];
  const first = netWorth[0] ?? 0;
  const last = netWorth.at(-1) ?? 0;
  return (
    <div className="ff-wealth-card">
      <div className="ff-wealth-head">
        <span>Patrimônio líquido em novembro</span>
        <strong>{money(last)}</strong>
        <small className="ff-income">+ {money(last - first)} em 12 meses</small>
      </div>
      <svg
        className="ff-wealth-chart"
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
        role="img"
        aria-label={`Evolução do patrimônio líquido em 12 meses: de ${money(first)} em dezembro para ${money(last)} em novembro.`}
      >
        {netWorth.map((cents, index) => (
          <rect
            key={months[index]}
            className={index === netWorth.length - 1 ? "is-current" : undefined}
            x={index * slot + 3}
            y={barY(cents)}
            width={slot - 6}
            height={height - barY(cents)}
            rx="2"
          />
        ))}
      </svg>
      <div className="ff-wealth-axis" aria-hidden="true">
        <span>dez</span>
        <span>nov</span>
      </div>
      <dl className="ff-wealth-parts">
        {parts.map((part) => (
          <div key={part.label}>
            <dt>{part.label}</dt>
            <dd className={part.cents < 0 ? "ff-expense" : undefined}>
              {part.cents < 0 ? `− ${money(-part.cents)}` : money(part.cents)}
            </dd>
          </div>
        ))}
      </dl>
      <div className="ff-wealth-debt">
        <div className="ff-wealth-debt-head">
          <strong>Quitar as dívidas com R$ 300 a mais por mês</strong>
          <fieldset className="ff-wealth-toggle" aria-label="Estratégia de quitação">
            {(Object.keys(strategies) as Strategy[]).map((id) => (
              <button
                key={id}
                type="button"
                aria-pressed={strategy === id}
                onClick={() => setStrategy(id)}
              >
                {strategies[id].label}
              </button>
            ))}
          </fieldset>
        </div>
        <div className="ff-wealth-plan" aria-live="polite">
          <p>
            <span>{plan.rule}:</span> quitadas em <strong>{plan.months} meses</strong>, com{" "}
            <strong>{money(plan.interest)}</strong> de juros.
          </p>
          <ol aria-label="Ordem de quitação">
            {plan.order.map((name) => (
              <li key={name}>{name}</li>
            ))}
          </ol>
          <small>Pagando só as parcelas: 41 meses e R$ 6.870,00 de juros.</small>
        </div>
      </div>
    </div>
  );
}

const items = [
  {
    icon: PiggyBank,
    title: "Investimentos",
    text: "Renda fixa, Tesouro, ações, fundos e previdência num só lugar, com rentabilidade e divisão por tipo e instituição.",
  },
  {
    icon: HandCoins,
    title: "Dívidas e financiamentos",
    text: "Parcelas, juros e saldo devedor de cada contrato, e a ordem de quitação que economiza mais.",
  },
  {
    icon: ChartNoAxesCombined,
    title: "Evolução do patrimônio",
    text: "O que você tem menos o que deve, no fim de cada mês, para ver se está andando para a frente.",
  },
];

export const calculators = [
  "Alugar ou comprar",
  "Carro próprio ou app de transporte",
  "Juros compostos",
  "Pequenos gastos",
  "13º salário",
  "Reserva de emergência",
  "Independência financeira",
  "Quitar dívidas",
  "Financiamento Price × SAC",
  "À vista ou parcelado",
  "CDB × poupança",
];

export function PlanningSection() {
  return (
    <section className="ff-section ff-wealth ff-container" id="patrimonio">
      <div className="ff-wealth-copy">
        <span className="ff-eyebrow">Patrimônio</span>
        <h2>
          Do gasto de hoje
          <br />
          ao patrimônio de amanhã.
        </h2>
        <p>
          Investimentos, dívidas e o que sobra entre eles, mês a mês. Tudo calculado no seu
          computador.
        </p>
        <ul className="ff-sheets-list">
          {items.map(({ icon: Icon, title, text }) => (
            <li key={title}>
              <span className="ff-feature-icon">
                <Icon size={18} aria-hidden="true" />
              </span>
              <div>
                <strong>{title}</strong>
                <p>{text}</p>
              </div>
            </li>
          ))}
        </ul>
      </div>
      <figure className="ff-wealth-figure">
        <WealthPreview />
        <figcaption className="ff-fineprint">Valores ilustrativos.</figcaption>
      </figure>
      <div className="ff-calculators">
        <h3>
          <Calculator size={18} aria-hidden="true" />
          {calculators.length} calculadoras para decidir com calma
        </h3>
        <ul aria-label="Calculadoras incluídas">
          {calculators.map((name) => (
            <li key={name}>{name}</li>
          ))}
        </ul>
        <p className="ff-fineprint">
          Premissas editáveis e explicadas. Os valores ficam no seu navegador.
        </p>
      </div>
    </section>
  );
}
