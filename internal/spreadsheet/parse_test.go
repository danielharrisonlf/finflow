package spreadsheet

import "testing"

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"R$ 1.234,56", 123456, true},
		{"1.234,56", 123456, true},
		{"1,234.56", 123456, true},
		{"-10", -1000, true},
		{"10", 1000, true},
		{"(10,00)", -1000, true},
		{"10 D", -1000, true},
		{"10 C", 1000, true},
		{"1.500,00 D", -150000, true},
		{"10D", -1000, true},
		{"R$ -45,90", -4590, true},
		{"-R$ 45,90", -4590, true},
		{"45,90-", -4590, true},
		{"+3.200,00", 320000, true},
		{"0,5", 50, true},
		{"12.5", 1250, true},
		{"1.234", 123400, true},
		{"1,234,567.89", 123456789, true},
		{"1.234.567", 123456700, true},
		{"R$ 99,99", 9999, true},
		{"0", 0, true},
		{"", 0, false},
		{"abc", 0, false},
		{"R$", 0, false},
		{"--10", 0, false},
		{"10,00 X", 0, false},
	}
	for _, tc := range cases {
		got, ok := ParseAmount(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseAmount(%q) = %d, %v; want %d, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseCellAmountUsesDotDecimalForRawNumbers(t *testing.T) {
	cases := map[string]int64{"1234.5": 123450, "12.3456": 1235, "-7": -700, "1E3": 100000, "R$ 1.234,56": 123456}
	for in, want := range cases {
		got, ok := parseCellAmount(in)
		if !ok || got != want {
			t.Errorf("parseCellAmount(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in     string
		serial bool
		want   string
		ok     bool
	}{
		{"05/03/2026", false, "2026-03-05", true},
		{"5/3/2026", false, "2026-03-05", true},
		{"05/03/26", false, "2026-03-05", true},
		{"31/12/99", false, "1999-12-31", true},
		{"05-03-2026", false, "2026-03-05", true},
		{"05.03.2026", false, "2026-03-05", true},
		{"2026-03-05", false, "2026-03-05", true},
		{"2026-03-05T10:00:00", false, "2026-03-05", true},
		{"20260305", false, "2026-03-05", true},
		{"03/25/2026", false, "2026-03-25", true}, // segundo número > 12 → mm/dd
		{"05/03/2026 14:30", false, "2026-03-05", true},
		{"46086", true, "2026-03-05", true},
		{"46086.75", true, "2026-03-05", true},
		{"46086", false, "", false},
		{"31/02/2026", false, "", false},
		{"29/02/2024", false, "2024-02-29", true},
		{"29/02/2026", false, "", false},
		{"ontem", false, "", false},
		{"", true, "", false},
		{"12", true, "", false},
	}
	for _, tc := range cases {
		got, ok := ParseDate(tc.in, tc.serial)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseDate(%q, %v) = %q, %v; want %q, %v", tc.in, tc.serial, got, ok, tc.want, tc.ok)
		}
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize("  Alimentação   e  CAFÉ "); got != "alimentacao e cafe" {
		t.Fatalf("Normalize = %q", got)
	}
}

func TestDuplicateKeyIgnoresSignCaseAndAccents(t *testing.T) {
	a := DuplicateKey("2026-01-02", -1000, "EXPENSE", "Padaria São João")
	b := DuplicateKey("2026-01-02", 1000, "EXPENSE", "padaria  sao joao")
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
}
