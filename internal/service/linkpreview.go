package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"finflow/internal/domain"
)

// ───────────────────────── Prévia de link de desejo (SSRF-safe) ─────────────────────────

const (
	linkPreviewTimeout      = 5 * time.Second
	linkPreviewMaxBytes     = 1 << 20 // 1 MiB
	linkPreviewMaxRedirects = 3
	linkPreviewUserAgent    = "Mozilla/5.0 (compatible; FinFlowLinkPreview/1.0)"
)

// errBlockedAddress indica destino proibido (loopback, rede privada, link-local...).
var errBlockedAddress = errors.New("endereço de destino não permitido")

// IPResolver resolve nomes; net.DefaultResolver satisfaz a interface (os testes injetam um falso).
type IPResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// LinkPreviewer busca páginas de produto com proteções contra SSRF: só http/https, resolve o host
// e recusa IPs de loopback, link-local, redes privadas, multicast e não especificados; o DialContext
// conecta só nos IPs validados e confere de novo o IP efetivamente conectado; até 3 redirecionamentos
// (revalidados), 5 s de timeout e no máximo 1 MiB lido. Proxies de ambiente são ignorados.
type LinkPreviewer struct {
	Resolver IPResolver
	// AllowIP decide se um IP pode ser contatado (padrão: IsPublicIP). Os testes podem liberar loopback.
	AllowIP func(net.IP) bool
	Timeout time.Duration
}

func NewLinkPreviewer() *LinkPreviewer {
	return &LinkPreviewer{Resolver: net.DefaultResolver, AllowIP: IsPublicIP, Timeout: linkPreviewTimeout}
}

// IsPublicIP recusa loopback, privados (RFC 1918 / fc00::/7), link-local, CGNAT, multicast e não especificados.
func IsPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 0, // 0.0.0.0/8
			v4[0] == 100 && v4[1]&0xc0 == 64,         // 100.64.0.0/10 (CGNAT)
			v4[0] == 192 && v4[1] == 0 && v4[2] == 0, // 192.0.0.0/24
			v4[0] == 198 && v4[1]&0xfe == 18,         // 198.18.0.0/15
			v4[0] >= 240:                             // reservado e broadcast
			return false
		}
	}
	return true
}

func (p *LinkPreviewer) allowed(ip net.IP) bool {
	allow := p.AllowIP
	if allow == nil {
		allow = IsPublicIP
	}
	return allow(ip)
}

// resolveAllowed resolve o host e exige que todos os IPs sejam permitidos.
func (p *LinkPreviewer) resolveAllowed(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if !p.allowed(ip) {
			return nil, errBlockedAddress
		}
		return []net.IP{ip}, nil
	}
	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("host %q sem endereço", host)
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		if !p.allowed(a.IP) {
			return nil, errBlockedAddress
		}
		ips = append(ips, a.IP)
	}
	return ips, nil
}

func (p *LinkPreviewer) validateURL(ctx context.Context, u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return errBlockedAddress
	}
	if u.Hostname() == "" {
		return errBlockedAddress
	}
	_, err := p.resolveAllowed(ctx, u.Hostname())
	return err
}

func (p *LinkPreviewer) client() *http.Client {
	dialer := &net.Dialer{Timeout: linkPreviewTimeout}
	transport := &http.Transport{
		Proxy: nil, // nunca usar HTTP(S)_PROXY: o destino validado seria outro
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := p.resolveAllowed(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err != nil {
					lastErr = err
					continue
				}
				// Confere o IP efetivamente conectado (defesa contra rebinding entre resolução e conexão).
				if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); !ok || !p.allowed(tcp.IP) {
					_ = conn.Close()
					return nil, errBlockedAddress
				}
				return conn, nil
			}
			return nil, lastErr
		},
		TLSHandshakeTimeout:   linkPreviewTimeout,
		ResponseHeaderTimeout: linkPreviewTimeout,
		MaxIdleConns:          1,
		DisableKeepAlives:     true,
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = linkPreviewTimeout
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > linkPreviewMaxRedirects {
				return fmt.Errorf("mais de %d redirecionamentos", linkPreviewMaxRedirects)
			}
			return p.validateURL(req.Context(), req.URL)
		},
	}
}

// Fetch baixa a página (até 1 MiB) e extrai a prévia. errBlockedAddress quando o destino é proibido.
func (p *LinkPreviewer) Fetch(ctx context.Context, rawURL string) (domain.LinkPreview, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return domain.LinkPreview{}, err
	}
	if err := p.validateURL(ctx, u); err != nil {
		return domain.LinkPreview{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return domain.LinkPreview{}, err
	}
	req.Header.Set("User-Agent", linkPreviewUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.5")
	resp, err := p.client().Do(req)
	if err != nil {
		if errors.Is(err, errBlockedAddress) {
			return domain.LinkPreview{}, errBlockedAddress
		}
		return domain.LinkPreview{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.LinkPreview{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		if mt, _, err := mime.ParseMediaType(ct); err == nil && !strings.Contains(mt, "html") && !strings.Contains(mt, "xml") {
			return domain.LinkPreview{}, nil
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, linkPreviewMaxBytes))
	if err != nil {
		return domain.LinkPreview{}, err
	}
	return domain.ParseLinkPreview(string(body), resp.Request.URL), nil
}

// PreviewLink lê og:title/preço/imagem do link de um desejo. URL inválida ou destino proibido → 400;
// falhas de rede, timeout ou página sem metadados devolvem todos os campos nulos.
func (s *Services) PreviewLink(ctx context.Context, in domain.LinkPreviewInput) (domain.LinkPreview, error) {
	if issues := domain.ValidateLinkPreview(in); len(issues) > 0 {
		return domain.LinkPreview{}, ValidationFailed(issues)
	}
	if linkPreviewDisabled {
		return domain.LinkPreview{}, nil
	}
	previewer := s.LinkPreview
	if previewer == nil {
		previewer = NewLinkPreviewer()
	}
	out, err := previewer.Fetch(ctx, in.URL)
	if errors.Is(err, errBlockedAddress) {
		return domain.LinkPreview{}, BadRequest("endereço não permitido: use um link público (http/https)")
	}
	if err != nil {
		return domain.LinkPreview{}, nil
	}
	return out, nil
}

// emptyToNil trata string vazia como ausente.
func emptyToNil(v *string) *string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	return v
}
