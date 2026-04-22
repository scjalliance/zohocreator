package zohocreator

import (
	"fmt"
	"strings"
)

// DataCenter identifies the Zoho region an account lives in. The data center
// determines both the API host used for Creator calls and the accounts host
// used for OAuth token exchange.
type DataCenter string

const (
	DCUS DataCenter = "us" // United States
	DCEU DataCenter = "eu" // European Union
	DCIN DataCenter = "in" // India
	DCAU DataCenter = "au" // Australia
	DCJP DataCenter = "jp" // Japan
	DCCA DataCenter = "ca" // Canada
	DCCN DataCenter = "cn" // China
	DCSA DataCenter = "sa" // Saudi Arabia
	DCAE DataCenter = "ae" // United Arab Emirates
)

// APIHost returns the API hostname for this data center (no scheme, no path).
func (dc DataCenter) APIHost() string {
	switch dc {
	case DCUS:
		return "www.zohoapis.com"
	case DCEU:
		return "www.zohoapis.eu"
	case DCIN:
		return "www.zohoapis.in"
	case DCAU:
		return "www.zohoapis.com.au"
	case DCJP:
		return "www.zohoapis.jp"
	case DCCA:
		return "www.zohoapis.ca"
	case DCCN:
		return "www.zohoapis.com.cn"
	case DCSA:
		return "www.zohoapis.sa"
	case DCAE:
		return "www.zohoapis.ae"
	}
	return ""
}

// AccountsHost returns the OAuth accounts hostname for this data center.
func (dc DataCenter) AccountsHost() string {
	switch dc {
	case DCUS:
		return "accounts.zoho.com"
	case DCEU:
		return "accounts.zoho.eu"
	case DCIN:
		return "accounts.zoho.in"
	case DCAU:
		return "accounts.zoho.com.au"
	case DCJP:
		return "accounts.zoho.jp"
	case DCCA:
		return "accounts.zohocloud.ca"
	case DCCN:
		return "accounts.zoho.com.cn"
	case DCSA:
		return "accounts.zoho.sa"
	case DCAE:
		return "accounts.zoho.ae"
	}
	return ""
}

// Valid reports whether dc is a recognised data-center code.
func (dc DataCenter) Valid() bool {
	return dc.APIHost() != ""
}

// ParseDataCenter normalises a user-provided data-center string (case-insensitive,
// accepts "us"/"com", "eu", "in", "com.au"/"au", "jp", "ca", "com.cn"/"cn",
// "sa", "ae"). Returns an error if nothing matches.
func ParseDataCenter(s string) (DataCenter, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "us", "com", "zohoapis.com":
		return DCUS, nil
	case "eu", "zohoapis.eu":
		return DCEU, nil
	case "in", "zohoapis.in":
		return DCIN, nil
	case "au", "com.au", "zohoapis.com.au":
		return DCAU, nil
	case "jp", "zohoapis.jp":
		return DCJP, nil
	case "ca", "zohoapis.ca":
		return DCCA, nil
	case "cn", "com.cn", "zohoapis.com.cn":
		return DCCN, nil
	case "sa", "zohoapis.sa":
		return DCSA, nil
	case "ae", "zohoapis.ae":
		return DCAE, nil
	}
	return "", fmt.Errorf("zohocreator: unknown data center %q", s)
}
