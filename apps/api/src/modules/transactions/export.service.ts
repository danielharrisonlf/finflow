import type { ExportTransactionsQuery, Transaction } from "@finflow/contracts";
import { Injectable } from "@nestjs/common";
import { TransactionsRepository } from "./transactions.repository";
import { toContract } from "./transactions.service";

const TYPE_LABELS: Record<Transaction["type"], string> = {
  INCOME: "Receita",
  EXPENSE: "Despesa",
  TRANSFER: "Transferência",
  CARD_EXPENSE: "Cartão de crédito",
  INVOICE_PAYMENT: "Pagamento de fatura",
};

function csvEscape(value: string): string {
  return /[";\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
}

/** Valor em formato pt-BR com vírgula decimal (Excel brasileiro). */
function centsToBRNumber(cents: number): string {
  return (cents / 100).toFixed(2).replace(".", ",");
}

export function buildCsv(transactions: Transaction[]): string {
  const header = "Data;Descrição;Tipo;Categoria;Conta/Cartão;Status;Parcela;Valor (R$)";
  const rows = transactions.map((t) =>
    [
      t.date,
      csvEscape(t.description),
      TYPE_LABELS[t.type],
      csvEscape(t.categoryName ?? ""),
      csvEscape(t.accountName ?? t.cardName ?? ""),
      t.status === "CONFIRMED" ? "Confirmada" : "Pendente",
      t.installmentNumber ? `${t.installmentNumber}/${t.installmentTotal}` : "",
      centsToBRNumber(t.amountCents),
    ].join(";"),
  );
  // BOM garante acentuação correta ao abrir no Excel
  return `﻿${[header, ...rows].join("\r\n")}\r\n`;
}

/** OFX 1.02 (SGML), aceito por Excel, planilhas e apps financeiros. */
export function buildOfx(transactions: Transaction[]): string {
  const now = new Date().toISOString().slice(0, 10).replace(/-/g, "");
  const dates = transactions.map((t) => t.date.replace(/-/g, ""));
  const start = dates.length > 0 ? dates.reduce((a, b) => (a < b ? a : b)) : now;
  const end = dates.length > 0 ? dates.reduce((a, b) => (a > b ? a : b)) : now;

  const items = transactions
    .map((t) => {
      const memo = [t.description, t.categoryName].filter(Boolean).join(" | ");
      return [
        "<STMTTRN>",
        `<TRNTYPE>${t.amountCents >= 0 ? "CREDIT" : "DEBIT"}`,
        `<DTPOSTED>${t.date.replace(/-/g, "")}`,
        `<TRNAMT>${(t.amountCents / 100).toFixed(2)}`,
        `<FITID>${t.id}`,
        `<MEMO>${memo.replace(/[<>&]/g, " ")}`,
        "</STMTTRN>",
      ].join("\n");
    })
    .join("\n");

  return [
    "OFXHEADER:100",
    "DATA:OFXSGML",
    "VERSION:102",
    "SECURITY:NONE",
    "ENCODING:UTF-8",
    "CHARSET:NONE",
    "COMPRESSION:NONE",
    "OLDFILEUID:NONE",
    "NEWFILEUID:NONE",
    "",
    "<OFX>",
    "<SIGNONMSGSRSV1><SONRS><STATUS><CODE>0<SEVERITY>INFO</STATUS>",
    `<DTSERVER>${now}<LANGUAGE>POR</SONRS></SIGNONMSGSRSV1>`,
    "<BANKMSGSRSV1><STMTTRNRS><TRNUID>1",
    "<STATUS><CODE>0<SEVERITY>INFO</STATUS>",
    "<STMTRS><CURDEF>BRL",
    "<BANKACCTFROM><BANKID>FINFLOW<ACCTID>EXTRATO<ACCTTYPE>CHECKING</BANKACCTFROM>",
    `<BANKTRANLIST><DTSTART>${start}<DTEND>${end}`,
    items,
    "</BANKTRANLIST>",
    "</STMTRS></STMTTRNRS></BANKMSGSRSV1>",
    "</OFX>",
  ].join("\n");
}

@Injectable()
export class ExportService {
  constructor(private readonly repository: TransactionsRepository) {}

  async export(
    query: ExportTransactionsQuery,
  ): Promise<{ content: string; contentType: string; filename: string }> {
    const { format, ...filters } = query;
    const items = await this.repository.listAll(filters);
    const transactions = items.map(toContract);
    const stamp = new Date().toISOString().slice(0, 10);
    if (format === "ofx") {
      return {
        content: buildOfx(transactions),
        contentType: "application/x-ofx",
        filename: `finflow-extrato-${stamp}.ofx`,
      };
    }
    return {
      content: buildCsv(transactions),
      contentType: "text/csv; charset=utf-8",
      filename: `finflow-extrato-${stamp}.csv`,
    };
  }
}
