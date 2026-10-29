package ddclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// httpClient: strict TLS by default; insecure=true accepts any certificate —
// an explicit opt-in (login --insecure, stored in the profile) for self-signed/LAN servers.
func httpClient(insecure bool) *http.Client {
	if insecure {
		return &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // explicit opt-in
		}}
	}
	return http.DefaultClient
}

// Cleartext reports whether serverURL sends the device token unencrypted over a
// network: plain http to anything but a loopback address. Login warns about it.
func Cleartext(serverURL string) bool {
	u, err := url.Parse(serverURL)
	if err != nil || !strings.EqualFold(u.Scheme, "http") {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}

// Option customizes the Client created by New.
type Option func(*Client)

// WithInsecureTLS makes the client accept any TLS certificate (self-signed servers).
func WithInsecureTLS() Option {
	return func(c *Client) { c.hc = httpClient(true) }
}

// WithJWTCache shares the session JWT between processes via a disk cache, so bulk
// dd-cli runs (one process per file) exchange the device token once per JWT lifetime
// instead of once per invocation. Enabled by the CLI/MCP entry points; unit tests
// use a bare client and never touch the disk.
func WithJWTCache() Option {
	return func(c *Client) { c.cacheJWT = true }
}

// Client talks to the DiscoDrive server using a device-token and caches the session JWT.
type Client struct {
	baseURL     string
	deviceToken string
	hc          *http.Client
	cacheJWT    bool

	mu  sync.Mutex
	jwt string
}

// New creates a client for the baseURL server with a device-token (the CLI works with the whole storage).
func New(baseURL, deviceToken string, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, deviceToken: deviceToken, hc: httpClient(false)}
	for _, o := range opts {
		o(c)
	}
	return c
}

// BaseURL is the address of the server the client is bound to.
func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.jwt != "" {
		return c.jwt, nil
	}
	if c.cacheJWT {
		if tok, ok := loadCachedJWT(c.baseURL, c.deviceToken); ok {
			c.jwt = tok
			return tok, nil
		}
	}
	body, _ := json.Marshal(map[string]string{"device_token": c.deviceToken})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/device/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", readError(resp, "POST /auth/device/token")
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	c.jwt = out.Token
	if c.cacheJWT {
		storeCachedJWT(c.baseURL, c.deviceToken, out.Token)
	}
	return c.jwt, nil
}

// clearJWT forgets the session JWT in memory and on disk (the server said 401 —
// a stale cached token must not be re-read on the retry).
func (c *Client) clearJWT() {
	c.mu.Lock()
	c.jwt = ""
	c.mu.Unlock()
	if c.cacheJWT {
		dropCachedJWT(c.baseURL, c.deviceToken)
	}
}

// do performs a request without a body (GET/DELETE), adding the Bearer header and retrying once on 401.
func (c *Client) do(ctx context.Context, method, path string) (*http.Response, error) {
	return c.doReq(ctx, method, path, nil, "")
}

// doJSON sends a JSON body with the Bearer header, retrying once on 401.
func (c *Client) doJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	raw, _ := json.Marshal(body)
	return c.doReq(ctx, method, path, raw, "application/json")
}

// doReq is the shared path for do/doJSON: the body may be nil. The body is re-read on retry.
func (c *Client) doReq(ctx context.Context, method, path string, body []byte, contentType string) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		tok, err := c.token(ctx)
		if err != nil {
			return nil, err
		}
		var rdr *bytes.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		var req *http.Request
		if rdr != nil {
			req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
		} else {
			req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
		}
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			resp.Body.Close()
			c.clearJWT()
			continue
		}
		return resp, nil
	}
	return nil, errors.New(i18n.T("err.auth_failed"))
}
