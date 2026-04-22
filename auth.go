package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenSource yields access tokens on demand, refreshing them as needed. It
// is safe for concurrent use.
type TokenSource interface {
	// Token returns a valid access token, refreshing if the cached token is
	// missing or within TokenEarlyRefresh of expiry.
	Token(ctx context.Context) (string, error)
	// Invalidate marks the current token stale so the next call refetches.
	// Use this after receiving a 401 that wasn't caused by a scope/client
	// error.
	Invalidate()
}

// refreshTokenSource is the default TokenSource: it trades a long-lived
// refresh token for access tokens against the accounts server.
type refreshTokenSource struct {
	clientID     string
	clientSecret string
	refreshToken string
	accountsURL  string
	httpClient   *http.Client
	userAgent    string
	earlyRefresh time.Duration

	mu      sync.Mutex
	current string
	expiry  time.Time
}

// staticTokenSource never refreshes; it always returns the same token. Useful
// for short-lived scripts and in tests.
type staticTokenSource struct{ token string }

// Token returns the static token.
func (s staticTokenSource) Token(context.Context) (string, error) { return s.token, nil }

// Invalidate is a no-op for a static source.
func (s staticTokenSource) Invalidate() {}

// Token returns a cached access token if one is still valid, otherwise
// refreshes via the accounts server.
func (r *refreshTokenSource) Token(ctx context.Context) (string, error) {
	r.mu.Lock()
	if r.current != "" && time.Until(r.expiry) > r.earlyRefresh {
		tok := r.current
		r.mu.Unlock()
		return tok, nil
	}
	r.mu.Unlock()
	return r.refresh(ctx)
}

// Invalidate drops the cached token so the next call forces a refresh.
func (r *refreshTokenSource) Invalidate() {
	r.mu.Lock()
	r.current = ""
	r.expiry = time.Time{}
	r.mu.Unlock()
}

// refresh posts to the accounts token endpoint and stores the new token.
func (r *refreshTokenSource) refresh(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("refresh_token", r.refreshToken)
	form.Set("client_id", r.clientID)
	form.Set("client_secret", r.clientSecret)
	form.Set("grant_type", "refresh_token")

	endpoint := r.accountsURL + "/oauth/v2/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", r.userAgent)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("decode token response (http=%d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode >= 400 || tr.AccessToken == "" {
		msg := tr.Error
		if tr.ErrorDescription != "" {
			msg = msg + ": " + tr.ErrorDescription
		}
		if msg == "" {
			msg = "unknown token error"
		}
		return "", &AuthError{Base: Error{Status: resp.StatusCode, Message: msg, Kind: "auth"}}
	}

	r.mu.Lock()
	r.current = tr.AccessToken
	ttl := time.Duration(tr.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	r.expiry = time.Now().Add(ttl)
	tok := r.current
	r.mu.Unlock()
	return tok, nil
}

// tokenResponse is the subset of fields we care about in /oauth/v2/token
// responses. Zoho returns extra fields (api_domain, token_type, scope); they
// are ignored.
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	APIDomain        string `json:"api_domain"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// NewStaticTokenSource returns a TokenSource that always returns token and
// never refreshes. Use when the caller manages token lifetime externally.
func NewStaticTokenSource(token string) TokenSource { return staticTokenSource{token: token} }

// AuthURL returns the URL a user should visit to grant access to the given
// scopes during a one-time authorization-code flow. The client must be
// registered in the Zoho API console with redirectURI as a valid redirect.
//
//	url := zohocreator.AuthURL(zohocreator.DCUS, "1000.abc", "https://example.com/cb",
//	    []string{"ZohoCreator.report.READ", "ZohoCreator.dashboard.READ"}, "state123")
//
// Pass accessType="offline" to obtain a refresh_token alongside the initial
// access_token, "online" for a short-lived token only.
func AuthURL(dc DataCenter, clientID, redirectURI string, scopes []string, state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("scope", strings.Join(scopes, ","))
	q.Set("redirect_uri", redirectURI)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	if state != "" {
		q.Set("state", state)
	}
	return "https://" + dc.AccountsHost() + "/oauth/v2/auth?" + q.Encode()
}

// ExchangeCode trades an authorization code for access + refresh tokens. Use
// once during the initial OAuth setup; persist the returned RefreshToken for
// long-term use via Config.RefreshToken.
type ExchangeResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	APIDomain    string
	TokenType    string
}

// ExchangeCode performs a one-shot OAuth authorization-code exchange. The
// redirectURI must exactly match the one registered in the API console and
// used to obtain code.
func ExchangeCode(ctx context.Context, dc DataCenter, clientID, clientSecret, code, redirectURI string, httpClient *http.Client) (*ExchangeResult, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("code", code)

	endpoint := "https://" + dc.AccountsHost() + "/oauth/v2/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read exchange response: %w", err)
	}
	var tr struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int    `json:"expires_in"`
		APIDomain        string `json:"api_domain"`
		TokenType        string `json:"token_type"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode exchange response (http=%d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode >= 400 || tr.AccessToken == "" {
		msg := tr.Error
		if tr.ErrorDescription != "" {
			msg += ": " + tr.ErrorDescription
		}
		if msg == "" {
			msg = "unknown exchange error"
		}
		return nil, &AuthError{Base: Error{Status: resp.StatusCode, Message: msg, Kind: "auth"}}
	}
	return &ExchangeResult{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresIn:    tr.ExpiresIn,
		APIDomain:    tr.APIDomain,
		TokenType:    tr.TokenType,
	}, nil
}
