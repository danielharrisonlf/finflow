//go:build !(js && wasm)

package ai

import "github.com/anthropics/anthropic-sdk-go/v2/option"

// platformOptions são as opções do SDK próprias da plataforma; no desktop, nenhuma.
func platformOptions() []option.RequestOption { return nil }

// platformConnectHint complementa o erro de conexão com um servidor local; no desktop, nada.
func platformConnectHint() string { return "" }
