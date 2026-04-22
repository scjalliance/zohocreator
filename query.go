package zohocreator

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// FieldConfig controls which fields Get Records returns.
type FieldConfig string

const (
	// FieldConfigQuickView returns the fields shown in the report's
	// quick-view column set (the default when unset).
	FieldConfigQuickView FieldConfig = "quick_view"
	// FieldConfigDetailView returns every field shown in the detail view.
	FieldConfigDetailView FieldConfig = "detail_view"
	// FieldConfigCustom returns only fields listed in Query.Fields.
	FieldConfigCustom FieldConfig = "custom"
	// FieldConfigAll returns every field on the underlying form.
	FieldConfigAll FieldConfig = "all"
)

// Query is a fluent builder for record-list parameters. All fields are
// optional; zero/empty values are omitted from the outgoing request.
type Query struct {
	From        int
	Limit       int
	MaxRecords  int
	Criteria    string
	FieldConfig FieldConfig
	Fields      []string
	// RecordCursor is the pagination cursor returned from a previous page
	// (header `record_cursor`). Set to resume; leave empty for the first
	// page. List / All handle this transparently.
	RecordCursor string
	// Extra holds any additional raw query parameters as an escape hatch
	// for endpoints that grow new options.
	Extra url.Values
}

// NewQuery returns an empty query builder.
func NewQuery() *Query { return &Query{} }

// FromOffset sets the `from` pagination offset (0-based).
func (q *Query) FromOffset(n int) *Query { q.From = n; return q }

// LimitN caps the number of records in a single response. Alias for the
// `limit` query parameter.
func (q *Query) LimitN(n int) *Query { q.Limit = n; return q }

// MaxRecordsN sets the `max_records` parameter (200/500/1000; 200 default).
func (q *Query) MaxRecordsN(n int) *Query { q.MaxRecords = n; return q }

// CriteriaExpr sets the Creator filter expression (e.g.
// `(Email.startsWith("a") && Active == true)`). See the Criteria helpers below.
func (q *Query) CriteriaExpr(expr string) *Query { q.Criteria = expr; return q }

// FieldsList requests explicit field link names (requires
// FieldConfig=FieldConfigCustom).
func (q *Query) FieldsList(fields ...string) *Query {
	q.Fields = append(q.Fields, fields...)
	return q
}

// FieldConfigMode sets which field set is returned.
func (q *Query) FieldConfigMode(m FieldConfig) *Query { q.FieldConfig = m; return q }

// Cursor sets the `record_cursor` header value.
func (q *Query) Cursor(c string) *Query { q.RecordCursor = c; return q }

// Set adds an arbitrary raw query parameter.
func (q *Query) Set(key, value string) *Query {
	if q.Extra == nil {
		q.Extra = url.Values{}
	}
	q.Extra.Set(key, value)
	return q
}

// buildParams returns the URL-encoded query string parameters.
func (q *Query) buildParams() url.Values {
	v := url.Values{}
	if q == nil {
		return v
	}
	if q.From > 0 {
		v.Set("from", strconv.Itoa(q.From))
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.MaxRecords > 0 {
		v.Set("max_records", strconv.Itoa(q.MaxRecords))
	}
	if q.Criteria != "" {
		v.Set("criteria", q.Criteria)
	}
	if q.FieldConfig != "" {
		v.Set("field_config", string(q.FieldConfig))
	}
	if len(q.Fields) > 0 {
		v.Set("fields", strings.Join(q.Fields, ","))
	}
	for k, vals := range q.Extra {
		for _, val := range vals {
			v.Add(k, val)
		}
	}
	return v
}

// clone returns a deep copy so pagination can safely mutate the cursor.
func (q *Query) clone() *Query {
	if q == nil {
		return &Query{}
	}
	dup := *q
	if q.Fields != nil {
		dup.Fields = append([]string(nil), q.Fields...)
	}
	if q.Extra != nil {
		dup.Extra = url.Values{}
		for k, vals := range q.Extra {
			dup.Extra[k] = append([]string(nil), vals...)
		}
	}
	return &dup
}

// Criteria assembles Creator filter expressions safely. Values are escaped and
// wrapped with the proper quoting for string types. Non-string types are
// rendered without quotes.
//
//	expr := zohocreator.Eq("Email", "a@b.com")
//	expr := zohocreator.And(
//	    zohocreator.Contains("Name", "smith"),
//	    zohocreator.Gt("Amount", 100),
//	)
func quoteString(s string) string {
	// Creator uses double-quoted string literals in criteria. Backslashes
	// and quotes need escaping.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func formatLiteral(v any) string {
	switch t := v.(type) {
	case string:
		return quoteString(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case nil:
		return "null"
	default:
		return quoteString(fmt.Sprintf("%v", t))
	}
}

// Eq renders `field == value`.
func Eq(field string, value any) string { return field + " == " + formatLiteral(value) }

// Ne renders `field != value`.
func Ne(field string, value any) string { return field + " != " + formatLiteral(value) }

// Gt renders `field > value`.
func Gt(field string, value any) string { return field + " > " + formatLiteral(value) }

// Ge renders `field >= value`.
func Ge(field string, value any) string { return field + " >= " + formatLiteral(value) }

// Lt renders `field < value`.
func Lt(field string, value any) string { return field + " < " + formatLiteral(value) }

// Le renders `field <= value`.
func Le(field string, value any) string { return field + " <= " + formatLiteral(value) }

// Contains renders `field.contains("value")`.
func Contains(field, value string) string { return field + ".contains(" + quoteString(value) + ")" }

// StartsWith renders `field.startsWith("value")`.
func StartsWith(field, value string) string {
	return field + ".startsWith(" + quoteString(value) + ")"
}

// EndsWith renders `field.endsWith("value")`.
func EndsWith(field, value string) string { return field + ".endsWith(" + quoteString(value) + ")" }

// And joins terms with Creator's `&&` conjunction, wrapping the result in
// parentheses.
func And(terms ...string) string { return joinTerms("&&", terms) }

// Or joins terms with Creator's `||` disjunction, wrapping the result in
// parentheses.
func Or(terms ...string) string { return joinTerms("||", terms) }

// Not negates a criterion: `!(term)`.
func Not(term string) string { return "!(" + term + ")" }

func joinTerms(op string, terms []string) string {
	if len(terms) == 0 {
		return ""
	}
	if len(terms) == 1 {
		return terms[0]
	}
	return "(" + strings.Join(terms, " "+op+" ") + ")"
}
