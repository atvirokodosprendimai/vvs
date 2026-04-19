---
tldr: Add editable profile fields to users — display name, division; admin edit modal + self-service on profile page
status: completed
---

# Plan: User edit — display name, division, role change

## Context

- Spec: [[spec - auth - session based authentication]]
- Related: `internal/modules/auth/` — domain, migrations, handlers, templates

**Current state:**
- `User` struct has only `Username`, `PasswordHash`, `Role`, `CreatedAt`, `UpdatedAt`
- No display name or division fields exist
- No edit endpoint — only create/delete/change-own-password
- Users table migration: `internal/modules/auth/migrations/001_create_auth.sql` (next = 004)

**Scope decisions:**
- **Display name** (`full_name`) — editable by admin + by self on profile page
- **Division** (`division`) — editable by admin only (org structure field)
- **Role** — editable by admin only (already in domain, just no UI)
- **Username** — intentionally NOT editable (login credential; changing invalidates sessions)
- **Admin edit modal** in `/users` page — inline modal, same pattern as create user
- **Self-service** on `/profile` page — can edit own display name only

## Phases

### Phase 1 — Domain + Migration — status: completed

1. [x] Add `FullName`, `Division` fields to `User` struct + `UpdateProfile` method
   - => `internal/modules/auth/domain/user.go` — added fields + `UpdateProfile()` + `ChangeRole()`
2. [x] Migration `004_add_user_profile_fields.sql`
   - => `internal/modules/auth/migrations/004_add_user_profile_fields.sql`
3. [x] Update GORM/SQLite persistence
   - => `UserModel` + `userToModel`/`userToDomain` mappers updated in `models.go`

### Phase 2 — Application Layer — status: completed

4. [x] Add `UpdateUserCommand` + handler
   - => admin: all fields; self: full_name only (division+role ignored); other non-admin: ErrForbidden
   - => `internal/modules/auth/app/commands/update_user.go`
5. [x] Update `UserRow` query model + `ListUsersHandler`
   - => `FullName`, `Division` added to `UserRow`
6. [x] Wire into `Handlers` + `app.go`
   - => `NewHandlers` signature extended; `updateUserCmd` added

### Phase 3 — HTTP + UI — status: completed

7. [x] `PUT /api/users/{id}` + `PUT /api/users/me/profile` routes + handlers
   - => signals: `editFullName`, `editDivision`, `editRole` (admin); `profileFullName` (self)
8. [x] Edit user modal in `templates.templ`
   - => inline edit modal; "Edit" button per row sets `$_editID`+`$editFullName`+`$editDivision`+`$editRole`
   - => `UserTable` + `userRow` show Full Name + Division columns
9. [x] Self-edit on `/profile` — full name input + `Save display name` button

### Phase 4 — Tests — status: completed

10. [x] Unit tests for `UpdateUserCommand` — 5 tests, all pass
    - => `update_user_test.go` — admin edit, self-edit, forbidden, not-found, invalid-role
11. [x] E2E additions
    - => `system.spec.js` — Full Name/Division columns visible, Edit button opens modal
    - => `profile.spec.js` — display name input + save button present

## Verification

- `/users` table shows `Full Name` and `Division` columns
- Admin can open edit modal, change any field, table updates in-place
- Role can be changed from the edit modal (not the create modal only)
- `/profile` shows user's full name + allows editing it
- Username cannot be changed anywhere in the UI
- `go test ./internal/modules/auth/...` passes
- E2E: edit user test green

## Adjustments

## Progress Log

- 2604191126 — all 4 phases complete; commits 2976d20, 9090de9, e10c4e2
