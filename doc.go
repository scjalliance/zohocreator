// Package zohocreator is a typed Go client for the Zoho Creator REST API v2.1.
//
// It covers the full v2.1 surface:
//
//   - Data APIs: get / add / update / delete records (single and bulk-by-criteria).
//   - File APIs: upload and download file-upload fields, including from subforms.
//   - Meta APIs: applications (global + workspace), forms, reports, fields, pages,
//     sections.
//   - Publish APIs: publicly published forms and reports.
//   - Bulk APIs: create, poll, and download Bulk Read jobs.
//   - Custom APIs: invoke Deluge-backed custom REST endpoints defined in Creator.
//
// The client handles OAuth 2.0 access-token refresh, data-center aware routing
// (US / EU / IN / AU / JP / CA / CN / SA / AE), the `environment` header
// (development / stage / production), transparent cursor pagination on list
// endpoints, typed errors keyed on both HTTP status and Creator's own error
// codes, and transient-error retries with exponential backoff.
//
// A minimum working client looks like:
//
//	client, err := zohocreator.NewClient(zohocreator.Config{
//	    DataCenter:   zohocreator.DCUS,
//	    ClientID:     os.Getenv("ZOHO_CLIENT_ID"),
//	    ClientSecret: os.Getenv("ZOHO_CLIENT_SECRET"),
//	    RefreshToken: os.Getenv("ZOHO_REFRESH_TOKEN"),
//	    Environment:  zohocreator.EnvProduction,
//	})
//	if err != nil { /* ... */ }
//
//	for rec, err := range client.Records.All(ctx, "accountowner", "myapp", "All_Contacts", nil) {
//	    if err != nil { /* ... */ }
//	    fmt.Println(rec["Email"])
//	}
//
// See the README for a fuller walkthrough and docs/agent-notes.md for LLM hints.
package zohocreator
