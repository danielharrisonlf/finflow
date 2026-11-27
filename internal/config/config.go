package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// dotEnv lembra qual .env foi carregado e quais chaves vieram dele, para que caminhos
// relativos definidos no arquivo sejam resolvidos a partir do diretório do próprio .env.
var dotEnv struct {
	dir  string
	keys map[string]bool
}

// LoadDotEnv carrega variáveis de um arquivo .env (sem sobrescrever variáveis já definidas no ambiente).
func LoadDotEnv() {
	candidates := []string{".env"}
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for range 4 {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			candidates = append(candidates, filepath.Join(parent, ".env"))
			dir = parent
		}
	}
	for _, path := range candidates {
		if loadFile(path) {
			return
		}
	}
}

func loadFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	dir := filepath.Dir(path)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	dotEnv.dir = dir
	dotEnv.keys = map[string]bool{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
			dotEnv.keys[key] = true
		}
	}
	return true
}

type Config struct {
	Port                  int
	Host                  string
	DatabaseURL           string
	DBPath                string
	WebOrigin             string
	WebDir                string
	Password              string
	AuthSecret            string
	APIToken              string
	TelegramBotToken      string
	TelegramChatID        string
	TelegramWebhookSecret string
	TelegramAPIBase       string
	EnableSwagger         bool
}

func Load() Config {
	port := 3333
	if p := strings.TrimSpace(os.Getenv("API_PORT")); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			port = parsed
		}
	}
	dbPath := resolveDBPath(strings.TrimSpace(os.Getenv("FINFLOW_DB_PATH")))
	return Config{
		Port:                  port,
		Host:                  strings.TrimSpace(os.Getenv("API_HOST")),
		DatabaseURL:           resolveDatabaseURL(strings.TrimSpace(os.Getenv("DATABASE_URL"))),
		DBPath:                dbPath,
		WebOrigin:             strings.TrimSpace(os.Getenv("WEB_ORIGIN")),
		WebDir:                strings.TrimSpace(os.Getenv("FINFLOW_WEB_DIR")),
		Password:              strings.TrimSpace(os.Getenv("FINFLOW_PASSWORD")),
		AuthSecret:            strings.TrimSpace(os.Getenv("FINFLOW_AUTH_SECRET")),
		APIToken:              strings.TrimSpace(os.Getenv("FINFLOW_API_TOKEN")),
		TelegramBotToken:      strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:        strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		TelegramWebhookSecret: strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET")),
		TelegramAPIBase:       strings.TrimSpace(os.Getenv("TELEGRAM_API_BASE")),
		EnableSwagger:         strings.EqualFold(strings.TrimSpace(os.Getenv("ENABLE_SWAGGER")), "true"),
	}
}

// resolveDBPath define onde fica o SQLite local, sem depender do diretório de onde o binário
// foi chamado (um host MCP pode iniciar `finflow mcp` de qualquer lugar):
//   - caminho absoluto: usado como está;
//   - relativo vindo do .env: relativo ao diretório do .env;
//   - relativo vindo do ambiente real: relativo ao diretório atual (escolha explícita);
//   - vazio: <diretório de configuração do usuário>/finflow/finflow.db.
func resolveDBPath(raw string) string {
	if raw == "" {
		if base, err := os.UserConfigDir(); err == nil {
			return filepath.Join(base, "finflow", "finflow.db")
		}
		raw = "finflow.db"
	}
	if strings.HasPrefix(raw, ":memory:") || strings.HasPrefix(raw, "file:") || filepath.IsAbs(raw) {
		return raw
	}
	if dotEnv.keys["FINFLOW_DB_PATH"] && dotEnv.dir != "" {
		return filepath.Join(dotEnv.dir, raw)
	}
	if abs, err := filepath.Abs(raw); err == nil {
		return abs
	}
	return raw
}

// resolveDatabaseURL aplica ao SQLite em DATABASE_URL (sqlite:// ou file:) a mesma regra do
// FINFLOW_DB_PATH: caminho relativo vindo do .env é resolvido a partir da pasta do .env.
func resolveDatabaseURL(raw string) string {
	if !dotEnv.keys["DATABASE_URL"] || dotEnv.dir == "" {
		return raw
	}
	lower := strings.ToLower(raw)
	for _, prefix := range []string{"sqlite://", "file:"} {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		path := raw[len(prefix):]
		if path == "" || strings.HasPrefix(path, ":memory:") || strings.HasPrefix(path, "file:") || filepath.IsAbs(path) {
			return raw
		}
		return raw[:len(prefix)] + filepath.Join(dotEnv.dir, path)
	}
	return raw
}
