package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

func init() {
	register(&command{
		Name:    "bookmarks",
		Summary: "cmd.bookmarks.summary",
		Usage:   "cmd.bookmarks.usage",
		Long:    "cmd.bookmarks.long",
		Examples: []string{
			"cmd.bookmarks.example.0",
			"cmd.bookmarks.example.1",
			"cmd.bookmarks.example.2",
			"cmd.bookmarks.example.3",
			"cmd.bookmarks.example.4",
		},
		Changing: true,
		Run:      runBookmarks,
	})
}

func runBookmarks(cc *cmdCtx, args []string) error {
	verb, rest := subverb(args)
	switch verb {
	case "", "tree":
		return bookmarksTree(cc, rest)
	case "list":
		return bookmarksList(cc, rest)
	case "add":
		return bookmarksAdd(cc, rest, false)
	case "mkdir":
		return bookmarksAdd(cc, rest, true)
	case "edit":
		return bookmarksEdit(cc, rest)
	case "rm":
		if len(rest) < 1 {
			return errors.New(i18n.T("cmd.bookmarks.msg.need_id"))
		}
		return bookmarksRm(cc, rest[0])
	default:
		return errors.New(i18n.T("cmd.bookmarks.msg.unknown_verb", i18n.M{"Verb": verb}))
	}
}

// loadBookmarks fetches the whole tree already sorted the way it is displayed.
func loadBookmarks(cc *cmdCtx) (*ddclient.Client, []ddclient.Bookmark, error) {
	c, err := cc.client()
	if err != nil {
		return nil, nil, err
	}
	items, err := c.ListBookmarks(context.Background())
	if err != nil {
		return nil, nil, err
	}
	ddclient.SortBookmarks(items)
	return c, items, nil
}

// filterBookmarks keeps the items whose id is in keep, preserving order.
func filterBookmarks(items []ddclient.Bookmark, keep map[string]bool) []ddclient.Bookmark {
	out := make([]ddclient.Bookmark, 0, len(items))
	for _, b := range items {
		if keep[b.ID] {
			out = append(out, b)
		}
	}
	return out
}

// matchBookmarks filters by a case-insensitive substring of the title, the url
// or the folder path, and optionally down to folders only.
func matchBookmarks(items []ddclient.Bookmark, paths map[string]string, q string, foldersOnly bool) []ddclient.Bookmark {
	needle := strings.ToLower(q)
	out := make([]ddclient.Bookmark, 0, len(items))
	for _, b := range items {
		if foldersOnly && !b.IsFolder {
			continue
		}
		if needle != "" {
			hay := strings.ToLower(b.Title + " " + b.URL + " " + paths[b.ID])
			if !strings.Contains(hay, needle) {
				continue
			}
		}
		out = append(out, b)
	}
	return out
}

func bookmarksTree(cc *cmdCtx, args []string) error {
	fs := newFlagSet("bookmarks tree", cc)
	folder := fs.String("folder", "", i18n.T("cmd.bookmarks.flag.folder"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, items, err := loadBookmarks(cc)
	if err != nil {
		return err
	}
	paths := ddclient.BookmarkPaths(items)
	if *folder != "" {
		id, err := ddclient.ResolveBookmarkFolder(items, *folder)
		if err != nil {
			return err
		}
		items = filterBookmarks(items, ddclient.BookmarkSubtree(items, id))
	}
	if cc.JSON {
		return renderBookmarksFlat(cc.Stdout, items, paths, true)
	}
	return renderBookmarkTree(cc.Stdout, items)
}

func bookmarksList(cc *cmdCtx, args []string) error {
	fs := newFlagSet("bookmarks list", cc)
	q := fs.String("q", "", i18n.T("cmd.bookmarks.flag.q"))
	folder := fs.String("folder", "", i18n.T("cmd.bookmarks.flag.folder"))
	foldersOnly := fs.Bool("folders", false, i18n.T("cmd.bookmarks.flag.folders"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, items, err := loadBookmarks(cc)
	if err != nil {
		return err
	}
	paths := ddclient.BookmarkPaths(items)
	if *folder != "" {
		id, err := ddclient.ResolveBookmarkFolder(items, *folder)
		if err != nil {
			return err
		}
		items = filterBookmarks(items, ddclient.BookmarkSubtree(items, id))
	}
	items = matchBookmarks(items, paths, *q, *foldersOnly)
	return renderBookmarksFlat(cc.Stdout, items, paths, cc.JSON)
}

func bookmarksAdd(cc *cmdCtx, args []string, isFolder bool) error {
	name := "bookmarks add"
	if isFolder {
		name = "bookmarks mkdir"
	}
	fs := newFlagSet(name, cc)
	title := fs.String("title", "", i18n.T("cmd.bookmarks.flag.title"))
	parent := fs.String("parent", "", i18n.T("cmd.bookmarks.flag.parent"))
	position := fs.Int("position", -1, i18n.T("cmd.bookmarks.flag.position"))
	pos, err := parsePositional(fs, args, 1)
	if err != nil {
		return err
	}

	// add takes the URL positionally, mkdir the folder title.
	in := ddclient.BookmarkCreate{ParentID: *parent, IsFolder: isFolder, Title: *title}
	if isFolder {
		if len(pos) > 0 && in.Title == "" {
			in.Title = pos[0]
		}
		if in.Title == "" {
			return errors.New(i18n.T("cmd.bookmarks.msg.need_title"))
		}
	} else {
		if len(pos) == 0 {
			return errors.New(i18n.T("cmd.bookmarks.msg.need_url"))
		}
		in.URL = pos[0]
		if in.Title == "" {
			in.Title = in.URL
		}
	}

	c, items, err := loadBookmarks(cc)
	if err != nil {
		return err
	}
	if in.ParentID, err = ddclient.ResolveBookmarkFolder(items, in.ParentID); err != nil {
		return err
	}
	in.Position = int32(*position)
	if *position < 0 {
		in.Position = ddclient.NextBookmarkPosition(items, in.ParentID)
	}
	created, err := c.CreateBookmark(context.Background(), in)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, created)
	}
	key := "cmd.bookmarks.msg.added"
	if isFolder {
		key = "cmd.bookmarks.msg.folder_created"
	}
	fmt.Fprint(cc.Stdout, i18n.T(key, i18n.M{"ID": created.ID, "Title": created.Title}))
	return nil
}

func bookmarksEdit(cc *cmdCtx, args []string) error {
	fs := newFlagSet("bookmarks edit", cc)
	title := fs.String("title", "", i18n.T("cmd.bookmarks.flag.title"))
	target := fs.String("url", "", i18n.T("cmd.bookmarks.flag.url"))
	parent := fs.String("parent", "", i18n.T("cmd.bookmarks.flag.parent_move"))
	position := fs.Int("position", 0, i18n.T("cmd.bookmarks.flag.position"))
	pos, err := parsePositional(fs, args, 1)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return errors.New(i18n.T("cmd.bookmarks.msg.need_id"))
	}

	// Only the flags actually typed are sent: an unset --parent must not drop
	// the bookmark into the root, and --parent "" is a deliberate move there.
	var patch ddclient.BookmarkPatch
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	if visited["title"] {
		patch.Title = title
	}
	if visited["url"] {
		patch.URL = target
	}
	if visited["parent"] {
		// An empty --parent is a move to the top level and needs no lookup;
		// anything else may be a folder path instead of an id.
		if *parent != "" {
			_, items, err := loadBookmarks(cc)
			if err != nil {
				return err
			}
			id, err := ddclient.ResolveBookmarkFolder(items, *parent)
			if err != nil {
				return err
			}
			parent = &id
		}
		patch.ParentID = parent
	}
	if visited["position"] {
		p := int32(*position)
		patch.Position = &p
	}
	if len(visited) == 0 {
		return errors.New(i18n.T("cmd.bookmarks.msg.nothing_to_change"))
	}

	c, err := cc.client()
	if err != nil {
		return err
	}
	updated, err := c.UpdateBookmark(context.Background(), pos[0], patch)
	if err != nil {
		return err
	}
	if cc.JSON {
		return renderJSON(cc.Stdout, updated)
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.bookmarks.msg.updated", i18n.M{"ID": updated.ID, "Title": updated.Title}))
	return nil
}

func bookmarksRm(cc *cmdCtx, id string) error {
	c, err := cc.client()
	if err != nil {
		return err
	}
	if err := c.DeleteBookmark(context.Background(), id); err != nil {
		return err
	}
	fmt.Fprint(cc.Stdout, i18n.T("cmd.bookmarks.msg.deleted", i18n.M{"ID": id}))
	return nil
}
