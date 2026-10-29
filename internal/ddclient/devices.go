package ddclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Unpair revokes the CURRENT device on the server (DELETE /devices/{id}), breaking the pairing.
// The device ID is taken from the "did" claim of the current session's JWT. After a successful
// revocation the device-token becomes invalid — the caller should forget the local profile.
func (c *Client) Unpair(ctx context.Context) error {
	deviceID, err := c.currentDeviceID(ctx)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodDelete, "/devices/"+url.PathEscape(deviceID))
	if err != nil {
		return err
	}
	return expect204(resp, "DELETE /devices/{id}")
}

// currentDeviceID extracts the device_id from the current session's JWT payload (the "did" claim).
// The signature is NOT verified — the token came from the server itself, and the server
// re-verifies the DELETE request; here we only need the identifier for the route.
func (c *Client) currentDeviceID(ctx context.Context) (string, error) {
	jwt, err := c.token(ctx)
	if err != nil {
		return "", err
	}
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return "", errors.New(i18n.T("err.session_token_invalid"))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("%s: %w", i18n.T("err.session_token_parse"), err)
	}
	var claims struct {
		Did string `json:"did"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("%s: %w", i18n.T("err.session_token_parse"), err)
	}
	if claims.Did == "" {
		return "", errors.New(i18n.T("err.session_token_no_device"))
	}
	return claims.Did, nil
}
