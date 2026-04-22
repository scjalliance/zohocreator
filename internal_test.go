package zohocreator

import "net/url"

// urlParse is test-only: it wraps net/url.Parse so client_test.go can use it
// without importing url (keeping the test focused).
func urlParse(s string) (*url.URL, error) { return url.Parse(s) }
