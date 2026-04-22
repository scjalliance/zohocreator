package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// newV2TestClient is newTestClient's sibling for the legacy v2 API.
func newV2TestClient(t *testing.T, fn http.HandlerFunc) *Client {
	t.Helper()
	c, _ := newTestClient(t, fn)
	c.config.APIVersion = APIVersionV2
	return c
}

func TestV2URLPath(t *testing.T) {
	var gotPath string
	c := newV2TestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, `{"code":3000,"applications":[]}`)
	})
	if _, err := c.Meta.Applications(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/creator/v2/meta/applications" {
		t.Errorf("v2 routed to %q", gotPath)
	}
}

func TestV2OffsetPagination(t *testing.T) {
	var n int32
	c := newV2TestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		from := q.Get("from")
		limit := q.Get("limit")
		if limit != "3" {
			t.Errorf("limit=%q want 3", limit)
		}
		call := atomic.AddInt32(&n, 1)
		switch call {
		case 1:
			if from != "" && from != "0" {
				t.Errorf("first call from=%q", from)
			}
			fmt.Fprint(w, `{"code":3000,"data":[{"ID":"1"},{"ID":"2"},{"ID":"3"}]}`)
		case 2:
			if from != "3" {
				t.Errorf("second call from=%q want 3", from)
			}
			fmt.Fprint(w, `{"code":3000,"data":[{"ID":"4"},{"ID":"5"}]}`)
		default:
			t.Errorf("unexpected call %d", call)
		}
	})
	got := []string{}
	for rec, err := range c.Records.All(context.Background(), "o", "a", "r",
		NewQuery().LimitN(3)) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, rec.ID())
	}
	if strings.Join(got, ",") != "1,2,3,4,5" {
		t.Errorf("got=%v", got)
	}
}

func TestV2DropsV21OnlyQueryParams(t *testing.T) {
	c := newV2TestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for _, drop := range []string{"max_records", "field_config", "fields"} {
			if q.Get(drop) != "" {
				t.Errorf("v2 request should not carry %q, got %q", drop, q.Get(drop))
			}
		}
		if q.Get("criteria") == "" {
			t.Error("criteria should survive on v2")
		}
		fmt.Fprint(w, `{"code":3000,"data":[]}`)
	})
	q := NewQuery().
		CriteriaExpr("Email.contains(\"@x\")").
		MaxRecordsN(500).
		FieldConfigMode(FieldConfigCustom).
		FieldsList("A", "B")
	if _, err := c.Records.Get(context.Background(), "o", "a", "r", q); err != nil {
		t.Fatal(err)
	}
}

func TestV2DropsSkipWorkflowOnWrites(t *testing.T) {
	c := newV2TestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var env map[string]any
		_ = json.Unmarshal(body, &env)
		if _, has := env["skip_workflow"]; has {
			t.Error("v2 Add request should not carry skip_workflow")
		}
		fmt.Fprint(w, `{"code":3000,"result":[{"code":3000,"data":{"ID":"1"}}]}`)
	})
	_, err := c.Records.Add(context.Background(), "o", "a", "f",
		[]Record{{"Name": "x"}},
		&AddOptions{SkipWorkflow: []string{"all"}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestV2IgnoresRecordCursorHeader(t *testing.T) {
	var sawCursor bool
	c := newV2TestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("record_cursor") != "" {
			sawCursor = true
		}
		fmt.Fprint(w, `{"code":3000,"data":[]}`)
	})
	// Caller sets a cursor, but v2 must not send the header.
	if _, err := c.Records.Get(context.Background(), "o", "a", "r",
		NewQuery().Cursor("ignored-on-v2")); err != nil {
		t.Fatal(err)
	}
	if sawCursor {
		t.Error("v2 should not emit record_cursor header")
	}
}

func TestAPIVersionValidate(t *testing.T) {
	zero := 0
	for _, ver := range []APIVersion{APIVersionV21, APIVersionV2, ""} {
		cfg := Config{
			DataCenter:  DCUS,
			AccessToken: "t",
			APIVersion:  ver,
			MaxRetries:  &zero,
		}
		if err := cfg.validate(); err != nil {
			t.Errorf("%q: %v", ver, err)
		}
	}
	bad := Config{
		DataCenter:  DCUS,
		AccessToken: "t",
		APIVersion:  "v9",
	}
	if err := bad.validate(); err == nil {
		t.Error("expected error for v9")
	}
}
