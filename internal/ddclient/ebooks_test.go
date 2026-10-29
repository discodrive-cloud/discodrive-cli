package ddclient

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func ebooksMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/ebooks/library", func(w http.ResponseWriter, r *http.Request) {
		// verify that the q filter is passed through
		if r.URL.Query().Get("q") != "go" {
			http.Error(w, `{"error":"expected q=go"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"books":[{"id":"b1","nodeId":"n1","title":"Go","authors":["A"],"format":"epub","tags":[],"hasCover":true}],"total":1}`))
	})
	mux.HandleFunc("GET /me/ebooks/library/facets", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"authors":["A"],"series":[],"genres":["sci"]}`))
	})
	mux.HandleFunc("GET /me/ebooks/library/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("EPUBDATA"))
	})
	mux.HandleFunc("GET /me/ebooks/meta/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"title":"Go","authors":["A"],"tags":[],"series":"","seriesIndex":0,"language":"en","description":"d","publisher":"P","date":"2020","edited":false}`))
	})
	mux.HandleFunc("PUT /me/ebooks/meta/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /me/ebooks/scan", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"scanning":false}`))
	})
	return mux
}

func TestListBooksFilter(t *testing.T) {
	srv := httptest.NewServer(ebooksMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	got, err := c.ListBooks(context.Background(), BookQuery{Q: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Books) != 1 || got.Books[0].NodeID != "n1" || got.Total != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestBookFacets(t *testing.T) {
	srv := httptest.NewServer(ebooksMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	f, err := c.BookFacets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Authors) != 1 || len(f.Genres) != 1 {
		t.Fatalf("got %+v", f)
	}
}

func TestDownloadBook(t *testing.T) {
	srv := httptest.NewServer(ebooksMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	var buf bytes.Buffer
	if err := c.DownloadBook(context.Background(), "b1", &buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "EPUBDATA" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestGetAndUpdateBookMeta(t *testing.T) {
	srv := httptest.NewServer(ebooksMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	m, err := c.GetBookMeta(context.Background(), "n1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Go" || m.Publisher != "P" {
		t.Fatalf("got %+v", m)
	}
	m.Title = "Go 2"
	if err := c.UpdateBookMeta(context.Background(), "n1", m); err != nil {
		t.Fatal(err)
	}
}

func TestEbookScan(t *testing.T) {
	srv := httptest.NewServer(ebooksMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	scanning, err := c.EbookScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if scanning {
		t.Fatal("expected scanning=false (no folder)")
	}
}
