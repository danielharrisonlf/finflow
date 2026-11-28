package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"finflow/internal/cli"
	"finflow/internal/config"
)

// version é preenchida no build de release: -ldflags "-X main.version=v0.2.0".
var version = "dev"

func main() {
	config.LoadDotEnv()
	cli.Version = version
	args := os.Args[1:]

	if len(args) == 1 && (args[0] == "version" || args[0] == "--version" || args[0] == "-v") {
		fmt.Printf("finflow %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}
	if len(args) == 0 {
		os.Exit(runDesktop())
	}
	cli.Execute()
}

// portFallbacks é quantas portas seguintes à configurada o modo duplo clique tenta quando ela
// está ocupada por outro programa. A sequência é fixa para a URL ficar estável entre execuções.
const portFallbacks = 20

// runDesktop é o modo "duplo clique": sem argumentos, sobe o `serve` e abre o navegador.
// Se já houver um FinFlow respondendo, só abre o navegador. Se a porta estiver ocupada por
// outro programa, usa a próxima livre. Erros vão para um log e, no Windows, para uma caixa
// de diálogo — a janela do console pode fechar ou nem estar visível.
func runDesktop() int {
	logw, logPath := openDesktopLog()
	if logw != nil {
		defer logw.Close()
	}
	var errBuf bytes.Buffer
	stdout := io.MultiWriter(os.Stdout, nonNil(logw))
	stderr := io.MultiWriter(os.Stderr, nonNil(logw), &errBuf)

	openBrowser := strings.TrimSpace(os.Getenv("FINFLOW_NO_BROWSER")) == ""
	base := config.Load().Port

	port, running, err := choosePort(base)
	if err != nil {
		return desktopFail(stderr, &errBuf, logPath, err.Error())
	}
	url := fmt.Sprintf("http://localhost:%d", port)
	if running {
		fmt.Fprintf(stdout, "FinFlow já está rodando em %s\n", url)
		if openBrowser {
			openURL(url, stderr)
		}
		return 0
	}
	if port != base {
		fmt.Fprintf(stdout, "A porta %d está em uso por outro programa; usando a %d.\n", base, port)
	}

	fmt.Fprintln(stdout, "Iniciando o FinFlow... Para encerrar, feche esta janela ou pressione Ctrl+C.")
	if openBrowser {
		go func() {
			for range 120 {
				time.Sleep(250 * time.Millisecond)
				if isFinFlow(url) {
					openURL(url, stderr)
					return
				}
			}
		}()
	}

	code := cli.Run([]string{"serve", "--port", strconv.Itoa(port)}, stdout, stderr)
	if code != 0 {
		return desktopFail(stderr, &errBuf, logPath, "")
	}
	return code
}

// choosePort devolve a porta a usar e se já existe um FinFlow nela. Uma porta conta como
// ocupada se algo aceita conexão em 127.0.0.1 (mesmo que seja um programa escutando em
// 0.0.0.0, que no Windows não impediria o bind e deixaria a URL ambígua) ou se o bind falha.
func choosePort(base int) (port int, running bool, err error) {
	for p := base; p <= base+portFallbacks && p <= 65535; p++ {
		url := fmt.Sprintf("http://localhost:%d", p)
		if isFinFlow(url) {
			return p, true, nil
		}
		if portInUse(p) {
			continue
		}
		return p, false, nil
	}
	return 0, false, fmt.Errorf("as portas %d a %d estão ocupadas por outros programas; defina API_PORT com uma porta livre", base, base+portFallbacks)
}

func portInUse(port int) bool {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	if conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond); err == nil {
		_ = conn.Close()
		return true
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

// isFinFlow confere se quem responde na URL é mesmo o FinFlow (e não outro servidor local
// que por acaso tenha um /api/health): /api/system/info devolve a versão e o dialeto do banco.
func isFinFlow(base string) bool {
	client := http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Get(base + "/api/system/info")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var info struct {
		Version   string `json:"version"`
		DBDialect string `json:"dbDialect"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&info); err != nil {
		return false
	}
	return info.Version != "" && info.DBDialect != ""
}

// desktopFail registra o erro e o mostra de um jeito que a pessoa consiga ler.
func desktopFail(stderr io.Writer, errBuf *bytes.Buffer, logPath, msg string) int {
	if msg != "" {
		fmt.Fprintln(stderr, "erro:", msg)
	}
	detail := strings.TrimSpace(errBuf.String())
	if len(detail) > 1500 {
		detail = "…" + detail[len(detail)-1500:]
	}
	text := "O FinFlow não conseguiu iniciar.\n\n" + detail
	if logPath != "" {
		text += "\n\nDetalhes em: " + logPath
	}
	if !showErrorDialog("FinFlow", text) && runtime.GOOS == "windows" {
		fmt.Print("\nPressione Enter para fechar...")
		_, _ = fmt.Fscanln(os.Stdin)
	}
	return 1
}

// openDesktopLog abre (sobrescrevendo) o log do modo duplo clique na pasta de configuração.
func openDesktopLog() (*os.File, string) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, ""
	}
	dir = filepath.Join(dir, "finflow")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, ""
	}
	path := filepath.Join(dir, "finflow-desktop.log")
	f, err := os.Create(path)
	if err != nil {
		return nil, ""
	}
	fmt.Fprintf(f, "FinFlow %s (%s/%s) — %s\n", version, runtime.GOOS, runtime.GOARCH, time.Now().Format(time.RFC3339))
	return f, path
}

func nonNil(f *os.File) io.Writer {
	if f == nil {
		return io.Discard
	}
	return f
}

func openURL(url string, stderr io.Writer) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "Abra %s no navegador (não foi possível abrir automaticamente: %v)\n", url, err)
		return
	}
	go func() { _ = cmd.Wait() }()
}
