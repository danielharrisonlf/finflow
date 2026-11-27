package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"finflow/internal/domain"
)

const sessionTTL = 30 * 24 * time.Hour

// isoMillisLayout reproduz Date.prototype.toISOString(): UTC com exatamente 3 casas decimais.
const isoMillisLayout = "2006-01-02T15:04:05.000Z"

type AuthService struct{}

func NewAuthService() *AuthService {
	return &AuthService{}
}

func (a *AuthService) expectedPassword() string {
	return strings.TrimSpace(os.Getenv("FINFLOW_PASSWORD"))
}

func (a *AuthService) IsRequired() bool {
	return a.expectedPassword() != ""
}

func (a *AuthService) signingSecret() string {
	if custom := strings.TrimSpace(os.Getenv("FINFLOW_AUTH_SECRET")); custom != "" {
		return custom
	}
	if pw := a.expectedPassword(); pw != "" {
		return pw
	}
	return "finflow-local-dev-secret"
}

func safeEqual(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

func (a *AuthService) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(a.signingSecret()))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *AuthService) Login(password string) (domain.LoginResponse, error) {
	// zod min(1): só a string vazia é inválida; espaços contam como senha (e falham na comparação).
	if password == "" {
		return domain.LoginResponse{}, ValidationFailed([]domain.ValidationIssue{
			{Path: "password", Message: "informe a senha"},
		})
	}
	if a.IsRequired() && !safeEqual(password, a.expectedPassword()) {
		return domain.LoginResponse{}, Unauthorized("senha incorreta")
	}
	expiresMs := time.Now().Add(sessionTTL).UnixMilli()
	nonceBytes := make([]byte, 12)
	_, _ = rand.Read(nonceBytes)
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	payload := fmt.Sprintf("%d.%s", expiresMs, nonce)
	sig := a.sign(payload)
	return domain.LoginResponse{
		Token:     payload + "." + sig,
		ExpiresAt: time.UnixMilli(expiresMs).UTC().Format(isoMillisLayout),
	}, nil
}

func (a *AuthService) VerifyToken(token string) bool {
	if !a.IsRequired() {
		return true
	}
	raw := strings.TrimSpace(token)
	if raw == "" {
		return false
	}
	if safeEqual(raw, a.expectedPassword()) {
		return true
	}
	if staticToken := strings.TrimSpace(os.Getenv("FINFLOW_API_TOKEN")); staticToken != "" && safeEqual(raw, staticToken) {
		return true
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return false
	}
	expiresMs, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || expiresMs <= time.Now().UTC().UnixMilli() {
		return false
	}
	expectedSig := a.sign(parts[0] + "." + parts[1])
	return safeEqual(parts[2], expectedSig)
}

func (a *AuthService) Status(token string) domain.AuthStatus {
	return domain.AuthStatus{
		Required:      a.IsRequired(),
		Authenticated: a.VerifyToken(token),
	}
}
