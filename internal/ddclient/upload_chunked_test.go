package ddclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// chunkServer imitates the server's resumable upload API (/upload/*) closely enough to
// drive the client: chunks must arrive in order, a resent chunk that was already taken
// is acknowledged, and Complete refuses a size that differs from the one declared.
type chunkServer struct {
	mu sync.Mutex
	t  *testing.T

	// knobs
	multipartStatus int                       // non-zero: POST /files/upload answers this
	failChunk       func(n, attempt int) int  // non-zero status: answer it instead of taking the chunk
	acceptThenFail  func(n, attempt int) bool // take the chunk, then answer 500 as if the reply was lost
	badHash         bool

	// observations
	multipartHits int
	inits         []map[string]any
	data          []byte
	declared      int64
	next          int
	chunkSizes    []int
	attempts      map[int]int
	aborted       int
	completed     int
}

func newChunkServer(t *testing.T) (*chunkServer, *httptest.Server) {
	cs := &chunkServer{t: t, attempts: map[int]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "jwt"})
	})
	mux.HandleFunc("POST /files/upload", func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		cs.multipartHits++
		st := cs.multipartStatus
		cs.mu.Unlock()
		if st != 0 {
			// Like Cloudflare: refuse on the declared length, without reading the body.
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(st)
			_, _ = io.WriteString(w, "<html><head><title>413 Payload Too Large</title></head></html>")
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"node": map[string]any{"id": "m1", "name": "x", "version": 1}})
	})
	mux.HandleFunc("POST /upload/init", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		cs.mu.Lock()
		cs.inits = append(cs.inits, body)
		size, _ := body["size"].(float64)
		cs.declared = int64(size)
		cs.data, cs.next = nil, 0
		cs.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"upload_id": "u1", "next_chunk": 0})
	})
	mux.HandleFunc("PUT /upload/{id}/chunk/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.PathValue("n"))
		b, _ := io.ReadAll(r.Body)
		cs.mu.Lock()
		defer cs.mu.Unlock()
		attempt := cs.attempts[n]
		cs.attempts[n]++
		if cs.failChunk != nil {
			if st := cs.failChunk(n, attempt); st != 0 {
				w.WriteHeader(st)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "nope"})
				return
			}
		}
		switch {
		case n < cs.next:
			_ = json.NewEncoder(w).Encode(map[string]any{"next_chunk": cs.next})
			return
		case n > cs.next:
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "chunk out of order", "next_chunk": cs.next})
			return
		}
		cs.data = append(cs.data, b...)
		cs.chunkSizes = append(cs.chunkSizes, len(b))
		cs.next++
		if cs.acceptThenFail != nil && cs.acceptThenFail(n, attempt) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"next_chunk": cs.next})
	})
	mux.HandleFunc("POST /upload/{id}/complete", func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		defer cs.mu.Unlock()
		cs.completed++
		if cs.declared > 0 && int64(len(cs.data)) != cs.declared {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "upload size mismatch"})
			return
		}
		sum := sha256.Sum256(cs.data)
		h := hex.EncodeToString(sum[:])
		if cs.badHash {
			h = strings.Repeat("0", 64)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"node": map[string]any{
			"id": "c1", "name": "x", "version": 1, "content_hash": h}, "conflicted": false})
	})
	mux.HandleFunc("DELETE /upload/{id}", func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		cs.aborted++
		cs.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return cs, srv
}

// smallChunks shrinks the chunking constants so tests move kilobytes, not gigabytes.
func smallChunks(t *testing.T, size, threshold int64) {
	t.Helper()
	oldSize, oldThreshold, oldDelay := chunkSize, chunkedThreshold, chunkRetryDelay
	chunkSize, chunkedThreshold, chunkRetryDelay = size, threshold, 0
	t.Cleanup(func() { chunkSize, chunkedThreshold, chunkRetryDelay = oldSize, oldThreshold, oldDelay })
}

func pattern(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + i/251)
	}
	return b
}

// A file above the threshold goes straight to chunks: the one-shot request would only be
// refused by a proxy (Cloudflare caps a request body at 100 MB) after a wasted round trip.
func TestUploadLargeGoesChunked(t *testing.T) {
	smallChunks(t, 1000, 4000)
	cs, srv := newChunkServer(t)
	payload := pattern(4500)
	mod := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

	n, err := New(srv.URL, "dt").Upload(context.Background(), "p1", "video.mp4", bytes.NewReader(payload), mod)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if n.ID != "c1" {
		t.Fatalf("node %+v, want the one Complete returned", n)
	}
	if cs.multipartHits != 0 {
		t.Fatalf("POST /files/upload hit %d times for a file above the threshold", cs.multipartHits)
	}
	if !bytes.Equal(cs.data, payload) {
		t.Fatalf("assembled %d bytes, want %d identical", len(cs.data), len(payload))
	}
	if want := []int{1000, 1000, 1000, 1000, 500}; !equalInts(cs.chunkSizes, want) {
		t.Fatalf("chunk sizes %v, want %v", cs.chunkSizes, want)
	}
	init := cs.inits[0]
	if init["parent_id"] != "p1" || init["name"] != "video.mp4" || init["size"] != float64(4500) {
		t.Fatalf("init body %v", init)
	}
	got, _ := time.Parse(time.RFC3339Nano, init["modified_at"].(string))
	if !got.Equal(mod) {
		t.Fatalf("modified_at %v, want %v", init["modified_at"], mod)
	}
}

// Root uploads send no parent_id and no modified_at for a zero time.
func TestUploadChunkedRootOmitsOptionalFields(t *testing.T) {
	smallChunks(t, 1000, 100)
	cs, srv := newChunkServer(t)
	if _, err := New(srv.URL, "dt").Upload(context.Background(), "", "a.bin", bytes.NewReader(pattern(300)), time.Time{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if _, ok := cs.inits[0]["parent_id"]; ok {
		t.Fatalf("parent_id sent for root: %v", cs.inits[0])
	}
	if _, ok := cs.inits[0]["modified_at"]; ok {
		t.Fatalf("modified_at sent for a zero time: %v", cs.inits[0])
	}
}

// Below the threshold the file goes in one request; a 413 from whatever stands in front
// of the server turns it into a chunked upload of the whole file.
func TestUploadFallsBackToChunksOn413(t *testing.T) {
	smallChunks(t, 1000, 1<<20)
	cs, srv := newChunkServer(t)
	cs.multipartStatus = http.StatusRequestEntityTooLarge
	payload := pattern(2500)

	if _, err := New(srv.URL, "dt").Upload(context.Background(), "", "x.bin", bytes.NewReader(payload), time.Time{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if cs.multipartHits != 1 {
		t.Fatalf("one-shot attempts %d, want 1", cs.multipartHits)
	}
	if !bytes.Equal(cs.data, payload) {
		t.Fatalf("assembled %d bytes, want %d identical", len(cs.data), len(payload))
	}
}

// The fallback must rewind a file source too: the refused attempt may have read part of it.
func TestUploadFallbackRewindsFile(t *testing.T) {
	smallChunks(t, 64<<10, 1<<30)
	cs, srv := newChunkServer(t)
	cs.multipartStatus = http.StatusRequestEntityTooLarge
	payload := pattern(300_000)
	f := tempFile(t, payload)

	if _, err := New(srv.URL, "dt").Upload(context.Background(), "", "f.bin", f, time.Time{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !bytes.Equal(cs.data, payload) {
		t.Fatalf("assembled %d bytes, want %d identical", len(cs.data), len(payload))
	}
}

// Other one-shot failures are not retried as chunks.
func TestUploadNo413NoFallback(t *testing.T) {
	smallChunks(t, 1000, 1<<20)
	cs, srv := newChunkServer(t)
	cs.multipartStatus = http.StatusInsufficientStorage
	if _, err := New(srv.URL, "dt").Upload(context.Background(), "", "x", bytes.NewReader(pattern(10)), time.Time{}); !IsAPIStatus(err, 507) {
		t.Fatalf("err = %v, want the 507", err)
	}
	if len(cs.inits) != 0 {
		t.Fatal("a 507 must not start a chunked upload")
	}
}

// A transient failure of one chunk retries that chunk alone.
func TestUploadChunkRetriesTransient(t *testing.T) {
	smallChunks(t, 1000, 100)
	cs, srv := newChunkServer(t)
	cs.failChunk = func(n, attempt int) int {
		if n == 1 && attempt < 2 {
			return http.StatusBadGateway
		}
		return 0
	}
	payload := pattern(3000)
	if _, err := New(srv.URL, "dt").Upload(context.Background(), "", "x", bytes.NewReader(payload), time.Time{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !bytes.Equal(cs.data, payload) {
		t.Fatalf("assembled %d bytes, want %d identical", len(cs.data), len(payload))
	}
	if cs.attempts[1] != 3 {
		t.Fatalf("chunk 1 sent %d times, want 3", cs.attempts[1])
	}
}

// A chunk the server took but whose reply was lost is resent; the server acknowledges it
// without appending it twice.
func TestUploadChunkResentAfterLostReply(t *testing.T) {
	smallChunks(t, 1000, 100)
	cs, srv := newChunkServer(t)
	cs.acceptThenFail = func(n, attempt int) bool { return n == 0 && attempt == 0 }
	payload := pattern(2000)
	if _, err := New(srv.URL, "dt").Upload(context.Background(), "", "x", bytes.NewReader(payload), time.Time{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !bytes.Equal(cs.data, payload) {
		t.Fatalf("assembled %d bytes, want %d identical", len(cs.data), len(payload))
	}
}

// A chunk that keeps failing, or fails for good, ends the upload and drops the session.
func TestUploadChunkGivesUpAndAborts(t *testing.T) {
	for name, fail := range map[string]func(n, attempt int) int{
		"permanent": func(n, attempt int) int {
			if n == 1 {
				return http.StatusInsufficientStorage
			}
			return 0
		},
		"persistent": func(n, attempt int) int {
			if n == 1 {
				return http.StatusServiceUnavailable
			}
			return 0
		},
	} {
		t.Run(name, func(t *testing.T) {
			smallChunks(t, 1000, 100)
			cs, srv := newChunkServer(t)
			cs.failChunk = fail
			if _, err := New(srv.URL, "dt").Upload(context.Background(), "", "x", bytes.NewReader(pattern(3000)), time.Time{}); err == nil {
				t.Fatal("Upload succeeded with a failing chunk")
			}
			if cs.aborted != 1 || cs.completed != 0 {
				t.Fatalf("aborted %d, completed %d; want the session dropped, not completed", cs.aborted, cs.completed)
			}
			if name == "permanent" && cs.attempts[1] != 1 {
				t.Fatalf("a 507 was retried %d times", cs.attempts[1]-1)
			}
		})
	}
}

// The bytes the server assembled must hash to what was sent.
func TestUploadChunkedChecksHash(t *testing.T) {
	smallChunks(t, 1000, 100)
	cs, srv := newChunkServer(t)
	cs.badHash = true
	payload := pattern(1500)
	sum := sha256.Sum256(payload)
	_, err := New(srv.URL, "dt").Upload(context.Background(), "", "x", bytes.NewReader(payload), time.Time{})
	if err == nil || !strings.Contains(err.Error(), hex.EncodeToString(sum[:])) {
		t.Fatalf("mismatch must fail naming the sent hash, got %v", err)
	}
}

// A source that fails mid-read must not complete the session with a short file.
func TestUploadChunkedSourceReadError(t *testing.T) {
	smallChunks(t, 1000, 100)
	cs, srv := newChunkServer(t)
	src := &failingSeeker{size: 5000, fail: 2500}
	if _, err := New(srv.URL, "dt").Upload(context.Background(), "", "x", src, time.Time{}); err == nil {
		t.Fatal("upload of a failing source must return an error")
	}
	if cs.completed != 0 || cs.aborted != 1 {
		t.Fatalf("completed %d, aborted %d; want the session dropped", cs.completed, cs.aborted)
	}
}

func tempFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "src.bin")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
