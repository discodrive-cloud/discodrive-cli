package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// command is a single dd-cli subcommand along with the data for its detailed help.
// Summary/Usage/Long/Examples hold i18n KEYS (not text) — they are set in init()
// before the language is chosen, so they are resolved via i18n.T() when help is rendered.
type command struct {
	Name     string
	Summary  string   // key: one-line entry for the root list
	Usage    string   // key: synopsis line
	Long     string   // key: description paragraph
	Examples []string // example keys
	Changing bool     // irreversible operation (rm/mv) — flagged in help
	Run      func(*cmdCtx, []string) error
}

var commands []*command

// register adds a command to the registry (called from init() in the command files).
func register(c *command) { commands = append(commands, c) }

func lookup(name string) *command {
	for _, c := range commands {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func printRootHelp(w io.Writer) {
	fmt.Fprintln(w, i18n.T("ui.root.tagline"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("ui.usage_label"))
	fmt.Fprintln(w, i18n.T("ui.root.usage_line"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("ui.commands_label"))
	sorted := append([]*command(nil), commands...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, c := range sorted {
		fmt.Fprintf(w, "  %-12s %s\n", c.Name, i18n.T(c.Summary))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("ui.global_flags_label"))
	fmt.Fprintln(w, i18n.T("ui.flag.json"))
	fmt.Fprintln(w, i18n.T("ui.flag.profile"))
	fmt.Fprintln(w, i18n.T("ui.flag.server"))
	fmt.Fprintln(w, i18n.T("ui.flag.lang"))
	fmt.Fprintln(w, i18n.T("ui.flag.version"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("ui.root.more"))
}

func printCommandHelp(w io.Writer, c *command) {
	fmt.Fprintf(w, "%s — %s\n\n", c.Name, i18n.T(c.Summary))
	if c.Usage != "" {
		fmt.Fprintf(w, "%s\n  %s\n\n", i18n.T("ui.usage_label"), i18n.T(c.Usage))
	}
	if c.Long != "" {
		fmt.Fprintf(w, "%s\n\n", i18n.T(c.Long))
	}
	if c.Changing {
		fmt.Fprintln(w, i18n.T("ui.irreversible"))
		fmt.Fprintln(w)
	}
	if len(c.Examples) > 0 {
		fmt.Fprintln(w, i18n.T("ui.examples_label"))
		for _, ex := range c.Examples {
			fmt.Fprintf(w, "  %s\n", i18n.T(ex))
		}
	}
}
