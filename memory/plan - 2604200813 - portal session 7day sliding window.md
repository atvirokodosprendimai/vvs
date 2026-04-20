---
tldr: Extend customer portal sessions to 7 days with sliding refresh — magic link stays short-lived, session token issued separately on first login
status: completed
---

# Plan: Portal session 7-day sliding window

## Context

- Portal token domain: `internal/modules/portal/domain/token.go`
- Portal handlers: `internal/modules/portal/adapters/http/handlers.go`
- Two token flows:
  1. Admin magic link → `domain.NewPortalToken(customerID, 15*time.Minute)` (direct)
  2. Portal self-service login → `h.loginClient.CreatePortalToken(ctx, customerID, 15*time.Minute)` via NATS

### Root cause
`portalAuth` sets `MaxAge = time.Until(tok.ExpiresAt)` — i.e. remaining TTL of the magic link (~15 min).
Magic link token and session token are the same object. Should be separate.

### Design decisions
- Magic link stays short-lived (15 min) — security unchanged
- On magic link consumption: create NEW session token (7 days), set cookie with it
- `portalTokenStore` interface already has `Save` — no interface change needed
- Sliding refresh: if session token has < 3.5 days remaining, issue new token + refresh cookie
- No DB migration — `portal_tokens` table has all needed columns
- `portalSessionTTL = 7 * 24 * time.Hour` constant, `portalRefreshThreshold = portalSessionTTL / 2`

---

## Phases

### Phase 1 — Session token separated from magic link — status: completed

**Goal:** First login via magic link issues a 7-day session token. Cookie MaxAge = 7 days.

1. [ ] Add constants to `handlers.go`
   ```go
   const portalSessionTTL       = 7 * 24 * time.Hour
   const portalRefreshThreshold = portalSessionTTL / 2 // refresh when < 3.5 days remain
   ```

2. [ ] Update `portalAuth` in `handlers.go` — after `MarkUsed`, create session token
   - Current (broken): `MaxAge = int(time.Until(tok.ExpiresAt).Seconds())` using magic link's remaining TTL
   - New:
     ```go
     sessionTok, sessionPlain, err := domain.NewPortalToken(tok.CustomerID, portalSessionTTL)
     if err != nil { /* 500 */ }
     if err := h.tokenRepo.Save(r.Context(), sessionTok); err != nil { /* 500 */ }
     http.SetCookie(w, &http.Cookie{
         Name:     portalCookieName,
         Value:    sessionPlain,
         Path:     "/",
         HttpOnly: true,
         Secure:   h.secureCookie,
         MaxAge:   int(portalSessionTTL.Seconds()),
     })
     ```
   - `tok.CustomerID` — verify field name exists on `PortalToken` struct

3. [ ] `go build ./...` — verify no compile errors

### Phase 2 — Sliding refresh in requirePortalAuth — status: completed

**Goal:** Every authenticated request refreshes the cookie if the session is past half-TTL.

1. [ ] Update `requirePortalAuth` middleware in `handlers.go`
   - After token validated (not expired, not used):
     ```go
     if time.Until(tok.ExpiresAt) < portalRefreshThreshold {
         newTok, newPlain, err := domain.NewPortalToken(tok.CustomerID, portalSessionTTL)
         if err == nil {
             if saveErr := h.tokenRepo.Save(r, newTok); saveErr == nil {
                 http.SetCookie(w, &http.Cookie{
                     Name:     portalCookieName,
                     Value:    newPlain,
                     Path:     "/",
                     HttpOnly: true,
                     Secure:   h.secureCookie,
                     MaxAge:   int(portalSessionTTL.Seconds()),
                 })
             }
         }
         // Failure is non-fatal — existing session remains valid
     }
     ```

2. [ ] `go test ./internal/modules/portal/...` — confirm existing tests pass

### Phase 3 — Tests + commit — status: completed

1. [x] Add test: `TestPortalAuth_IssuesSessionToken_WithLongTTL`
   - => MaxAge ≥ 604800, session token in repo with ~7-day ExpiresAt ✓

2. [x] Add test: `TestPortalSession_Refresh_WhenNearExpiry` + `TestPortalSession_NoRefresh_WhenFresh`
   - => Added stubInvoiceLister to fix nil panic on /portal/invoices hit ✓

3. [x] `go test ./internal/modules/portal/...` — all green (e09d351)

4. [x] Committed — e09d351

---

## Verification

```bash
go build ./...
go test ./internal/modules/portal/...

# Manual:
# Click magic link → lands in portal → stay logged in for 7 days
# Revisit portal page after 3.5 days → cookie silently refreshed
# Logout → cookie cleared → redirect to login
# Wait 15 min → magic link expired, but existing session cookie still valid
```

---

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

---

## Progress Log

- 2026-04-20 08:13 — Plan created. Root cause: portalAuth reuses magic link token as session token. Fix: issue new 7-day token on magic link consumption + sliding refresh.
- 2026-04-20 — All 3 phases complete. Tests pass. Commits: 2000764 (impl), e09d351 (test fix stub).
