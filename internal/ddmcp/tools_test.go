package ddmcp

import (
	"strings"
	"testing"
)

func names(specs []toolSpec) map[string]bool {
	m := map[string]bool{}
	for _, s := range specs {
		m[s.Name] = true
	}
	return m
}

func TestDefaultModeHidesChangingTools(t *testing.T) {
	got := names(selectTools(toolSpecs(), false))
	for _, want := range []string{"dd_files_list", "dd_files_read", "dd_files_search", "dd_files_mkdir"} {
		if !got[want] {
			t.Fatalf("read/create tool %q must be present by default", want)
		}
	}
	for _, banned := range []string{"dd_files_delete", "dd_files_move", "dd_files_rename", "dd_files_upload"} {
		if got[banned] {
			t.Fatalf("changing tool %q must be hidden without --allow-changes", banned)
		}
	}
}

func TestAllowChangesExposesAll(t *testing.T) {
	got := names(selectTools(toolSpecs(), true))
	for _, want := range []string{"dd_files_delete", "dd_files_move", "dd_files_rename", "dd_files_upload"} {
		if !got[want] {
			t.Fatalf("changing tool %q must appear under --allow-changes", want)
		}
	}
}

// TestToolDescriptionsAreSourceOfTruth: every tool has a non-empty Description,
// and dd_files_delete carries the trash-bin caveat (which must reach the LLM).
func TestToolDescriptionsAreSourceOfTruth(t *testing.T) {
	var deleteSeen bool
	for _, sp := range toolSpecs() {
		if sp.Description == "" {
			t.Fatalf("tool %s has no Description", sp.Name)
		}
		if sp.Name == "dd_files_delete" {
			deleteSeen = true
			if !strings.Contains(sp.Description, "trash") {
				t.Fatalf("dd_files_delete: expected a trash caveat, got %q", sp.Description)
			}
		}
	}
	if !deleteSeen {
		t.Fatal("dd_files_delete not found")
	}
}
