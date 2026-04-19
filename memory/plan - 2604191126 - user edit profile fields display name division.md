---
tldr: Add editable profile fields to users — display name, division; admin edit modal + self-service on profile page
status: active
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

### Phase 1 — Domain + Migration — status: open

1. [ ] Add `FullName`, `Division` fields to `User` struct + `UpdateProfile` method
   - `FullName string` — display name shown in UI instead of username where appropriate
   - `Division string` — optional org unit / department
   - `UpdateProfile(fullName, division string)` — sets fields + `UpdatedAt`; admin-only setter for division
   - no validation beyond trim (both optional)

2. [ ] Migration `004_add_user_profile_fields.sql`
   - `ALTER TABLE users ADD COLUMN full_name TEXT NOT NULL DEFAULT ''`
   - `ALTER TABLE users ADD COLUMN division TEXT NOT NULL DEFAULT ''`

3. [ ] Update GORM/SQLite persistence in `internal/modules/auth/adapters/db/`
   - add columns to the GORM model / raw scan structs
   - `Save()` already exists — just ensure new fields are persisted

### Phase 2 — Application Layer — status: open

4. [ ] Add `UpdateUserCommand` + handler (admin edits any user)
   - fields: `UserID`, `FullName`, `Division`, `Role`
   - guard: caller must be admin
   - `internal/modules/auth/app/commands/update_user.go`

5. [ ] Update `UserRow` query model + `ListUsersHandler` to include new fields
   - `internal/modules/auth/app/queries/list_users.go`
   - add `FullName`, `Division` to `UserRow`

6. [ ] Wire `UpdateUserCommand` into `Handlers` in `app.go` + `NewHandlers`

### Phase 3 — HTTP + UI — status: open

7. [ ] Add `PUT /api/users/{id}` route + `updateUserSSE` handler
   - admin-only guard (`IsAdmin()`)
   - reads signals: `editFullName`, `editDivision`, `editRole`
   - patches `#user-table` on success

8. [ ] Edit user modal in `templates.templ`
   - trigger: "Edit" button per row in `userRow` — sets `$_editUserID`, `$editFullName`, `$editDivision`, `$editRole`
   - modal: full name input, division input, role select
   - submit: `@put('/api/users/' + $_editUserID)`
   - add `FullName`, `Division` columns to `UserTable` header + `userRow`

9. [ ] Self-edit on `/profile` page — full name only
   - add `FullName` display + editable input on `ProfilePage`
   - `PUT /api/users/me/profile` endpoint (self: full_name only, no division/role)

### Phase 4 — Tests — status: open

10. [ ] Unit test for `UpdateUserCommand`
    - admin can update full_name, division, role
    - non-admin returns error
    - unknown user returns error

11. [ ] E2E test: edit user flow
    - open edit modal, change display name + division, verify row updates
    - add to `e2e/system.spec.js`

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
