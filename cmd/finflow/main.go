package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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

// runDesktop é o modo "duplo clique": sem argumentos, sobe o `serve` e abre o navegador.
// Se já houver um FinFlow respondendo na porta, só abre o navegador.
func runDesktop() int {
	url := fmt.Sprintf("http://localhost:%d", config.Load().Port)
	openBrowser := strings.TrimSpace(os.Getenv("FINFLOW_NO_BROWSER")) == ""

	if healthy(url) {
		fmt.Printf("FinFlow já está rodando em %s\n", url)
		if openBrowser {
			openURL(url)
		}
		return 0
	}

	fmt.Println("Iniciando o FinFlow... Para encerrar, feche esta janela ou pressione Ctrl+C.")
	if openBrowser {
		go func() {
			for range 60 {
				time.Sleep(250 * time.Millisecond)
				if healthy(url) {
					openURL(url)
					return
				}
			}
		}()
	}

	code := cli.Run([]string{"serve"}, os.Stdout, os.Stderr)
	if code != 0 && runtime.GOOS == "windows" {
		// No duplo clique a janela fecharia antes de a mensagem de erro ser lida.
		fmt.Print("\nPressione Enter para fechar...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	return code
}

func healthy(base string) bool {
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(base + "/api/health")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func openURL(url string) {
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
		fmt.Fprintf(os.Stderr, "Abra %s no navegador (não foi possível abrir automaticamente: %v)\n", url, err)
		return
	}
	go func() { _ = cmd.Wait() }()
}
