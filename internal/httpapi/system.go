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
	resp, ok := mcp.NewServer(&client.LocalBackend{Svcs: s.svcs}).HandleJSONRPC(r.Context(), body)
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
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := SystemInfo{
		Version:      s.opts.Version,
		AuthRequired: s.svcs.Auth.IsRequired(),
		LANEnabled:   s.opts.LAN,
		LANURLs:      []string{},
		MCPHTTPPath:  MCPHTTPPath,
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	if d := s.svcs.Store.DB; d != nil {
		info.DBDialect = string(d.Dialect)
		if d.Dialect == db.DialectSQLite && d.Target != "" {
			target := d.Target
			info.DataPath = &target
		}
	}
	if s.opts.LAN {
		info.LANURLs = LANURLs(s.opts.Port)
	}
	if st, err := newAIService(s.svcs).Status(r.Context()); err == nil {
		info.AIConfigured = st.Configured
	}
	writeJSON(w, http.StatusOK, info)
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
