---
tldr: Single-use magic-link tokens + rate limiting on /i/{token} and magic-link request endpoints
status: completed
---

# Plan: Magic-Link Hardening + Portal Rate Limiting

## Context

Consilium 2026-04-19 priority #2. Two related portal auth weaknesses bundled as one security sprint.

1. **Magic-link tokens** — currently multi-use with long TTL; an intercepted link grants persistent access
2. **Portal rate limiting** — `/i/{token}` PDF endpoint and magic-link auth endpoint have no rate limit; enumerable

- Consilium backlog: [[project_consilium_backlog_priority]] — #2

## Phases

### Phase 1 — Magic-Link Single-Use — status: completed

1. [x] Add `used_at` column to portal magic-link tokens table
   - => migration `002_portal_tokens_used_at.sql`
   - => `UsedAt *time.Time` added to `PortalToken` + `portalTokenModel`

2. [x] Mark token used on first successful auth
   - => `MarkUsed(ctx, tokenHash)` added to `PortalTokenRepository` interface + `GormPortalTokenRepository`
   - => bridge: new subject `SubjectTokenMarkUsed = "isp.portal.rpc.token.markused"` + handler
   - => client: `MarkUsed` calls bridge via NATS; `FindByHash` now returns `UsedAt`
   - => handler: `portalAuth` checks `tok.IsUsed()` → renders expired page; calls `MarkUsed` before cookie

3. [x] Shorten magic-link TTL from 24h to 15 minutes
   - => `generatePortalLink` now uses `15*time.Minute`

4. [x] Tests: used token returns expired page; fresh token works once
   - => 3 new tests: `TestPortalAuth_SingleUse_*` — all pass

### Phase 2 — Portal Rate Limiting — status: completed

1. [x] Add `IPRateLimiter` to `internal/infrastructure/http/ratelimit.go`
   - => sliding window, per-IP, thread-safe; `Middleware()` returns chi-compatible middleware

2. [x] Apply rate limiter middleware to portal routes
   - => `/portal/auth` → 10/15min (`authLimiter` in portal handlers)
   - => `/i/{token}` → 20/5min (`pdfTokenLimiter` in invoice handlers)
   - => returns 429 with `Retry-After` header

3. [x] Tests: IPRateLimiter_* — 4 tests, all pass

## Verification

```bash
go test ./internal/modules/portal/... ./cmd/portal/... -v
go build ./cmd/portal/
# Click magic link → works; click again → expired page
# > 10 requests to /portal/auth from same IP within 15min → 429
```

## Adjustments

2026-04-19: Session cookie still valid after magic-link consumed (requirePortalAuth
does NOT check IsUsed — correct behavior for ongoing sessions).

## Progress Log

2026-04-19: All phases complete. commit 878141f. 12 files changed.
