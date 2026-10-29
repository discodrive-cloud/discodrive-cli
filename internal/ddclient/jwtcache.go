package ddclient

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Disk cache for the session JWT, shared between processes. Every dd-cli invocation
// is a fresh process, so without it a bulk operation exchanges the device token once
// per file; with the cache the exchange happens once per JWT lifetime.
// All operations are best-effort: a cache failure must never break the request path.

// jwtMargin: a cached JWT this close to expiry is discarded so it does not expire mid-request.
const jwtMargin = 30 * time.Second

// jwtCachePath returns the cache file for the (server, device-token) pair. The name is
// a hash of the pair so the token itself never appears on disk outside the file body.
func jwtCachePath(serverURL, deviceToken string) (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	sum := sha256.Sum256([]byte(serverURL + "\n" + deviceToken))
	return filepath.Join(base, "discodrive", "jwt-"+hex.EncodeToString(sum[:8])), nil
}

// jwtUsable reports whether tok is a JWT whose exp claim is at least jwtMargin away.
// The signature is not verified — the server does that; here exp only gates the cache.
func jwtUsable(tok string, now time.Time) bool {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Exp == 0 {
		return false
	}
	return time.Unix(claims.Exp, 0).After(now.Add(jwtMargin))
}

// loadCachedJWT returns a still-usable cached JWT for the pair, if any.
func loadCachedJWT(serverURL, deviceToken string) (string, bool) {
	p, err := jwtCachePath(serverURL, deviceToken)
	if err != nil {
		return "", false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	tok := strings.TrimSpace(string(b))
	if !jwtUsable(tok, time.Now()) {
		return "", false
	}
	return tok, true
}

// storeCachedJWT persists the JWT (dir 0700, file 0600, atomic rename against
// concurrent dd-cli processes writing the same pair). Tokens that loadCachedJWT
// would reject anyway (no usable exp claim) are not written.
func storeCachedJWT(serverURL, deviceToken, tok string) {
	if !jwtUsable(tok, time.Now()) {
		return
	}
	p, err := jwtCachePath(serverURL, deviceToken)
	if err != nil {
		return
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "jwt-*.tmp")
	if err != nil {
		return
	}
	if _, err := tmp.WriteString(tok); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return
	}
	_ = os.Rename(tmp.Name(), p)
}

// dropCachedJWT removes the cached JWT for the pair (called on 401).
func dropCachedJWT(serverURL, deviceToken string) {
	if p, err := jwtCachePath(serverURL, deviceToken); err == nil {
		_ = os.Remove(p)
	}
}
