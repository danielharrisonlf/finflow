package service

import (
	"testing"
)

func TestParseTelegramMessage(t *testing.T) {
	if got := ParseTelegramMessage("/saldo"); got.Kind != "balance" {
		t.Fatalf("/saldo -> %+v", got)
	}
	if got := ParseTelegramMessage("/resumo@finflow_bot"); got.Kind != "summary" {
		t.Fatalf("/resumo@bot -> %+v", got)
	}
	if got := ParseTelegramMessage("/extrato"); got.Kind != "statement" {
		t.Fatalf("/extrato -> %+v", got)
	}
	if got := ParseTelegramMessage("/ajuda"); got.Kind != "help" {
		t.Fatalf("/ajuda -> %+v", got)
	}
	got := ParseTelegramMessage("45,90 almoço executivo #alimentação")
	if got.Kind != "expense" || got.AmountCents != 4590 || got.Description != "almoço executivo" || got.CategoryName != "alimentação" {
		t.Fatalf("quick expense -> %+v", got)
	}
	if got := ParseTelegramMessage("olá tudo bem"); got.Kind != "unknown" {
		t.Fatalf("unknown -> %+v", got)
	}
}

func TestAuthService(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "")
	t.Setenv("FINFLOW_AUTH_SECRET", "")
	t.Setenv("FINFLOW_API_TOKEN", "")

	auth := NewAuthService()
	if auth.IsRequired() {
		t.Fatal("expected auth disabled without FINFLOW_PASSWORD")
	}
	if !auth.VerifyToken("") {
		t.Fatal("expected open mode to accept empty token")
	}

	t.Setenv("FINFLOW_PASSWORD", "segredo-forte")
	t.Setenv("FINFLOW_API_TOKEN", "api-tok-123")
	if !auth.IsRequired() {
		t.Fatal("expected auth enabled")
	}
	if auth.VerifyToken("") || auth.VerifyToken("errado") {
		t.Fatal("expected invalid token rejected")
	}
	if !auth.VerifyToken("segredo-forte") || !auth.VerifyToken("api-tok-123") {
		t.Fatal("expected direct password and static API token accepted")
	}

	if _, err := auth.Login("errado"); err == nil {
		t.Fatal("expected login error for wrong password")
	}
	res, err := auth.Login("segredo-forte")
	if err != nil || res.Token == "" {
		t.Fatalf("login failed: %v", err)
	}
	if !auth.VerifyToken(res.Token) {
		t.Fatal("expected signed session token accepted")
	}
}
