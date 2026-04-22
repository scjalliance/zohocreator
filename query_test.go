package zohocreator

import (
	"strings"
	"testing"
)

func TestQueryBuildParams(t *testing.T) {
	q := NewQuery().
		FromOffset(10).
		LimitN(50).
		MaxRecordsN(1000).
		CriteriaExpr("Name == \"Alice\"").
		FieldConfigMode(FieldConfigCustom).
		FieldsList("Email", "Phone").
		Set("foo", "bar")
	v := q.buildParams()
	got := v.Encode()
	for _, want := range []string{
		"from=10", "limit=50", "max_records=1000",
		"field_config=custom", "fields=Email%2CPhone", "foo=bar",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if !strings.Contains(got, "criteria=") {
		t.Errorf("missing criteria in %q", got)
	}
}

func TestQueryZero(t *testing.T) {
	var q *Query
	if q.buildParams().Encode() != "" {
		t.Error("nil query should encode to empty")
	}
	q2 := NewQuery()
	if q2.buildParams().Encode() != "" {
		t.Error("empty query should encode to empty")
	}
}

func TestQueryClone(t *testing.T) {
	q := NewQuery().FieldsList("a", "b").Set("x", "1")
	q.RecordCursor = "cur"
	c := q.clone()
	c.Fields = append(c.Fields, "z")
	c.Extra.Set("x", "2")
	if len(q.Fields) != 2 {
		t.Error("clone leaked Fields mutation")
	}
	if q.Extra.Get("x") != "1" {
		t.Error("clone leaked Extra mutation")
	}
	if c.RecordCursor != "cur" {
		t.Error("clone lost RecordCursor")
	}
}

func TestCriteriaHelpers(t *testing.T) {
	if Eq("Age", 30) != "Age == 30" {
		t.Errorf("Eq int: %q", Eq("Age", 30))
	}
	if Eq("Name", "Alice") != `Name == "Alice"` {
		t.Errorf("Eq string: %q", Eq("Name", "Alice"))
	}
	if Eq("Name", `He said "hi"`) != `Name == "He said \"hi\""` {
		t.Errorf("Eq escape: %q", Eq("Name", `He said "hi"`))
	}
	if Eq("Active", true) != "Active == true" {
		t.Errorf("Eq bool: %q", Eq("Active", true))
	}
	if Contains("Notes", "urgent") != `Notes.contains("urgent")` {
		t.Errorf("Contains: %q", Contains("Notes", "urgent"))
	}
	if Ne("X", 1) != "X != 1" {
		t.Error("Ne")
	}
	if Gt("X", 1) != "X > 1" {
		t.Error("Gt")
	}
	if Ge("X", 1) != "X >= 1" {
		t.Error("Ge")
	}
	if Lt("X", 1) != "X < 1" {
		t.Error("Lt")
	}
	if Le("X", 1) != "X <= 1" {
		t.Error("Le")
	}
	if StartsWith("X", "pre") != `X.startsWith("pre")` {
		t.Error("StartsWith")
	}
	if EndsWith("X", "suf") != `X.endsWith("suf")` {
		t.Error("EndsWith")
	}
}

func TestCriteriaCombinators(t *testing.T) {
	if And(Eq("A", 1)) != "A == 1" {
		t.Errorf("And one: %q", And(Eq("A", 1)))
	}
	if And() != "" {
		t.Error("And zero")
	}
	got := And(Eq("A", 1), Eq("B", 2))
	if got != "(A == 1 && B == 2)" {
		t.Errorf("And two: %q", got)
	}
	if Or(Eq("A", 1), Eq("B", 2)) != "(A == 1 || B == 2)" {
		t.Errorf("Or: %q", Or(Eq("A", 1), Eq("B", 2)))
	}
	if Not("A == 1") != "!(A == 1)" {
		t.Errorf("Not")
	}
}
