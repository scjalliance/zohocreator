package zohocreator

import (
	"encoding/json"
	"strconv"
)

// Response codes surfaced by the Zoho Creator API. 3000 indicates success for
// most endpoints; the remainder are error codes echoed in Error.Code.
const (
	CodeSuccess               = 3000
	CodeAuthFailure           = 1030
	CodeInvalidScope          = 2945
	CodeIAMError              = 1000
	CodeInvalidMethod         = 1020
	CodePermissionDenied      = 2933
	CodeAddPermissionDenied   = 2899
	CodeDisabledApp           = 1080
	CodeFormValidationLimit   = 2965
	CodeOwnerNotFound         = 1040
	CodeAppNotFound           = 2892
	CodeFormNotFound          = 2893
	CodeRequestBodyMissing    = 3020
	CodeBadRequest            = 3100
	CodeMaxBatchExceeded      = 3950
	CodeDataValidationFailure = 3070
	CodeRecordLimitReached    = 3060
	CodeDeveloperLimit        = 4000
	CodeRateLimit             = 2955
	CodeAPIAccessDenied       = 1130
)

// codeEnvelope is the common JSON prefix for nearly every Creator response.
// Individual endpoints add fields like data / result / details / forms /
// reports / fields etc.; we decode those in the endpoint-specific methods.
type codeEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// Record is a dynamically-typed Creator record. Field link names map to their
// values. Values may be strings, numbers, booleans, nil, nested objects
// (lookups, subforms), or arrays (multi-select, subform rows). Use Get to
// read a field by link name.
type Record map[string]any

// Get returns the value for link name k, or nil when absent.
func (r Record) Get(k string) any { return r[k] }

// String returns the value for k coerced to string: plain strings are returned
// as-is, other types are encoded as JSON.
func (r Record) String(k string) string {
	v, ok := r[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// ID returns the record's ID field, empty if missing. Creator returns IDs as
// strings (large int64-ish), sometimes nested in objects — this helper extracts
// the common cases.
func (r Record) ID() string {
	v, ok := r["ID"]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	}
	return ""
}
