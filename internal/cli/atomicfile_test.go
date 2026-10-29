package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicDownloadPreservesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	for _, mode := range []os.FileMode{0755, 0640, 0600} {
		path := filepath.Join(t.TempDir(), "existing")
		if err := os.WriteFile(path, []byte("old"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		if err := writeFileAtomic(path, func(w io.Writer) error { _, err := io.WriteString(w, "new"); return err }); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Errorf("mode=%o want=%o", info.Mode().Perm(), mode)
		}
		err = writeFileAtomic(path, func(w io.Writer) error { io.WriteString(w, "partial"); return errors.New("interrupted") })
		if err == nil {
			t.Fatal("expected error")
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "new" {
			t.Fatalf("data=%q err=%v", data, err)
		}
	}
}

func TestJournalRemainsPrivateWhenReplacingPermissiveFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	cc := &cmdCtx{ConfigPath: filepath.Join(t.TempDir(), "cli.json")}
	path, err := downloadsPath(cc)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err = saveDownloads(cc, []downloadEntry{{ID: "new"}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}
