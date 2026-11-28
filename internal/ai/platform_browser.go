//go:build js && wasm

package ai

import "github.com/anthropics/anthropic-sdk-go/v2/option"

// platformOptions: no runtime do navegador o SDK chama a API pelo fetch da página, e a API do Claude
// só aceita CORS com este cabeçalho (a chave é do próprio usuário e fica no banco local).
func platformOptions() []option.RequestOption {
	return []option.RequestOption{option.WithHeader("anthropic-dangerous-direct-browser-access", "true")}
}
