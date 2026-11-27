package httpapi

import (
	"bufio"
	"errors"
	"io"
	"net/http"

	"finflow/internal/domain"
)

// decodeOptionalJSON aceita corpo vazio (todos os campos opcionais); senão age como decodeJSON.
func decodeOptionalJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	br := bufio.NewReader(r.Body)
	if _, err := br.Peek(1); errors.Is(err, io.EOF) {
		return nil
	}
	r.Body = io.NopCloser(br)
	return decodeJSON(r, dst)
}

// ───────────────────────── Recurring ─────────────────────────

func (s *Server) handleListRecurring(w http.ResponseWriter, r *http.Request) {
	out, err := s.svcs.ListRecurring(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateRecurring(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateRecurringInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.CreateRecurring(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateRecurring(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateRecurringInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateRecurring(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteRecurring(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.DeleteRecurring(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePostRecurring(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.PostRecurringInput
	if err := decodeOptionalJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.PostRecurring(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// ───────────────────────── Settings ─────────────────────────

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	out, err := s.svcs.GetSettings(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var in domain.UpdateSettingsInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateSettings(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ───────────────────────── Category rules ─────────────────────────

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	out, err := s.svcs.ListCategoryRules(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateCategoryRuleInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.CreateCategoryRule(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateCategoryRuleInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateCategoryRule(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.DeleteCategoryRule(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleApplyRules(w http.ResponseWriter, r *http.Request) {
	var in domain.ApplyRulesInput
	if err := decodeOptionalJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ApplyCategoryRules(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ───────────────────────── Wishlist ─────────────────────────

func (s *Server) handleListWishlist(w http.ResponseWriter, r *http.Request) {
	out, err := s.svcs.ListWishlist(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateWishItem(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateWishItemInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.CreateWishItem(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateWishItem(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateWishItemInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateWishItem(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteWishItem(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.DeleteWishItem(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Insights ─────────────────────────

func (s *Server) handleInsightsOverview(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.InsightsOverview(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleInsightsProjection(w http.ResponseWriter, r *http.Request) {
	days, err := parseOptionalInt(r.URL.Query().Get("days"), "days", 1, domain.ProjectionHorizonDays)
	if err != nil {
		writeError(w, r, err)
		return
	}
	n := 0
	if days != nil {
		n = *days
	}
	out, err := s.svcs.InsightsProjection(r.Context(), n)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleInsightsYear(w http.ResponseWriter, r *http.Request) {
	year, err := parseOptionalInt(r.URL.Query().Get("year"), "year", domain.MinQueryYear, domain.MaxQueryYear)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.InsightsYear(r.Context(), year)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleInsightsCommitments(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.InsightsCommitments(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
