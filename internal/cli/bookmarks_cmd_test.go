package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const cliTree = `[
	{"id":"root1","parent_id":null,"is_folder":true,"title":"Toolbar","position":0},
	{"id":"dev","parent_id":"root1","is_folder":true,"title":"Dev","position":0},
	{"id":"kid","parent_id":"dev","is_folder":false,"title":"Go","url":"https://go.dev","position":1}
]`

// bookmarksMuxCLI stubs the bookmark endpoints; body gets the last write body.
func bookmarksMuxCLI(t *testing.T, body chan map[string]any) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(cliTree))
	})
	mux.HandleFunc("POST /me/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		_ = json.NewDecoder(r.Body).Decode(&got)
		if body != nil {
			body <- got
		}
		_, _ = w.Write([]byte(`{"id":"new","parent_id":"dev","title":"New"}`))
	})
	mux.HandleFunc("PATCH /me/bookmarks/{id}", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		_ = json.NewDecoder(r.Body).Decode(&got)
		if body != nil {
			body <- got
		}
		_, _ = w.Write([]byte(`{"id":"kid","parent_id":"dev","title":"The Go site","url":"https://go.dev"}`))
	})
	return mux
}

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, &out, &errb)
	return out.String(), errb.String(), code
}

func TestBookmarksTreeIndentsByDepth(t *testing.T) {
	srv := httptest.NewServer(bookmarksMuxCLI(t, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, errb, code := runCLI(t, "bookmarks")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "Toolbar/") {
		t.Fatalf("first line must be the root folder:\n%s", out)
	}
	// each level adds two spaces after the id column
	depth := func(s string) int {
		body := s[39:]
		return len(body) - len(strings.TrimLeft(body, " "))
	}
	if depth(lines[0]) != 0 || depth(lines[1]) != 2 || depth(lines[2]) != 4 {
		t.Fatalf("indents = %d/%d/%d, want 0/2/4:\n%s", depth(lines[0]), depth(lines[1]), depth(lines[2]), out)
	}
	if !strings.Contains(lines[2], "https://go.dev") {
		t.Fatalf("a bookmark must show its url:\n%s", out)
	}
}

func TestBookmarksListShowsFolderPath(t *testing.T) {
	srv := httptest.NewServer(bookmarksMuxCLI(t, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, errb, code := runCLI(t, "bookmarks", "list", "--q", "go.dev")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	if strings.Count(out, "\n") != 1 || !strings.Contains(out, "Toolbar/Dev/Go") {
		t.Fatalf("want a single Toolbar/Dev/Go line:\n%s", out)
	}
}

// An edit must send only the flags that were typed: an untouched --parent that
// leaked through as "" would silently move the bookmark to the top level.
func TestBookmarksEditSendsOnlyTypedFlags(t *testing.T) {
	body := make(chan map[string]any, 1)
	srv := httptest.NewServer(bookmarksMuxCLI(t, body))
	defer srv.Close()
	setupProfile(t, srv.URL)

	if _, errb, code := runCLI(t, "bookmarks", "edit", "kid", "--title", "The Go site"); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	got := <-body
	if got["title"] != "The Go site" {
		t.Fatalf("title not sent: %v", got)
	}
	if len(got) != 1 {
		t.Fatalf("only the typed flag may be sent, got %v", got)
	}
}

// An explicitly empty --parent is a deliberate move to the top level.
func TestBookmarksEditEmptyParentMovesToRoot(t *testing.T) {
	body := make(chan map[string]any, 1)
	srv := httptest.NewServer(bookmarksMuxCLI(t, body))
	defer srv.Close()
	setupProfile(t, srv.URL)

	if _, errb, code := runCLI(t, "bookmarks", "edit", "kid", "--parent", ""); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	got := <-body
	v, ok := got["parent_id"]
	if !ok || v != "" {
		t.Fatalf(`want "parent_id": "", got %v`, got)
	}
}

func TestBookmarksEditWithoutFlagsFails(t *testing.T) {
	srv := httptest.NewServer(bookmarksMuxCLI(t, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	if _, _, code := runCLI(t, "bookmarks", "edit", "kid"); code == 0 {
		t.Fatal("an edit with no flags must fail instead of sending an empty patch")
	}
}

func TestBookmarksAddPositionsAfterSiblings(t *testing.T) {
	body := make(chan map[string]any, 1)
	srv := httptest.NewServer(bookmarksMuxCLI(t, body))
	defer srv.Close()
	setupProfile(t, srv.URL)

	if _, errb, code := runCLI(t, "bookmarks", "add", "https://new.example", "--parent", "dev"); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	got := <-body
	if got["position"] != float64(2) {
		t.Fatalf("position = %v, want max+1 = 2", got["position"])
	}
	// a bookmark without --title falls back to its url, so the tree is readable
	if got["title"] != "https://new.example" {
		t.Fatalf("title = %v", got["title"])
	}
}

// The flags must work after the positional argument as well as before it.
func TestBookmarksMkdirTakesTitlePositionally(t *testing.T) {
	body := make(chan map[string]any, 1)
	srv := httptest.NewServer(bookmarksMuxCLI(t, body))
	defer srv.Close()
	setupProfile(t, srv.URL)

	if _, errb, code := runCLI(t, "bookmarks", "mkdir", "Dev", "--parent", "root1"); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	got := <-body
	if got["title"] != "Dev" || got["is_folder"] != true || got["parent_id"] != "root1" {
		t.Fatalf("mkdir body = %v", got)
	}
}

func TestBookmarksListJSONEmptyIsArray(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, _, _ := runCLI(t, "--json", "bookmarks", "list")
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty --json = %q, want []", out)
	}
}

// A folder path is accepted wherever an id is, so nobody has to copy a UUID.
func TestBookmarksAcceptsFolderPath(t *testing.T) {
	body := make(chan map[string]any, 1)
	srv := httptest.NewServer(bookmarksMuxCLI(t, body))
	defer srv.Close()
	setupProfile(t, srv.URL)

	if _, errb, code := runCLI(t, "bookmarks", "add", "https://x.example", "--parent", "Toolbar/Dev"); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	if got := <-body; got["parent_id"] != "dev" {
		t.Fatalf("the path must resolve to the folder id, got %v", got["parent_id"])
	}

	out, errb, code := runCLI(t, "bookmarks", "tree", "--folder", "Toolbar/Dev")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	if strings.Count(out, "\n") != 2 || !strings.Contains(out, "Dev/") {
		t.Fatalf("want the Dev subtree only:\n%s", out)
	}
}

func TestBookmarksUnknownFolderPathFails(t *testing.T) {
	srv := httptest.NewServer(bookmarksMuxCLI(t, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	_, errb, code := runCLI(t, "bookmarks", "add", "https://x.example", "--parent", "Nope/Here")
	if code == 0 {
		t.Fatal("an unknown folder path must fail instead of landing in the root")
	}
	if !strings.Contains(errb, "Nope/Here") {
		t.Fatalf("the error must name the folder: %s", errb)
	}
}
