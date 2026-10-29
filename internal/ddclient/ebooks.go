package ddclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// EbookFolder is the source folder for books.
type EbookFolder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// EbookSettings holds ebook-domain settings (read-only).
type EbookSettings struct {
	Enabled     bool         `json:"enabled"`
	Folder      *EbookFolder `json:"folder"`
	HasPassword bool         `json:"hasPassword"`
	HasApiKey   bool         `json:"hasApiKey"`
}

// Book is a library item. Note: ID is book.ID (for download/cover),
// NodeID is node.ID (for metadata). The server returns both.
type Book struct {
	ID          string   `json:"id"`
	NodeID      string   `json:"nodeId"`
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	Series      string   `json:"series,omitempty"`
	SeriesIndex float32  `json:"seriesIndex,omitempty"`
	Language    string   `json:"language,omitempty"`
	Format      string   `json:"format"`
	Tags        []string `json:"tags"`
	HasCover    bool     `json:"hasCover"`
}

// BookList is a page of the library. NOTE: Total is only reliable without a filter;
// with a search/filter the server puts the length of the current page in Total (see reference).
type BookList struct {
	Books []Book `json:"books"`
	Total int64  `json:"total"`
}

// BookQuery holds the library query parameters. The filters are mutually exclusive on the server
// (priority q > author > series > genre); the page size is fixed (50) and Start sets the offset.
type BookQuery struct {
	Q      string
	Author string
	Series string
	Genre  string
	Start  int
}

// BookFacets holds the available filter values.
type BookFacets struct {
	Authors []string `json:"authors"`
	Series  []string `json:"series"`
	Genres  []string `json:"genres"`
}

// BookMeta is a book's editable metadata (keyed by node.ID). PUT is a full overwrite.
type BookMeta struct {
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	Tags        []string `json:"tags"`
	Series      string   `json:"series"`
	SeriesIndex float64  `json:"seriesIndex"`
	Language    string   `json:"language"`
	Description string   `json:"description"`
	Publisher   string   `json:"publisher"`
	Date        string   `json:"date"`
	Edited      bool     `json:"edited"`
}

// GetEbookSettings — GET /me/ebooks.
func (c *Client) GetEbookSettings(ctx context.Context) (EbookSettings, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/ebooks")
	if err != nil {
		return EbookSettings{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return EbookSettings{}, readError(resp, "GET /me/ebooks")
	}
	var out EbookSettings
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return EbookSettings{}, err
	}
	return out, nil
}

// ListBooks — GET /me/ebooks/library. Sends only the parameters that are set.
func (c *Client) ListBooks(ctx context.Context, q BookQuery) (BookList, error) {
	v := url.Values{}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.Author != "" {
		v.Set("author", q.Author)
	}
	if q.Series != "" {
		v.Set("series", q.Series)
	}
	if q.Genre != "" {
		v.Set("genre", q.Genre)
	}
	if q.Start > 0 {
		v.Set("start", strconv.Itoa(q.Start))
	}
	path := "/me/ebooks/library"
	if len(v) > 0 {
		path += "?" + v.Encode()
	}
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return BookList{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return BookList{}, readError(resp, "GET /me/ebooks/library")
	}
	var out BookList
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return BookList{}, err
	}
	return out, nil
}

// BookFacets — GET /me/ebooks/library/facets.
func (c *Client) BookFacets(ctx context.Context) (BookFacets, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/ebooks/library/facets")
	if err != nil {
		return BookFacets{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return BookFacets{}, readError(resp, "GET /me/ebooks/library/facets")
	}
	var out BookFacets
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return BookFacets{}, err
	}
	return out, nil
}

// DownloadBook — GET /me/ebooks/library/{id}/download (id is book.ID). Writes the contents to w.
func (c *Client) DownloadBook(ctx context.Context, bookID string, w io.Writer) error {
	resp, err := c.do(ctx, http.MethodGet, "/me/ebooks/library/"+url.PathEscape(bookID)+"/download")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return readError(resp, "GET /me/ebooks/library/{id}/download")
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// GetBookMeta — GET /me/ebooks/meta/{id} (id is node.ID).
func (c *Client) GetBookMeta(ctx context.Context, nodeID string) (BookMeta, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/ebooks/meta/"+url.PathEscape(nodeID))
	if err != nil {
		return BookMeta{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return BookMeta{}, readError(resp, "GET /me/ebooks/meta/{id}")
	}
	var out BookMeta
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return BookMeta{}, err
	}
	return out, nil
}

// UpdateBookMeta — PUT /me/ebooks/meta/{id} (id is node.ID). Full overwrite; responds 204.
func (c *Client) UpdateBookMeta(ctx context.Context, nodeID string, m BookMeta) error {
	resp, err := c.doJSON(ctx, http.MethodPut, "/me/ebooks/meta/"+url.PathEscape(nodeID), m)
	if err != nil {
		return err
	}
	return expect204(resp, "PUT /me/ebooks/meta/{id}")
}

// EbookScan — POST /me/ebooks/scan. Fire-and-forget.
func (c *Client) EbookScan(ctx context.Context) (bool, error) {
	resp, err := c.do(ctx, http.MethodPost, "/me/ebooks/scan")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, readError(resp, "POST /me/ebooks/scan")
	}
	var out ScanResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Scanning, nil
}
