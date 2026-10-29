package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
)

// User is the current user (GET /me).
type User struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	TenantID           string `json:"tenant_id"`
	MustChangePassword bool   `json:"must_change_password"`
}

// Me returns the current user.
func (c *Client) Me(ctx context.Context) (User, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me")
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return User{}, readError(resp, "GET /me")
	}
	var u User
	return u, json.NewDecoder(resp.Body).Decode(&u)
}
