//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// showErrorDialog mostra uma caixa de erro nativa (MessageBoxW), sem cgo. Devolve false se não
// foi possível exibi-la, para o chamador cair no "Pressione Enter" do console.
func showErrorDialog(title, text string) bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	if proc.Find() != nil {
		return false
	}
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return false
	}
	m, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return false
	}
	const mbOK, mbIconError, mbSetForeground = 0x0, 0x10, 0x10000
	ret, _, _ := proc.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbOK|mbIconError|mbSetForeground)
	return ret != 0
}
