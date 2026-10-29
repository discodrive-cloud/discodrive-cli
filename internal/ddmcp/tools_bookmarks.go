package ddmcp

import (
	"context"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// rootSentinel moves a node to the top level in dd_bookmarks_update. A literal
// is used instead of an empty string so that "field omitted" stays telling
// apart from "move to the root".
const rootSentinel = "root"

// defaultBookmarkLimit caps the listing: a real collection is thousands of rows
// and would flood the model's context.
const defaultBookmarkLimit = 200

type bookmarkOut struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	IsFolder bool   `json:"is_folder"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Path     string `json:"path"`
	Position int32  `json:"position"`
}

type bookmarksListInput struct {
	Q           string `json:"q,omitempty" jsonschema:"case-insensitive substring of the title, the url or the folder path (optional)"`
	FolderID    string `json:"folder_id,omitempty" jsonschema:"limit the result to this folder and everything under it (optional)"`
	FoldersOnly bool   `json:"folders_only,omitempty" jsonschema:"return only folders — use it to find a parent_id for dd_bookmarks_add"`
	Limit       int    `json:"limit,omitempty" jsonschema:"max items to return (default 200)"`
}

type bookmarksListOutput struct {
	Bookmarks []bookmarkOut `json:"bookmarks"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated"`
}

type bookmarkAddInput struct {
	Title    string `json:"title" jsonschema:"title (required for a folder, recommended for a bookmark)"`
	URL      string `json:"url,omitempty" jsonschema:"target URL — required unless is_folder is true"`
	IsFolder bool   `json:"is_folder,omitempty" jsonschema:"true = create a folder instead of a bookmark"`
	ParentID string `json:"parent_id,omitempty" jsonschema:"id of an existing bookmark folder; empty = top level"`
}

type bookmarkUpdateInput struct {
	ID       string `json:"id" jsonschema:"bookmark or folder id from dd_bookmarks_list"`
	Title    string `json:"title,omitempty" jsonschema:"new title; empty = leave unchanged"`
	URL      string `json:"url,omitempty" jsonschema:"new URL; empty = leave unchanged; folders have no URL"`
	ParentID string `json:"parent_id,omitempty" jsonschema:"move into this folder id; the literal \"root\" moves to the top level; empty = leave where it is"`
}

type bookmarkIDInput struct {
	ID string `json:"id" jsonschema:"bookmark or folder id from dd_bookmarks_list"`
}

func toBookmarkOut(b ddclient.Bookmark, path string) bookmarkOut {
	parent := ""
	if b.ParentID != nil {
		parent = *b.ParentID
	}
	return bookmarkOut{
		ID: b.ID, ParentID: parent, IsFolder: b.IsFolder,
		Title: b.Title, URL: b.URL, Path: path, Position: b.Position,
	}
}

func bookmarkToolSpecs() []toolSpec {
	return []toolSpec{
		{Name: "dd_bookmarks_list", Description: i18n.T("tool.dd_bookmarks_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in bookmarksListInput) (*mcp.CallToolResult, bookmarksListOutput, error) {
				items, err := c.ListBookmarks(ctx)
				if err != nil {
					return nil, bookmarksListOutput{}, err
				}
				ddclient.SortBookmarks(items)
				paths := ddclient.BookmarkPaths(items)

				var subtree map[string]bool
				if in.FolderID != "" {
					subtree = ddclient.BookmarkSubtree(items, in.FolderID)
				}
				needle := strings.ToLower(in.Q)

				out := bookmarksListOutput{Bookmarks: []bookmarkOut{}}
				limit := in.Limit
				if limit <= 0 {
					limit = defaultBookmarkLimit
				}
				for _, b := range items {
					if in.FoldersOnly && !b.IsFolder {
						continue
					}
					if subtree != nil && !subtree[b.ID] {
						continue
					}
					if needle != "" {
						hay := strings.ToLower(b.Title + " " + b.URL + " " + paths[b.ID])
						if !strings.Contains(hay, needle) {
							continue
						}
					}
					out.Total++
					if len(out.Bookmarks) < limit {
						out.Bookmarks = append(out.Bookmarks, toBookmarkOut(b, paths[b.ID]))
					}
				}
				out.Truncated = out.Total > len(out.Bookmarks)
				return nil, out, nil
			})
		}},
		{Name: "dd_bookmarks_add", Description: i18n.T("tool.dd_bookmarks_add.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in bookmarkAddInput) (*mcp.CallToolResult, bookmarkOut, error) {
				items, err := c.ListBookmarks(ctx)
				if err != nil {
					return nil, bookmarkOut{}, err
				}
				created, err := c.CreateBookmark(ctx, ddclient.BookmarkCreate{
					ParentID: in.ParentID,
					IsFolder: in.IsFolder,
					Title:    in.Title,
					URL:      in.URL,
					Position: ddclient.NextBookmarkPosition(items, in.ParentID),
				})
				if err != nil {
					return nil, bookmarkOut{}, err
				}
				return nil, toBookmarkOut(created, ddclient.BookmarkPaths(append(items, created))[created.ID]), nil
			})
		}},
		{Name: "dd_bookmarks_update", Description: i18n.T("tool.dd_bookmarks_update.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in bookmarkUpdateInput) (*mcp.CallToolResult, bookmarkOut, error) {
				var patch ddclient.BookmarkPatch
				if in.Title != "" {
					patch.Title = &in.Title
				}
				if in.URL != "" {
					patch.URL = &in.URL
				}
				if in.ParentID != "" {
					target := in.ParentID
					if target == rootSentinel {
						target = ""
					}
					patch.ParentID = &target
				}
				updated, err := c.UpdateBookmark(ctx, in.ID, patch)
				if err != nil {
					return nil, bookmarkOut{}, err
				}
				items, err := c.ListBookmarks(ctx)
				if err != nil {
					return nil, toBookmarkOut(updated, ""), nil
				}
				return nil, toBookmarkOut(updated, ddclient.BookmarkPaths(items)[updated.ID]), nil
			})
		}},
		{Name: "dd_bookmarks_delete", Description: i18n.T("tool.dd_bookmarks_delete.desc"), Changing: true, register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in bookmarkIDInput) (*mcp.CallToolResult, okOutput, error) {
				if err := c.DeleteBookmark(ctx, in.ID); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_bookmarks_delete.msg.deleted", i18n.M{"ID": in.ID})}, nil
			})
		}},
	}
}
