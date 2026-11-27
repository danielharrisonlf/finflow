package domain

import (
	"encoding/json"
	"html"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ───────────────────────── Prévia de link de desejo ─────────────────────────

type LinkPreviewInput struct {
	URL string `json:"url"`
}

type LinkPreview struct {
	Title      *string `json:"title"`
	PriceCents *int64  `json:"priceCents"`
	ImageURL   *string `json:"imageUrl"`
}

const (
	MaxLinkPreviewURLChars = 2000
	maxPreviewTitleChars   = 200
)

func ValidateLinkPreview(in LinkPreviewInput) []ValidationIssue {
	issue := []ValidationIssue{{Path: "url", Message: "informe uma URL http(s) válida"}}
	raw := strings.TrimSpace(in.URL)
	if raw == "" || runeLen(raw) > MaxLinkPreviewURLChars {
		return issue
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return issue
	}
	return nil
}

var (
	metaTagRe  = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	attrRe     = regexp.MustCompile(`(?is)([a-z_:.-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	titleTagRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	jsonLDRe   = regexp.MustCompile(`(?is)<script[^>]*type\s*=\s*["']?application/ld\+json["']?[^>]*>(.*?)</script>`)
)

func tagAttrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attrRe.FindAllStringSubmatch(tag, -1) {
		v := m[2]
		if v == "" {
			v = m[3]
		}
		if v == "" {
			v = m[4]
		}
		out[strings.ToLower(m[1])] = html.UnescapeString(v)
	}
	return out
}

func cleanTitle(s string) string {
	s = strings.Join(strings.Fields(html.UnescapeString(s)), " ")
	if utf8.RuneCountInString(s) > maxPreviewTitleChars {
		s = string([]rune(s)[:maxPreviewTitleChars])
	}
	return s
}

// ParsePriceCents lê preços como "1299.90", "1.299,90", "R$ 49,90" ou "1299".
func ParsePriceCents(raw string) (int64, bool) {
	s := strings.TrimSpace(brlPrefixRe.ReplaceAllString(strings.TrimSpace(raw), ""))
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ",") {
		return ParseBRLToCents(s)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, false
	}
	cents := math.Round(v * 100)
	if cents < 1 || cents > MaxParsedCents {
		return 0, false
	}
	return int64(cents), true
}

func priceFromJSON(v any) (int64, bool) {
	switch p := v.(type) {
	case float64:
		return ParsePriceCents(strconv.FormatFloat(p, 'f', -1, 64))
	case string:
		return ParsePriceCents(p)
	}
	return 0, false
}

// findOfferPrice procura offers.price (ou lowPrice) em qualquer nível do JSON-LD.
func findOfferPrice(v any, depth int) (int64, bool) {
	if depth > 8 {
		return 0, false
	}
	switch node := v.(type) {
	case []any:
		for _, it := range node {
			if c, ok := findOfferPrice(it, depth+1); ok {
				return c, true
			}
		}
	case map[string]any:
		if offers, ok := node["offers"]; ok {
			for _, o := range asList(offers) {
				if m, ok := o.(map[string]any); ok {
					for _, k := range []string{"price", "lowPrice"} {
						if c, ok := priceFromJSON(m[k]); ok {
							return c, true
						}
					}
					if spec, ok := m["priceSpecification"].(map[string]any); ok {
						if c, ok := priceFromJSON(spec["price"]); ok {
							return c, true
						}
					}
				}
			}
		}
		for _, k := range []string{"@graph", "mainEntity", "itemOffered"} {
			if c, ok := findOfferPrice(node[k], depth+1); ok {
				return c, true
			}
		}
	}
	return 0, false
}

func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{v}
}

func resolveImageURL(base *url.URL, raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	if base != nil {
		ref = base.ResolveReference(ref)
	}
	if ref.Scheme != "http" && ref.Scheme != "https" {
		return nil
	}
	s := ref.String()
	return &s
}

// ParseLinkPreview extrai título (og:title, twitter:title ou <title>), preço (product:price:amount,
// og:price:amount, itemprop=price ou JSON-LD offers.price) e imagem (og:image) de um HTML.
// baseURL resolve imagens relativas (normalmente a URL final após redirecionamentos).
func ParseLinkPreview(body string, baseURL *url.URL) LinkPreview {
	meta := map[string]string{}
	for _, tag := range metaTagRe.FindAllString(body, -1) {
		attrs := tagAttrs(tag)
		content, ok := attrs["content"]
		if !ok {
			continue
		}
		for _, k := range []string{"property", "name", "itemprop"} {
			if key := strings.ToLower(strings.TrimSpace(attrs[k])); key != "" {
				if _, seen := meta[key]; !seen {
					meta[key] = content
				}
			}
		}
	}

	var out LinkPreview
	for _, k := range []string{"og:title", "twitter:title"} {
		if t := cleanTitle(meta[k]); t != "" {
			out.Title = &t
			break
		}
	}
	if out.Title == nil {
		if m := titleTagRe.FindStringSubmatch(body); m != nil {
			if t := cleanTitle(m[1]); t != "" {
				out.Title = &t
			}
		}
	}

	for _, k := range []string{"product:price:amount", "og:price:amount", "price"} {
		if c, ok := ParsePriceCents(meta[k]); ok {
			out.PriceCents = &c
			break
		}
	}
	if out.PriceCents == nil {
		for _, m := range jsonLDRe.FindAllStringSubmatch(body, -1) {
			var doc any
			if json.Unmarshal([]byte(strings.TrimSpace(m[1])), &doc) != nil {
				continue
			}
			if c, ok := findOfferPrice(doc, 0); ok {
				out.PriceCents = &c
				break
			}
		}
	}

	for _, k := range []string{"og:image:secure_url", "og:image", "twitter:image"} {
		if img := resolveImageURL(baseURL, meta[k]); img != nil {
			out.ImageURL = img
			break
		}
	}
	return out
}
