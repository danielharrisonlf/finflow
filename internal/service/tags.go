package service

import (
	"context"
	"fmt"
	"strings"

	"finflow/internal/domain"
)

// ───────────────────────── Tags ─────────────────────────

func (s *Services) ListTags(ctx context.Context) ([]domain.Tag, error) {
	return s.Store.ListTags(ctx)
}

func (s *Services) getTag(ctx context.Context, id string) (domain.Tag, error) {
	t, err := s.Store.FindTagByID(ctx, id)
	if err != nil {
		return domain.Tag{}, err
	}
	if t == nil {
		return domain.Tag{}, NotFound("tag não encontrada")
	}
	return *t, nil
}

// tagNameTaken confere o nome sem diferenciar maiúsculas/acentos (ignorando a própria tag).
func (s *Services) tagNameTaken(ctx context.Context, name, exceptID string) (bool, error) {
	tags, err := s.Store.ListTags(ctx)
	if err != nil {
		return false, err
	}
	norm := domain.NormalizeText(name)
	for _, t := range tags {
		if t.ID != exceptID && domain.NormalizeText(t.Name) == norm {
			return true, nil
		}
	}
	return false, nil
}

func (s *Services) CreateTag(ctx context.Context, in domain.CreateTagInput) (domain.Tag, error) {
	if issues := domain.ValidateCreateTag(in); len(issues) > 0 {
		return domain.Tag{}, ValidationFailed(issues)
	}
	name := strings.TrimSpace(in.Name)
	taken, err := s.tagNameTaken(ctx, name, "")
	if err != nil {
		return domain.Tag{}, err
	}
	if taken {
		return domain.Tag{}, Conflict(fmt.Sprintf("tag %q já existe", name))
	}
	color := domain.DefaultTagColor
	if in.Color != nil {
		color = *in.Color
	}
	id, err := s.Store.CreateTag(ctx, name, color)
	if err != nil {
		return domain.Tag{}, err
	}
	return s.getTag(ctx, id)
}

func (s *Services) UpdateTag(ctx context.Context, id string, in domain.UpdateTagInput) (domain.Tag, error) {
	if issues := domain.ValidateUpdateTag(in); len(issues) > 0 {
		return domain.Tag{}, ValidationFailed(issues)
	}
	cur, err := s.getTag(ctx, id)
	if err != nil {
		return domain.Tag{}, err
	}
	if in.Name.Set {
		name := strings.TrimSpace(in.Name.Value)
		taken, err := s.tagNameTaken(ctx, name, id)
		if err != nil {
			return domain.Tag{}, err
		}
		if taken {
			return domain.Tag{}, Conflict(fmt.Sprintf("tag %q já existe", name))
		}
		cur.Name = name
	}
	if in.Color.Set {
		cur.Color = in.Color.Value
	}
	if err := s.Store.UpdateTag(ctx, id, cur.Name, cur.Color); err != nil {
		return domain.Tag{}, err
	}
	return s.getTag(ctx, id)
}

func (s *Services) DeleteTag(ctx context.Context, id string) error {
	if _, err := s.getTag(ctx, id); err != nil {
		return err
	}
	return s.Store.DeleteTag(ctx, id)
}

// requireTags garante que todas as tags existem (404 na primeira ausente).
func (s *Services) requireTags(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	exist, err := s.Store.ExistingTagIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if !exist[id] {
			return NotFound("tag não encontrada")
		}
	}
	return nil
}

// ruleTagFor devolve a tag da regra de automação que casa com a descrição (ou nil).
func (s *Services) ruleTagFor(ctx context.Context, description string, kind domain.CategoryKind) (*string, error) {
	if !kind.Valid() {
		return nil, nil
	}
	rules, err := s.ruleCandidates(ctx)
	if err != nil {
		return nil, err
	}
	if r := domain.MatchRule(rules, description, kind); r != nil && r.TagID != nil {
		id := *r.TagID
		return &id, nil
	}
	return nil, nil
}

// withRuleTag soma às tags informadas a tag aplicada pela regra que casa com a descrição.
func (s *Services) withRuleTag(ctx context.Context, tagIDs []string, description string, kind domain.CategoryKind) ([]string, error) {
	ruleTag, err := s.ruleTagFor(ctx, description, kind)
	if err != nil {
		return nil, err
	}
	if ruleTag != nil {
		tagIDs = append(append([]string{}, tagIDs...), *ruleTag)
	}
	return domain.UniqueStrings(tagIDs), nil
}

// InsightsTags devolve o gasto do mês (EXPENSE + CARD_EXPENSE) por tag, do maior para o menor.
func (s *Services) InsightsTags(ctx context.Context, q domain.PeriodQuery) ([]domain.TagSpending, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return nil, err
	}
	start, end := domain.MonthRange(year, month)
	return s.Store.TagSpendingBetween(ctx, start, end)
}
