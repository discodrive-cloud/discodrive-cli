package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
)

// mutateForID sends a JSON body via method to path, expects okStatus, and decodes a
// response of the form {key: "<id>"} (the server returns only the identifier on create/update).
func (c *Client) mutateForID(ctx context.Context, method, path string, body any, okStatus int, key string) (string, error) {
	resp, err := c.doJSON(ctx, method, path, body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != okStatus {
		return "", readError(resp, method+" "+path)
	}
	var out map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out[key], nil
}

// expect204 closes the response and checks that the status is 204/200 (for DELETE).
func expect204(resp *http.Response, what string) error {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return readError(resp, what)
	}
	return nil
}
