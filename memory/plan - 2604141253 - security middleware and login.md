---
tldr: Multi-user authentication with SQLite sessions, login form, user management, and auth middleware protecting all routes
status: completed
---

# Plan: Security middleware and login

## Context

- Spec: [[spec - architecture - system design and key decisions.md]] (existing)
- Spec to create: `eidos/spec - auth - session based authentication.md`

**Decisions locked in:**
- Multi-user: accounts in `users` table (id, username, password_hash, role)
- Sessions in SQLite: `sessions` table, server-side invalidation on logout
- All routes protected: chi middleware redirects unauthenticated requests to `/login`
- Password hashing: bcrypt cost 12
- Session token: 32 bytes crypto/rand → full token in httponly cookie, SHA-256 hash in DB
- Cookie: `vvs_session`, httponly, samesite=lax, 24h expiry (configurable)
- Roles: `admin` (can manage users) + `operator` (normal access)
- Initial admin: `--admin-user` + `--admin-password` CLI flags; creates/updates on startup

## Phases

### Phase 1 — Spec — status: completed

1. [x] Create `eidos/spec - auth - session based authentication.md`
   - covers user aggregate, session aggregate, middleware behaviour, login UX, user management
   - => [[spec - auth - session based authentication.md]] created

### Phase 2 — Domain + migrations — status: completed

2. [x] Create `modules/auth/domain/user.go` + `user_test.go`
   - => User{ID, Username, PasswordHash, Role, CreatedAt, UpdatedAt} with NewUser (bcrypt cost 12), VerifyPassword, ChangePassword, IsAdmin
   - => all tests pass
3. [x] Create `modules/auth/domain/session.go`
   - => Session with NewSession(userID, tokenHash, ttl) and IsExpired()
4. [x] Create `modules/auth/domain/repository.go`
   - => UserRepository and SessionRepository interfaces
5. [x] Create `modules/auth/migrations/001_create_auth.sql` + embed.go
   - => users + sessions tables with unique indexes on token_hash; FK sessions→users ON DELETE CASCADE

### Phase 3 — Application layer — status: completed

6. [x] Create `modules/auth/app/commands/`
   - => create_user (uniqueness check), delete_user (clears sessions too), change_password, login (32-byte crypto/rand token + SHA-256 hash), logout
7. [x] Create `modules/auth/app/queries/`
   - => list_users returns []UserRow; get_current_user hashes raw cookie token, checks expiry, returns *User or nil

### Phase 4 — Persistence — status: completed

8. [x] Create `modules/auth/adapters/persistence/models.go` + `gorm_repository.go`
   - => GormUserRepository + GormSessionRepository with writer/reader split

### Phase 5 — HTTP handlers + templates — status: completed

9. [x] Create `modules/auth/adapters/http/handlers.go`
   - => all routes implemented; PUT /api/users/{id}/password deferred (not in scope of initial plan)
   - => context.go exports WithUser/UserFromContext/WithUser helpers
10. [x] Create `modules/auth/adapters/http/templates.templ`
    - => LoginPage with dark card and orange accents; UserListPage + UserTable with role badges; inline create modal; roleBadge, createUserError, loginError partials

### Phase 6 — Middleware — status: completed

11. [x] Create `infrastructure/http/auth_middleware.go`
    - => RequireAuth wraps chi group; skips /login, /api/login, /static/*
    - => stores *User via authhttp.WithUser; UserFromContext re-export for other packages

### Phase 7 — Wiring — status: completed

12. [x] Wire auth module into `internal/app/app.go`
    - => auth migration runs first; seedAdmin creates/updates admin on startup
13. [x] Update `internal/app/config.go` — AdminUser, AdminPassword added
14. [x] Update `cmd/server/main.go` — --admin-user, --admin-password flags + VVS_ADMIN_USER/VVS_ADMIN_PASSWORD env vars
15. [x] Update `infrastructure/http/router.go`
    - => RequireAuth wraps protected chi group; /login and /static/* outside group

## Verification

- Visiting any page while logged out redirects to /login
- Correct username + password logs in and redirects to /
- Wrong password shows error on login form
- Logout clears session and redirects to /login
- Admin can create and delete users
- Non-admin cannot access /users
- Session survives page refresh; expires after 24h
- Server restart with same DB: existing sessions still valid

## Adjustments

## Progress Log

- 2604141253 — Phase 1 done: auth spec created
- 2604141400 — Phases 2–7 done: full auth implementation; build clean, all tests pass
