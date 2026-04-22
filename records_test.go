package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRecordsGetAndAll(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/creator/v2.1/data/owner/app/report/rpt") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		cursor := r.Header.Get("record_cursor")
		switch cursor {
		case "":
			w.Header().Set("record_cursor", "next123")
			fmt.Fprint(w, `{"code":3000,"data":[{"ID":"1","Name":"Alice"},{"ID":"2","Name":"Bob"}]}`)
		case "next123":
			fmt.Fprint(w, `{"code":3000,"data":[{"ID":"3","Name":"Carol"}]}`)
		default:
			t.Errorf("unexpected cursor %q", cursor)
		}
	})
	ctx := context.Background()
	names := []string{}
	for rec, err := range c.Records.All(ctx, "owner", "app", "rpt", nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, rec.String("Name"))
	}
	if strings.Join(names, ",") != "Alice,Bob,Carol" {
		t.Errorf("names=%q", names)
	}
}

func TestRecordsPageHasNext(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("record_cursor", "abc")
		fmt.Fprint(w, `{"code":3000,"data":[{"ID":"1"}]}`)
	})
	page, err := c.Records.Get(context.Background(), "o", "a", "r", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasNext() {
		t.Error("HasNext should be true")
	}
	if page.Cursor != "abc" {
		t.Errorf("cursor=%q", page.Cursor)
	}
}

func TestRecordsAllNoCursor(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":3000,"data":[{"ID":"1"},{"ID":"2"}]}`)
	})
	count := 0
	for _, err := range c.Records.All(context.Background(), "owner", "app", "rpt", nil) {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 2 {
		t.Errorf("got %d records", count)
	}
}

func TestRecordsGetByID(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rpt/42") {
			t.Errorf("path=%s", r.URL.Path)
		}
		fmt.Fprint(w, `{"code":3000,"data":{"ID":"42","Name":"Alice"}}`)
	})
	rec, err := c.Records.GetByID(context.Background(), "owner", "app", "rpt", "42")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID() != "42" {
		t.Errorf("ID=%q", rec.ID())
	}
	if rec.String("Name") != "Alice" {
		t.Errorf("Name=%q", rec.String("Name"))
	}
}

func TestRecordsAdd(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var env map[string]any
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatal(err)
		}
		if _, ok := env["data"]; !ok {
			t.Error("missing data")
		}
		if res, ok := env["result"].(map[string]any); ok {
			if _, ok := res["fields"]; !ok {
				t.Error("missing result.fields")
			}
		} else {
			t.Error("missing result map")
		}
		fmt.Fprint(w, `{"code":3000,"result":[{"code":3000,"data":{"ID":"x1","Name":"Alice"},"message":"ok"}]}`)
	})
	out, err := c.Records.Add(context.Background(), "owner", "app", "form",
		[]Record{{"Name": "Alice"}},
		&AddOptions{ReturnFields: []string{"Name"}, Message: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Data.ID() != "x1" {
		t.Errorf("%+v", out)
	}
}

func TestRecordsUpdateByID(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method=%s", r.Method)
		}
		fmt.Fprint(w, `{"code":3000,"message":"ok","data":{"ID":"1","Name":"X"}}`)
	})
	res, err := c.Records.UpdateByID(context.Background(), "o", "a", "r", "1",
		Record{"Name": "X"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Data.String("Name") != "X" {
		t.Errorf("data=%v", res.Data)
	}
}

func TestRecordsUpdateManyRequiresCriteria(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := c.Records.UpdateMany(context.Background(), "o", "a", "r",
		Record{"X": 1}, nil)
	if err == nil {
		t.Error("expected error when criteria missing")
	}
}

func TestRecordsDeleteByID(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method=%s", r.Method)
		}
		fmt.Fprint(w, `{"code":3000,"message":"Deleted","data":{"ID":1}}`)
	})
	res, err := c.Records.DeleteByID(context.Background(), "o", "a", "r", "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != 3000 {
		t.Errorf("code=%d", res.Code)
	}
}

func TestRecordsDeleteManyRequiresCriteria(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := c.Records.DeleteMany(context.Background(), "o", "a", "r", nil)
	if err == nil {
		t.Error("expected error")
	}
}
