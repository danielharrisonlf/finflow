package domain

import (
	"regexp"
	"strings"
)

// ───────────────────────── Espaços (v3 fase 2 §1–§3) ─────────────────────────

type SpaceKind string

const (
	SpacePersonal SpaceKind = "PERSONAL"
	SpaceHome     SpaceKind = "HOME"
	SpaceBusiness SpaceKind = "BUSINESS"
)

func (k SpaceKind) Valid() bool {
	switch k {
	case SpacePersonal, SpaceHome, SpaceBusiness:
		return true
	}
	return false
}

// DefaultSpaceSlug é o espaço do banco atual: sempre existe, não é movido nem apagado.
const DefaultSpaceSlug = "pessoal"

// MaxSpaceNameChars limita o nome de um espaço.
const MaxSpaceNameChars = 60

// DefaultSpaceColor devolve a cor padrão de cada tipo de espaço.
func DefaultSpaceColor(kind SpaceKind) string {
	switch kind {
	case SpaceHome:
		return "#f59e0b"
	case SpaceBusiness:
		return "#6366f1"
	}
	return "#10b981"
}

type Space struct {
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Kind      SpaceKind `json:"kind"`
	Color     string    `json:"color"`
	CreatedAt string    `json:"createdAt"`
	IsDefault bool      `json:"isDefault"`
}

// CreateSpaceInput.Slug ausente: derivado do nome (com sufixo -2, -3… se já existir).
type CreateSpaceInput struct {
	Name  string    `json:"name"`
	Kind  SpaceKind `json:"kind"`
	Color *string   `json:"color,omitempty"`
	Slug  *string   `json:"slug,omitempty"`
}

type UpdateSpaceInput struct {
	Name  Optional[string] `json:"name"`
	Color Optional[string] `json:"color"`
}

// SpaceTotals são os números de um espaço na visão unificada (mês corrente).
type SpaceTotals struct {
	BalanceCents     int64 `json:"balanceCents"`
	NetWorthCents    int64 `json:"netWorthCents"`
	MonthIncomeCents int64 `json:"monthIncomeCents"`
	MonthOutCents    int64 `json:"monthOutCents"`
	MonthResultCents int64 `json:"monthResultCents"`
}

// Add soma os totais de outro espaço.
func (t *SpaceTotals) Add(o SpaceTotals) {
	t.BalanceCents += o.BalanceCents
	t.NetWorthCents += o.NetWorthCents
	t.MonthIncomeCents += o.MonthIncomeCents
	t.MonthOutCents += o.MonthOutCents
	t.MonthResultCents += o.MonthResultCents
}

type SpaceOverviewItem struct {
	Slug  string    `json:"slug"`
	Name  string    `json:"name"`
	Kind  SpaceKind `json:"kind"`
	Color string    `json:"color"`
	SpaceTotals
}

type SpacesOverview struct {
	Spaces []SpaceOverviewItem `json:"spaces"`
	Totals SpaceTotals         `json:"totals"`
}

// spaceSlugRe é o formato do slug: também é o nome do arquivo do banco, então nada de
// pontos, barras ou maiúsculas (impede path traversal).
var spaceSlugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}$`)

// IsValidSpaceSlug confere o formato do slug de um espaço.
func IsValidSpaceSlug(slug string) bool {
	return spaceSlugRe.MatchString(slug)
}

// SlugifySpaceName deriva um slug do nome ("Casa da Praia" → "casa-da-praia"); devolve
// "espaco" quando o nome não tem letras nem dígitos aproveitáveis.
func SlugifySpaceName(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range NormalizeText(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if b.Len() > 0 && !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 31 {
		slug = strings.TrimRight(slug[:31], "-")
	}
	if !IsValidSpaceSlug(slug) {
		return "espaco"
	}
	return slug
}

var spaceNameIssue = ValidationIssue{Path: "name", Message: "nome deve ter entre 1 e 60 caracteres"}

func ValidateCreateSpace(in CreateSpaceInput) []ValidationIssue {
	var issues []ValidationIssue
	name := strings.TrimSpace(in.Name)
	if runeLen(name) < 1 || runeLen(name) > MaxSpaceNameChars {
		issues = append(issues, spaceNameIssue)
	}
	if !in.Kind.Valid() {
		issues = append(issues, ValidationIssue{Path: "kind", Message: "tipo deve ser PERSONAL, HOME ou BUSINESS"})
	}
	if in.Color != nil && !IsValidHexColor(*in.Color) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	if in.Slug != nil && !IsValidSpaceSlug(*in.Slug) {
		issues = append(issues, ValidationIssue{Path: "slug", Message: "slug deve ter de 2 a 31 caracteres: letras minúsculas, dígitos e hífen (sem começar com hífen)"})
	}
	return issues
}

func ValidateUpdateSpace(in UpdateSpaceInput) []ValidationIssue {
	var issues []ValidationIssue
	if in.Name.Set {
		name := strings.TrimSpace(in.Name.Value)
		if !in.Name.Valid || runeLen(name) < 1 || runeLen(name) > MaxSpaceNameChars {
			issues = append(issues, spaceNameIssue)
		}
	}
	if in.Color.Set && (!in.Color.Valid || !IsValidHexColor(in.Color.Value)) {
		issues = append(issues, ValidationIssue{Path: "color", Message: "cor deve ser hex #RRGGBB"})
	}
	return issues
}
