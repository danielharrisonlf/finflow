package domain

import "time"

type InvoicePeriod struct {
	Year  int
	Month int
}

// InvoicePeriodForPurchase determina a fatura da compra:
// a primeira cuja data de fechamento é posterior à compra (no dia do fechamento cai na próxima).
func InvoicePeriodForPurchase(dateStr string, closingDay int) InvoicePeriod {
	d := FromDateString(dateStr)
	year := d.Year()
	month := int(d.Month())
	if d.Day() >= closingDay {
		month++
		if month > 12 {
			month = 1
			year++
		}
	}
	return InvoicePeriod{Year: year, Month: month}
}

func AddMonthsToPeriod(period InvoicePeriod, delta int) InvoicePeriod {
	zeroBased := period.Year*12 + (period.Month - 1) + delta
	year := zeroBased / 12
	month := (zeroBased % 12) + 1
	if zeroBased < 0 && zeroBased%12 != 0 {
		year--
		month += 12
	}
	return InvoicePeriod{Year: year, Month: month}
}

func InvoiceClosingDate(period InvoicePeriod, closingDay int) string {
	d := time.Date(period.Year, time.Month(period.Month), closingDay, 0, 0, 0, 0, time.UTC)
	return ToDateString(d)
}

// InvoiceDueDate: mesmo mês quando dueDay > closingDay; senão, mês seguinte.
func InvoiceDueDate(period InvoicePeriod, closingDay, dueDay int) string {
	duePeriod := period
	if dueDay <= closingDay {
		duePeriod = AddMonthsToPeriod(period, 1)
	}
	d := time.Date(duePeriod.Year, time.Month(duePeriod.Month), dueDay, 0, 0, 0, 0, time.UTC)
	return ToDateString(d)
}

// EffectiveInvoiceStatus: PAID é persistido; OPEN e CLOSED derivam da data de fechamento.
func EffectiveInvoiceStatus(paid bool, closingDate, today string) InvoiceStatus {
	if paid {
		return InvoicePaid
	}
	if today >= closingDate {
		return InvoiceClosed
	}
	return InvoiceOpen
}
