package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func savedServer(t *testing.T, handler func(mux *http.ServeMux, body *map[string]any, query *string)) (*Client, *map[string]any, *string) {
	t.Helper()
	body := map[string]any{}
	query := ""
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	handler(mux, &body, &query)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL, "kfd"), &body, &query
}

func TestSaveArticleBody(t *testing.T) {
	c, body, _ := savedServer(t, func(mux *http.ServeMux, body *map[string]any, _ *string) {
		mux.HandleFunc("POST /me/saved", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(body)
			_, _ = w.Write([]byte(`{"id":"a1","kind":"article","status":"pending"}`))
		})
	})
	item, err := c.SaveArticle(context.Background(), "https://example.com/post", "Post")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if item.ID != "a1" || item.Status != SavedStatusPending {
		t.Fatalf("item = %+v", item)
	}
	if (*body)["kind"] != SavedKindArticle || (*body)["title"] != "Post" {
		t.Fatalf("body = %v", *body)
	}
	if _, ok := (*body)["cookie"]; ok {
		t.Fatalf("an article must never carry a cookie: %v", *body)
	}
}

func TestQueueDownloadBody(t *testing.T) {
	c, body, _ := savedServer(t, func(mux *http.ServeMux, body *map[string]any, _ *string) {
		mux.HandleFunc("POST /me/saved", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(body)
			_, _ = w.Write([]byte(`{"id":"d1","kind":"download","status":"pending"}`))
		})
	})
	if _, err := c.QueueDownload(context.Background(), "https://example.com/x.iso", "", ""); err != nil {
		t.Fatalf("queue: %v", err)
	}
	if (*body)["kind"] != SavedKindDownload {
		t.Fatalf("kind = %v", (*body)["kind"])
	}
	if _, ok := (*body)["cookie"]; ok {
		t.Fatalf("empty cookie must be omitted: %v", *body)
	}

	if _, err := c.QueueDownload(context.Background(), "https://example.com/x.iso", "", "s=1"); err != nil {
		t.Fatalf("queue with cookie: %v", err)
	}
	if (*body)["cookie"] != "s=1" {
		t.Fatalf("cookie not sent: %v", *body)
	}
}

func TestListSavedArticlesQuery(t *testing.T) {
	c, _, query := savedServer(t, func(mux *http.ServeMux, _ *map[string]any, query *string) {
		mux.HandleFunc("GET /me/saved", func(w http.ResponseWriter, r *http.Request) {
			*query = r.URL.RawQuery
			_, _ = w.Write([]byte(`[{"id":"a1","kind":"article","status":"done"}]`))
		})
	})
	items, err := c.ListSavedArticles(context.Background(), SavedQuery{Status: "done", Q: "go", Limit: 10, Offset: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
	for _, want := range []string{"kind=article", "status=done", "q=go", "limit=10", "offset=20"} {
		if !strings.Contains(*query, want) {
			t.Fatalf("query %q misses %q", *query, want)
		}
	}
	if strings.Contains(*query, "kind=download") {
		t.Fatalf("downloads are never listed: %q", *query)
	}
}

// The endpoint answers text/markdown; a JSON decoder here would silently mangle
// the body, so the fixture deliberately looks like JSON.
func TestSavedContentIsRawMarkdown(t *testing.T) {
	const md = "# Title\n{\"not\":\"json\"}\n"
	c, _, _ := savedServer(t, func(mux *http.ServeMux, _ *map[string]any, _ *string) {
		mux.HandleFunc("GET /me/saved/{id}/content", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			_, _ = w.Write([]byte(md))
		})
	})
	var sb strings.Builder
	if err := c.SavedContent(context.Background(), "a1", &sb); err != nil {
		t.Fatalf("content: %v", err)
	}
	if sb.String() != md {
		t.Fatalf("body must be copied byte for byte, got %q", sb.String())
	}
}

func TestSavedContentNotFound(t *testing.T) {
	c, _, _ := savedServer(t, func(mux *http.ServeMux, _ *map[string]any, _ *string) {
		mux.HandleFunc("GET /me/saved/{id}/content", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"no content"}`))
		})
	})
	var sb strings.Builder
	err := c.SavedContent(context.Background(), "d1", &sb)
	if !IsNotFound(err) {
		t.Fatalf("expected a wrapped 404, got %v", err)
	}
	if !strings.Contains(err.Error(), "no saved text") {
		t.Fatalf("the human hint is missing: %v", err)
	}
}

func TestRetrySavedConflict(t *testing.T) {
	c, _, _ := savedServer(t, func(mux *http.ServeMux, _ *map[string]any, _ *string) {
		mux.HandleFunc("POST /me/saved/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"item is not retryable"}`))
		})
	})
	_, err := c.RetrySavedItem(context.Background(), "a1")
	if !IsAPIStatus(err, http.StatusConflict) {
		t.Fatalf("the APIError must stay unwrappable, got %v", err)
	}
	if !strings.Contains(err.Error(), "still being processed") {
		t.Fatalf("the human hint is missing: %v", err)
	}
}

func TestGetAndDeleteSavedItem(t *testing.T) {
	c, _, _ := savedServer(t, func(mux *http.ServeMux, _ *map[string]any, _ *string) {
		mux.HandleFunc("GET /me/saved/{id}", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":"d1","kind":"download","status":"processing","bytes_done":1024,"size_bytes":4096}`))
		})
		mux.HandleFunc("DELETE /me/saved/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	})
	item, err := c.GetSavedItem(context.Background(), "d1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if item.BytesDone != 1024 || item.SizeBytes == nil || *item.SizeBytes != 4096 {
		t.Fatalf("progress = %+v", item)
	}
	if err := c.DeleteSavedItem(context.Background(), "d1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// Only the server's exact "not found" for the id counts as an unknown saved item.
func TestIsUnknownSavedItem(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&APIError{Status: 404, Message: "not found", FromBody: true}, true},
		{&APIError{Status: 404, Message: "Not Found", FromBody: false}, false},
		{&APIError{Status: 404, Message: "route missing", FromBody: true}, false},
		{&APIError{Status: 500, Message: "not found", FromBody: true}, false},
		{nil, false},
	}
	for i, tc := range cases {
		if got := IsUnknownSavedItem(tc.err); got != tc.want {
			t.Errorf("case %d (%v): got %v, want %v", i, tc.err, got, tc.want)
		}
	}
}
