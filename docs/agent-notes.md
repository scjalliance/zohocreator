# LLM Agent Notes

Companion to [AGENTS.md](../AGENTS.md). Use this when an LLM needs the "how do I do X" answer fast without reading every file.

## Minimum Working Client

```go
client, err := zohocreator.NewClient(zohocreator.Config{
    DataCenter:   zohocreator.DCUS,
    ClientID:     os.Getenv("ZOHO_CLIENT_ID"),
    ClientSecret: os.Getenv("ZOHO_CLIENT_SECRET"),
    RefreshToken: os.Getenv("ZOHO_REFRESH_TOKEN"),
})
```

Everything else hangs off `client.<Service>.<Method>`.

## Pattern Recognition

- **Pages**: list endpoints return `*Page[T]` with `Cursor`, `Items`, `HasNext()`, `NextPage(ctx)`, `Iter(ctx)`, `Collect(ctx)`.
- **All**: whenever a `Get` / `List` method exists, an `All` counterpart yields an `iter.Seq2[T, error]` that follows the cursor transparently. Prefer `All` unless you need the cursor or per-page control.
- **GetByID**: takes owner, app, report, recordID. Returns a `Record`.
- **Add**: `records []Record, opts *AddOptions` → `[]AddResult` with per-record `Code`/`Message`/`Data`/`Tasks`.
- **UpdateByID vs UpdateMany**: use `UpdateByID` for one record; `UpdateMany` requires `opts.Criteria` — validated at the library level before making a request.
- **Delete** mirrors update: `DeleteByID` vs `DeleteMany`.
- **Bulk**: `Create` → poll `Status` → `DownloadResult`. Statuses: `Scheduled`, `In-progress`, `Completed`, `Failed`.
- **Publish**: everything with a `Publish` prefix can take a `privatelink` string; when non-empty the client drops the OAuth header and appends `?privatelink=...`.
- **CustomAPIs.Invoke**: returns `([]byte, http.Header, error)` because Deluge endpoints may produce arbitrary payloads — decode at call site.

## Query Cheat Sheet

```go
q := zohocreator.NewQuery().
    FromOffset(0).
    LimitN(500).
    MaxRecordsN(1000).
    FieldConfigMode(zohocreator.FieldConfigCustom).
    FieldsList("ID", "Email", "Name").
    CriteriaExpr(zohocreator.And(
        zohocreator.Eq("Active", true),
        zohocreator.Contains("Email", "@example.com"),
    ))

q.Cursor("prev-record-cursor") // resume a previous page
q.Set("extra", "val")          // escape hatch for new server params
```

Criteria helpers: `Eq`, `Ne`, `Gt`, `Ge`, `Lt`, `Le`, `Contains`, `StartsWith`, `EndsWith`, `And`, `Or`, `Not`. Strings auto-quote; numeric/bool literals are rendered bare.

## Errors At A Glance

| Sentinel | Typical cause |
|----------|--------------|
| `ErrUnauthorized` | 401: token invalid/expired (client also auto-refreshes once) |
| `ErrForbidden` | 403: authenticated but lacks permission or app disabled |
| `ErrNotFound` | 404: owner/app/form/report/record missing |
| `ErrBadRequest` | 400: validation / schema failure (Zoho code often 3070, 3020, 3950) |
| `ErrConflict` | 409 (rare; some bulk operations) |
| `ErrRateLimited` | 429: 50/min/endpoint per IP — use `RateLimitError.RetryAfter` |
| `ErrServer` | 5xx |

```go
if errors.Is(err, zohocreator.ErrNotFound) { /* … */ }

var ve *zohocreator.ValidationError
if errors.As(err, &ve) {
    log.Printf("zoho code=%d, msg=%s", ve.Base.Code, ve.Base.Message)
}
```

## Zoho Response Codes (most common)

| Code | Meaning |
|------|---------|
| `3000` | Success |
| `1030` | Auth failure (stale token) |
| `2933` | Permission denied on application |
| `2892` | Application not found |
| `2893` | Form not found |
| `2899` | Permission denied on add |
| `2945` | Invalid scope on token |
| `2955` | Rate limit exceeded |
| `2965` | Form validation: too many rows |
| `3020` | Request body missing |
| `3060` | Record limit reached (upgrade subscription) |
| `3070` | Data validation failure (per-row) |
| `3950` | > 200 records per request |
| `4000` | Developer API daily limit reached |
| `1080` | API calls blocked for disabled app |
| `1130` | API access denied (permission set) |
| `1040` | Owner not found |

Exposed as constants: `zohocreator.CodeSuccess`, `zohocreator.CodeRateLimit`, etc.

## OAuth Flow Recap

1. `zohocreator.AuthURL(dc, clientID, redirect, scopes, state)` → prints the consent URL.
2. User visits URL, approves, is redirected to `redirect?code=...&state=...`.
3. `zohocreator.ExchangeCode(ctx, dc, clientID, clientSecret, code, redirect, nil)` → returns `AccessToken` + `RefreshToken`.
4. Persist `RefreshToken`. Hand it to `Config.RefreshToken`. Client auto-refreshes access tokens on demand.

Scopes — pick the union of these per use case:

| Scope | Enables |
|-------|---------|
| `ZohoCreator.dashboard.READ` | `Meta.Applications` / `ApplicationsByWorkspace` |
| `ZohoCreator.meta.application.READ` | `Meta.Forms` / `Meta.Reports` / `Meta.Pages` / `Meta.Sections` |
| `ZohoCreator.meta.form.READ` | `Meta.Fields` |
| `ZohoCreator.form.CREATE` | `Records.Add`, `Publish.PublishAdd` |
| `ZohoCreator.report.READ` | `Records.Get`, `Records.GetByID`, `Files.Download` |
| `ZohoCreator.report.UPDATE` | `Records.UpdateByID`, `Records.UpdateMany` |
| `ZohoCreator.report.DELETE` | `Records.DeleteByID`, `Records.DeleteMany` |
| `ZohoCreator.report.CREATE` | `Files.Upload` (despite the name) |
| `ZohoCreator.bulk.CREATE` | `Bulk.Create` |
| `ZohoCreator.bulk.READ` | `Bulk.Status`, `Bulk.DownloadResult` |

## Pagination Recipes

All records as a slice:

```go
rows, err := client.Records.Get(ctx, owner, app, rpt, nil).Collect(ctx)
```

Wait — `Get` already returns the first page; to collect:

```go
page, err := client.Records.Get(ctx, owner, app, rpt, zohocreator.NewQuery().MaxRecordsN(1000))
if err != nil { return err }
all, err := page.Collect(ctx) // follows record_cursor
```

Or stream directly:

```go
for rec, err := range client.Records.All(ctx, owner, app, rpt, nil) {
    if err != nil { return err }
    // rec is a Record (map[string]any)
}
```

## Bulk Read Loop

```go
job, err := client.Bulk.Create(ctx, owner, app, rpt, &zohocreator.BulkReadQuery{
    Criteria:   zohocreator.Gt("Created_Time", `"2026-01-01"`),
    MaxRecords: 100_000,
})
for {
    time.Sleep(10 * time.Second)
    job, err = client.Bulk.Status(ctx, owner, app, rpt, job.ID)
    if err != nil { return err }
    switch job.Status {
    case "Completed":
        f, _ := os.Create("out.zip")
        defer f.Close()
        _, err = client.Bulk.DownloadResult(ctx, owner, app, rpt, job.ID, f)
        return err
    case "Failed":
        return fmt.Errorf("bulk job failed")
    }
}
```

## Custom API Invocation

```go
body, headers, err := client.CustomAPIs.Invoke(ctx, "admin", "my-api",
    &zohocreator.CustomAPIOptions{
        Method:    "POST",
        Body:      map[string]any{"from": "2026-01-01"},
        Query:     url.Values{"format": []string{"json"}},
    })
// or with a public key (no OAuth):
body, _, err = client.CustomAPIs.Invoke(ctx, "admin", "my-api",
    &zohocreator.CustomAPIOptions{PublicKey: "ak_xxxxxxxxxxx"})
```

## Known Upstream Gaps (2026-04-21)

- Upstream OAS files do not describe `record_cursor` usage; it's documented as a response *and* request header in HTML docs only — the client mirrors the HTML docs.
- Sections API components have inconsistent optional fields (`page_type` vs `view_type`) — modelled as optional fields on `SectionComponent`.
- No OAS for Custom APIs; return type is `[]byte` + `http.Header`.
- Some per-record `AddResult` payloads include `tasks` with arbitrary Deluge-generated shapes (e.g. `openurl`); kept as `json.RawMessage`.
- Saudi Arabia (SA) and UAE (AE) data centers were added in 2024–2025; accounts-host endpoints rarely fail but verify in production if you hit a region-specific issue.
