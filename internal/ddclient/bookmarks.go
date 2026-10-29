package ddclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Bookmark is a node of the browser bookmark tree stored on the server.
// ParentID is nil for a top-level node.
type Bookmark struct {
	ID         string  `json:"id"`
	ParentID   *string `json:"parent_id"`
	IsFolder   bool    `json:"is_folder"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Position   int32   `json:"position"`
	HasFavicon bool    `json:"has_favicon"`
	UpdatedAt  string  `json:"updated_at"`
}

// BookmarkCreate is the POST body. An empty ParentID means the top level.
type BookmarkCreate struct {
	ParentID string
	IsFolder bool
	Title    string
	URL      string
	Position int32
}

// BookmarkPatch is the PATCH body. A nil field is not sent at all, so it stays
// unchanged; ParentID pointing at "" moves the node to the top level.
type BookmarkPatch struct {
	Title    *string
	URL      *string
	Position *int32
	ParentID *string
}

// ListBookmarks returns the whole tree as a flat slice (the server paginates nothing).
func (c *Client) ListBookmarks(ctx context.Context) ([]Bookmark, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/bookmarks")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/bookmarks")
	}
	var out []Bookmark
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateBookmark creates a bookmark or a folder (in.IsFolder).
func (c *Client) CreateBookmark(ctx context.Context, in BookmarkCreate) (Bookmark, error) {
	body := map[string]any{
		"is_folder": in.IsFolder,
		"title":     in.Title,
		"url":       in.URL,
		"position":  in.Position,
	}
	if in.ParentID != "" {
		body["parent_id"] = in.ParentID
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/me/bookmarks", body)
	if err != nil {
		return Bookmark{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusBadRequest {
		return Bookmark{}, fmt.Errorf("%s: %w", i18n.T("err.bookmarks.bad_parent"),
			readError(resp, "POST /me/bookmarks"))
	}
	if resp.StatusCode != http.StatusOK {
		return Bookmark{}, readError(resp, "POST /me/bookmarks")
	}
	var out Bookmark
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// UpdateBookmark changes only the fields set in p (see BookmarkPatch).
func (c *Client) UpdateBookmark(ctx context.Context, id string, p BookmarkPatch) (Bookmark, error) {
	body := map[string]any{}
	if p.Title != nil {
		body["title"] = *p.Title
	}
	if p.URL != nil {
		body["url"] = *p.URL
	}
	if p.Position != nil {
		body["position"] = *p.Position
	}
	if p.ParentID != nil {
		body["parent_id"] = *p.ParentID
	}
	resp, err := c.doJSON(ctx, http.MethodPatch, "/me/bookmarks/"+url.PathEscape(id), body)
	if err != nil {
		return Bookmark{}, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusConflict:
		return Bookmark{}, fmt.Errorf("%s: %w", i18n.T("err.bookmarks.move_cycle"),
			readError(resp, "PATCH /me/bookmarks/{id}"))
	case http.StatusBadRequest:
		return Bookmark{}, fmt.Errorf("%s: %w", i18n.T("err.bookmarks.bad_parent"),
			readError(resp, "PATCH /me/bookmarks/{id}"))
	default:
		return Bookmark{}, readError(resp, "PATCH /me/bookmarks/{id}")
	}
	var out Bookmark
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// DeleteBookmark deletes a bookmark, or a folder together with everything inside it.
func (c *Client) DeleteBookmark(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/me/bookmarks/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return expect204(resp, "DELETE /me/bookmarks/{id}")
}

// BookmarkPaths maps every id to its folder path ("Toolbar/Dev"), excluding the
// node's own title. Nodes whose parent is missing from items are treated as top level.
func BookmarkPaths(items []Bookmark) map[string]string {
	byID := make(map[string]Bookmark, len(items))
	for _, b := range items {
		byID[b.ID] = b
	}
	paths := make(map[string]string, len(items))
	var pathOf func(id string, depth int) string
	pathOf = func(id string, depth int) string {
		b, ok := byID[id]
		if !ok || b.ParentID == nil || *b.ParentID == "" || depth > 64 {
			return ""
		}
		parent, ok := byID[*b.ParentID]
		if !ok {
			return ""
		}
		if prefix := pathOf(parent.ID, depth+1); prefix != "" {
			return prefix + "/" + parent.Title
		}
		return parent.Title
	}
	for _, b := range items {
		paths[b.ID] = pathOf(b.ID, 0)
	}
	return paths
}

// BookmarkSubtree returns the ids of folderID and everything under it.
func BookmarkSubtree(items []Bookmark, folderID string) map[string]bool {
	children := make(map[string][]string, len(items))
	for _, b := range items {
		if b.ParentID != nil && *b.ParentID != "" {
			children[*b.ParentID] = append(children[*b.ParentID], b.ID)
		}
	}
	out := map[string]bool{folderID: true}
	queue := []string{folderID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, kid := range children[id] {
			if !out[kid] {
				out[kid] = true
				queue = append(queue, kid)
			}
		}
	}
	return out
}

// ResolveBookmarkFolder turns ref into a folder id. ref may already be an id, or
// a folder path such as "Toolbar/Dev", or a bare folder title — matched
// case-insensitively. An empty ref stays empty, meaning the top level.
// A full-path match wins over a title match, so "Toolbar/Dev" is unambiguous
// even when another folder is called just "Dev".
func ResolveBookmarkFolder(items []Bookmark, ref string) (string, error) {
	if ref == "" {
		return "", nil
	}
	for _, b := range items {
		if b.ID == ref {
			return b.ID, nil
		}
	}
	paths := BookmarkPaths(items)
	want := strings.ToLower(strings.Trim(ref, "/"))
	var byPath, byTitle []string
	for _, b := range items {
		if !b.IsFolder {
			continue
		}
		full := b.Title
		if p := paths[b.ID]; p != "" {
			full = p + "/" + b.Title
		}
		switch {
		case strings.ToLower(full) == want:
			byPath = append(byPath, b.ID)
		case strings.ToLower(b.Title) == want:
			byTitle = append(byTitle, b.ID)
		}
	}
	hits := byPath
	if len(hits) == 0 {
		hits = byTitle
	}
	switch len(hits) {
	case 0:
		return "", errors.New(i18n.T("err.bookmarks.folder_not_found", i18n.M{"Ref": ref}))
	case 1:
		return hits[0], nil
	default:
		return "", errors.New(i18n.T("err.bookmarks.folder_ambiguous", i18n.M{"Ref": ref}))
	}
}

// NextBookmarkPosition returns max(position)+1 among the children of parentID
// (an empty parentID means the top level). The server assigns no position of its
// own, and a pile of zeroes shows up as a shuffled tree in the browser.
func NextBookmarkPosition(items []Bookmark, parentID string) int32 {
	var max int32 = -1
	for _, b := range items {
		p := ""
		if b.ParentID != nil {
			p = *b.ParentID
		}
		if p == parentID && b.Position > max {
			max = b.Position
		}
	}
	return max + 1
}

// SortBookmarks orders siblings the way the tree is meant to be shown:
// folders keep their position, ties break by title.
func SortBookmarks(items []Bookmark) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Position != items[j].Position {
			return items[i].Position < items[j].Position
		}
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
}
