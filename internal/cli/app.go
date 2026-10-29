package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/buildinfo"
	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Run is the CLI entry point. args excludes the program name. Returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	cc := &cmdCtx{Stdout: stdout, Stderr: stderr}
	if p, err := ddclient.ConfigPath(); err == nil {
		cc.ConfigPath = p
	}

	// Parse global flags that precede the subcommand.
	var rest []string
	var langFlag string
	wantHelp := false
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--json":
			cc.JSON = true
		case a == "--version":
			fmt.Fprintf(stdout, "dd-cli %s\n", buildinfo.Version())
			return 0
		case a == "--help" || a == "-h":
			wantHelp = true
		case a == "--profile":
			i++
			if i < len(args) {
				cc.Profile = args[i]
			}
		case strings.HasPrefix(a, "--profile="):
			cc.Profile = strings.TrimPrefix(a, "--profile=")
		case a == "--server":
			i++
			if i < len(args) {
				cc.Server = args[i]
			}
		case strings.HasPrefix(a, "--server="):
			cc.Server = strings.TrimPrefix(a, "--server=")
		case a == "--lang":
			i++
			if i < len(args) {
				langFlag = args[i]
			}
		case strings.HasPrefix(a, "--lang="):
			langFlag = strings.TrimPrefix(a, "--lang=")
		default:
			rest = args[i:]
			i = len(args)
			continue
		}
		i++
	}

	// Human output carries server-supplied text: filter terminal controls out of it.
	cc.RawStdout = stdout
	if !cc.JSON {
		cc.Stdout, cc.Stderr = safeWriter{stdout}, safeWriter{stderr}
	}

	// Resolve the language before any rendering: --lang → DISCODRIVE_LANG → LC_ALL → LANG → en.
	i18n.SetActive(i18n.Resolve(langFlag, os.Getenv))

	if len(rest) == 0 {
		printRootHelp(stdout)
		return 0
	}

	name := rest[0]
	cmdArgs := rest[1:]
	cmd := lookup(name)
	if cmd == nil {
		fmt.Fprintf(stderr, "%s\n\n", i18n.T("ui.unknown_command", i18n.M{"Name": name}))
		printRootHelp(stderr)
		return 2
	}

	// --help for the subcommand (the flag may appear at any position among the command's args).
	if wantHelp || hasHelpFlag(cmdArgs) {
		printCommandHelp(stdout, cmd)
		return 0
	}

	if err := cmd.Run(cc, cmdArgs); err != nil {
		writeRunErr(cc, err)
		return 1
	}
	return 0
}

func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

// writeRunErr prints the error to stderr: {"error":...} with --json, otherwise as plain text.
func writeRunErr(cc *cmdCtx, err error) {
	if cc.JSON {
		fmt.Fprintf(cc.Stderr, "{\"error\": %q}\n", err.Error())
		return
	}
	fmt.Fprintf(cc.Stderr, "%s\n", i18n.T("ui.error", i18n.M{"Msg": err.Error()}))
}
