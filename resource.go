package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// Page is one page of list results plus the cursor that fetches the next.
// Page is generic so the same plumbing works for records, forms, reports, etc.
type Page[T any] struct {
	Items  []T
	Code   int
	Cursor string // next record_cursor value, empty when no more pages

	client    *Client
	next      func(ctx context.Context, cursor string) (*Page[T], error)
	hasCursor bool
}

// HasNext reports whether another page exists.
func (p *Page[T]) HasNext() bool { return p.hasCursor && p.Cursor != "" && p.next != nil }

// NextPage fetches the next page or returns nil when exhausted.
func (p *Page[T]) NextPage(ctx context.Context) (*Page[T], error) {
	if !p.HasNext() {
		return nil, nil
	}
	return p.next(ctx, p.Cursor)
}

// Iter returns an iterator over all items starting from this page, fetching
// subsequent pages via the cursor until exhausted.
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

// fetchPage runs a cursor-aware GET and decodes Data into []T.
//
// pathAbsolute=false means `path` is relative to /creator (e.g.
// "/v2.1/data/..."). baseQuery is the non-cursor query params.
//
// The data field name defaults to "data" — some endpoints (forms, reports)
// use different field names; those services build their own page functions.
func fetchPage[T any](ctx context.Context, c *Client, path string, baseQuery *Query, dataField string) (*Page[T], error) {
	q := baseQuery.clone()
	params := q.buildParams()
	headers := http.Header{}
	if q.RecordCursor != "" {
		headers.Set("record_cursor", q.RecordCursor)
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
	page, err := decodePage[T](res, dataField)
	if err != nil {
		return nil, err
	}
	page.client = c
	page.next = func(ctx context.Context, cursor string) (*Page[T], error) {
		nq := baseQuery.clone()
		nq.RecordCursor = cursor
		return fetchPage[T](ctx, c, path, nq, dataField)
	}
	return page, nil
}

// decodePage extracts a code + array-of-T payload under dataField from a
// response, and reads the next cursor from the response header.
func decodePage[T any](res *doResult, dataField string) (*Page[T], error) {
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
	cursor := ""
	hasCursor := false
	if res.headers != nil {
		if v := res.headers.Get("record_cursor"); v != "" {
			cursor = v
			hasCursor = true
		}
	}
	return &Page[T]{Items: items, Code: code, Cursor: cursor, hasCursor: hasCursor}, nil
}
