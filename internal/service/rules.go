package service

import (
	"context"
	"strings"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Category rules ─────────────────────────

func toCategoryRule(r store.CategoryRuleRow) domain.CategoryRule {
	return domain.CategoryRule{
		ID:           r.ID,
		Pattern:      r.Pattern,
		CategoryID:   r.CategoryID,
		CategoryName: r.CategoryName,
		Priority:     r.Priority,
	}
}

func (s *Services) ListCategoryRules(ctx context.Context) ([]domain.CategoryRule, error) {
	rows, err := s.Store.ListCategoryRules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CategoryRule, 0, len(rows))
	for _, r := range rows {
		out = append(out, toCategoryRule(r))
	}
	return out, nil
}

func (s *Services) requireCategory(ctx context.Context, id string) error {
	cat, err := s.Store.FindCategoryByID(ctx, id)
	if err != nil {
		return err
	}
	if cat == nil {
		return NotFound("categoria não encontrada")
	}
	return nil
}

func (s *Services) getCategoryRule(ctx context.Context, id string) (domain.CategoryRule, error) {
	r, err := s.Store.FindCategoryRuleByID(ctx, id)
	if err != nil {
		return domain.CategoryRule{}, err
	}
	if r == nil {
		return domain.CategoryRule{}, NotFound("regra não encontrada")
	}
	return toCategoryRule(*r), nil
}

func (s *Services) CreateCategoryRule(ctx context.Context, in domain.CreateCategoryRuleInput) (domain.CategoryRule, error) {
	if issues := domain.ValidateCreateCategoryRule(in); len(issues) > 0 {
		return domain.CategoryRule{}, ValidationFailed(issues)
	}
	if err := s.requireCategory(ctx, in.CategoryID); err != nil {
		return domain.CategoryRule{}, err
	}
	priority := 0
	if in.Priority != nil {
		priority = *in.Priority
	}
	id, err := s.Store.CreateCategoryRule(ctx, strings.TrimSpace(in.Pattern), in.CategoryID, priority)
	if err != nil {
		return domain.CategoryRule{}, err
	}
	return s.getCategoryRule(ctx, id)
}

func (s *Services) UpdateCategoryRule(ctx context.Context, id string, in domain.UpdateCategoryRuleInput) (domain.CategoryRule, error) {
	if issues := domain.ValidateUpdateCategoryRule(in); len(issues) > 0 {
		return domain.CategoryRule{}, ValidationFailed(issues)
	}
	r, err := s.Store.FindCategoryRuleByID(ctx, id)
	if err != nil {
		return domain.CategoryRule{}, err
	}
	if r == nil {
		return domain.CategoryRule{}, NotFound("regra não encontrada")
	}
	if in.Pattern.Set && in.Pattern.Valid {
		r.Pattern = strings.TrimSpace(in.Pattern.Value)
	}
	if in.CategoryID.Set && in.CategoryID.Valid {
		if err := s.requireCategory(ctx, in.CategoryID.Value); err != nil {
			return domain.CategoryRule{}, err
		}
		r.CategoryID = in.CategoryID.Value
	}
	if in.Priority.Set && in.Priority.Valid {
		r.Priority = in.Priority.Value
	}
	if err := s.Store.UpdateCategoryRule(ctx, r); err != nil {
		return domain.CategoryRule{}, err
	}
	return s.getCategoryRule(ctx, id)
}

func (s *Services) DeleteCategoryRule(ctx context.Context, id string) error {
	r, err := s.Store.FindCategoryRuleByID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return NotFound("regra não encontrada")
	}
	return s.Store.DeleteCategoryRule(ctx, id)
}

func (s *Services) ruleCandidates(ctx context.Context) ([]domain.RuleCandidate, error) {
	rows, err := s.Store.ListCategoryRules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RuleCandidate, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.RuleCandidate{
			Pattern:      r.Pattern,
			CategoryID:   r.CategoryID,
			CategoryKind: r.CategoryKind,
			Priority:     r.Priority,
		})
	}
	return out, nil
}

// SuggestCategory devolve a categoria da regra de automação que casa com a descrição
// (ou nil). kind é o tipo da categoria: INCOME para receitas, EXPENSE para despesas e
// compras no cartão.
func (s *Services) SuggestCategory(ctx context.Context, description string, kind domain.CategoryKind) (*string, error) {
	if !kind.Valid() {
		return nil, nil
	}
	rules, err := s.ruleCandidates(ctx)
	if err != nil {
		return nil, err
	}
	return domain.MatchCategoryRule(rules, description, kind), nil
}

// ApplyCategoryRules aplica as regras às transações existentes (por padrão só às sem categoria).
func (s *Services) ApplyCategoryRules(ctx context.Context, in domain.ApplyRulesInput) (domain.ApplyRulesResult, error) {
	onlyUncategorized := true
	if in.OnlyUncategorized != nil {
		onlyUncategorized = *in.OnlyUncategorized
	}
	rules, err := s.ruleCandidates(ctx)
	if err != nil {
		return domain.ApplyRulesResult{}, err
	}
	if len(rules) == 0 {
		return domain.ApplyRulesResult{Updated: 0}, nil
	}
	targets, err := s.Store.ListRuleTargets(ctx, onlyUncategorized)
	if err != nil {
		return domain.ApplyRulesResult{}, err
	}
	assignments := make(map[string]string)
	for _, t := range targets {
		kind, ok := domain.CategoryKindForTransaction(t.Type)
		if !ok {
			continue
		}
		suggested := domain.MatchCategoryRule(rules, t.Description, kind)
		if suggested == nil || (t.CategoryID != nil && *t.CategoryID == *suggested) {
			continue
		}
		assignments[t.ID] = *suggested
	}
	if err := s.Store.AssignCategories(ctx, assignments); err != nil {
		return domain.ApplyRulesResult{}, err
	}
	return domain.ApplyRulesResult{Updated: len(assignments)}, nil
}
