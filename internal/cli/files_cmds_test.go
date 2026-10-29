package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

// setupProfile writes cli.json at ConfigPath() with a profile pointing at srv and sets up env cleanup.
func setupProfile(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, _ := ddclient.ConfigPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	cfg := ddclient.Config{}
	cfg.Upsert(ddclient.Profile{Name: "default", ServerURL: srvURL, DeviceToken: "kfd"}, true)
	if err := cfg.Save(p); err != nil {
		t.Fatal(err)
	}
}

func TestLsListsRoot(t *testing.T) {
	srv := httptest.NewServer(filesMuxCLI(t))
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"ls"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("docs/")) {
		t.Fatalf("ls out:\n%s", out.String())
	}
}

func TestGetDownloadsToDest(t *testing.T) {
	srv := httptest.NewServer(filesMuxCLI(t))
	defer srv.Close()
	setupProfile(t, srv.URL)

	dest := filepath.Join(t.TempDir(), "out.txt")
	var out, errb bytes.Buffer
	if code := Run([]string{"get", "docs/a.txt", dest}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "hello" {
		t.Fatalf("downloaded %q", string(b))
	}
}

// filesMuxCLI is like filesMux from the ddclient tests, but in the cli package.
func filesMuxCLI(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("parent_id") {
		case "":
			_, _ = w.Write([]byte(`[{"id":"d1","name":"docs","is_dir":true,"version":1}]`))
		case "d1":
			_, _ = w.Write([]byte(`[{"id":"f1","name":"a.txt","is_dir":false,"size":5,"version":2}]`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	})
	mux.HandleFunc("GET /files/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	return mux
}

// TestMkdirIntoFileErrors: creating a folder inside a file must return a "not a folder" error, without hitting the server.
func TestMkdirIntoFileErrors(t *testing.T) {
	srv := httptest.NewServer(filesMuxCLI(t))
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	code := Run([]string{"mkdir", "docs/a.txt/sub"}, &out, &errb)
	if code == 0 {
		t.Fatal("expected an error: docs/a.txt is not a folder")
	}
	if !bytes.Contains(errb.Bytes(), []byte("is not a folder")) {
		t.Fatalf("stderr=%q, expected \"is not a folder\"", errb.String())
	}
}

func TestRmIsMarkedIrreversibleInHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"rm", "--help"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("Irreversible")) {
		t.Fatalf("rm help must flag irreversibility:\n%s", out.String())
	}
}

func TestRmDeletes(t *testing.T) {
	deleted := make(chan string, 1)
	mux := filesMuxCLI(t).(*http.ServeMux)
	mux.HandleFunc("DELETE /files/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleted <- r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"rm", "docs/a.txt"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	select {
	case id := <-deleted:
		if id != "f1" {
			t.Fatalf("deleted id=%q", id)
		}
	default:
		t.Fatal("DELETE not called")
	}
}

// TestRenameRenamesInPlace: rename sends PATCH /files/{id}/rename with the new name.
func TestRenameRenamesInPlace(t *testing.T) {
	type call struct{ id, name string }
	got := make(chan call, 1)
	mux := filesMuxCLI(t).(*http.ServeMux)
	mux.HandleFunc("PATCH /files/{id}/rename", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		got <- call{id: r.PathValue("id"), name: b.Name}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": r.PathValue("id"), "name": b.Name, "is_dir": false})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"rename", "docs/a.txt", "final.txt"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	select {
	case c := <-got:
		if c.id != "f1" || c.name != "final.txt" {
			t.Fatalf("rename got id=%q name=%q, expected f1/final.txt", c.id, c.name)
		}
	default:
		t.Fatal("PATCH rename was not called")
	}
}

// TestRenameRejectsPath: a <new-name> containing "/" is rejected (that's a move — mv), the server is not contacted.
func TestRenameRejectsPath(t *testing.T) {
	mux := filesMuxCLI(t).(*http.ServeMux)
	mux.HandleFunc("PATCH /files/{id}/rename", func(w http.ResponseWriter, r *http.Request) {
		t.Error("PATCH rename must not be called when the name contains a path")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"rename", "docs/a.txt", "sub/final.txt"}, &out, &errb); code == 0 {
		t.Fatalf("expected a non-zero code for a path in <new-name>, out=%s", out.String())
	}
}

// TestShareLinkDefault: without --email, a public link is created.
func TestShareLinkDefault(t *testing.T) {
	mux := filesMuxCLI(t).(*http.ServeMux)
	mux.HandleFunc("POST /files/{id}/share", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Link   bool   `json:"link"`
			Email  string `json:"email"`
			Access string `json:"access"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if !b.Link || b.Email != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"share_id": "s1", "token": "tok", "access": b.Access})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"share", "docs/a.txt"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("/s/tok")) {
		t.Fatalf("out=%s", out.String())
	}
}

// TestShareEmailFlagsAfterPath: flags after <path> are applied — a regression test for the flag-parsing bug.
func TestShareEmailFlagsAfterPath(t *testing.T) {
	var gotEmail, gotAccess string
	var gotLink bool
	mux := filesMuxCLI(t).(*http.ServeMux)
	mux.HandleFunc("POST /files/{id}/share", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Link   bool   `json:"link"`
			Email  string `json:"email"`
			Access string `json:"access"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		gotEmail, gotAccess, gotLink = b.Email, b.Access, b.Link
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"share_id": "s2", "access": b.Access})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	code := Run([]string{"share", "docs/a.txt", "--email", "bob@example.com", "--access", "edit"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if gotEmail != "bob@example.com" || gotAccess != "edit" || gotLink {
		t.Fatalf("server got email=%q access=%q link=%v; expected an email grant with edit", gotEmail, gotAccess, gotLink)
	}
}
