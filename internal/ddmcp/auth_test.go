package ddmcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

// mcpJWT is an unsigned JWT carrying a did claim (for revocation tests).
func mcpJWT(did string) string {
	seg := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	return seg(`{"alg":"none"}`) + "." + seg(`{"sub":"u1","did":"`+did+`"}`) + "." + seg("sig")
}

// writeProfile saves a default profile to an isolated config (XDG is already overridden).
func writeProfile(t *testing.T, srvURL string) {
	t.Helper()
	p, _ := ddclient.ConfigPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	cfg := ddclient.Config{}
	cfg.Upsert(ddclient.Profile{Name: "default", ServerURL: srvURL, DeviceToken: "kfd"}, true)
	if err := cfg.Save(p); err != nil {
		t.Fatal(err)
	}
}

func TestResolveCredsEnvWins(t *testing.T) {
	env := map[string]string{"DISCODRIVE_SERVER": "https://env.example/", "DISCODRIVE_TOKEN": "env-tok"}
	getenv := func(k string) string { return env[k] }
	// lookup would return a profile, but both env vars are set — env must win (and without the trailing slash).
	lookup := func(string) (ddclient.Profile, bool) {
		return ddclient.Profile{ServerURL: "https://cfg.example", DeviceToken: "cfg-tok"}, true
	}
	p, err := resolveCreds("", getenv, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if p.ServerURL != "https://env.example" || p.DeviceToken != "env-tok" {
		t.Fatalf("env must win: s=%q tok=%q", p.ServerURL, p.DeviceToken)
	}
	if p.Insecure {
		t.Fatal("insecure is not set in env — must be false")
	}
}

func TestResolveCredsEnvInsecure(t *testing.T) {
	env := map[string]string{
		"DISCODRIVE_SERVER":       "https://env.example",
		"DISCODRIVE_TOKEN":        "env-tok",
		"DISCODRIVE_INSECURE_TLS": "1",
	}
	getenv := func(k string) string { return env[k] }
	lookup := func(string) (ddclient.Profile, bool) { return ddclient.Profile{}, false }
	p, err := resolveCreds("", getenv, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Insecure {
		t.Fatal("DISCODRIVE_INSECURE_TLS=1 in env mode must enable insecure")
	}
}

func TestResolveCredsPartialEnvFallsBackToConfig(t *testing.T) {
	// only SERVER is set, TOKEN is empty → env is ignored, use the profile.
	env := map[string]string{"DISCODRIVE_SERVER": "https://env.example"}
	getenv := func(k string) string { return env[k] }
	lookup := func(string) (ddclient.Profile, bool) {
		return ddclient.Profile{ServerURL: "https://cfg.example", DeviceToken: "cfg-tok", Insecure: true}, true
	}
	p, err := resolveCreds("", getenv, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if p.ServerURL != "https://cfg.example" || p.DeviceToken != "cfg-tok" {
		t.Fatalf("expected the config profile: s=%q tok=%q", p.ServerURL, p.DeviceToken)
	}
	if !p.Insecure {
		t.Fatal("insecure from the profile must be preserved")
	}
}

func TestResolveCredsNoneErrors(t *testing.T) {
	getenv := func(string) string { return "" }
	lookup := func(string) (ddclient.Profile, bool) { return ddclient.Profile{}, false }
	if _, err := resolveCreds("", getenv, lookup); err == nil {
		t.Fatal("expected an error when both env and profile are absent")
	}
}

func TestLoginPairsAndSavesProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	mux := http.NewServeMux()
	mux.HandleFunc("POST /pair/init", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"AB-CD","verification_uri":"/p?c=AB-CD","interval":1,"expires_in":600}`))
	})
	mux.HandleFunc("POST /pair/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"approved","device_token":"kfd_new"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var out, errb bytes.Buffer
	// --no-claude: this test verifies that the profile is saved, not the Claude registration
	// (that is covered in claudecfg), and it must not touch the real ~/.claude.json.
	if err := Login(context.Background(), []string{"--server", srv.URL, "--no-claude"}, &out, &errb); err != nil {
		t.Fatalf("login: %v (err=%s)", err, errb.String())
	}
	p, _ := ddclient.ConfigPath()
	cfg, _ := ddclient.Load(p)
	prof, ok := cfg.Lookup("default")
	if !ok || prof.DeviceToken != "kfd_new" || prof.ServerURL != srv.URL {
		t.Fatalf("profile not saved correctly: %+v ok=%v", prof, ok)
	}
}

// --insecure: pairing against a self-signed (httptest TLS) server succeeds, and the
// flag is persisted in the profile so later clients (MCP included) inherit it.
func TestLoginInsecurePairsSelfSignedAndPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	mux := http.NewServeMux()
	mux.HandleFunc("POST /pair/init", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"AB-CD","verification_uri":"/p?c=AB-CD","interval":1,"expires_in":600}`))
	})
	mux.HandleFunc("POST /pair/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"approved","device_token":"kfd_tls"}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	var out, errb bytes.Buffer
	// without --insecure the self-signed certificate must be rejected
	if err := Login(context.Background(), []string{"--server", srv.URL, "--no-claude"}, &out, &errb); err == nil {
		t.Fatal("without --insecure a self-signed certificate must be rejected")
	}
	if err := Login(context.Background(), []string{"--server", srv.URL, "--no-claude", "--insecure"}, &out, &errb); err != nil {
		t.Fatalf("login --insecure: %v (err=%s)", err, errb.String())
	}
	p, _ := ddclient.ConfigPath()
	cfg, _ := ddclient.Load(p)
	prof, ok := cfg.Lookup("default")
	if !ok || prof.DeviceToken != "kfd_tls" || !prof.Insecure {
		t.Fatalf("insecure must be persisted in the profile: %+v ok=%v", prof, ok)
	}
}

func TestLogoutRevokesProfileAndForgets(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gotDelete := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"` + mcpJWT("dev-7") + `"}`))
	})
	mux.HandleFunc("DELETE /devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotDelete <- r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	writeProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if err := Logout(context.Background(), nil, &out, &errb); err != nil {
		t.Fatalf("logout: %v (err=%s)", err, errb.String())
	}
	select {
	case id := <-gotDelete:
		if id != "dev-7" {
			t.Fatalf("revoked %q, expected dev-7", id)
		}
	default:
		t.Fatal("server did not receive DELETE /devices/{id}")
	}
	p, _ := ddclient.ConfigPath()
	cfg, _ := ddclient.Load(p)
	if _, ok := cfg.Lookup("default"); ok {
		t.Fatal("profile must be deleted")
	}
}

// env-only mode (without a saved profile): logout still revokes the device using the env credentials.
func TestLogoutEnvOnlyRevokes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // config is empty — no profile
	gotDelete := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"` + mcpJWT("dev-env") + `"}`))
	})
	mux.HandleFunc("DELETE /devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotDelete <- r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("DISCODRIVE_SERVER", srv.URL)
	t.Setenv("DISCODRIVE_TOKEN", "env-tok")

	var out, errb bytes.Buffer
	if err := Logout(context.Background(), nil, &out, &errb); err != nil {
		t.Fatalf("logout env-only: %v (err=%s)", err, errb.String())
	}
	select {
	case id := <-gotDelete:
		if id != "dev-env" {
			t.Fatalf("revoked %q, expected dev-env", id)
		}
	default:
		t.Fatal("env-only logout did not revoke the device")
	}
}

func TestLogoutCredentialPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		want    string
		keep    bool
		wantErr bool
	}{
		{"env overrides default", nil, "env", true, false},
		{"explicit profile", []string{"--profile", "default"}, "saved", false, false},
		{"unknown explicit profile", []string{"--profile", "missing"}, "", true, true},
		{"local only env", []string{"--local-only"}, "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			deleted := make(chan string, 2)
			mux := http.NewServeMux()
			mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				json.NewEncoder(w).Encode(map[string]string{"token": mcpJWT(body["device_token"])})
			})
			mux.HandleFunc("DELETE /devices/{id}", func(w http.ResponseWriter, r *http.Request) { deleted <- r.PathValue("id"); w.WriteHeader(204) })
			srv := httptest.NewServer(mux)
			defer srv.Close()
			path, _ := ddclient.ConfigPath()
			cfg := ddclient.Config{}
			cfg.Upsert(ddclient.Profile{Name: "default", ServerURL: srv.URL, DeviceToken: "saved"}, true)
			if err := cfg.Save(path); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DISCODRIVE_SERVER", srv.URL)
			t.Setenv("DISCODRIVE_TOKEN", "env")
			var out, errb bytes.Buffer
			err := Logout(context.Background(), tc.args, &out, &errb)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v stderr=%s", err, &errb)
			}
			got := ""
			select {
			case got = <-deleted:
			default:
			}
			if got != tc.want {
				t.Fatalf("revoked=%q want=%q", got, tc.want)
			}
			cfg, err = ddclient.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := cfg.Lookup("default"); ok != tc.keep {
				t.Fatalf("profile exists=%v want=%v", ok, tc.keep)
			}
		})
	}
}
