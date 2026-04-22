package zohocreator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a Zoho Creator REST API client. It is safe for concurrent use by
// multiple goroutines. Each API section is exposed as a field, e.g.
// client.Records.Get(ctx, ...) and client.Meta.Applications(ctx, ...).
type Client struct {
	config      Config
	baseURL     string // scheme+host prefix, e.g. https://www.zohoapis.com
	accountsURL string
	httpClient  *http.Client
	tokens      TokenSource

	Records    *RecordService
	Files      *FileService
	Meta       *MetaService
	Publish    *PublishService
	Bulk       *BulkService
	CustomAPIs *CustomAPIService
}

// NewClient constructs a Client from the given Config.
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	cfg.setDefaults()

	c := &Client{
		config:      cfg,
		baseURL:     cfg.apiBaseURL(),
		accountsURL: cfg.accountsBaseURL(),
		httpClient:  cfg.HTTPClient,
	}
	c.tokens = newTokenSource(cfg, c.httpClient)
	c.Records = &RecordService{client: c}
	c.Files = &FileService{client: c}
	c.Meta = &MetaService{client: c}
	c.Publish = &PublishService{client: c}
	c.Bulk = &BulkService{client: c}
	c.CustomAPIs = &CustomAPIService{client: c}
	return c, nil
}

// newTokenSource picks between a refresh-token and a static seeded token.
func newTokenSource(cfg Config, hc *http.Client) TokenSource {
	if cfg.RefreshToken != "" {
		rts := &refreshTokenSource{
			clientID:     cfg.ClientID,
			clientSecret: cfg.ClientSecret,
			refreshToken: cfg.RefreshToken,
			accountsURL:  cfg.accountsBaseURL(),
			httpClient:   hc,
			userAgent:    cfg.UserAgent,
			earlyRefresh: cfg.TokenEarlyRefresh,
		}
		if cfg.AccessToken != "" {
			rts.current = cfg.AccessToken
			rts.expiry = cfg.AccessTokenExpiry
		}
		return rts
	}
	return staticTokenSource{token: cfg.AccessToken}
}

// BaseURL returns the resolved API base URL (no trailing slash).
func (c *Client) BaseURL() string { return c.baseURL }

// TokenSource exposes the underlying token source for advanced use (e.g.
// invalidating on externally-detected auth failures).
func (c *Client) TokenSource() TokenSource { return c.tokens }

// requestOptions customises an individual request.
type requestOptions struct {
	// method defaults to http.MethodGet.
	method string
	// path is the API path. For v2.1 endpoints, pass the portion after
	// /creator/v2.1/ prefixed with "/v2.1/..."; for custom APIs, pass
	// "/custom/...". absolutePath=true treats path verbatim.
	path string
	// absolutePath, when true, treats path as already containing the full
	// URL or the full `/creator/...` path. Used for bulk result download.
	absolutePath bool
	// query parameters to encode.
	query url.Values
	// headers to add on top of the defaults.
	headers http.Header
	// body is marshaled as JSON when non-nil (unless rawBody is set).
	body any
	// rawBody is an already-encoded body (used for multipart uploads).
	rawBody io.Reader
	// contentType overrides the default application/json when rawBody is
	// set.
	contentType string
	// accept overrides the default application/json.
	accept string
	// noAuth disables the Authorization header (used for public endpoints).
	noAuth bool
	// skipEnvHeader disables the environment header (used for some global
	// endpoints).
	skipEnvHeader bool
	// stream, when true, returns the response body without decoding so
	// callers can read large downloads.
	stream bool
}

// doResult is what do() returns: a decoded JSON envelope or a streaming body.
type doResult struct {
	// envelope is the raw JSON body (closed response). Empty when stream==true.
	envelope []byte
	// status is the HTTP status for success paths (2xx).
	status int
	// headers is the response header (always populated).
	headers http.Header
	// stream is set when requestOptions.stream==true; caller must Close().
	stream io.ReadCloser
}

// do executes an HTTP request with auth injection, retries on transient
// failures, and error classification.
func (c *Client) do(ctx context.Context, opts requestOptions) (*doResult, error) {
	if opts.method == "" {
		opts.method = http.MethodGet
	}
	fullURL, err := c.resolveURL(opts.path, opts.absolutePath, opts.query)
	if err != nil {
		return nil, err
	}

	var jsonBody []byte
	if opts.rawBody == nil && opts.body != nil {
		b, err := json.Marshal(opts.body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		jsonBody = b
	}

	maxRetries := *c.config.MaxRetries
	var lastErr error
	authRefreshAttempted := false
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var reader io.Reader
		switch {
		case opts.rawBody != nil:
			reader = opts.rawBody
		case jsonBody != nil:
			reader = bytes.NewReader(jsonBody)
		}
		req, err := http.NewRequestWithContext(ctx, opts.method, fullURL, reader)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		if err := c.setHeaders(ctx, req, opts); err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			if attempt < maxRetries && isRetryableNetErr(err) {
				if werr := waitOrCancel(ctx, backoff(attempt)); werr != nil {
					return nil, werr
				}
				continue
			}
			return nil, lastErr
		}

		if opts.stream && resp.StatusCode < 400 {
			return &doResult{stream: resp.Body, status: resp.StatusCode, headers: resp.Header}, nil
		}

		respBody, rerr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if rerr != nil {
			return nil, fmt.Errorf("read response body: %w", rerr)
		}

		if resp.StatusCode == http.StatusUnauthorized && !authRefreshAttempted && opts.rawBody == nil {
			// Stale token: invalidate and retry once outside the
			// normal retry budget. We can only retry safely when
			// rawBody is nil because multipart readers are not
			// rewindable here.
			authRefreshAttempted = true
			c.tokens.Invalidate()
			attempt--
			continue
		}
		if resp.StatusCode >= 500 && attempt < maxRetries {
			if werr := waitOrCancel(ctx, backoff(attempt)); werr != nil {
				return nil, werr
			}
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			d := retryAfter(resp.Header, backoff(attempt))
			if werr := waitOrCancel(ctx, d); werr != nil {
				return nil, werr
			}
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, classifyError(resp.StatusCode, resp.Header, respBody)
		}

		return &doResult{envelope: respBody, status: resp.StatusCode, headers: resp.Header}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("exhausted retries")
	}
	return nil, lastErr
}

// resolveURL builds the full request URL from a path + query.
func (c *Client) resolveURL(path string, absolute bool, query url.Values) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return appendQuery(path, query), nil
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !absolute {
		path = "/creator" + path
	}
	return appendQuery(c.baseURL+path, query), nil
}

// appendQuery safely merges extra query params into a URL string.
func appendQuery(raw string, extra url.Values) string {
	if len(extra) == 0 {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		// Fallback: naive append; url.Parse shouldn't fail on URLs we
		// just built.
		sep := "?"
		if strings.Contains(raw, "?") {
			sep = "&"
		}
		return raw + sep + extra.Encode()
	}
	existing := u.Query()
	for k, vals := range extra {
		for _, v := range vals {
			existing.Add(k, v)
		}
	}
	u.RawQuery = existing.Encode()
	return u.String()
}

// setHeaders applies auth + default headers + caller-supplied overrides.
func (c *Client) setHeaders(ctx context.Context, req *http.Request, opts requestOptions) error {
	ua := c.config.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("User-Agent", ua)

	accept := opts.accept
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)

	if opts.rawBody != nil {
		if opts.contentType != "" {
			req.Header.Set("Content-Type", opts.contentType)
		}
	} else if opts.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if !opts.skipEnvHeader && c.config.Environment != "" {
		req.Header.Set("environment", string(c.config.Environment))
	}

	if !opts.noAuth {
		tok, err := c.tokens.Token(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Zoho-oauthtoken "+tok)
	}

	for k, vals := range opts.headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	return nil
}

// classifyError maps an HTTP response to a typed error. The body is parsed for
// Zoho's `{code, message}` envelope when possible.
func classifyError(status int, header http.Header, body []byte) error {
	base := Error{Status: status, Kind: "api"}
	var env codeEnvelope
	if len(body) > 0 {
		_ = json.Unmarshal(body, &env)
	}
	base.Code = env.Code
	base.Message = env.Message
	if base.Message == "" {
		base.Message = env.Description
	}
	if base.Message == "" {
		base.Message = http.StatusText(status)
	}
	switch status {
	case http.StatusBadRequest:
		base.Kind = "validation"
		return &ValidationError{Base: base}
	case http.StatusUnauthorized:
		base.Kind = "auth"
		return &AuthError{Base: base}
	case http.StatusForbidden:
		base.Kind = "forbidden"
		return &ForbiddenError{Base: base}
	case http.StatusNotFound:
		base.Kind = "not_found"
		return &NotFoundError{Base: base}
	case http.StatusConflict:
		base.Kind = "conflict"
		return &ConflictError{Base: base}
	case http.StatusTooManyRequests:
		base.Kind = "rate_limited"
		ra := 0
		if h := header.Get("Retry-After"); h != "" {
			if n, err := strconv.Atoi(strings.TrimSpace(h)); err == nil {
				ra = n
			}
		}
		return &RateLimitError{Base: base, RetryAfter: ra}
	}
	if status >= 500 {
		base.Kind = "server"
	}
	return &APIError{Base: base}
}

// retryAfter parses the Retry-After header, falling back to backoff.
func retryAfter(h http.Header, fallback time.Duration) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}

// waitOrCancel sleeps for d or returns early if ctx is done.
func waitOrCancel(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// backoff returns an exponential backoff duration capped at 30s.
func backoff(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt)) * time.Second
	return min(d, 30*time.Second)
}

// isRetryableNetErr reports whether an HTTP-client error looks transient.
// Conservative: we retry anything non-nil except context cancellation, which
// is already short-circuited by the retry loop.
func isRetryableNetErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return !strings.Contains(s, "context canceled") && !strings.Contains(s, "context deadline exceeded")
}
