package zohocreator

import "testing"

func TestDataCenterHosts(t *testing.T) {
	cases := []struct {
		dc        DataCenter
		api, acct string
	}{
		{DCUS, "www.zohoapis.com", "accounts.zoho.com"},
		{DCEU, "www.zohoapis.eu", "accounts.zoho.eu"},
		{DCIN, "www.zohoapis.in", "accounts.zoho.in"},
		{DCAU, "www.zohoapis.com.au", "accounts.zoho.com.au"},
		{DCJP, "www.zohoapis.jp", "accounts.zoho.jp"},
		{DCCA, "www.zohoapis.ca", "accounts.zohocloud.ca"},
		{DCCN, "www.zohoapis.com.cn", "accounts.zoho.com.cn"},
		{DCSA, "www.zohoapis.sa", "accounts.zoho.sa"},
		{DCAE, "www.zohoapis.ae", "accounts.zoho.ae"},
	}
	for _, c := range cases {
		if got := c.dc.APIHost(); got != c.api {
			t.Errorf("%s APIHost=%q want %q", c.dc, got, c.api)
		}
		if got := c.dc.AccountsHost(); got != c.acct {
			t.Errorf("%s AccountsHost=%q want %q", c.dc, got, c.acct)
		}
		if !c.dc.Valid() {
			t.Errorf("%s should be valid", c.dc)
		}
	}
}

func TestParseDataCenter(t *testing.T) {
	cases := map[string]DataCenter{
		"us":              DCUS,
		"US":              DCUS,
		"com":             DCUS,
		"zohoapis.com":    DCUS,
		"eu":              DCEU,
		"in":              DCIN,
		"au":              DCAU,
		"com.au":          DCAU,
		"zohoapis.com.au": DCAU,
		"jp":              DCJP,
		"ca":              DCCA,
		"cn":              DCCN,
		"sa":              DCSA,
		"ae":              DCAE,
	}
	for in, want := range cases {
		got, err := ParseDataCenter(in)
		if err != nil {
			t.Errorf("ParseDataCenter(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseDataCenter(%q)=%q want %q", in, got, want)
		}
	}
	if _, err := ParseDataCenter("garbage"); err == nil {
		t.Error("ParseDataCenter(garbage) should error")
	}
}

func TestDataCenterInvalid(t *testing.T) {
	dc := DataCenter("xx")
	if dc.Valid() {
		t.Error("xx should not be valid")
	}
	if dc.APIHost() != "" || dc.AccountsHost() != "" {
		t.Error("unknown DC should return empty hosts")
	}
}
