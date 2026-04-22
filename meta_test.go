package zohocreator

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestMetaApplications(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/creator/v2.1/meta/applications" {
			t.Errorf("path=%s", r.URL.Path)
		}
		fmt.Fprint(w, `{"code":3000,"applications":[{"application_name":"Zylker","link_name":"zylker","category":1}]}`)
	})
	apps, err := c.Meta.Applications(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].LinkName != "zylker" {
		t.Errorf("apps=%+v", apps)
	}
}

func TestMetaApplicationsByWorkspace(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/meta/jason/applications") {
			t.Errorf("path=%s", r.URL.Path)
		}
		fmt.Fprint(w, `{"code":3000,"applications":[]}`)
	})
	if _, err := c.Meta.ApplicationsByWorkspace(context.Background(), "jason"); err != nil {
		t.Fatal(err)
	}
}

func TestMetaForms(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":3000,"forms":[{"display_name":"X","link_name":"X1","type":1}]}`)
	})
	forms, err := c.Meta.Forms(context.Background(), "o", "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 1 || forms[0].LinkName != "X1" {
		t.Errorf("%+v", forms)
	}
}

func TestMetaReports(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":3000,"reports":[{"display_name":"R","link_name":"R1","type":5}]}`)
	})
	r, _ := c.Meta.Reports(context.Background(), "o", "a")
	if len(r) != 1 {
		t.Error("reports")
	}
}

func TestMetaFields(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":3000,"fields":[{"display_name":"Name","link_name":"Name","type":29,"mandatory":true}]}`)
	})
	fs, _ := c.Meta.Fields(context.Background(), "o", "a", "f")
	if len(fs) != 1 {
		t.Error("fields")
	}
}

func TestMetaPagesAndSections(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pages"):
			fmt.Fprint(w, `{"code":3000,"pages":[{"display_name":"Home","link_name":"Home"}]}`)
		case strings.HasSuffix(r.URL.Path, "/sections"):
			fmt.Fprint(w, `{"code":3000,"sections":[{"display_name":"S","link_name":"s","components":[]}]}`)
		default:
			t.Errorf("path=%s", r.URL.Path)
		}
	})
	if _, err := c.Meta.Pages(context.Background(), "o", "a"); err != nil {
		t.Error(err)
	}
	if _, err := c.Meta.Sections(context.Background(), "o", "a"); err != nil {
		t.Error(err)
	}
}

func TestMetaValidation(t *testing.T) {
	m := &MetaService{client: &Client{}}
	if _, err := m.Forms(context.Background(), "", ""); err == nil {
		t.Error("expected validation error for empty params")
	}
	if _, err := m.Reports(context.Background(), "", ""); err == nil {
		t.Error("expected validation error")
	}
	if _, err := m.Fields(context.Background(), "o", "a", ""); err == nil {
		t.Error("expected validation error")
	}
	if _, err := m.Pages(context.Background(), "", ""); err == nil {
		t.Error("expected validation error")
	}
	if _, err := m.Sections(context.Background(), "", ""); err == nil {
		t.Error("expected validation error")
	}
	if _, err := m.ApplicationsByWorkspace(context.Background(), ""); err == nil {
		t.Error("expected validation error")
	}
}

func TestFieldTypeName(t *testing.T) {
	if FieldTypeName(1) != "SingleLine" {
		t.Error("1")
	}
	if FieldTypeName(18) != "Subform" {
		t.Error("18")
	}
	if FieldTypeName(9999) != "" {
		t.Error("unknown")
	}
}
