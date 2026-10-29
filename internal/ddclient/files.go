package ddclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Node is a file or folder as returned by the server (GET /files...).
type Node struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    *int64 `json:"size"`
	Version int64  `json:"version"`
	// ContentHash is the sha256 (hex) of the bytes the server stored; empty for a folder.
	ContentHash string `json:"content_hash,omitempty"`
}

func decodeNodes(resp *http.Response) ([]Node, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /files")
	}
	var out []Node
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListRoot returns the nodes at the root of the user's storage.
func (c *Client) ListRoot(ctx context.Context) ([]Node, error) {
	resp, err := c.do(ctx, http.MethodGet, "/files")
	if err != nil {
		return nil, err
	}
	return decodeNodes(resp)
}

// ListChildren returns the contents of folder parentID.
func (c *Client) ListChildren(ctx context.Context, parentID string) ([]Node, error) {
	if parentID == "" {
		return c.ListRoot(ctx)
	}
	resp, err := c.do(ctx, http.MethodGet, "/files?parent_id="+url.QueryEscape(parentID))
	if err != nil {
		return nil, err
	}
	return decodeNodes(resp)
}

// GetNode returns a single node by id.
func (c *Client) GetNode(ctx context.Context, id string) (Node, error) {
	resp, err := c.do(ctx, http.MethodGet, "/files/"+url.PathEscape(id))
	if err != nil {
		return Node{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Node{}, readError(resp, "GET /files/{id}")
	}
	var n Node
	return n, json.NewDecoder(resp.Body).Decode(&n)
}

// Resolve turns a slash-separated path into a Node, descending from the root. "", "/", "." → the synthetic root.
func (c *Client) Resolve(ctx context.Context, path string) (Node, error) {
	clean := strings.Trim(path, "/")
	if clean == "" || clean == "." {
		return Node{ID: "", Name: "/", IsDir: true}, nil
	}
	parts := strings.Split(clean, "/")
	cur := Node{ID: "", IsDir: true}
	for i, part := range parts {
		if !cur.IsDir {
			return Node{}, &APIError{Status: http.StatusNotFound, Message: i18n.T("err.files.not_dir", i18n.M{"Path": strings.Join(parts[:i], "/")})}
		}
		kids, err := c.ListChildren(ctx, cur.ID)
		if err != nil {
			return Node{}, err
		}
		found := false
		for _, k := range kids {
			if k.Name == part {
				cur, found = k, true
				break
			}
		}
		if !found {
			return Node{}, &APIError{Status: http.StatusNotFound, Message: i18n.T("err.files.no_path", i18n.M{"Path": clean})}
		}
	}
	return cur, nil
}

// Download writes the contents of file id to w.
func (c *Client) Download(ctx context.Context, id string, w io.Writer) error {
	resp, err := c.do(ctx, http.MethodGet, "/files/"+url.PathEscape(id)+"/content")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return readError(resp, "GET /files/{id}/content")
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func decodeNode(resp *http.Response, what string, okStatus int) (Node, error) {
	defer resp.Body.Close()
	if resp.StatusCode != okStatus {
		return Node{}, readError(resp, what)
	}
	var n Node
	return n, json.NewDecoder(resp.Body).Decode(&n)
}

// CreateFolder creates folder name under parentID ("" = root).
func (c *Client) CreateFolder(ctx context.Context, parentID, name string) (Node, error) {
	body := map[string]any{"name": name}
	if parentID != "" {
		body["parent_id"] = parentID
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/files/folder", body)
	if err != nil {
		return Node{}, err
	}
	return decodeNode(resp, "POST /files/folder", http.StatusCreated)
}

// Rename renames node id.
func (c *Client) Rename(ctx context.Context, id, newName string) (Node, error) {
	resp, err := c.doJSON(ctx, http.MethodPatch, "/files/"+url.PathEscape(id)+"/rename", map[string]string{"name": newName})
	if err != nil {
		return Node{}, err
	}
	return decodeNode(resp, "PATCH rename", http.StatusOK)
}

// Move moves node id under newParentID ("" = root).
func (c *Client) Move(ctx context.Context, id, newParentID string) (Node, error) {
	var pid *string
	if newParentID != "" {
		pid = &newParentID
	}
	resp, err := c.doJSON(ctx, http.MethodPatch, "/files/"+url.PathEscape(id)+"/move", map[string]any{"parent_id": pid})
	if err != nil {
		return Node{}, err
	}
	return decodeNode(resp, "PATCH move", http.StatusOK)
}

// Relocate atomically moves and renames a node. Older servers reject this endpoint
// before changing anything; do not fall back to separate Move and Rename requests.
func (c *Client) Relocate(ctx context.Context, id, parentID, name string) (Node, error) {
	var parent *string
	if parentID != "" {
		parent = &parentID
	}
	resp, err := c.doJSON(ctx, http.MethodPatch, "/files/"+url.PathEscape(id)+"/relocate", map[string]any{"parent_id": parent, "name": name})
	if err != nil {
		return Node{}, err
	}
	return decodeNode(resp, "PATCH relocate", http.StatusOK)
}

// Delete soft-deletes node id (along with its subtree).
func (c *Client) Delete(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/files/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return readError(resp, "DELETE /files/{id}")
	}
	return nil
}

// countingWriter counts bytes written and discards them.
type countingWriter int64

func (c *countingWriter) Write(p []byte) (int, error) {
	*c += countingWriter(len(p))
	return len(p), nil
}

// multipartOverhead measures the multipart envelope — the field parts, the file part's
// headers and the closing boundary — for a file part carrying no bytes. Adding the file
// size gives an exact Content-Length, which keeps the upload out of chunked encoding and
// lets net/http reject a body that ends short instead of sending a truncated file.
//
// Every field counted here must also be written by the goroutine that streams the real
// body, in the same order, or the declared length will not match what is sent.
func multipartOverhead(boundary, parentID, name, modifiedAt string) (int64, error) {
	var n countingWriter
	mw := multipart.NewWriter(&n)
	if err := mw.SetBoundary(boundary); err != nil {
		return 0, err
	}
	if parentID != "" {
		if err := mw.WriteField("parent_id", parentID); err != nil {
			return 0, err
		}
	}
	if err := mw.WriteField("name", name); err != nil {
		return 0, err
	}
	if modifiedAt != "" {
		if err := mw.WriteField("modified_at", modifiedAt); err != nil {
			return 0, err
		}
	}
	if _, err := mw.CreateFormFile("file", name); err != nil {
		return 0, err
	}
	if err := mw.Close(); err != nil {
		return 0, err
	}
	return int64(n), nil
}

// replayableBody prepares a source that can be read more than once — Upload retries after
// a 401, and by then the first attempt has consumed the reader.
//
// A seekable source (the `put` command hands us an *os.File) is streamed straight from
// disk and rewound between attempts, so uploading a multi-gigabyte file no longer needs a
// copy of it in RAM. Only a source that cannot be rewound is buffered.
//
// Returns a factory for fresh readers and the exact body length.
func replayableBody(r io.Reader) (func() (io.Reader, error), int64, error) {
	if s, ok := r.(io.Seeker); ok {
		if start, err := s.Seek(0, io.SeekCurrent); err == nil {
			if end, err := s.Seek(0, io.SeekEnd); err == nil {
				if _, err := s.Seek(start, io.SeekStart); err == nil {
					first := true
					return func() (io.Reader, error) {
						if first {
							first = false // already positioned; avoid a redundant syscall
						} else if _, err := s.Seek(start, io.SeekStart); err != nil {
							return nil, err
						}
						return r, nil
					}, end - start, nil
				}
			}
		}
		// Seeking failed part-way and the offset may be anywhere now: buffer what is still
		// readable rather than send a truncated body.
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, err
	}
	return func() (io.Reader, error) { return bytes.NewReader(buf), nil }, int64(len(buf)), nil
}

// Upload uploads file name into parentID ("" = root) via multipart. The body is streamed
// through an io.Pipe rather than assembled in memory. modTime is the file's own date,
// which the server stores instead of dating the upload; the zero time sends nothing.
// The bytes are hashed on the way out and checked against the hash the server reports
// for what it stored; a server that reports none is not checked.
func (c *Client) Upload(ctx context.Context, parentID, name string, r io.Reader, modTime time.Time) (Node, error) {
	newBody, size, err := replayableBody(r)
	if err != nil {
		return Node{}, err
	}
	var modField string
	if !modTime.IsZero() {
		modField = modTime.UTC().Format(time.RFC3339Nano)
	}

	for attempt := 0; attempt < 2; attempt++ {
		tok, err := c.token(ctx)
		if err != nil {
			return Node{}, err
		}
		src, err := newBody()
		if err != nil {
			return Node{}, err
		}

		pr, pw := io.Pipe()
		mw := multipart.NewWriter(pw)
		sent := make(chan string, 1) // hex sha256 of the file part, "" if it was not all written
		overhead, err := multipartOverhead(mw.Boundary(), parentID, name, modField)
		if err != nil {
			_ = pw.Close()
			_ = pr.Close()
			return Node{}, err
		}
		// The writer runs alongside the request. Any failure closes the pipe with that
		// error, so hc.Do reports it rather than the server receiving a half-formed body.
		go func() {
			werr := func() error {
				if parentID != "" {
					if err := mw.WriteField("parent_id", parentID); err != nil {
						return err
					}
				}
				if err := mw.WriteField("name", name); err != nil {
					return err
				}
				if modField != "" {
					if err := mw.WriteField("modified_at", modField); err != nil {
						return err
					}
				}
				fw, err := mw.CreateFormFile("file", name)
				if err != nil {
					return err
				}
				h := sha256.New()
				if _, err := io.Copy(fw, io.TeeReader(src, h)); err != nil {
					return err
				}
				if err := mw.Close(); err != nil {
					return err
				}
				sent <- hex.EncodeToString(h.Sum(nil))
				return nil
			}()
			if werr != nil {
				sent <- ""
			}
			_ = pw.CloseWithError(werr)
		}()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/files/upload", pr)
		if err != nil {
			_ = pr.CloseWithError(err) // unblocks the writer goroutine
			return Node{}, err
		}
		req.ContentLength = overhead + size
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := c.hc.Do(req)
		if err != nil {
			return Node{}, err
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			resp.Body.Close()
			c.clearJWT()
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return Node{}, readError(resp, "POST /files/upload")
		}
		var out struct {
			Node       Node `json:"node"`
			Conflicted bool `json:"conflicted"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return Node{}, err
		}
		// A server that answered before reading the whole body leaves the writer blocked:
		// closing the pipe ends it, and the hash is then "" (nothing to compare).
		_ = pr.Close()
		want := <-sent
		if got := strings.ToLower(out.Node.ContentHash); got != "" && want != "" && got != want {
			return out.Node, errors.New(i18n.T("err.upload_hash_mismatch", i18n.M{"Got": got, "Want": want}))
		}
		return out.Node, nil
	}
	return Node{}, errors.New(i18n.T("err.auth_failed"))
}

// SearchHit is a matched node together with its full path.
type SearchHit struct {
	Node Node
	Path string
}

// Search recursively walks the tree under root and returns nodes whose name contains query
// (case-insensitive). The server has no file search — this is a client-side walk of the listing.
func (c *Client) Search(ctx context.Context, root, query string) ([]SearchHit, error) {
	start, err := c.Resolve(ctx, root)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var hits []SearchHit
	var walk func(parentID, prefix string) error
	walk = func(parentID, prefix string) error {
		kids, err := c.ListChildren(ctx, parentID)
		if err != nil {
			return err
		}
		for _, k := range kids {
			p := k.Name
			if prefix != "" {
				p = prefix + "/" + k.Name
			}
			if strings.Contains(strings.ToLower(k.Name), q) {
				hits = append(hits, SearchHit{Node: k, Path: p})
			}
			if k.IsDir {
				if err := walk(k.ID, p); err != nil {
					return err
				}
			}
		}
		return nil
	}
	prefix := strings.Trim(root, "/")
	if prefix == "." {
		prefix = ""
	}
	if err := walk(start.ID, prefix); err != nil {
		return nil, err
	}
	return hits, nil
}
