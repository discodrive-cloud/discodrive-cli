package ddclient

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// APIError is a non-2xx server response. Message is taken from the {"error": ...} body,
// otherwise from the HTTP status.
type APIError struct {
	Status  int
	Message string
	// FromBody is true when Message came from the server's JSON error body, i.e. the
	// DiscoDrive handler itself answered (not a proxy or a missing route).
	FromBody bool
}

func (e *APIError) Error() string {
	return i18n.T("err.server_status", i18n.M{"Status": e.Status, "Message": e.Message})
}

// readError reads the response body and builds an *APIError. what is the context ("GET /files").
func readError(resp *http.Response, what string) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	msg, fromBody := "", false
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(b, &body) == nil {
		msg = body.Error
		fromBody = msg != ""
	}
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
		if msg == "" {
			msg = what
		}
	}
	return &APIError{Status: resp.StatusCode, Message: msg, FromBody: fromBody}
}

// IsNotFound reports whether the error is a 404 from the server.
func IsNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

// IsUnauthorized reports whether the error is a 401 from the server.
func IsUnauthorized(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusUnauthorized
}

// IsAPIStatus reports whether the error is a server response with the given HTTP status.
func IsAPIStatus(err error, status int) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == status
}
