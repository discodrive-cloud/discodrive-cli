package ddclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// helper: a test server that issues a JWT for a known device-token.
func tokenMux() (*http.ServeMux, *int32) {
	var issued int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DeviceToken string `json:"device_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DeviceToken != "kfd_test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		atomic.AddInt32(&issued, 1)
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "jwt-123"})
	})
	return mux, &issued
}

func TestClientCachesJWT(t *testing.T) {
	mux, issued := tokenMux()
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jwt-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "kfd_test")
	for i := 0; i < 3; i++ {
		resp, err := c.do(context.Background(), http.MethodGet, "/me")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if *issued != 1 {
		t.Fatalf("JWT issued %d times, want 1 (cached)", *issued)
	}
}

// WithInsecureTLS: a self-signed (httptest TLS) server is rejected by the default client
// and accepted with the option.
func TestClientInsecureTLS(t *testing.T) {
	mux, _ := tokenMux()
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	if resp, err := New(srv.URL, "kfd_test").do(context.Background(), http.MethodGet, "/me"); err == nil {
		resp.Body.Close()
		t.Fatal("strict client must reject a self-signed certificate")
	}
	resp, err := New(srv.URL, "kfd_test", WithInsecureTLS()).do(context.Background(), http.MethodGet, "/me")
	if err != nil {
		t.Fatalf("insecure client must accept a self-signed certificate: %v", err)
	}
	resp.Body.Close()
}

func TestClientRefreshesOn401(t *testing.T) {
	mux, issued := tokenMux()
	var first int32 = 1
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		if atomic.CompareAndSwapInt32(&first, 1, 0) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "kfd_test")
	resp, err := c.do(context.Background(), http.MethodGet, "/me")
	if err != nil {
		t.Fatalf("expected success after refresh: %v", err)
	}
	resp.Body.Close()
	if *issued < 2 {
		t.Fatalf("JWT should be reissued on 401, issued %d", *issued)
	}
}

func TestCleartext(t *testing.T) {
	for u, want := range map[string]bool{
		"http://drive.example":     true,
		"http://192.168.1.10:8080": true,
		"HTTP://drive.example":     true,
		"https://drive.example":    false,
		"http://127.0.0.1:8080":    false,
		"http://[::1]:8080":        false,
		"http://localhost:8080":    false,
		"http://localhost.evil":    true,
	} {
		if got := Cleartext(u); got != want {
			t.Errorf("Cleartext(%q) = %v, want %v", u, got, want)
		}
	}
}
