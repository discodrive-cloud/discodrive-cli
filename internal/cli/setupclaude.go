package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/discodrive-cloud/discodrive-cli/internal/claudecfg"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// setupClaudeConfig finds dd-mcp next to the current binary (or in PATH) and registers it in
// all existing Claude configs. allowChanges enables the mutating tools. Returns the changed paths.
func setupClaudeConfig(allowChanges bool) ([]string, error) {
	binPath, err := ddmcpPath()
	if err != nil {
		return nil, err
	}
	return claudecfg.Setup(binPath, allowChanges)
}

// ddmcpPath looks for dd-mcp next to the current executable, otherwise in PATH.
func ddmcpPath() (string, error) {
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "dd-mcp")
		if _, err := os.Stat(cand); err == nil {
			return claudecfg.StablePath(cand), nil
		}
	}
	if p, err := exec.LookPath("dd-mcp"); err == nil {
		return p, nil
	}
	return "dd-mcp", nil // last resort: let Claude find it in PATH itself
}

func init() {
	register(&command{
		Name:    "setup-claude",
		Summary: "cmd.setup_claude.summary",
		Usage:   "cmd.setup_claude.usage",
		Long:    "cmd.setup_claude.long",
		Examples: []string{
			"cmd.setup_claude.example.0",
			"cmd.setup_claude.example.1",
		},
		Run: func(cc *cmdCtx, args []string) error {
			fs := newFlagSet("setup-claude", cc)
			allow := fs.Bool("allow-changes", false, i18n.T("cmd.setup_claude.flag.allow_changes"))
			if err := fs.Parse(args); err != nil {
				return err
			}
			changed, err := setupClaudeConfig(*allow)
			if err != nil {
				return err
			}
			if len(changed) == 0 {
				fmt.Fprintln(cc.Stdout, i18n.T("cmd.setup_claude.msg.none_found"))
				return nil
			}
			mode := i18n.T("cmd.setup_claude.msg.mode_readonly")
			if *allow {
				mode = i18n.T("cmd.setup_claude.msg.mode_changes")
			}
			for _, p := range changed {
				fmt.Fprint(cc.Stdout, i18n.T("cmd.setup_claude.msg.updated", i18n.M{"Path": p}))
			}
			fmt.Fprint(cc.Stdout, i18n.T("cmd.setup_claude.msg.mode_line", i18n.M{"Mode": mode}))
			return nil
		},
	})
}
