package spreadsheet

import (
	"bytes"
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"

	"finflow/internal/domain"
)

var (
	ofxTxRe  = regexp.MustCompile(`(?is)<STMTTRN>(.*?)</STMTTRN>`)
	ofxTagRe = regexp.MustCompile(`(?is)<([A-Z0-9.]+)>([^<\r\n]*)`)
)

func looksLikeOFX(data []byte) bool {
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	upper := bytes.ToUpper(head)
	return bytes.Contains(upper, []byte("OFXHEADER")) || bytes.Contains(upper, []byte("<OFX>"))
}

// parseOFX lê os STMTTRN (TRNAMT, DTPOSTED, MEMO/NAME) de OFX 1.x (SGML) ou 2.x (XML).
func parseOFX(data []byte) (*Result, error) {
	text, _ := decodeText(data)
	res := newResult(FormatOFX)
	matches := ofxTxRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		if !looksLikeOFX(data) {
			return nil, ErrUnsupported
		}
		res.warn("nenhum lançamento (STMTTRN) encontrado no OFX")
		return res, nil
	}
	for i, m := range matches {
		tags := map[string]string{}
		for _, t := range ofxTagRe.FindAllStringSubmatch(m[1], -1) {
			name := strings.ToUpper(t[1])
			if _, seen := tags[name]; !seen {
				tags[name] = strings.TrimSpace(html.UnescapeString(t[2]))
			}
		}
		var problems []string
		out := Row{}
		if d, ok := parseOFXDate(tags["DTPOSTED"]); ok {
			out.Date = d
		} else {
			problems = append(problems, fmt.Sprintf("DTPOSTED inválido %q", tags["DTPOSTED"]))
		}
		cents, ok := parseOFXAmount(tags["TRNAMT"])
		if !ok {
			problems = append(problems, fmt.Sprintf("TRNAMT inválido %q", tags["TRNAMT"]))
		}
		out.Kind = domain.CategoryIncome
		if cents < 0 {
			out.Kind = domain.CategoryExpense
		}
		if ok && cents == 0 {
			problems = append(problems, "valor zerado")
		}
		out.AmountCents = abs(cents)
		out.Description = tags["MEMO"]
		if out.Description == "" {
			out.Description = tags["NAME"]
		}
		if out.Description == "" {
			out.Description = defaultDescription(out.Kind)
		}
		if len(problems) > 0 {
			out.Error = strPtr(fmt.Sprintf("lançamento %d: %s", i+1, strings.Join(problems, "; ")))
		}
		if !res.addRow(out) {
			break
		}
	}
	return res, nil
}

// parseOFXAmount aceita ponto decimal (padrão OFX) e vírgula (alguns bancos brasileiros).
func parseOFXAmount(raw string) (int64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false
	}
	if !strings.Contains(s, ".") {
		s = strings.Replace(s, ",", ".", 1)
	} else {
		s = strings.ReplaceAll(s, ",", "")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	cents := math.Round(v * 100)
	if math.Abs(cents) > domain.MaxParsedCents {
		return 0, false
	}
	return int64(cents), true
}
