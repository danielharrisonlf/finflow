package httpapi_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/httpapi"
	"finflow/internal/service"
	"finflow/internal/spreadsheet"
	"finflow/internal/testutil"
)

func multipartReq(t *testing.T, srv http.Handler, target string, fields map[string]string, filename string, file []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if filename != "" {
		part, err := w.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(file); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func openXLSXResponse(t *testing.T, rr *httptest.ResponseRecorder) *excelize.File {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != service.XLSXContentType {
		t.Fatalf("content-type = %q", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, ".xlsx") {
		t.Fatalf("content-disposition = %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatalf("excelize não abriu: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestSpreadsheetImportFlow(t *testing.T) {
	testutil.ForEachDialect(t, runSpreadsheetImportFlow)
}

func runSpreadsheetImportFlow(t *testing.T, database *db.DB) {
	t.Setenv("FINFLOW_PASSWORD", "")
	srv := newTestServer(t, database, httpapi.ServerOptions{})
	res := doReq(t, srv, "POST", "/api/accounts", map[string]any{"name": "Conta Import HTTP", "type": "CHECKING", "initialBalanceCents": 0}, nil)
	if res.Code != 201 {
		t.Fatalf("create account: %d %s", res.Code, res.Body.String())
	}
	acc := decodeBody[domain.Account](t, res)

	csv := "Data;Histórico;Valor\n01/04/2026;Mercado do bairro;-150,30\n02/04/2026;Pix recebido;1.200,00\n02/04/2026;Pix recebido;1.200,00\n"
	rr := multipartReq(t, srv, "/api/import/preview", map[string]string{"accountId": acc.ID}, "extrato.csv", []byte(csv))
	if rr.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rr.Code, rr.Body.String())
	}
	preview := decodeBody[spreadsheet.Result](t, rr)
	if preview.Format != spreadsheet.FormatCSV || len(preview.Rows) != 3 || !preview.Rows[2].Duplicate {
		t.Fatalf("preview = %+v", preview)
	}
	// Formato JSON do contrato: chaves camelCase e mapping com null.
	for _, key := range []string{`"format":"CSV"`, `"sheets":[]`, `"sheet":null`, `"columns":["Data","Histórico","Valor"]`,
		`"mapping":{"date":0,"description":1,"amount":2,"debit":null`, `"suggestedCategoryId":null`, `"warnings":[]`} {
		if !strings.Contains(rr.Body.String(), key) {
			t.Errorf("resposta sem %s: %s", key, rr.Body.String())
		}
	}

	var rows []map[string]any
	for _, r := range preview.Rows {
		if r.Duplicate || r.Error != nil {
			continue
		}
		rows = append(rows, map[string]any{"date": r.Date, "description": r.Description, "amountCents": r.AmountCents, "kind": r.Kind})
	}
	rr = doReq(t, srv, "POST", "/api/import/commit", map[string]any{"accountId": acc.ID, "rows": rows}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("commit: %d %s", rr.Code, rr.Body.String())
	}
	commit := decodeBody[service.ImportCommitResult](t, rr)
	if commit.Created != 2 || commit.Skipped != 0 {
		t.Fatalf("commit = %+v", commit)
	}
	rr = doReq(t, srv, "GET", "/api/transactions?accountId="+acc.ID, nil, nil)
	list := decodeBody[domain.Paginated[domain.Transaction]](t, rr)
	if list.Total != 2 {
		t.Fatalf("transações = %+v", list)
	}
	// Reenviar o mesmo lote pula tudo.
	rr = doReq(t, srv, "POST", "/api/import/commit", map[string]any{"accountId": acc.ID, "rows": rows}, nil)
	if commit := decodeBody[service.ImportCommitResult](t, rr); commit.Created != 0 || commit.Skipped != 2 {
		t.Fatalf("recommit = %+v", commit)
	}
	// Preview agora marca as linhas como duplicadas na conta.
	rr = multipartReq(t, srv, "/api/import/preview?accountId="+acc.ID, nil, "extrato.csv", []byte(csv))
	if p := decodeBody[spreadsheet.Result](t, rr); !p.Rows[0].Duplicate || !p.Rows[1].Duplicate {
		t.Fatalf("duplicados na conta: %+v", p.Rows)
	}

	// Mapeamento manual e aba inexistente.
	rr = multipartReq(t, srv, "/api/import/preview", map[string]string{"mapping": `{"date":1,"description":0,"amount":2}`}, "x.csv",
		[]byte("Mercado;03/04/2026;-10,00\n"))
	if p := decodeBody[spreadsheet.Result](t, rr); rr.Code != 200 || len(p.Rows) != 1 || p.Rows[0].Description != "Mercado" {
		t.Fatalf("mapping: %d %s", rr.Code, rr.Body.String())
	}
	if rr := multipartReq(t, srv, "/api/import/preview", map[string]string{"mapping": `{oops`}, "x.csv", []byte("a")); rr.Code != 400 {
		t.Fatalf("mapping inválido: %d", rr.Code)
	}
	if rr := multipartReq(t, srv, "/api/import/preview", nil, "", nil); rr.Code != 400 {
		t.Fatalf("sem arquivo: %d %s", rr.Code, rr.Body.String())
	}
}

func TestImportPreviewBodyLimits(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "")
	srv := newTestServer(t, testutil.Open(t, db.DialectSQLite), httpapi.ServerOptions{})

	// ~2 MiB: acima do limite global (1 MiB), abaixo do limite de importação (10 MiB).
	var b strings.Builder
	b.WriteString("Data;Descrição;Valor;Obs\n")
	pad := strings.Repeat("x", 2000)
	for i := 0; i < 1100; i++ {
		fmt.Fprintf(&b, "%02d/05/2026;Item %d;-1,%02d;%s\n", i%28+1, i, i%100, pad)
	}
	if b.Len() <= int(httpapi.MaxBodyBytes) {
		t.Fatalf("arquivo de teste pequeno demais: %d", b.Len())
	}
	rr := multipartReq(t, srv, "/api/import/preview", nil, "grande.csv", []byte(b.String()))
	if rr.Code != http.StatusOK {
		t.Fatalf("2 MiB deveria passar: %d %s", rr.Code, rr.Body.String()[:min(300, rr.Body.Len())])
	}

	huge := bytes.Repeat([]byte("a"), int(httpapi.MaxImportBodyBytes)+1024)
	rr = multipartReq(t, srv, "/api/import/preview", nil, "enorme.csv", huge)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("> 10 MiB deveria dar 413: %d %s", rr.Code, rr.Body.String())
	}

	// Outras rotas continuam com 1 MiB.
	big := []byte(`{"name":"` + strings.Repeat("a", int(httpapi.MaxBodyBytes)+1) + `","type":"CHECKING"}`)
	if rr := rawReq(t, srv, "POST", "/api/accounts", big, map[string]string{"Content-Type": "application/json"}); rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("limite global: %d", rr.Code)
	}
}

func TestSpreadsheetExports(t *testing.T) {
	t.Setenv("FINFLOW_PASSWORD", "segredo")
	t.Setenv("FINFLOW_API_TOKEN", "tok-planilha")
	srv := newTestServer(t, testutil.Open(t, db.DialectSQLite), httpapi.ServerOptions{})

	if rr := doReq(t, srv, "GET", "/api/spreadsheet/export.xlsx?year=2026", nil, nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("sem token: %d", rr.Code)
	}
	// Download direto pelo navegador: token na query, como no export CSV.
	f := openXLSXResponse(t, doReq(t, srv, "GET", "/api/spreadsheet/export.xlsx?year=2026&token=tok-planilha", nil, nil))
	if got := strings.Join(f.GetSheetList(), ","); got != "2026,Economia,Lançamentos,Categorias" {
		t.Fatalf("abas = %s", got)
	}
	auth := map[string]string{"Authorization": "Bearer tok-planilha"}
	if rr := doReq(t, srv, "GET", "/api/spreadsheet/export.xlsx?year=1800", nil, auth); rr.Code != http.StatusBadRequest {
		t.Fatalf("ano inválido: %d", rr.Code)
	}
	tpl := openXLSXResponse(t, doReq(t, srv, "GET", "/api/spreadsheet/template.xlsx", nil, auth))
	if got := strings.Join(tpl.GetSheetList(), ","); got != "Lançamentos,Como usar" {
		t.Fatalf("abas do modelo = %s", got)
	}
	tx := openXLSXResponse(t, doReq(t, srv, "GET", "/api/transactions/export?format=xlsx", nil, auth))
	if got := strings.Join(tx.GetSheetList(), ","); got != "Lançamentos" {
		t.Fatalf("abas do extrato = %s", got)
	}
	if rr := doReq(t, srv, "GET", "/api/transactions/export?format=pdf", nil, auth); rr.Code != http.StatusBadRequest {
		t.Fatalf("formato inválido: %d", rr.Code)
	}
}
