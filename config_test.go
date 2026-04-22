package zohocreator

import (
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "missing_dc_and_baseurl",
			cfg:     Config{ClientID: "x", ClientSecret: "y", RefreshToken: "z"},
			wantErr: true,
		},
		{
			name: "dc_only_ok",
			cfg: Config{
				DataCenter: DCUS, ClientID: "x", ClientSecret: "y", RefreshToken: "z",
			},
		},
		{
			name: "baseurl_without_scheme",
			cfg: Config{
				BaseURL: "zohoapis.com", ClientID: "x", ClientSecret: "y", RefreshToken: "z",
			},
			wantErr: true,
		},
		{
			name: "bad_environment",
			cfg: Config{
				DataCenter: DCUS, ClientID: "x", ClientSecret: "y", RefreshToken: "z",
				Environment: "preprod",
			},
			wantErr: true,
		},
		{
			name: "missing_client_when_refresh",
			cfg: Config{
				DataCenter: DCUS, RefreshToken: "z",
			},
			wantErr: true,
		},
		{
			name: "access_token_only",
			cfg: Config{
				DataCenter: DCUS, AccessToken: "t",
			},
		},
		{
			name: "no_token_at_all",
			cfg: Config{
				DataCenter: DCUS, ClientID: "x", ClientSecret: "y",
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if tc.wantErr && err == nil {
				t.Error("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfigSetDefaults(t *testing.T) {
	cfg := Config{DataCenter: DCUS, AccessToken: "t"}
	cfg.setDefaults()
	if cfg.DefaultTimeout != 60*time.Second {
		t.Errorf("DefaultTimeout=%v", cfg.DefaultTimeout)
	}
	if cfg.HTTPClient == nil {
		t.Error("HTTPClient nil")
	}
	if cfg.MaxRetries == nil || *cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries=%v", cfg.MaxRetries)
	}
	if cfg.UserAgent == "" {
		t.Error("UserAgent empty")
	}
	if cfg.Environment != EnvProduction {
		t.Errorf("Environment=%v", cfg.Environment)
	}
	if cfg.TokenEarlyRefresh != 60*time.Second {
		t.Errorf("TokenEarlyRefresh=%v", cfg.TokenEarlyRefresh)
	}
}

func TestConfigBaseURLs(t *testing.T) {
	cfg := Config{DataCenter: DCEU}
	if cfg.apiBaseURL() != "https://www.zohoapis.eu" {
		t.Errorf("apiBaseURL=%q", cfg.apiBaseURL())
	}
	if cfg.accountsBaseURL() != "https://accounts.zoho.eu" {
		t.Errorf("accountsBaseURL=%q", cfg.accountsBaseURL())
	}
	cfg2 := Config{BaseURL: "https://localhost:4000/", AccountsURL: "https://accounts.local/"}
	if cfg2.apiBaseURL() != "https://localhost:4000" {
		t.Errorf("apiBaseURL=%q", cfg2.apiBaseURL())
	}
	if cfg2.accountsBaseURL() != "https://accounts.local" {
		t.Errorf("accountsBaseURL=%q", cfg2.accountsBaseURL())
	}
}
