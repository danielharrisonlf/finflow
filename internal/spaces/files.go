package spaces

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"finflow/internal/db"
)

// SpaceFiles abre e descarta os bancos dos espaços adicionais (o slug já chega validado): pasta
// <dataDir>/spaces no desktop (DirFiles), VFS do worker no navegador.
type SpaceFiles interface {
	// Open abre (ou cria) o banco do espaço com o schema aplicado.
	Open(ctx context.Context, slug string) (*db.DB, error)
	// Exists indica se já há um banco com esse slug (ex.: sobra de uma cópia manual).
	Exists(slug string) bool
	// Trash tira o banco (já fechado) de uso ao apagar o espaço.
	Trash(slug string) error
	// Remove apaga um banco que o próprio registro acabou de criar (falha ao gravar o catálogo).
	Remove(slug string)
}

// DirFiles guarda os espaços em <Dir>/<slug>.db; apagar move para <Dir>/trash/<slug>-<data>.db.
type DirFiles struct{ Dir string }

// path é sempre derivado do slug (já validado), nunca do campo "file" do catálogo.
func (f DirFiles) path(slug string) string { return filepath.Join(f.Dir, slug+".db") }

func (f DirFiles) Open(ctx context.Context, slug string) (*db.DB, error) {
	return db.Open(ctx, "", f.path(slug))
}

func (f DirFiles) Exists(slug string) bool {
	_, err := os.Stat(f.path(slug))
	return err == nil
}

func (f DirFiles) Trash(slug string) error {
	src := f.path(slug)
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	trash := filepath.Join(f.Dir, "trash")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return fmt.Errorf("criar lixeira de espaços: %w", err)
	}
	dst := filepath.Join(trash, fmt.Sprintf("%s-%s.db", slug, time.Now().UTC().Format("20060102-150405")))
	for n := 2; ; n++ {
		if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
			break
		}
		dst = filepath.Join(trash, fmt.Sprintf("%s-%s-%d.db", slug, time.Now().UTC().Format("20060102-150405"), n))
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("mover espaço %q para a lixeira: %w", slug, err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(src + suffix); err == nil {
			_ = os.Rename(src+suffix, dst+suffix)
		}
	}
	return nil
}

func (f DirFiles) Remove(slug string) {
	path := f.path(slug)
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		_ = os.Remove(p)
	}
}
