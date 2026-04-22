# zohocreator

A typed Go client for the [Zoho Creator REST API v2.1](https://www.zoho.com/creator/help/api/v2.1/). Full v2.1 coverage — data, meta, files, publish, bulk, and custom APIs — with OAuth 2.0 refresh, data-center aware routing, cursor pagination, typed errors, and transient-error retry.

## Installation

```
go get github.com/scjalliance/zohocreator
```

Requires Go 1.26+ (uses `iter.Seq2` range-over-func iterators).

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/scjalliance/zohocreator"
)

func main() {
    client, err := zohocreator.NewClient(zohocreator.Config{
        DataCenter:   zohocreator.DCUS,
        ClientID:     os.Getenv("ZOHO_CLIENT_ID"),
        ClientSecret: os.Getenv("ZOHO_CLIENT_SECRET"),
        RefreshToken: os.Getenv("ZOHO_REFRESH_TOKEN"),
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    for rec, err := range client.Records.All(ctx, "accountowner", "myapp", "All_Contacts", nil) {
        if err != nil {
            log.Fatal(err)
        }
        fmt.Println(rec.ID(), rec.String("Email"))
    }
}
```

## OAuth setup (one-time)

Register a self-client or server-based application in the [Zoho API console](https://api-console.zoho.com/), then run the bundled CLI to bootstrap tokens:

```
# 1. Open the consent URL in a browser, authorize, copy the ?code= value from the redirect
go run ./cmd/zc oauth authurl -client <client_id>

# 2. Trade the code for tokens — prints an access_token and a refresh_token
go run ./cmd/zc oauth exchange <code> -client <client_id> -secret <client_secret>

# 3. Persist the refresh_token in .secrets/.env
```

Only the refresh token is long-lived; access tokens are auto-refreshed by the client (60s early-refresh window by default). If you operate tokens externally, seed a static access token via `Config.AccessToken` and omit `RefreshToken` — no refresh will happen.

## Data centers

Pass the matching `DataCenter` constant:

| DC | API host | OAuth host |
|----|----------|------------|
| `DCUS` | `www.zohoapis.com` | `accounts.zoho.com` |
| `DCEU` | `www.zohoapis.eu` | `accounts.zoho.eu` |
| `DCIN` | `www.zohoapis.in` | `accounts.zoho.in` |
| `DCAU` | `www.zohoapis.com.au` | `accounts.zoho.com.au` |
| `DCJP` | `www.zohoapis.jp` | `accounts.zoho.jp` |
| `DCCA` | `www.zohoapis.ca` | `accounts.zohocloud.ca` |
| `DCCN` | `www.zohoapis.com.cn` | `accounts.zoho.com.cn` |
| `DCSA` | `www.zohoapis.sa` | `accounts.zoho.sa` |
| `DCAE` | `www.zohoapis.ae` | `accounts.zoho.ae` |

Or parse a user-provided string with `zohocreator.ParseDataCenter`.

## Environment header

Every Creator account has up to three deployment targets (`development`, `stage`, `production`). The client emits the `environment` HTTP header on every request; defaults to `production`. Override with `Config.Environment`.

## Services

| Service | Methods |
|---------|---------|
| `Records` | `Get`, `All`, `GetByID`, `Add`, `UpdateByID`, `UpdateMany`, `DeleteByID`, `DeleteMany` |
| `Files` | `Upload`, `Download`, `DownloadSubform` |
| `Meta` | `Applications`, `ApplicationsByWorkspace`, `Forms`, `Reports`, `Fields`, `Pages`, `Sections` |
| `Publish` | `PublishAdd`, `PublishGet`, `PublishAll`, `PublishGetByID` |
| `Bulk` | `Create`, `Status`, `DownloadResult` |
| `CustomAPIs` | `Invoke`, `InvokeStream` |

## Criteria helpers

Zoho Creator's `criteria` expressions look like Deluge. The helpers generate them safely with correct string quoting:

```go
expr := zohocreator.And(
    zohocreator.Eq("Active", true),
    zohocreator.Contains("Email", "@example.com"),
    zohocreator.Ge("Amount", 100),
)
// (Active == true && Email.contains("@example.com") && Amount >= 100)

q := zohocreator.NewQuery().CriteriaExpr(expr).MaxRecordsN(1000)
records, err := client.Records.Get(ctx, "owner", "app", "All_Orders", q)
```

## Pagination

Get Records uses a `record_cursor` HTTP header for pagination. `All` iterators follow the cursor transparently. For page-level control:

```go
page, err := client.Records.Get(ctx, owner, app, report, zohocreator.NewQuery().MaxRecordsN(1000))
for page != nil {
    for _, rec := range page.Items { /* ... */ }
    if !page.HasNext() { break }
    page, err = page.NextPage(ctx)
    if err != nil { log.Fatal(err) }
}
```

## Error handling

Errors wrap both sentinels (for `errors.Is`) and typed errors (for `errors.As`):

```go
_, err := client.Records.GetByID(ctx, owner, app, report, id)
if err != nil {
    switch {
    case errors.Is(err, zohocreator.ErrNotFound):
        // 404
    case errors.Is(err, zohocreator.ErrUnauthorized):
        // 401 (note: client also transparently retries once after a 401
        // by refreshing the access token)
    case errors.Is(err, zohocreator.ErrRateLimited):
        var rl *zohocreator.RateLimitError
        errors.As(err, &rl)
        log.Printf("retry after %ds", rl.RetryAfter)
    default:
        log.Printf("api error: %v", err)
    }
}
```

Zoho's own numeric error codes are exposed via `Error.Code` (e.g. `3000` success, `1130` API access denied, `2955` rate limit).

## Bulk read

Bulk jobs materialise a report to a CSV zip asynchronously:

```go
job, err := client.Bulk.Create(ctx, owner, app, report, &zohocreator.BulkReadQuery{
    MaxRecords: 100_000,
    Fields:     []string{"ID", "Email", "Amount"},
})

for job.Status != "Completed" {
    time.Sleep(5 * time.Second)
    job, err = client.Bulk.Status(ctx, owner, app, report, job.ID)
    if err != nil { log.Fatal(err) }
}

f, _ := os.Create("out.zip")
_, err = client.Bulk.DownloadResult(ctx, owner, app, report, job.ID, f)
```

## Custom APIs

Custom APIs are Deluge-backed user-defined endpoints at `/creator/custom/<appadmin>/<apiname>`. Use OAuth (authenticated caller) or a public key:

```go
body, headers, err := client.CustomAPIs.Invoke(ctx, "jason18", "my-report-api",
    &zohocreator.CustomAPIOptions{
        Method: "POST",
        Body:   map[string]any{"from": "2026-04-01"},
    })
```

## CLI

`cmd/zc` wraps the client library for ad-hoc exploration and useful ops:

```
envwith -f .secrets/.env -- go run ./cmd/zc whoami
envwith -f .secrets/.env -- go run ./cmd/zc apps
envwith -f .secrets/.env -- go run ./cmd/zc records owner app All_Orders -all -format ndjson
envwith -f .secrets/.env -- go run ./cmd/zc bulk-create owner app All_Orders -max 100000
```

Run `go run ./cmd/zc help` for the full list.

Required environment:

| Variable | Purpose |
|----------|---------|
| `ZOHO_CLIENT_ID` | OAuth client ID |
| `ZOHO_CLIENT_SECRET` | OAuth client secret |
| `ZOHO_REFRESH_TOKEN` | Long-lived refresh token |
| `ZOHO_DATA_CENTER` | `us`/`eu`/`in`/`au`/`jp`/`ca`/`cn`/`sa`/`ae` (default `us`) |
| `ZOHO_ENV` | `production`/`stage`/`development` (default `production`) |

## Documentation

- [Zoho Creator v2.1 API docs](https://www.zoho.com/creator/help/api/v2.1/)
- [Agent notes](docs/agent-notes.md) — LLM-oriented cheat sheet
- [PRD](docs/prd.md) — goals and scope
- [AGENTS.md](AGENTS.md) — collaboration notes

## License

MIT.
