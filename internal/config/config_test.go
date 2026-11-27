package config

import (
	"os"
	"path/filepath"
	"testing"
)

// resetDotEnv limpa o estado global do último .env carregado.
func resetDotEnv(t *testing.T) {
	t.Helper()
	dotEnv.dir, dotEnv.keys = "", nil
	t.Cleanup(func() { dotEnv.dir, dotEnv.keys = "", nil })
}

// unsetEnv remove a variável durante o teste e a restaura no fim.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	_ = os.Unsetenv(key)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFileParsing(t *testing.T) {
	resetDotEnv(t)
	for _, k := range []string{"FF_T_PLAIN", "FF_T_DQ", "FF_T_SQ", "FF_T_SPACES", "FF_T_EQ", "FF_T_EMPTY", "FF_T_COMMENTED"} {
		unsetEnv(t, k)
	}
	t.Setenv("FF_T_EXISTING", "do-ambiente")

	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, `# comentário
FF_T_PLAIN=valor
FF_T_DQ="com aspas duplas"
FF_T_SQ='com aspas simples'
  FF_T_SPACES  =  espaçado
FF_T_EQ=a=b
FF_T_EMPTY=
# FF_T_COMMENTED=nao
linha sem igual
FF_T_EXISTING=do-arquivo
`)
	if !loadFile(path) {
		t.Fatal("loadFile retornou false para arquivo existente")
	}

	want := map[string]string{
		"FF_T_PLAIN":    "valor",
		"FF_T_DQ":       "com aspas duplas",
		"FF_T_SQ":       "com aspas simples",
		"FF_T_SPACES":   "espaçado",
		"FF_T_EQ":       "a=b",
		"FF_T_EMPTY":    "",
		"FF_T_EXISTING": "do-ambiente",
	}
	for k, v := range want {
		if got, ok := os.LookupEnv(k); !ok || got != v {
			t.Errorf("%s = %q (definida=%v), want %q", k, got, ok, v)
		}
	}
	if _, ok := os.LookupEnv("FF_T_COMMENTED"); ok {
		t.Error("linha comentada não deveria definir FF_T_COMMENTED")
	}
	if dotEnv.keys["FF_T_EXISTING"] {
		t.Error("variável já existente no ambiente não deveria ser marcada como vinda do .env")
	}
	if !dotEnv.keys["FF_T_PLAIN"] {
		t.Error("FF_T_PLAIN deveria ser marcada como vinda do .env")
	}
	if loadFile(filepath.Join(t.TempDir(), "nao-existe.env")) {
		t.Error("loadFile deveria retornar false para arquivo inexistente")
	}
}

func TestDBPathResolution(t *testing.T) {
	cfgHome := t.TempDir()
	// os.UserConfigDir: APPDATA (Windows), XDG_CONFIG_HOME (Linux), HOME (macOS).
	t.Setenv("APPDATA", cfgHome)
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("HOME", cfgHome)
	userCfg, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	absPath := filepath.Join(t.TempDir(), "abs.db")

	t.Run("vazio usa diretório de configuração do usuário", func(t *testing.T) {
		resetDotEnv(t)
		unsetEnv(t, "FINFLOW_DB_PATH")
		got := Load().DBPath
		want := filepath.Join(userCfg, "finflow", "finflow.db")
		if got != want {
			t.Fatalf("DBPath = %q, want %q", got, want)
		}
	})

	t.Run("absoluto é usado como está", func(t *testing.T) {
		resetDotEnv(t)
		t.Setenv("FINFLOW_DB_PATH", absPath)
		if got := Load().DBPath; got != absPath {
			t.Fatalf("DBPath = %q, want %q", got, absPath)
		}
	})

	t.Run("relativo do ambiente é relativo ao diretório atual", func(t *testing.T) {
		resetDotEnv(t)
		t.Setenv("FINFLOW_DB_PATH", filepath.Join("dados", "x.db"))
		want := filepath.Join(cwd, "dados", "x.db")
		if got := Load().DBPath; got != want {
			t.Fatalf("DBPath = %q, want %q", got, want)
		}
	})

	t.Run("relativo do .env é relativo ao diretório do .env", func(t *testing.T) {
		resetDotEnv(t)
		unsetEnv(t, "FINFLOW_DB_PATH")
		envDir := t.TempDir()
		writeFile(t, filepath.Join(envDir, ".env"), "FINFLOW_DB_PATH=dados/finflow.db\n")
		if !loadFile(filepath.Join(envDir, ".env")) {
			t.Fatal("loadFile falhou")
		}
		want := filepath.Join(envDir, "dados", "finflow.db")
		if got := Load().DBPath; got != want {
			t.Fatalf("DBPath = %q, want %q", got, want)
		}
	})

	t.Run("DATABASE_URL sqlite relativo do .env é relativo ao diretório do .env", func(t *testing.T) {
		resetDotEnv(t)
		unsetEnv(t, "DATABASE_URL")
		envDir := t.TempDir()
		writeFile(t, filepath.Join(envDir, ".env"), "DATABASE_URL=sqlite://dados/f.db\n")
		if !loadFile(filepath.Join(envDir, ".env")) {
			t.Fatal("loadFile falhou")
		}
		want := "sqlite://" + filepath.Join(envDir, "dados", "f.db")
		if got := Load().DatabaseURL; got != want {
			t.Fatalf("DatabaseURL = %q, want %q", got, want)
		}
	})

	t.Run("DATABASE_URL postgres do .env fica intacta", func(t *testing.T) {
		resetDotEnv(t)
		unsetEnv(t, "DATABASE_URL")
		envDir := t.TempDir()
		pg := "postgresql://u:p@localhost:5432/db?schema=public"
		writeFile(t, filepath.Join(envDir, ".env"), "DATABASE_URL="+pg+"\n")
		if !loadFile(filepath.Join(envDir, ".env")) {
			t.Fatal("loadFile falhou")
		}
		if got := Load().DatabaseURL; got != pg {
			t.Fatalf("DatabaseURL = %q, want %q", got, pg)
		}
	})

	t.Run("relativo do ambiente prevalece sobre o .env", func(t *testing.T) {
		resetDotEnv(t)
		t.Setenv("FINFLOW_DB_PATH", "ambiente.db")
		envDir := t.TempDir()
		writeFile(t, filepath.Join(envDir, ".env"), "FINFLOW_DB_PATH=arquivo.db\n")
		loadFile(filepath.Join(envDir, ".env"))
		want := filepath.Join(cwd, "ambiente.db")
		if got := Load().DBPath; got != want {
			t.Fatalf("DBPath = %q, want %q", got, want)
		}
	})

	t.Run("memória não é alterada", func(t *testing.T) {
		resetDotEnv(t)
		t.Setenv("FINFLOW_DB_PATH", ":memory:")
		if got := Load().DBPath; got != ":memory:" {
			t.Fatalf("DBPath = %q, want :memory:", got)
		}
	})

	t.Run("DATABASE_URL continua sendo repassada", func(t *testing.T) {
		resetDotEnv(t)
		t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/finflow")
		if got := Load().DatabaseURL; got != "postgres://u:p@localhost:5432/finflow" {
			t.Fatalf("DatabaseURL = %q", got)
		}
	})
}
