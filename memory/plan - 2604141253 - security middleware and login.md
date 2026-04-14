---
tldr: Multi-user authentication with SQLite sessions, login form, user management, and auth middleware protecting all routes
status: active
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

### Phase 2 — Domain + migrations — status: open

2. [ ] Create `modules/auth/domain/user.go` + `user_test.go`
   - User{ID, Username, PasswordHash, Role, CreatedAt, UpdatedAt}
   - NewUser(username, plainPassword, role) → bcrypt hashes on creation
   - ChangePassword(plain) error
   - roles: const admin/operator
3. [ ] Create `modules/auth/domain/session.go`
   - Session{ID, UserID, TokenHash, CreatedAt, ExpiresAt}
   - NewSession(userID, tokenHash, ttl)
   - IsExpired() bool
4. [ ] Create `modules/auth/domain/repository.go`
   - UserRepository: Save, FindByID, FindByUsername, ListAll, Delete
   - SessionRepository: Save, FindByTokenHash, DeleteByID, DeleteByUserID, PruneExpired
5. [ ] Create `modules/auth/migrations/001_create_auth.sql` + embed.go
   - users table: id, username (unique), password_hash, role, created_at, updated_at
   - sessions table: id, user_id (FK), token_hash (unique index), created_at, expires_at

### Phase 3 — Application layer — status: open

6. [ ] Create `modules/auth/app/commands/`
   - `create_user.go`: CreateUserCommand{Username, Password, Role} → validates uniqueness
   - `delete_user.go`: DeleteUserCommand{ID}
   - `change_password.go`: ChangePasswordCommand{UserID, NewPassword}
   - `login.go`: LoginCommand{Username, Password} → verifies bcrypt, creates session, returns token
   - `logout.go`: LogoutCommand{TokenHash} → deletes session
7. [ ] Create `modules/auth/app/queries/`
   - `list_users.go`: returns []UserRow{ID, Username, Role, CreatedAt}
   - `get_current_user.go`: GetCurrentUser{TokenHash} → returns *User or nil

### Phase 4 — Persistence — status: open

8. [ ] Create `modules/auth/adapters/persistence/models.go` + `gorm_repository.go`
   - UserModel + SessionModel GORM structs
   - GormUserRepository + GormSessionRepository

### Phase 5 — HTTP handlers + templates — status: open

9. [ ] Create `modules/auth/adapters/http/handlers.go`
   - GET /login → login page (unauthenticated only)
   - POST /api/login → LoginCommand → set cookie → redirect to /
   - POST /api/logout → LogoutCommand → clear cookie → redirect to /login
   - GET /users → user management page (admin only)
   - GET /api/users → list users SSE
   - POST /api/users → create user SSE
   - DELETE /api/users/{id} → delete user SSE
   - PUT /api/users/{id}/password → change password SSE
10. [ ] Create `modules/auth/adapters/http/templates.templ`
    - LoginPage: centered card, username + password fields, dark mode orange accents
    - UserListPage + UserTable: username, role badge, created_at, delete button (not self)
    - CreateUserForm: inline or modal, username/password/role fields

### Phase 6 — Middleware — status: open

11. [ ] Create `infrastructure/http/auth_middleware.go`
    - `RequireAuth(sessionQuery)` chi middleware
    - extracts `vvs_session` cookie → SHA-256 hash → calls GetCurrentUser query
    - if missing/expired/invalid: redirect to /login
    - if valid: store *User in context (via typed key)
    - helper: `UserFromContext(ctx) *auth.User`
    - skip if path == /login or /static/*

### Phase 7 — Wiring — status: open

12. [ ] Wire auth module into `internal/app/app.go`
    - add `{Name: "auth", FS: authmigrations.FS, TableName: "goose_auth"}` to migrations
    - create userRepo, sessionRepo, all commands, all queries
    - seed admin: if `cfg.AdminUser != ""` → CreateOrUpdateAdmin
    - pass auth handlers to router
13. [ ] Update `internal/app/config.go` — add AdminUser, AdminPassword fields
14. [ ] Update `cmd/server/main.go` — add --admin-user, --admin-password flags
15. [ ] Update `infrastructure/http/router.go`
    - add RequireAuth middleware wrapping all routes except /login + /static
    - register auth module routes

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
