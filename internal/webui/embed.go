package webui

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed all:dist
var embeddedFS embed.FS

// DistFS retorna o filesystem do frontend compilado (prioriza override em disco, depois o embutido no binário).
func DistFS(overrideDir string) (fs.FS, bool) {
	if overrideDir != "" {
		if _, err := os.Stat(overrideDir + "/index.html"); err == nil {
			return os.DirFS(overrideDir), true
		}
	}
	sub, err := fs.Sub(embeddedFS, "dist")
	if err != nil {
		return nil, false
	}
	if f, err := sub.Open("index.html"); err == nil {
		_ = f.Close()
		return sub, true
	}
	if _, err := os.Stat("apps/web/dist/index.html"); err == nil {
		return os.DirFS("apps/web/dist"), true
	}
	return nil, false
}
