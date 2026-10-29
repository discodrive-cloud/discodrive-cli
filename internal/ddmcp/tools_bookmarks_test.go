package ddmcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mustStructured returns a tool result's structured output as JSON, failing the
// test if the tool itself reported an error.
func mustStructured(t *testing.T, res *mcp.CallToolResult) []byte {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	return b
}

func bookmarkNames(specs []toolSpec) map[string]bool {
	out := map[string]bool{}
	for _, s := range specs {
		out[s.Name] = true
	}
	return out
}

func TestBookmarksDefaultHidesChanging(t *testing.T) {
	got := bookmarkNames(selectTools(bookmarkToolSpecs(), false))
	for _, want := range []string{"dd_bookmarks_list", "dd_bookmarks_add"} {
		if !got[want] {
			t.Fatalf("read-only tool %q must be available by default", want)
		}
	}
	for _, banned := range []string{"dd_bookmarks_update", "dd_bookmarks_delete"} {
		if got[banned] {
			t.Fatalf("changing tool %q must be hidden without --allow-changes", banned)
		}
	}
}

func TestBookmarksAllowChangesExposesAll(t *testing.T) {
	got := bookmarkNames(selectTools(bookmarkToolSpecs(), true))
	for _, want := range []string{"dd_bookmarks_update", "dd_bookmarks_delete"} {
		if !got[want] {
			t.Fatalf("changing tool %q must appear under --allow-changes", want)
		}
	}
}

// bookmarkAPI stubs the server; body receives the last mutating request body.
func bookmarkAPI(t *testing.T, items string, body chan map[string]any) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(items))
	})
	mux.HandleFunc("PATCH /me/bookmarks/{id}", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		_ = json.NewDecoder(r.Body).Decode(&got)
		if body != nil {
			body <- got
		}
		_, _ = w.Write([]byte(`{"id":"kid","title":"Go","url":"https://go.dev"}`))
	})
	mux.HandleFunc("POST /me/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		_ = json.NewDecoder(r.Body).Decode(&got)
		if body != nil {
			body <- got
		}
		_, _ = w.Write([]byte(`{"id":"new","parent_id":"root1","title":"New","url":"https://new.example"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

const threeLevelTree = `[
	{"id":"root1","parent_id":null,"is_folder":true,"title":"Toolbar","position":0},
	{"id":"dev","parent_id":"root1","is_folder":true,"title":"Dev","position":0},
	{"id":"kid","parent_id":"dev","is_folder":false,"title":"Go","url":"https://go.dev","position":1}
]`

func TestBookmarksListComputesPath(t *testing.T) {
	res := callTool(t, bookmarkAPI(t, threeLevelTree, nil), "dd_bookmarks_list", map[string]any{})
	var out bookmarksListOutput
	if err := json.Unmarshal(mustStructured(t, res), &out); err != nil {
		t.Fatalf("decode: %v (%s)", err, contentText(res))
	}
	if out.Total != 3 || out.Truncated {
		t.Fatalf("total=%d truncated=%v", out.Total, out.Truncated)
	}
	var leaf *bookmarkOut
	for i := range out.Bookmarks {
		if out.Bookmarks[i].ID == "kid" {
			leaf = &out.Bookmarks[i]
		}
	}
	if leaf == nil {
		t.Fatalf("leaf missing: %+v", out.Bookmarks)
	}
	if leaf.Path != "Toolbar/Dev" {
		t.Fatalf("path = %q, want Toolbar/Dev", leaf.Path)
	}
	if leaf.ParentID != "dev" {
		t.Fatalf("parent = %q", leaf.ParentID)
	}
}

func TestBookmarksListTruncates(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`[`)
	for i := 0; i < 300; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id":"b%d","parent_id":null,"is_folder":false,"title":"item %d","url":"https://e.example/%d"}`, i, i, i)
	}
	sb.WriteString(`]`)

	res := callTool(t, bookmarkAPI(t, sb.String(), nil), "dd_bookmarks_list", map[string]any{})
	var out bookmarksListOutput
	if err := json.Unmarshal(mustStructured(t, res), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Bookmarks) != defaultBookmarkLimit {
		t.Fatalf("returned %d, want the default cap %d", len(out.Bookmarks), defaultBookmarkLimit)
	}
	if out.Total != 300 || !out.Truncated {
		t.Fatalf("total=%d truncated=%v", out.Total, out.Truncated)
	}
}

func TestBookmarksListFiltersByQuery(t *testing.T) {
	res := callTool(t, bookmarkAPI(t, threeLevelTree, nil), "dd_bookmarks_list", map[string]any{"q": "go.dev"})
	var out bookmarksListOutput
	if err := json.Unmarshal(mustStructured(t, res), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Total != 1 || out.Bookmarks[0].ID != "kid" {
		t.Fatalf("query must match the url only: %+v", out)
	}
}

// The "root" sentinel must reach the server as an empty parent_id.
func TestBookmarksUpdateRootSentinel(t *testing.T) {
	body := make(chan map[string]any, 1)
	api := bookmarkAPI(t, threeLevelTree, body)
	callToolAllow(t, api, "dd_bookmarks_update", map[string]any{"id": "kid", "parent_id": "root"}, true)

	got := <-body
	v, ok := got["parent_id"]
	if !ok || v != "" {
		t.Fatalf(`parent_id "root" must be sent as "", got %v`, got)
	}
	if _, ok := got["title"]; ok {
		t.Fatalf("an untouched title must not be sent: %v", got)
	}
}

// A new bookmark lands after its siblings instead of colliding at position 0.
func TestBookmarksAddPicksNextPosition(t *testing.T) {
	body := make(chan map[string]any, 1)
	api := bookmarkAPI(t, threeLevelTree, body)
	callTool(t, api, "dd_bookmarks_add", map[string]any{
		"title": "New", "url": "https://new.example", "parent_id": "dev",
	})
	got := <-body
	if got["position"] != float64(2) {
		t.Fatalf("position = %v, want max+1 = 2", got["position"])
	}
}

func TestNewToolsHaveDescriptions(t *testing.T) {
	for _, sp := range slices.Concat(bookmarkToolSpecs(), savedToolSpecs()) {
		if strings.TrimSpace(sp.Description) == "" || strings.HasPrefix(sp.Description, "tool.") {
			t.Fatalf("tool %q has no description (missing i18n key?)", sp.Name)
		}
	}
}
