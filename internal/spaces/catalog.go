// Package spaces implementa os espaços (v3 fase 2): cada espaço é um banco SQLite separado em
// <dataDir>/spaces/<slug>.db, listado no catálogo <dataDir>/spaces.json. O banco atual continua
// sendo o espaço "pessoal" e nunca é movido. No runtime do navegador o catálogo é uma tabela SQLite
// (SQLCatalog) e os bancos são arquivos do VFS do worker (Options.Files).
package spaces

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"finflow/internal/domain"
)

// CatalogFile é o nome do catálogo dentro da pasta de dados.
const CatalogFile = "spaces.json"

// CatalogEntry é um espaço no catálogo; File é relativo à pasta de dados (informativo: o caminho
// real é sempre derivado do slug, para que um catálogo editado à mão não aponte para fora).
type CatalogEntry struct {
	Slug      string           `json:"slug"`
	Name      string           `json:"name"`
	Kind      domain.SpaceKind `json:"kind"`
	Color     string           `json:"color"`
	CreatedAt string           `json:"createdAt"`
	File      string           `json:"file"`
}

type catalog struct {
	Spaces []CatalogEntry `json:"spaces"`
}

// Catalog guarda a lista de espaços, na ordem: arquivo spaces.json no desktop (FileCatalog), tabela
// SQLite no navegador (SQLCatalog). Load com o catálogo ausente devolve lista vazia.
type Catalog interface {
	Load() ([]CatalogEntry, error)
	Save([]CatalogEntry) error
}

// FileCatalog é o catálogo em arquivo JSON (gravação atômica).
type FileCatalog struct{ Path string }

func (f FileCatalog) Load() ([]CatalogEntry, error) {
	c, err := readCatalog(f.Path)
	return c.Spaces, err
}

func (f FileCatalog) Save(entries []CatalogEntry) error {
	return writeCatalog(f.Path, catalog{Spaces: entries})
}

func (c *catalog) find(slug string) int {
	for i, e := range c.Spaces {
		if e.Slug == slug {
			return i
		}
	}
	return -1
}

// readCatalog lê o catálogo; ausente = vazio.
func readCatalog(path string) (catalog, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return catalog{}, nil
	}
	if err != nil {
		return catalog{}, fmt.Errorf("ler catálogo de espaços: %w", err)
	}
	var c catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return catalog{}, fmt.Errorf("catálogo de espaços inválido (%s): %w", path, err)
	}
	return c, nil
}

// cleanCatalog ignora entradas com slug inválido ou repetido e corrige tipo e cor inválidos.
func cleanCatalog(entries []CatalogEntry) catalog {
	clean := catalog{Spaces: make([]CatalogEntry, 0, len(entries))}
	seen := map[string]bool{}
	for _, e := range entries {
		if !domain.IsValidSpaceSlug(e.Slug) || seen[e.Slug] {
			continue
		}
		if !e.Kind.Valid() {
			e.Kind = domain.SpacePersonal
		}
		if !domain.IsValidHexColor(e.Color) {
			e.Color = domain.DefaultSpaceColor(e.Kind)
		}
		seen[e.Slug] = true
		clean.Spaces = append(clean.Spaces, e)
	}
	return clean
}

// writeCatalog grava o catálogo de forma atômica: arquivo temporário na mesma pasta + rename.
func writeCatalog(path string, c catalog) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("criar pasta de dados: %w", err)
	}
	tmp, err := os.CreateTemp(dir, CatalogFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("gravar catálogo de espaços: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op depois do rename
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("gravar catálogo de espaços: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("gravar catálogo de espaços: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("gravar catálogo de espaços: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("gravar catálogo de espaços: %w", err)
	}
	return nil
}
