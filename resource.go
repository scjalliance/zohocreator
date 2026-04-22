package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// Page is one page of list results. Pagination is version-aware:
//
//   - v2.1 uses the `record_cursor` response header; Cursor carries it and
//     HasNext becomes true while a non-empty cursor is present.
//   - v2 uses offset pagination via `from`/`limit`; Cursor is always empty
//     and HasNext is true while the page is full (i.e. it returned `limit`
//     items, implying another page may exist). The client increments `from`
//     by `limit` for each successive fetch.
type Page[T any] struct {
	Items  []T
	Code   int
	Cursor string // next record_cursor value (v2.1 only); empty on v2

	client  *Client
	next    func(ctx context.Context) (*Page[T], error)
	hasNext bool
}

// HasNext reports whether another page exists.
func (p *Page[T]) HasNext() bool { return p.hasNext && p.next != nil }

// NextPage fetches the next page or returns nil when exhausted.
func (p *Page[T]) NextPage(ctx context.Context) (*Page[T], error) {
	if !p.HasNext() {
		return nil, nil
	}
	return p.next(ctx)
}

// Iter returns an iterator over all items starting from this page, fetching
// subsequent pages until exhausted.
func (p *Page[T]) Iter(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		page := p
		for page != nil {
			for _, it := range page.Items {
				if !yield(it, nil) {
					return
				}
			}
			if !page.HasNext() {
				return
			}
			np, err := page.NextPage(ctx)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			page = np
		}
	}
}

// Collect drains every remaining page into a slice. Prefer Iter for large sets.
func (p *Page[T]) Collect(ctx context.Context) ([]T, error) {
	var all []T
	for item, err := range p.Iter(ctx) {
		if err != nil {
			return all, err
		}
		all = append(all, item)
	}
	return all, nil
}

// fetchPage runs a version-appropriate paginated GET and decodes the response
// body into a Page[T]. The `dataField` is the top-level JSON field holding
// the array (usually "data" for records; "forms"/"reports"/etc. for meta).
//
// On v2.1 pagination uses the `record_cursor` header; on v2 it uses
// `from`/`limit` offsets incremented page-by-page.
func fetchPage[T any](ctx context.Context, c *Client, path string, baseQuery *Query, dataField string) (*Page[T], error) {
	q := baseQuery.clone()
	return runFetch[T](ctx, c, path, q, dataField)
}

// runFetch issues a single page request honouring the active API version,
// decodes it, and attaches a `next` closure that re-invokes runFetch with
// the pagination state advanced appropriately.
func runFetch[T any](ctx context.Context, c *Client, path string, q *Query, dataField string) (*Page[T], error) {
	version := c.APIVersion()
	params := q.buildParamsForVersion(version)
	headers := http.Header{}
	if version == APIVersionV21 && q.RecordCursor != "" {
		headers.Set("record_cursor", q.RecordCursor)
	}
	if version == APIVersionV2 {
		// Force a bounded limit so HasNext can decide when we're done.
		if q.Limit <= 0 {
			q.Limit = 200
			params.Set("limit", "200")
		}
		if q.From < 0 {
			q.From = 0
		}
		if _, ok := params["from"]; !ok && q.From > 0 {
			params.Set("from", intStr(q.From))
		}
	}

	res, err := c.do(ctx, requestOptions{
		method:  http.MethodGet,
		path:    path,
		query:   params,
		headers: headers,
	})
	if err != nil {
		return nil, err
	}
	page, err := decodePage[T](res, dataField, version, q.Limit)
	if err != nil {
		return nil, err
	}
	page.client = c

	// Build the advance closure appropriate to the version.
	switch version {
	case APIVersionV2:
		page.next = func(ctx context.Context) (*Page[T], error) {
			adv := q.clone()
			adv.From = q.From + q.Limit
			return runFetch[T](ctx, c, path, adv, dataField)
		}
	default:
		page.next = func(ctx context.Context) (*Page[T], error) {
			adv := q.clone()
			adv.RecordCursor = page.Cursor
			return runFetch[T](ctx, c, path, adv, dataField)
		}
	}
	return page, nil
}

// decodePage extracts a code + array-of-T payload under dataField from a
// response, and reads the next cursor from the response header (v2.1). On v2
// it flags HasNext based on whether the page filled to the requested limit.
func decodePage[T any](res *doResult, dataField string, version APIVersion, limit int) (*Page[T], error) {
	if res == nil {
		return &Page[T]{}, nil
	}
	var generic map[string]json.RawMessage
	if len(res.envelope) > 0 {
		if err := json.Unmarshal(res.envelope, &generic); err != nil {
			return nil, fmt.Errorf("decode envelope: %w", err)
		}
	}
	var code int
	if raw, ok := generic["code"]; ok {
		_ = json.Unmarshal(raw, &code)
	}
	var items []T
	if raw, ok := generic[dataField]; ok && len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("decode %s: %w", dataField, err)
		}
	}
	page := &Page[T]{Items: items, Code: code}
	if version == APIVersionV21 && res.headers != nil {
		if v := res.headers.Get("record_cursor"); v != "" {
			page.Cursor = v
			page.hasNext = true
		}
	}
	if version == APIVersionV2 && limit > 0 && len(items) >= limit {
		page.hasNext = true
	}
	return page, nil
}

// intStr renders an int as a string without pulling in strconv at the call
// site. Tiny helper kept local so resource.go stays self-contained.
func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
