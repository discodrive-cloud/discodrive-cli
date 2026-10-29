package ddmcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReviewNetworkAddressRequiresAuth(t *testing.T) {
	for _, addr := range []string{"192.168.1.10:8080", "[2001:db8::1]:8080", "drive.example:8080"} {
		if !networkExposed(normalizeAddr(addr)) {
			t.Errorf("network address %s classified as local", addr)
		}
	}
}
func TestReviewUploadIsGated(t *testing.T) {
	if names(selectTools(toolSpecs(), false))["dd_files_upload"] {
		t.Fatal("overwrite-capable upload exposed without allow-changes")
	}
}

func TestReviewFileReadRejectsOversizeContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"token":"jwt"}`) })
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":"f1","name":"large.log","size":1}]`)
	})
	mux.HandleFunc("GET /files/f1/content", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, strings.Repeat("x", 2<<20)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res := callTool(t, srv.URL, "dd_files_read", map[string]any{"path": "large.log"})
	if !res.IsError {
		t.Fatal("oversize content returned without error")
	}
}
