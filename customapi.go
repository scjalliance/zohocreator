package zohocreator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// CustomAPIService invokes user-defined Custom APIs configured inside a Zoho
// Creator application (Deluge-backed REST endpoints).
//
// Custom APIs can be secured either with OAuth2 (the authenticated user must
// be in the API's user scope) or with a PublicKey (appended as ?publickey=
// on the request). Both are supported here.
type CustomAPIService struct{ client *Client }

// CustomAPIOptions customises an Invoke call.
type CustomAPIOptions struct {
	// Method defaults to GET. Any HTTP verb the custom API is registered
	// for can be used.
	Method string
	// Query parameters to add to the URL.
	Query url.Values
	// Body is the request body. When non-nil it is JSON-encoded unless
	// RawBody is set. Mutually exclusive with RawBody.
	Body any
	// RawBody is a pre-encoded body stream. When set, ContentType should
	// also be set.
	RawBody io.Reader
	// ContentType applies when RawBody is set.
	ContentType string
	// PublicKey switches the auth mode: when non-empty, the client omits
	// the OAuth bearer token and appends `publickey=<value>` as a query
	// parameter.
	PublicKey string
	// Headers allows adding arbitrary request headers.
	Headers http.Header
}

// Invoke calls a custom API and returns the raw response body and HTTP
// headers. Decoding is the caller's responsibility because Deluge custom APIs
// may return arbitrary JSON, plain text, binary, etc.
//
// The URL shape is: https://<api-host>/creator/custom/<appadmin>/<customAPIname>
// where appadmin is the app admin (usually the account owner name) and
// customAPIname is the API's link name.
func (s *CustomAPIService) Invoke(ctx context.Context, appadmin, apiName string, opts *CustomAPIOptions) ([]byte, http.Header, error) {
	if appadmin == "" || apiName == "" {
		return nil, nil, fmt.Errorf("appadmin and apiName are required")
	}
	path := fmt.Sprintf("/custom/%s/%s", url.PathEscape(appadmin), url.PathEscape(apiName))
	reqOpts := requestOptions{
		method: http.MethodGet,
		path:   path,
	}
	if opts != nil {
		if opts.Method != "" {
			reqOpts.method = opts.Method
		}
		if len(opts.Query) > 0 {
			reqOpts.query = cloneValues(opts.Query)
		}
		if opts.RawBody != nil {
			reqOpts.rawBody = opts.RawBody
			reqOpts.contentType = opts.ContentType
		} else if opts.Body != nil {
			reqOpts.body = opts.Body
		}
		if opts.PublicKey != "" {
			if reqOpts.query == nil {
				reqOpts.query = url.Values{}
			}
			reqOpts.query.Set("publickey", opts.PublicKey)
			reqOpts.noAuth = true
			reqOpts.skipEnvHeader = true
		}
		if len(opts.Headers) > 0 {
			reqOpts.headers = opts.Headers.Clone()
		}
	}
	res, err := s.client.do(ctx, reqOpts)
	if err != nil {
		return nil, nil, err
	}
	return res.envelope, res.headers, nil
}

// InvokeStream is the streaming variant of Invoke. The caller must Close the
// returned reader.
func (s *CustomAPIService) InvokeStream(ctx context.Context, appadmin, apiName string, opts *CustomAPIOptions) (io.ReadCloser, http.Header, error) {
	if appadmin == "" || apiName == "" {
		return nil, nil, fmt.Errorf("appadmin and apiName are required")
	}
	path := fmt.Sprintf("/custom/%s/%s", url.PathEscape(appadmin), url.PathEscape(apiName))
	reqOpts := requestOptions{
		method: http.MethodGet,
		path:   path,
		stream: true,
	}
	if opts != nil {
		if opts.Method != "" {
			reqOpts.method = opts.Method
		}
		if len(opts.Query) > 0 {
			reqOpts.query = cloneValues(opts.Query)
		}
		if opts.RawBody != nil {
			reqOpts.rawBody = opts.RawBody
			reqOpts.contentType = opts.ContentType
		} else if opts.Body != nil {
			reqOpts.body = opts.Body
		}
		if opts.PublicKey != "" {
			if reqOpts.query == nil {
				reqOpts.query = url.Values{}
			}
			reqOpts.query.Set("publickey", opts.PublicKey)
			reqOpts.noAuth = true
			reqOpts.skipEnvHeader = true
		}
		if len(opts.Headers) > 0 {
			reqOpts.headers = opts.Headers.Clone()
		}
	}
	res, err := s.client.do(ctx, reqOpts)
	if err != nil {
		return nil, nil, err
	}
	return res.stream, res.headers, nil
}

// cloneValues deep-copies url.Values (url.Values itself is a map so the
// shallow copy would leak mutations).
func cloneValues(v url.Values) url.Values {
	if v == nil {
		return nil
	}
	out := make(url.Values, len(v))
	for k, vals := range v {
		out[k] = append([]string(nil), vals...)
	}
	return out
}
