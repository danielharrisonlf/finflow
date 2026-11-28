//go:build !windows

package main

// showErrorDialog não tem equivalente portátil fora do Windows; o erro fica no terminal e no log.
func showErrorDialog(_, _ string) bool { return false }
