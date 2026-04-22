package zohocreator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBulkCreate(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"query"`) {
			t.Error("missing query wrapper")
		}
		fmt.Fprint(w, `{"code":3000,"details":{"id":"job1","operation":"read","status":"Scheduled","created_by":"u","created_time":"t"}}`)
	})
	job, err := c.Bulk.Create(context.Background(), "o", "a", "r", &BulkReadQuery{Criteria: "X==1"})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "job1" {
		t.Errorf("id=%q", job.ID)
	}
}

func TestBulkStatus(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":3000,"details":{"id":"job1","status":"Completed","result":{"count":5,"download_url":"/creator/v2.1/bulk/o/a/report/r/read/job1/result"}}}`)
	})
	job, err := c.Bulk.Status(context.Background(), "o", "a", "r", "job1")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "Completed" || job.Result.Count != 5 {
		t.Errorf("%+v", job)
	}
}

func TestBulkDownload(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte("PK\x03\x04mockzip"))
	})
	var buf bytes.Buffer
	n, err := c.Bulk.DownloadResult(context.Background(), "o", "a", "r", "job1", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) || buf.Len() == 0 {
		t.Errorf("n=%d buf=%d", n, buf.Len())
	}
}
