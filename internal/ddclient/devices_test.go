package ddclient

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeJWT assembles an unsigned JWT with the given payload (for extracting the "did" claim).
func fakeJWT(payloadJSON string) string {
	seg := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	return seg(`{"alg":"none","typ":"JWT"}`) + "." + seg(payloadJSON) + "." + seg("sig")
}

func TestUnpairRevokesCurrentDevice(t *testing.T) {
	gotID := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"` + fakeJWT(`{"sub":"u1","did":"dev-123"}`) + `"}`))
	})
	mux.HandleFunc("DELETE /devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotID <- r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "kfd")
	if err := c.Unpair(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-gotID:
		if id != "dev-123" {
			t.Fatalf("revoked device %q, expected dev-123", id)
		}
	default:
		t.Fatal("server did not receive DELETE /devices/{id}")
	}
}

func TestUnpairNoDeviceIDInToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"` + fakeJWT(`{"sub":"u1"}`) + `"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "kfd")
	if err := c.Unpair(context.Background()); err == nil {
		t.Fatal("expected an error when the token lacks did")
	}
}

// Token is not a JWT at all (no dots / broken base64) — expect an error, not a panic.
func TestUnpairMalformedToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"not-a-jwt"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "kfd")
	if err := c.Unpair(context.Background()); err == nil {
		t.Fatal("expected an error on a non-JWT token")
	}
}
