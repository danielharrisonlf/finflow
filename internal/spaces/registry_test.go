package spaces

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

func newTestRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	base := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	dir := t.TempDir()
	reg := New(base, Options{DataDir: dir})
	t.Cleanup(func() { _ = reg.Close() })
	return reg, dir
}

func appStatus(err error) int {
	var appErr *service.AppError
	if errors.As(err, &appErr) {
		return appErr.Status
	}
	return 0
}

func TestRegistryLifecycle(t *testing.T) {
	ctx := context.Background()
	reg, dir := newTestRegistry(t)

	list, err := reg.List()
	if err != nil || len(list) != 1 || list[0].Slug != domain.DefaultSpaceSlug || !list[0].IsDefault || list[0].Kind != domain.SpacePersonal {
		t.Fatalf("sem catálogo: %+v %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(dir, CatalogFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("listar não deveria criar o catálogo: %v", err)
	}

	casa, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "Casa", Kind: domain.SpaceHome})
	if err != nil || casa.Slug != "casa" || casa.Kind != domain.SpaceHome || casa.Color != domain.DefaultSpaceColor(domain.SpaceHome) || casa.IsDefault || casa.CreatedAt == "" {
		t.Fatalf("criar casa: %+v %v", casa, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "spaces", "casa.db")); err != nil {
		t.Fatalf("banco do espaço: %v", err)
	}
	color := "#123456"
	empresa, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "Minha Empresa", Kind: domain.SpaceBusiness, Color: &color})
	if err != nil || empresa.Slug != "minha-empresa" || empresa.Color != color {
		t.Fatalf("criar empresa: %+v %v", empresa, err)
	}
	dup, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "Casa", Kind: domain.SpaceHome})
	if err != nil || dup.Slug != "casa-2" {
		t.Fatalf("slug repetido derivado: %+v %v", dup, err)
	}
	for _, slug := range []string{"casa", "pessoal"} {
		s := slug
		if _, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "X", Kind: domain.SpaceHome, Slug: &s}); appStatus(err) != 409 {
			t.Errorf("slug explícito %q: %v", slug, err)
		}
	}
	bad := "../fora"
	if _, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "X", Kind: domain.SpaceHome, Slug: &bad}); appStatus(err) != 400 {
		t.Errorf("slug inválido: %v", err)
	}
	if _, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "Pessoal", Kind: domain.SpacePersonal}); err != nil {
		t.Errorf("nome Pessoal deveria virar pessoal-2: %v", err)
	}

	// Categorias padrão por tipo.
	casaSvcs, err := reg.Services(ctx, "casa")
	if err != nil {
		t.Fatal(err)
	}
	again, _ := reg.Services(ctx, "casa")
	if again != casaSvcs {
		t.Fatal("serviços do espaço deveriam ficar em cache")
	}
	hasCategory := func(svcs *service.Services, name string) bool {
		cats, err := svcs.ListCategories(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cats {
			if c.Name == name {
				return true
			}
		}
		return false
	}
	if !hasCategory(casaSvcs, "Moradia") || hasCategory(casaSvcs, "Pró-labore") {
		t.Fatal("espaço Casa deveria ter a taxonomia pessoal")
	}
	bizSvcs, err := reg.Services(ctx, "minha-empresa")
	if err != nil || !hasCategory(bizSvcs, "Pró-labore") || hasCategory(bizSvcs, "Moradia") {
		t.Fatalf("espaço Empresa deveria ter as categorias de empresa: %v", err)
	}
	if base, _ := reg.Services(ctx, ""); base != reg.Base() {
		t.Fatal("slug vazio = pessoal")
	}
	for _, slug := range []string{"nao-existe", "../casa", "CASA", "casa.db"} {
		if _, err := reg.Services(ctx, slug); appStatus(err) != 404 {
			t.Errorf("Services(%q) = %v", slug, err)
		}
	}

	// Outro processo (novo registro na mesma pasta) enxerga os espaços.
	other := New(reg.Base(), Options{DataDir: dir})
	t.Cleanup(func() { _ = other.Close() })
	otherList, err := other.List()
	if err != nil || len(otherList) != 5 || otherList[1].Slug != "casa" {
		t.Fatalf("outro registro: %+v %v", otherList, err)
	}
	_ = other.Close()

	// Atualizar nome e cor (inclusive do pessoal).
	upd, err := reg.Update("casa", domain.UpdateSpaceInput{Name: domain.Optional[string]{Set: true, Valid: true, Value: " Casa da Praia "}})
	if err != nil || upd.Name != "Casa da Praia" || upd.Slug != "casa" {
		t.Fatalf("atualizar: %+v %v", upd, err)
	}
	pes, err := reg.Update(domain.DefaultSpaceSlug, domain.UpdateSpaceInput{Color: domain.Optional[string]{Set: true, Valid: true, Value: "#000000"}})
	if err != nil || pes.Color != "#000000" || !pes.IsDefault {
		t.Fatalf("atualizar pessoal: %+v %v", pes, err)
	}
	if _, err := reg.Update("nao-existe", domain.UpdateSpaceInput{}); appStatus(err) != 404 {
		t.Errorf("atualizar inexistente: %v", err)
	}

	// Apagar: pessoal 409, confirmação errada 400, depois move para a lixeira.
	if err := reg.Delete(domain.DefaultSpaceSlug, domain.DefaultSpaceSlug); appStatus(err) != 409 {
		t.Errorf("apagar pessoal: %v", err)
	}
	if err := reg.Delete("casa", "outra"); appStatus(err) != 400 {
		t.Errorf("confirmação errada: %v", err)
	}
	if err := reg.Delete("nao-existe", "nao-existe"); appStatus(err) != 404 {
		t.Errorf("apagar inexistente: %v", err)
	}
	if err := reg.Delete("casa", "casa"); err != nil {
		t.Fatalf("apagar casa: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "spaces", "casa.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("banco deveria sair da pasta de espaços: %v", err)
	}
	trash, _ := filepath.Glob(filepath.Join(dir, "spaces", "trash", "casa-*.db"))
	if len(trash) != 1 {
		t.Fatalf("lixeira: %v", trash)
	}
	if _, err := reg.Services(ctx, "casa"); appStatus(err) != 404 {
		t.Fatalf("espaço apagado: %v", err)
	}
	// O slug volta a ficar livre (o arquivo foi para a lixeira).
	if again, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "Casa", Kind: domain.SpaceHome}); err != nil || again.Slug != "casa" {
		t.Fatalf("recriar casa: %+v %v", again, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, CatalogFile))
	if err != nil || !strings.Contains(string(raw), `"file": "spaces/casa.db"`) || !strings.Contains(string(raw), `"slug": "pessoal"`) {
		t.Fatalf("catálogo: %s %v", raw, err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, CatalogFile+".*.tmp")); len(leftovers) != 0 {
		t.Fatalf("temporários do catálogo: %v", leftovers)
	}

	if err := reg.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Services(ctx, "minha-empresa"); err == nil {
		t.Fatal("registro encerrado deveria recusar")
	}
}

func TestRegistryOverviewAndConcurrency(t *testing.T) {
	ctx := context.Background()
	reg, _ := newTestRegistry(t)
	if _, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "Casa", Kind: domain.SpaceHome}); err != nil {
		t.Fatal(err)
	}
	casa, err := reg.Services(ctx, "casa")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := casa.CreateAccount(ctx, domain.CreateAccountInput{Name: "Casa", Type: domain.AccountChecking, InitialBalanceCents: 70000}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Base().CreateAccount(ctx, domain.CreateAccountInput{Name: "Pessoal", Type: domain.AccountChecking, InitialBalanceCents: 30000}); err != nil {
		t.Fatal(err)
	}
	ov, err := reg.Overview(ctx)
	if err != nil || len(ov.Spaces) != 2 || ov.Spaces[0].BalanceCents != 30000 || ov.Spaces[1].BalanceCents != 70000 || ov.Totals.BalanceCents != 100000 || ov.Totals.NetWorthCents != 100000 {
		t.Fatalf("visão unificada: %+v %v", ov, err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			svcs, err := reg.Services(ctx, "casa")
			if err == nil && svcs != casa {
				err = errors.New("instância diferente")
			}
			errs <- err
		}()
		go func() {
			defer wg.Done()
			_, err := reg.List()
			if i%5 == 0 && err == nil {
				_, err = reg.Overview(ctx)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRegistryUnsupportedAndCatalog(t *testing.T) {
	ctx := context.Background()
	base := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	single := Single(base)
	if _, err := single.Create(ctx, domain.CreateSpaceInput{Name: "Casa", Kind: domain.SpaceHome}); appStatus(err) != 501 {
		t.Fatalf("criar sem pasta de dados: %v", err)
	}
	if _, err := single.Update(domain.DefaultSpaceSlug, domain.UpdateSpaceInput{}); appStatus(err) != 501 {
		t.Fatalf("atualizar sem pasta de dados: %v", err)
	}
	if list, err := single.List(); err != nil || len(list) != 1 {
		t.Fatalf("listar: %+v %v", list, err)
	}
	if _, err := single.Services(ctx, "casa"); appStatus(err) != 404 {
		t.Fatalf("Services sem pasta: %v", err)
	}
	if ov, err := single.Overview(ctx); err != nil || len(ov.Spaces) != 1 {
		t.Fatalf("visão unificada: %+v %v", ov, err)
	}

	if o := OptionsFor(&db.DB{Dialect: db.DialectPostgres}); o.DataDir != "" || o.Unsupported != UnsupportedPostgres {
		t.Fatalf("OptionsFor postgres: %+v", o)
	}
	if o := OptionsFor(&db.DB{Dialect: db.DialectSQLite, Target: ":memory:"}); o.DataDir != "" || o.Unsupported != UnsupportedMemory {
		t.Fatalf("OptionsFor memória: %+v", o)
	}
	dir := t.TempDir()
	if o := OptionsFor(&db.DB{Dialect: db.DialectSQLite, Target: filepath.Join(dir, "finflow.db")}); o.DataDir != dir || o.PersonalFile != filepath.Join(dir, "finflow.db") {
		t.Fatalf("OptionsFor arquivo: %+v", o)
	}

	// Catálogo editado à mão: entradas inválidas são ignoradas e o caminho nunca sai da pasta.
	manual := `{"spaces":[{"slug":"../evil","name":"x","kind":"HOME","file":"../../evil.db"},` +
		`{"slug":"ok","name":"Ok","kind":"WHAT","color":"red","file":"../../evil.db"},{"slug":"ok","name":"dup","kind":"HOME"}]}`
	if err := os.WriteFile(filepath.Join(dir, CatalogFile), []byte(manual), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := New(base, Options{DataDir: dir})
	t.Cleanup(func() { _ = reg.Close() })
	list, err := reg.List()
	if err != nil || len(list) != 2 || list[1].Slug != "ok" || list[1].Kind != domain.SpacePersonal || list[1].Color != domain.DefaultSpaceColor(domain.SpacePersonal) {
		t.Fatalf("catálogo saneado: %+v %v", list, err)
	}
	if _, err := reg.Services(ctx, "ok"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "spaces", "ok.db")); err != nil {
		t.Fatalf("o banco deve seguir o slug, não o campo file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, CatalogFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.List(); err == nil {
		t.Fatal("catálogo corrompido deveria dar erro")
	}
}
