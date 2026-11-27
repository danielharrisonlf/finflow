import { type ReactNode, useId, useState } from "react";
import { money } from "./money";

export interface FinancePoint {
  day: string;
  income: number;
  expense: number;
  balance: number;
}

export function FinanceChart({
  points,
  period,
  children,
}: {
  points: FinancePoint[];
  period: string;
  children?: ReactNode;
}) {
  const [view, setView] = useState<"balance" | "comparison">("balance");
  const [selected, setSelected] = useState(3);
  const id = useId();
  const current = points[selected] ?? points[points.length - 1];
  const balanceView = view === "balance";
  const max =
    Math.ceil(
      Math.max(...points.flatMap((p) => (balanceView ? [p.balance] : [p.income, p.expense]))) /
        500000,
    ) * 500000;
  const x = (index: number) => 24 + index * 184;
  const y = (value: number) => 132 - (value / max) * 120;
  const line = points.map((p, i) => `${x(i)},${y(p.balance)}`).join(" ");
  const month = period === "Outubro" ? "out" : "nov";

  return (
    <section className="cashflow" aria-label="Evolução das finanças">
      <div className="panel-heading">
        <h3>{balanceView ? "Evolução do saldo" : "Entradas e saídas"}</h3>
        <span>Em reais</span>
      </div>
      <div className="ff-chart-plot">
        <div className="ff-chart-axis" aria-hidden="true">
          {[max, max / 2, 0].map((value) => (
            <span key={value}>
              {value === 0 ? "0" : `${(value / 100000).toLocaleString("pt-BR")} mil`}
            </span>
          ))}
        </div>
        <svg
          viewBox="0 0 600 144"
          preserveAspectRatio="none"
          role="img"
          aria-label={`${balanceView ? "Evolução do saldo" : "Entradas e saídas"} de ${period}. ${points.map((p) => `Dia ${p.day}: saldo ${money(p.balance)}, entradas ${money(p.income)}, saídas ${money(p.expense)}`).join("; ")}.`}
        >
          <defs>
            <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--ff-income)" stopOpacity=".12" />
              <stop offset="100%" stopColor="var(--ff-income)" stopOpacity="0" />
            </linearGradient>
          </defs>
          {[12, 72, 132].map((height) => (
            <line
              key={height}
              x1="0"
              x2="600"
              y1={height}
              y2={height}
              stroke="var(--ff-border)"
              strokeDasharray="2 6"
              vectorEffect="non-scaling-stroke"
            />
          ))}
          {balanceView ? (
            <>
              <polygon points={`24,132 ${line} 576,132`} fill={`url(#${id})`} />
              <polyline
                points={line}
                stroke="var(--ff-income)"
                strokeWidth="2.25"
                fill="none"
                strokeLinejoin="round"
                vectorEffect="non-scaling-stroke"
              />
              <circle
                cx={x(points.length - 1)}
                cy={y(points[points.length - 1]?.balance ?? 0)}
                r="4"
                fill="var(--ff-surface)"
                stroke="var(--ff-income)"
                strokeWidth="2"
                vectorEffect="non-scaling-stroke"
              />
            </>
          ) : (
            points.map((p, i) => (
              <g key={p.day}>
                <rect
                  x={x(i) - 20}
                  y={y(p.income)}
                  width="18"
                  height={132 - y(p.income)}
                  rx="3"
                  fill="var(--ff-income)"
                />
                <rect
                  x={x(i) + 2}
                  y={y(p.expense)}
                  width="18"
                  height={132 - y(p.expense)}
                  rx="3"
                  fill="var(--ff-expense)"
                />
              </g>
            ))
          )}
        </svg>
      </div>
      <div className="ff-chart-dates" aria-hidden="true">
        {points.map((p) => (
          <span key={p.day}>
            {p.day} {month}
          </span>
        ))}
      </div>
      <details className="ff-preview-details">
        <summary>
          Explorar exemplo<span aria-hidden="true">+</span>
        </summary>
        <div className="ff-preview-expanded">
          <fieldset className="ff-chart-tabs" aria-label="Visualização do gráfico">
            <button type="button" aria-pressed={balanceView} onClick={() => setView("balance")}>
              Saldo ao longo do mês
            </button>
            <button type="button" aria-pressed={!balanceView} onClick={() => setView("comparison")}>
              Entradas e saídas
            </button>
          </fieldset>
          <fieldset className="chart-months" aria-label="Consultar valores por data">
            {points.map((p, i) => (
              <button
                key={p.day}
                type="button"
                aria-pressed={selected === i}
                onClick={() => setSelected(i)}
              >
                {p.day} {month}
              </button>
            ))}
          </fieldset>
          <div className="ff-chart-readout" aria-live="polite" aria-atomic="true">
            {current &&
              (balanceView ? (
                <>
                  <span>
                    Saldo em {current.day} {month}
                  </span>
                  <strong>{money(current.balance)}</strong>
                </>
              ) : (
                <>
                  <span>
                    <i className="legend-dot" />
                    Entradas <strong>{money(current.income)}</strong>
                  </span>
                  <span>
                    <i className="legend-dot expense" />
                    Saídas <strong>{money(current.expense)}</strong>
                  </span>
                </>
              ))}
          </div>
          <details className="ff-chart-data">
            <summary>Ver valores do gráfico</summary>
            <table>
              <caption>{period} · valores de exemplo em reais</caption>
              <thead>
                <tr>
                  <th scope="col">Até o dia</th>
                  <th scope="col">Saldo</th>
                  <th scope="col">Saídas no período</th>
                </tr>
              </thead>
              <tbody>
                {points.map((p) => (
                  <tr key={p.day}>
                    <th scope="row">{p.day}</th>
                    <td>{money(p.balance)}</td>
                    <td>{money(p.expense)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </details>
          {children}
        </div>
      </details>
    </section>
  );
}
