---
tldr: Multi-user login with SQLite-backed sessions, bcrypt passwords, and chi middleware protecting all routes
---

# Auth

## Target

Operators must authenticate before accessing any resource. The system supports multiple named accounts with roles so permissions can be differentiated and sessions can be revoked per user.

## Behaviour

- Every route except `/login` and `/static/*` requires an active session; unauthenticated requests redirect to `/login`
- The login form accepts username + password; incorrect credentials show an inline error without revealing which field was wrong
- A successful login creates a session and sets a `vvs_session` httponly cookie; the user is redirected to `/`
- Sessions expire after 24 hours; expired sessions are treated as unauthenticated
- Logout deletes the session server-side and clears the cookie; subsequent requests redirect to `/login`
- Admins can list, create, and delete user accounts at `/users`
- Operators cannot access `/users`; attempting to do so returns 403
- An admin cannot delete their own account
- Passwords are never stored or logged in plain text

## Design

### User model
`User{ID, Username, PasswordHash, Role, CreatedAt, UpdatedAt}`
- Roles: `admin`, `operator`
- `NewUser(username, plainPassword, role)` — bcrypt cost 12 on creation
- `ChangePassword(plain)` — re-hashes and updates

### Session model
`Session{ID, UserID, TokenHash, CreatedAt, ExpiresAt}`
- Token: 32 bytes `crypto/rand` → hex string sent in cookie; SHA-256 hash stored in DB
  - {>> never store the raw token — only the hash. Cookie holds full token, DB holds hash only}
- `IsExpired() bool` — compares ExpiresAt to now

### Middleware
`RequireAuth(sessionQuery)` chi middleware:
1. Extract `vvs_session` cookie value (the full token)
2. SHA-256 hash it → call GetCurrentUser query
3. If missing, expired, or not found → `http.Redirect` to `/login`
4. If valid → store `*User` in request context via typed key
5. Helper `UserFromContext(ctx) *auth.User` for handlers to retrieve the user

### Initial admin
Seeded at startup via `--admin-user` + `--admin-password` CLI flags.
If the username already exists, its password is updated. Allows password reset without DB access.

### Module layout
Follows standard hexagonal pattern in `modules/auth/`:
```
domain/      user.go, session.go, repository.go
app/commands/  create_user, delete_user, change_password, login, logout
app/queries/   list_users, get_current_user
adapters/persistence/  GORM models + repos
adapters/http/  handlers + templates (login page, user management)
migrations/    001_create_auth.sql
```

## Verification

- Unauthenticated GET / redirects to /login
- POST /api/login with correct creds → cookie set → redirect to /
- POST /api/login with wrong creds → 200 SSE with inline error, no cookie
- POST /api/logout → session deleted in DB → cookie cleared → redirect /login
- After logout, old cookie value no longer authenticates
- Admin can create and delete users at /users
- Operator gets 403 on /users
- `--admin-password newpass` on restart updates admin password

## Interactions

- Depends on [[spec - architecture - system design and key decisions.md]] (WriteSerializer, chi router, Datastar SSE pattern)
- Auth middleware wraps the chi router registered in `infrastructure/http/router.go`

## Mapping

> [[internal/modules/auth/domain/user.go]]
> [[internal/modules/auth/domain/session.go]]
> [[internal/modules/auth/adapters/http/handlers.go]]
> [[internal/infrastructure/http/auth_middleware.go]]
> [[internal/modules/auth/migrations/001_create_auth.sql]]
