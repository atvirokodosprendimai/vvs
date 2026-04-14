---
tldr: Fix login cookie never being sent (Set-Cookie after headers locked) and logout session not found (raw token passed as hash)
status: active
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

### Phase 1 — Fix login cookie timing — status: open

1. [ ] Fix `loginSSE` in `modules/auth/adapters/http/handlers.go`
   - Move command execution before `NewSSE`
   - On success: set cookie THEN call `NewSSE` THEN redirect
   - On error: create `NewSSE` only on the error path (for `PatchElementTempl`)
   - Use `http.Redirect(w, r, "/", http.StatusFound)` for success path (no SSE needed — just a plain redirect after cookie is set)

### Phase 2 — Fix logout token hashing — status: open

2. [ ] Fix `logoutSSE` in `modules/auth/adapters/http/handlers.go`
   - Hash `cookie.Value` with SHA-256 before passing to `LogoutCommand`
   - OR change `LogoutCommand` to accept raw token and hash internally (keeps hashing logic in one place)
   - Prefer: hash in the command — consistent with `LoginHandler` and `GetCurrentUserHandler`

## Verification

- POST `/api/login` with correct creds → response has `Set-Cookie: vvs_session=...` header → browser redirected to `/` → no redirect loop
- Navigating any page while logged in stays on that page
- POST `/api/logout` → session row deleted from DB → cookie cleared → redirected to `/login`
- After logout, old cookie value rejected (redirects to /login)

## Progress Log

