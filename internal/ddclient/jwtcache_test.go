package ddclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// expJWT builds an unsigned JWT with the given exp claim (the client never verifies signatures).
func expJWT(exp time.Time) string {
	seg := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return seg(map[string]string{"alg": "none"}) + "." +
		seg(map[string]int64{"exp": exp.Unix()}) + ".sig"
}

// cacheMux serves the token exchange (counting issues) and an authenticated /me.
// Each issued JWT is unique so tests can tell which one a request carries.
func cacheMux(t *testing.T) (*http.ServeMux, *int32) {
	t.Helper()
	var issued int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&issued, 1)
		tok := expJWT(time.Now().Add(time.Hour)) + fmt.Sprintf("%d", n)
		_ = json.NewEncoder(w).Encode(map[string]string{"token": tok})
	})
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	return mux, &issued
}

// Two consecutive processes (two clients) against the same server: the second must
// reuse the cached JWT and not hit the token endpoint again.
func TestJWTCacheSharedBetweenClients(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	mux, issued := cacheMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for i := 0; i < 3; i++ {
		c := New(srv.URL, "kfd", WithJWTCache())
		resp, err := c.do(context.Background(), http.MethodGet, "/me")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if *issued != 1 {
		t.Fatalf("JWT issued %d times for 3 clients, want 1 (disk cache)", *issued)
	}
}

// An expired cached JWT must be ignored and re-exchanged.
func TestJWTCacheExpiredIgnored(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	mux, issued := cacheMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p, err := jwtCachePath(srv.URL, "kfd")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(expJWT(time.Now().Add(-time.Minute))), 0o600); err != nil {
		t.Fatal(err)
	}

	c := New(srv.URL, "kfd", WithJWTCache())
	resp, err := c.do(context.Background(), http.MethodGet, "/me")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if *issued != 1 {
		t.Fatalf("expired cache must force a re-exchange, issued=%d", *issued)
	}
}

// A cached JWT the server no longer accepts (401) must be dropped and replaced.
func TestJWTCacheDroppedOn401(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	stale := expJWT(time.Now().Add(time.Hour)) + "stale"
	mux, issued := cacheMux(t)
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer "+stale {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `[]`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p, _ := jwtCachePath(srv.URL, "kfd")
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	if err := os.WriteFile(p, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	c := New(srv.URL, "kfd", WithJWTCache())
	resp, err := c.do(context.Background(), http.MethodGet, "/files")
	if err != nil {
		t.Fatalf("request must succeed after the 401 retry: %v", err)
	}
	resp.Body.Close()
	if *issued != 1 {
		t.Fatalf("expected exactly one exchange after dropping the stale cache, issued=%d", *issued)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) == stale {
		t.Fatalf("cache must hold the fresh JWT, err=%v", err)
	}
}

// A bare client (unit-test mode) must not touch the disk; and tokens without a
// usable exp claim must not be written even with the cache enabled.
func TestJWTCacheOptInAndExpOnly(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "not-a-jwt"})
	})
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, c := range []*Client{New(srv.URL, "kfd"), New(srv.URL, "kfd", WithJWTCache())} {
		resp, err := c.do(context.Background(), http.MethodGet, "/me")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "discodrive")); !os.IsNotExist(err) {
		t.Fatalf("no cache entries expected (bare client / token without exp), stat err=%v", err)
	}
}
