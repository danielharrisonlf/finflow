package domain

import "strings"

// ───────────────────────── Tags ─────────────────────────

// TagRef é a tag resumida que acompanha cada transação.
type TagRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Tag struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	UsageCount int    `json:"usageCount"`
}

type CreateTagInput struct {
	Name  string  `json:"name"`
	Color *string `json:"color,omitempty"`
}

type UpdateTagInput struct {
	Name  Optional[string] `json:"name"`
	Color Optional[string] `json:"color"`
}

// TagSpending é o gasto (EXPENSE + CARD_EXPENSE) de uma tag no mês.
type TagSpending struct {
	TagID      string `json:"tagId"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	TotalCents int64  `json:"totalCents"`
	Count      int    `json:"count"`
}

const (
	// DefaultTagColor é a cor usada quando a tag é criada sem cor.
	DefaultTagColor = "#64748b"
	MaxTagNameChars = 40
	// MaxTagsPerTransaction limita o conjunto de tags enviado em uma transação.
	MaxTagsPerTransaction = 20
)

var tagNameIssue = ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 40 caracteres"}

func ValidateCreateTag(in CreateTagInput) []ValidationIssue {
	var issues []ValidationIssue
	name := strings.TrimSpace(in.Name)
	if runeLen(name) < 1 || runeLen(name) > MaxTagNameChars {
		issues = append(issues, tagNameIssue)
	}
	if in.Color != nil && !IsValidHexColor(*in.Color) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}

func ValidateUpdateTag(in UpdateTagInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set {
		name := strings.TrimSpace(in.Name.Value)
		if !in.Name.Valid || runeLen(name) < 1 || runeLen(name) > MaxTagNameChars {
			issues = append(issues, tagNameIssue)
		}
	}
	if in.Color.Set && (!in.Color.Valid || !IsValidHexColor(in.Color.Value)) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}

// ValidateTagIDs confere o conjunto de tags de uma transação (UUIDs, no máximo 20).
func ValidateTagIDs(ids []string) []ValidationIssue {
	if len(ids) > MaxTagsPerTransaction {
		return []ValidationIssue{{Path: "tagIds", Message: "no máximo 20 tags por lançamento"}}
	}
	for _, id := range ids {
		if !IsValidUUID(id) {
			return []ValidationIssue{{Path: "tagIds", Message: "tagIds deve conter UUIDs válidos"}}
		}
	}
	return nil
}

// UniqueStrings remove repetições mantendo a ordem.
func UniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
