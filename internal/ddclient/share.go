package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// ShareOpts holds the share parameters: either Email (grant a user access) or Link (a public link).
type ShareOpts struct {
	Email            string
	Link             bool
	Access           string // "view" | "edit"
	ExpiresInSeconds *int64
}

// ShareResult is the server response: the share id and (for Link) the public-link token.
type ShareResult struct {
	ShareID string
	Token   string
	Access  string
}

// Share grants access to node nodeID. Access defaults to "view".
func (c *Client) Share(ctx context.Context, nodeID string, opts ShareOpts) (ShareResult, error) {
	if opts.Access == "" {
		opts.Access = "view"
	}
	body := map[string]any{"access": opts.Access}
	if opts.Link {
		body["link"] = true
	}
	if opts.Email != "" {
		body["email"] = opts.Email
	}
	if opts.ExpiresInSeconds != nil {
		body["expires_in_seconds"] = *opts.ExpiresInSeconds
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/files/"+url.PathEscape(nodeID)+"/share", body)
	if err != nil {
		return ShareResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return ShareResult{}, readError(resp, "POST /files/{id}/share")
	}
	var out struct {
		ShareID string `json:"share_id"`
		Token   string `json:"token"`
		Access  string `json:"access"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ShareResult{}, err
	}
	return ShareResult(out), nil
}

// ShareLinkURL builds the public share URL from a token. Knowledge of the /s/ route lives in ddclient.
func (c *Client) ShareLinkURL(token string) string {
	return c.baseURL + "/s/" + token
}
