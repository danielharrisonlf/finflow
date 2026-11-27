package service

import (
	"context"
	"fmt"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Recurring ─────────────────────────

func recurringSchedule(r store.RecurringRow) domain.RecurringSchedule {
	return domain.RecurringSchedule{DayOfMonth: r.DayOfMonth, StartDate: r.StartDate, EndDate: r.EndDate}
}

func toRecurring(r store.RecurringRow, realized map[string]bool, today string) domain.Recurring {
	return domain.Recurring{
		ID:            r.ID,
		Description:   r.Description,
		Kind:          r.Kind,
		AmountCents:   r.AmountCents,
		DayOfMonth:    r.DayOfMonth,
		AccountID:     r.AccountID,
		AccountName:   r.AccountName,
		CategoryID:    r.CategoryID,
		CategoryName:  r.CategoryName,
		CategoryIcon:  r.CategoryIcon,
		CategoryColor: r.CategoryColor,
		StartDate:     r.StartDate,
		EndDate:       r.EndDate,
		Active:        r.Active,
		NextDate:      domain.NextRecurringDate(recurringSchedule(r), r.Active, realized, today),

		IsSubscription: r.IsSubscription,
		DeactivatedAt:  r.DeactivatedAt,
	}
}

func (s *Services) ListRecurring(ctx context.Context) ([]domain.Recurring, error) {
	rows, err := s.Store.ListRecurring(ctx)
	if err != nil {
		return nil, err
	}
	realized, err := s.Store.RealizedRecurringMonths(ctx)
	if err != nil {
		return nil, err
	}
	today := domain.TodayString()
	out := make([]domain.Recurring, 0, len(rows))
	for _, r := range rows {
		out = append(out, toRecurring(r, realized[r.ID], today))
	}
	return out, nil
}

func (s *Services) GetRecurring(ctx context.Context, id string) (domain.Recurring, error) {
	r, err := s.Store.FindRecurringByID(ctx, id)
	if err != nil {
		return domain.Recurring{}, err
	}
	if r == nil {
		return domain.Recurring{}, NotFound("recorrência não encontrada")
	}
	realized, err := s.Store.RealizedRecurringMonths(ctx)
	if err != nil {
		return domain.Recurring{}, err
	}
	return toRecurring(*r, realized[r.ID], domain.TodayString()), nil
}

// validateRecurringRefs confere conta (existente e ativa) e categoria (existente e do mesmo tipo).
func (s *Services) validateRecurringRefs(ctx context.Context, kind domain.CategoryKind, accountID, categoryID *string) error {
	if accountID != nil {
		if err := s.requireActiveAccount(ctx, *accountID); err != nil {
			return err
		}
	}
	if categoryID != nil {
		cat, err := s.Store.FindCategoryByID(ctx, *categoryID)
		if err != nil {
			return err
		}
		if cat == nil {
			return NotFound("categoria não encontrada")
		}
		if cat.Kind != kind {
			return BadRequest("a categoria deve ser do mesmo tipo (receita/despesa) da recorrência")
		}
	}
	return nil
}

func (s *Services) CreateRecurring(ctx context.Context, in domain.CreateRecurringInput) (domain.Recurring, error) {
	if issues := domain.ValidateCreateRecurring(in); len(issues) > 0 {
		return domain.Recurring{}, ValidationFailed(issues)
	}
	start := domain.TodayString()
	if in.StartDate != nil {
		start = *in.StartDate
	}
	if issues := domain.ValidateRecurringPeriod(start, in.EndDate); len(issues) > 0 {
		return domain.Recurring{}, ValidationFailed(issues)
	}
	if err := s.validateRecurringRefs(ctx, in.Kind, in.AccountID, in.CategoryID); err != nil {
		return domain.Recurring{}, err
	}
	id, err := s.Store.CreateRecurring(ctx, store.RecurringRow{
		Description: in.Description,
		Kind:        in.Kind,
		AmountCents: in.AmountCents,
		DayOfMonth:  in.DayOfMonth,
		AccountID:   in.AccountID,
		CategoryID:  in.CategoryID,
		StartDate:   start,
		EndDate:     in.EndDate,
		Active:      true,

		IsSubscription: in.IsSubscription != nil && *in.IsSubscription,
	})
	if err != nil {
		return domain.Recurring{}, err
	}
	return s.GetRecurring(ctx, id)
}

func (s *Services) UpdateRecurring(ctx context.Context, id string, in domain.UpdateRecurringInput) (domain.Recurring, error) {
	if issues := domain.ValidateUpdateRecurring(in); len(issues) > 0 {
		return domain.Recurring{}, ValidationFailed(issues)
	}
	r, err := s.Store.FindRecurringByID(ctx, id)
	if err != nil {
		return domain.Recurring{}, err
	}
	if r == nil {
		return domain.Recurring{}, NotFound("recorrência não encontrada")
	}
	if in.Description.Set && in.Description.Valid {
		r.Description = in.Description.Value
	}
	if in.Kind.Set && in.Kind.Valid {
		r.Kind = in.Kind.Value
	}
	if in.AmountCents.Set && in.AmountCents.Valid {
		r.AmountCents = in.AmountCents.Value
	}
	if in.DayOfMonth.Set && in.DayOfMonth.Valid {
		r.DayOfMonth = in.DayOfMonth.Value
	}
	if in.AccountID.Set {
		r.AccountID = optionalString(in.AccountID)
	}
	if in.CategoryID.Set {
		r.CategoryID = optionalString(in.CategoryID)
	}
	if in.StartDate.Set && in.StartDate.Valid {
		r.StartDate = in.StartDate.Value
	}
	if in.EndDate.Set {
		r.EndDate = optionalString(in.EndDate)
	}
	if in.Active.Set && in.Active.Valid {
		// deactivated_at registra quando a recorrência deixou de valer (churn nas métricas da empresa).
		if r.Active && !in.Active.Value {
			today := domain.TodayString()
			r.DeactivatedAt = &today
		} else if in.Active.Value {
			r.DeactivatedAt = nil
		}
		r.Active = in.Active.Value
	}
	if in.IsSubscription.Set {
		r.IsSubscription = in.IsSubscription.Valid && in.IsSubscription.Value
	}
	if issues := domain.ValidateRecurringPeriod(r.StartDate, r.EndDate); len(issues) > 0 {
		return domain.Recurring{}, ValidationFailed(issues)
	}
	// Só revalida as referências alteradas (ou todas, se o tipo mudou).
	var accountID, categoryID *string
	if in.AccountID.Set {
		accountID = r.AccountID
	}
	if in.CategoryID.Set || in.Kind.Set {
		categoryID = r.CategoryID
	}
	if err := s.validateRecurringRefs(ctx, r.Kind, accountID, categoryID); err != nil {
		return domain.Recurring{}, err
	}
	if err := s.Store.UpdateRecurring(ctx, r); err != nil {
		return domain.Recurring{}, err
	}
	return s.GetRecurring(ctx, id)
}

func (s *Services) DeleteRecurring(ctx context.Context, id string) error {
	r, err := s.Store.FindRecurringByID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return NotFound("recorrência não encontrada")
	}
	return s.Store.DeleteRecurring(ctx, id)
}

// PostRecurring lança a ocorrência do mês de `date` (padrão: a ocorrência do mês corrente)
// como transação confirmada ligada à recorrência. Cada mês só pode ser lançado uma vez.
func (s *Services) PostRecurring(ctx context.Context, id string, in domain.PostRecurringInput) (domain.Transaction, error) {
	if issues := domain.ValidatePostRecurring(in); len(issues) > 0 {
		return domain.Transaction{}, ValidationFailed(issues)
	}
	r, err := s.Store.FindRecurringByID(ctx, id)
	if err != nil {
		return domain.Transaction{}, err
	}
	if r == nil {
		return domain.Transaction{}, NotFound("recorrência não encontrada")
	}
	date := ""
	if in.Date != nil {
		date = *in.Date
	} else {
		month, year := domain.CurrentMonthYear()
		date = domain.RecurringOccurrenceDate(r.DayOfMonth, year, month)
	}
	accountID := r.AccountID
	if in.AccountID != nil {
		accountID = in.AccountID
	}
	if accountID == nil {
		return domain.Transaction{}, ValidationFailed([]domain.ValidationIssue{{Path: "accountId", Message: "informe a conta para lançar a recorrência"}})
	}
	if err := s.requireActiveAccount(ctx, *accountID); err != nil {
		return domain.Transaction{}, err
	}
	realized, err := s.Store.RealizedRecurringMonths(ctx)
	if err != nil {
		return domain.Transaction{}, err
	}
	if realized[r.ID][domain.MonthKey(date)] {
		return domain.Transaction{}, Conflict(fmt.Sprintf("a ocorrência de %s/%s já foi lançada", date[5:7], date[:4]))
	}
	amount := r.AmountCents
	if in.AmountCents != nil {
		amount = *in.AmountCents
	}
	txType := domain.TxExpense
	if r.Kind == domain.CategoryIncome {
		txType = domain.TxIncome
	}
	created, err := s.Store.CreateRecurringTransaction(ctx, r.ID, txType, r.Description, domain.SignedAmount(txType, amount), date, *accountID, r.CategoryID)
	if err != nil {
		return domain.Transaction{}, err
	}
	if txType == domain.TxExpense && r.CategoryID != nil && domain.IsInCurrentMonth(date) {
		catID := *r.CategoryID
		go s.Notifications.NotifyBudgetThresholdCrossed(context.Background(), catID, amount)
	}
	return *created, nil
}

// optionalString converte um campo PATCH (já marcado como enviado) em ponteiro (null → nil).
func optionalString(o domain.Optional[string]) *string {
	if !o.Valid {
		return nil
	}
	v := o.Value
	return &v
}
