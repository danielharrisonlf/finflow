package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"finflow/internal/domain"
	"finflow/internal/service"
)

// registerWealthRoutes registra investimentos, empréstimos, evolução do patrimônio, MCP via HTTP
// e informações do sistema (v3 §10–§14).
func (s *Server) registerWealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/investments", s.requireAuth(s.handleListInvestments))
	mux.HandleFunc("GET /api/investments/summary", s.requireAuth(s.handleInvestmentSummary))
	mux.HandleFunc("POST /api/investments", s.requireAuth(s.handleCreateInvestment))
	mux.HandleFunc("PATCH /api/investments/{id}", s.requireAuth(s.handleUpdateInvestment))
	mux.HandleFunc("DELETE /api/investments/{id}", s.requireAuth(s.handleDeleteInvestment))
	mux.HandleFunc("POST /api/investments/{id}/value", s.requireAuth(s.handleInvestmentValue))
	mux.HandleFunc("POST /api/investments/{id}/contribution", s.requireAuth(s.handleInvestmentContribution))
	mux.HandleFunc("POST /api/investments/{id}/redeem", s.requireAuth(s.handleInvestmentRedeem))

	mux.HandleFunc("GET /api/loans", s.requireAuth(s.handleListLoans))
	mux.HandleFunc("GET /api/loans/strategy", s.requireAuth(s.handleDebtStrategy))
	mux.HandleFunc("POST /api/loans", s.requireAuth(s.handleCreateLoan))
	mux.HandleFunc("PATCH /api/loans/{id}", s.requireAuth(s.handleUpdateLoan))
	mux.HandleFunc("DELETE /api/loans/{id}", s.requireAuth(s.handleDeleteLoan))
	mux.HandleFunc("POST /api/loans/{id}/pay", s.requireAuth(s.handlePayLoan))

	mux.HandleFunc("GET /api/insights/net-worth", s.requireAuth(s.handleNetWorthHistory))

	mux.HandleFunc("POST /api/mcp", s.requireAuth(s.handleMCP))
	mux.HandleFunc("GET /api/mcp", s.handleMCPMethodNotAllowed)
	mux.HandleFunc("DELETE /api/mcp", s.handleMCPMethodNotAllowed)

	mux.HandleFunc("GET /api/system/info", s.requireAuthGlobal(s.handleSystemInfo))
	mux.HandleFunc("POST /api/system/seed", s.requireAuth(s.handleSystemSeed))
}

// ───────────────────────── Investimentos ─────────────────────────

func (s *Server) handleListInvestments(w http.ResponseWriter, r *http.Request) {
	inc, err := parseOptionalBool(r.URL.Query().Get("includeArchived"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).ListInvestments(r.Context(), inc)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleInvestmentSummary(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc(r).InvestmentSummary(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateInvestment(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateInvestmentInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).CreateInvestment(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateInvestment(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateInvestmentInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).UpdateInvestment(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteInvestment(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svc(r).DeleteInvestment(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleInvestmentValue(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateInvestmentValueInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).UpdateInvestmentValue(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleInvestmentContribution(w http.ResponseWriter, r *http.Request) {
	s.handleInvestmentMovement(w, r, s.svc(r).ContributeInvestment)
}

func (s *Server) handleInvestmentRedeem(w http.ResponseWriter, r *http.Request) {
	s.handleInvestmentMovement(w, r, s.svc(r).RedeemInvestment)
}

func (s *Server) handleInvestmentMovement(w http.ResponseWriter, r *http.Request, move func(ctx context.Context, id string, in domain.InvestmentMovementInput) (domain.Investment, error)) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.InvestmentMovementInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := move(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ───────────────────────── Empréstimos ─────────────────────────

func (s *Server) handleListLoans(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc(r).ListLoans(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDebtStrategy(w http.ResponseWriter, r *http.Request) {
	var extra int64
	if raw := r.URL.Query().Get("extraCents"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, r, service.ValidationFailed([]domain.ValidationIssue{{Path: "extraCents", Message: "valor numérico inválido"}}))
			return
		}
		extra = v
	}
	out, err := s.svc(r).DebtStrategy(r.Context(), extra)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateLoan(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateLoanInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).CreateLoan(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateLoan(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateLoanInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).UpdateLoan(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteLoan(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svc(r).DeleteLoan(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePayLoan(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.PayLoanInput
	if err := decodeOptionalJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).PayLoan(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// ───────────────────────── Patrimônio ─────────────────────────

func (s *Server) handleNetWorthHistory(w http.ResponseWriter, r *http.Request) {
	months, err := parseOptionalInt(r.URL.Query().Get("months"), "months", 1, service.MaxNetWorthMonths)
	if err != nil {
		writeError(w, r, err)
		return
	}
	n := 0
	if months != nil {
		n = *months
	}
	out, err := s.svc(r).NetWorthHistory(r.Context(), n)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
