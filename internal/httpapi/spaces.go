package httpapi

import (
	"net/http"

	"finflow/internal/domain"
)

// registerSpacesRoutes registra as rotas da v3 fase 2: espaços e visão unificada (não dependem do
// espaço da requisição), membros e divisão (Casa) e métricas/DRE (Empresa), que dependem.
func (s *Server) registerSpacesRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/spaces", s.requireAuthGlobal(s.handleListSpaces))
	mux.HandleFunc("GET /api/spaces/overview", s.requireAuthGlobal(s.handleSpacesOverview))
	mux.HandleFunc("POST /api/spaces", s.requireAuthGlobal(s.handleCreateSpace))
	mux.HandleFunc("PATCH /api/spaces/{slug}", s.requireAuthGlobal(s.handleUpdateSpace))
	mux.HandleFunc("DELETE /api/spaces/{slug}", s.requireAuthGlobal(s.handleDeleteSpace))

	mux.HandleFunc("GET /api/members", s.requireAuth(s.handleListMembers))
	mux.HandleFunc("POST /api/members", s.requireAuth(s.handleCreateMember))
	mux.HandleFunc("GET /api/members/balances", s.requireAuth(s.handleMemberBalances))
	mux.HandleFunc("POST /api/members/settle", s.requireAuth(s.handleSettle))
	mux.HandleFunc("PATCH /api/members/{id}", s.requireAuth(s.handleUpdateMember))
	mux.HandleFunc("DELETE /api/members/{id}", s.requireAuth(s.handleDeleteMember))

	mux.HandleFunc("GET /api/business/metrics", s.requireAuth(s.handleBusinessMetrics))
	mux.HandleFunc("GET /api/business/dre", s.requireAuth(s.handleBusinessDRE))
}

// ───────────────────────── Espaços ─────────────────────────

func (s *Server) handleListSpaces(w http.ResponseWriter, r *http.Request) {
	out, err := s.spaces.List()
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSpacesOverview(w http.ResponseWriter, r *http.Request) {
	out, err := s.spaces.Overview(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateSpace(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateSpaceInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.spaces.Create(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateSpace(w http.ResponseWriter, r *http.Request) {
	var in domain.UpdateSpaceInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.spaces.Update(r.PathValue("slug"), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteSpace(w http.ResponseWriter, r *http.Request) {
	if err := s.spaces.Delete(r.PathValue("slug"), r.URL.Query().Get("confirm")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Membros (Casa) ─────────────────────────

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc(r).ListMembers(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateMember(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateMemberInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).CreateMember(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateMemberInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).UpdateMember(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteMember(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svc(r).DeleteMember(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMemberBalances(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).MemberBalances(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSettle(w http.ResponseWriter, r *http.Request) {
	var in domain.SettleInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).Settle(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// ───────────────────────── Empresa ─────────────────────────

func (s *Server) handleBusinessMetrics(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).BusinessMetrics(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleBusinessDRE(w http.ResponseWriter, r *http.Request) {
	year, err := parseOptionalInt(r.URL.Query().Get("year"), "year", 2000, 2100)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svc(r).BusinessDRE(r.Context(), year)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
