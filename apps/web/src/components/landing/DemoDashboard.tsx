import {
  ArrowDownLeft,
  ArrowUpRight,
  BriefcaseBusiness,
  ChevronDown,
  Download,
  Plus,
  ShoppingBag,
  Undo2,
  Wallet,
} from "lucide-react";
import { useState } from "react";
import { FinanceChart, type FinancePoint } from "./FinanceChart";
import { money } from "./money";

const periods = {
  Novembro: {
    opening: 2180300,
    income: [650000, 0, 200000, 0],
    expenses: [128400, 96050, 148700, 198500],
    salary: 650000,
  },
  Outubro: {
    opening: 1892000,
    income: [620000, 0, 100000, 0],
    expenses: [104000, 82900, 121800, 123000],
    salary: 620000,
  },
};

export function DemoDashboard() {
  const [period, setPeriod] = useState<keyof typeof periods>("Novembro");
  const [added, setAdded] = useState(false);
  const data = periods[period];
  const income = data.income.reduce((sum, value) => sum + value, 0);
  const expenses = data.expenses.reduce((sum, value) => sum + value, 0) + (added ? 2450 : 0);
  const balance = data.opening + income - expenses;
  let runningBalance = data.opening;
  const points: FinancePoint[] = data.income.map((value, i) => {
    const expense = (data.expenses[i] ?? 0) + (added && i === 3 ? 2450 : 0);
    runningBalance += value - expense;
    return {
      day: ["07", "14", "21", "30"][i] ?? "30",
      income: value,
      expense,
      balance: runningBalance,
    };
  });
  const transactions = [
    ...(added
      ? [{ name: "Café com a equipe", category: "Alimentação", amount: -2450, icon: Wallet }]
      : []),
    { name: "Salário do mês", category: "Trabalho", amount: data.salary, icon: BriefcaseBusiness },
    { name: "Mercado da semana", category: "Alimentação", amount: -28690, icon: ShoppingBag },
    { name: "Streaming e música", category: "Lazer", amount: -8990, icon: Wallet },
  ];
  const budgets = [
    { name: "Alimentação", value: added ? 72 : 70, tone: "warning", text: "Acompanhe o ritmo" },
    { name: "Lazer", value: 82, tone: "warning", text: "Pede atenção" },
    { name: "Transporte", value: 35, tone: "income", text: "Dentro do planejado" },
  ];

  function exportCsv() {
    const content =
      "\uFEFFDescrição;Categoria;Valor\n" +
      transactions
        .map((t) => `${t.name};${t.category};${(t.amount / 100).toFixed(2).replace(".", ",")}`)
        .join("\n");
    const url = URL.createObjectURL(new Blob([content], { type: "text/csv;charset=utf-8" }));
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `finflow-exemplo-${period.toLowerCase()}.csv`;
    anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  return (
    <div className="dashboard-bezel" id="demonstracao">
      <div className="dashboard">
        <div className="app-topbar">
          <div className="app-title">
            <h2>Visão geral</h2>
          </div>
          <label className="period-select">
            <span className="sr-only">Período do dashboard</span>
            <select
              value={period}
              onChange={(e) => setPeriod(e.target.value as keyof typeof periods)}
            >
              <option>Novembro</option>
              <option>Outubro</option>
            </select>
            <ChevronDown size={15} aria-hidden="true" />
          </label>
        </div>
        <div className="app-summary">
          <div className="balance">
            <span className="app-label">Saldo nas contas</span>
            <strong>{money(balance)}</strong>
          </div>
          <div className="flows">
            <div>
              <span className="app-label">
                <ArrowDownLeft size={15} aria-hidden="true" />
                Entradas do mês
              </span>
              <strong className="ff-income">{money(income)}</strong>
            </div>
            <div>
              <span className="app-label">
                <ArrowUpRight size={15} aria-hidden="true" />
                Saídas do mês
              </span>
              <strong className="ff-expense">{money(expenses)}</strong>
            </div>
          </div>
        </div>
        <FinanceChart points={points} period={period}>
          <details className="budget-panel">
            <summary>
              Como estão meus limites?<span>{period}</span>
            </summary>
            <div className="budget-list">
              {budgets.map((budget) => (
                <div className="budget" key={budget.name}>
                  <div className="budget-row">
                    <strong>{budget.name}</strong>
                    <span>{budget.value}% usado</span>
                  </div>
                  <meter
                    className="ff-budget-meter"
                    aria-label={`Limite de ${budget.name} usado`}
                    value={budget.value}
                    min={0}
                    max={100}
                    low={70}
                    high={90}
                    optimum={0}
                  >
                    {budget.value}%
                  </meter>
                  <small className={`ff-${budget.tone}`}>{budget.text}</small>
                </div>
              ))}
            </div>
          </details>
          <section className="transactions" aria-label="Exemplos de lançamentos">
            <div className="panel-heading">
              <h3>Últimos lançamentos</h3>
              <button type="button" className="export-button" onClick={exportCsv}>
                <Download size={15} aria-hidden="true" />
                Baixar exemplo CSV
              </button>
            </div>
            {transactions.slice(0, 3).map((t) => (
              <div className="transaction" key={t.name}>
                <span className={`transaction-icon ${t.amount > 0 ? "income" : ""}`}>
                  <t.icon size={17} aria-hidden="true" />
                </span>
                <div className="transaction-name">
                  <strong>{t.name}</strong>
                  <small>{t.category}</small>
                </div>
                <strong className={`transaction-amount ${t.amount > 0 ? "ff-income" : ""}`}>
                  {t.amount > 0 ? "+ " : "− "}
                  {money(Math.abs(t.amount))}
                </strong>
              </div>
            ))}
          </section>
          <button
            type="button"
            className="demo-add"
            aria-label={added ? "Desfazer despesa" : "Simular uma despesa"}
            onClick={() => setAdded(!added)}
          >
            {added ? <Undo2 size={17} aria-hidden="true" /> : <Plus size={17} aria-hidden="true" />}
            {added ? "Desfazer despesa" : "Simular uma despesa"}
          </button>
        </FinanceChart>
      </div>
      <span className="sr-only" role="status">
        {added
          ? "Gasto de exemplo de R$ 24,50 adicionado. Saldo e gráfico atualizados."
          : "Prévia com dados ilustrativos. Nenhum valor é salvo."}
      </span>
    </div>
  );
}
