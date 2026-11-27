package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"finflow/internal/domain"
)

// fakeResolver resolve nomes .test sem DNS real.
type fakeResolver map[string]string

func (f fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	ip, ok := f[host]
	if !ok {
		return nil, fmt.Errorf("host desconhecido: %s", host)
	}
	return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
}

// loopbackOnly libera só 127.0.0.1 (o httptest) e recusa todo o resto, inclusive públicos.
func loopbackOnly(ip net.IP) bool { return ip.Equal(net.IPv4(127, 0, 0, 1)) }

func previewSvcs(p *LinkPreviewer) *Services {
	return &Services{LinkPreview: p}
}

func TestIsPublicIP(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8": true, "2001:4860:4860::8888": true,
		"127.0.0.1": false, "::1": false, "10.1.2.3": false, "172.16.0.1": false, "192.168.0.10": false,
		"169.254.169.254": false, "fe80::1": false, "0.0.0.0": false, "::": false, "100.64.0.1": false,
		"fc00::1": false, "224.0.0.1": false, "255.255.255.255": false, "::ffff:127.0.0.1": false,
	}
	for raw, want := range cases {
		if got := IsPublicIP(net.ParseIP(raw)); got != want {
			t.Errorf("IsPublicIP(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestPreviewLinkFetchesMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/r1":
			http.Redirect(w, r, "/r2", http.StatusFound)
		case "/r2":
			http.Redirect(w, r, "/produto", http.StatusFound)
		case "/produto":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><meta property="og:title" content="Fone Bluetooth"><meta property="og:price:amount" content="199.90"><meta property="og:image" content="/f.jpg"></head></html>`))
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"title":"x"}`))
		case "/big":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(strings.Repeat("a", linkPreviewMaxBytes+10)))
			_, _ = w.Write([]byte(`<title>Depois do limite</title>`))
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/evil":
			http.Redirect(w, r, "http://evil.test/", http.StatusFound)
		case "/slow":
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	shop := "http://shop.test:" + port

	p := &LinkPreviewer{Resolver: fakeResolver{"shop.test": "127.0.0.1", "evil.test": "10.0.0.5"}, AllowIP: loopbackOnly, Timeout: time.Second}
	svcs := previewSvcs(p)
	ctx := context.Background()

	got, err := svcs.PreviewLink(ctx, domain.LinkPreviewInput{URL: shop + "/r1"})
	if err != nil || got.Title == nil || *got.Title != "Fone Bluetooth" || got.PriceCents == nil || *got.PriceCents != 19990 ||
		got.ImageURL == nil || *got.ImageURL != shop+"/f.jpg" {
		t.Fatalf("prévia com 2 redirecionamentos: %+v %v", got, err)
	}

	empty := []string{"/json", "/big", "/loop", "/slow", "/404"}
	for _, path := range empty {
		got, err := svcs.PreviewLink(ctx, domain.LinkPreviewInput{URL: shop + path})
		if err != nil || got.Title != nil || got.PriceCents != nil || got.ImageURL != nil {
			t.Errorf("%s: esperava prévia vazia sem erro, veio %+v %v", path, got, err)
		}
	}

	// Redirecionamento para host privado é recusado na revalidação (sem conectar).
	_, err = svcs.PreviewLink(ctx, domain.LinkPreviewInput{URL: shop + "/evil"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestPreviewLinkBlocksPrivateTargets(t *testing.T) {
	// Política padrão: nada de loopback, privados, link-local ou não especificados.
	p := &LinkPreviewer{Resolver: fakeResolver{"intranet.test": "192.168.1.10", "meta.test": "169.254.169.254", "dual.test": "127.0.0.1"}}
	svcs := previewSvcs(p)
	ctx := context.Background()
	for _, raw := range []string{
		"http://127.0.0.1/", "http://[::1]:8080/", "http://0.0.0.0/", "http://10.0.0.1/admin",
		"http://intranet.test/", "https://meta.test/latest/meta-data", "http://dual.test/",
	} {
		_, err := svcs.PreviewLink(ctx, domain.LinkPreviewInput{URL: raw})
		assertStatus(t, err, http.StatusBadRequest)
	}
	for _, raw := range []string{"ftp://example.com/x", "file:///etc/passwd", "javascript:alert(1)", "", "http://"} {
		_, err := svcs.PreviewLink(ctx, domain.LinkPreviewInput{URL: raw})
		assertValidation(t, err, "url")
	}
}

func TestPreviewLinkRechecksDialedIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<title>não deveria ler</title>`))
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	// A resolução passa (o resolver "mente" um IP liberado só na validação), mas o IP efetivamente
	// conectado é conferido de novo e recusado.
	calls := 0
	p := &LinkPreviewer{
		Resolver: fakeResolver{"rebind.test": "127.0.0.1"},
		AllowIP: func(ip net.IP) bool {
			calls++
			return calls <= 2 // validação da URL + resolução no dial; o IP conectado é recusado
		},
		Timeout: time.Second,
	}
	_, err := previewSvcs(p).PreviewLink(context.Background(), domain.LinkPreviewInput{URL: "http://rebind.test:" + port + "/"})
	assertStatus(t, err, http.StatusBadRequest)
}
