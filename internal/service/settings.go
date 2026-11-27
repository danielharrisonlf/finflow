package service

import (
	"context"
	"encoding/json"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Settings ─────────────────────────

// DefaultSettings são os valores usados quando a chave ainda não foi gravada.
func DefaultSettings() domain.Settings {
	return domain.Settings{
		ReserveMonths:        6,
		ReserveRefillRatePct: 10,
		BalanceBands:         domain.DefaultBalanceBands(),
	}
}

// GetSettings lê as configurações; valores ausentes ou ilegíveis ficam com o padrão.
func (s *Services) GetSettings(ctx context.Context) (domain.Settings, error) {
	values, err := s.Store.SettingValues(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	out := DefaultSettings()
	decode := func(key string, dst any) {
		if raw, ok := values[key]; ok {
			_ = json.Unmarshal([]byte(raw), dst)
		}
	}
	decode(store.SettingMonthlyLimit, &out.MonthlyLimitCents)
	decode(store.SettingReserveMonths, &out.ReserveMonths)
	decode(store.SettingHasDependents, &out.HasDependents)
	decode(store.SettingReserveRefillRate, &out.ReserveRefillRatePct)
	decode(store.SettingBalanceBands, &out.BalanceBands)
	decode(store.SettingSpendPlan, &out.SpendPlan)
	if issues := domain.ValidateSettings(out); len(issues) > 0 {
		// Valor gravado inconsistente (ex.: editado à mão): volta aos padrões dos campos estruturais.
		def := DefaultSettings()
		out.BalanceBands = def.BalanceBands
		if out.ReserveMonths < 1 || out.ReserveMonths > 24 {
			out.ReserveMonths = def.ReserveMonths
		}
	}
	return out, nil
}

// UpdateSettings aplica um merge parcial. null limpa monthlyLimitCents/spendPlan e devolve
// os demais campos ao padrão.
func (s *Services) UpdateSettings(ctx context.Context, in domain.UpdateSettingsInput) (domain.Settings, error) {
	cur, err := s.GetSettings(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	def := DefaultSettings()
	if in.MonthlyLimitCents.Set {
		cur.MonthlyLimitCents = nil
		if in.MonthlyLimitCents.Valid {
			v := in.MonthlyLimitCents.Value
			cur.MonthlyLimitCents = &v
		}
	}
	if in.ReserveMonths.Set {
		cur.ReserveMonths = def.ReserveMonths
		if in.ReserveMonths.Valid {
			cur.ReserveMonths = in.ReserveMonths.Value
		}
	}
	if in.HasDependents.Set {
		cur.HasDependents = in.HasDependents.Valid && in.HasDependents.Value
	}
	if in.ReserveRefillRatePct.Set {
		cur.ReserveRefillRatePct = def.ReserveRefillRatePct
		if in.ReserveRefillRatePct.Valid {
			cur.ReserveRefillRatePct = in.ReserveRefillRatePct.Value
		}
	}
	if in.BalanceBands.Set {
		if !in.BalanceBands.Valid {
			cur.BalanceBands = def.BalanceBands
		} else {
			p := in.BalanceBands.Value
			if p.DarkGreen != nil {
				cur.BalanceBands.DarkGreen = *p.DarkGreen
			}
			if p.LightGreen != nil {
				cur.BalanceBands.LightGreen = *p.LightGreen
			}
			if p.Yellow != nil {
				cur.BalanceBands.Yellow = *p.Yellow
			}
			if p.LightRed != nil {
				cur.BalanceBands.LightRed = *p.LightRed
			}
		}
	}
	if in.SpendPlan.Set {
		cur.SpendPlan = nil
		if in.SpendPlan.Valid {
			v := in.SpendPlan.Value
			cur.SpendPlan = &v
		}
	}
	if issues := domain.ValidateSettings(cur); len(issues) > 0 {
		return domain.Settings{}, ValidationFailed(issues)
	}
	values := map[string]any{
		store.SettingMonthlyLimit:      cur.MonthlyLimitCents,
		store.SettingReserveMonths:     cur.ReserveMonths,
		store.SettingHasDependents:     cur.HasDependents,
		store.SettingReserveRefillRate: cur.ReserveRefillRatePct,
		store.SettingBalanceBands:      cur.BalanceBands,
		store.SettingSpendPlan:         cur.SpendPlan,
	}
	encoded := make(map[string]string, len(values))
	for k, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return domain.Settings{}, err
		}
		encoded[k] = string(b)
	}
	if err := s.Store.SaveSettingValues(ctx, encoded); err != nil {
		return domain.Settings{}, err
	}
	return cur, nil
}
