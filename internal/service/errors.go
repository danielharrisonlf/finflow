package service

import (
	"net/http"
	"strings"

	"finflow/internal/domain"
)

type AppError struct {
	Status  int                      `json:"statusCode"`
	Message string                   `json:"message"`
	Issues  []domain.ValidationIssue `json:"issues,omitempty"`
}

// Error inclui os issues de validação — a CLI e o MCP imprimem só esta string.
// Ex.: "dados inválidos (date: data deve estar no formato YYYY-MM-DD; amountCents: valor deve ser positivo)".
func (e *AppError) Error() string {
	if len(e.Issues) == 0 {
		return e.Message
	}
	parts := make([]string, 0, len(e.Issues))
	for _, is := range e.Issues {
		if is.Path == "" {
			parts = append(parts, is.Message)
			continue
		}
		parts = append(parts, is.Path+": "+is.Message)
	}
	return e.Message + " (" + strings.Join(parts, "; ") + ")"
}

func BadRequest(msg string) *AppError {
	return &AppError{Status: http.StatusBadRequest, Message: msg}
}

func ValidationFailed(issues []domain.ValidationIssue) *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Message: "dados inválidos",
		Issues:  issues,
	}
}

func Unauthorized(msg string) *AppError {
	return &AppError{Status: http.StatusUnauthorized, Message: msg}
}

func Forbidden(msg string) *AppError {
	return &AppError{Status: http.StatusForbidden, Message: msg}
}

func NotFound(msg string) *AppError {
	return &AppError{Status: http.StatusNotFound, Message: msg}
}

func Conflict(msg string) *AppError {
	return &AppError{Status: http.StatusConflict, Message: msg}
}
