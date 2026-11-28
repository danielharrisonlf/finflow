//go:build js && wasm

package service

// linkPreviewDisabled: no navegador o CORS bloqueia páginas de terceiros, então a prévia devolve
// todos os campos nulos sem fazer requisição.
const linkPreviewDisabled = true
