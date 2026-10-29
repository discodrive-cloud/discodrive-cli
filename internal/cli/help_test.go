package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandHelpHasExample(t *testing.T) {
	register(&command{
		Name: "demo", Summary: "demo command",
		Usage: "dd-cli demo <arg>", Long: "Does a demo.",
		Examples: []string{"dd-cli demo foo"},
		Run:      func(*cmdCtx, []string) error { return nil },
	})
	var out, errb bytes.Buffer
	code := Run([]string{"demo", "--help"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	s := out.String()
	for _, want := range []string{"dd-cli demo <arg>", "Does a demo.", "dd-cli demo foo"} {
		if !strings.Contains(s, want) {
			t.Fatalf("command help missing %q\n%s", want, s)
		}
	}
}
