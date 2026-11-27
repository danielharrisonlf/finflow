package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func findTool(name string) (toolDef, bool) {
	for _, t := range allTools() {
		if t.Name == name {
			return t, true
		}
	}
	return toolDef{}, false
}

// validateAndNormalizeArgs confere os argumentos contra o InputSchema da tool (obrigatórios,
// tipos, enums, limites, tamanho mínimo e padrão), espelhando os schemas zod do servidor
// original. Devolve a lista de problemas ("" quando válido) e os argumentos com inteiros
// normalizados (4590.0 ou 1.2e3 viram 4590/1200), já que o zod aceitava essas formas e o
// json.Unmarshal em campos int as rejeitaria.
func validateAndNormalizeArgs(schema map[string]any, raw json.RawMessage) (json.RawMessage, string) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var args map[string]any
	if err := dec.Decode(&args); err != nil || args == nil {
		return raw, "arguments deve ser um objeto JSON"
	}

	var problems []string
	props, _ := schema["properties"].(map[string]any)
	required, _ := schema["required"].([]string)
	for _, key := range required {
		if v, ok := args[key]; !ok || v == nil {
			problems = append(problems, key+": obrigatório")
		}
	}

	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		prop, ok := props[key].(map[string]any)
		if !ok || args[key] == nil {
			continue
		}
		if msg := validateValue(prop, args[key]); msg != "" {
			problems = append(problems, key+": "+msg)
			continue
		}
		if prop["type"] == "integer" {
			if num, ok := args[key].(json.Number); ok {
				if f, err := num.Float64(); err == nil {
					args[key] = json.Number(strconv.FormatInt(int64(f), 10))
				}
			}
		}
	}
	if len(problems) > 0 {
		return raw, strings.Join(problems, "; ")
	}
	normalized, err := json.Marshal(args)
	if err != nil {
		return raw, "arguments deve ser um objeto JSON"
	}
	return normalized, ""
}

func validateValue(prop map[string]any, v any) string {
	switch prop["type"] {
	case "string":
		str, ok := v.(string)
		if !ok {
			return "deve ser texto"
		}
		if min, ok := prop["minLength"].(int); ok && utf8.RuneCountInString(str) < min {
			return fmt.Sprintf("deve ter ao menos %d caractere(s)", min)
		}
		if pattern, ok := prop["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(str) {
			return "formato inválido (" + pattern + ")"
		}
		if enum, ok := prop["enum"].([]string); ok {
			for _, e := range enum {
				if e == str {
					return ""
				}
			}
			return "deve ser um de: " + strings.Join(enum, ", ")
		}
	case "integer":
		num, ok := v.(json.Number)
		if !ok {
			return "deve ser um número inteiro"
		}
		f, err := num.Float64()
		if err != nil || f != math.Trunc(f) || math.IsInf(f, 0) {
			return "deve ser um número inteiro"
		}
		if min, ok := prop["minimum"].(int); ok && f < float64(min) {
			return fmt.Sprintf("deve ser >= %d", min)
		}
		if max, ok := prop["maximum"].(int); ok && f > float64(max) {
			return fmt.Sprintf("deve ser <= %d", max)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return "deve ser booleano"
		}
	}
	return ""
}
