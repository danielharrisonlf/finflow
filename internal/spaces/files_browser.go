//go:build js && wasm

package spaces

import (
	"context"

	"finflow/internal/db"
)

// BrowserFiles guarda os espaços como arquivos do VFS do runtime do navegador (<Prefix><slug>.db,
// ex.: "/finflow/casa.db"). O VFS não tem lixeira: apagar um espaço remove o arquivo.
type BrowserFiles struct{ Prefix string }

func (f BrowserFiles) path(slug string) string { return f.Prefix + slug + ".db" }

func (f BrowserFiles) Open(ctx context.Context, slug string) (*db.DB, error) {
	return db.OpenBrowser(ctx, f.path(slug))
}

func (f BrowserFiles) Exists(slug string) bool { return db.BrowserFileExists(f.path(slug)) }

func (f BrowserFiles) Trash(slug string) error { return db.BrowserRemove(f.path(slug)) }

func (f BrowserFiles) Remove(slug string) { _ = db.BrowserRemove(f.path(slug)) }
