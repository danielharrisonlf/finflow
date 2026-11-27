package cli

import (
	"errors"
	"os"
	"strings"
)

// Version é a versão do binário (preenchida pelo main a partir de -X main.version) e aparece em
// GET /api/system/info.
var Version = "dev"

// Endereços de bind do `serve`: só a máquina local por padrão; a rede local exige --lan.
const (
	localhostBind = "127.0.0.1"
	lanBind       = "0.0.0.0"
)

// errLANWithoutPassword impede expor o FinFlow na rede sem senha.
var errLANWithoutPassword = errors.New("o modo rede local (--lan ou FINFLOW_LAN=1) exige uma senha: defina FINFLOW_PASSWORD " +
	"(no .env ou no ambiente) antes de iniciar — sem ela, qualquer aparelho conectado à sua rede veria e alteraria seus dados")

// lanFromEnv lê FINFLOW_LAN (1, true, yes, sim ou on).
func lanFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FINFLOW_LAN"))) {
	case "1", "true", "yes", "sim", "on":
		return true
	}
	return false
}

// resolveBindHost escolhe o endereço de escuta: --lan → 0.0.0.0; senão --host, depois API_HOST,
// e por fim 127.0.0.1 (só esta máquina).
func resolveBindHost(flagHost, envHost string, lan bool) string {
	switch {
	case lan:
		return lanBind
	case strings.TrimSpace(flagHost) != "":
		return strings.TrimSpace(flagHost)
	case strings.TrimSpace(envHost) != "":
		return strings.TrimSpace(envHost)
	}
	return localhostBind
}

// checkLAN recusa o modo rede local sem FINFLOW_PASSWORD.
func checkLAN(lan bool, password string) error {
	if lan && strings.TrimSpace(password) == "" {
		return errLANWithoutPassword
	}
	return nil
}

// displayHost é o host mostrado no terminal para abrir no navegador desta máquina.
func displayHost(bind string) string {
	switch bind {
	case "", localhostBind, lanBind, "::", "[::]":
		return "localhost"
	}
	return bind
}
