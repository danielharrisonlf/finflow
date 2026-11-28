//go:build js && wasm

package ai

import (
	"syscall/js"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// platformOptions: no runtime do navegador o SDK chama a API pelo fetch da página, e a API do Claude
// só aceita CORS com este cabeçalho (a chave é do próprio usuário e fica no banco local).
func platformOptions() []option.RequestOption {
	return []option.RequestOption{option.WithHeader("anthropic-dangerous-direct-browser-access", "true")}
}

// platformConnectHint: no navegador as chamadas ao servidor local passam pelo fetch da página, e uma
// recusa de CORS chega como falha de conexão. Explica como liberar a origem desta página.
func platformConnectHint() string {
	origin := "*"
	if loc := js.Global().Get("location"); loc.Truthy() {
		if o := loc.Get("origin"); o.Type() == js.TypeString && o.String() != "" && o.String() != "null" {
			origin = o.String()
		}
	}
	return " No navegador, o servidor local também precisa permitir esta página (CORS): no Ollama, defina OLLAMA_ORIGINS=" +
		origin + " e reinicie-o; no LM Studio, ative “Enable CORS” no servidor."
}
