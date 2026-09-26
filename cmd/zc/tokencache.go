package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Zoho lets one refresh token mint only about ten access tokens in ten
// minutes, and every zc invocation is a fresh process. Without a cache, a
// short run of commands exhausts that cap and Zoho answers "too many
// requests" for the rest of the window. The cache keeps the current access
// token on disk so later runs reuse it until it nears expiry.

// cachedToken is the on-disk token file.
type cachedToken struct {
	AccessToken string    `json:"access_token"`
	Expiry      time.Time `json:"expiry"`
}

// tokenCachePath returns the cache file for one credential set, or "" when
// caching is off. The file name is a hash of the data center, client ID, and
// refresh token, so switching credentials never reuses another set's token
// and the name itself reveals nothing.
func tokenCachePath(dc, clientID, refreshToken string) string {
	if refreshToken == "" || cacheDisabled(os.Getenv("ZC_NO_TOKEN_CACHE")) {
		return ""
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(dc + "\x00" + clientID + "\x00" + refreshToken))
	return filepath.Join(dir, "zc", "token-"+hex.EncodeToString(sum[:8])+".json")
}

// cacheDisabled reports whether ZC_NO_TOKEN_CACHE turns the cache off. Any
// value turns it off except an empty one or one strconv reads as false, so
// "yes" and "on" still opt out while "0" and "false" keep caching on.
func cacheDisabled(v string) bool {
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	return err != nil || b
}

// loadToken returns the cached token when the file exists and the token has
// not expired. Any read or decode problem is treated as a cache miss.
func loadToken(path string) (cachedToken, bool) {
	if path == "" {
		return cachedToken{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cachedToken{}, false
	}
	var ct cachedToken
	if json.Unmarshal(b, &ct) != nil || ct.AccessToken == "" || !time.Now().Before(ct.Expiry) {
		return cachedToken{}, false
	}
	return ct, true
}

// storeToken writes the token with owner-only permissions, via a temp file
// and rename so a concurrent zc never reads a half-written file.
func storeToken(path string, ct cachedToken) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create token cache dir: %w", err)
	}
	b, err := json.Marshal(ct)
	if err != nil {
		return fmt.Errorf("encode token cache: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".token-*")
	if err != nil {
		return fmt.Errorf("create token cache temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write token cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close token cache: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("install token cache: %w", err)
	}
	return nil
}
