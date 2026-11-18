package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
