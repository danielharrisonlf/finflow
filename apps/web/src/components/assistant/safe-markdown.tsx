import type { ReactNode } from "react";

// Renderizador mínimo e seguro para as respostas do modelo: só gera elementos React (nunca HTML cru).
// Suporta parágrafos, quebras de linha, listas (-, *, 1.), títulos curtos (#), **negrito**, *itálico* e `código`.

type Block =
  | { kind: "p"; lines: string[] }
  | { kind: "ul"; items: string[] }
  | { kind: "ol"; items: string[] }
  | { kind: "h"; text: string };

const BULLET = /^\s*[-*•]\s+(.*)$/;
const ORDERED = /^\s*\d+[.)]\s+(.*)$/;
const HEADING = /^\s*#{1,6}\s+(.*)$/;

function toBlocks(text: string): Block[] {
  const blocks: Block[] = [];
  const push = (block: Block) => blocks.push(block);
  for (const line of text.replace(/\r\n/g, "\n").split("\n")) {
    const last = blocks[blocks.length - 1];
    if (line.trim() === "") {
      if (last && last.kind === "p" && last.lines.length > 0) push({ kind: "p", lines: [] });
      continue;
    }
    const bullet = BULLET.exec(line);
    const ordered = bullet ? null : ORDERED.exec(line);
    const heading = bullet || ordered ? null : HEADING.exec(line);
    if (bullet) {
      if (last?.kind === "ul") last.items.push(bullet[1] ?? "");
      else push({ kind: "ul", items: [bullet[1] ?? ""] });
    } else if (ordered) {
      if (last?.kind === "ol") last.items.push(ordered[1] ?? "");
      else push({ kind: "ol", items: [ordered[1] ?? ""] });
    } else if (heading) {
      push({ kind: "h", text: heading[1] ?? "" });
    } else if (last?.kind === "p") {
      last.lines.push(line);
    } else {
      push({ kind: "p", lines: [line] });
    }
  }
  return blocks.filter((b) => b.kind !== "p" || b.lines.length > 0);
}

const INLINE = /(\*\*[^*\n]+\*\*|__[^_\n]+__|`[^`\n]+`|\*[^*\n]+\*)/g;

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const out: ReactNode[] = [];
  let lastIndex = 0;
  let i = 0;
  for (const match of text.matchAll(INLINE)) {
    const token = match[0];
    const index = match.index ?? 0;
    if (index > lastIndex) out.push(text.slice(lastIndex, index));
    const key = `${keyPrefix}-${i++}`;
    if (token.startsWith("**") || token.startsWith("__")) {
      out.push(
        <strong key={key} className="font-semibold text-app-text">
          {token.slice(2, -2)}
        </strong>,
      );
    } else if (token.startsWith("`")) {
      out.push(
        <code key={key} className="rounded bg-app-soft px-1 py-0.5 text-[0.9em]">
          {token.slice(1, -1)}
        </code>,
      );
    } else {
      out.push(<em key={key}>{token.slice(1, -1)}</em>);
    }
    lastIndex = index + token.length;
  }
  if (lastIndex < text.length) out.push(text.slice(lastIndex));
  return out;
}

export function SafeMarkdown({ text }: { text: string }) {
  const blocks = toBlocks(text);
  return (
    <div className="flex flex-col gap-2 leading-relaxed">
      {blocks.map((block, bi) => {
        const key = `b${bi}`;
        switch (block.kind) {
          case "h":
            return (
              <p key={key} className="font-semibold text-app-text">
                {renderInline(block.text, key)}
              </p>
            );
          case "ul":
            return (
              <ul key={key} className="flex list-disc flex-col gap-1 pl-5">
                {block.items.map((item, ii) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: itens de texto estático, sem reordenação
                  <li key={`${key}-${ii}`}>{renderInline(item, `${key}-${ii}`)}</li>
                ))}
              </ul>
            );
          case "ol":
            return (
              <ol key={key} className="flex list-decimal flex-col gap-1 pl-5">
                {block.items.map((item, ii) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: itens de texto estático, sem reordenação
                  <li key={`${key}-${ii}`}>{renderInline(item, `${key}-${ii}`)}</li>
                ))}
              </ol>
            );
          default:
            return (
              <p key={key}>
                {block.lines.map((line, li) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: linhas de texto estático, sem reordenação
                  <span key={`${key}-${li}`}>
                    {li > 0 ? <br /> : null}
                    {renderInline(line, `${key}-${li}`)}
                  </span>
                ))}
              </p>
            );
        }
      })}
    </div>
  );
}
