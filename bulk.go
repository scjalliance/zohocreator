package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// BulkService wraps the Bulk Read API. A bulk read is an asynchronous job that
// materialises a report snapshot into a downloadable CSV (or result archive).
//
// Lifecycle: Create -> poll Status until "Completed" -> download via
// DownloadResult. Requires scope ZohoCreator.bulk.CREATE to submit and
// ZohoCreator.bulk.READ to poll/download.
type BulkService struct{ client *Client }

// BulkReadQuery is the Create job request body.
type BulkReadQuery struct {
	// Criteria optionally filters the rows the job extracts.
	Criteria string `json:"criteria,omitempty"`
	// MaxRecords caps the rows (server accepts 100_000..200_000). Leave
	// zero to let Zoho pick the default.
	MaxRecords int `json:"max_records,omitempty"`
	// Fields optionally restricts the columns in the output.
	Fields []string `json:"fields,omitempty"`
}

// BulkJob describes a bulk read job. Status transitions are: Scheduled,
// In-progress, Completed, Failed. Result fields are populated only when the
// status is Completed.
type BulkJob struct {
	ID          string `json:"id"`
	Operation   string `json:"operation"`
	Status      string `json:"status"`
	CreatedBy   string `json:"created_by"`
	CreatedTime string `json:"created_time"`

	Result *BulkJobResult `json:"result,omitempty"`
}

// BulkJobResult is the Completed-job result summary.
type BulkJobResult struct {
	Count       int64  `json:"count"`
	DownloadURL string `json:"download_url"`
}

// Create submits a new bulk-read job and returns the job record. The returned
// job is typically in "Scheduled" status.
func (s *BulkService) Create(ctx context.Context, owner, app, report string, query *BulkReadQuery) (*BulkJob, error) {
	if owner == "" || app == "" || report == "" {
		return nil, fmt.Errorf("owner, app, and report are required")
	}
	body := map[string]any{}
	if query != nil {
		body["query"] = query
	} else {
		body["query"] = map[string]any{}
	}
	path := fmt.Sprintf("/bulk/%s/%s/report/%s/read",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report))
	res, err := s.client.do(ctx, requestOptions{
		method: http.MethodPost,
		path:   path,
		body:   body,
	})
	if err != nil {
		return nil, err
	}
	return decodeBulkJob(res.envelope)
}

// Status polls the status of a bulk read job.
func (s *BulkService) Status(ctx context.Context, owner, app, report, jobID string) (*BulkJob, error) {
	if owner == "" || app == "" || report == "" || jobID == "" {
		return nil, fmt.Errorf("owner, app, report, and jobID are required")
	}
	path := fmt.Sprintf("/bulk/%s/%s/report/%s/read/%s",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report), url.PathEscape(jobID))
	res, err := s.client.do(ctx, requestOptions{method: http.MethodGet, path: path})
	if err != nil {
		return nil, err
	}
	return decodeBulkJob(res.envelope)
}

// DownloadResult streams the completed job's result to dst, returning the
// number of bytes copied. The body is typically a .zip archive containing a
// CSV.
func (s *BulkService) DownloadResult(ctx context.Context, owner, app, report, jobID string, dst io.Writer) (int64, error) {
	if owner == "" || app == "" || report == "" || jobID == "" {
		return 0, fmt.Errorf("owner, app, report, and jobID are required")
	}
	if dst == nil {
		return 0, fmt.Errorf("dst writer is required")
	}
	path := fmt.Sprintf("/bulk/%s/%s/report/%s/read/%s/result",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report), url.PathEscape(jobID))
	res, err := s.client.do(ctx, requestOptions{
		method: http.MethodGet,
		path:   path,
		stream: true,
		accept: "application/zip",
	})
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.stream.Close() }()
	return io.Copy(dst, res.stream)
}

func decodeBulkJob(body []byte) (*BulkJob, error) {
	var env struct {
		Code    int     `json:"code"`
		Details BulkJob `json:"details"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode bulk job: %w", err)
	}
	return &env.Details, nil
}
