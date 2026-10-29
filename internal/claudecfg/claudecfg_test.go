package claudecfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteEntryMergesWithoutClobbering(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "claude.json")
	// existing config with a different server — we must not clobber it entirely
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"other":{"command":"x"}}}`), 0o600)

	if err := WriteEntry(cfg, "/usr/local/bin/dd-mcp", false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(cfg)
	var parsed struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.MCPServers["discodrive"].Command != "/usr/local/bin/dd-mcp" {
		t.Fatalf("discodrive entry is missing: %+v", parsed.MCPServers)
	}
	if len(parsed.MCPServers["discodrive"].Args) != 0 {
		t.Fatalf("readonly: args must be absent, got %v", parsed.MCPServers["discodrive"].Args)
	}
	if _, ok := parsed.MCPServers["other"]; !ok {
		t.Fatal("existing entry was clobbered")
	}
}

func TestWriteEntryTogglesAllowChanges(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "claude.json")

	read := func() []string {
		b, _ := os.ReadFile(cfg)
		var parsed struct {
			MCPServers map[string]struct {
				Args []string `json:"args"`
			} `json:"mcpServers"`
		}
		_ = json.Unmarshal(b, &parsed)
		return parsed.MCPServers["discodrive"].Args
	}

	// enable
	if err := WriteEntry(cfg, "/bin/dd-mcp", true); err != nil {
		t.Fatal(err)
	}
	if args := read(); len(args) != 1 || args[0] != "--allow-changes" {
		t.Fatalf("expected [--allow-changes], got %v", args)
	}
	// disable — args should disappear
	if err := WriteEntry(cfg, "/bin/dd-mcp", false); err != nil {
		t.Fatal(err)
	}
	if args := read(); len(args) != 0 {
		t.Fatalf("after disabling, args must be empty, got %v", args)
	}
}

func TestStdioSnippet(t *testing.T) {
	ro, err := StdioSnippet("/bin/dd-mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ro, `"discodrive"`) || !strings.Contains(ro, `/bin/dd-mcp`) {
		t.Fatalf("expected content missing from the snippet:\n%s", ro)
	}
	if strings.Contains(ro, "allow-changes") {
		t.Fatalf("readonly snippet must not contain allow-changes:\n%s", ro)
	}
	rw, _ := StdioSnippet("/bin/dd-mcp", true)
	if !strings.Contains(rw, "--allow-changes") {
		t.Fatalf("rw snippet must contain --allow-changes:\n%s", rw)
	}
}

// The entry's other fields (e.g. env) must survive toggling the flag.
func TestWriteEntryPreservesEnv(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "claude.json")
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"discodrive":{"command":"/old","env":{"DISCODRIVE_TOKEN":"t"}}}}`), 0o600)

	if err := WriteEntry(cfg, "/new/dd-mcp", true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(cfg)
	var parsed struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	_ = json.Unmarshal(b, &parsed)
	dd := parsed.MCPServers["discodrive"]
	if dd.Command != "/new/dd-mcp" || len(dd.Args) != 1 || dd.Env["DISCODRIVE_TOKEN"] != "t" {
		t.Fatalf("env/command/args not preserved correctly: %+v", dd)
	}
}

// A symlinked config is updated through the link, keeping its permissions,
// and no temp files are left behind.
func TestWriteEntryAtomicThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "claude.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(`{"theme":"dark"}`), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".claude.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := WriteEntry(link, "/bin/dd-mcp", false); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink was replaced: %v %v", fi, err)
	}
	fi, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want 0640", fi.Mode().Perm())
	}
	b, _ := os.ReadFile(target)
	if !strings.Contains(string(b), `"theme": "dark"`) || !strings.Contains(string(b), "/bin/dd-mcp") {
		t.Fatalf("target content: %s", b)
	}
	for _, d := range []string{dir, filepath.Dir(target)} {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".tmp") {
				t.Fatalf("temp file left behind: %s", e.Name())
			}
		}
	}
}

// A new config is created private.
func TestWriteEntryNewFileIsPrivate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "claude_desktop_config.json")
	if err := WriteEntry(p, "/bin/dd-mcp", true); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", fi.Mode().Perm())
	}
}

// A binary reached through a PATH symlink is registered by its PATH entry, not by
// the versioned directory the symlink points into.
func TestStablePathPrefersPathSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("package-manager PATH symlinks are a macOS/Linux concern")
	}
	dir := t.TempDir()
	cellar := filepath.Join(dir, "Cellar", "1.2.3", "bin")
	bin := filepath.Join(dir, "bin")
	for _, d := range []string{cellar, bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	real := filepath.Join(cellar, "dd-mcp")
	if err := os.WriteFile(real, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(bin, "dd-mcp")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Setenv("PATH", bin)

	if got := StablePath(real); got != filepath.Join(bin, "dd-mcp") {
		t.Fatalf("StablePath = %q, want the PATH entry", got)
	}
	other := filepath.Join(dir, "elsewhere", "dd-mcp")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := StablePath(other); got != other {
		t.Fatalf("a different binary on PATH must not be substituted: %q", got)
	}
}
