package ddclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Chunked upload over the server's resumable API: POST /upload/init, then
// PUT /upload/{id}/chunk/{n} in order, then POST /upload/{id}/complete. The web app uses
// the same API with the same chunk size. Variables rather than constants so tests can
// move kilobytes instead of gigabytes.
var (
	// chunkSize is the body of one PUT; well under the 100 MB a Cloudflare-proxied
	// request may carry, and one chunk is all a lost connection costs.
	chunkSize int64 = 8 << 20
	// chunkedThreshold is the size above which Upload skips the one-shot request.
	chunkedThreshold int64 = 64 << 20
	// chunkRetryDelay is the first pause before resending a failed chunk; it doubles.
	chunkRetryDelay = time.Second
)

// maxChunkAttempts bounds how often one chunk is sent before the upload gives up.
const maxChunkAttempts = 5

// maxRetryAfter caps a server-requested pause, so a bogus Retry-After cannot stall us.
const maxRetryAfter = 30 * time.Second

func (c *Client) uploadChunked(ctx context.Context, parentID, name string, newBody func() (io.Reader, error), size int64, modField string) (Node, error) {
	src, err := newBody()
	if err != nil {
		return Node{}, err
	}
	init := map[string]any{"name": name, "size": size}
	if parentID != "" {
		init["parent_id"] = parentID
	}
	if modField != "" {
		init["modified_at"] = modField
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/upload/init", init)
	if err != nil {
		return Node{}, err
	}
	var started struct {
		UploadID string `json:"upload_id"`
	}
	err = decodeStatus(resp, "POST /upload/init", http.StatusCreated, &started)
	if err != nil {
		return Node{}, err
	}
	if started.UploadID == "" {
		return Node{}, errors.New("POST /upload/init: no upload_id in the reply")
	}
	base := "/upload/" + url.PathEscape(started.UploadID)

	done := false
	defer func() {
		if done {
			return
		}
		// Drop the session so its staged bytes and quota reservation go now rather than
		// when the server's GC finds it. Best effort, and not bound to a cancelled ctx.
		actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if resp, err := c.do(actx, http.MethodDelete, base); err == nil {
			resp.Body.Close()
		}
	}()

	h := sha256.New()
	buf := make([]byte, min(chunkSize, max(size, 1)))
	var sent int64
	for n := 0; sent < size; n++ {
		k, err := io.ReadFull(src, buf[:min(int64(len(buf)), size-sent)])
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return Node{}, errors.New(i18n.T("err.upload_source_changed", i18n.M{"Sent": sent + int64(k), "Size": size}))
			}
			return Node{}, err
		}
		h.Write(buf[:k])
		if err := c.putChunk(ctx, base, n, buf[:k]); err != nil {
			return Node{}, err
		}
		sent += int64(k)
	}
	// The source must end where it was measured: a file still being written would
	// otherwise land as a silently truncated copy.
	if k, _ := src.Read(buf[:1]); k > 0 {
		return Node{}, errors.New(i18n.T("err.upload_source_changed", i18n.M{"Sent": sent, "Size": size}))
	}

	resp, err = c.doReq(ctx, http.MethodPost, base+"/complete", nil, "")
	if err != nil {
		return Node{}, err
	}
	var out struct {
		Node       Node `json:"node"`
		Conflicted bool `json:"conflicted"`
	}
	if err := decodeStatus(resp, "POST /upload/{id}/complete", http.StatusCreated, &out); err != nil {
		return Node{}, err
	}
	done = true // the server consumed the session
	if err := checkStoredHash(out.Node, hex.EncodeToString(h.Sum(nil))); err != nil {
		return out.Node, err
	}
	return out.Node, nil
}

// putChunk sends chunk n, retrying a lost connection or a transient server answer. A
// chunk the server already took (its reply was lost) is acknowledged on resend, so a
// retry never duplicates bytes.
func (c *Client) putChunk(ctx context.Context, base string, n int, data []byte) error {
	path := base + "/chunk/" + strconv.Itoa(n)
	delay := chunkRetryDelay
	var last error
	for attempt := range maxChunkAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
		resp, err := c.doReq(ctx, http.MethodPut, path, data, "application/octet-stream")
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			last = err
			continue
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			resp.Body.Close()
			return nil
		case resp.StatusCode == http.StatusConflict:
			// Out of order: only "the server is already past this chunk" is fine; the
			// chunks go out strictly in sequence, so anything else is a lost session.
			var body struct {
				NextChunk *int `json:"next_chunk"`
			}
			_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body)
			resp.Body.Close()
			if body.NextChunk != nil && *body.NextChunk > n {
				return nil
			}
			return &APIError{Status: http.StatusConflict, Message: fmt.Sprintf("chunk %d out of order", n)}
		case retryableStatus(resp.StatusCode):
			if ra := retryAfter(resp); ra > delay {
				delay = ra
			}
			last = readError(resp, "PUT /upload/{id}/chunk/{n}")
			resp.Body.Close()
			continue
		default:
			err := readError(resp, "PUT /upload/{id}/chunk/{n}")
			resp.Body.Close()
			return err
		}
	}
	return last
}

// retryableStatus: answers that say "try again" rather than "this is wrong" — overload,
// the server's upload slots being busy (429), and gateway/proxy failures, including
// Cloudflare's 520–524 for an origin that dropped or timed out.
func retryableStatus(s int) bool {
	switch s {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return s >= 520 && s <= 524
}

func retryAfter(resp *http.Response) time.Duration {
	secs, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || secs <= 0 {
		return 0
	}
	return min(time.Duration(secs)*time.Second, maxRetryAfter)
}

// decodeStatus closes resp after decoding its JSON body into out when the status is ok,
// or turning it into an *APIError otherwise.
func decodeStatus(resp *http.Response, what string, ok int, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode != ok {
		return readError(resp, what)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
