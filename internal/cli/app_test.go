package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

func TestRootHelpListsCommands(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"--help"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	s := out.String()
	for _, want := range []string{"dd-cli", "Commands:", "--json", "--profile", "--server"} {
		if !strings.Contains(s, want) {
			t.Fatalf("root help missing %q\n%s", want, s)
		}
	}
}

func TestUnknownCommandErrors(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"bogus"}, &out, &errb)
	if code != 2 {
		t.Fatalf("expected code 2 for an unknown command, got %d", code)
	}
	if !strings.Contains(errb.String(), "bogus") {
		t.Fatalf("stderr should name the command: %q", errb.String())
	}
}

func TestNoArgsShowsHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run(nil, &out, &errb); code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatal("expected root help on no args")
	}
}

// TestGlobalFlagsParsed: global flags (including the =-forms) actually make it into cmdCtx.
func TestGlobalFlagsParsed(t *testing.T) {
	var got *cmdCtx
	register(&command{Name: "flagprobe", Summary: "probe", Run: func(cc *cmdCtx, args []string) error { got = cc; return nil }})
	cases := []struct {
		args            []string
		json            bool
		profile, server string
	}{
		{[]string{"--json", "--profile", "work", "--server", "http://x", "flagprobe"}, true, "work", "http://x"},
		{[]string{"--profile=home", "--server=https://y", "flagprobe"}, false, "home", "https://y"},
	}
	for _, c := range cases {
		got = nil
		var out, errb bytes.Buffer
		if code := Run(c.args, &out, &errb); code != 0 {
			t.Fatalf("code=%d err=%s", code, errb.String())
		}
		if got == nil {
			t.Fatal("command did not run")
		}
		if got.JSON != c.json || got.Profile != c.profile || got.Server != c.server {
			t.Fatalf("parsed JSON=%v Profile=%q Server=%q, expected %v/%q/%q",
				got.JSON, got.Profile, got.Server, c.json, c.profile, c.server)
		}
	}
}

// TestJSONErrorOutput: with --json, a command error is printed as {"error": ...} to stderr, exit code 1.
func TestJSONErrorOutput(t *testing.T) {
	register(&command{Name: "errcmd", Summary: "probe", Run: func(cc *cmdCtx, args []string) error { return fmt.Errorf("boom") }})
	var out, errb bytes.Buffer
	code := Run([]string{"--json", "errcmd"}, &out, &errb)
	if code != 1 {
		t.Fatalf("code=%d, expected 1", code)
	}
	if !strings.Contains(errb.String(), `{"error":`) || !strings.Contains(errb.String(), "boom") {
		t.Fatalf("stderr=%q", errb.String())
	}
}

// TestWhoamiJSON: `whoami --json` hits GET /me and prints the user object.
func TestWhoamiJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "jwt"})
	})
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "email": "me@example.com", "role": "user"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath, err := ddclient.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	cfg := ddclient.Config{}
	cfg.Upsert(ddclient.Profile{Name: "default", ServerURL: srv.URL, DeviceToken: "kfd"}, true)
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := Run([]string{"--json", "whoami"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "me@example.com") {
		t.Fatalf("whoami out: %s", out.String())
	}
}
