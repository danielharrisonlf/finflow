package cli

import (
	"math"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func colorEnabled() bool {
	return os.Getenv("NO_COLOR") == ""
}

func ansi(code, text string) string {
	if !colorEnabled() {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func cRed(s string) string    { return ansi("31", s) }
func cGreen(s string) string  { return ansi("32", s) }
func cYellow(s string) string { return ansi("33", s) }
func cDim(s string) string    { return ansi("2", s) }
func cBold(s string) string   { return ansi("1", s) }

type Align string

const (
	AlignLeft  Align = "left"
	AlignRight Align = "right"
)

type Column struct {
	Header string
	Align  Align
}

// VisibleWidth retorna a largura visível em caracteres ignorando sequências ANSI.
func VisibleWidth(text string) int {
	return utf8.RuneCountInString(ansiRe.ReplaceAllString(text, ""))
}

func pad(text string, width int, align Align) string {
	gap := width - VisibleWidth(text)
	if gap < 0 {
		gap = 0
	}
	if align == AlignRight {
		return strings.Repeat(" ", gap) + text
	}
	return text + strings.Repeat(" ", gap)
}

func padRightRunes(text string, width int) string {
	return pad(text, width, AlignLeft)
}

// RenderTable renderiza uma tabela alinhada para terminal.
func RenderTable(columns []Column, rows [][]string) string {
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = VisibleWidth(col.Header)
		for _, r := range rows {
			if i < len(r) {
				if w := VisibleWidth(r[i]); w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	headerParts := make([]string, len(columns))
	for i, col := range columns {
		al := col.Align
		if al == "" {
			al = AlignLeft
		}
		headerParts[i] = cDim(pad(col.Header, widths[i], al))
	}

	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, strings.Join(headerParts, "  "))
	for _, row := range rows {
		parts := make([]string, len(columns))
		for i, col := range columns {
			al := col.Align
			if al == "" {
				al = AlignLeft
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			parts[i] = pad(cell, widths[i], al)
		}
		lines = append(lines, strings.Join(parts, "  "))
	}
	return strings.Join(lines, "\n")
}

// ProgressBar renderiza barra textual colorida: ███████░░░
func ProgressBar(percent float64, width int) string {
	if width <= 0 {
		width = 14
	}
	clamped := math.Min(100, math.Max(0, percent))
	filled := int(math.Round((clamped / 100) * float64(width)))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	if percent > 90 {
		return cRed(bar)
	}
	if percent >= 70 {
		return cYellow(bar)
	}
	return cGreen(bar)
}
