package cli

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

func jwtWithDid(did string) string {
	seg := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	return seg(`{"alg":"none"}`) + "." + seg(`{"sub":"u1","did":"`+did+`"}`) + "." + seg("sig")
}

func TestLogoutRevokesAndForgets(t *testing.T) {
	gotDelete := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"` + jwtWithDid("dev-9") + `"}`))
	})
	mux.HandleFunc("DELETE /devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotDelete <- r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"logout"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	// the device was revoked on the server
	select {
	case id := <-gotDelete:
		if id != "dev-9" {
			t.Fatalf("revoked %q, expected dev-9", id)
		}
	default:
		t.Fatal("server did not receive DELETE /devices/{id}")
	}
	// the profile was forgotten locally
	p, _ := ddclient.ConfigPath()
	cfg, _ := ddclient.Load(p)
	if _, ok := cfg.Lookup("default"); ok {
		t.Fatal("the default profile must be removed from the config")
	}
}

func TestLogoutLocalOnlySkipsServer(t *testing.T) {
	// ANY contact with the server is a failure: with --local-only the network must not be touched.
	// (Revocation starts with POST /auth/device/token, so a single DELETE handler isn't enough —
	// we catch all requests.)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("with --local-only there must be no requests to the server, but got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"logout", "--local-only"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	p, _ := ddclient.ConfigPath()
	cfg, _ := ddclient.Load(p)
	if _, ok := cfg.Lookup("default"); ok {
		t.Fatal("the profile must be removed with --local-only as well")
	}
}
