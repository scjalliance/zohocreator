package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/url"
)

// PublishService wraps the public "/publish/" endpoints. Published forms and
// reports are reachable without end-user authentication when a privatelink
// query parameter is supplied. The authenticated OAuth variants (no
// privatelink) behave like the Data APIs but act against publish scopes.
type PublishService struct{ client *Client }

// PublishAdd adds records to a published form. The record shape matches
// RecordService.Add. Requires scope ZohoCreator.form.CREATE when authenticated.
//
// When privatelink is non-empty, OAuth is omitted and the privatelink query
// param is sent; this matches Creator's public-endpoint contract.
func (s *PublishService) PublishAdd(ctx context.Context, owner, app, form string, records []Record, opts *AddOptions, privatelink string) ([]AddResult, error) {
	if owner == "" || app == "" || form == "" {
		return nil, fmt.Errorf("owner, app, and form are required")
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("records: at least one required")
	}
	body := map[string]any{"data": records}
	if opts != nil {
		result := map[string]any{}
		if len(opts.ReturnFields) > 0 {
			result["fields"] = opts.ReturnFields
		}
		if opts.Message {
			result["message"] = true
		}
		if opts.Tasks {
			result["tasks"] = true
		}
		if len(result) > 0 {
			body["result"] = result
		}
		if len(opts.SkipWorkflow) > 0 && s.client.APIVersion() != APIVersionV2 {
			body["skip_workflow"] = opts.SkipWorkflow
		}
	}
	path := fmt.Sprintf("/publish/%s/%s/form/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(form))
	reqOpts := requestOptions{
		method: http.MethodPost,
		path:   path,
		body:   body,
	}
	if privatelink != "" {
		reqOpts.query = url.Values{"privatelink": []string{privatelink}}
		reqOpts.noAuth = true
	}
	res, err := s.client.do(ctx, reqOpts)
	if err != nil {
		return nil, err
	}
	var env struct {
		Code   int         `json:"code"`
		Result []AddResult `json:"result"`
	}
	if err := json.Unmarshal(res.envelope, &env); err != nil {
		return nil, fmt.Errorf("decode publish add: %w", err)
	}
	return env.Result, nil
}

// PublishGet fetches one page from a published report. privatelink optional.
func (s *PublishService) PublishGet(ctx context.Context, owner, app, report string, q *Query, privatelink string) (*Page[Record], error) {
	if owner == "" || app == "" || report == "" {
		return nil, fmt.Errorf("owner, app, and report are required")
	}
	path := fmt.Sprintf("/publish/%s/%s/report/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report))
	return fetchPublishPage(ctx, s.client, path, q, privatelink)
}

// PublishAll iterates every record in a published report.
func (s *PublishService) PublishAll(ctx context.Context, owner, app, report string, q *Query, privatelink string) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		page, err := s.PublishGet(ctx, owner, app, report, q, privatelink)
		if err != nil {
			yield(nil, err)
			return
		}
		for rec, err := range page.Iter(ctx) {
			if !yield(rec, err) {
				return
			}
		}
	}
}

// PublishGetByID fetches a single published record's detail view.
func (s *PublishService) PublishGetByID(ctx context.Context, owner, app, report, recordID, privatelink string) (Record, error) {
	if owner == "" || app == "" || report == "" || recordID == "" {
		return nil, fmt.Errorf("owner, app, report, and recordID are required")
	}
	path := fmt.Sprintf("/publish/%s/%s/report/%s/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report), url.PathEscape(recordID))
	opts := requestOptions{method: http.MethodGet, path: path}
	if privatelink != "" {
		opts.query = url.Values{"privatelink": []string{privatelink}}
		opts.noAuth = true
	}
	res, err := s.client.do(ctx, opts)
	if err != nil {
		return nil, err
	}
	var env struct {
		Code int    `json:"code"`
		Data Record `json:"data"`
	}
	if err := json.Unmarshal(res.envelope, &env); err != nil {
		return nil, fmt.Errorf("decode publish record: %w", err)
	}
	return env.Data, nil
}

func fetchPublishPage(ctx context.Context, c *Client, path string, q *Query, privatelink string) (*Page[Record], error) {
	version := c.APIVersion()
	nq := q.clone()
	params := nq.buildParamsForVersion(version)
	if privatelink != "" {
		if params == nil {
			params = url.Values{}
		}
		params.Set("privatelink", privatelink)
	}
	headers := http.Header{}
	if version == APIVersionV21 && nq.RecordCursor != "" {
		headers.Set("record_cursor", nq.RecordCursor)
	}
	if version == APIVersionV2 {
		if nq.Limit <= 0 {
			nq.Limit = 200
			params.Set("limit", "200")
		}
		if nq.From < 0 {
			nq.From = 0
		}
		if _, ok := params["from"]; !ok && nq.From > 0 {
			params.Set("from", intStr(nq.From))
		}
	}
	opts := requestOptions{
		method:  http.MethodGet,
		path:    path,
		query:   params,
		headers: headers,
	}
	if privatelink != "" {
		opts.noAuth = true
	}
	res, err := c.do(ctx, opts)
	if err != nil {
		return nil, err
	}
	page, err := decodePage[Record](res, "data", version, nq.Limit)
	if err != nil {
		return nil, err
	}
	page.client = c
	switch version {
	case APIVersionV2:
		page.next = func(ctx context.Context) (*Page[Record], error) {
			adv := nq.clone()
			adv.From = nq.From + nq.Limit
			return fetchPublishPage(ctx, c, path, adv, privatelink)
		}
	default:
		page.next = func(ctx context.Context) (*Page[Record], error) {
			adv := nq.clone()
			adv.RecordCursor = page.Cursor
			return fetchPublishPage(ctx, c, path, adv, privatelink)
		}
	}
	return page, nil
}
