---
tldr: Single-use magic-link tokens + rate limiting on /i/{token} and magic-link request endpoints
status: active
---

# Plan: Magic-Link Hardening + Portal Rate Limiting

## Context

Consilium 2026-04-19 priority #2. Two related portal auth weaknesses bundled as one security sprint.

1. **Magic-link tokens** — currently multi-use with long TTL; an intercepted link (forwarded email, proxy log) grants persistent access
2. **Portal rate limiting** — `/i/{token}` PDF endpoint and magic-link request endpoint have no rate limit; enumerable

- Consilium backlog: [[project_consilium_backlog_priority]] — #2
- Existing: login rate limit (5 attempts / 15min) in office auth middleware

## Phases

### Phase 1 — Magic-Link Single-Use — status: open

1. [ ] Add `used_at` column to portal magic-link tokens table
   - new goose migration in portal module
   - `ALTER TABLE portal_magic_link_tokens ADD COLUMN used_at DATETIME`

2. [ ] Mark token used on first successful auth
   - file: portal auth middleware / token validator
   - after validating token: `UPDATE portal_magic_link_tokens SET used_at = NOW() WHERE token_hash = ?`
   - reject if `used_at IS NOT NULL` → return 401 "link already used"

3. [ ] Shorten magic-link TTL from current value to 15 minutes
   - find where TTL is set (likely `NewMagicLinkToken` or similar)
   - change to 15 * time.Minute

4. [ ] Test: used token returns 401; expired token returns 401; fresh token works once
   - extend existing portal auth tests

### Phase 2 — Portal Rate Limiting — status: open

1. [ ] Add in-memory rate limiter for portal endpoints
   - reuse existing rate limit pattern from office login (check `internal/infrastructure/http/`)
   - key by remote IP
   - limits: magic-link request → 5 per 10min per IP; `/i/{token}` → 20 per 5min per IP

2. [ ] Apply rate limiter middleware to portal routes
   - magic-link request endpoint: find in `internal/modules/portal/adapters/http/`
   - PDF token endpoint: `GET /i/{token}` in invoice public routes
   - return 429 with `Retry-After` header on limit exceeded

3. [ ] Test: > N requests from same IP within window → 429
   - use `httptest` with loopback IP

## Verification

```bash
go test ./internal/modules/portal/... -v
go build ./cmd/portal/
# Request magic link → follow link → works
# Follow same link again → 401 "link already used"
# Request magic link 6× in 10min from same IP → 429
# GET /i/{token} 21× in 5min → 429
```

## Adjustments

## Progress Log
