package spreadsheet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// decodeText converte o arquivo para UTF-8: BOM UTF-8/UTF-16 é respeitado; bytes inválidos em UTF-8
// são lidos como Windows-1252 (Excel e bancos brasileiros).
func decodeText(data []byte) (string, string) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return string(data[3:]), "UTF-8 (BOM)"
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		dec := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder()
		out, _, err := transform.Bytes(dec, data)
		if err == nil {
			return string(out), "UTF-16"
		}
	}
	if utf8.Valid(data) {
		return string(data), "UTF-8"
	}
	out, _, err := transform.Bytes(charmap.Windows1252.NewDecoder(), data)
	if err != nil {
		return string(data), "desconhecida"
	}
	return string(out), "Windows-1252"
}

// detectDelimiter escolhe ';', ',' ou tab pela contagem consistente (fora de aspas) nas primeiras linhas.
func detectDelimiter(text string) rune {
	lines := strings.Split(text, "\n")
	candidates := []rune{';', '\t', ','}
	bestDelim, bestScore := ';', -1
	for _, d := range candidates {
		counts := map[int]int{}
		sampled := 0
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			n := countOutsideQuotes(line, d)
			counts[n]++
			sampled++
			if sampled >= 15 {
				break
			}
		}
		// Pontuação: linhas que concordam com a contagem mais comum (> 0), ponderada pelo nº de colunas.
		score := 0
		for n, c := range counts {
			if n > 0 && c*100+n > score {
				score = c*100 + n
			}
		}
		if score > bestScore {
			bestDelim, bestScore = d, score
		}
	}
	return bestDelim
}

func countOutsideQuotes(line string, d rune) int {
	inQuotes := false
	n := 0
	for _, r := range line {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case r == d && !inQuotes:
			n++
		}
	}
	return n
}

func parseCSV(data []byte, opts Options) (*Result, error) {
	text, _ := decodeText(data)
	if strings.ContainsRune(text, '\x00') {
		return nil, ErrUnsupported
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	res := newResult(FormatCSV)
	// Dica de separador do Excel ("sep=;") na primeira linha.
	delim := rune(0)
	if first, rest, ok := strings.Cut(text, "\n"); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(first)), "sep=") {
		if r := []rune(strings.TrimSpace(first)[4:]); len(r) > 0 {
			delim = r[0]
		}
		text = rest
	}
	if delim == 0 {
		delim = detectDelimiter(text)
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = delim
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	var rows [][]string
	for {
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV inválido: %w", err)
		}
		rows = append(rows, rec)
		if len(rows) > maxImportRows+50 {
			break
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("arquivo vazio")
	}
	parseTable(rows, opts.Mapping, false, res)
	return res, nil
}
