package service

import (
	"context"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Wishlist ─────────────────────────

func toWishItem(r store.WishItemRow) domain.WishItem {
	return domain.WishItem{
		ID:         r.ID,
		Name:       r.Name,
		PriceCents: r.PriceCents,
		URL:        r.URL,
		Notes:      r.Notes,
		Priority:   r.Priority,
		Status:     r.Status,
	}
}

// ListWishlist devolve os desejos com affordableOn: a 1ª data em que o saldo projetado
// comporta o item somado aos LISTED de maior prioridade e segue >= 0 até o fim do horizonte.
func (s *Services) ListWishlist(ctx context.Context) ([]domain.WishItem, error) {
	rows, err := s.Store.ListWishItems(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.WishItem, 0, len(rows))
	var costs []int64
	var listedIdx []int
	var cumulative int64
	for _, r := range rows {
		out = append(out, toWishItem(r))
		if r.Status == domain.WishListed {
			cumulative += r.PriceCents
			costs = append(costs, cumulative)
			listedIdx = append(listedIdx, len(out)-1)
		}
	}
	if len(costs) == 0 {
		return out, nil
	}
	p, err := s.simulateProjection(ctx)
	if err != nil {
		return nil, err
	}
	horizon := p.Days
	if len(horizon) > domain.ProjectionHorizonDays {
		horizon = horizon[:domain.ProjectionHorizonDays]
	}
	for k, date := range domain.AffordableDates(horizon, costs) {
		out[listedIdx[k]].AffordableOn = date
	}
	return out, nil
}

func (s *Services) getWishItem(ctx context.Context, id string) (domain.WishItem, error) {
	items, err := s.ListWishlist(ctx)
	if err != nil {
		return domain.WishItem{}, err
	}
	for _, it := range items {
		if it.ID == id {
			return it, nil
		}
	}
	return domain.WishItem{}, NotFound("desejo não encontrado")
}

func (s *Services) CreateWishItem(ctx context.Context, in domain.CreateWishItemInput) (domain.WishItem, error) {
	if issues := domain.ValidateCreateWishItem(in); len(issues) > 0 {
		return domain.WishItem{}, ValidationFailed(issues)
	}
	priority := 0
	if in.Priority != nil {
		priority = *in.Priority
	} else {
		next, err := s.Store.NextWishPriority(ctx)
		if err != nil {
			return domain.WishItem{}, err
		}
		priority = next
	}
	id, err := s.Store.CreateWishItem(ctx, store.WishItemRow{
		Name:       in.Name,
		PriceCents: in.PriceCents,
		URL:        in.URL,
		Notes:      in.Notes,
		Priority:   priority,
		Status:     domain.WishListed,
	})
	if err != nil {
		return domain.WishItem{}, err
	}
	return s.getWishItem(ctx, id)
}

func (s *Services) UpdateWishItem(ctx context.Context, id string, in domain.UpdateWishItemInput) (domain.WishItem, error) {
	if issues := domain.ValidateUpdateWishItem(in); len(issues) > 0 {
		return domain.WishItem{}, ValidationFailed(issues)
	}
	r, err := s.Store.FindWishItemByID(ctx, id)
	if err != nil {
		return domain.WishItem{}, err
	}
	if r == nil {
		return domain.WishItem{}, NotFound("desejo não encontrado")
	}
	if in.Name.Set && in.Name.Valid {
		r.Name = in.Name.Value
	}
	if in.PriceCents.Set && in.PriceCents.Valid {
		r.PriceCents = in.PriceCents.Value
	}
	if in.URL.Set {
		r.URL = optionalString(in.URL)
	}
	if in.Notes.Set {
		r.Notes = optionalString(in.Notes)
	}
	if in.Priority.Set && in.Priority.Valid {
		r.Priority = in.Priority.Value
	}
	if in.Status.Set && in.Status.Valid {
		r.Status = in.Status.Value
	}
	if err := s.Store.UpdateWishItem(ctx, r); err != nil {
		return domain.WishItem{}, err
	}
	return s.getWishItem(ctx, id)
}

func (s *Services) DeleteWishItem(ctx context.Context, id string) error {
	r, err := s.Store.FindWishItemByID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return NotFound("desejo não encontrado")
	}
	return s.Store.DeleteWishItem(ctx, id)
}
