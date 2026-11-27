package domain

import (
	"fmt"
	"strings"
)

var typeLabels = map[TransactionType]string{
	TxIncome:         "Receita",
	TxExpense:        "Despesa",
	TxTransfer:       "Transferência",
	TxCardExpense:    "Cartão de crédito",
	TxInvoicePayment: "Pagamento de fatura",
}

func csvEscape(value string) string {
	if strings.ContainsAny(value, "\";\n\r") {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return value
}

func centsToBRNumber(cents int64) string {
	return strings.ReplaceAll(fmt.Sprintf("%.2f", float64(cents)/100), ".", ",")
}

// BuildCSV gera CSV com separador ';', vírgula decimal e BOM UTF-8 para Excel em pt-BR.
func BuildCSV(transactions []Transaction) string {
	var lines []string
	lines = append(lines, "Data;Descrição;Tipo;Categoria;Conta/Cartão;Status;Parcela;Valor (R$)")
	for _, t := range transactions {
		cat := ""
		if t.CategoryName != nil {
			cat = *t.CategoryName
		}
		origin := ""
		if t.AccountName != nil {
			origin = *t.AccountName
		} else if t.CardName != nil {
			origin = *t.CardName
		}
		status := "Pendente"
		if t.Status == TxConfirmed {
			status = "Confirmada"
		}
		installment := ""
		if t.InstallmentNumber != nil && t.InstallmentTotal != nil {
			installment = fmt.Sprintf("%d/%d", *t.InstallmentNumber, *t.InstallmentTotal)
		}
		lines = append(lines, strings.Join([]string{
			t.Date,
			csvEscape(t.Description),
			typeLabels[t.Type],
			csvEscape(cat),
			csvEscape(origin),
			status,
			installment,
			centsToBRNumber(t.AmountCents),
		}, ";"))
	}
	return "\uFEFF" + strings.Join(lines, "\r\n") + "\r\n"
}

var ofxSanitizeReplacer = strings.NewReplacer("<", " ", ">", " ", "&", " ")

// BuildOFX gera OFX 1.02 (SGML) aceito por planilhas e apps financeiros.
func BuildOFX(transactions []Transaction) string {
	now := strings.ReplaceAll(TodayString(), "-", "")
	start, end := now, now
	if len(transactions) > 0 {
		start = strings.ReplaceAll(transactions[0].Date, "-", "")
		end = start
		for _, t := range transactions[1:] {
			d := strings.ReplaceAll(t.Date, "-", "")
			if d < start {
				start = d
			}
			if d > end {
				end = d
			}
		}
	}

	var items []string
	for _, t := range transactions {
		memo := t.Description
		if t.CategoryName != nil && *t.CategoryName != "" {
			memo += " | " + *t.CategoryName
		}
		trnType := "DEBIT"
		if t.AmountCents >= 0 {
			trnType = "CREDIT"
		}
		items = append(items, strings.Join([]string{
			"<STMTTRN>",
			"<TRNTYPE>" + trnType,
			"<DTPOSTED>" + strings.ReplaceAll(t.Date, "-", ""),
			fmt.Sprintf("<TRNAMT>%.2f", float64(t.AmountCents)/100),
			"<FITID>" + t.ID,
			"<MEMO>" + ofxSanitizeReplacer.Replace(memo),
			"</STMTTRN>",
		}, "\n"))
	}

	return strings.Join([]string{
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
		"<DTSERVER>" + now + "<LANGUAGE>POR</SONRS></SIGNONMSGSRSV1>",
		"<BANKMSGSRSV1><STMTTRNRS><TRNUID>1",
		"<STATUS><CODE>0<SEVERITY>INFO</STATUS>",
		"<STMTRS><CURDEF>BRL",
		"<BANKACCTFROM><BANKID>FINFLOW<ACCTID>EXTRATO<ACCTTYPE>CHECKING</BANKACCTFROM>",
		"<BANKTRANLIST><DTSTART>" + start + "<DTEND>" + end,
		strings.Join(items, "\n"),
		"</BANKTRANLIST>",
		"</STMTRS></STMTTRNRS></BANKMSGSRSV1>",
		"</OFX>",
	}, "\n")
}
