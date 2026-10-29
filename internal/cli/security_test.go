package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

// tokenSpy is a server that records every device token presented to it.
func tokenSpy(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// saveProfiles writes cli.json with the given profiles; the first one is the default.
func saveProfiles(t *testing.T, profiles ...ddclient.Profile) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, _ := ddclient.ConfigPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	cfg := ddclient.Config{}
	for i, pr := range profiles {
		cfg.Upsert(pr, i == 0)
	}
	if err := cfg.Save(p); err != nil {
		t.Fatal(err)
	}
}

// --server must never send the default profile's device token to another address.
func TestServerFlagDoesNotRedirectToken(t *testing.T) {
	var hits atomic.Int32
	other := tokenSpy(t, &hits)
	saveProfiles(t, ddclient.Profile{Name: "default", ServerURL: "https://drive.example", DeviceToken: "secret"})

	var out, errb bytes.Buffer
	if code := Run([]string{"--server", other.URL, "ls"}, &out, &errb); code == 0 {
		t.Fatal("ls against an unpaired server must fail")
	}
	if hits.Load() != 0 {
		t.Fatalf("the unpaired server received %d requests", hits.Load())
	}
	if !strings.Contains(errb.String(), "no saved login for "+other.URL) {
		t.Fatalf("stderr = %q", errb.String())
	}
}

// --server picks the saved login paired with that address, with its own token.
func TestServerFlagSelectsMatchingProfile(t *testing.T) {
	srv := httptest.NewServer(filesMuxCLI(t))
	defer srv.Close()
	saveProfiles(t,
		ddclient.Profile{Name: "default", ServerURL: "https://drive.example", DeviceToken: "secret"},
		ddclient.Profile{Name: "lan", ServerURL: srv.URL + "/", DeviceToken: "kfd"},
	)

	var out, errb bytes.Buffer
	if code := Run([]string{"--server", srv.URL, "ls"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "docs/") {
		t.Fatalf("ls out:\n%s", out.String())
	}
}

// An explicit --profile paired elsewhere is refused rather than redirected.
func TestServerFlagMismatchedProfileRefused(t *testing.T) {
	var hits atomic.Int32
	other := tokenSpy(t, &hits)
	saveProfiles(t, ddclient.Profile{Name: "work", ServerURL: "https://drive.example", DeviceToken: "secret"})

	var out, errb bytes.Buffer
	if code := Run([]string{"--profile", "work", "--server", other.URL, "ls"}, &out, &errb); code == 0 {
		t.Fatal("a mismatched --profile/--server pair must fail")
	}
	if hits.Load() != 0 {
		t.Fatalf("the unpaired server received %d requests", hits.Load())
	}
	if !strings.Contains(errb.String(), `profile "work" is paired with https://drive.example`) {
		t.Fatalf("stderr = %q", errb.String())
	}
}

// Server-supplied names must not reach the terminal as escape sequences.
func TestHumanOutputStripsTerminalControls(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"f1","name":"a\u001b]52;c;cHduZWQ=\u0007b‮\r.txt","is_dir":false,"size":1,"version":1}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"ls"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if strings.ContainsAny(out.String(), "\x1b\x07\r‮") {
		t.Fatalf("control characters leaked: %q", out.String())
	}
	if !strings.Contains(out.String(), "a]52;c;cHduZWQ=b.txt") {
		t.Fatalf("visible text lost: %q", out.String())
	}

	// --json keeps the exact name (encoding/json escapes the controls itself).
	out.Reset()
	if code := Run([]string{"--json", "ls"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), `\u001b]52`) || !strings.Contains(out.String(), "‮") {
		t.Fatalf("json output altered: %q", out.String())
	}
}

// get - streams the file verbatim, control bytes included.
func TestGetToStdoutIsVerbatim(t *testing.T) {
	payload := "\x1b[31mbinary\x00\r\n\xff"
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"f1","name":"bin","is_dir":false,"size":13,"version":1}]`))
	})
	mux.HandleFunc("GET /files/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"get", "bin", "-"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if out.String() != payload {
		t.Fatalf("get - altered the bytes: %q", out.String())
	}
}
