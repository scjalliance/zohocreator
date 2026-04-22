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

func TestFileUpload(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("content-type=%s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hello world") {
			t.Error("body missing content")
		}
		fmt.Fprint(w, `{"code":3000,"message":"Uploaded"}`)
	})
	res, err := c.Files.Upload(context.Background(), "o", "a", "r", "1", "Attachment",
		"doc.txt", "text/plain", bytes.NewReader([]byte("hello world")))
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != 3000 {
		t.Errorf("code=%d", res.Code)
	}
}

func TestFileDownload(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="doc.txt"`)
		_, _ = w.Write([]byte("data"))
	})
	var buf bytes.Buffer
	fn, n, err := c.Files.Download(context.Background(), "o", "a", "r", "1", "Attachment", "", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if fn != "doc.txt" || n != 4 {
		t.Errorf("fn=%q n=%d", fn, n)
	}
}

func TestFileDownloadPrivatelink(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("privatelink should skip auth")
		}
		if r.URL.Query().Get("privatelink") != "XYZ" {
			t.Error("privatelink not passed")
		}
		w.Header().Set("Content-Disposition", `attachment; filename="x.bin"`)
		_, _ = w.Write([]byte{0})
	})
	var buf bytes.Buffer
	fn, _, err := c.Files.Download(context.Background(), "o", "a", "r", "1", "F", "XYZ", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if fn != "x.bin" {
		t.Errorf("fn=%q", fn)
	}
}

func TestParseFilename(t *testing.T) {
	if parseFilename("") != "" {
		t.Error("empty")
	}
	if got := parseFilename(`attachment; filename="a b.txt"`); got != "a b.txt" {
		t.Errorf("got %q", got)
	}
	if got := parseFilename(`attachment; filename*=UTF-8''a%20b.txt`); got != "a b.txt" {
		t.Errorf("rfc2231 got %q", got)
	}
}
