import { FileDown, FileSpreadsheet, FileText, Landmark, Sheet } from "lucide-react";
import { type Band, bandOf } from "./projection";

// A December block from the daily sheet. Values in whole reais, as typed in a spreadsheet.
const rows: { day: string; income?: number; out?: number; daily?: number; balance: number }[] = [
  { day: "01/12", income: 4200, daily: 62, balance: 4618 },
  { day: "02/12", out: 1300, daily: 48, balance: 3270 },
  { day: "03/12", out: 129, daily: 35, balance: 3106 },
  { day: "04/12", daily: 210, balance: 2896 },
  { day: "05/12", out: 1100, daily: 57, balance: 1739 },
  { day: "06/12", daily: 830, balance: 909 },
  { day: "07/12", out: 1150, daily: 40, balance: -281 },
];
const cell = (value?: number) => (value === undefined ? "" : value.toLocaleString("pt-BR"));
const bandLabel: Record<Band, string> = {
  "dark-green": "folga",
  "light-green": "confortável",
  yellow: "atenção",
  "light-red": "negativo",
  red: "muito negativo",
};

const imports = [
  {
    icon: Sheet,
    title: "Planilha diária",
    text: "Entrada · Saída · Diário · Saldo, um bloco por mês. Cada dia vira lançamento e o saldo é conferido.",
  },
  {
    icon: FileSpreadsheet,
    title: "Excel e CSV",
    text: "Data, descrição, valor, débito e crédito são reconhecidos automaticamente.",
  },
  {
    icon: Landmark,
    title: "Extrato OFX do banco",
    text: "O arquivo que o internet banking exporta. Sem senha bancária.",
  },
];

export function SpreadsheetSection() {
  return (
    <section className="ff-section ff-sheets ff-container" id="planilhas">
      <div className="ff-sheets-copy">
        <span className="ff-eyebrow">Planilhas</span>
        <h2>
          Sua planilha entra.
          <br />
          Uma planilha melhor sai.
        </h2>
        <p>
          Traga o histórico que você já tem e leve tudo de volta para o Excel quando quiser, sem
          copiar e colar linha por linha.
        </p>
        <h3 className="ff-sheets-label">
          <FileText size={16} aria-hidden="true" />
          Importe
        </h3>
        <ul className="ff-sheets-list">
          {imports.map(({ icon: Icon, title, text }) => (
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
        <h3 className="ff-sheets-label">
          <FileDown size={16} aria-hidden="true" />
          Exporte
        </h3>
        <p className="ff-sheets-export">
          Uma pasta de trabalho <strong>.xlsx</strong> com o ano inteiro: fórmulas vivas, saldo
          diário colorido por faixas, aba <strong>Economia</strong> com quanto você guardou em cada
          mês, lançamentos e categorias.
        </p>
      </div>

      <figure className="ff-sheet">
        <div className="ff-sheet-bar" aria-hidden="true">
          <span>E9</span>
          <code>=E8+B9-C9-D9</code>
        </div>
        <table>
          <caption>Dezembro · exemplo de planilha exportada</caption>
          <thead>
            <tr>
              <th scope="col">Data</th>
              <th scope="col">Entrada</th>
              <th scope="col">Saída</th>
              <th scope="col">Diário</th>
              <th scope="col">Saldo</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const band = bandOf(row.balance * 100);
              return (
                <tr key={row.day}>
                  <th scope="row">{row.day}</th>
                  <td>{cell(row.income)}</td>
                  <td>{cell(row.out)}</td>
                  <td>{cell(row.daily)}</td>
                  <td className={`ff-band-cell is-${band}`}>
                    {cell(row.balance)}
                    <span className="sr-only">, {bandLabel[band]}</span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        <div className="ff-sheet-tabs" aria-hidden="true">
          <span className="active">2026</span>
          <span>Economia</span>
          <span>Lançamentos</span>
          <span>Categorias</span>
        </div>
        <figcaption className="ff-fineprint">
          Valores ilustrativos. O saldo de cada dia é uma fórmula ligada ao dia anterior.
        </figcaption>
      </figure>
    </section>
  );
}
