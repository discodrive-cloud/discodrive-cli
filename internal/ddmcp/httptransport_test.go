package ddmcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Integration test: the HTTP transport actually serves MCP via the official SDK client
// (a full handshake), and safe-by-default holds over HTTP too.
func TestHTTPTransportServesToolsAndFilters(t *testing.T) {
	c := ddclient.New("http://example.invalid", "tok") // the API is not hit for tools/list
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return buildServer(c, false) }, nil))
	defer httpServer.Close()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	res, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	got := map[string]bool{}
	for _, tl := range res.Tools {
		got[tl.Name] = true
	}
	if !got["dd_music_list"] || !got["dd_files_list"] {
		t.Fatalf("expected read tools over HTTP, got: %v", res.Tools)
	}
	if got["dd_music_radio_delete"] {
		t.Fatal("mutating tool is visible over HTTP without --allow-changes")
	}
}

// normalizeAddr must turn an address without a host (":8080") into the loopback 127.0.0.1 —
// a safe default instead of binding to all interfaces. Explicit addresses are left untouched.
func TestNormalizeAddr(t *testing.T) {
	cases := map[string]string{
		":8080":          "127.0.0.1:8080",
		"127.0.0.1:8080": "127.0.0.1:8080",
		"localhost:8080": "127.0.0.1:8080",
		"0.0.0.0:8080":   "0.0.0.0:8080",
		"[::]:8080":      "[::]:8080",
		"garbage":        "garbage", // could not parse — return as is
	}
	for in, want := range cases {
		if got := normalizeAddr(in); got != want {
			t.Errorf("normalizeAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

// RunHTTP must not bring up an unauthenticated server on a network address:
// without a token, it refuses before any binding.
func TestRunHTTPRefusesNetworkWithoutToken(t *testing.T) {
	err := RunHTTP(context.Background(), false, "no-such-profile", "0.0.0.0:0", "")
	if err == nil {
		t.Fatal("expected a refusal for a network address without a token")
	}
	// The refusal must trigger before resolveCreds (not about missing credentials).
	if strings.Contains(err.Error(), "credential") {
		t.Fatalf("the refusal must be about a missing token, not about credentials: %v", err)
	}
}

// bearerAuth: without a valid Authorization header — 401, with it — pass through.
func TestBearerAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	})
	h := bearerAuth("s3cret", next)

	do := func(authHeader string) int {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do(""); code != http.StatusUnauthorized {
		t.Errorf("expected 401 without the header, got %d", code)
	}
	if code := do("Bearer wrong"); code != http.StatusUnauthorized {
		t.Errorf("expected 401 with a wrong token, got %d", code)
	}
	if code := do("Bearer s3cret"); code != http.StatusOK {
		t.Errorf("expected 200 with the correct token, got %d", code)
	}
}

func TestNetworkExposed(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8080": false,
		"localhost:8080": true,
		"[::1]:8080":     false,
		":8080":          true, // all interfaces
		"0.0.0.0:8080":   true,
		"[::]:8080":      true,
		"garbage":        true, // could not parse — warn just in case
	}
	for addr, want := range cases {
		if got := networkExposed(addr); got != want {
			t.Errorf("networkExposed(%q) = %v, want %v", addr, got, want)
		}
	}
}
