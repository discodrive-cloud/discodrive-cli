package ddclient

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func resp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func TestReadErrorDecodesMessage(t *testing.T) {
	err := readError(resp(404, `{"error":"not found"}`), "GET /files")
	var ae *APIError
	if !asAPIError(err, &ae) || ae.Status != 404 || ae.Message != "not found" {
		t.Fatalf("got %#v", err)
	}
	if !IsNotFound(err) {
		t.Fatal("IsNotFound should be true")
	}
	if !strings.Contains(ae.Error(), "not found") {
		t.Fatalf("Error() = %q", ae.Error())
	}
}

func TestReadErrorFallbackWhenNoJSON(t *testing.T) {
	err := readError(resp(500, `boom`), "GET /files")
	var ae *APIError
	if !asAPIError(err, &ae) || ae.Status != 500 {
		t.Fatalf("got %#v", err)
	}
	if ae.Message == "" {
		t.Fatal("expected a non-empty fallback message")
	}
}

// asAPIError is a local helper used instead of errors.As so the test is self-contained.
func asAPIError(err error, target **APIError) bool {
	ae, ok := err.(*APIError)
	if ok {
		*target = ae
	}
	return ok
}
