package spaces

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/service"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

// O registro com catálogo SQLite e arquivos injetados (configuração do runtime do navegador).
func TestRegistryWithSQLCatalog(t *testing.T) {
	ctx := context.Background()
	catDB := testutil.Open(t, db.DialectSQLite)
	cat, err := NewSQLCatalog(ctx, catDB)
	if err != nil {
		t.Fatal(err)
	}
	base := service.New(store.New(testutil.Open(t, db.DialectSQLite)), nil)
	dir := t.TempDir()
	reg := New(base, Options{Catalog: cat, Files: DirFiles{Dir: dir}})
	t.Cleanup(func() { _ = reg.Close() })
	if !reg.Supported() {
		t.Fatal("catálogo + arquivos injetados deveriam habilitar espaços")
	}

	if _, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "Casa", Kind: domain.SpaceHome}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Create(ctx, domain.CreateSpaceInput{Name: "Empresa", Kind: domain.SpaceBusiness}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "casa.db")); err != nil {
		t.Fatalf("banco do espaço: %v", err)
	}
	if _, err := reg.Update(domain.DefaultSpaceSlug, domain.UpdateSpaceInput{Name: domain.Optional[string]{Set: true, Valid: true, Value: "Eu"}}); err != nil {
		t.Fatal(err)
	}

	// Um registro novo sobre o mesmo catálogo enxerga a mesma lista, na mesma ordem.
	other := New(base, Options{Catalog: cat, Files: DirFiles{Dir: dir}})
	t.Cleanup(func() { _ = other.Close() })
	list, err := other.List()
	if err != nil || len(list) != 3 || list[0].Name != "Eu" || list[1].Slug != "casa" || list[2].Slug != "empresa" || list[2].Kind != domain.SpaceBusiness {
		t.Fatalf("lista: %+v %v", list, err)
	}

	if err := reg.Delete("casa", "casa"); err != nil {
		t.Fatal(err)
	}
	entries, err := cat.Load()
	if err != nil || len(entries) != 2 || entries[0].Slug != domain.DefaultSpaceSlug || entries[1].Slug != "empresa" {
		t.Fatalf("catálogo depois de apagar: %+v %v", entries, err)
	}
}
