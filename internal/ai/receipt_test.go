package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"finflow/internal/domain"
	"finflow/internal/service"
)

// pngHeader é o início de um PNG válido (suficiente para http.DetectContentType).
var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00")

func receiptJSONResponse(t *testing.T, stopReason string, out map[string]any) fakeResponse {
	t.Helper()
	text, _ := json.Marshal(out)
	body, _ := json.Marshal(map[string]any{
		"id": "msg_r", "type": "message", "role": "assistant", "model": Model,
		"content":     []any{map[string]any{"type": "text", "text": string(text)}},
		"stop_reason": stopReason, "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": 100, "output_tokens": 50},
	})
	return fakeResponse{json: string(body)}
}

func TestExtractReceipt(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t, receiptJSONResponse(t, "end_turn", map[string]any{
		"description": "Supermercado Bom Preço", "amountCents": 12345, "date": "2026-11-25", "kind": "EXPENSE",
		"categoryName": "alimentacao", "merchant": "BOM PRECO LTDA", "installments": nil, "confidence": "HIGH",
	}))
	svc, svcs := newTestService(t, api, true)
	cat, err := svcs.CreateCategory(ctx, domain.CreateCategoryInput{Name: "Alimentação", Kind: domain.CategoryExpense, Icon: "utensils", Color: "#22c55e"})
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.ExtractReceipt(ctx, pngHeader, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if res.AmountCents != 12345 || res.Date != "2026-11-25" || res.Kind != "EXPENSE" || res.Confidence != "HIGH" ||
		res.SuggestedCategoryID == nil || *res.SuggestedCategoryID != cat.ID || res.Merchant == nil || res.Installments != nil {
		t.Fatalf("resultado: %+v", res)
	}

	req := api.calls()[0]
	if req.body["stream"] == true || req.body["fallbacks"] != "default" {
		t.Fatalf("receipt: stream/fallbacks %v %v", req.body["stream"], req.body["fallbacks"])
	}
	oc := req.body["output_config"].(map[string]any)
	format := oc["format"].(map[string]any)
	if oc["effort"] != "low" || format["type"] != "json_schema" || format["schema"] == nil {
		t.Fatalf("output_config: %v", oc)
	}
	if _, ok := req.body["thinking"]; ok {
		t.Fatal("thinking deve ser omitido (adaptativo)")
	}
	content := req.body["messages"].([]any)[0].(map[string]any)["content"].([]any)
	img := content[0].(map[string]any)
	if img["type"] != "image" || img["source"].(map[string]any)["media_type"] != "image/png" {
		t.Fatalf("bloco de imagem: %v", img)
	}
	if !strings.Contains(content[1].(map[string]any)["text"].(string), "Alimentação") {
		t.Fatal("lista de categorias deveria ir no turno do usuário")
	}
}

func TestExtractReceiptRefusalAndPDF(t *testing.T) {
	api := newFakeAPI(t, receiptJSONResponse(t, "refusal", map[string]any{}))
	svc, _ := newTestService(t, api, true)
	_, err := svc.ExtractReceipt(context.Background(), []byte("%PDF-1.4\n"), "application/pdf")
	var appErr *service.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("recusa deveria virar 422: %v", err)
	}
	doc := api.calls()[0].body["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)
	if doc["type"] != "document" {
		t.Fatalf("PDF deveria ir como document: %v", doc)
	}
}

func TestExtractReceiptNotConfigured(t *testing.T) {
	svc, _ := newTestService(t, nil, false)
	_, err := svc.ExtractReceipt(context.Background(), pngHeader, "image/png")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("esperava ErrNotConfigured: %v", err)
	}
}

func TestDetectReceiptMedia(t *testing.T) {
	if mt, err := DetectReceiptMedia(pngHeader, "foto.png"); err != nil || mt != "image/png" {
		t.Fatalf("png: %s %v", mt, err)
	}
	if mt, err := DetectReceiptMedia([]byte("%PDF-1.7\n..."), "nota.pdf"); err != nil || mt != "application/pdf" {
		t.Fatalf("pdf: %s %v", mt, err)
	}
	heic := append([]byte{0, 0, 0, 0x18}, []byte("ftypheic\x00\x00\x00\x00")...)
	var appErr *service.AppError
	if _, err := DetectReceiptMedia(heic, "IMG_0001.HEIC"); !errors.As(err, &appErr) || appErr.Status != http.StatusUnsupportedMediaType || !strings.Contains(appErr.Message, "HEIC") {
		t.Fatalf("heic: %v", err)
	}
	if _, err := DetectReceiptMedia([]byte("a,b,c\n1,2,3"), "planilha.csv"); !errors.As(err, &appErr) || appErr.Status != http.StatusUnsupportedMediaType {
		t.Fatalf("csv: %v", err)
	}
	if _, err := DetectReceiptMedia(nil, "x.png"); err == nil {
		t.Fatal("vazio deveria falhar")
	}
}

func TestNormalizeReceipt(t *testing.T) {
	inc := domain.Category{ID: "c-inc", Name: "Salário", Kind: domain.CategoryIncome}
	exp := domain.Category{ID: "c-exp", Name: "Salário", Kind: domain.CategoryExpense}
	name := "SALARIO"
	three := 3
	got := normalizeReceipt(receiptRaw{Description: "", AmountCents: -500, Date: "25/11/2026", Kind: "INCOME",
		CategoryName: &name, Installments: &three, Confidence: "???"}, []domain.Category{exp, inc})
	if got.AmountCents != 500 || got.Date != domain.TodayString() || got.Confidence != "LOW" || got.Description != "Comprovante" ||
		got.SuggestedCategoryID == nil || *got.SuggestedCategoryID != "c-inc" || got.Installments == nil || *got.Installments != 3 {
		t.Fatalf("normalização: %+v", got)
	}
}
