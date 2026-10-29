package ddmcp

import (
	"os"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// TestMain pins the test language to English so assertions on localized
// strings are deterministic regardless of the host locale.
func TestMain(m *testing.M) {
	os.Setenv("DISCODRIVE_LANG", "en")
	i18n.SetActive("en")
	os.Exit(m.Run())
}
