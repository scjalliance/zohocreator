package zohocreator

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Environment is the Zoho Creator deployment target (the `environment` HTTP
// header). Production is the default when none is specified.
type Environment string

const (
	EnvProduction  Environment = "production"
	EnvStage       Environment = "stage"
	EnvDevelopment Environment = "development"
)

// Valid reports whether e is a recognised environment value.
func (e Environment) Valid() bool {
	switch e {
	case EnvProduction, EnvStage, EnvDevelopment, "":
		return true
	}
	return false
}

// Config is the input to NewClient. Most fields have sensible defaults; the
// only always-required value is an OAuth refresh token plus the corresponding
// client-id/secret pair.
type Config struct {
	// DataCenter selects both the API and accounts hosts. Required unless
	// BaseURL and AccountsURL are both set.
	DataCenter DataCenter

	// ClientID and ClientSecret are the OAuth client credentials issued in
	// the Zoho API console. Required unless a non-empty AccessToken is
	// provided and refresh is disabled.
	ClientID     string
	ClientSecret string

	// RefreshToken is the long-lived OAuth refresh token obtained once via
	// the authorization-code flow. Required unless AccessToken is set and
	// refresh is never desired.
	RefreshToken string

	// AccessToken seeds the in-memory token cache when non-empty. When the
	// token expires or the server returns 401 the client will attempt to
	// refresh using RefreshToken. Optional.
	AccessToken string

	// AccessTokenExpiry is the expiry time for the seeded AccessToken.
	// Defaults to the zero time, which treats the token as already-expired
	// and forces an immediate refresh on first use.
	AccessTokenExpiry time.Time

	// Environment sets the `environment` HTTP header on every request.
	// Defaults to EnvProduction.
	Environment Environment

	// BaseURL overrides the derived API base (e.g. for local testing with
	// httptest). When set it is used verbatim and must include scheme + host
	// but no trailing slash. The full path prefix "/creator/v2.1" is
	// appended only for v2.1 endpoints; custom APIs use "/creator/custom".
	BaseURL string

	// AccountsURL overrides the derived accounts base used for token
	// refresh. Must include scheme + host, no trailing slash. Optional.
	AccountsURL string

	// HTTPClient injects a custom http.Client. When nil, a client is built
	// using DefaultTimeout. The client is shared across API and token
	// requests, so any custom Transport affects both.
	HTTPClient *http.Client

	// MaxRetries is the retry count for transient failures (5xx, network).
	// Defaults to 3 when nil. Set to a pointer to 0 to disable retries.
	MaxRetries *int

	// DefaultTimeout is the timeout applied to the default HTTP client when
	// HTTPClient is not provided. Defaults to 60 seconds (bulk operations
	// can be slow).
	DefaultTimeout time.Duration

	// UserAgent is the User-Agent header on all requests. Defaults to
	// "zohocreator-go/<version>".
	UserAgent string

	// TokenEarlyRefresh is how long before an access token's expiry the
	// client should proactively refresh. Defaults to 60 seconds.
	TokenEarlyRefresh time.Duration
}

const defaultUserAgent = "zohocreator-go/0.1.0"

func (c *Config) validate() error {
	if c.BaseURL == "" && !c.DataCenter.Valid() {
		return fmt.Errorf("either DataCenter or BaseURL is required")
	}
	if c.BaseURL != "" && !strings.HasPrefix(c.BaseURL, "https://") && !strings.HasPrefix(c.BaseURL, "http://") {
		return fmt.Errorf("BaseURL must include scheme (https:// or http://)")
	}
	if c.AccountsURL != "" && !strings.HasPrefix(c.AccountsURL, "https://") && !strings.HasPrefix(c.AccountsURL, "http://") {
		return fmt.Errorf("AccountsURL must include scheme (https:// or http://)")
	}
	if !c.Environment.Valid() {
		return fmt.Errorf("invalid Environment %q", c.Environment)
	}
	if c.MaxRetries != nil && *c.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries cannot be negative")
	}
	if c.DefaultTimeout < 0 {
		return fmt.Errorf("DefaultTimeout cannot be negative")
	}
	if c.TokenEarlyRefresh < 0 {
		return fmt.Errorf("TokenEarlyRefresh cannot be negative")
	}
	if c.RefreshToken == "" && c.AccessToken == "" {
		return fmt.Errorf("RefreshToken or AccessToken is required")
	}
	if c.RefreshToken != "" && (c.ClientID == "" || c.ClientSecret == "") {
		return fmt.Errorf("ClientID and ClientSecret are required when RefreshToken is set")
	}
	return nil
}

func (c *Config) setDefaults() {
	if c.DefaultTimeout == 0 {
		c.DefaultTimeout = 60 * time.Second
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: c.DefaultTimeout}
	}
	if c.MaxRetries == nil {
		n := 3
		c.MaxRetries = &n
	}
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent
	}
	if c.Environment == "" {
		c.Environment = EnvProduction
	}
	if c.TokenEarlyRefresh == 0 {
		c.TokenEarlyRefresh = 60 * time.Second
	}
}

// apiBaseURL returns the scheme+host+/creator prefix without a trailing slash.
// Callers append "/v2.1/..." or "/custom/..." as needed.
func (c *Config) apiBaseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://" + c.DataCenter.APIHost()
}

// accountsBaseURL returns scheme+host of the accounts server without a
// trailing slash. The token path is appended by the caller.
func (c *Config) accountsBaseURL() string {
	if c.AccountsURL != "" {
		return strings.TrimRight(c.AccountsURL, "/")
	}
	return "https://" + c.DataCenter.AccountsHost()
}
