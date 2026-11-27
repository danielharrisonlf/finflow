package httpapi

import (
	"fmt"
	"net/http"
	"unicode/utf8"

	"finflow/internal/domain"
	"finflow/internal/service"
)

// ───────────────────────── Auth ─────────────────────────

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.svcs.Auth.Status(extractRequestToken(r)))
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var in domain.LoginInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	res, err := s.svcs.Auth.Login(in.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ───────────────────────── Accounts ─────────────────────────

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	inc, err := parseOptionalBool(r.URL.Query().Get("includeArchived"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ListAccounts(r.Context(), inc)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.GetAccount(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateAccountInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.CreateAccount(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateAccountInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateAccount(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.DeleteAccount(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Categories ─────────────────────────

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	var kindPtr *domain.CategoryKind
	if raw := r.URL.Query().Get("kind"); raw != "" {
		k := domain.CategoryKind(raw)
		if !k.Valid() {
			writeError(w, r, service.ValidationFailed([]domain.ValidationIssue{{Path: "kind", Message: "tipo inválido"}}))
			return
		}
		kindPtr = &k
	}
	out, err := s.svcs.ListCategories(r.Context(), kindPtr)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateCategoryInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.CreateCategory(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateCategoryInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateCategory(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.DeleteCategory(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Cards & Invoices ─────────────────────────

func (s *Server) handleListCards(w http.ResponseWriter, r *http.Request) {
	inc, err := parseOptionalBool(r.URL.Query().Get("includeArchived"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ListCards(r.Context(), inc)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetCard(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.GetCard(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateCard(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateCardInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.CreateCard(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateCard(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateCardInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateCard(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteCard(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.DeleteCard(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddCardExpense(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.CreateCardExpenseInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.AddCardExpense(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ListInvoices(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListInvoiceItems(w http.ResponseWriter, r *http.Request) {
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
	out, err := s.svcs.ListInvoiceItems(r.Context(), cardID, invoiceID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePayInvoice(w http.ResponseWriter, r *http.Request) {
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
	var in domain.PayInvoiceInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.PayInvoice(r.Context(), cardID, invoiceID, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// ───────────────────────── Transactions ─────────────────────────

func parseTransactionsQuery(r *http.Request) (domain.ListTransactionsQuery, error) {
	q := r.URL.Query()
	var out domain.ListTransactionsQuery

	page, err := parseOptionalInt(q.Get("page"), "page", 1, 100000)
	if err != nil {
		return out, err
	}
	if page != nil {
		out.Page = *page
	} else {
		out.Page = 1
	}

	pageSize, err := parseOptionalInt(q.Get("pageSize"), "pageSize", 1, 100)
	if err != nil {
		return out, err
	}
	if pageSize != nil {
		out.PageSize = *pageSize
	} else {
		out.PageSize = 50
	}

	month, err := parseOptionalInt(q.Get("month"), "month", 1, 12)
	if err != nil {
		return out, err
	}
	out.Month = month

	year, err := parseOptionalInt(q.Get("year"), "year", 2000, 2100)
	if err != nil {
		return out, err
	}
	out.Year = year

	if v := q.Get("accountId"); v != "" {
		if !domain.IsValidUUID(v) {
			return out, service.ValidationFailed([]domain.ValidationIssue{{Path: "accountId", Message: "UUID inválido"}})
		}
		out.AccountID = &v
	}
	if v := q.Get("cardId"); v != "" {
		if !domain.IsValidUUID(v) {
			return out, service.ValidationFailed([]domain.ValidationIssue{{Path: "cardId", Message: "UUID inválido"}})
		}
		out.CardID = &v
	}
	if v := q.Get("categoryId"); v != "" {
		if !domain.IsValidUUID(v) {
			return out, service.ValidationFailed([]domain.ValidationIssue{{Path: "categoryId", Message: "UUID inválido"}})
		}
		out.CategoryID = &v
	}
	if v := q.Get("type"); v != "" {
		t := domain.TransactionType(v)
		if !t.Valid() {
			return out, service.ValidationFailed([]domain.ValidationIssue{{Path: "type", Message: "tipo inválido"}})
		}
		out.Type = &t
	}
	if v := q.Get("status"); v != "" {
		st := domain.TransactionStatus(v)
		if !st.Valid() {
			return out, service.ValidationFailed([]domain.ValidationIssue{{Path: "status", Message: "status inválido"}})
		}
		out.Status = &st
	}
	if v := q.Get("search"); v != "" {
		if utf8.RuneCountInString(v) > 120 {
			return out, service.ValidationFailed([]domain.ValidationIssue{{Path: "search", Message: "busca muito longa"}})
		}
		out.Search = &v
	}
	if v := q.Get("tagId"); v != "" {
		if !domain.IsValidUUID(v) {
			return out, service.ValidationFailed([]domain.ValidationIssue{{Path: "tagId", Message: "UUID inválido"}})
		}
		out.TagID = &v
	}
	return out, nil
}

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	q, err := parseTransactionsQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ListTransactions(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleExportTransactions(w http.ResponseWriter, r *http.Request) {
	q, err := parseTransactionsQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	format := r.URL.Query().Get("format")
	content, contentType, filename, err := s.svcs.ExportTransactions(r.Context(), format, q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.GetTransaction(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateTransactionInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.CreateTransaction(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var in domain.TransferInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.Transfer(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateTransactionInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateTransaction(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleConfirmTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ConfirmTransaction(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	scope := r.URL.Query().Get("scope")
	if err := s.svcs.DeleteTransaction(r.Context(), id, scope); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Budgets ─────────────────────────

func (s *Server) handleListBudgets(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ListBudgets(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUpsertBudget(w http.ResponseWriter, r *http.Request) {
	var in domain.UpsertBudgetInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpsertBudget(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleDeleteBudget(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.DeleteBudget(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Goals ─────────────────────────

func (s *Server) handleListGoals(w http.ResponseWriter, r *http.Request) {
	out, err := s.svcs.ListGoals(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetGoal(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.GetGoal(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateGoal(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateGoalInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.CreateGoal(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.UpdateGoalInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.UpdateGoal(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleContributeGoal(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	var in domain.ContributeGoalInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.ContributeGoal(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	id, err := requireUUIDParam(r, "id")
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.DeleteGoal(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Dashboard ─────────────────────────

func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.DashboardSummary(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDashboardCashflow(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.DashboardCashflow(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDashboardCategories(w http.ResponseWriter, r *http.Request) {
	q, err := parsePeriodQuery(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out, err := s.svcs.DashboardCategories(r.Context(), q)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDashboardUpcoming(w http.ResponseWriter, r *http.Request) {
	out, err := s.svcs.DashboardUpcoming(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ───────────────────────── Telegram ─────────────────────────

func (s *Server) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	if expected := telegramWebhookSecret(); expected != "" {
		if !secretMatches(r.Header.Get("X-Telegram-Bot-Api-Secret-Token"), expected) {
			writeError(w, r, service.Forbidden("secret token inválido"))
			return
		}
	}
	var update domain.TelegramUpdate
	if err := decodeJSON(r, &update); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svcs.Telegram.HandleUpdate(r.Context(), update); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTelegramStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.svcs.Telegram.Status())
}
