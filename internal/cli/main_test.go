package cli

import (
	"os"
	"testing"
)

// TestMain pins the output language to en so that behavioral tests don't depend on
// the host machine's locale ($LANG). The assertions in these tests check behavior (help
// lists the commands, an error names the path, etc.) against the English text.
// The en rendering and locale parity are verified in the internal/i18n package.
func TestMain(m *testing.M) {
	os.Setenv("DISCODRIVE_LANG", "en")
	os.Exit(m.Run())
}
