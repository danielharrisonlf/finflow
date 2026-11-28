package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"finflow/internal/ai"
	"finflow/internal/service"
)

// MaxReceiptBodyBytes é o limite de corpo de POST /api/ai/receipt: o arquivo (até 10 MiB) mais a
// sobra do envelope multipart.
const MaxReceiptBodyBytes int64 = ai.MaxReceiptBytes + 64<<10

// newAIService cria o serviço do Assistente; os testes trocam por um apontado para um servidor falso.
var newAIService = func(svcs *service.Services) *ai.Service {
	return ai.NewService(svcs, ai.Config{})
}

// chatWriteTimeout estende o WriteTimeout do servidor (60 s) só para o streaming do chat.
const chatWriteTimeout = 15 * time.Minute

func (s *Server) registerAIRoutes(mux *http.ServeMux) {
	// A configuração do provedor (e as chaves) fica no espaço pessoal; o chat e os comprovantes
	// consultam o espaço da requisição.
	svc := newAIService(s.svcs)
	s.ai = svc
	mux.HandleFunc("GET /api/ai/status", s.requireAuthGlobal(func(w http.ResponseWriter, r *http.Request) {
		st, err := svc.Status(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	}))
	mux.HandleFunc("PUT /api/ai/config", s.requireAuthGlobal(func(w http.ResponseWriter, r *http.Request) {
		var body ai.ConfigInput
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, r, err)
			return
		}
		st, err := svc.SaveConfig(r.Context(), body)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	}))
	mux.HandleFunc("GET /api/ai/models", s.requireAuthGlobal(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		res, err := svc.Models(r.Context(), q.Get("provider"), q.Get("baseUrl"))
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}))
	// Legado: equivale a PUT /api/ai/config {provider: "claude", apiKey}.
	mux.HandleFunc("PUT /api/ai/key", s.requireAuthGlobal(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			APIKey *string `json:"apiKey"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, r, err)
			return
		}
		st, err := svc.SaveKey(r.Context(), body.APIKey)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	}))
	mux.HandleFunc("POST /api/ai/chat", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		handleAIChat(w, r, svc.ForSpace(s.svc(r)))
	}))
	mux.HandleFunc("POST /api/ai/receipt", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		handleAIReceipt(w, r, svc.ForSpace(s.svc(r)))
	}))
}

// sseWriter escreve eventos Server-Sent Events; os cabeçalhos só saem no primeiro evento, para que erros
// anteriores (ex.: sem configuração, servidor local fora do ar) ainda possam ser respondidos em JSON com o status correto.
type sseWriter struct {
	w       http.ResponseWriter
	rc      *http.ResponseController
	mu      sync.Mutex
	started bool
	broken  bool
}

func (e *sseWriter) emit(event string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.broken {
		return
	}
	if !e.started {
		h := e.w.Header()
		h.Set("Content-Type", "text/event-stream; charset=utf-8")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		e.w.WriteHeader(http.StatusOK)
		e.started = true
	}
	payload, err := json.Marshal(data)
	if err != nil {
		payload = []byte(`{}`)
	}
	if _, err := fmt.Fprintf(e.w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		e.broken = true // cliente desconectou
		return
	}
	_ = e.rc.Flush()
}

func (e *sseWriter) hasStarted() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.started
}

func handleAIChat(w http.ResponseWriter, r *http.Request, svc *ai.Service) {
	var in ai.ChatInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(chatWriteTimeout)) // sem suporte (ex.: testes): ignora
	sse := &sseWriter{w: w, rc: rc}
	err := svc.Chat(r.Context(), in, sse.emit)
	if err == nil {
		return
	}
	if !sse.hasStarted() {
		writeError(w, r, err)
		return
	}
	msg := "Não foi possível concluir a resposta."
	var appErr *service.AppError
	if errors.As(err, &appErr) {
		msg = appErr.Message
	} else if r.Context().Err() == nil {
		slog.Error("erro no chat do assistente", "err", err)
	}
	sse.emit("error", map[string]string{"message": msg})
	sse.emit("done", map[string]string{"stopReason": "error"})
}

func handleAIReceipt(w http.ResponseWriter, r *http.Request, svc *ai.Service) {
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, r, &service.AppError{Status: http.StatusRequestEntityTooLarge, Message: "arquivo maior que 10 MiB"})
			return
		}
		writeError(w, r, service.BadRequest("envie o comprovante no campo multipart \"file\""))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, ai.MaxReceiptBytes+1))
	if err != nil {
		writeError(w, r, service.BadRequest("falha ao ler o arquivo"))
		return
	}
	mediaType, err := ai.DetectReceiptMedia(data, header.Filename)
	if err != nil {
		writeError(w, r, err)
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(3 * time.Minute))
	res, err := svc.ExtractReceipt(r.Context(), data, mediaType)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
