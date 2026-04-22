package zohocreator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
)

// FileService wraps Upload File and Download File endpoints (including the
// subform-scoped download variant).
type FileService struct{ client *Client }

// UploadResult is the Upload File response envelope.
type UploadResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Upload attaches file to a file-upload field of a record. The field must be
// a FileUpload, Image, Audio, Video, or Signature column on the underlying
// form. Requires scope ZohoCreator.report.CREATE.
//
// filename is used verbatim in the multipart Content-Disposition; contentType
// defaults to application/octet-stream.
func (s *FileService) Upload(ctx context.Context, owner, app, report, recordID, field, filename, contentType string, body io.Reader) (*UploadResult, error) {
	if owner == "" || app == "" || report == "" || recordID == "" || field == "" {
		return nil, fmt.Errorf("owner, app, report, recordID, and field are required")
	}
	if body == nil {
		return nil, fmt.Errorf("upload body is required")
	}
	if filename == "" {
		filename = "upload"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("create multipart part: %w", err)
	}
	if _, err := io.Copy(part, body); err != nil {
		return nil, fmt.Errorf("write upload body: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	path := fmt.Sprintf("/v2.1/data/%s/%s/report/%s/%s/%s/upload",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report),
		url.PathEscape(recordID), url.PathEscape(field))

	res, err := s.client.do(ctx, requestOptions{
		method:      http.MethodPost,
		path:        path,
		rawBody:     &buf,
		contentType: mw.FormDataContentType(),
	})
	if err != nil {
		return nil, err
	}
	var ur UploadResult
	if len(bytes.TrimSpace(res.envelope)) > 0 {
		if err := json.Unmarshal(res.envelope, &ur); err != nil {
			return nil, fmt.Errorf("decode upload response: %w", err)
		}
	}
	return &ur, nil
}

// Download streams a file-upload field to dst, returning the server-supplied
// filename (when present) and number of bytes copied. Requires scope
// ZohoCreator.report.READ (unless privatelink is used).
//
// When privatelink is non-empty the request uses it as a query parameter,
// bypassing OAuth (used with publish-link style private-link downloads).
func (s *FileService) Download(ctx context.Context, owner, app, report, recordID, field, privatelink string, dst io.Writer) (filename string, n int64, err error) {
	if owner == "" || app == "" || report == "" || recordID == "" || field == "" {
		return "", 0, fmt.Errorf("owner, app, report, recordID, and field are required")
	}
	if dst == nil {
		return "", 0, fmt.Errorf("dst writer is required")
	}
	path := fmt.Sprintf("/v2.1/data/%s/%s/report/%s/%s/%s/download",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report),
		url.PathEscape(recordID), url.PathEscape(field))
	return s.downloadPath(ctx, path, privatelink, dst)
}

// DownloadSubform streams a file attached to a subform row.
func (s *FileService) DownloadSubform(ctx context.Context, owner, app, report, recordID, subform, field, subformRecordID, privatelink string, dst io.Writer) (filename string, n int64, err error) {
	if owner == "" || app == "" || report == "" || recordID == "" || subform == "" || field == "" || subformRecordID == "" {
		return "", 0, fmt.Errorf("owner, app, report, recordID, subform, field, and subformRecordID are required")
	}
	if dst == nil {
		return "", 0, fmt.Errorf("dst writer is required")
	}
	path := fmt.Sprintf("/v2.1/data/%s/%s/report/%s/%s/%s/%s/%s/download",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(report),
		url.PathEscape(recordID), url.PathEscape(subform),
		url.PathEscape(field), url.PathEscape(subformRecordID))
	return s.downloadPath(ctx, path, privatelink, dst)
}

func (s *FileService) downloadPath(ctx context.Context, path, privatelink string, dst io.Writer) (string, int64, error) {
	opts := requestOptions{
		method: http.MethodGet,
		path:   path,
		stream: true,
	}
	if privatelink != "" {
		opts.query = url.Values{"privatelink": []string{privatelink}}
		opts.noAuth = true
	}
	res, err := s.client.do(ctx, opts)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = res.stream.Close() }()
	n, cerr := io.Copy(dst, res.stream)
	if cerr != nil {
		return "", n, fmt.Errorf("stream body: %w", cerr)
	}
	return parseFilename(res.headers.Get("Content-Disposition")), n, nil
}

// parseFilename extracts the filename from a Content-Disposition header.
// mime.ParseMediaType decodes RFC 2231-encoded `filename*` parameters and
// stores the decoded value under the plain `filename` key.
func parseFilename(cd string) string {
	if cd == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return ""
	}
	return params["filename"]
}
