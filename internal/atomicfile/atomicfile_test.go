package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeString(s string) func(io.Writer) error {
	return func(w io.Writer) error { _, err := io.WriteString(w, s); return err }
}

func TestWritePreservesPermissions(t *testing.T) {
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
		if err := Write(path, writeString("new")); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Errorf("mode=%o want=%o", info.Mode().Perm(), mode)
		}
		err = Write(path, func(w io.Writer) error { io.WriteString(w, "partial"); return errors.New("interrupted") })
		if err == nil {
			t.Fatal("expected error")
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "new" {
			t.Fatalf("data=%q err=%v", data, err)
		}
	}
}

func TestWriteNewCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new")
	if err := WriteNew(path, writeString("data")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "data" {
		t.Fatalf("got %q", b)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("temporary file left behind: %v", entries)
	}
}

func TestWriteNewNeverReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(path, []byte("old"), 0o600)
	if err := WriteNew(path, writeString("new")); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "old" {
		t.Fatalf("replaced: %q", b)
	}
}

// A file that appears while the bytes are being written is not replaced either.
func TestWriteNewRaceWithAppearingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	err := WriteNew(path, func(w io.Writer) error {
		if err := os.WriteFile(path, []byte("theirs"), 0o600); err != nil {
			return err
		}
		_, err := io.WriteString(w, "ours")
		return err
	})
	if !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "theirs" {
		t.Fatalf("replaced: %q", b)
	}
}
