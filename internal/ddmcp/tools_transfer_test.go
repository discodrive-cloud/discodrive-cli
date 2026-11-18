package ddmcp

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeDrive is a stateful stub of the files API: a root holding the folder
// Downloads, plus whatever the test puts or uploads.
type fakeDrive struct {
	mu      sync.Mutex
	nodes   map[string]map[string]any // id → node JSON
	content map[string][]byte
	parent  map[string]string
	seq     int
	// badHash makes the listing report a wrong content_hash for every file.
	badHash bool
	// modified is the modified_at field of the last upload.
	modified string
	// onContent, if set, runs while a file's bytes are being served.
	onContent func()
}

func newFakeDrive(t *testing.T) (*fakeDrive, string) {
	t.Helper()
	d := &fakeDrive{nodes: map[string]map[string]any{}, content: map[string][]byte{}, parent: map[string]string{}}
	d.nodes["dl"] = map[string]any{"id": "dl", "name": "Downloads", "is_dir": true, "version": 1}
	d.parent["dl"] = ""

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		pid := r.URL.Query().Get("parent_id")
		out := []map[string]any{}
		for id, n := range d.nodes {
			if d.parent[id] == pid {
				out = append(out, d.view(id, n))
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /files/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		b, hook := d.content[r.PathValue("id")], d.onContent
		d.mu.Unlock()
		if hook != nil {
			hook()
		}
		_, _ = w.Write(b)
	})
	mux.HandleFunc("POST /files/upload", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		b, _ := io.ReadAll(f)
		d.mu.Lock()
		defer d.mu.Unlock()
		d.modified = r.FormValue("modified_at")
		id := d.put(r.FormValue("parent_id"), r.FormValue("name"), b)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"node": d.view(id, d.nodes[id])})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return d, srv.URL
}

// put stores a file, replacing one of the same name in the same folder. Caller holds mu.
func (d *fakeDrive) put(parentID, name string, b []byte) string {
	for id, n := range d.nodes {
		if d.parent[id] == parentID && n["name"] == name {
			delete(d.nodes, id)
		}
	}
	d.seq++
	id := fmt.Sprintf("f%d", d.seq)
	d.nodes[id] = map[string]any{"id": id, "name": name, "is_dir": false, "version": 1}
	d.parent[id] = parentID
	d.content[id] = b
	return id
}

func (d *fakeDrive) view(id string, n map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range n {
		out[k] = v
	}
	if b, ok := d.content[id]; ok {
		sum := sha256.Sum256(b)
		out["size"] = len(b)
		out["content_hash"] = hex.EncodeToString(sum[:])
		if d.badHash {
			out["content_hash"] = strings.Repeat("0", 64)
		}
	}
	return out
}

func (d *fakeDrive) file(parentID, name string) ([]byte, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, n := range d.nodes {
		if d.parent[id] == parentID && n["name"] == name {
			return d.content[id], true
		}
	}
	return nil, false
}

func (d *fakeDrive) seed(parentID, name string, b []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.put(parentID, name, b)
}

// connect runs a full MCP handshake with buildServer(o) over in-memory transports.
func connect(t *testing.T, apiURL string, o serverOptions) *mcp.ClientSession {
	t.Helper()
	srvT, cliT := mcp.NewInMemoryTransports()
	s := buildServer(ddclient.New(apiURL, "tok"), o)
	ss, err := s.Connect(t.Context(), srvT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(t.Context(), cliT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// call calls a tool and returns its result; a protocol-level error (e.g. schema
// validation) is folded into an error result so tests can treat both alike.
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
	}
	return res
}

func wantToolError(t *testing.T, res *mcp.CallToolResult, fragments ...string) {
	t.Helper()
	if !res.IsError {
		t.Fatalf("expected an error, got success: %v", res.StructuredContent)
	}
	msg := contentText(res)
	for _, f := range fragments {
		if !strings.Contains(msg, f) {
			t.Errorf("error %q does not mention %q", msg, f)
		}
	}
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[0] = 0 // surely not text
	return b
}

func hexSum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// --- instructions ---

func TestInstructionsDependOnMode(t *testing.T) {
	_, api := newFakeDrive(t)
	cases := []struct {
		name              string
		o                 serverOptions
		changesOff, local bool
		remote            bool
	}{
		{"stdio default", stdioOptions(false), true, false, false},
		{"stdio allow-changes", stdioOptions(true), false, true, false},
		{"http default", httpOptions(false), true, false, true},
		{"http allow-changes", httpOptions(true), false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connect(t, api, tc.o).InitializeResult().Instructions
			has := func(s string) bool { return strings.Contains(got, s) }
			if !has("dd-cli put") || !has("dd-cli get") {
				t.Errorf("instructions must name dd-cli put/get:\n%s", got)
			}
			if off := has("without --allow-changes"); off != tc.changesOff {
				t.Errorf("block about switched-off tools present=%v, want %v:\n%s", off, tc.changesOff, got)
			}
			// The block on moving files through the tools themselves.
			if local := has("absolute path on this machine"); local != tc.local {
				t.Errorf("local transfer block present=%v, want %v:\n%s", local, tc.local, got)
			}
			if tc.remote {
				// setup-claude edits the local client config, not a running dd-mcp --http;
				// and dd-cli need not be where the user is.
				for _, banned := range []string{"setup-claude", "local_path", "installed together"} {
					if has(banned) {
						t.Errorf("HTTP instructions must not mention %q:\n%s", banned, got)
					}
				}
				if tc.changesOff && !has("dd-mcp --http") {
					t.Errorf("HTTP instructions must say the server is restarted with --allow-changes:\n%s", got)
				}
			} else if tc.changesOff && (!has("setup-claude") || !has("local_path")) {
				t.Errorf("stdio instructions must name setup-claude and what enabling adds (local_path):\n%s", got)
			}
			if lines := strings.Count(got, "\n") + 1; lines > 15 {
				t.Errorf("instructions are %d lines; keep them short", lines)
			}
		})
	}
}

// --- tool set per transport ---

func TestHTTPTransportHasNoLocalFileAccess(t *testing.T) {
	_, api := newFakeDrive(t)
	cs := connect(t, api, httpOptions(true))
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "dd_files_download" {
			t.Fatal("dd_files_download must not be offered over HTTP")
		}
		if tool.Name == "dd_files_upload" {
			schema, _ := json.Marshal(tool.InputSchema)
			if bytes.Contains(schema, []byte("local_path")) || strings.Contains(tool.Description, "local_path") {
				t.Fatalf("dd_files_upload over HTTP must not offer local_path: %s / %s", tool.Description, schema)
			}
		}
	}

	local := filepath.Join(t.TempDir(), "secret.txt")
	_ = os.WriteFile(local, []byte("secret"), 0o600)
	wantToolError(t, call(t, cs, "dd_files_upload", map[string]any{"path": "Downloads/x", "local_path": local}))

	// Even if the schema let it through, the handler refuses.
	_, err = upload(t.Context(), ddclient.New(api, "tok"), httpOptions(true),
		uploadInput{Path: "Downloads/x", LocalPath: local})
	if err == nil || !strings.Contains(err.Error(), "HTTP") {
		t.Fatalf("upload with local_path over HTTP: err=%v", err)
	}
}

func TestStdioOffersLocalPath(t *testing.T) {
	_, api := newFakeDrive(t)
	res, err := connect(t, api, stdioOptions(true)).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var upload, download bool
	for _, tool := range res.Tools {
		schema, _ := json.Marshal(tool.InputSchema)
		switch tool.Name {
		case "dd_files_upload":
			upload = bytes.Contains(schema, []byte("local_path")) && strings.Contains(tool.Description, "local_path")
		case "dd_files_download":
			download = bytes.Contains(schema, []byte("overwrite"))
		}
	}
	if !upload || !download {
		t.Fatalf("stdio with --allow-changes: upload local_path=%v, download=%v", upload, download)
	}
}

// --- upload ---

func TestUploadLocalBinaryIntoExistingFolder(t *testing.T) {
	d, api := newFakeDrive(t)
	data := randomBytes(t, 8<<20) // binary, and well past the 1 MiB read limit
	local := filepath.Join(t.TempDir(), "test.bin")
	if err := os.WriteFile(local, data, 0o600); err != nil {
		t.Fatal(err)
	}

	res := call(t, connect(t, api, stdioOptions(true)), "dd_files_upload", map[string]any{"path": "Downloads", "local_path": local})
	var out nodeOut
	if err := json.Unmarshal(mustStructured(t, res), &out); err != nil {
		t.Fatal(err)
	}
	got, ok := d.file("dl", "test.bin")
	if !ok || !bytes.Equal(got, data) {
		t.Fatalf("stored %d bytes (found=%v), want the %d local bytes intact", len(got), ok, len(data))
	}
	if out.Name != "test.bin" || out.ContentHash != hexSum(data) || out.Size == nil || *out.Size != int64(len(data)) {
		t.Fatalf("result = %+v", out)
	}
	if d.modified == "" {
		t.Error("the local file's modification time was not sent")
	}
}

func TestUploadLocalUnderNewName(t *testing.T) {
	d, api := newFakeDrive(t)
	local := filepath.Join(t.TempDir(), "app.apk")
	_ = os.WriteFile(local, []byte{0, 1, 2}, 0o600)

	res := call(t, connect(t, api, stdioOptions(true)), "dd_files_upload", map[string]any{"path": "Downloads/renamed.apk", "local_path": local})
	mustStructured(t, res)
	if _, ok := d.file("dl", "renamed.apk"); !ok {
		t.Fatal("expected Downloads/renamed.apk")
	}
}

func TestUploadTextStillWorks(t *testing.T) {
	d, api := newFakeDrive(t)
	cs := connect(t, api, stdioOptions(true))
	mustStructured(t, call(t, cs, "dd_files_upload", map[string]any{"path": "Downloads/note.md", "content": "# hi"}))
	if got, _ := d.file("dl", "note.md"); string(got) != "# hi" {
		t.Fatalf("stored %q", got)
	}
	// An empty text file is a legitimate upload.
	mustStructured(t, call(t, cs, "dd_files_upload", map[string]any{"path": "Downloads/empty.txt", "content": ""}))
	if _, ok := d.file("dl", "empty.txt"); !ok {
		t.Fatal("empty file not uploaded")
	}
}

func TestUploadArgumentErrorsSayWhatToPass(t *testing.T) {
	_, api := newFakeDrive(t)
	cs := connect(t, api, stdioOptions(true))
	local := filepath.Join(t.TempDir(), "a.bin")
	_ = os.WriteFile(local, []byte{0}, 0o600)

	wantToolError(t, call(t, cs, "dd_files_upload", map[string]any{"path": "Downloads/a"}), "content", "local_path")
	wantToolError(t, call(t, cs, "dd_files_upload", map[string]any{"path": "Downloads/a", "content": "x", "local_path": local}), "exactly one")
	wantToolError(t, call(t, cs, "dd_files_upload", map[string]any{"path": "Downloads", "local_path": "a.bin"}), "absolute")
	wantToolError(t, call(t, cs, "dd_files_upload", map[string]any{"path": "Downloads", "local_path": t.TempDir()}), "folder")
	wantToolError(t, call(t, cs, "dd_files_upload", map[string]any{"path": "Downloads", "content": "x"}), "file name")

	// Over HTTP the only option is content, and the error says so.
	http := connect(t, api, httpOptions(true))
	wantToolError(t, call(t, http, "dd_files_upload", map[string]any{"path": "Downloads/a"}), "content", "dd-cli put")
}

// --- download ---

func TestDownloadBinaryIntoFolder(t *testing.T) {
	d, api := newFakeDrive(t)
	data := randomBytes(t, 3<<20)
	d.seed("dl", "test.bin", data)
	dir := t.TempDir()

	res := call(t, connect(t, api, stdioOptions(true)), "dd_files_download", map[string]any{"path": "Downloads/test.bin", "local_path": dir})
	var out downloadOutput
	if err := json.Unmarshal(mustStructured(t, res), &out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "test.bin"))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("saved %d bytes, err=%v", len(got), err)
	}
	if out.LocalPath != filepath.Join(dir, "test.bin") || out.Size != int64(len(data)) || out.ContentHash != hexSum(data) {
		t.Fatalf("result = %+v", out)
	}
}

func TestDownloadOverwriteOnlyWhenAsked(t *testing.T) {
	d, api := newFakeDrive(t)
	d.seed("dl", "a.bin", []byte("new"))
	dst := filepath.Join(t.TempDir(), "a.bin")
	_ = os.WriteFile(dst, []byte("old"), 0o600)
	cs := connect(t, api, stdioOptions(true))

	wantToolError(t, call(t, cs, "dd_files_download", map[string]any{"path": "Downloads/a.bin", "local_path": dst}), "overwrite")
	if b, _ := os.ReadFile(dst); string(b) != "old" {
		t.Fatalf("existing file changed without overwrite: %q", b)
	}
	mustStructured(t, call(t, cs, "dd_files_download", map[string]any{"path": "Downloads/a.bin", "local_path": dst, "overwrite": true}))
	if b, _ := os.ReadFile(dst); string(b) != "new" {
		t.Fatalf("overwrite: %q", b)
	}
}

func TestDownloadHashMismatchSavesNothing(t *testing.T) {
	d, api := newFakeDrive(t)
	d.seed("dl", "a.bin", []byte("data"))
	d.badHash = true
	dir := t.TempDir()

	wantToolError(t, call(t, connect(t, api, stdioOptions(true)), "dd_files_download",
		map[string]any{"path": "Downloads/a.bin", "local_path": filepath.Join(dir, "a.bin")}), "hash")
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("left behind: %v", entries)
	}
}

func TestDownloadErrors(t *testing.T) {
	_, api := newFakeDrive(t)
	cs := connect(t, api, stdioOptions(true))
	wantToolError(t, call(t, cs, "dd_files_download", map[string]any{"path": "Downloads", "local_path": t.TempDir()}), "folder")
	wantToolError(t, call(t, cs, "dd_files_download", map[string]any{"path": "Downloads/x", "local_path": "x"}), "absolute")
}

// --- dd_files_read points the way out ---

func TestReadBinaryOrLargeNamesTheWayOut(t *testing.T) {
	d, api := newFakeDrive(t)
	d.seed("dl", "app.apk", []byte{0x50, 0x4b, 0, 0xff})
	d.seed("dl", "big.txt", bytes.Repeat([]byte("a"), maxFileReadBytes+1))

	cases := []struct {
		name string
		o    serverOptions
		want []string
	}{
		{"stdio allow-changes", stdioOptions(true), []string{"dd_files_download", "dd-cli get Downloads/"}},
		{"stdio default", stdioOptions(false), []string{"dd-cli get Downloads/", "--allow-changes"}},
		{"http", httpOptions(true), []string{"dd-cli get Downloads/"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := connect(t, api, tc.o)
			for _, file := range []string{"Downloads/app.apk", "Downloads/big.txt"} {
				res := call(t, cs, "dd_files_read", map[string]any{"path": file})
				wantToolError(t, res, tc.want...)
				if tc.o.LocalFiles == false && strings.Contains(contentText(res), "dd_files_download") {
					t.Errorf("over HTTP the hint must not point at dd_files_download: %s", contentText(res))
				}
			}
		})
	}
}

func TestReadTextStillWorks(t *testing.T) {
	d, api := newFakeDrive(t)
	d.seed("dl", "note.md", []byte("привет"))
	res := call(t, connect(t, api, stdioOptions(false)), "dd_files_read", map[string]any{"path": "Downloads/note.md"})
	var out readOutput
	if err := json.Unmarshal(mustStructured(t, res), &out); err != nil || out.Content != "привет" {
		t.Fatalf("read = %+v, err=%v", out, err)
	}
}

// --- review follow-ups ---

func TestIsLocalName(t *testing.T) {
	for _, bad := range []string{"..", ".", "", "a/b", `..\x`, `a\b`, "/abs"} {
		if isLocalName(bad) {
			t.Errorf("isLocalName(%q) = true", bad)
		}
	}
	for _, good := range []string{"a.bin", "..hidden", "a..b", "файл.txt"} {
		if !isLocalName(good) {
			t.Errorf("isLocalName(%q) = false", good)
		}
	}
}

func TestDownloadNeverLeavesTheFolder(t *testing.T) {
	d, api := newFakeDrive(t)
	for _, name := range []string{"..", ".", `..\x`} {
		d.seed("dl", name, []byte("evil"))
	}
	cs := connect(t, api, stdioOptions(true))
	base := t.TempDir()
	dir := filepath.Join(base, "inside")
	_ = os.Mkdir(dir, 0o700)
	for _, name := range []string{"..", ".", `..\x`} {
		res := call(t, cs, "dd_files_download", map[string]any{"path": "Downloads/" + name, "local_path": dir, "overwrite": true})
		wantToolError(t, res, "outside")
	}
	for _, d := range []string{base, dir} {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if e.Name() != "inside" {
				t.Errorf("%s: unexpected %s", d, e.Name())
			}
		}
	}
}

// A file that appears at the destination while the download runs is not replaced.
func TestDownloadDoesNotReplaceFileThatAppears(t *testing.T) {
	d, api := newFakeDrive(t)
	d.seed("dl", "a.bin", []byte("ours"))
	dst := filepath.Join(t.TempDir(), "a.bin")
	d.onContent = func() { _ = os.WriteFile(dst, []byte("theirs"), 0o600) }

	wantToolError(t, call(t, connect(t, api, stdioOptions(true)), "dd_files_download",
		map[string]any{"path": "Downloads/a.bin", "local_path": dst}), "overwrite")
	if b, _ := os.ReadFile(dst); string(b) != "theirs" {
		t.Fatalf("replaced: %q", b)
	}
}

func TestDownloadUnwritableDestinationNamesTheWayOut(t *testing.T) {
	d, api := newFakeDrive(t)
	d.seed("dl", "a.bin", []byte("x"))
	dst := filepath.Join(t.TempDir(), "missing", "a.bin")
	wantToolError(t, call(t, connect(t, api, stdioOptions(true)), "dd_files_download",
		map[string]any{"path": "Downloads/a.bin", "local_path": dst}), "another local_path")
}

func TestUploadUnreadableLocalPathNamesTheWayOut(t *testing.T) {
	_, api := newFakeDrive(t)
	missing := filepath.Join(t.TempDir(), "nope.bin")
	wantToolError(t, call(t, connect(t, api, stdioOptions(true)), "dd_files_upload",
		map[string]any{"path": "Downloads", "local_path": missing}), "cannot read local_path", "check the path")
}

// Text in another encoding is refused like a binary file, and the error leads to a
// lossless download instead.
func TestReadNonUTF8LeadsToDownload(t *testing.T) {
	d, api := newFakeDrive(t)
	d.seed("dl", "cp1251.txt", []byte{0xcf, 0xf0, 0xe8, 0xe2, 0xe5, 0xf2}) // "Привет" in cp1251
	wantToolError(t, call(t, connect(t, api, stdioOptions(true)), "dd_files_read",
		map[string]any{"path": "Downloads/cp1251.txt"}), "not UTF-8", "dd_files_download")
}
