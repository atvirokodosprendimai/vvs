---
tldr: Fix login cookie never being sent (Set-Cookie after headers locked) and logout session not found (raw token passed as hash)
status: completed
---

# Plan: Fix auth cookie and session lookup

## Context

- Spec: [[spec - auth - session based authentication.md]]
- Bugs surfaced during first test on localhost:8080

**Root causes identified:**

1. **`loginSSE`** — `datastar.NewSSE(w, r)` locks response headers; `http.SetCookie` called afterwards is silently dropped → browser never receives the session cookie → every request redirects back to /login
2. **`logoutSSE`** — passes `cookie.Value` (raw 64-char hex token) as `LogoutCommand.TokenHash`, but `SessionRepository.FindByTokenHash` expects a SHA-256 hash of the token → session never found → logout appears to work but session remains in DB
3. **Login handler** — uses `sse.Redirect("/")` which is a Datastar JS navigation; for the success path a plain `http.Redirect` is more reliable since headers aren't streaming yet

## Phases

### Phase 1 — Fix login cookie timing — status: completed

1. [x] Fix `loginSSE` in `modules/auth/adapters/http/handlers.go`
   - => execute command first; on error create SSE and patch error element; on success set cookie then `http.Redirect(302)` — no SSE needed for success path
   - => `NewSSE` is now only called on the error path so headers stay unlocked for `Set-Cookie`

### Phase 2 — Fix logout token hashing — status: completed

2. [x] Fix `logoutSSE` in `modules/auth/adapters/http/handlers.go`
   - => SHA-256 hash the raw `cookie.Value` in the handler before passing `LogoutCommand{TokenHash: hex}`
   - => hashing stays at the adapter boundary; `LogoutCommand` keeps its current contract

## Verification

- POST `/api/login` with correct creds → response has `Set-Cookie: vvs_session=...` header → browser redirected to `/` → no redirect loop
- Navigating any page while logged in stays on that page
- POST `/api/logout` → session row deleted from DB → cookie cleared → redirected to `/login`
- After logout, old cookie value rejected (redirects to /login)

## Progress Log

- 2604141316 — Both fixes applied in one commit; build clean, all tests pass

