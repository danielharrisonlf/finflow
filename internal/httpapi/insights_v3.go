package httpapi

import (
	"net/http"

	"finflow/internal/domain"
)

// registerInsightsV3Routes registra as rotas do v3 §1–§9 (tags, assinaturas, alertas, fluxo de
// caixa, composição da fatura, transferências inteligentes, "posso comprar?" e prévia de link).
func (s *Server) registerInsightsV3Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tags", s.requireAuth(s.handleListTags))
	mux.HandleFunc("POST /api/tags", s.requireAuth(s.handleCreateTag))
	mux.HandleFunc("PATCH /api/tags/{id}", s.requireAuth(s.handleUpdateTag))
	mux.HandleFunc("DELETE /api/tags/{id}", s.requireAuth(s.handleDeleteTag))
	mux.HandleFunc("GET /api/insights/tags", s.requireAuth(s.handleInsightsTags))

	mux.HandleFunc("GET /api/insights/subscriptions", s.requireAuth(s.handleSubscriptions))
	mux.HandleFunc("POST /api/insights/subscriptions/{key}/track", s.requireAuth(s.handleTrackSubscription))
	mux.HandleFunc("POST /api/insights/subscriptions/{key}/dismiss", s.requireAuth(s.handleDismissSubscription))

	mux.HandleFunc("GET /api/alerts", s.requireAuth(s.handleAlerts))
	mux.HandleFunc("POST /api/alerts/{id}/dismiss", s.requireAuth(s.handleDismissAlert))

	mux.HandleFunc("GET /api/insights/cashflow", s.requireAuth(s.handleInsightsCashflow))
	mux.HandleFunc("POST /api/insights/can-afford", s.requireAuth(s.handleCanAfford))
	mux.HandleFunc("GET /api/cards/{id}/invoices/{invoiceId}/breakdown", s.requireAuth(s.handleInvoiceBreakdown))

	mux.HandleFunc("GET /api/transactions/transfer-candidates", s.requireAuth(s.handleTransferCandidates))
	mux.HandleFunc("POST /api/transactions/merge-transfer", s.requireAuth(s.handleMergeTransfer))

	mux.HandleFunc("POST /api/wishlist/preview-link", s.requireAuth(s.handlePreviewLink))
}

// ───────────────────────── Tags ─────────────────────────

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc(r).ListTags(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateTagInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).CreateTag(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateTag(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateTagInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).UpdateTag(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svc(r).DeleteTag(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleInsightsTags(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).InsightsTags(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ───────────────────────── Assinaturas ─────────────────────────

func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc(r).Subscriptions(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleTrackSubscription(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc(r).TrackSubscription(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleDismissSubscription(w http.ResponseWriter, r *http.Request) {
	if err := s.svc(r).DismissSubscription(r.Context(), r.PathValue("key")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Alertas ─────────────────────────

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc(r).Alerts(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDismissAlert(w http.ResponseWriter, r *http.Request) {
	if err := s.svc(r).DismissAlert(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Fluxo de caixa, fatura e "posso comprar?" ─────────────────────────

func (s *Server) handleInsightsCashflow(w http.ResponseWriter, r *http.Request) {
	period, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	months, err := parseOptionalInt(r.URL.Query().Get("months"), "months", 1, domain.MaxCashflowMonths)
	if err != nil {
		writeError(w, r, err)
		return
	}
	q := domain.CashflowQuery{Month: period.Month, Year: period.Year, Basis: r.URL.Query().Get("basis")}
	if months != nil {
		q.Months = *months
	}
	out, err := s.svc(r).InsightsCashflow(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCanAfford(w http.ResponseWriter, r *http.Request) {
	var in domain.CanAffordInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).CanAfford(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleInvoiceBreakdown(w http.ResponseWriter, r *http.Request) {
	cardID, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	invoiceID, err := requireUUIDParam(r, "invoiceId")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).InvoiceBreakdown(r.Context(), cardID, invoiceID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ───────────────────────── Transferências inteligentes ─────────────────────────

func (s *Server) handleTransferCandidates(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc(r).TransferCandidates(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMergeTransfer(w http.ResponseWriter, r *http.Request) {
	var in domain.MergeTransferInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).MergeTransfer(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ───────────────────────── Prévia de link ─────────────────────────

func (s *Server) handlePreviewLink(w http.ResponseWriter, r *http.Request) {
	var in domain.LinkPreviewInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).PreviewLink(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
