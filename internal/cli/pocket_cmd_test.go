package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// savedMuxCLI stubs the saved endpoints; queries receives every list query string.
func savedMuxCLI(t *testing.T, queries chan string) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/saved", func(w http.ResponseWriter, r *http.Request) {
		if queries != nil {
			queries <- r.URL.RawQuery
		}
		_, _ = w.Write([]byte(`[{"id":"s1","kind":"article","status":"done",
			"url":"https://example.org/a","title":"Long read","has_content":true}]`))
	})
	mux.HandleFunc("GET /me/saved/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		// deliberately JSON-looking: a stray json.Decoder in the path would eat it
		_, _ = w.Write([]byte("# Long read\n{\"not\":\"json\"}\n"))
	})
	return mux
}

func TestPocketListShowsStatusAndTitle(t *testing.T) {
	queries := make(chan string, 1)
	srv := httptest.NewServer(savedMuxCLI(t, queries))
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, errb, code := runCLI(t, "pocket", "list", "--status", "done")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	if !strings.Contains(out, "s1") || !strings.Contains(out, "done") || !strings.Contains(out, "Long read") {
		t.Fatalf("out:\n%s", out)
	}
	// downloads must never leak into the pocket listing
	if q := <-queries; !strings.Contains(q, "kind=article") || !strings.Contains(q, "status=done") {
		t.Fatalf("query = %q", q)
	}
}

func TestPocketReadWritesMarkdownVerbatim(t *testing.T) {
	srv := httptest.NewServer(savedMuxCLI(t, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	dest := filepath.Join(t.TempDir(), "article.md")
	if _, errb, code := runCLI(t, "pocket", "read", "s1", "--out", dest); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "# Long read\n{\"not\":\"json\"}\n" {
		t.Fatalf("the markdown must be copied byte for byte, got %q", b)
	}
}

func TestPocketReadDefaultsToStdout(t *testing.T) {
	srv := httptest.NewServer(savedMuxCLI(t, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, errb, code := runCLI(t, "pocket", "read", "s1")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	if !strings.HasPrefix(out, "# Long read") {
		t.Fatalf("stdout:\n%s", out)
	}
}

func TestPocketListJSONEmptyIsArray(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/saved", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, _, _ := runCLI(t, "--json", "pocket", "list")
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty --json = %q, want []", out)
	}
}
