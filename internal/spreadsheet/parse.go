package spreadsheet

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"finflow/internal/domain"
)

var (
	plainNumberRe = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
	digitsRe      = regexp.MustCompile(`^\d+$`)
	isoDateRe     = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})(?:[T ].*)?$`)
	dmyDateRe     = regexp.MustCompile(`^(\d{1,2})[/.\-](\d{1,2})[/.\-](\d{2}|\d{4})(?:\s.*)?$`)
	compactDateRe = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})`)
)

// excelEpoch é o dia zero dos seriais do Excel (sistema 1900, com o bug do 29/02/1900 embutido).
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// ParseAmount converte texto de valor para centavos com sinal. Aceita "R$ 1.234,56", "1,234.56",
// "-10", "(10,00)", "10 D" / "10 C", "R$ -45,90", "45,90-". Valor zero é válido (retorna 0, true).
func ParseAmount(raw string) (int64, bool) {
	s := strings.TrimSpace(strings.NewReplacer(" ", " ", " ", " ", " ", " ").Replace(raw))
	if s == "" {
		return 0, false
	}
	neg := false
	upper := strings.ToUpper(s)
	// Sufixo D/C (débito/crédito) usado por bancos.
	if n := len(upper); n >= 2 {
		last := upper[n-1]
		prev := upper[n-2]
		if (last == 'D' || last == 'C') && (prev == ' ' || (prev >= '0' && prev <= '9')) {
			neg = last == 'D'
			s = strings.TrimSpace(s[:n-1])
		}
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg = !neg
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	// Sinais e prefixo R$ em qualquer ordem: "-R$ 10", "R$ -10", "10-".
	signs := 0
	for again := true; again; {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "-"):
			neg = !neg
			signs++
			s = s[1:]
		case strings.HasPrefix(s, "+"):
			s = s[1:]
		case len(s) > 1 && strings.HasSuffix(s, "-"):
			neg = !neg
			signs++
			s = s[:len(s)-1]
		case len(s) >= 2 && strings.EqualFold(s[:2], "R$"):
			s = s[2:]
		default:
			again = false
		}
		if signs > 1 {
			return 0, false
		}
	}
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, false
	}
	value, ok := parseDecimal(s)
	if !ok {
		return 0, false
	}
	cents := math.Round(value * 100)
	if math.IsNaN(cents) || math.IsInf(cents, 0) || cents > domain.MaxParsedCents {
		return 0, false
	}
	if neg {
		cents = -cents
	}
	return int64(cents), true
}

// parseDecimal interpreta só dígitos e separadores: o último separador presente é o decimal quando
// há os dois; com um só tipo, vírgula é decimal (padrão BR) e ponto é decimal salvo "1.234" (milhar).
func parseDecimal(s string) (float64, bool) {
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' && r != ',' {
			return 0, false
		}
	}
	lastDot := strings.LastIndex(s, ".")
	lastComma := strings.LastIndex(s, ",")
	var normalized string
	switch {
	case lastDot >= 0 && lastComma >= 0:
		if lastComma > lastDot {
			normalized = strings.ReplaceAll(s, ".", "")
			normalized = strings.Replace(normalized, ",", ".", 1)
		} else {
			normalized = strings.ReplaceAll(s, ",", "")
		}
	case lastComma >= 0:
		// Uma vírgula é sempre decimal (padrão BR); várias são separador de milhar ("1,234,567").
		parts := strings.Split(s, ",")
		if len(parts) > 2 {
			normalized = strings.Join(parts, "")
		} else {
			normalized = strings.Replace(s, ",", ".", 1)
		}
	case lastDot >= 0:
		parts := strings.Split(s, ".")
		if len(parts) > 2 || (len(parts) == 2 && len(parts[1]) == 3 && parts[0] != "0" && parts[0] != "") {
			normalized = strings.Join(parts, "")
		} else {
			normalized = s
		}
	default:
		normalized = s
	}
	if normalized == "" || normalized == "." {
		return 0, false
	}
	v, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseCellAmount lê valores vindos de célula xlsx em modo bruto: números puros ("1234.5", "1E3") usam
// ponto decimal; texto cai no ParseAmount.
func parseCellAmount(raw string) (int64, bool) {
	s := strings.TrimSpace(raw)
	if plainNumberRe.MatchString(s) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		cents := math.Round(v * 100)
		if math.Abs(cents) > domain.MaxParsedCents {
			return 0, false
		}
		return int64(cents), true
	}
	return ParseAmount(s)
}

// ExcelSerialToDate converte serial do Excel (dias desde 1899-12-30) em YYYY-MM-DD.
func ExcelSerialToDate(serial float64) (string, bool) {
	if serial < 1 || serial > 2958465 {
		return "", false
	}
	days := int(math.Floor(serial))
	return excelEpoch.AddDate(0, 0, days).Format("2006-01-02"), true
}

func validYMD(y, m, d int) (string, bool) {
	if y < 1900 || y > 2200 || m < 1 || m > 12 || d < 1 || d > daysIn(y, m) {
		return "", false
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d), true
}

// ParseDate aceita dd/mm/aaaa, dd/mm/aa, d.m.aaaa, aaaa-mm-dd (com hora opcional), aaaammdd e seriais
// do Excel (quando allowSerial). Para dd/mm ambíguo prevalece o padrão brasileiro; só vira mm/dd quando
// o segundo número passa de 12.
func ParseDate(raw string, allowSerial bool) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if m := isoDateRe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		return validYMD(y, mo, d)
	}
	if m := dmyDateRe.FindStringSubmatch(s); m != nil {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		y, _ := strconv.Atoi(m[3])
		if len(m[3]) == 2 {
			// Janela de dois dígitos: 00–69 → 2000–2069, 70–99 → 1970–1999.
			if y < 70 {
				y += 2000
			} else {
				y += 1900
			}
		}
		if out, ok := validYMD(y, b, a); ok {
			return out, true
		}
		if b > 12 {
			return validYMD(y, a, b)
		}
		return "", false
	}
	if len(s) >= 8 && digitsRe.MatchString(s[:8]) && (len(s) == 8 || !digitsRe.MatchString(s)) {
		if m := compactDateRe.FindStringSubmatch(s); m != nil {
			y, _ := strconv.Atoi(m[1])
			mo, _ := strconv.Atoi(m[2])
			d, _ := strconv.Atoi(m[3])
			if out, ok := validYMD(y, mo, d); ok {
				return out, true
			}
		}
	}
	if allowSerial && plainNumberRe.MatchString(s) {
		v, err := strconv.ParseFloat(s, 64)
		// Seriais plausíveis para lançamentos: 1954 a 2119.
		if err == nil && v >= 20000 && v <= 80000 {
			return ExcelSerialToDate(v)
		}
	}
	return "", false
}

// parseOFXDate lê DTPOSTED (AAAAMMDD[HHMMSS[.xxx]][[-3:BRT]]).
func parseOFXDate(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if m := compactDateRe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		return validYMD(y, mo, d)
	}
	return ParseDate(s, false)
}
