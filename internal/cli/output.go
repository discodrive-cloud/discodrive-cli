package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

// safeWriter strips terminal control characters from human-readable output. Names,
// titles and error messages come from the server, and through it from arbitrary web
// pages; printed raw, an escape sequence could rewrite the screen, plant a hyperlink
// or set the clipboard. JSON output is left alone (encoding/json already escapes
// controls), and verbatim file contents go through cmdCtx.RawStdout.
type safeWriter struct{ w io.Writer }

func (s safeWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(s.w, terminalSafe(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// terminalSafe drops C0/C1 controls (ESC, CSI, OSC, CR, BEL…) and bidi overrides,
// keeping newlines and tabs. Invalid UTF-8 becomes U+FFFD, so a lone 8-bit CSI byte
// cannot slip through either.
func terminalSafe(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case unicode.IsControl(r),
			r >= 0x202A && r <= 0x202E,
			r >= 0x2066 && r <= 0x2069:
			return -1
		}
		return r
	}, s)
}

// renderJSON prints v as indented JSON with a trailing newline.
func renderJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// renderBookmarksFlat prints bookmarks one per line: id, folder path and title
// (a folder gets a trailing "/", a bookmark shows its url). paths comes from the
// full tree, not from items — a filtered slice has no ancestors left to walk.
// With asJSON a nil slice still renders as [], not null.
func renderBookmarksFlat(w io.Writer, items []ddclient.Bookmark, paths map[string]string, asJSON bool) error {
	if asJSON {
		if items == nil {
			items = []ddclient.Bookmark{}
		}
		return renderJSON(w, items)
	}
	for _, b := range items {
		name := b.Title
		if b.IsFolder {
			name += "/"
		}
		if p := paths[b.ID]; p != "" {
			name = p + "/" + name
		}
		if _, err := fmt.Fprintf(w, "%-38s %-40s %s\n", b.ID, name, b.URL); err != nil {
			return err
		}
	}
	return nil
}

// renderBookmarkTree prints the tree with the id in the first column and the
// title indented by depth. Nodes whose parent is missing from items are shown at
// the top level, so a subtree slice still renders instead of coming out empty.
func renderBookmarkTree(w io.Writer, items []ddclient.Bookmark) error {
	present := make(map[string]bool, len(items))
	for _, b := range items {
		present[b.ID] = true
	}
	children := make(map[string][]ddclient.Bookmark, len(items))
	var roots []ddclient.Bookmark
	for _, b := range items {
		parent := ""
		if b.ParentID != nil {
			parent = *b.ParentID
		}
		if parent == "" || !present[parent] {
			roots = append(roots, b)
			continue
		}
		children[parent] = append(children[parent], b)
	}

	var walk func(nodes []ddclient.Bookmark, depth int) error
	walk = func(nodes []ddclient.Bookmark, depth int) error {
		for _, b := range nodes {
			name := b.Title
			if b.IsFolder {
				name += "/"
			} else if b.URL != "" {
				name += "  " + b.URL
			}
			if _, err := fmt.Fprintf(w, "%-38s %s%s\n", b.ID, strings.Repeat("  ", depth), name); err != nil {
				return err
			}
			if depth < 64 {
				if err := walk(children[b.ID], depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(roots, 0)
}

// savedProgress describes how far a saved item got: a percentage when the total
// size is known, otherwise just the bytes fetched so far. The server reports no
// size for chunked responses, so the total may legitimately be nil or zero.
func savedProgress(it ddclient.SavedItem) string {
	if it.Status == ddclient.SavedStatusError {
		if it.Error != "" {
			return it.Error
		}
		return "-"
	}
	if it.SizeBytes != nil && *it.SizeBytes > 0 {
		return fmt.Sprintf("%d%% of %s", it.BytesDone*100 / *it.SizeBytes, humanBytes(*it.SizeBytes))
	}
	if it.BytesDone > 0 {
		return humanBytes(it.BytesDone)
	}
	return "-"
}

// humanBytes formats a byte count with a binary unit suffix.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}

// renderSavedItems prints saved articles or downloads one per line: id, status,
// progress and title. With asJSON a nil slice still renders as [], not null.
func renderSavedItems(w io.Writer, items []ddclient.SavedItem, asJSON bool) error {
	if asJSON {
		if items == nil {
			items = []ddclient.SavedItem{}
		}
		return renderJSON(w, items)
	}
	for _, it := range items {
		title := it.Title
		if title == "" {
			title = it.URL
		}
		if _, err := fmt.Fprintf(w, "%-38s %-11s %-18s %s\n", it.ID, it.Status, savedProgress(it), title); err != nil {
			return err
		}
	}
	return nil
}

// renderNodes prints a list of nodes: a JSON array when asJSON, otherwise a human-readable table.
// Directories are marked with a trailing "/", files show their size in bytes.
func renderNodes(w io.Writer, nodes []ddclient.Node, asJSON bool) error {
	if asJSON {
		if nodes == nil {
			nodes = []ddclient.Node{}
		}
		return renderJSON(w, nodes)
	}
	for _, n := range nodes {
		name := n.Name
		size := "-"
		if n.IsDir {
			name += "/"
		} else if n.Size != nil {
			size = fmt.Sprintf("%d", *n.Size)
		}
		if _, err := fmt.Fprintf(w, "%-30s %10s\n", name, size); err != nil {
			return err
		}
	}
	return nil
}
