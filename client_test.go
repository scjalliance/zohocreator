package zohocreator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient spins up an httptest.Server that invokes fn for each request
// and returns a ready-to-use Client pointed at it.
func newTestClient(t *testing.T, fn http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(fn)
	t.Cleanup(srv.Close)
	zero := 0
	c, err := NewClient(Config{
		BaseURL:     srv.URL,
		AccountsURL: srv.URL,
		AccessToken: "test-token",
		Environment: EnvProduction,
		MaxRetries:  &zero,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

func TestClientHeadersAndAuth(t *testing.T) {
	var gotAuth, gotEnv, gotAgent string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotEnv = r.Header.Get("environment")
		gotAgent = r.Header.Get("User-Agent")
		fmt.Fprint(w, `{"code":3000,"applications":[]}`)
	})
	if _, err := c.Meta.Applications(context.Background()); err != nil {
		t.Fatalf("Applications: %v", err)
	}
	if gotAuth != "Zoho-oauthtoken test-token" {
		t.Errorf("Authorization=%q", gotAuth)
	}
	if gotEnv != "production" {
		t.Errorf("environment=%q", gotEnv)
	}
	if !strings.HasPrefix(gotAgent, "zohocreator-go/") {
		t.Errorf("User-Agent=%q", gotAgent)
	}
}

func TestClientRefreshOn401(t *testing.T) {
	var tokenCalls, apiCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/v2/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenCalls, 1)
		fmt.Fprint(w, `{"access_token":"new-token","expires_in":3600,"token_type":"Bearer"}`)
	})
	mux.HandleFunc("/creator/v2.1/meta/applications", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&apiCalls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"code":1030,"message":"Authorization Failure"}`)
			return
		}
		fmt.Fprint(w, `{"code":3000,"applications":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	zero := 0
	c, err := NewClient(Config{
		BaseURL:      srv.URL,
		AccountsURL:  srv.URL,
		ClientID:     "id",
		ClientSecret: "sec",
		RefreshToken: "refresh",
		AccessToken:  "stale",
		Environment:  EnvProduction,
		MaxRetries:   &zero,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.Meta.Applications(context.Background()); err != nil {
		t.Fatalf("Applications: %v", err)
	}
	if atomic.LoadInt32(&tokenCalls) < 1 {
		t.Error("expected token refresh")
	}
	if atomic.LoadInt32(&apiCalls) < 2 {
		t.Error("expected retry after 401")
	}
}

func TestClientRetryOn500(t *testing.T) {
	var n int32
	mux := http.NewServeMux()
	mux.HandleFunc("/creator/v2.1/meta/applications", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"code":9999,"message":"boom"}`)
			return
		}
		fmt.Fprint(w, `{"code":3000,"applications":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	max := 2
	c, err := NewClient(Config{
		BaseURL:     srv.URL,
		AccountsURL: srv.URL,
		AccessToken: "t",
		Environment: EnvProduction,
		MaxRetries:  &max,
		HTTPClient:  &http.Client{Timeout: 5 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Disable backoff effectively by using a cancellable context that still
	// permits the 1s first sleep.
	if _, err := c.Meta.Applications(context.Background()); err != nil {
		t.Fatalf("Applications: %v", err)
	}
	if atomic.LoadInt32(&n) < 2 {
		t.Errorf("expected >=2 calls, got %d", n)
	}
}

func TestClientMapsErrorsByStatus(t *testing.T) {
	cases := []struct {
		status int
		body   string
		is     error
	}{
		{http.StatusNotFound, `{"code":2892,"message":"No app"}`, ErrNotFound},
		{http.StatusForbidden, `{"code":2933,"message":"Denied"}`, ErrForbidden},
		{http.StatusBadRequest, `{"code":3020,"message":"Bad"}`, ErrBadRequest},
	}
	for _, c := range cases {
		cl, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			fmt.Fprint(w, c.body)
		})
		_, err := cl.Meta.Applications(context.Background())
		if !errors.Is(err, c.is) {
			t.Errorf("status %d: want %v, got %v", c.status, c.is, err)
		}
	}
}

func TestClientRateLimitRetry(t *testing.T) {
	var n int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"code":2955}`)
			return
		}
		fmt.Fprint(w, `{"code":3000,"applications":[]}`)
	})
	// Override retries on this instance.
	two := 2
	c.config.MaxRetries = &two
	if _, err := c.Meta.Applications(context.Background()); err != nil {
		t.Fatalf("Applications: %v", err)
	}
	if atomic.LoadInt32(&n) < 2 {
		t.Errorf("rate-limit retry did not occur, calls=%d", n)
	}
}

func TestResolveURL(t *testing.T) {
	c := &Client{baseURL: "https://host"}
	u, err := c.resolveURL("/v2.1/meta/applications", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://host/creator/v2.1/meta/applications" {
		t.Errorf("got %q", u)
	}
	u, _ = c.resolveURL("https://foo/x", false, nil)
	if u != "https://foo/x" {
		t.Errorf("absolute URL mangled: %q", u)
	}
	u, _ = c.resolveURL("/custom/admin/myapi", false, nil)
	if u != "https://host/creator/custom/admin/myapi" {
		t.Errorf("custom path: %q", u)
	}
}

func TestAppendQuery(t *testing.T) {
	raw := "https://host/x?a=1"
	got := appendQuery(raw, nil)
	if got != raw {
		t.Errorf("nil extras should not change URL")
	}
	got = appendQuery(raw, map[string][]string{"b": {"2"}})
	if !strings.Contains(got, "a=1") || !strings.Contains(got, "b=2") {
		t.Errorf("merge lost params: %q", got)
	}
}

func TestAuthURLExchange(t *testing.T) {
	got := AuthURL(DCUS, "cid", "https://cb", []string{"ZohoCreator.report.READ"}, "state1")
	if !strings.Contains(got, "https://accounts.zoho.com/oauth/v2/auth") {
		t.Errorf("accounts host missing: %q", got)
	}
	if !strings.Contains(got, "client_id=cid") || !strings.Contains(got, "state=state1") {
		t.Errorf("missing params: %q", got)
	}
}

func TestExchangeCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/v2/token" {
			w.WriteHeader(404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "grant_type=authorization_code") {
			t.Errorf("missing grant_type: %q", body)
		}
		fmt.Fprint(w, `{"access_token":"a","refresh_token":"r","expires_in":3600}`)
	}))
	defer srv.Close()
	// Use a custom httpClient that rewrites accounts host to srv.URL.
	hc := &http.Client{Transport: hostRewriter{target: srv.URL}}
	got, err := ExchangeCode(context.Background(), DCUS, "id", "sec", "code", "https://cb", hc)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "a" || got.RefreshToken != "r" {
		t.Errorf("got %+v", got)
	}
}

// hostRewriter redirects all requests to target URL's host, preserving path.
type hostRewriter struct{ target string }

func (h hostRewriter) RoundTrip(r *http.Request) (*http.Response, error) {
	u, err := urlParse(h.target)
	if err != nil {
		return nil, err
	}
	r.URL.Scheme = u.Scheme
	r.URL.Host = u.Host
	r.Host = u.Host
	return http.DefaultTransport.RoundTrip(r)
}
