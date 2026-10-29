package cli

import (
	"bytes"
	"fmt"
	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestReviewFailedGetPreservesDestination(t *testing.T) {
	base := filesMuxCLI(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/files/f1/content" {
			http.Error(w, "failed", 500)
			return
		}
		base.ServeHTTP(w, r)
	}))
	defer srv.Close()
	setupProfile(t, srv.URL)
	dest := filepath.Join(t.TempDir(), "existing.txt")
	os.WriteFile(dest, []byte("precious"), 0600)
	var out, errb bytes.Buffer
	code := Run([]string{"get", "docs/a.txt", dest}, &out, &errb)
	if code == 0 {
		t.Fatal("expected download failure")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "precious" {
		t.Fatalf("existing destination lost: %q", got)
	}
}
func TestReviewFailedBookPreservesDestination(t *testing.T) {
	srv := httptest.NewServer(filesMuxCLI(t))
	defer srv.Close()
	setupProfile(t, srv.URL)
	dest := filepath.Join(t.TempDir(), "existing.epub")
	os.WriteFile(dest, []byte("precious"), 0600)
	var out, errb bytes.Buffer
	code := Run([]string{"books", "get", "missing", "--out", dest}, &out, &errb)
	if code == 0 {
		t.Fatal("expected download failure")
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "precious" {
		t.Fatalf("existing destination lost: %q (%v)", got, err)
	}
}
func TestReviewListPreservesConcurrentJournalAppend(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"token":"jwt"}`) })
	mux.HandleFunc("GET /me/saved/old", func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(404)
		io.WriteString(w, `{"error":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)
	config, _ := ddclient.ConfigPath()
	cc := &cmdCtx{ConfigPath: config, Stdout: io.Discard, Stderr: io.Discard}
	if err := rememberDownload(cc, downloadEntry{ID: "old"}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- downloadList(cc) }()
	<-entered
	err := rememberDownload(cc, downloadEntry{ID: "new"})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for _, e := range loadDownloads(cc) {
		if e.ID == "new" {
			return
		}
	}
	t.Fatal("download list erased concurrently appended download")
}

func TestReviewMoveUsesOneAtomicRequest(t *testing.T) {
	for _, status := range []int{200, 409, 404} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			base := filesMuxCLI(t)
			mutations := []string{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch {
					mutations = append(mutations, r.URL.Path)
					if r.URL.Path != "/files/f1/relocate" {
						http.Error(w, "unexpected non-atomic operation", 500)
						return
					}
					w.WriteHeader(status)
					io.WriteString(w, `{"id":"f1","name":"new.txt"}`)
					return
				}
				base.ServeHTTP(w, r)
			}))
			defer srv.Close()
			setupProfile(t, srv.URL)
			var out, errb bytes.Buffer
			code := Run([]string{"mv", "docs/a.txt", "new.txt"}, &out, &errb)
			if (code == 0) != (status == 200) {
				t.Fatalf("code=%d status=%d stderr=%s", code, status, &errb)
			}
			if len(mutations) != 1 || mutations[0] != "/files/f1/relocate" {
				t.Fatalf("mutations=%v", mutations)
			}
		})
	}
}

func TestDownloadCommandsPreserveDestinationOnInterruptedBody(t *testing.T) {
	for _, cmd := range [][]string{{"get", "docs/a.txt"}, {"books", "get", "b1", "--out"}, {"pocket", "read", "a1", "--out"}} {
		t.Run(cmd[0], func(t *testing.T) {
			base := filesMuxCLI(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/files/f1/content" || r.URL.Path == "/me/ebooks/library/b1/download" || r.URL.Path == "/me/saved/a1/content" {
					w.Header().Set("Content-Length", "100")
					io.WriteString(w, "partial")
					return
				}
				base.ServeHTTP(w, r)
			}))
			defer srv.Close()
			setupProfile(t, srv.URL)
			dir := t.TempDir()
			dest := filepath.Join(dir, "existing")
			if err := os.WriteFile(dest, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			var out, errb bytes.Buffer
			args := append(append([]string{}, cmd...), dest)
			if code := Run(args, &out, &errb); code == 0 {
				t.Fatal("expected interrupted download to fail")
			}
			got, err := os.ReadFile(dest)
			if err != nil || string(got) != "original" {
				t.Fatalf("destination=%q err=%v", got, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary files leaked: %v %v", entries, err)
			}
		})
	}
}

func TestJournalConcurrentProcesses(t *testing.T) {
	if path := os.Getenv("DD_TEST_JOURNAL_CONFIG"); path != "" {
		cc := &cmdCtx{ConfigPath: path}
		if err := rememberDownload(cc, downloadEntry{ID: os.Getenv("DD_TEST_JOURNAL_ID")}); err != nil {
			t.Fatal(err)
		}
		return
	}
	config := filepath.Join(t.TempDir(), "cli.json")
	const count = 8
	cmds := make([]*exec.Cmd, count)
	logs := make([]bytes.Buffer, count)
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestJournalConcurrentProcesses$")
		cmds[i].Env = append(os.Environ(), "DD_TEST_JOURNAL_CONFIG="+config, fmt.Sprintf("DD_TEST_JOURNAL_ID=%d", i))
		cmds[i].Stdout = &logs[i]
		cmds[i].Stderr = &logs[i]
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for i, c := range cmds {
		if err := c.Wait(); err != nil {
			t.Fatalf("child: %v %s", err, &logs[i])
		}
	}
	got := loadDownloads(&cmdCtx{ConfigPath: config})
	if len(got) != count {
		t.Fatalf("journal retained %d of %d additions", len(got), count)
	}
}
