package domain

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// MaxParsedCents limita valores digitados (R$ 100 bilhões) e evita overflow ao converter para int64.
const MaxParsedCents = 1e13

var (
	brlPrefixRe = regexp.MustCompile(`(?i)^r\$\s*`)
	brlCharsRe  = regexp.MustCompile(`^[\d.,]+$`)
)

// FormatBRL formata centavos inteiros como moeda brasileira ("R$ 1.234,56" ou "-R$ 45,90").
func FormatBRL(cents int64) string {
	neg := cents < 0
	abs := cents
	if neg {
		abs = -abs
	}
	reais := abs / 100
	rem := abs % 100

	s := strconv.FormatInt(reais, 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	b.WriteString("R$ ")
	n := len(s)
	for i := range n {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte(',')
	fmt.Fprintf(&b, "%02d", rem)
	return b.String()
}

// SplitInstallments divide um total em N parcelas inteiras cuja soma é exatamente totalCents.
// O resto vai 1 centavo por vez nas primeiras parcelas.
func SplitInstallments(totalCents int64, installments int) ([]int64, error) {
	if installments < 1 {
		return nil, errors.New("número de parcelas inválido")
	}
	base := totalCents / int64(installments)
	remainder := totalCents - base*int64(installments)
	out := make([]int64, installments)
	for i := range installments {
		out[i] = base
		if int64(i) < remainder {
			out[i]++
		}
	}
	return out, nil
}

// ParseBRLToCents converte "1.234,56", "45,90", "50" para centavos positivos; retorna (0, false) se inválido.
func ParseBRLToCents(raw string) (int64, bool) {
	cleaned := brlPrefixRe.ReplaceAllString(strings.TrimSpace(raw), "")
	if cleaned == "" || !brlCharsRe.MatchString(cleaned) {
		return 0, false
	}
	normalized := cleaned
	if strings.Contains(cleaned, ",") {
		normalized = strings.ReplaceAll(cleaned, ".", "")
		normalized = strings.ReplaceAll(normalized, ",", ".")
	}
	value, err := strconv.ParseFloat(normalized, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0, false
	}
	cents := math.Round(value * 100)
	if cents < 1 || cents > MaxParsedCents {
		return 0, false
	}
	return int64(cents), true
}

func SignedAmount(txType TransactionType, amountCents int64) int64 {
	abs := amountCents
	if abs < 0 {
		abs = -abs
	}
	if txType == TxIncome {
		return abs
	}
	return -abs
}
