package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"finflow/internal/domain"
	"finflow/internal/service"
)

// MaxReceiptBytes é o tamanho máximo do arquivo de comprovante (10 MiB).
const MaxReceiptBytes = 10 << 20

const receiptMaxTokens = 16000

// ReceiptResult é a resposta de POST /api/ai/receipt.
type ReceiptResult struct {
	Description         string  `json:"description"`
	AmountCents         int64   `json:"amountCents"`
	Date                string  `json:"date"`
	Kind                string  `json:"kind"`
	SuggestedCategoryID *string `json:"suggestedCategoryId"`
	Merchant            *string `json:"merchant"`
	Installments        *int    `json:"installments"`
	Confidence          string  `json:"confidence"`
}

// DetectReceiptMedia identifica o tipo pelo conteúdo (não confia no Content-Type do cliente).
// Aceita JPEG, PNG, WebP, GIF e PDF; HEIC e outros formatos são recusados com 415.
func DetectReceiptMedia(data []byte, filename string) (string, error) {
	if len(data) == 0 {
		return "", service.BadRequest("arquivo vazio")
	}
	if len(data) > MaxReceiptBytes {
		return "", &service.AppError{Status: http.StatusRequestEntityTooLarge, Message: "arquivo maior que 10 MiB"}
	}
	switch mt := http.DetectContentType(data); mt {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf":
		return mt, nil
	}
	ext := strings.ToLower(path.Ext(filename))
	if ext == ".heic" || ext == ".heif" || isHEIC(data) {
		return "", &service.AppError{Status: http.StatusUnsupportedMediaType,
			Message: "Fotos HEIC não são aceitas. No iPhone, exporte como JPEG (ou ative Ajustes → Câmera → Formatos → Mais compatível) e envie de novo."}
	}
	return "", &service.AppError{Status: http.StatusUnsupportedMediaType,
		Message: "Formato não suportado. Envie uma foto JPEG, PNG, WebP ou GIF, ou um PDF."}
}

func isHEIC(data []byte) bool {
	if len(data) < 12 || string(data[4:8]) != "ftyp" {
		return false
	}
	brand := string(data[8:12])
	return brand == "heic" || brand == "heix" || brand == "mif1" || brand == "msf1" || brand == "hevc"
}

func nullable(t string) map[string]any {
	return map[string]any{"anyOf": []any{map[string]any{"type": t}, map[string]any{"type": "null"}}}
}

// receiptSchema restringe a saída (structured outputs).
func receiptSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"description":  map[string]any{"type": "string"},
			"amountCents":  map[string]any{"type": "integer"},
			"date":         map[string]any{"type": "string", "format": "date"},
			"kind":         map[string]any{"type": "string", "enum": []string{"EXPENSE", "INCOME"}},
			"categoryName": nullable("string"),
			"merchant":     nullable("string"),
			"installments": nullable("integer"),
			"confidence":   map[string]any{"type": "string", "enum": []string{"HIGH", "MEDIUM", "LOW"}},
		},
		"required": []string{"description", "amountCents", "date", "kind", "categoryName", "merchant", "installments", "confidence"},
	}
}

type receiptRaw struct {
	Description  string  `json:"description"`
	AmountCents  int64   `json:"amountCents"`
	Date         string  `json:"date"`
	Kind         string  `json:"kind"`
	CategoryName *string `json:"categoryName"`
	Merchant     *string `json:"merchant"`
	Installments *int    `json:"installments"`
	Confidence   string  `json:"confidence"`
}

func (s *Service) categoryList(ctx context.Context) ([]domain.Category, string, error) {
	cats, err := s.svcs.ListCategories(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	var exp, inc []string
	for _, c := range cats {
		if c.Kind == domain.CategoryIncome {
			inc = append(inc, c.Name)
		} else {
			exp = append(exp, c.Name)
		}
	}
	text := fmt.Sprintf("Categorias de despesa (EXPENSE): %s\nCategorias de receita (INCOME): %s",
		strings.Join(exp, "; "), strings.Join(inc, "; "))
	return cats, text, nil
}

// ExtractReceipt lê um comprovante (imagem ou PDF) com structured outputs e mapeia a categoria pelo nome.
func (s *Service) ExtractReceipt(ctx context.Context, data []byte, mediaType string) (ReceiptResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ReceiptResult{}, err
	}
	cats, catText, err := s.categoryList(ctx)
	if err != nil {
		return ReceiptResult{}, err
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	var fileBlock anthropic.BetaContentBlockParamUnion
	if mediaType == "application/pdf" {
		fileBlock = anthropic.NewBetaDocumentBlock(anthropic.BetaBase64PDFSourceParam{Data: b64})
	} else {
		fileBlock = anthropic.NewBetaImageBlock(anthropic.BetaBase64ImageSourceParam{
			Data: b64, MediaType: anthropic.BetaBase64ImageSourceMediaType(mediaType),
		})
	}
	prompt := todayContext(domain.Now()) + "\n" + catText + "\n\nExtraia os dados deste comprovante."

	msg, err := client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     Model,
		MaxTokens: receiptMaxTokens,
		System:    []anthropic.BetaTextBlockParam{{Text: receiptSystemPrompt}},
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortLow,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: receiptSchema()},
		},
		Betas:     []anthropic.AnthropicBeta{fallbackBeta},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
		Messages:  []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(fileBlock, anthropic.NewBetaTextBlock(prompt))},
	})
	if err != nil {
		return ReceiptResult{}, upstreamError(err)
	}
	switch msg.StopReason {
	case anthropic.BetaStopReasonRefusal:
		return ReceiptResult{}, &service.AppError{Status: http.StatusUnprocessableEntity, Message: "Não foi possível ler este arquivo. Envie a foto de um comprovante ou nota fiscal."}
	case anthropic.BetaStopReasonMaxTokens:
		return ReceiptResult{}, &service.AppError{Status: http.StatusBadGateway, Message: "A leitura do comprovante foi interrompida. Tente uma foto mais nítida."}
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	var raw receiptRaw
	if err := json.Unmarshal([]byte(text.String()), &raw); err != nil {
		return ReceiptResult{}, &service.AppError{Status: http.StatusBadGateway, Message: "A resposta da leitura do comprovante veio em formato inesperado. Tente de novo."}
	}
	return normalizeReceipt(raw, cats), nil
}

// normalizeReceipt saneia a saída do modelo e resolve a categoria pelo nome (sem acento/maiúsculas).
func normalizeReceipt(raw receiptRaw, cats []domain.Category) ReceiptResult {
	out := ReceiptResult{
		Description: strings.TrimSpace(raw.Description),
		AmountCents: raw.AmountCents,
		Date:        raw.Date,
		Kind:        raw.Kind,
		Confidence:  raw.Confidence,
	}
	if out.Kind != string(domain.TxIncome) {
		out.Kind = string(domain.TxExpense)
	}
	if out.AmountCents < 0 {
		out.AmountCents = -out.AmountCents
	}
	if !domain.IsValidDateString(out.Date) {
		out.Date = domain.TodayString()
	}
	switch out.Confidence {
	case "HIGH", "MEDIUM", "LOW":
	default:
		out.Confidence = "LOW"
	}
	if raw.Merchant != nil && strings.TrimSpace(*raw.Merchant) != "" {
		m := strings.TrimSpace(*raw.Merchant)
		out.Merchant = &m
	}
	if out.Description == "" {
		if out.Merchant != nil {
			out.Description = *out.Merchant
		} else {
			out.Description = "Comprovante"
		}
	}
	if r := []rune(out.Description); len(r) > 120 {
		out.Description = string(r[:120])
	}
	if raw.Installments != nil && *raw.Installments >= 2 && *raw.Installments <= 48 {
		n := *raw.Installments
		out.Installments = &n
	}
	if raw.CategoryName != nil {
		want := foldName(*raw.CategoryName)
		kind := domain.CategoryExpense
		if out.Kind == string(domain.TxIncome) {
			kind = domain.CategoryIncome
		}
		for _, c := range cats {
			if c.Kind == kind && foldName(c.Name) == want {
				id := c.ID
				out.SuggestedCategoryID = &id
				break
			}
		}
	}
	return out
}

func foldName(s string) string {
	return domain.NormalizeText(s)
}
