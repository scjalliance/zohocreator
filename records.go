package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strings"
)

// RecordService wraps the v2.1 Data APIs: add/get/update/delete records.
type RecordService struct{ client *Client }

// AddResult is one row in the Add Records response. Code is Zoho's per-record
// status code (3000 = success). Data carries the saved record payload
// (including the generated ID). Tasks holds any openurl / alert directives
// triggered by workflow rules.
type AddResult struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    Record          `json:"data"`
	Tasks   json.RawMessage `json:"tasks,omitempty"`
}

// AddOptions customises AddRecords. Zero values mean "server default".
type AddOptions struct {
	// ReturnFields lists field link names to echo back in each AddResult.
	// When empty, only the ID is returned.
	ReturnFields []string
	// Message requests that workflow-generated messages be returned.
	Message bool
	// Tasks requests that workflow-generated tasks (openurl, etc.) be
	// returned.
	Tasks bool
	// SkipWorkflow bypasses the listed workflow triggers. Typical values:
	// "form_workflow", "schedules", "all".
	SkipWorkflow []string
}

// Add submits one or more records to a form.
//
// Up to 200 records per request; exceeding the limit returns a 400 with Zoho
// code 3950. Requires scope ZohoCreator.form.CREATE.
func (s *RecordService) Add(ctx context.Context, owner, app, form string, records []Record, opts *AddOptions) ([]AddResult, error) {
	if owner == "" || app == "" || form == "" {
		return nil, fmt.Errorf("owner, app, and form are required")
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("records: at least one record required")
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
	path := fmt.Sprintf("/data/%s/%s/form/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(form))
	res, err := s.client.do(ctx, requestOptions{
		method: http.MethodPost,
		path:   path,
		body:   body,
	})
	if err != nil {
		return nil, err
	}
	var env struct {
		Code   int         `json:"code"`
		Result []AddResult `json:"result"`
	}
	if err := json.Unmarshal(res.envelope, &env); err != nil {
		return nil, fmt.Errorf("decode add response: %w", err)
	}
	return env.Result, nil
}

// Get fetches one page of records from a report. Use All to stream every
// record with transparent cursor pagination.
func (s *RecordService) Get(ctx context.Context, owner, app, report string, q *Query) (*Page[Record], error) {
	if owner == "" || app == "" || report == "" {
		return nil, fmt.Errorf("owner, app, and report are required")
	}
	path := fmt.Sprintf("/data/%s/%s/report/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report))
	return fetchPage[Record](ctx, s.client, path, q, "data")
}

// All returns an iterator yielding every record across all pages.
func (s *RecordService) All(ctx context.Context, owner, app, report string, q *Query) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		page, err := s.Get(ctx, owner, app, report, q)
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

// GetByID fetches a single record in detail view from a report.
func (s *RecordService) GetByID(ctx context.Context, owner, app, report, recordID string) (Record, error) {
	if owner == "" || app == "" || report == "" || recordID == "" {
		return nil, fmt.Errorf("owner, app, report, and recordID are required")
	}
	path := fmt.Sprintf("/data/%s/%s/report/%s/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report), url.PathEscape(recordID))
	res, err := s.client.do(ctx, requestOptions{method: http.MethodGet, path: path})
	if err != nil {
		return nil, err
	}
	var env struct {
		Code int    `json:"code"`
		Data Record `json:"data"`
	}
	if err := json.Unmarshal(res.envelope, &env); err != nil {
		return nil, fmt.Errorf("decode record: %w", err)
	}
	return env.Data, nil
}

// UpdateOptions mirrors AddOptions, with an optional Criteria for bulk-update.
type UpdateOptions struct {
	ReturnFields []string
	Message      bool
	Tasks        bool
	SkipWorkflow []string
	// Criteria limits a bulk update to matching rows. Required for
	// UpdateMany; ignored for UpdateByID.
	Criteria string
}

// UpdateResult describes the outcome of one row in an update operation. For
// UpdateByID, Message is set and Data contains the updated record. For
// UpdateMany, Result is a slice of per-record UpdateResult entries.
type UpdateResult struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    Record          `json:"data"`
	Tasks   json.RawMessage `json:"tasks,omitempty"`
}

// UpdateByID patches a single record.
func (s *RecordService) UpdateByID(ctx context.Context, owner, app, report, recordID string, record Record, opts *UpdateOptions) (*UpdateResult, error) {
	if owner == "" || app == "" || report == "" || recordID == "" {
		return nil, fmt.Errorf("owner, app, report, and recordID are required")
	}
	body := s.buildUpdateBody(record, opts, false)
	path := fmt.Sprintf("/data/%s/%s/report/%s/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report), url.PathEscape(recordID))
	res, err := s.client.do(ctx, requestOptions{
		method: http.MethodPatch,
		path:   path,
		body:   body,
	})
	if err != nil {
		return nil, err
	}
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    Record          `json:"data"`
		Tasks   json.RawMessage `json:"tasks,omitempty"`
	}
	if err := json.Unmarshal(res.envelope, &env); err != nil {
		return nil, fmt.Errorf("decode update response: %w", err)
	}
	return &UpdateResult{Code: env.Code, Message: env.Message, Data: env.Data, Tasks: env.Tasks}, nil
}

// UpdateMany applies a patch to every record matching opts.Criteria. Returns
// per-record results.
func (s *RecordService) UpdateMany(ctx context.Context, owner, app, report string, record Record, opts *UpdateOptions) ([]UpdateResult, error) {
	if owner == "" || app == "" || report == "" {
		return nil, fmt.Errorf("owner, app, and report are required")
	}
	if opts == nil || strings.TrimSpace(opts.Criteria) == "" {
		return nil, fmt.Errorf("opts.Criteria is required for UpdateMany")
	}
	body := s.buildUpdateBody(record, opts, true)
	path := fmt.Sprintf("/data/%s/%s/report/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report))
	res, err := s.client.do(ctx, requestOptions{
		method: http.MethodPatch,
		path:   path,
		body:   body,
	})
	if err != nil {
		return nil, err
	}
	var env struct {
		Code   int            `json:"code"`
		Result []UpdateResult `json:"result"`
	}
	if err := json.Unmarshal(res.envelope, &env); err != nil {
		return nil, fmt.Errorf("decode update response: %w", err)
	}
	return env.Result, nil
}

func (s *RecordService) buildUpdateBody(record Record, opts *UpdateOptions, bulk bool) map[string]any {
	body := map[string]any{"data": record}
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
		if bulk && opts.Criteria != "" {
			body["criteria"] = opts.Criteria
		}
	}
	return body
}

// DeleteResult describes the outcome of one row in a delete operation. For
// DeleteByID, Data carries the deleted record's ID. For DeleteMany, Result
// lists each affected record.
type DeleteResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    Record `json:"data"`
}

// DeleteOptions controls a delete operation.
type DeleteOptions struct {
	// Criteria is required for DeleteMany, ignored for DeleteByID.
	Criteria string
	// Message asks for workflow-generated messages in the result.
	Message bool
	// SkipWorkflow bypasses the listed workflow triggers.
	SkipWorkflow []string
}

// DeleteByID deletes a single record.
func (s *RecordService) DeleteByID(ctx context.Context, owner, app, report, recordID string, opts *DeleteOptions) (*DeleteResult, error) {
	if owner == "" || app == "" || report == "" || recordID == "" {
		return nil, fmt.Errorf("owner, app, report, and recordID are required")
	}
	body := map[string]any{}
	if opts != nil {
		if opts.Message {
			body["result"] = map[string]any{"message": true}
		}
		if len(opts.SkipWorkflow) > 0 && s.client.APIVersion() != APIVersionV2 {
			body["skip_workflow"] = opts.SkipWorkflow
		}
	}
	path := fmt.Sprintf("/data/%s/%s/report/%s/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report), url.PathEscape(recordID))
	var opt requestOptions
	opt.method = http.MethodDelete
	opt.path = path
	if len(body) > 0 {
		opt.body = body
	}
	res, err := s.client.do(ctx, opt)
	if err != nil {
		return nil, err
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    Record `json:"data"`
	}
	if err := json.Unmarshal(res.envelope, &env); err != nil {
		return nil, fmt.Errorf("decode delete response: %w", err)
	}
	return &DeleteResult{Code: env.Code, Message: env.Message, Data: env.Data}, nil
}

// DeleteMany deletes every record matching opts.Criteria.
func (s *RecordService) DeleteMany(ctx context.Context, owner, app, report string, opts *DeleteOptions) ([]DeleteResult, error) {
	if owner == "" || app == "" || report == "" {
		return nil, fmt.Errorf("owner, app, and report are required")
	}
	if opts == nil || strings.TrimSpace(opts.Criteria) == "" {
		return nil, fmt.Errorf("opts.Criteria is required for DeleteMany")
	}
	body := map[string]any{"criteria": opts.Criteria}
	if opts.Message {
		body["result"] = map[string]any{"message": true}
	}
	if len(opts.SkipWorkflow) > 0 && s.client.APIVersion() != APIVersionV2 {
		body["skip_workflow"] = opts.SkipWorkflow
	}
	path := fmt.Sprintf("/data/%s/%s/report/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report))
	res, err := s.client.do(ctx, requestOptions{
		method: http.MethodDelete,
		path:   path,
		body:   body,
	})
	if err != nil {
		return nil, err
	}
	var env struct {
		Code   int            `json:"code"`
		Result []DeleteResult `json:"result"`
	}
	if err := json.Unmarshal(res.envelope, &env); err != nil {
		return nil, fmt.Errorf("decode delete response: %w", err)
	}
	return env.Result, nil
}
