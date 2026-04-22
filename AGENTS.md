# AGENTS.md

Project-level notes for LLM agents working on the `zohocreator` Go client.

## What This Library Does

Typed Go client for the Zoho Creator REST API v2.1 (`/creator/v2.1/...`). Covers: Data APIs (CRUD records), File APIs (upload/download), Meta APIs (apps/forms/reports/fields/pages/sections), Publish APIs, Bulk Read, and Custom APIs. OAuth 2.0 with refresh-token flow, per-data-center routing, typed errors.

Upstream docs: <https://www.zoho.com/creator/help/api/v2.1/>. Upstream has per-endpoint OpenAPI 3.0 files (21 of them, downloadable via `/api/v2.1/downloadOAS?api_ref_id=<N>`) plus HTML reference pages. The OAS files are terse on parameter semantics — use the HTML docs for field-config/criteria/environment nuances.

## Architecture

- `client.go` — `Client` with `do` handling auth injection, retries, error classification. Sub-services attached as fields.
- `config.go` — `Config` validation + defaults. Supports `DataCenter` or an explicit `BaseURL`/`AccountsURL` override for tests.
- `auth.go` — `TokenSource` interface, concurrent-safe refresh-token source with 60s early-refresh window, plus `AuthURL` and `ExchangeCode` helpers for one-time OAuth setup.
- `datacenter.go` — enum + parser for the 9 Zoho regions (US/EU/IN/AU/JP/CA/CN/SA/AE).
- `errors.go` — sentinels (`ErrNotFound` etc.) + typed errors (`NotFoundError`, `RateLimitError`, etc.) that `Unwrap()` to sentinels.
- `types.go` — `Record` (map-shaped dynamic row), response codes, `codeEnvelope` helper.
- `resource.go` — generic `Page[T]` with `Iter`/`Collect`/`NextPage` and `fetchPage[T]` for cursor-paginated GETs.
- `query.go` — `Query` builder plus safe criteria helpers (`Eq`/`Ne`/`Gt`/`Contains`/`And`/`Or`/`Not`).
- `meta.go` — applications / apps-by-workspace / forms / reports / fields / pages / sections; `FieldTypeName` lookup.
- `records.go` — add / get / get-by-ID / update-by-ID / update-many / delete-by-ID / delete-many plus `AddOptions`, `UpdateOptions`, `DeleteOptions`.
- `files.go` — multipart upload, streaming download (including subform variant), RFC 2231-aware filename parsing.
- `publish.go` — privatelink-aware variants of the publish endpoints (form add + report get/getByID + iterator).
- `bulk.go` — `Create` → `Status` → `DownloadResult` lifecycle for Bulk Read jobs.
- `customapi.go` — user-defined custom API invocation with OAuth or `publickey` modes.

## Zoho Creator Quirks Worth Remembering

1. **JSON case varies.** Meta types use snake_case (`display_name`, `link_name`). Record field values echo back under their raw link names (user-authored, often TitleCase or CamelCase).
2. **Pagination via a header, not a query param.** `record_cursor` appears in the response header and must be sent back in the *request* header for the next page. The iterator handles this transparently.
3. **max_records clamps.** Defaults to 200, server caps at 1000. The bulk-read job has a separate range of 100_000..200_000.
4. **Response shape differs per endpoint.** Nearly all responses include a top-level `code` (3000 = success). Add returns `{code, result: [{code, data, message, tasks}]}` — a per-record status list. Get returns `{code, data: [...]}`. Meta returns `{code, <applications|forms|reports|fields|pages|sections>: [...]}`. Bulk wraps everything in `{code, details: {...}}`.
5. **Environment header is required.** Creator has three deployment targets (development/stage/production). The client sets it on every request; default is `production`. Omitting it often yields silent surprise when the caller expected dev data.
6. **Criteria quoting.** Criteria are Deluge-like expressions: `Field == "value"` for strings (double quotes, backslash-escaped), `Field == 42` for numbers, `Field == true` for booleans. Use the `Eq/Ne/Gt/...` helpers; never hand-concatenate user input into criteria — injection is a real risk.
7. **IDs are strings (sometimes).** `ID` fields come back as strings on reads but as integers in some create/update responses (`"ID": 3888833000000114027` vs `"ID": "3888833..."`). `Record.ID()` coerces both.
8. **OAuth per data center.** Accounts host and API host always live in the same region; the client derives both from `Config.DataCenter`. Canada is the outlier (`accounts.zohocloud.ca` rather than `accounts.zoho.ca`).
9. **Rate limits.** 50 req/min per endpoint per IP; daily quota by subscription tier. Server returns 429 with code 2955 and an optional `Retry-After`. The client honours Retry-After and falls back to exponential backoff.
10. **Under-specified endpoints.** `sections` returns components with inconsistent field sets (`page_type` sometimes, `view_type` sometimes). Custom APIs have no upstream OAS at all — we return raw bytes + headers and let callers decode.
11. **File download Content-Disposition.** Filenames can be RFC 2231-encoded (`filename*=UTF-8''...`). `parseFilename` uses `mime.ParseMediaType` so both `filename=` and `filename*=` are handled.
12. **401 handling.** The client transparently refreshes the access token and retries once on 401, outside the normal retry budget. Subsequent 401s are surfaced as `AuthError`.

## Adding or Modifying Endpoints

1. Check the per-endpoint OAS under `~/.cache/api-explorer/apis/zohocreator/raw/<timestamp>/oas-<N>.json` and the matching HTML doc under `https://www.zoho.com/creator/help/api/v2.1/<slug>.html`.
2. Add a typed request/response struct in the relevant section file (records/meta/files/publish/bulk).
3. Use `fetchPage[T]` when the endpoint returns a cursor-paginated list under a named field.
4. For one-off JSON shapes, call `client.do` directly and unmarshal the `res.envelope` into a small struct literal.
5. When upstream is under-specified (e.g. Sections components), prefer `json.RawMessage` over fabricating fields.
6. Add httptest-based tests; use `newTestClient` from `client_test.go` for boilerplate.

## Testing

```
go test ./...
```

Tests are `httptest`-based with `MaxRetries: &zero` in the helper to keep runs fast. No live-tenant integration tests — add them only with redacted creds loaded from `.secrets/.env` via `envwith`.

## Committing / Merging

Follow the user's global CLAUDE.md and the `emmaly:git-workflow` / `emmaly:integration` skills:

1. Feature branches only (`feature/<short-desc>` or `fix/<short-desc>`).
2. Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`).
3. Run `gofmt -w .`, `go vet ./...`, `go test ./...` before committing.
4. Run `coderabbit review --plain --base main` locally, fix findings, re-review until clean, then push.
5. Open PR, wait for CodeRabbit, fix follow-ups through another local clean review, then merge.

Never push to GitHub without a clean local `coderabbit` report.
