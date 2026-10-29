package ddmcp

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunHTTP starts the MCP server over HTTP (Streamable HTTP transport) on addr instead of stdio,
// for MCP clients that work over HTTP and for remote access. It serves until ctx is cancelled.
//
// Note about net/http: here it is used as the TRANSPORT for the MCP server itself, not for calls
// to the DiscoDrive API — those still go exclusively through internal/ddclient.
//
// Security: the MCP layer has no authentication of its own beyond the optional bearer token.
// Anyone who can reach the port gets access to your DiscoDrive (with device-token privileges).
// Therefore: an address without a host (":8080") is bound to 127.0.0.1; binding to a network
// address (0.0.0.0/external) REQUIRES a token — otherwise startup is refused. The token (if set)
// is required in the "Authorization: Bearer <token>" header. Be especially careful with --allow-changes.
func RunHTTP(ctx context.Context, allowChanges bool, profile, addr, token string) error {
	addr = normalizeAddr(addr)
	exposed := networkExposed(addr)
	if exposed && token == "" {
		return errors.New(i18n.T("mcp.http.err.refuse_no_token", i18n.M{"Addr": addr}))
	}

	p, err := resolveCreds(profile, os.Getenv, configLookup)
	if err != nil {
		return err
	}
	mcpServer := buildServer(newClient(p), allowChanges)

	var handler http.Handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return mcpServer }, nil)
	if token != "" {
		handler = bearerAuth(token, handler)
	}
	httpSrv := &http.Server{Addr: addr, Handler: handler}

	switch {
	case exposed:
		fmt.Fprint(os.Stderr, i18n.T("mcp.http.msg.exposed", i18n.M{"Addr": addr}))
	case token != "":
		fmt.Fprint(os.Stderr, i18n.T("mcp.http.msg.local_auth", i18n.M{"Addr": addr}))
	default:
		fmt.Fprint(os.Stderr, i18n.T("mcp.http.msg.local", i18n.M{"Addr": addr}))
	}

	go func() {
		<-ctx.Done()
		_ = httpSrv.Close()
	}()
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// normalizeAddr applies a safe default: an address without a host (":8080") is bound to the
// loopback 127.0.0.1 rather than all interfaces. Other addresses (and unparsable ones) are returned as is.
func normalizeAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "localhost" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return addr
}

// bearerAuth requires the "Authorization: Bearer <token>" header (compared in constant time).
// Without it, or with the wrong token, it returns 401. It wraps the MCP handler in HTTP mode.
func bearerAuth(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// networkExposed permits unauthenticated access only on a literal loopback IP.
// Hostnames are not trusted: their resolution may change between this check and bind.
func networkExposed(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// could not parse host:port — treat as potentially exposed, so we warn.
		return true
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}
