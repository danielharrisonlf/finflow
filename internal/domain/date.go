package domain

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	// Embute a base de fusos para LoadLocation funcionar no Windows e em containers scratch.
	_ "time/tzdata"
)

var dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// DefaultTimezone é o fuso usado para "hoje" e "mês corrente" quando FINFLOW_TZ não está definida.
const DefaultTimezone = "America/Sao_Paulo"

var (
	locOnce sync.Once
	loc     *time.Location
	// nowFunc permite fixar o relógio nos testes.
	nowFunc = time.Now
)

// Location devolve o fuso de negócio (FINFLOW_TZ, padrão America/Sao_Paulo), lido uma única vez.
// Datas armazenadas continuam 'YYYY-MM-DD'; timestamps de auditoria seguem em UTC.
func Location() *time.Location {
	locOnce.Do(func() {
		name := strings.TrimSpace(os.Getenv("FINFLOW_TZ"))
		if name == "" {
			name = DefaultTimezone
		}
		l, err := time.LoadLocation(name)
		if err != nil {
			log.Printf("FINFLOW_TZ inválido (%q): %v — usando %s", name, err, DefaultTimezone)
			l, err = time.LoadLocation(DefaultTimezone)
			if err != nil {
				l = time.UTC
			}
		}
		loc = l
	})
	return loc
}

// Now devolve o instante atual no fuso de negócio.
func Now() time.Time {
	return nowFunc().In(Location())
}

func IsValidDateString(v string) bool {
	if !dateRegex.MatchString(v) {
		return false
	}
	_, err := time.Parse("2006-01-02", v)
	return err == nil
}

func ToDateString(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func FromDateString(v string) time.Time {
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// TodayString devolve a data de hoje (YYYY-MM-DD) no fuso de negócio.
func TodayString() string {
	return Now().Format("2006-01-02")
}

func AddDays(value string, days int) string {
	return ToDateString(FromDateString(value).AddDate(0, 0, days))
}

func MonthStart(year, month int) string {
	return fmt.Sprintf("%04d-%02d-01", year, month)
}

// MonthRange retorna [start, end) como strings YYYY-MM-DD.
func MonthRange(year, month int) (string, string) {
	nextYear, nextMonth := year, month+1
	if month == 12 {
		nextYear++
		nextMonth = 1
	}
	return MonthStart(year, month), MonthStart(nextYear, nextMonth)
}

// YearRange retorna [YYYY-01-01, (YYYY+1)-01-01).
func YearRange(year int) (string, string) {
	return MonthStart(year, 1), MonthStart(year+1, 1)
}

// AddMonthsToDateString soma meses mantendo o dia (clampado ao último dia do mês destino).
func AddMonthsToDateString(value string, delta int) string {
	d := FromDateString(value)
	targetFirst := time.Date(d.Year(), d.Month()+time.Month(delta), 1, 0, 0, 0, 0, time.UTC)
	lastDay := time.Date(targetFirst.Year(), targetFirst.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := d.Day()
	if day > lastDay {
		day = lastDay
	}
	return ToDateString(time.Date(targetFirst.Year(), targetFirst.Month(), day, 0, 0, 0, 0, time.UTC))
}

// CurrentMonthYear devolve mês e ano correntes no fuso de negócio.
func CurrentMonthYear() (month int, year int) {
	now := Now()
	return int(now.Month()), now.Year()
}

// IsInCurrentMonth indica se a data YYYY-MM-DD cai no mês corrente (fuso de negócio).
func IsInCurrentMonth(date string) bool {
	month, year := CurrentMonthYear()
	start, end := MonthRange(year, month)
	return date >= start && date < end
}

// MonthsUntil calcula meses inteiros entre `from` e `deadline` (mínimo 1).
func MonthsUntil(deadline, from string) int {
	if from == "" {
		from = TodayString()
	}
	a := FromDateString(from)
	b := FromDateString(deadline)
	months := (b.Year()-a.Year())*12 + int(b.Month()-a.Month())
	adjusted := months
	if b.Day() < a.Day() {
		adjusted--
	}
	if adjusted < 1 {
		return 1
	}
	return adjusted
}
