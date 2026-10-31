import pc from "picocolors";

export type Align = "left" | "right";

export interface Column {
  header: string;
  align?: Align;
}

/** Largura visível ignorando códigos ANSI de cor. */
export function visibleWidth(text: string): number {
  // biome-ignore lint/suspicious/noControlCharactersInRegex: remoção de códigos ANSI
  return text.replace(/\u001b\[[0-9;]*m/g, "").length;
}

function pad(text: string, width: number, align: Align): string {
  const gap = Math.max(0, width - visibleWidth(text));
  return align === "right" ? " ".repeat(gap) + text : text + " ".repeat(gap);
}

/** Renderiza uma tabela alinhada para o terminal (cabeçalho discreto). */
export function renderTable(columns: Column[], rows: string[][]): string {
  const widths = columns.map((col, i) =>
    Math.max(visibleWidth(col.header), ...rows.map((r) => visibleWidth(r[i] ?? ""))),
  );
  const header = columns
    .map((col, i) => pc.dim(pad(col.header, widths[i] ?? 0, col.align ?? "left")))
    .join("  ");
  const lines = rows.map((row) =>
    columns.map((col, i) => pad(row[i] ?? "", widths[i] ?? 0, col.align ?? "left")).join("  "),
  );
  return [header, ...lines].join("\n");
}

/** Barra de progresso textual: ███████░░░ 70% */
export function progressBar(percent: number, width = 14): string {
  const clamped = Math.min(100, Math.max(0, percent));
  const filled = Math.round((clamped / 100) * width);
  const color = percent > 90 ? pc.red : percent >= 70 ? pc.yellow : pc.green;
  return color("█".repeat(filled) + "░".repeat(width - filled));
}
