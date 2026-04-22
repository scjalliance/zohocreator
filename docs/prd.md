# PRD — zohocreator Go client

## Goal

Provide a production-ready, idiomatic Go client for the Zoho Creator REST API v2.1 with:

- Full v2.1 endpoint coverage (Data, Meta, Files, Publish, Bulk, Custom APIs).
- OAuth 2.0 refresh-token handling with automatic access-token rotation.
- Multi-data-center support (US/EU/IN/AU/JP/CA/CN/SA/AE).
- Idiomatic Go: generics for paginated lists, `errors.Is`/`errors.As` ergonomics, iterator-based streaming, zero external deps.
- A usable CLI (`cmd/zc`) that an LLM agent or human operator can exercise without writing code.
- Documentation sufficient for a fresh LLM to be productive in under 60 seconds.

## Target users

- **Emmaly and other engineers at SCJ** integrating Creator apps with internal services.
- **LLM coding agents** operating on behalf of the engineering team; they need fast orientation via README + `docs/agent-notes.md`.
- **Automation scripts** (one-off migrations, exports, bulk fixups) — served by the `zc` CLI.

## Scope

In-scope:

- All 21 v2.1 endpoints as enumerated by Zoho's OAS downloads (add/get/update/delete records, file upload/download incl. subform, publish add/get, meta apps/forms/reports/fields/pages/sections, bulk read lifecycle).
- Custom APIs (out-of-band of the OAS but documented).
- OAuth 2.0 with refresh tokens; one-time authorization-code exchange for bootstrapping.
- Typed error classification and rate-limit handling.

Out of scope (for v0):

- v2.0 endpoints (deprecated; Creator recommends v2.1 for all new work).
- Webhook receivers (Creator webhooks use a Creator-side outbound model; there is no typed client surface for receiving them beyond a plain `http.Handler`).
- On-prem Creator deployments with custom base URLs (supported via `Config.BaseURL` but not explicitly documented).
- Deluge script execution or editing (not exposed by v2.1 REST).

## Constraints

- Single Go module `github.com/scjalliance/zohocreator` with no runtime dependencies beyond the standard library.
- Go 1.26+ (uses `iter.Seq2`).
- All tests are `httptest`-based to avoid hitting a live tenant in CI.
- Documentation structured per the user's project-setup skill: README (getting started), `docs/*.md` (deep dives), AGENTS.md at repo root, symlinked from CLAUDE.md.

## Success criteria

- `go test ./...` passes with >60% coverage on a fresh clone.
- `go run ./cmd/zc help` prints usage without credentials.
- `go run ./cmd/zc oauth authurl -client <id>` works without any API call.
- README walks a new engineer from "have a Zoho API console client" to "listing records" in under ten minutes.
- CodeRabbit review against a clean branch yields no high-severity findings.

## Non-goals

- Feature parity with unofficial SDKs (we are not trying to match any particular surface).
- Auto-generated code from the OAS — the OAS files are terse and don't describe all headers/semantics, so hand-written services track the API more faithfully.
