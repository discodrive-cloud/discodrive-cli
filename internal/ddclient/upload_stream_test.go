package ddclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// uploadTestServer answers the token exchange and parses POST /files/upload as real
// multipart, recording the file part, the modified_at field and the declared length.
// firstUnauthorized makes the first attempt return 401 to drive the retry path.
func uploadTestServer(t *testing.T, firstUnauthorized bool) (*httptest.Server, *[][]byte, *[]int64, *string) {
	t.Helper()
	var files [][]byte
	var lengths []int64
	var modField string
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "jwt"})
	})
	mux.HandleFunc("POST /files/upload", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if firstUnauthorized && attempts == 1 {
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		length := r.ContentLength
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		var content []byte
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				// A truncated body fails to parse — record nothing, as a real server would.
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			switch p.FormName() {
			case "modified_at":
				b, _ := io.ReadAll(p)
				modField = string(b)
			case "file":
				content, _ = io.ReadAll(p)
			default:
				_, _ = io.Copy(io.Discard, p)
			}
		}
		files = append(files, content)
		lengths = append(lengths, length)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "n1", "name": "x", "is_dir": false, "version": 1})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &files, &lengths, &modField
}

// TestUploadStreamsExactBytes: the streamed multipart body round-trips through a real
// parser byte-for-byte, and Content-Length is exact rather than chunked.
func TestUploadStreamsExactBytes(t *testing.T) {
	payload := bytes.Repeat([]byte("xy"), 400_000) // 800 KB
	path := filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	srv, files, lengths, _ := uploadTestServer(t, false)
	c := New(srv.URL, "dt")
	if _, err := c.Upload(context.Background(), "p1", "big.bin", f, time.Time{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if len(*files) != 1 {
		t.Fatalf("server accepted %d uploads, want 1", len(*files))
	}
	if !bytes.Equal((*files)[0], payload) {
		t.Fatalf("file part is %d bytes, want %d, and the contents differ", len((*files)[0]), len(payload))
	}
	if (*lengths)[0] <= int64(len(payload)) {
		t.Fatalf("Content-Length = %d must exceed the %d-byte payload by the multipart envelope; "+
			"0 or -1 means the body went out chunked", (*lengths)[0], len(payload))
	}
}

// TestUploadResendsWholeBodyAfterUnauthorized: the 401 retry has to rebuild the whole
// multipart body from a rewound source, not send the remains of a consumed reader.
func TestUploadResendsWholeBodyAfterUnauthorized(t *testing.T) {
	payload := bytes.Repeat([]byte("z"), 300_000)
	path := filepath.Join(t.TempDir(), "r.bin")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	srv, files, _, _ := uploadTestServer(t, true)
	c := New(srv.URL, "dt")
	if _, err := c.Upload(context.Background(), "", "r.bin", f, time.Time{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if len(*files) != 1 {
		t.Fatalf("server accepted %d uploads, want 1 after the 401 retry", len(*files))
	}
	if !bytes.Equal((*files)[0], payload) {
		t.Fatalf("retry sent %d bytes, want %d: the source was not rewound", len((*files)[0]), len(payload))
	}
}

// TestUploadSendsModifiedAt also guards the Content-Length arithmetic: the extra field has
// to be counted by multipartOverhead as well as written, or the declared length stops
// matching the body and the request fails outright.
func TestUploadSendsModifiedAt(t *testing.T) {
	srv, _, _, modField := uploadTestServer(t, false)
	c := New(srv.URL, "dt")

	want := time.Date(2019, 6, 15, 12, 30, 0, 0, time.UTC)
	if _, err := c.Upload(context.Background(), "", "a.txt", bytes.NewReader([]byte("hi")), want); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	got, err := time.Parse(time.RFC3339Nano, *modField)
	if err != nil {
		t.Fatalf("modified_at = %q, not RFC3339: %v", *modField, err)
	}
	if !got.Equal(want) {
		t.Fatalf("modified_at = %s, want %s", got, want)
	}
}

// TestUploadOmitsZeroModifiedAt: nothing to declare means no field at all.
func TestUploadOmitsZeroModifiedAt(t *testing.T) {
	srv, _, _, modField := uploadTestServer(t, false)
	c := New(srv.URL, "dt")

	if _, err := c.Upload(context.Background(), "", "a.txt", bytes.NewReader([]byte("hi")), time.Time{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if *modField != "" {
		t.Fatalf("modified_at = %q was sent for a zero time", *modField)
	}
}

// failingSeeker is seekable (so replayableBody streams instead of buffering) but fails
// part-way through reading — a disk read error during upload.
type failingSeeker struct {
	size int64
	off  int64
	fail int64
}

func (f *failingSeeker) Read(p []byte) (int, error) {
	if f.off >= f.fail {
		return 0, errors.New("disk read error")
	}
	n := int64(len(p))
	if f.off+n > f.fail {
		n = f.fail - f.off
	}
	for i := int64(0); i < n; i++ {
		p[i] = 'a'
	}
	f.off += n
	return int(n), nil
}

func (f *failingSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.off = offset
	case io.SeekCurrent:
		f.off += offset
	case io.SeekEnd:
		f.off = f.size + offset
	}
	return f.off, nil
}

// TestUploadReportsSourceReadError is what makes streaming safe: a source that dies
// mid-body must fail the upload loudly. Silently sending a short body would land a
// truncated file that the server accepts as complete.
func TestUploadReportsSourceReadError(t *testing.T) {
	srv, files, _, _ := uploadTestServer(t, false)
	c := New(srv.URL, "dt")

	src := &failingSeeker{size: 500_000, fail: 100_000}
	if _, err := c.Upload(context.Background(), "", "broken.bin", src, time.Time{}); err == nil {
		t.Fatal("upload of a source that fails mid-read must return an error")
	}
	if len(*files) != 0 {
		t.Fatalf("server accepted %d uploads for a failed source, want 0", len(*files))
	}
}

// TestUploadChecksStoredHash: the hash the server reports for what it stored must match
// the bytes sent; a mismatch is an error, a matching or missing hash is not.
func TestUploadChecksStoredHash(t *testing.T) {
	payload := bytes.Repeat([]byte("q"), 200_000)
	sum := sha256.Sum256(payload)
	good := hex.EncodeToString(sum[:])
	report := ""
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "jwt"})
	})
	mux.HandleFunc("POST /files/upload", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"node": map[string]any{
			"id": "n1", "name": "x", "is_dir": false, "version": 1, "content_hash": report}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := New(srv.URL, "dt")
	ctx := context.Background()

	for _, h := range []string{good, strings.ToUpper(good), ""} {
		report = h
		n, err := c.Upload(ctx, "", "x", bytes.NewReader(payload), time.Time{})
		if err != nil {
			t.Fatalf("reported %q: %v", h, err)
		}
		if n.ContentHash != h {
			t.Fatalf("node hash %q, want %q", n.ContentHash, h)
		}
	}
	report = strings.Repeat("0", 64)
	if _, err := c.Upload(ctx, "", "x", bytes.NewReader(payload), time.Time{}); err == nil || !strings.Contains(err.Error(), good) {
		t.Fatalf("mismatch must fail naming the sent hash, got %v", err)
	}
}
