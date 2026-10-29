package ddclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Kinds and statuses of a saved item (mirror of the server's CHECK constraints).
const (
	SavedKindArticle  = "article"
	SavedKindDownload = "download"

	SavedStatusPending    = "pending"
	SavedStatusProcessing = "processing"
	SavedStatusDone       = "done"
	SavedStatusError      = "error"
)

// SavedItem is a read-later article or a server-side download.
type SavedItem struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	SizeBytes  *int64 `json:"size_bytes"`
	BytesDone  int64  `json:"bytes_done"`
	HasContent bool   `json:"has_content"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// SavedQuery filters the article list.
type SavedQuery struct {
	Status string
	Q      string
	Limit  int
	Offset int
}

// SaveArticle queues a page for read-later. Saving a url that already exists is
// a no-op, except that a failed item is retried; its title is never changed.
func (c *Client) SaveArticle(ctx context.Context, pageURL, title string) (SavedItem, error) {
	return c.createSaved(ctx, map[string]any{
		"url":   pageURL,
		"kind":  SavedKindArticle,
		"title": title,
	})
}

// QueueDownload asks the server to fetch a file into the user's Downloads folder.
// cookie (optional) is a Cookie header for sites behind a login.
func (c *Client) QueueDownload(ctx context.Context, fileURL, title, cookie string) (SavedItem, error) {
	body := map[string]any{
		"url":   fileURL,
		"kind":  SavedKindDownload,
		"title": title,
	}
	if cookie != "" {
		body["cookie"] = cookie
	}
	return c.createSaved(ctx, body)
}

func (c *Client) createSaved(ctx context.Context, body map[string]any) (SavedItem, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, "/me/saved", body)
	if err != nil {
		return SavedItem{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SavedItem{}, readError(resp, "POST /me/saved")
	}
	var out SavedItem
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// ListSavedArticles lists read-later articles. Server-side downloads are never
// listed — the kind filter is always article.
func (c *Client) ListSavedArticles(ctx context.Context, q SavedQuery) ([]SavedItem, error) {
	v := url.Values{}
	v.Set("kind", SavedKindArticle)
	if q.Status != "" {
		v.Set("status", q.Status)
	}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset > 0 {
		v.Set("offset", strconv.Itoa(q.Offset))
	}
	resp, err := c.do(ctx, http.MethodGet, "/me/saved?"+v.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/saved")
	}
	var out []SavedItem
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// savedNotFound is the exact error the server's /me/saved/{id} handlers write for an
// id the account does not have.
const savedNotFound = "not found"

// IsUnknownSavedItem reports whether err is the server's own 404 for a saved-item
// id it does not know (for this account). Any other 404 — without the server's
// JSON error body (a proxy, a server without the endpoint) or with a different
// message — says nothing about the id.
func IsUnknownSavedItem(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound && ae.FromBody &&
		ae.Message == savedNotFound
}

// GetSavedItem returns one item by id — the only way to inspect a download.
func (c *Client) GetSavedItem(ctx context.Context, id string) (SavedItem, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/saved/"+url.PathEscape(id))
	if err != nil {
		return SavedItem{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SavedItem{}, readError(resp, "GET /me/saved/{id}")
	}
	var out SavedItem
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// SavedContent copies the article body into w. The response is text/markdown,
// not JSON. A 404 means the item is not a finished article.
func (c *Client) SavedContent(ctx context.Context, id string, w io.Writer) error {
	resp, err := c.do(ctx, http.MethodGet, "/me/saved/"+url.PathEscape(id)+"/content")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s: %w", i18n.T("err.saved.no_content"),
			readError(resp, "GET /me/saved/{id}/content"))
	}
	if resp.StatusCode != http.StatusOK {
		return readError(resp, "GET /me/saved/{id}/content")
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// RetrySavedItem re-fetches a saved item. It fails with 409 while the item is
// still pending or processing.
func (c *Client) RetrySavedItem(ctx context.Context, id string) (SavedItem, error) {
	resp, err := c.do(ctx, http.MethodPost, "/me/saved/"+url.PathEscape(id)+"/retry")
	if err != nil {
		return SavedItem{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return SavedItem{}, fmt.Errorf("%s: %w", i18n.T("err.saved.not_retryable"),
			readError(resp, "POST /me/saved/{id}/retry"))
	}
	if resp.StatusCode != http.StatusOK {
		return SavedItem{}, readError(resp, "POST /me/saved/{id}/retry")
	}
	var out SavedItem
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// DeleteSavedItem removes the record. Files already written into the storage stay.
func (c *Client) DeleteSavedItem(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/me/saved/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return expect204(resp, "DELETE /me/saved/{id}")
}
