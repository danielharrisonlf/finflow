package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"finflow/internal/client"
	"finflow/internal/db"
	"finflow/internal/mcp"
	"finflow/internal/service"
)

// MCPHTTPPath é a rota do MCP via Streamable HTTP.
const MCPHTTPPath = "/api/mcp"

// ───────────────────────── MCP via HTTP (v3 §13) ─────────────────────────

// handleMCP é o transporte Streamable HTTP do MCP (sem sessão e sem SSE): cada POST traz uma
// mensagem JSON-RPC (ou um lote) e recebe a resposta em application/json; só notificações → 202.
// O dispatcher é o mesmo do `finflow mcp`, rodando sobre os serviços deste servidor.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if !s.mcpOriginAllowed(r) {
		writeError(w, r, service.Forbidden("origem não permitida para o MCP"))
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, r, &service.AppError{Status: http.StatusRequestEntityTooLarge, Message: fmt.Sprintf("corpo da requisição excede %d bytes", tooLarge.Limit)})
			return
		}
		writeError(w, r, service.BadRequest("não foi possível ler o corpo da requisição"))
		return
	}
	resp, ok := mcp.NewServer(&client.LocalBackend{Svcs: s.svc(r)}).HandleJSONRPC(r.Context(), body)
	if !ok {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}

// handleMCPMethodNotAllowed: sem stream SSE nem sessões, GET/DELETE em /api/mcp respondem 405.
func (s *Server) handleMCPMethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", http.MethodPost)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
		"statusCode": http.StatusMethodNotAllowed,
		"message":    "use POST para o MCP via HTTP",
	})
}

// mcpOriginAllowed protege contra DNS rebinding (exigido pela especificação do transporte):
// clientes MCP não enviam Origin; navegadores só são aceitos na mesma origem ou em WEB_ORIGIN.
func (s *Server) mcpOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	if u, err := url.Parse(origin); err == nil && strings.EqualFold(u.Host, r.Host) {
		return true
	}
	for _, o := range strings.Split(s.opts.WebOrigin, ",") {
		if o = strings.TrimSpace(o); o != "" && (o == "*" || o == origin) {
			return true
		}
	}
	return false
}

// ───────────────────────── Sistema (v3 §13–§14) ─────────────────────────

// SystemInfo é a resposta de GET /api/system/info.
type SystemInfo struct {
	Version      string   `json:"version"`
	DBDialect    string   `json:"dbDialect"`
	DataPath     *string  `json:"dataPath"`
	AuthRequired bool     `json:"authRequired"`
	LANEnabled   bool     `json:"lanEnabled"`
	LANURLs      []string `json:"lanUrls"`
	AIConfigured bool     `json:"aiConfigured"`
	MCPHTTPPath  string   `json:"mcpHttpPath"`
	// Runtime: "server" (binário) ou "browser" (WebAssembly no navegador).
	Runtime string `json:"runtime"`
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := SystemInfo{
		Version:      s.opts.Version,
		AuthRequired: s.svcs.Auth.IsRequired(),
		LANEnabled:   s.opts.LAN,
		LANURLs:      []string{},
		MCPHTTPPath:  MCPHTTPPath,
		Runtime:      s.runtime(),
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	browser := info.Runtime == RuntimeBrowser
	if browser {
		info.LANEnabled = false
	}
	if d := s.svcs.Store.DB; d != nil {
		info.DBDialect = string(d.Dialect)
		if d.Dialect == db.DialectSQLite && d.Target != "" && !browser {
			target := d.Target
			info.DataPath = &target
		}
	}
	if info.LANEnabled {
		info.LANURLs = LANURLs(s.opts.Port)
	}
	// Sem ping ao servidor local: a configuração resolvida e o último resultado da detecção automática.
	if ok, err := s.ai.Configured(r.Context()); err == nil {
		info.AIConfigured = ok
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) runtime() string {
	if s.opts.Runtime == "" {
		return RuntimeServer
	}
	return s.opts.Runtime
}

// handleSystemSeed cria os dados de exemplo do tipo do espaço da requisição (pessoal, Casa ou
// Empresa); só num espaço vazio (409 caso contrário).
func (s *Server) handleSystemSeed(w http.ResponseWriter, r *http.Request) {
	sp, err := s.spaces.Get(requestSpace(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.svc(r).SeedDemo(r.Context(), sp.Kind); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"seeded": true, "space": sp.Slug, "kind": sp.Kind})
}

// LANURLs lista http://<ip>:<porta> para os IPv4 privados das interfaces ativas (modo rede local).
func LANURLs(port int) []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{}
	}
	var ips []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok {
				ips = append(ips, ipNet.IP)
			}
		}
	}
	return privateIPv4URLs(ips, port)
}

// privateIPv4URLs filtra IPv4 privados (10/8, 172.16/12, 192.168/16), sem repetir, em ordem.
func privateIPv4URLs(ips []net.IP, port int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, ip := range ips {
		v4 := ip.To4()
		if v4 == nil || !v4.IsPrivate() {
			continue
		}
		u := fmt.Sprintf("http://%s:%d", v4.String(), port)
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	sort.Strings(out)
	return out
}
