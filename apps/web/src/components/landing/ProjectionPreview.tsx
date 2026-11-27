import { TriangleAlert } from "lucide-react";
import { money } from "./money";
import { bandNames, bandOf, lowestDay, projectionDays } from "./projection";

const width = 300;
const height = 112;
const slot = width / projectionDays.length;
const max = Math.max(...projectionDays.map((d) => d.balance));
// Bars rise from a floor below the lowest balance, so the dip before payday stays visible.
const floor = Math.min(0, ...projectionDays.map((d) => d.balance)) - max * 0.14;
const y = (cents: number) => 4 + ((max - cents) / (max - floor)) * (height - 4);

export function ProjectionPreview() {
  const first = projectionDays[0];
  const last = projectionDays.at(-1);
  const label = `Saldo projetado dia a dia nos próximos 30 dias: começa em ${money(
    first?.balance ?? 0,
  )}, chega ao menor valor de ${money(lowestDay.balance)} em ${lowestDay.day} dias e termina em ${money(
    last?.balance ?? 0,
  )} depois do salário.`;
  return (
    <div className="ff-projection">
      <div className="ff-projection-alert">
        <TriangleAlert size={17} aria-hidden="true" />
        <div>
          <span>Menor saldo nos próximos 30 dias</span>
          <strong className="ff-expense">{money(lowestDay.balance)}</strong>
          <small>em {lowestDay.day} dias, um dia antes do salário</small>
        </div>
      </div>
      <svg
        className="ff-projection-chart"
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
        role="img"
        aria-label={label}
      >
        {projectionDays.map((d) => {
          const top = y(d.balance);
          const barHeight = height - top;
          return (
            <rect
              key={d.day}
              className={`is-${bandOf(d.balance)}${d.day === lowestDay.day ? " is-lowest" : ""}`}
              x={d.day * slot + 1.5}
              y={top}
              width={slot - 3}
              height={barHeight}
              rx="1.5"
            />
          );
        })}
        <line className="ff-projection-zero" x1="0" x2={width} y1={y(0)} y2={y(0)} />
      </svg>
      <div className="ff-projection-axis" aria-hidden="true">
        <span>Hoje</span>
        <span>+15 dias</span>
        <span>+30 dias</span>
      </div>
      <ul className="ff-projection-legend" aria-label="Faixas de cor do saldo">
        {bandNames.map(({ band, label: text }) => (
          <li key={band}>
            <span className={`ff-band-dot is-${band}`} aria-hidden="true" />
            {text}
          </li>
        ))}
      </ul>
    </div>
  );
}
