package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func ptrStr(s string) *string { return &s }

// bookmarkServer serves a stub API and records the last request body.
func bookmarkServer(t *testing.T, handler func(mux *http.ServeMux, body *map[string]any)) (*Client, *map[string]any) {
	t.Helper()
	body := map[string]any{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	handler(mux, &body)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL, "kfd"), &body
}

func TestListBookmarksDecodesNullParent(t *testing.T) {
	c, _ := bookmarkServer(t, func(mux *http.ServeMux, _ *map[string]any) {
		mux.HandleFunc("GET /me/bookmarks", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[
				{"id":"root1","parent_id":null,"is_folder":true,"title":"Toolbar","url":"","position":0},
				{"id":"kid","parent_id":"root1","is_folder":false,"title":"Go","url":"https://go.dev","position":1}
			]`))
		})
	})
	items, err := c.ListBookmarks(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].ParentID != nil {
		t.Fatalf("top-level parent must decode to nil, got %v", *items[0].ParentID)
	}
	if items[1].ParentID == nil || *items[1].ParentID != "root1" {
		t.Fatalf("child parent = %v", items[1].ParentID)
	}
}

func TestCreateBookmarkOmitsEmptyParent(t *testing.T) {
	c, body := bookmarkServer(t, func(mux *http.ServeMux, body *map[string]any) {
		mux.HandleFunc("POST /me/bookmarks", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(body)
			_, _ = w.Write([]byte(`{"id":"new","parent_id":null,"is_folder":false,"title":"Go","url":"https://go.dev"}`))
		})
	})
	if _, err := c.CreateBookmark(context.Background(), BookmarkCreate{Title: "Go", URL: "https://go.dev"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, ok := (*body)["parent_id"]; ok {
		t.Fatalf("parent_id must be omitted for a top-level bookmark, body=%v", *body)
	}
	if (*body)["title"] != "Go" {
		t.Fatalf("title not sent: %v", *body)
	}
}

// The patch must carry exactly the fields the caller set: sending an unset field
// would silently rename a bookmark or move it to the top level.
func TestUpdateBookmarkSendsOnlySetFields(t *testing.T) {
	c, body := bookmarkServer(t, func(mux *http.ServeMux, body *map[string]any) {
		mux.HandleFunc("PATCH /me/bookmarks/{id}", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(body)
			_, _ = w.Write([]byte(`{"id":"b1","title":"New"}`))
		})
	})
	if _, err := c.UpdateBookmark(context.Background(), "b1", BookmarkPatch{Title: ptrStr("New")}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if (*body)["title"] != "New" {
		t.Fatalf("title not sent: %v", *body)
	}
	for _, absent := range []string{"url", "position", "parent_id"} {
		if _, ok := (*body)[absent]; ok {
			t.Fatalf("unset field %q must not be sent, body=%v", absent, *body)
		}
	}
}

func TestUpdateBookmarkParentEmptyMeansRoot(t *testing.T) {
	c, body := bookmarkServer(t, func(mux *http.ServeMux, body *map[string]any) {
		mux.HandleFunc("PATCH /me/bookmarks/{id}", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(body)
			_, _ = w.Write([]byte(`{"id":"b1"}`))
		})
	})
	if _, err := c.UpdateBookmark(context.Background(), "b1", BookmarkPatch{ParentID: ptrStr("")}); err != nil {
		t.Fatalf("update: %v", err)
	}
	v, ok := (*body)["parent_id"]
	if !ok || v != "" {
		t.Fatalf("moving to the top level must send an empty parent_id, body=%v", *body)
	}
}

func TestUpdateBookmarkCycleConflict(t *testing.T) {
	c, _ := bookmarkServer(t, func(mux *http.ServeMux, _ *map[string]any) {
		mux.HandleFunc("PATCH /me/bookmarks/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"move would create a cycle"}`))
		})
	})
	_, err := c.UpdateBookmark(context.Background(), "b1", BookmarkPatch{ParentID: ptrStr("kid")})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !IsAPIStatus(err, http.StatusConflict) {
		t.Fatalf("the APIError must stay unwrappable, got %v", err)
	}
}

func TestDeleteBookmarkNotFound(t *testing.T) {
	c, _ := bookmarkServer(t, func(mux *http.ServeMux, _ *map[string]any) {
		mux.HandleFunc("DELETE /me/bookmarks/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	})
	if err := c.DeleteBookmark(context.Background(), "nope"); !IsNotFound(err) {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestBookmarkPathsAndSubtree(t *testing.T) {
	items := []Bookmark{
		{ID: "root", IsFolder: true, Title: "Toolbar"},
		{ID: "dev", ParentID: ptrStr("root"), IsFolder: true, Title: "Dev"},
		{ID: "go", ParentID: ptrStr("dev"), Title: "Go docs"},
		{ID: "orphan", ParentID: ptrStr("gone"), Title: "Lost"},
	}
	paths := BookmarkPaths(items)
	if paths["root"] != "" {
		t.Fatalf("top level has no path, got %q", paths["root"])
	}
	if paths["dev"] != "Toolbar" {
		t.Fatalf("dev path = %q", paths["dev"])
	}
	if paths["go"] != "Toolbar/Dev" {
		t.Fatalf("go path = %q", paths["go"])
	}
	if paths["orphan"] != "" {
		t.Fatalf("a node with a missing parent is treated as top level, got %q", paths["orphan"])
	}

	sub := BookmarkSubtree(items, "root")
	for _, id := range []string{"root", "dev", "go"} {
		if !sub[id] {
			t.Fatalf("%s must be in the subtree", id)
		}
	}
	if sub["orphan"] {
		t.Fatal("orphan must not be in the subtree")
	}
}

func folderTree() []Bookmark {
	toolbar, dev := "toolbar", "dev"
	return []Bookmark{
		{ID: "toolbar", IsFolder: true, Title: "Toolbar"},
		{ID: "dev", ParentID: &toolbar, IsFolder: true, Title: "Dev"},
		{ID: "other", IsFolder: true, Title: "Dev"},
		{ID: "kid", ParentID: &dev, Title: "Go", URL: "https://go.dev"},
	}
}

func TestResolveBookmarkFolder(t *testing.T) {
	items := folderTree()
	for _, tc := range []struct{ ref, want string }{
		{"", ""},
		{"dev", "dev"},           // an id wins over anything else
		{"Toolbar/Dev", "dev"},   // full path
		{"toolbar/dev", "dev"},   // case-insensitive
		{"/Toolbar/Dev/", "dev"}, // stray slashes
		{"Toolbar", "toolbar"},   // a bare title, unique
	} {
		got, err := ResolveBookmarkFolder(items, tc.ref)
		if err != nil {
			t.Fatalf("%q: %v", tc.ref, err)
		}
		if got != tc.want {
			t.Fatalf("%q → %q, want %q", tc.ref, got, tc.want)
		}
	}
}

// A title shared by two nested folders is ambiguous; the full path of either
// one still resolves. A folder sitting at the top level is not ambiguous with
// them — its own path is exactly its title, and a full-path match wins.
func TestResolveBookmarkFolderAmbiguous(t *testing.T) {
	a, b := "a", "b"
	items := []Bookmark{
		{ID: "a", IsFolder: true, Title: "Work"},
		{ID: "b", IsFolder: true, Title: "Home"},
		{ID: "a-dev", ParentID: &a, IsFolder: true, Title: "Dev"},
		{ID: "b-dev", ParentID: &b, IsFolder: true, Title: "Dev"},
	}
	if _, err := ResolveBookmarkFolder(items, "Dev"); err == nil {
		t.Fatal("a title matching two folders must not resolve silently")
	}
	if got, err := ResolveBookmarkFolder(items, "Work/Dev"); err != nil || got != "a-dev" {
		t.Fatalf("the full path must stay unambiguous: %q %v", got, err)
	}
	// folderTree has a root-level "Dev" alongside "Toolbar/Dev": the exact
	// full-path match decides instead of erroring.
	if got, err := ResolveBookmarkFolder(folderTree(), "Dev"); err != nil || got != "other" {
		t.Fatalf("a full-path match must win over a title match: %q %v", got, err)
	}
}

func TestResolveBookmarkFolderMisses(t *testing.T) {
	items := folderTree()
	if _, err := ResolveBookmarkFolder(items, "Nope"); err == nil {
		t.Fatal("an unknown folder must be an error")
	}
	// a bookmark is not a folder, so its title must not resolve
	if _, err := ResolveBookmarkFolder(items, "Go"); err == nil {
		t.Fatal("a bookmark title must not resolve as a folder")
	}
}
