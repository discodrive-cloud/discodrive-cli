package ddmcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

func TestSavedDefaultHidesChanging(t *testing.T) {
	got := bookmarkNames(selectTools(savedToolSpecs(), false))
	for _, want := range []string{"dd_pocket_list", "dd_pocket_read", "dd_download_status"} {
		if !got[want] {
			t.Fatalf("read-only tool %q must be available by default", want)
		}
	}
	for _, banned := range []string{"dd_pocket_retry", "dd_pocket_delete", "dd_download_cancel", "dd_pocket_add", "dd_download_add"} {
		if got[banned] {
			t.Fatalf("changing tool %q must be hidden without --allow-changes", banned)
		}
	}
}

func TestToolCountsMatchTheGate(t *testing.T) {
	all := allToolSpecs()
	open, gated := len(selectTools(all, false)), len(selectTools(all, true))
	if gated != 43 || open != 22 {
		t.Fatalf("tools: %d by default, %d under --allow-changes; want 22 and 43", open, gated)
	}
}

// savedAPI stubs the saved endpoints. content is served as text/markdown.
func savedAPI(t *testing.T, item, content string, body chan map[string]any) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/saved", func(w http.ResponseWriter, r *http.Request) {
		if body != nil {
			body <- map[string]any{"query": r.URL.RawQuery}
		}
		_, _ = w.Write([]byte("[" + item + "]"))
	})
	mux.HandleFunc("GET /me/saved/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(item))
	})
	mux.HandleFunc("GET /me/saved/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte(content))
	})
	mux.HandleFunc("POST /me/saved", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		_ = json.NewDecoder(r.Body).Decode(&got)
		if body != nil {
			body <- got
		}
		_, _ = w.Write([]byte(item))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

const doneArticle = `{"id":"a1","kind":"article","status":"done","url":"https://example.org/a",
	"title":"Long read","has_content":true}`

func TestPocketReadPagesThroughRunes(t *testing.T) {
	// Cyrillic on purpose: a byte-based slice would cut a character in half.
	article := "# Заголовок\n" + strings.Repeat("я", 100)
	api := savedAPI(t, doneArticle, article, nil)

	res := callTool(t, api, "dd_pocket_read", map[string]any{"id": "a1", "max_chars": 12})
	var first pocketReadOutput
	if err := json.Unmarshal(mustStructured(t, res), &first); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if first.Content != "# Заголовок\n" {
		t.Fatalf("first page = %q, want the header sliced by runes", first.Content)
	}
	if first.TotalChars != 112 || !first.Truncated {
		t.Fatalf("total=%d truncated=%v", first.TotalChars, first.Truncated)
	}

	res = callTool(t, api, "dd_pocket_read", map[string]any{"id": "a1", "offset": 12, "max_chars": 500})
	var rest pocketReadOutput
	if err := json.Unmarshal(mustStructured(t, res), &rest); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rest.Truncated || rest.Content != strings.Repeat("я", 100) {
		t.Fatalf("second page truncated=%v len=%d", rest.Truncated, len([]rune(rest.Content)))
	}
	if rest.Title != "Long read" || rest.URL != "https://example.org/a" {
		t.Fatalf("metadata lost: %+v", rest)
	}
}

// An offset past the end must return an empty page instead of panicking.
func TestPocketReadOffsetPastEnd(t *testing.T) {
	api := savedAPI(t, doneArticle, "short", nil)
	res := callTool(t, api, "dd_pocket_read", map[string]any{"id": "a1", "offset": 9000})
	var out pocketReadOutput
	if err := json.Unmarshal(mustStructured(t, res), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Content != "" || out.Truncated {
		t.Fatalf("want an empty last page, got %+v", out)
	}
}

// dd_pocket_list must never leak downloads, so kind=article is always sent.
func TestPocketListAlwaysAsksForArticles(t *testing.T) {
	body := make(chan map[string]any, 1)
	api := savedAPI(t, doneArticle, "", body)
	callTool(t, api, "dd_pocket_list", map[string]any{"status": "done"})

	q := (<-body)["query"].(string)
	if !strings.Contains(q, "kind=article") {
		t.Fatalf("query = %q, want kind=article", q)
	}
}

func TestDownloadAddSendsDownloadKind(t *testing.T) {
	body := make(chan map[string]any, 1)
	api := savedAPI(t, `{"id":"d1","kind":"download","status":"pending","url":"https://example.org/f.iso"}`, "", body)
	res := callToolAllow(t, api, "dd_download_add", map[string]any{"url": "https://example.org/f.iso"}, true)

	got := <-body
	if got["kind"] != ddclient.SavedKindDownload {
		t.Fatalf("kind = %v, want %q", got["kind"], ddclient.SavedKindDownload)
	}
	var item ddclient.SavedItem
	if err := json.Unmarshal(mustStructured(t, res), &item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.ID != "d1" || item.Status != ddclient.SavedStatusPending {
		t.Fatalf("the id and the pending status must come back: %+v", item)
	}
}

func TestDownloadStatusReportsProgress(t *testing.T) {
	api := savedAPI(t, `{"id":"d1","kind":"download","status":"processing",
		"url":"https://example.org/f.iso","bytes_done":1048576,"size_bytes":4194304}`, "", nil)
	res := callTool(t, api, "dd_download_status", map[string]any{"id": "d1"})

	var item ddclient.SavedItem
	if err := json.Unmarshal(mustStructured(t, res), &item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.BytesDone != 1048576 || item.SizeBytes == nil || *item.SizeBytes != 4194304 {
		t.Fatalf("progress lost: %+v", item)
	}
}

// The model has no way to enumerate downloads, so the descriptions must say so
// out loud — otherwise it hunts for a listing tool that does not exist.
func TestNewToolDescriptionsWarnAboutNoDownloadListing(t *testing.T) {
	desc := map[string]string{}
	for _, sp := range savedToolSpecs() {
		desc[sp.Name] = strings.ToLower(sp.Description)
	}
	for name, want := range map[string]string{
		"dd_download_add":    "downloads are never listed anywhere",
		"dd_download_status": "there is no list of downloads",
		"dd_pocket_list":     "never appear",
	} {
		if !strings.Contains(desc[name], want) {
			t.Fatalf("%s must warn about the missing listing (%q): %q", name, want, desc[name])
		}
	}
}
