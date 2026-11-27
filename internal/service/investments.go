package service

import (
	"context"
	"fmt"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Investimentos ─────────────────────────

func toInvestment(r store.InvestmentRow) domain.Investment {
	return domain.Investment{
		ID:             r.ID,
		Name:           r.Name,
		Kind:           r.Kind,
		Institution:    r.Institution,
		AccountID:      r.AccountID,
		AccountName:    r.AccountName,
		InvestedCents:  r.InvestedCents,
		CurrentCents:   r.CurrentCents,
		ReturnCents:    r.CurrentCents - r.InvestedCents,
		ReturnPct:      domain.ReturnPct(r.InvestedCents, r.CurrentCents),
		ValueUpdatedAt: r.ValueUpdatedAt,
		Notes:          r.Notes,
		Archived:       r.Archived,
	}
}

func (s *Services) ListInvestments(ctx context.Context, includeArchived bool) ([]domain.Investment, error) {
	rows, err := s.Store.ListInvestments(ctx, includeArchived)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Investment, 0, len(rows))
	for _, r := range rows {
		out = append(out, toInvestment(r))
	}
	return out, nil
}

func (s *Services) findInvestment(ctx context.Context, id string) (*store.InvestmentRow, error) {
	r, err := s.Store.FindInvestmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, NotFound("investimento não encontrado")
	}
	return r, nil
}

func (s *Services) GetInvestment(ctx context.Context, id string) (domain.Investment, error) {
	r, err := s.findInvestment(ctx, id)
	if err != nil {
		return domain.Investment{}, err
	}
	return toInvestment(*r), nil
}

// requireInvestmentAccount confere que a conta vinculada existe, está ativa e é do tipo INVESTMENT.
func (s *Services) requireInvestmentAccount(ctx context.Context, accountID string) (*store.AccountRow, error) {
	acc, err := s.Store.FindAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, NotFound("conta não encontrada")
	}
	if acc.Archived {
		return nil, BadRequest("conta está arquivada")
	}
	if acc.Type != domain.AccountInvestment {
		return nil, BadRequest("vincule a posição a uma conta do tipo investimento")
	}
	return acc, nil
}

func dateOrToday(d *string) string {
	if d != nil && *d != "" {
		return *d
	}
	return domain.TodayString()
}

func (s *Services) CreateInvestment(ctx context.Context, in domain.CreateInvestmentInput) (domain.Investment, error) {
	if issues := domain.ValidateCreateInvestment(in); len(issues) > 0 {
		return domain.Investment{}, ValidationFailed(issues)
	}
	if in.AccountID != nil {
		if _, err := s.requireInvestmentAccount(ctx, *in.AccountID); err != nil {
			return domain.Investment{}, err
		}
	}
	current := in.InvestedCents
	if in.CurrentCents != nil {
		current = *in.CurrentCents
	}
	date := dateOrToday(in.Date)
	id, err := s.Store.CreateInvestment(ctx, store.InvestmentRow{
		Name:           in.Name,
		Kind:           in.Kind,
		Institution:    in.Institution,
		AccountID:      in.AccountID,
		InvestedCents:  in.InvestedCents,
		CurrentCents:   current,
		Notes:          in.Notes,
		ValueUpdatedAt: &date,
	}, &domain.InvestmentSnapshot{Date: date, ValueCents: current, InvestedCents: in.InvestedCents})
	if err != nil {
		return domain.Investment{}, err
	}
	return s.GetInvestment(ctx, id)
}

func (s *Services) UpdateInvestment(ctx context.Context, id string, in domain.UpdateInvestmentInput) (domain.Investment, error) {
	if issues := domain.ValidateUpdateInvestment(in); len(issues) > 0 {
		return domain.Investment{}, ValidationFailed(issues)
	}
	r, err := s.findInvestment(ctx, id)
	if err != nil {
		return domain.Investment{}, err
	}
	if in.Name.Set {
		r.Name = in.Name.Value
	}
	if in.Kind.Set {
		r.Kind = in.Kind.Value
	}
	if in.Institution.Set {
		r.Institution = optionalString(in.Institution)
	}
	if in.AccountID.Set {
		r.AccountID = optionalString(in.AccountID)
		if r.AccountID != nil {
			if _, err := s.requireInvestmentAccount(ctx, *r.AccountID); err != nil {
				return domain.Investment{}, err
			}
		}
	}
	if in.Notes.Set {
		r.Notes = optionalString(in.Notes)
	}
	if in.Archived.Set {
		r.Archived = in.Archived.Value
	}
	// Mudança de valores grava o snapshot de hoje para o histórico refletir a correção.
	var snap *domain.InvestmentSnapshot
	if in.InvestedCents.Set || in.CurrentCents.Set {
		if in.InvestedCents.Set {
			r.InvestedCents = in.InvestedCents.Value
		}
		if in.CurrentCents.Set && in.CurrentCents.Value != r.CurrentCents {
			r.CurrentCents = in.CurrentCents.Value
			today := domain.TodayString()
			r.ValueUpdatedAt = &today
		}
		snap = &domain.InvestmentSnapshot{Date: domain.TodayString(), ValueCents: r.CurrentCents, InvestedCents: r.InvestedCents}
	}
	if err := s.Store.SaveInvestment(ctx, r, snap, nil); err != nil {
		return domain.Investment{}, err
	}
	return s.GetInvestment(ctx, id)
}

func (s *Services) DeleteInvestment(ctx context.Context, id string) error {
	if _, err := s.findInvestment(ctx, id); err != nil {
		return err
	}
	return s.Store.DeleteInvestment(ctx, id)
}

// UpdateInvestmentValue registra o valor de mercado na data (padrão hoje) e grava o snapshot.
// Um valor retroativo (anterior à última atualização) só entra no histórico.
func (s *Services) UpdateInvestmentValue(ctx context.Context, id string, in domain.UpdateInvestmentValueInput) (domain.Investment, error) {
	if issues := domain.ValidateUpdateInvestmentValue(in); len(issues) > 0 {
		return domain.Investment{}, ValidationFailed(issues)
	}
	r, err := s.findInvestment(ctx, id)
	if err != nil {
		return domain.Investment{}, err
	}
	date := dateOrToday(in.Date)
	if r.ValueUpdatedAt == nil || date >= *r.ValueUpdatedAt {
		r.CurrentCents = in.CurrentCents
		r.ValueUpdatedAt = &date
	}
	snap := &domain.InvestmentSnapshot{Date: date, ValueCents: in.CurrentCents, InvestedCents: r.InvestedCents}
	if err := s.Store.SaveInvestment(ctx, r, snap, nil); err != nil {
		return domain.Investment{}, err
	}
	return s.GetInvestment(ctx, id)
}

// ContributeInvestment soma o aporte no investido e no valor atual (grava snapshot). Com
// fromAccountId, cria a transferência da conta de origem para a conta INVESTMENT vinculada.
func (s *Services) ContributeInvestment(ctx context.Context, id string, in domain.InvestmentMovementInput) (domain.Investment, error) {
	in.ToAccountID = nil
	return s.moveInvestment(ctx, id, in, true)
}

// RedeemInvestment é o inverso do aporte: subtrai do valor atual e do investido (sem ficar
// negativo); com toAccountId, transfere da conta INVESTMENT vinculada para a conta de destino.
func (s *Services) RedeemInvestment(ctx context.Context, id string, in domain.InvestmentMovementInput) (domain.Investment, error) {
	in.FromAccountID = nil
	return s.moveInvestment(ctx, id, in, false)
}

func (s *Services) moveInvestment(ctx context.Context, id string, in domain.InvestmentMovementInput, contribution bool) (domain.Investment, error) {
	if issues := domain.ValidateInvestmentMovement(in); len(issues) > 0 {
		return domain.Investment{}, ValidationFailed(issues)
	}
	r, err := s.findInvestment(ctx, id)
	if err != nil {
		return domain.Investment{}, err
	}
	if r.Archived {
		return domain.Investment{}, BadRequest("investimento arquivado")
	}
	date := dateOrToday(in.Date)
	if contribution {
		r.InvestedCents += in.AmountCents
		r.CurrentCents += in.AmountCents
	} else {
		if in.AmountCents > r.CurrentCents {
			return domain.Investment{}, BadRequest("resgate maior que o valor atual do investimento")
		}
		r.CurrentCents -= in.AmountCents
		r.InvestedCents = max(0, r.InvestedCents-in.AmountCents)
	}

	other := in.FromAccountID
	if !contribution {
		other = in.ToAccountID
	}
	var transfer *store.TransferSpec
	if other != nil {
		path := "fromAccountId"
		if !contribution {
			path = "toAccountId"
		}
		if r.AccountID == nil {
			return domain.Investment{}, ValidationFailed([]domain.ValidationIssue{{Path: path, Message: "vincule o investimento a uma conta de investimento para movimentar dinheiro entre contas"}})
		}
		linked, err := s.requireInvestmentAccount(ctx, *r.AccountID)
		if err != nil {
			return domain.Investment{}, err
		}
		if *other == linked.ID {
			return domain.Investment{}, ValidationFailed([]domain.ValidationIssue{{Path: path, Message: "escolha uma conta diferente da conta do investimento"}})
		}
		if err := s.requireActiveAccount(ctx, *other); err != nil {
			return domain.Investment{}, err
		}
		otherAcc, err := s.Store.FindAccountByID(ctx, *other)
		if err != nil {
			return domain.Investment{}, err
		}
		if contribution {
			transfer = &store.TransferSpec{
				FromAccountID: otherAcc.ID, ToAccountID: linked.ID, AmountCents: in.AmountCents, Date: date,
				FromDesc: fmt.Sprintf("Aporte em %s → %s", r.Name, linked.Name),
				ToDesc:   fmt.Sprintf("Aporte em %s ← %s", r.Name, otherAcc.Name),
			}
		} else {
			transfer = &store.TransferSpec{
				FromAccountID: linked.ID, ToAccountID: otherAcc.ID, AmountCents: in.AmountCents, Date: date,
				FromDesc: fmt.Sprintf("Resgate de %s → %s", r.Name, otherAcc.Name),
				ToDesc:   fmt.Sprintf("Resgate de %s ← %s", r.Name, linked.Name),
			}
		}
	}
	snap := &domain.InvestmentSnapshot{Date: date, ValueCents: r.CurrentCents, InvestedCents: r.InvestedCents}
	if err := s.Store.SaveInvestment(ctx, r, snap, transfer); err != nil {
		return domain.Investment{}, err
	}
	return s.GetInvestment(ctx, id)
}

// InvestmentHistoryMonths é quantos meses o resumo de investimentos devolve no histórico.
const InvestmentHistoryMonths = 12

// InvestmentSummary consolida as posições ativas: totais, rentabilidade, alocação e 12 meses de histórico.
func (s *Services) InvestmentSummary(ctx context.Context) (domain.InvestmentSummary, error) {
	items, err := s.ListInvestments(ctx, false)
	if err != nil {
		return domain.InvestmentSummary{}, err
	}
	snaps, err := s.Store.InvestmentSnapshots(ctx)
	if err != nil {
		return domain.InvestmentSummary{}, err
	}
	month, year := domain.CurrentMonthYear()
	return domain.SummarizeInvestments(items, snaps, domain.LastMonths(year, month, InvestmentHistoryMonths)), nil
}
