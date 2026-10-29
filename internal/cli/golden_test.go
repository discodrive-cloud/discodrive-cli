package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

func sampleNodes() []ddclient.Node {
	sz := int64(1234)
	return []ddclient.Node{
		{ID: "d1", Name: "docs", IsDir: true, Version: 1},
		{ID: "f1", Name: "a.txt", IsDir: false, Size: &sz, Version: 2},
	}
}

func TestRenderNodesHuman(t *testing.T) {
	var out bytes.Buffer
	if err := renderNodes(&out, sampleNodes(), false); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	// a directory is marked with a slash, a file shows its size
	if !strings.Contains(s, "docs/") || !strings.Contains(s, "a.txt") || !strings.Contains(s, "1234") {
		t.Fatalf("human output:\n%s", s)
	}
}

func TestRenderNodesJSON(t *testing.T) {
	var out bytes.Buffer
	if err := renderNodes(&out, sampleNodes(), true); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, `"is_dir": true`) || !strings.Contains(s, `"name": "a.txt"`) {
		t.Fatalf("json output:\n%s", s)
	}
}

// TestRenderNodesJSONEmptyIsArray: an empty/nil slice with --json yields [], not null.
func TestRenderNodesJSONEmptyIsArray(t *testing.T) {
	var out bytes.Buffer
	if err := renderNodes(&out, nil, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Fatalf("nil → %q, expected []", got)
	}
}
