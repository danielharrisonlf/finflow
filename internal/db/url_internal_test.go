package db

import (
	"net/url"
	"testing"
)

func TestNormalizePostgresURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want map[string]string // parâmetros esperados na query (valor "" = ausente)
	}{
		{
			name: "schema do Prisma vira search_path e timezone é forçado",
			in:   "postgres://u:p@localhost:5432/app?schema=public",
			want: map[string]string{"search_path": "public", "schema": "", "timezone": "UTC"},
		},
		{
			name: "search_path explícito prevalece sobre schema",
			in:   "postgres://u:p@localhost:5432/app?schema=public&search_path=outro",
			want: map[string]string{"search_path": "outro", "schema": "", "timezone": "UTC"},
		},
		{
			name: "timezone informado é preservado",
			in:   "postgresql://u:p@localhost/app?sslmode=disable&TimeZone=America/Sao_Paulo",
			want: map[string]string{"TimeZone": "America/Sao_Paulo", "timezone": "", "sslmode": "disable"},
		},
		{
			name: "sem parâmetros",
			in:   "postgres://u:p@localhost/app",
			want: map[string]string{"timezone": "UTC", "search_path": ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizePostgresURL(tc.in)
			if err != nil {
				t.Fatalf("normalizePostgresURL: %v", err)
			}
			u, err := url.Parse(got)
			if err != nil {
				t.Fatalf("URL resultante inválida %q: %v", got, err)
			}
			q := u.Query()
			for k, v := range tc.want {
				if v == "" {
					if q.Has(k) {
						t.Errorf("%q não deveria estar presente em %q", k, got)
					}
					continue
				}
				if q.Get(k) != v {
					t.Errorf("%s = %q, esperado %q (URL %q)", k, q.Get(k), v, got)
				}
			}
			if u.User.Username() != "u" || u.Host == "" || u.Path != "/app" {
				t.Errorf("credenciais/host/caminho alterados: %q", got)
			}
		})
	}
}
