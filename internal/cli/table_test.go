package cli

import (
	"strings"
	"testing"
)

func TestTableAndProgressBar(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if w := VisibleWidth("\x1b[32mOlá\x1b[0m"); w != 3 {
		t.Fatalf("VisibleWidth = %d, want 3", w)
	}
	out := RenderTable(
		[]Column{{Header: "Conta"}, {Header: "Saldo", Align: AlignRight}},
		[][]string{{"Nubank", "R$ 10,00"}},
	)
	if !strings.Contains(out, "Conta") || !strings.Contains(out, "Nubank") || !strings.Contains(out, "R$ 10,00") {
		t.Fatalf("RenderTable unexpected:\n%s", out)
	}
	bar := ProgressBar(50, 10)
	if VisibleWidth(bar) != 10 {
		t.Fatalf("ProgressBar width = %d, want 10", VisibleWidth(bar))
	}
}
