// Package claudecfg registers dd-mcp as an MCP server in the Claude Desktop/Code configs.
// Shared code for dd-cli (setup-claude, login) and dd-mcp (login) to avoid duplication.
package claudecfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Candidates returns the likely paths to the Claude Desktop/Code configs on the current OS.
func Candidates() []string {
	home, _ := os.UserHomeDir()
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
			filepath.Join(home, ".claude.json"),
		)
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			paths = append(paths, filepath.Join(appdata, "Claude", "claude_desktop_config.json"))
		}
		paths = append(paths, filepath.Join(home, ".claude.json"))
	default: // linux and others
		paths = append(paths,
			filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"),
			filepath.Join(home, ".claude.json"),
		)
	}
	return paths
}

// WriteEntry merges the "discodrive" MCP server entry (command: binPath) into an existing
// Claude JSON config without touching the other keys (including existing fields of the entry
// itself, e.g. env). allowChanges controls the argument: true → args:["--allow-changes"],
// false → args is removed. Creates the file/directory if missing.
func WriteEntry(path, binPath string, allowChanges bool) error {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	root := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("err.claude.config_parse", i18n.M{"Path": path}), err)
		}
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	// preserve the entry's other fields (e.g. env), change only command and args
	entry, _ := servers["discodrive"].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	entry["command"] = binPath
	if allowChanges {
		entry["args"] = []any{"--allow-changes"}
	} else {
		delete(entry, "args")
	}
	servers["discodrive"] = entry
	root["mcpServers"] = servers

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, b)
}

// writeAtomic replaces path via a temp file and rename: Claude rewrites these configs
// itself, and a truncate-then-write could leave a half-written file that loses all
// of the user's settings. A symlinked config (dotfile managers) is written through
// to its target rather than replaced by a regular file. An existing file keeps its
// permissions; a new one is private.
func writeAtomic(path string, b []byte) error {
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	mode := fs.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".claude-config-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// StablePath returns the PATH entry for the binary at p when it is the same file, else p.
// A package manager runs binaries from a versioned directory behind a PATH symlink
// (Homebrew: …/Cellar/discodrive-cli/1.2.3/bin), and on Linux os.Executable resolves
// that symlink; registering the versioned path would break on the next upgrade.
func StablePath(p string) string {
	found, err := exec.LookPath(filepath.Base(p))
	if err != nil {
		return p
	}
	if abs, err := filepath.Abs(found); err == nil {
		found = abs
	}
	a, errA := os.Stat(found)
	b, errB := os.Stat(p)
	if errA != nil || errB != nil || !os.SameFile(a, b) {
		return p
	}
	return found
}

// StdioSnippet returns an mcpServers JSON fragment for registering dd-mcp in stdio clients
// (Claude/Cursor/Zed and others) for manual pasting. It writes nothing to disk.
func StdioSnippet(binPath string, allowChanges bool) (string, error) {
	entry := map[string]any{"command": binPath}
	if allowChanges {
		entry["args"] = []any{"--allow-changes"}
	}
	root := map[string]any{"mcpServers": map[string]any{"discodrive": entry}}
	b, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Setup registers binPath as the "discodrive" MCP server in all existing Claude configs
// (plus always in ~/.claude.json). allowChanges enables/disables the mutating tools.
// Returns the list of changed paths.
func Setup(binPath string, allowChanges bool) ([]string, error) {
	var changed []string
	for _, p := range Candidates() {
		// write only to already existing configs, plus always to ~/.claude.json
		if _, statErr := os.Stat(p); statErr != nil && filepath.Base(p) != ".claude.json" {
			continue
		}
		if err := WriteEntry(p, binPath, allowChanges); err != nil {
			return changed, err
		}
		changed = append(changed, p)
	}
	return changed, nil
}
