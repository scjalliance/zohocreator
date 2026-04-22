package zohocreator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// MetaService wraps Zoho Creator's metadata endpoints: applications, forms,
// reports, fields, pages, and sections.
type MetaService struct{ client *Client }

// Application describes a Creator application owned by the authenticated user
// or a specific workspace owner.
type Application struct {
	ApplicationName string `json:"application_name"`
	DateFormat      string `json:"date_format"`
	CreationDate    string `json:"creation_date"`
	LinkName        string `json:"link_name"`
	Category        int    `json:"category"`
	TimeZone        string `json:"time_zone"`
	CreatedBy       string `json:"created_by"`
	WorkspaceName   string `json:"workspace_name"`
}

// Form describes a form in an application.
type Form struct {
	DisplayName string `json:"display_name"`
	LinkName    string `json:"link_name"`
	Type        int    `json:"type"`
}

// Report describes a report in an application.
type Report struct {
	DisplayName string `json:"display_name"`
	LinkName    string `json:"link_name"`
	Type        int    `json:"type"`
}

// AppPage describes an application page (named AppPage to avoid colliding
// with the generic pagination type Page[T]).
type AppPage struct {
	DisplayName string `json:"display_name"`
	LinkName    string `json:"link_name"`
}

// SectionComponent is a single component entry inside a Section.
type SectionComponent struct {
	DisplayName string `json:"display_name"`
	LinkName    string `json:"link_name"`
	Type        int    `json:"type"`
	PageType    int    `json:"page_type,omitempty"`
	ViewType    int    `json:"view_type,omitempty"`
}

// Section groups reports/forms in the left-nav of a Creator app.
type Section struct {
	DisplayName string             `json:"display_name"`
	LinkName    string             `json:"link_name"`
	Components  []SectionComponent `json:"components"`
}

// Field describes a form field definition. Creator's "type" is a numeric code;
// see FieldTypeName for the human-readable mapping.
type Field struct {
	DisplayName string `json:"display_name"`
	LinkName    string `json:"link_name"`
	Type        int    `json:"type"`
	Mandatory   bool   `json:"mandatory"`
	Unique      bool   `json:"unique"`
	MaxChar     int    `json:"max_char,omitempty"`
}

// Applications returns every Creator application the authenticated user can
// access across all workspaces.
//
// Requires scope ZohoCreator.dashboard.READ.
func (m *MetaService) Applications(ctx context.Context) ([]Application, error) {
	return m.applications(ctx, "/v2.1/meta/applications")
}

// ApplicationsByWorkspace returns the applications owned by the given
// workspace owner (also known as "account owner name"). The authenticated
// user must have access to that workspace.
func (m *MetaService) ApplicationsByWorkspace(ctx context.Context, owner string) ([]Application, error) {
	if owner == "" {
		return nil, fmt.Errorf("owner is required")
	}
	return m.applications(ctx, "/v2.1/meta/"+url.PathEscape(owner)+"/applications")
}

func (m *MetaService) applications(ctx context.Context, path string) ([]Application, error) {
	res, err := m.client.do(ctx, requestOptions{method: http.MethodGet, path: path})
	if err != nil {
		return nil, err
	}
	var body struct {
		Code         int           `json:"code"`
		Applications []Application `json:"applications"`
	}
	if err := json.Unmarshal(res.envelope, &body); err != nil {
		return nil, fmt.Errorf("decode applications: %w", err)
	}
	return body.Applications, nil
}

// Forms returns every form in the given application.
//
// Requires scope ZohoCreator.meta.application.READ.
func (m *MetaService) Forms(ctx context.Context, owner, app string) ([]Form, error) {
	if owner == "" || app == "" {
		return nil, fmt.Errorf("owner and app are required")
	}
	path := fmt.Sprintf("/v2.1/meta/%s/%s/forms", url.PathEscape(owner), url.PathEscape(app))
	res, err := m.client.do(ctx, requestOptions{method: http.MethodGet, path: path})
	if err != nil {
		return nil, err
	}
	var body struct {
		Code  int    `json:"code"`
		Forms []Form `json:"forms"`
	}
	if err := json.Unmarshal(res.envelope, &body); err != nil {
		return nil, fmt.Errorf("decode forms: %w", err)
	}
	return body.Forms, nil
}

// Reports returns every report in the given application.
func (m *MetaService) Reports(ctx context.Context, owner, app string) ([]Report, error) {
	if owner == "" || app == "" {
		return nil, fmt.Errorf("owner and app are required")
	}
	path := fmt.Sprintf("/v2.1/meta/%s/%s/reports", url.PathEscape(owner), url.PathEscape(app))
	res, err := m.client.do(ctx, requestOptions{method: http.MethodGet, path: path})
	if err != nil {
		return nil, err
	}
	var body struct {
		Code    int      `json:"code"`
		Reports []Report `json:"reports"`
	}
	if err := json.Unmarshal(res.envelope, &body); err != nil {
		return nil, fmt.Errorf("decode reports: %w", err)
	}
	return body.Reports, nil
}

// Pages returns every standalone page in the application.
func (m *MetaService) Pages(ctx context.Context, owner, app string) ([]AppPage, error) {
	if owner == "" || app == "" {
		return nil, fmt.Errorf("owner and app are required")
	}
	path := fmt.Sprintf("/v2.1/meta/%s/%s/pages", url.PathEscape(owner), url.PathEscape(app))
	res, err := m.client.do(ctx, requestOptions{method: http.MethodGet, path: path})
	if err != nil {
		return nil, err
	}
	var body struct {
		Code  int       `json:"code"`
		Pages []AppPage `json:"pages"`
	}
	if err := json.Unmarshal(res.envelope, &body); err != nil {
		return nil, fmt.Errorf("decode pages: %w", err)
	}
	return body.Pages, nil
}

// Sections returns the nav sections (grouping of reports/forms) of the app.
func (m *MetaService) Sections(ctx context.Context, owner, app string) ([]Section, error) {
	if owner == "" || app == "" {
		return nil, fmt.Errorf("owner and app are required")
	}
	path := fmt.Sprintf("/v2.1/meta/%s/%s/sections", url.PathEscape(owner), url.PathEscape(app))
	res, err := m.client.do(ctx, requestOptions{method: http.MethodGet, path: path})
	if err != nil {
		return nil, err
	}
	var body struct {
		Code     int       `json:"code"`
		Sections []Section `json:"sections"`
	}
	if err := json.Unmarshal(res.envelope, &body); err != nil {
		return nil, fmt.Errorf("decode sections: %w", err)
	}
	return body.Sections, nil
}

// Fields returns every field definition for a form. Requires scope
// ZohoCreator.meta.form.READ.
func (m *MetaService) Fields(ctx context.Context, owner, app, form string) ([]Field, error) {
	if owner == "" || app == "" || form == "" {
		return nil, fmt.Errorf("owner, app, and form are required")
	}
	path := fmt.Sprintf("/v2.1/meta/%s/%s/form/%s/fields",
		url.PathEscape(owner), url.PathEscape(app), url.PathEscape(form))
	res, err := m.client.do(ctx, requestOptions{method: http.MethodGet, path: path})
	if err != nil {
		return nil, err
	}
	var body struct {
		Code   int     `json:"code"`
		Fields []Field `json:"fields"`
	}
	if err := json.Unmarshal(res.envelope, &body); err != nil {
		return nil, fmt.Errorf("decode fields: %w", err)
	}
	return body.Fields, nil
}

// FieldTypeName returns the Creator human-readable name for a numeric field
// type code. Unknown codes return an empty string.
//
// Sources: Zoho Creator field-type reference; the list is periodically
// extended upstream, so unknown codes are surfaced rather than guessed.
func FieldTypeName(code int) string {
	if name, ok := fieldTypeNames[code]; ok {
		return name
	}
	return ""
}

// fieldTypeNames maps numeric Creator field type codes to names. Sourced from
// the Zoho Creator v2.1 meta docs.
var fieldTypeNames = map[int]string{
	1:  "SingleLine",
	2:  "Number",
	3:  "Email",
	4:  "PhoneNumber",
	5:  "Picklist",
	6:  "MultiSelect",
	7:  "Date",
	8:  "DateTime",
	9:  "Decimal",
	10: "Percent",
	11: "Currency",
	12: "Image",
	13: "Radio",
	14: "MultiLine",
	15: "Checkbox",
	16: "URL",
	17: "Lookup",
	18: "Subform",
	19: "FileUpload",
	20: "Audio",
	21: "Video",
	22: "Signature",
	23: "Notes",
	24: "Address",
	25: "AutoNumber",
	26: "Users",
	27: "RichText",
	28: "Integer",
	29: "Name",
	30: "FormulaText",
	31: "FormulaNumber",
	32: "FormulaDate",
	33: "FormulaDateTime",
	34: "Rating",
	35: "Slider",
	36: "Profile",
	37: "Prediction",
	38: "Sentiment",
	39: "Keyword",
	40: "OCR",
	41: "ObjectDetection",
	42: "BarCode",
	43: "Section",
	44: "PageBreak",
	45: "Note",
	46: "Decision",
	47: "AddNotes",
	48: "RecordSummary",
	49: "ZiaFormulaField",
	50: "FileUploadBulk",
}
