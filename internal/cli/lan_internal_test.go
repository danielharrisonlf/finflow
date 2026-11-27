package cli

import "testing"

func TestResolveBindHost(t *testing.T) {
	cases := []struct {
		name, flag, env string
		lan             bool
		want            string
	}{
		{"padrão só local", "", "", false, "127.0.0.1"},
		{"API_HOST", "", "192.168.0.2", false, "192.168.0.2"},
		{"--host vence API_HOST", "10.0.0.1", "192.168.0.2", false, "10.0.0.1"},
		{"--lan escuta em todas", "10.0.0.1", "192.168.0.2", true, "0.0.0.0"},
	}
	for _, c := range cases {
		if got := resolveBindHost(c.flag, c.env, c.lan); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if checkLAN(true, " ") == nil || checkLAN(true, "senha") != nil || checkLAN(false, "") != nil {
		t.Errorf("checkLAN errado")
	}
	for bind, want := range map[string]string{"127.0.0.1": "localhost", "0.0.0.0": "localhost", "": "localhost", "192.168.0.2": "192.168.0.2"} {
		if got := displayHost(bind); got != want {
			t.Errorf("displayHost(%q) = %q", bind, got)
		}
	}
}

func TestLANFromEnv(t *testing.T) {
	for v, want := range map[string]bool{"1": true, "true": true, "SIM": true, "0": false, "": false, "no": false} {
		t.Setenv("FINFLOW_LAN", v)
		if got := lanFromEnv(); got != want {
			t.Errorf("FINFLOW_LAN=%q → %v", v, got)
		}
	}
}
