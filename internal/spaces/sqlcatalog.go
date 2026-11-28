package spaces

import (
	"context"
	"fmt"

	"finflow/internal/db"
	"finflow/internal/domain"
)

// SQLCatalog é o catálogo numa tabela SQLite (runtime do navegador: /finflow/_catalog.db). A ordem
// das linhas (position) é a ordem do catálogo.
type SQLCatalog struct{ DB *db.DB }

// NewSQLCatalog cria a tabela do catálogo se faltar.
func NewSQLCatalog(ctx context.Context, d *db.DB) (*SQLCatalog, error) {
	if _, err := d.Exec(ctx, `CREATE TABLE IF NOT EXISTS spaces (
		position INTEGER NOT NULL,
		slug TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		color TEXT NOT NULL,
		created_at TEXT NOT NULL,
		file TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("criar catálogo de espaços: %w", err)
	}
	return &SQLCatalog{DB: d}, nil
}

func (c *SQLCatalog) Load() ([]CatalogEntry, error) {
	rows, err := c.DB.Query(context.Background(), `SELECT slug, name, kind, color, created_at, file FROM spaces ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("ler catálogo de espaços: %w", err)
	}
	defer rows.Close()
	var out []CatalogEntry
	for rows.Next() {
		var e CatalogEntry
		var kind string
		if err := rows.Scan(&e.Slug, &e.Name, &kind, &e.Color, &e.CreatedAt, &e.File); err != nil {
			return nil, fmt.Errorf("ler catálogo de espaços: %w", err)
		}
		e.Kind = domain.SpaceKind(kind)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Save substitui o catálogo inteiro numa transação.
func (c *SQLCatalog) Save(entries []CatalogEntry) error {
	ctx := context.Background()
	tx, err := c.DB.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("gravar catálogo de espaços: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(ctx, `DELETE FROM spaces`); err != nil {
		return fmt.Errorf("gravar catálogo de espaços: %w", err)
	}
	for i, e := range entries {
		if _, err := tx.Exec(ctx, `INSERT INTO spaces (position, slug, name, kind, color, created_at, file) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			i, e.Slug, e.Name, string(e.Kind), e.Color, e.CreatedAt, e.File); err != nil {
			return fmt.Errorf("gravar catálogo de espaços: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gravar catálogo de espaços: %w", err)
	}
	return nil
}
