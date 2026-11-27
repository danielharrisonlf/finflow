package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"finflow/internal/service"
	"finflow/internal/spreadsheet"
)

// MaxImportBodyBytes é o limite de corpo das rotas de importação (10 MiB).
const MaxImportBodyBytes int64 = 10 << 20

// importRoutes recebem arquivos/lotes grandes e usam MaxImportBodyBytes no lugar de MaxBodyBytes.
var importRoutes = map[string]bool{
	"/api/import/preview": true,
	"/api/import/commit":  true,
}

// bodyLimitFor devolve o limite de corpo da requisição (usado por limitBodyMiddleware).
func bodyLimitFor(r *http.Request) int64 {
	if r.Method == http.MethodPost && importRoutes[r.URL.Path] {
		return MaxImportBodyBytes
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/ai/receipt" {
		return MaxReceiptBodyBytes
	}
	return MaxBodyBytes
}

func (s *Server) registerSpreadsheetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/spreadsheet/export.xlsx", s.requireAuth(s.handleSpreadsheetExport))
	mux.HandleFunc("GET /api/spreadsheet/template.xlsx", s.requireAuth(s.handleSpreadsheetTemplate))
	mux.HandleFunc("POST /api/import/preview", s.requireAuth(s.handleImportPreview))
	mux.HandleFunc("POST /api/import/commit", s.requireAuth(s.handleImportCommit))
}

func writeXLSX(w http.ResponseWriter, content []byte, filename string) {
	w.Header().Set("Content-Type", service.XLSXContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) handleSpreadsheetExport(w http.ResponseWriter, r *http.Request) {
	year, err := parseOptionalInt(r.URL.Query().Get("year"), "year", 2000, 2100)
	if err != nil {
		writeError(w, r, err)
		return
	}
	content, filename, err := s.svcs.ExportYearWorkbook(r.Context(), year)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeXLSX(w, content, filename)
}

func (s *Server) handleSpreadsheetTemplate(w http.ResponseWriter, r *http.Request) {
	content, filename, err := s.svcs.ImportTemplate()
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeXLSX(w, content, filename)
}

func tooLargeError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &service.AppError{
			Status:  http.StatusRequestEntityTooLarge,
			Message: fmt.Sprintf("arquivo excede %d MiB", tooLarge.Limit>>20),
		}
	}
	return nil
}

func (s *Server) handleImportPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(MaxImportBodyBytes); err != nil {
		if tl := tooLargeError(err); tl != nil {
			writeError(w, r, tl)
			return
		}
		writeError(w, r, service.BadRequest("envie o arquivo como multipart/form-data no campo file"))
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, service.BadRequest("envie o arquivo como multipart/form-data no campo file"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxImportBodyBytes+1))
	if err != nil {
		writeError(w, r, err)
		return
	}
	if int64(len(data)) > MaxImportBodyBytes {
		writeError(w, r, &service.AppError{Status: http.StatusRequestEntityTooLarge, Message: "arquivo excede 10 MiB"})
		return
	}

	in := service.ImportPreviewInput{
		Data:     data,
		Filename: header.Filename,
		Sheet:    strings.TrimSpace(r.FormValue("sheet")),
	}
	if raw := strings.TrimSpace(r.FormValue("mapping")); raw != "" && raw != "null" {
		var m spreadsheet.Mapping
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			writeError(w, r, service.BadRequest("mapping deve ser um JSON válido: "+err.Error()))
			return
		}
		in.Mapping = &m
	}
	accountID := strings.TrimSpace(r.FormValue("accountId"))
	if accountID == "" {
		accountID = strings.TrimSpace(r.URL.Query().Get("accountId"))
	}
	if accountID != "" {
		in.AccountID = &accountID
	}
	out, err := s.svcs.ImportPreview(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleImportCommit(w http.ResponseWriter, r *http.Request) {
	var in service.ImportCommitInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ImportCommit(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
