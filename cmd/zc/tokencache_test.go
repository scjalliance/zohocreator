package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zc", "token-x.json")
	want := cachedToken{AccessToken: "tok", Expiry: time.Now().Add(time.Hour).Round(0)}
	if err := storeToken(path, want); err != nil {
		t.Fatalf("storeToken: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %v, want 0600", perm)
	}
	got, ok := loadToken(path)
	if !ok || got.AccessToken != want.AccessToken || !got.Expiry.Equal(want.Expiry) {
		t.Fatalf("loadToken = (%+v, %v), want (%+v, true)", got, ok, want)
	}
}

func TestTokenCacheMisses(t *testing.T) {
	dir := t.TempDir()
	expired := filepath.Join(dir, "expired.json")
	if err := storeToken(expired, cachedToken{AccessToken: "old", Expiry: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	garbage := filepath.Join(dir, "garbage.json")
	if err := os.WriteFile(garbage, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"disabled": "", "missing": filepath.Join(dir, "none.json"), "expired": expired, "garbage": garbage,
	} {
		if _, ok := loadToken(path); ok {
			t.Errorf("%s: loadToken reported a hit", name)
		}
	}
}

func TestTokenCachePath(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("ZC_NO_TOKEN_CACHE", "")
	a := tokenCachePath("us", "id", "refresh-a")
	if a == "" {
		t.Fatal("expected a path")
	}
	if b := tokenCachePath("us", "id", "refresh-b"); b == a {
		t.Error("different refresh tokens share a cache file")
	}
	if p := tokenCachePath("us", "id", ""); p != "" {
		t.Errorf("no refresh token: got %q, want no cache", p)
	}
	t.Setenv("ZC_NO_TOKEN_CACHE", "0")
	if p := tokenCachePath("us", "id", "refresh-a"); p == "" {
		t.Error("ZC_NO_TOKEN_CACHE=0: caching turned off, want on")
	}
	t.Setenv("ZC_NO_TOKEN_CACHE", "yes")
	if p := tokenCachePath("us", "id", "refresh-a"); p != "" {
		t.Errorf("ZC_NO_TOKEN_CACHE=yes: got %q, want no cache", p)
	}
	t.Setenv("ZC_NO_TOKEN_CACHE", "true")
	if p := tokenCachePath("us", "id", "refresh-a"); p != "" {
		t.Errorf("ZC_NO_TOKEN_CACHE=true: got %q, want no cache", p)
	}
}
