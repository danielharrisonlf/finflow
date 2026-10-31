import { parseBRLToCents } from "../../common/money.util";

export type ParsedTelegramMessage =
  | { kind: "help" }
  | { kind: "balance" }
  | { kind: "summary" }
  | { kind: "statement" }
  | { kind: "expense"; amountCents: number; description: string; categoryName: string | null }
  | { kind: "unknown"; raw: string };

/**
 * Entende comandos (/saldo, /resumo, /extrato, /ajuda) e lançamentos rápidos
 * no formato "45,90 almoço #alimentacao" (categoria opcional via hashtag).
 */
export function parseTelegramMessage(raw: string): ParsedTelegramMessage {
  const text = raw.trim();
  if (!text) return { kind: "unknown", raw };

  const command = text.split(/[\s@]/, 1)[0]?.toLowerCase();
  switch (command) {
    case "/start":
    case "/ajuda":
    case "/help":
      return { kind: "help" };
    case "/saldo":
      return { kind: "balance" };
    case "/resumo":
      return { kind: "summary" };
    case "/extrato":
      return { kind: "statement" };
    default:
      break;
  }
  if (text.startsWith("/")) return { kind: "unknown", raw };

  const match = text.match(/^(?<amount>[\d.,]+)\s+(?<rest>.+)$/s);
  if (!match?.groups) return { kind: "unknown", raw };

  const amountCents = parseBRLToCents(match.groups.amount ?? "");
  if (amountCents === null) return { kind: "unknown", raw };

  let description = (match.groups.rest ?? "").trim();
  let categoryName: string | null = null;
  const hashtag = description.match(/#([\p{L}\p{N}_-]+)/u);
  if (hashtag) {
    categoryName = (hashtag[1] ?? "").replace(/[_-]/g, " ");
    description = description
      .replace(hashtag[0], "")
      .replace(/\s{2,}/g, " ")
      .trim();
  }
  if (!description) return { kind: "unknown", raw };

  return { kind: "expense", amountCents, description, categoryName };
}
