package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

// downloadMuxCLI stubs the download endpoints. items maps an id to its JSON;
// an id that is missing answers 404, the way the server forgets old downloads.
func downloadMuxCLI(t *testing.T, items map[string]string, body chan map[string]any) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("POST /me/saved", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		_ = json.NewDecoder(r.Body).Decode(&got)
		if body != nil {
			body <- got
		}
		_, _ = w.Write([]byte(`{"id":"d1","kind":"download","status":"pending",
			"url":"https://example.org/f.iso","title":"f.iso"}`))
	})
	mux.HandleFunc("GET /me/saved/{id}", func(w http.ResponseWriter, r *http.Request) {
		item, ok := items[r.PathValue("id")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		_, _ = w.Write([]byte(item))
	})
	return mux
}

func journalFile(t *testing.T) string {
	t.Helper()
	p, err := ddclient.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(filepath.Dir(p), "downloads.json")
}

func readJournal(t *testing.T) []downloadEntry {
	t.Helper()
	b, err := os.ReadFile(journalFile(t))
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	var out []downloadEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDownloadAddRecordsTheJournal(t *testing.T) {
	body := make(chan map[string]any, 1)
	srv := httptest.NewServer(downloadMuxCLI(t, nil, body))
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, errb, code := runCLI(t, "download", "add", "https://example.org/f.iso")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	if got := <-body; got["kind"] != ddclient.SavedKindDownload {
		t.Fatalf("kind = %v", got["kind"])
	}
	if !strings.Contains(out, "d1") {
		t.Fatalf("the id must be printed, it is the only handle on a download:\n%s", out)
	}
	entries := readJournal(t)
	if len(entries) != 1 || entries[0].ID != "d1" || entries[0].At == "" {
		t.Fatalf("journal = %+v", entries)
	}
}

// The server drops finished downloads after a while; a 404 prunes the entry
// instead of failing the whole listing.
func TestDownloadListPrunesVanishedEntries(t *testing.T) {
	items := map[string]string{
		"d2": `{"id":"d2","kind":"download","status":"processing",
			"url":"https://example.org/b.iso","bytes_done":524288,"size_bytes":1048576}`,
	}
	srv := httptest.NewServer(downloadMuxCLI(t, items, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	cc := &cmdCtx{}
	if p, err := ddclient.ConfigPath(); err == nil {
		cc.ConfigPath = p
	}
	if err := saveDownloads(cc, []downloadEntry{{ID: "gone"}, {ID: "d2"}}); err != nil {
		t.Fatal(err)
	}

	out, errb, code := runCLI(t, "download", "list")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	if strings.Contains(out, "gone") || !strings.Contains(out, "d2") {
		t.Fatalf("out:\n%s", out)
	}
	if !strings.Contains(out, "50%") {
		t.Fatalf("progress must be shown when the size is known:\n%s", out)
	}
	if entries := readJournal(t); len(entries) != 1 || entries[0].ID != "d2" {
		t.Fatalf("the vanished entry must be pruned, journal = %+v", entries)
	}
}

// A chunked response has no size; the progress column must not divide by zero.
func TestDownloadStatusWithoutSize(t *testing.T) {
	items := map[string]string{
		"d3": `{"id":"d3","kind":"download","status":"processing",
			"url":"https://example.org/c.iso","bytes_done":2048,"size_bytes":null}`,
	}
	srv := httptest.NewServer(downloadMuxCLI(t, items, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, errb, code := runCLI(t, "download", "status", "d3")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb)
	}
	if !strings.Contains(out, "2.0 KiB") {
		t.Fatalf("want the bytes fetched so far:\n%s", out)
	}
}

func TestDownloadListEmptyJournal(t *testing.T) {
	srv := httptest.NewServer(downloadMuxCLI(t, nil, nil))
	defer srv.Close()
	setupProfile(t, srv.URL)

	out, _, code := runCLI(t, "download", "list")
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("an empty journal must say so, code=%d out=%q", code, out)
	}
	out, _, _ = runCLI(t, "--json", "download", "list")
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty --json = %q, want []", out)
	}
}

func TestJournalKeepsOnlyTheNewest(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, err := ddclient.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	cc := &cmdCtx{ConfigPath: p}
	for i := 0; i < journalLimit+10; i++ {
		if err := rememberDownload(cc, downloadEntry{ID: string(rune('a' + i%26))}); err != nil {
			t.Fatal(err)
		}
	}
	// ids repeat every 26, so dedup by id caps it well below the limit
	if got := len(loadDownloads(cc)); got != 26 {
		t.Fatalf("journal has %d entries, want 26 distinct ids", got)
	}

	var many []downloadEntry
	for i := 0; i < journalLimit+10; i++ {
		many = append(many, downloadEntry{ID: strings.Repeat("x", i+1)})
	}
	if err := saveDownloads(cc, many); err != nil {
		t.Fatal(err)
	}
	if got := len(loadDownloads(cc)); got != journalLimit {
		t.Fatalf("journal kept %d entries, want %d", got, journalLimit)
	}
}

// A corrupt journal must not make the command fail: the ids are recoverable,
// the ability to queue a download is not.
func TestCorruptJournalIsIgnored(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, err := ddclient.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	cc := &cmdCtx{ConfigPath: p}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(p), "downloads.json"), []byte("{oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadDownloads(cc); got != nil {
		t.Fatalf("a corrupt journal must read as empty, got %+v", got)
	}
}

func TestJournalIsPrivate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, err := ddclient.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	cc := &cmdCtx{ConfigPath: p}
	if err := rememberDownload(cc, downloadEntry{ID: "d1", URL: "https://example.org/f.iso"}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(filepath.Dir(p), "downloads.json"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600 — the urls are the user's browsing history", st.Mode().Perm())
	}
}

// setupTwoProfiles saves profiles "a" (default) and "b", each on its own server.
func setupTwoProfiles(t *testing.T, urlA, urlB string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, _ := ddclient.ConfigPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	cfg := ddclient.Config{}
	cfg.Upsert(ddclient.Profile{Name: "a", ServerURL: urlA, DeviceToken: "kfd"}, true)
	cfg.Upsert(ddclient.Profile{Name: "b", ServerURL: urlB, DeviceToken: "kfd"}, false)
	if err := cfg.Save(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func journalIDs(t *testing.T) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for _, e := range readJournal(t) {
		ids[e.ID] = true
	}
	return ids
}

// Each profile talks to its own server, so another profile's download ids are
// unknown here and 404: listing under one profile must neither show nor prune
// the entries another profile queued.
func TestDownloadListKeepsOtherProfilesEntries(t *testing.T) {
	srvA := httptest.NewServer(downloadMuxCLI(t, map[string]string{
		"d1": `{"id":"d1","kind":"download","status":"processing","url":"https://example.org/a.iso"}`,
	}, nil))
	defer srvA.Close()
	muxB := http.NewServeMux()
	muxB.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	muxB.HandleFunc("POST /me/saved", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"b1","kind":"download","status":"pending","url":"https://example.org/b.iso"}`))
	})
	muxB.HandleFunc("GET /me/saved/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "b1" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"b1","kind":"download","status":"processing","url":"https://example.org/b.iso"}`))
	})
	srvB := httptest.NewServer(muxB)
	defer srvB.Close()
	setupTwoProfiles(t, srvA.URL, srvB.URL)

	if _, errb, code := runCLI(t, "download", "add", "https://example.org/a.iso"); code != 0 {
		t.Fatalf("add a: %s", errb)
	}
	if _, errb, code := runCLI(t, "--profile", "b", "download", "add", "https://example.org/b.iso"); code != 0 {
		t.Fatalf("add b: %s", errb)
	}
	out, errb, code := runCLI(t, "download", "list")
	if code != 0 {
		t.Fatalf("list a: %s", errb)
	}
	if !strings.Contains(out, "d1") || strings.Contains(out, "b1") {
		t.Fatalf("profile a must list only its own downloads:\n%s", out)
	}
	if ids := journalIDs(t); !ids["d1"] || !ids["b1"] {
		t.Fatalf("listing under a pruned another profile's entry: %v", ids)
	}
	out, errb, code = runCLI(t, "--profile", "b", "download", "list")
	if code != 0 || !strings.Contains(out, "b1") || strings.Contains(out, "d1") {
		t.Fatalf("list b: code=%d err=%s out:\n%s", code, errb, out)
	}
	if ids := journalIDs(t); !ids["d1"] || !ids["b1"] {
		t.Fatalf("listing under b pruned another profile's entry: %v", ids)
	}
}

// A journal written before entries carried their profile cannot say whose an id
// is once several profiles exist: a 404 then proves nothing and must not prune.
// An id the current server does know is adopted by the current profile.
func TestDownloadListKeepsUntaggedEntriesWithSeveralProfiles(t *testing.T) {
	srvA := httptest.NewServer(downloadMuxCLI(t, map[string]string{
		"d2": `{"id":"d2","kind":"download","status":"processing","url":"https://example.org/a.iso"}`,
	}, nil))
	defer srvA.Close()
	p := setupTwoProfiles(t, srvA.URL, "http://127.0.0.1:1")
	legacy := `[{"id":"other"},{"id":"d2"}]`
	if err := os.WriteFile(filepath.Join(filepath.Dir(p), "downloads.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errb, code := runCLI(t, "download", "list")
	if code != 0 || !strings.Contains(out, "d2") || strings.Contains(out, "other") {
		t.Fatalf("code=%d err=%s out:\n%s", code, errb, out)
	}
	for _, e := range readJournal(t) {
		switch e.ID {
		case "other":
			if e.Profile != "" {
				t.Fatalf("an unknown legacy id must stay untagged: %+v", e)
			}
		case "d2":
			if e.Profile != "a" {
				t.Fatalf("a legacy id this server knows must be adopted by a: %+v", e)
			}
		}
	}
	if ids := journalIDs(t); !ids["other"] || !ids["d2"] {
		t.Fatalf("journal = %v, want both kept", ids)
	}
}

// Only the server's own "not found" for the id prunes: a 404 without its JSON
// error body (a proxy, or a server without the endpoint) says nothing about the id.
func TestDownloadListKeepsEntriesOnForeign404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	srv := httptest.NewServer(mux) // no /me/saved route: the mux answers a plain 404
	defer srv.Close()
	setupProfile(t, srv.URL)
	cc := &cmdCtx{}
	if p, err := ddclient.ConfigPath(); err == nil {
		cc.ConfigPath = p
	}
	if err := rememberDownload(cc, downloadEntry{ID: "d7"}); err != nil {
		t.Fatal(err)
	}
	_, _, _ = runCLI(t, "download", "list")
	if ids := journalIDs(t); !ids["d7"] {
		t.Fatalf("a foreign 404 pruned the journal: %v", ids)
	}
}
