package client

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"

	"finflow/internal/service"
	"finflow/internal/spreadsheet"
)

// SpreadsheetBackend reúne exportação da planilha anual e importação de planilhas/extratos.
type SpreadsheetBackend interface {
	ExportYearWorkbook(ctx context.Context, year *int) ([]byte, error)
	ImportPreview(ctx context.Context, in service.ImportPreviewInput) (*spreadsheet.Result, error)
	ImportCommit(ctx context.Context, in service.ImportCommitInput) (service.ImportCommitResult, error)
}

// ───────────────────────── Local Backend ─────────────────────────

func (l *LocalBackend) ExportYearWorkbook(ctx context.Context, year *int) ([]byte, error) {
	content, _, err := l.Svcs.ExportYearWorkbook(ctx, year)
	return content, err
}
func (l *LocalBackend) ImportPreview(ctx context.Context, in service.ImportPreviewInput) (*spreadsheet.Result, error) {
	return l.Svcs.ImportPreview(ctx, in)
}
func (l *LocalBackend) ImportCommit(ctx context.Context, in service.ImportCommitInput) (service.ImportCommitResult, error) {
	return l.Svcs.ImportCommit(ctx, in)
}

// ───────────────────────── HTTP Backend ─────────────────────────

func (c *HTTPClient) ExportYearWorkbook(ctx context.Context, year *int) ([]byte, error) {
	p := "/spreadsheet/export.xlsx"
	if year != nil {
		p += "?year=" + strconv.Itoa(*year)
	}
	var out string
	if err := c.do(ctx, http.MethodGet, p, nil, &out); err != nil {
		return nil, err
	}
	return []byte(out), nil
}

func (c *HTTPClient) ImportPreview(ctx context.Context, in service.ImportPreviewInput) (*spreadsheet.Result, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	name := filepath.Base(in.Filename)
	if name == "." || name == "" {
		name = "arquivo"
	}
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(in.Data); err != nil {
		return nil, err
	}
	if in.Sheet != "" {
		if err := w.WriteField("sheet", in.Sheet); err != nil {
			return nil, err
		}
	}
	if in.AccountID != nil {
		if err := w.WriteField("accountId", *in.AccountID); err != nil {
			return nil, err
		}
	}
	if in.Mapping != nil {
		raw, err := json.Marshal(in.Mapping)
		if err != nil {
			return nil, err
		}
		if err := w.WriteField("mapping", string(raw)); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	var out spreadsheet.Result
	if err := c.doRaw(ctx, http.MethodPost, "/import/preview", w.FormDataContentType(), &body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPClient) ImportCommit(ctx context.Context, in service.ImportCommitInput) (service.ImportCommitResult, error) {
	var out service.ImportCommitResult
	return out, c.do(ctx, http.MethodPost, "/import/commit", in, &out)
}
