package service

import (
	"context"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Dados de exemplo ─────────────────────────

// SeedDemo popula o espaço com os dados de exemplo do tipo (Casa, Empresa ou pessoal). Só roda num
// banco sem dados do usuário; caso contrário responde 409.
func (s *Services) SeedDemo(ctx context.Context, kind domain.SpaceKind) error {
	has, err := s.Store.HasUserData(ctx)
	if err != nil {
		return err
	}
	if has {
		return Conflict("o espaço já tem dados: os dados de exemplo só podem ser criados num espaço vazio")
	}
	var seeded bool
	switch kind {
	case domain.SpaceHome:
		seeded, err = store.SeedHome(ctx, s.Store)
	case domain.SpaceBusiness:
		seeded, err = store.SeedBusiness(ctx, s.Store)
	default:
		seeded, err = store.Seed(ctx, s.Store)
	}
	if err != nil {
		return err
	}
	if !seeded {
		return Conflict("o espaço já tem dados: os dados de exemplo só podem ser criados num espaço vazio")
	}
	return nil
}
