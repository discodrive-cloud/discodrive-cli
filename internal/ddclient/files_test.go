package ddclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// filesMux: a two-level listing (root → "docs" (dir) → "a.txt"), download, and operations.
func filesMux(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "jwt"})
	})
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("parent_id") {
		case "": // root
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "d1", "name": "docs", "is_dir": true, "version": 1},
			})
		case "d1":
			size := int64(5)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "f1", "name": "a.txt", "is_dir": false, "size": size, "version": 2},
			})
		default:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}
	})
	mux.HandleFunc("GET /files/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello")
	})
	mux.HandleFunc("POST /files/folder", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "new", "name": b.Name, "is_dir": true, "version": 1})
	})
	mux.HandleFunc("POST /files/upload", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"node":       map[string]any{"id": "up", "name": r.FormValue("name"), "is_dir": false, "version": 1},
			"conflicted": false,
		})
	})
	mux.HandleFunc("DELETE /files/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func TestResolveWalksPath(t *testing.T) {
	srv := httptest.NewServer(filesMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")

	root, err := c.Resolve(context.Background(), "/")
	if err != nil || root.ID != "" || !root.IsDir {
		t.Fatalf("root: %+v err=%v", root, err)
	}
	n, err := c.Resolve(context.Background(), "docs/a.txt")
	if err != nil || n.ID != "f1" || n.Name != "a.txt" {
		t.Fatalf("resolve: %+v err=%v", n, err)
	}
	if _, err := c.Resolve(context.Background(), "docs/missing"); !IsNotFound(err) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestListChildrenAndDownload(t *testing.T) {
	srv := httptest.NewServer(filesMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")

	kids, err := c.ListChildren(context.Background(), "d1")
	if err != nil || len(kids) != 1 || kids[0].Name != "a.txt" || kids[0].Size == nil || *kids[0].Size != 5 {
		t.Fatalf("children: %+v err=%v", kids, err)
	}
	var buf bytes.Buffer
	if err := c.Download(context.Background(), "f1", &buf); err != nil || buf.String() != "hello" {
		t.Fatalf("download: %q err=%v", buf.String(), err)
	}
}

func TestUploadAndCreateFolder(t *testing.T) {
	srv := httptest.NewServer(filesMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")

	dir, err := c.CreateFolder(context.Background(), "", "newdir")
	if err != nil || dir.Name != "newdir" || !dir.IsDir {
		t.Fatalf("mkdir: %+v err=%v", dir, err)
	}
	up, err := c.Upload(context.Background(), "d1", "b.txt", strings.NewReader("data"), time.Time{})
	if err != nil || up.Name != "b.txt" {
		t.Fatalf("upload: %+v err=%v", up, err)
	}
}

func TestSearchRecursive(t *testing.T) {
	srv := httptest.NewServer(filesMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")
	hits, err := c.Search(context.Background(), "/", "a.txt")
	if err != nil || len(hits) != 1 || hits[0].Path != "docs/a.txt" {
		t.Fatalf("search: %+v err=%v", hits, err)
	}
}
